#!/usr/bin/env bash
set -euo pipefail

# Idempotently provisions the Cloud Logging metrics, Cloud Monitoring alerts
# and dashboard for the Pymes v3 worker. The worker emits one bounded,
# tenant-aggregated `worker_metrics` JSON record per minute; filters below
# intentionally use only service names, counters and booleans.

: "${PYMES_DEPLOY_ENV:?set PYMES_DEPLOY_ENV to stg or prd}"
case "$PYMES_DEPLOY_ENV" in stg|prd) ;; *) echo "PYMES_DEPLOY_ENV must be stg or prd" >&2; exit 2 ;; esac

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
prefix="pymes-v3-${PYMES_DEPLOY_ENV}"
worker_service="$prefix-worker"
dry_run=${PYMES_MONITORING_DRY_RUN:-false}
channels=${PYMES_MONITORING_NOTIFICATION_CHANNELS:-}

case "$dry_run" in true|false) ;; *) echo "PYMES_MONITORING_DRY_RUN must be true or false" >&2; exit 2 ;; esac
if [[ "$PYMES_DEPLOY_ENV" == "prd" && -z "$channels" && "$dry_run" != "true" && "${PYMES_MONITORING_ALLOW_NO_CHANNELS:-false}" != "true" ]]; then
  echo "PRD requires PYMES_MONITORING_NOTIFICATION_CHANNELS or explicit PYMES_MONITORING_ALLOW_NO_CHANNELS=true" >&2
  exit 2
fi
for dependency in jq; do
  command -v "$dependency" >/dev/null || { echo "$dependency is required" >&2; exit 2; }
done
if [[ "$dry_run" != "true" ]]; then
  command -v gcloud >/dev/null || { echo "gcloud is required" >&2; exit 2; }
fi

channels_json='[]'
if [[ -n "$channels" ]]; then
  channels_json=$(jq -nc --arg channels "$channels" '
    $channels | split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))
  ')
  invalid_channel=$(jq -r --arg project "$project" '
    map(select(test("^projects/[^/]+/notificationChannels/[^/]+$") | not)) | first // ""
  ' <<<"$channels_json")
  if [[ -n "$invalid_channel" ]]; then
    echo "notification channels must be full projects/.../notificationChannels/... resource names" >&2
    exit 2
  fi
fi

base_filter="resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"$worker_service\" AND jsonPayload.event=\"worker_metrics\""
runbook="https://github.com/devpablocristo/pymes/blob/main/v3/docs/10-runbook-operacion.md"

declare -A metric_filters=(
  [heartbeat]="$base_filter"
  [outbox_backlog]="$base_filter AND jsonPayload.outbox_pending>0"
  [outbox_delayed]="$base_filter AND jsonPayload.outbox_oldest_age_seconds>300"
  [outbox_dead_letters]="$base_filter AND jsonPayload.outbox_dead_letters>0"
  [fiscal_uncertain]="$base_filter AND jsonPayload.fiscal_uncertain>0"
  [notifications_stalled]="$base_filter AND jsonPayload.notifications_stalled>0"
  [notifications_failed]="$base_filter AND jsonPayload.notifications_failed>0"
  [dependency_circuit_open]="$base_filter AND jsonPayload.dependency_circuits_open>0"
  [worker_not_ready]="$base_filter AND jsonPayload.ready=false"
)

metric_name() {
  printf 'pymes_v3_%s_%s' "$PYMES_DEPLOY_ENV" "$1"
}

ensure_log_metric() {
  local suffix="$1" name
  name=$(metric_name "$suffix")
  if [[ "$dry_run" == "true" ]]; then
    printf 'metric %s\n' "$name"
    return
  fi
  if gcloud logging metrics describe "$name" --project="$project" >/dev/null 2>&1; then
    gcloud logging metrics update "$name" \
      --project="$project" \
      --description="Pymes v3 ${PYMES_DEPLOY_ENV}: worker ${suffix//_/ } intervals" \
      --log-filter="${metric_filters[$suffix]}" \
      --quiet >/dev/null
  else
    gcloud logging metrics create "$name" \
      --project="$project" \
      --description="Pymes v3 ${PYMES_DEPLOY_ENV}: worker ${suffix//_/ } intervals" \
      --log-filter="${metric_filters[$suffix]}" \
      --quiet >/dev/null
  fi
}

log_match_policy() {
  local display_name="$1" condition_name="$2" filter="$3" severity="$4" guidance="$5"
  jq -nc \
    --arg display "$display_name" \
    --arg condition "$condition_name" \
    --arg filter "$filter" \
    --arg environment "$PYMES_DEPLOY_ENV" \
    --arg severity "$severity" \
    --arg guidance "$guidance" \
    --arg runbook "$runbook" \
    --argjson channels "$channels_json" '
    {
      displayName: $display,
      documentation: {
        mimeType: "text/markdown",
        content: ($guidance + "\n\nRunbook: " + $runbook)
      },
      userLabels: {app: "pymes-v3", environment: $environment, severity: $severity},
      conditions: [{
        displayName: $condition,
        conditionMatchedLog: {filter: $filter}
      }],
      combiner: "OR",
      enabled: true,
      notificationChannels: $channels,
      alertStrategy: {
        notificationRateLimit: {period: "900s"},
        autoClose: "1800s"
      }
    }'
}

heartbeat_policy() {
  local display_name="$1" heartbeat_metric
  heartbeat_metric=$(metric_name heartbeat)
  jq -nc \
    --arg display "$display_name" \
    --arg environment "$PYMES_DEPLOY_ENV" \
    --arg metric "$heartbeat_metric" \
    --arg runbook "$runbook" \
    --argjson channels "$channels_json" '
    {
      displayName: $display,
      documentation: {
        mimeType: "text/markdown",
        content: ("No Pymes v3 worker metric heartbeat arrived for three minutes. Verify the Cloud Run revision, readiness and database connectivity.\n\nRunbook: " + $runbook)
      },
      userLabels: {app: "pymes-v3", environment: $environment, severity: "page"},
      conditions: [{
        displayName: "Worker metric heartbeat absent",
        conditionAbsent: {
          filter: ("metric.type=\"logging.googleapis.com/user/" + $metric + "\" AND resource.type=\"cloud_run_revision\""),
          duration: "180s",
          aggregations: [{
            alignmentPeriod: "60s",
            perSeriesAligner: "ALIGN_SUM",
            crossSeriesReducer: "REDUCE_SUM",
            groupByFields: ["resource.label.service_name"]
          }],
          trigger: {count: 1}
        }
      }],
      combiner: "OR",
      enabled: true,
      notificationChannels: $channels,
      alertStrategy: {autoClose: "1800s"}
    }'
}

ensure_policy() {
  local policy_json="$1" display_name existing count
  display_name=$(jq -r '.displayName' <<<"$policy_json")
  jq -e '(.displayName | length > 0) and (.conditions | length == 1)' <<<"$policy_json" >/dev/null
  if [[ "$dry_run" == "true" ]]; then
    printf 'policy %s\n' "$display_name"
    return
  fi
  existing=$(gcloud monitoring policies list \
    --project="$project" \
    --filter="displayName=\"$display_name\"" \
    --format='value(name)')
  count=$(awk 'NF {count++} END {print count+0}' <<<"$existing")
  if [[ "$count" -gt 1 ]]; then
    echo "multiple alert policies found with display name: $display_name" >&2
    exit 1
  fi
  if [[ "$count" -eq 1 ]]; then
    policy_json=$(jq -c --arg name "$existing" '.name = $name' <<<"$policy_json")
    gcloud monitoring policies update "$existing" --project="$project" --policy="$policy_json" --quiet >/dev/null
  else
    gcloud monitoring policies create --project="$project" --policy="$policy_json" --quiet >/dev/null
  fi
}

dashboard_json() {
  local heartbeat_metric outbox_metric delayed_metric dead_letter_metric uncertain_metric notification_stalled_metric notification_failed_metric circuit_metric
  heartbeat_metric=$(metric_name heartbeat)
  outbox_metric=$(metric_name outbox_backlog)
  delayed_metric=$(metric_name outbox_delayed)
  dead_letter_metric=$(metric_name outbox_dead_letters)
  uncertain_metric=$(metric_name fiscal_uncertain)
  notification_stalled_metric=$(metric_name notifications_stalled)
  notification_failed_metric=$(metric_name notifications_failed)
  circuit_metric=$(metric_name dependency_circuit_open)
  jq -nc \
    --arg display "Pymes v3 ${PYMES_DEPLOY_ENV^^} delivery" \
    --arg heartbeat "$heartbeat_metric" \
    --arg outbox "$outbox_metric" \
    --arg delayed "$delayed_metric" \
    --arg deadletters "$dead_letter_metric" \
    --arg uncertain "$uncertain_metric" \
    --arg notification_stalled "$notification_stalled_metric" \
    --arg notification_failed "$notification_failed_metric" \
    --arg circuits "$circuit_metric" '
    def chart($title; $metric):
      {
        title: $title,
        xyChart: {
          dataSets: [{
            timeSeriesQuery: {
              timeSeriesFilter: {
                filter: ("metric.type=\"logging.googleapis.com/user/" + $metric + "\" AND resource.type=\"cloud_run_revision\""),
                aggregation: {
                  alignmentPeriod: "60s",
                  perSeriesAligner: "ALIGN_SUM",
                  crossSeriesReducer: "REDUCE_SUM"
                }
              },
              unitOverride: "1"
            },
            plotType: "LINE"
          }],
          yAxis: {label: "intervals", scale: "LINEAR"}
        }
      };
    {
      displayName: $display,
      gridLayout: {
        columns: "2",
        widgets: [
          chart("Worker heartbeat"; $heartbeat),
          chart("Outbox backlog intervals"; $outbox),
          chart("Outbox delayed intervals"; $delayed),
          chart("Dead-letter intervals"; $deadletters),
          chart("Fiscal uncertainty intervals"; $uncertain),
          chart("WhatsApp stalled intervals"; $notification_stalled),
          chart("WhatsApp terminal failure intervals"; $notification_failed),
          chart("Open circuit intervals"; $circuits)
        ]
      },
      labels: {app: "pymes-v3"}
    }'
}

ensure_dashboard() {
  local config="$1" display_name existing count current
  display_name=$(jq -r '.displayName' <<<"$config")
  jq -e '(.displayName | length > 0) and (.gridLayout.widgets | length > 0)' <<<"$config" >/dev/null
  if [[ "$dry_run" == "true" ]]; then
    printf 'dashboard %s\n' "$display_name"
    return
  fi
  existing=$(gcloud monitoring dashboards list \
    --project="$project" \
    --filter="displayName=\"$display_name\"" \
    --format='value(name)')
  count=$(awk 'NF {count++} END {print count+0}' <<<"$existing")
  if [[ "$count" -gt 1 ]]; then
    echo "multiple dashboards found with display name: $display_name" >&2
    exit 1
  fi
  if [[ "$count" -eq 1 ]]; then
    current=$(gcloud monitoring dashboards describe "$existing" --project="$project" --format=json)
    config=$(jq -c \
      --arg name "$existing" \
      --arg etag "$(jq -r '.etag' <<<"$current")" \
      '.name = $name | .etag = $etag' <<<"$config")
    gcloud monitoring dashboards update "$existing" --project="$project" --config="$config" --quiet >/dev/null
  else
    gcloud monitoring dashboards create --project="$project" --config="$config" --quiet >/dev/null
  fi
}

for suffix in heartbeat outbox_backlog outbox_delayed outbox_dead_letters fiscal_uncertain notifications_stalled notifications_failed dependency_circuit_open worker_not_ready; do
  ensure_log_metric "$suffix"
done

environment_label=${PYMES_DEPLOY_ENV^^}
ensure_policy "$(heartbeat_policy "[Pymes v3 $environment_label] Worker heartbeat absent")"
ensure_policy "$(log_match_policy \
  "[Pymes v3 $environment_label] Worker not ready" \
  "Worker reported not ready" \
  "${metric_filters[worker_not_ready]}" \
  "page" \
  "The worker cannot collect delivery metrics, normally because PostgreSQL is unavailable.")"
ensure_policy "$(log_match_policy \
  "[Pymes v3 $environment_label] Dead-letter queue non-empty" \
  "Dead-letter queue has events" \
  "${metric_filters[outbox_dead_letters]}" \
  "page" \
  "Inspect failure codes and repair the cause before an explicit, audited replay.")"
ensure_policy "$(log_match_policy \
  "[Pymes v3 $environment_label] Outbox delivery delayed" \
  "Oldest outbox event exceeds five minutes" \
  "${metric_filters[outbox_delayed]}" \
  "warning" \
  "The oldest unpublished outbox event is older than five minutes.")"
ensure_policy "$(log_match_policy \
  "[Pymes v3 $environment_label] Fiscal reconciliation required" \
  "Fiscal result is uncertain" \
  "${metric_filters[fiscal_uncertain]}" \
  "warning" \
  "Reconcile by consulting the exact reserved voucher; never authorize a new number.")"
ensure_policy "$(log_match_policy \
  "[Pymes v3 $environment_label] WhatsApp delivery stalled" \
  "WhatsApp delivery has not converged" \
  "${metric_filters[notifications_stalled]}" \
  "warning" \
  "Recover PerGo and retry the same trace ID; never create a second notification intent.")"
ensure_policy "$(log_match_policy \
  "[Pymes v3 $environment_label] WhatsApp delivery failed" \
  "WhatsApp delivery reached a terminal failure" \
  "${metric_filters[notifications_failed]}" \
  "warning" \
  "Inspect only the stable failure code and provider configuration; never copy the recipient or body into logs or tickets.")"
ensure_policy "$(log_match_policy \
  "[Pymes v3 $environment_label] Dependency circuit open" \
  "Fiscal or Accounting circuit is open" \
  "${metric_filters[dependency_circuit_open]}" \
  "warning" \
  "Fiscal or Accounting failed enough requests to open its circuit breaker.")"
ensure_dashboard "$(dashboard_json)"

printf 'monitoring configuration validated for %s (%s)\n' "$prefix" "$project"
