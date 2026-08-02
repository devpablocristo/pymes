#!/usr/bin/env bash
set -Eeuo pipefail
set +x
umask 077

# Read-only, fail-closed evidence collector for controlled Pymes v3 pilots.
# It never creates business data or changes cloud/provider state. The operator
# completes the pilot first; this script then reads the exact deployed revision
# and narrowly scoped, redacted terminal projections. Raw HTTP responses and
# bearer tokens live only in a private staging directory that is removed before
# the evidence bundle is published atomically.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../../.." && pwd)

fail() {
  echo "pilot evidence collection failed: $*" >&2
  exit 1
}

for command in \
  awk basename cat chmod curl date dirname find git gcloud grep id jq mktemp \
  mv python3 rm sha256sum stat tail tr wc; do
  command -v "$command" >/dev/null 2>&1 ||
    fail "$command is required"
done

: "${PYMES_PILOT_KIND:?set PYMES_PILOT_KIND to agenda, pergo, google or arca}"
: "${PYMES_PILOT_ENV:?set PYMES_PILOT_ENV to stg or prd}"
: "${PYMES_PILOT_SOURCE_SHA:?set the exact deployed source SHA}"
: "${PYMES_PILOT_PUBLIC_BASE_URL:?set the exact deployed HTTPS origin}"
: "${PYMES_PILOT_RELEASE_MANIFEST:?set the validated release manifest path}"
: "${PYMES_PILOT_RELEASE_MANIFEST_SHA256:?set the independently recorded manifest SHA-256}"
: "${PYMES_PILOT_EVIDENCE_DIR:?set a new absolute evidence directory}"
: "${PYMES_PILOT_CONFIRMATION:?set the exact collection confirmation}"

pilot_kind=$PYMES_PILOT_KIND
environment=$PYMES_PILOT_ENV
source_sha=$PYMES_PILOT_SOURCE_SHA
public_base_url=$PYMES_PILOT_PUBLIC_BASE_URL
release_manifest=$PYMES_PILOT_RELEASE_MANIFEST
release_manifest_sha256=$PYMES_PILOT_RELEASE_MANIFEST_SHA256
evidence_dir=$PYMES_PILOT_EVIDENCE_DIR
project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
prefix="pymes-v3-${environment}"

case "$pilot_kind" in
  agenda|pergo|google|arca) ;;
  *) fail "PYMES_PILOT_KIND must be agenda, pergo, google or arca" ;;
esac
case "$environment" in
  stg|prd) ;;
  *) fail "PYMES_PILOT_ENV must be stg or prd" ;;
esac
[[ "$project" == "pymes-dev-352318" ]] ||
  fail "the pilot target project must be pymes-dev-352318"
[[ "$region" == "us-central1" ]] ||
  fail "the pilot target region must be us-central1"
[[ "$source_sha" =~ ^[0-9a-f]{40}$ ]] ||
  fail "PYMES_PILOT_SOURCE_SHA must be one full lowercase commit SHA"
[[ "$release_manifest_sha256" =~ ^[0-9a-f]{64}$ ]] ||
  fail "PYMES_PILOT_RELEASE_MANIFEST_SHA256 must be lowercase SHA-256"
[[ "$public_base_url" =~ ^https://[A-Za-z0-9.-]+$ ]] ||
  fail "PYMES_PILOT_PUBLIC_BASE_URL must be an HTTPS origin without path"
[[ "$evidence_dir" == /* ]] ||
  fail "PYMES_PILOT_EVIDENCE_DIR must be absolute"
[[ "$(basename -- "$evidence_dir")" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] ||
  fail "PYMES_PILOT_EVIDENCE_DIR has an unsafe basename"
[[ ! -e "$evidence_dir" && ! -L "$evidence_dir" ]] ||
  fail "the evidence directory already exists"
evidence_parent=$(dirname -- "$evidence_dir")
[[ -d "$evidence_parent" && ! -L "$evidence_parent" ]] ||
  fail "the evidence parent must be an existing real directory"

confirmation_kind=${pilot_kind^^}
confirmation_environment=${environment^^}
expected_confirmation="COLLECT_${confirmation_environment}_${confirmation_kind}_${source_sha}"
[[ "$PYMES_PILOT_CONFIRMATION" == "$expected_confirmation" ]] ||
  fail "PYMES_PILOT_CONFIRMATION must equal $expected_confirmation"

[[ "$release_manifest" == /* && -f "$release_manifest" &&
   ! -L "$release_manifest" ]] ||
  fail "the release manifest must be an absolute regular non-symlink file"
actual_manifest_sha256=$(sha256sum "$release_manifest" | awk '{print $1}')
[[ "$actual_manifest_sha256" == "$release_manifest_sha256" ]] ||
  fail "the release manifest checksum differs from the independently recorded value"

manifest_value() {
  local name="$1" count value
  count=$(grep -cE "^${name}=" "$release_manifest" || true)
  [[ "$count" == "1" ]] ||
    fail "the release manifest must contain exactly one $name"
  value=$(grep -E "^${name}=" "$release_manifest")
  printf '%s' "${value#*=}"
}

[[ "$(manifest_value PYMES_RELEASE_ENV)" == "$environment" ]] ||
  fail "the release manifest belongs to another environment"
[[ "$(manifest_value PYMES_SOURCE_SHA)" == "$source_sha" ]] ||
  fail "the release manifest belongs to another source SHA"
[[ "$(git -C "$repo_root" rev-parse HEAD)" == "$source_sha" ]] ||
  fail "the collector checkout HEAD differs from the deployed source SHA"

validate_identifier() {
  local value="$1" label="$2"
  [[ "$value" =~ ^[A-Za-z0-9:_./-]{1,255}$ ]] ||
    fail "$label is not a safe opaque identifier"
}

validate_uuid() {
  local value="$1" label="$2"
  [[ "$value" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]] ||
    fail "$label must be a canonical lowercase UUID"
}

validate_fiscal_credential_id() {
  local value="$1"
  [[ "$value" =~ ^fcred_[A-Za-z0-9_-]{8,80}$ ]] ||
    fail "fiscal_credential_id must be an opaque fcred_ identifier"
}

validate_secret_file() {
  local path="$1" label="$2" mode owner
  [[ "$path" == /* && -f "$path" && ! -L "$path" ]] ||
    fail "$label must be an absolute regular non-symlink file"
  mode=$(stat -c '%a' "$path")
  case "$mode" in
    400|600) ;;
    *) fail "$label must have mode 0400 or 0600" ;;
  esac
  owner=$(stat -c '%u' "$path")
  [[ "$owner" == "$(id -u)" ]] ||
    fail "$label must be owned by the current operator"
  [[ "$(wc -l <"$path")" -le 1 ]] ||
    fail "$label must contain exactly one token"
  local token
  token=$(tr -d '\n' <"$path")
  [[ "$token" =~ ^[A-Za-z0-9._~-]{20,8192}$ ]] ||
    fail "$label contains an invalid bearer token"
}

require_api_token() {
  : "${PYMES_PILOT_BEARER_TOKEN_FILE:?set a private Clerk bearer token file}"
  validate_secret_file "$PYMES_PILOT_BEARER_TOKEN_FILE" \
    PYMES_PILOT_BEARER_TOKEN_FILE
}

# Validate every pilot-specific input before the first gcloud or curl call.
case "$pilot_kind" in
  agenda)
    : "${PYMES_PILOT_ORGANIZATION_A:?set the first pilot organization}"
    : "${PYMES_PILOT_ORGANIZATION_B:?set the second pilot organization}"
    : "${PYMES_PILOT_BOOKING_A:?set the first pilot booking}"
    : "${PYMES_PILOT_BOOKING_B:?set the second pilot booking}"
    : "${PYMES_PILOT_BEARER_TOKEN_A_FILE:?set the first private Clerk token file}"
    : "${PYMES_PILOT_BEARER_TOKEN_B_FILE:?set the second private Clerk token file}"
    validate_identifier "$PYMES_PILOT_ORGANIZATION_A" organization_a
    validate_identifier "$PYMES_PILOT_ORGANIZATION_B" organization_b
    [[ "$PYMES_PILOT_ORGANIZATION_A" != "$PYMES_PILOT_ORGANIZATION_B" ]] ||
      fail "Agenda evidence requires two different organizations"
    validate_uuid "$PYMES_PILOT_BOOKING_A" booking_a
    validate_uuid "$PYMES_PILOT_BOOKING_B" booking_b
    [[ "$PYMES_PILOT_BOOKING_A" != "$PYMES_PILOT_BOOKING_B" ]] ||
      fail "Agenda evidence requires two different bookings"
    validate_secret_file "$PYMES_PILOT_BEARER_TOKEN_A_FILE" \
      PYMES_PILOT_BEARER_TOKEN_A_FILE
    validate_secret_file "$PYMES_PILOT_BEARER_TOKEN_B_FILE" \
      PYMES_PILOT_BEARER_TOKEN_B_FILE
    ;;
  pergo)
    : "${PYMES_PILOT_ORGANIZATION_ID:?set the pilot organization}"
    : "${PYMES_PILOT_NOTIFICATION_ID:?set the delivered notification ID}"
    validate_identifier "$PYMES_PILOT_ORGANIZATION_ID" organization_id
    validate_identifier "$PYMES_PILOT_NOTIFICATION_ID" notification_id
    require_api_token
    ;;
  google)
    : "${PYMES_PILOT_ORGANIZATION_ID:?set the pilot organization}"
    : "${PYMES_PILOT_GOOGLE_CONNECTION_ID:?set the active Google connection}"
    : "${PYMES_PILOT_BOOKING_ID:?set the projected booking}"
    : "${PYMES_PILOT_GOOGLE_CALENDAR_ID:?set the controlled calendar ID}"
    : "${PYMES_PILOT_GOOGLE_TOKEN_FILE:?set a private Google bearer token file}"
    : "${PYMES_PILOT_PROVIDER_CONFIRMATION:?confirm the read-only Google contact}"
    validate_identifier "$PYMES_PILOT_ORGANIZATION_ID" organization_id
    validate_uuid "$PYMES_PILOT_GOOGLE_CONNECTION_ID" google_connection_id
    validate_uuid "$PYMES_PILOT_BOOKING_ID" booking_id
    [[ "$PYMES_PILOT_GOOGLE_CALENDAR_ID" =~ ^[^[:cntrl:]\"\\]{1,1024}$ ]] ||
      fail "PYMES_PILOT_GOOGLE_CALENDAR_ID is invalid"
    validate_secret_file "$PYMES_PILOT_GOOGLE_TOKEN_FILE" \
      PYMES_PILOT_GOOGLE_TOKEN_FILE
    require_api_token
    expected_provider_confirmation="READ_GOOGLE_${confirmation_environment}_${source_sha}"
    [[ "$PYMES_PILOT_PROVIDER_CONFIRMATION" == "$expected_provider_confirmation" ]] ||
      fail "PYMES_PILOT_PROVIDER_CONFIRMATION must equal $expected_provider_confirmation"
    ;;
  arca)
    : "${PYMES_PILOT_ORGANIZATION_ID:?set the pilot organization}"
    : "${PYMES_PILOT_FISCAL_CREDENTIAL_ID:?set the homologation credential ID}"
    : "${PYMES_PILOT_SALE_ID:?set the homologated sale ID}"
    validate_identifier "$PYMES_PILOT_ORGANIZATION_ID" organization_id
    validate_fiscal_credential_id "$PYMES_PILOT_FISCAL_CREDENTIAL_ID"
    validate_identifier "$PYMES_PILOT_SALE_ID" sale_id
    require_api_token
    ;;
esac

staging_dir=$(mktemp -d "$evidence_parent/.pymes-pilot-evidence.XXXXXX")
published=false
cleanup() {
  if [[ "$published" != "true" ]]; then
    rm -rf -- "$staging_dir"
  fi
}
trap cleanup EXIT
chmod 700 "$staging_dir"

urlencode() {
  jq -rn --arg value "$1" '$value|@uri'
}

sha_ref() {
  local value="$1"
  printf 'sha256:%s' "$(printf '%s' "$value" | sha256sum | awk '{print $1}')"
}

service_env_value() {
  local name="$1"
  jq -er --arg name "$name" '
    [
      (
        .spec.template.spec.containers[0].env //
        .template.containers[0].env //
        []
      )[]
      | select(.name == $name)
      | .value
    ] | select(length == 1) | .[0]
  '
}

declare -A service_json_by_role=()
cloud_records='[]'
capture_service() {
  local role="$1" manifest_key="$2" expected_sa_prefix="$3"
  local service="$prefix-$role" service_json revision revision_json
  local release_label image expected_image service_account expected_sa
  service_json=$(gcloud run services describe "$service" \
    --project="$project" --region="$region" --format=json) ||
    fail "could not describe Cloud Run service $service"
  revision=$(jq -er '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select((.tag // "") == "" and (.percent // 0) == 100)
      | (.revisionName // .revision // empty)
    ] | unique | select(length == 1) | .[0]
  ' <<<"$service_json") ||
    fail "$service does not have one untagged 100% active revision"
  revision_json=$(gcloud run revisions describe "$revision" \
    --service="$service" --project="$project" --region="$region" \
    --format=json) ||
    fail "could not describe active revision $revision"
  release_label=$(jq -er '
    .metadata.labels["pymes-v3-release"] //
    .spec.template.metadata.labels["pymes-v3-release"] //
    empty
  ' <<<"$revision_json") ||
    fail "$revision lacks the release label"
  [[ "$release_label" == "$source_sha" ]] ||
    fail "$revision belongs to another source SHA"
  image=$(jq -er '
    .spec.containers[0].image //
    .spec.template.spec.containers[0].image //
    empty
  ' <<<"$revision_json") ||
    fail "$revision lacks an image"
  expected_image=$(manifest_value "$manifest_key")
  [[ "$image" == "$expected_image" ]] ||
    fail "$revision image differs from the pinned release manifest"
  service_account=$(jq -er '
    .spec.serviceAccount //
    .spec.serviceAccountName //
    .spec.template.spec.serviceAccountName //
    empty
  ' <<<"$revision_json") ||
    fail "$revision lacks a service account"
  expected_sa="${expected_sa_prefix}-${environment}@${project}.iam.gserviceaccount.com"
  [[ "$service_account" == "$expected_sa" ]] ||
    fail "$revision uses an unexpected service account"
  jq -e '
    [
      (.status.conditions // [])[]
      | select(.type == "Ready" and .status == "True")
    ] | length == 1
  ' <<<"$revision_json" >/dev/null ||
    fail "$revision is not ready"
  local digest
  digest=${image##*@}
  [[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]] ||
    fail "$revision image is not pinned by digest"
  cloud_records=$(jq -c \
    --argjson records "$cloud_records" \
    --arg role "$role" \
    --arg service "$service" \
    --arg revision "$revision" \
    --arg digest "$digest" \
    --arg service_account "$service_account" '
      $records + [{
        role: $role,
        service: $service,
        revision: $revision,
        image_digest: $digest,
        service_account: $service_account,
        traffic_percent: 100,
        ready: true
      }]
    ' <<<"null")
  service_json_by_role["$role"]=$service_json
}

capture_service fiscal PYMES_FISCAL_IMAGE pymes-v3-fiscal
capture_service accounting PYMES_ACCOUNTING_IMAGE pymes-v3-accounting
capture_service accounting-admin PYMES_ACCOUNTING_ADMIN_IMAGE \
  pymes-v3-accounting-admin
capture_service worker PYMES_WORKER_IMAGE pymes-v3-worker
capture_service api PYMES_API_IMAGE pymes-v3-api
capture_service web PYMES_WEB_IMAGE pymes-v3-web

http_get() {
  local url="$1" token_file="$2" body_file="$3" headers_file="$4"
  local config_file status token
  config_file=$(mktemp "$staging_dir/.curl-config.XXXXXX")
  chmod 600 "$config_file"
  {
    printf 'url = "%s"\n' "$url"
    printf 'request = "GET"\n'
    printf 'output = "%s"\n' "$body_file"
    printf 'dump-header = "%s"\n' "$headers_file"
    if [[ -n "$token_file" ]]; then
      token=$(tr -d '\n' <"$token_file")
      printf 'header = "Authorization: Bearer %s"\n' "$token"
    fi
  } >"$config_file"
  status=$(curl --silent --show-error --location --max-redirs 0 \
    --connect-timeout 10 --max-time 30 --proto '=https' --tlsv1.2 \
    --max-filesize 1048576 --write-out '%{http_code}' \
    --config "$config_file") || {
      rm -f -- "$config_file"
      fail "read-only HTTP probe failed"
    }
  rm -f -- "$config_file"
  [[ -f "$body_file" && ! -L "$body_file" ]] ||
    fail "HTTP probe did not produce a regular response"
  [[ "$(stat -c '%s' "$body_file")" -le 1048576 ]] ||
    fail "HTTP response exceeded one MiB"
  printf '%s' "$status"
}

api_url() {
  local organization="$1" suffix="$2"
  printf '%s/api/v1/organizations/%s/%s' \
    "$public_base_url" "$(urlencode "$organization")" "$suffix"
}

assert_status() {
  local actual="$1" expected="$2" label="$3"
  [[ "$actual" == "$expected" ]] ||
    fail "$label returned HTTP $actual, expected $expected"
}

probe_body="$staging_dir/.ready-body"
probe_headers="$staging_dir/.ready-headers"
ready_status=$(http_get "$public_base_url/readyz" "" \
  "$probe_body" "$probe_headers")
assert_status "$ready_status" 200 "Web readiness"
web_digest=$(manifest_value PYMES_WEB_IMAGE)
web_digest=${web_digest##*@}
expected_release_marker="${environment}:${source_sha}:${web_digest}"
actual_release_marker=$(awk '
  tolower($0) ~ /^x-pymes-release:[[:space:]]*/ {
    sub(/\r$/, "")
    sub(/^[^:]+:[[:space:]]*/, "")
    print
  }
' "$probe_headers" | tail -1)
[[ "$actual_release_marker" == "$expected_release_marker" ]] ||
  fail "the public Web release marker differs from the pinned release"

feature_probe() {
  local organization="$1" token_file="$2" feature="$3" label="$4"
  local body="$staging_dir/.features-${label}" headers="$staging_dir/.features-${label}.headers"
  local status
  status=$(http_get \
    "$(api_url "$organization" features)" "$token_file" "$body" "$headers")
  assert_status "$status" 200 "$label feature probe"
  jq -e --arg feature "$feature" --arg organization "$organization" '
    type == "object" and .organization_id == $organization and
    (.version | type == "number" and . >= 1) and
    (.[$feature] == true)
  ' "$body" >/dev/null ||
    fail "$label does not have $feature enabled"
  jq -c --arg feature "$feature" '
    {feature: $feature, enabled: .[$feature], version: .version}
  ' "$body"
}

pilot_record=
case "$pilot_kind" in
  agenda)
    org_a=$(urlencode "$PYMES_PILOT_ORGANIZATION_A")
    org_b=$(urlencode "$PYMES_PILOT_ORGANIZATION_B")
    booking_a=$(urlencode "$PYMES_PILOT_BOOKING_A")
    booking_b=$(urlencode "$PYMES_PILOT_BOOKING_B")
    feature_a=$(feature_probe "$PYMES_PILOT_ORGANIZATION_A" \
      "$PYMES_PILOT_BEARER_TOKEN_A_FILE" scheduling_enabled tenant-a)
    feature_b=$(feature_probe "$PYMES_PILOT_ORGANIZATION_B" \
      "$PYMES_PILOT_BEARER_TOKEN_B_FILE" scheduling_enabled tenant-b)
    agenda_a="$staging_dir/.agenda-a"
    agenda_b="$staging_dir/.agenda-b"
    headers_a="$staging_dir/.agenda-a.headers"
    headers_b="$staging_dir/.agenda-b.headers"
    status_a=$(http_get \
      "$public_base_url/api/v1/organizations/$org_a/scheduling/bookings/$booking_a" \
      "$PYMES_PILOT_BEARER_TOKEN_A_FILE" "$agenda_a" "$headers_a")
    status_b=$(http_get \
      "$public_base_url/api/v1/organizations/$org_b/scheduling/bookings/$booking_b" \
      "$PYMES_PILOT_BEARER_TOKEN_B_FILE" "$agenda_b" "$headers_b")
    assert_status "$status_a" 200 "Agenda tenant A booking"
    assert_status "$status_b" 200 "Agenda tenant B booking"
    for item in "$agenda_a:$PYMES_PILOT_BOOKING_A" \
      "$agenda_b:$PYMES_PILOT_BOOKING_B"; do
      file=${item%%:*}
      expected_id=${item#*:}
      jq -e --arg id "$expected_id" '
        type == "object" and .id == $id and
        (.status | IN("confirmed","checked_in","completed","no_show")) and
        (.start_at | type == "string" and length > 0) and
        (.end_at | type == "string" and length > 0) and
        (.timezone | type == "string" and length > 0) and
        (.duration_minutes | type == "number" and . > 0) and
        (.allocations | type == "array" and length > 0)
      ' "$file" >/dev/null ||
        fail "Agenda booking is not a completed real-pilot projection"
    done
    cross_a="$staging_dir/.agenda-cross-a"
    cross_b="$staging_dir/.agenda-cross-b"
    cross_headers_a="$staging_dir/.agenda-cross-a.headers"
    cross_headers_b="$staging_dir/.agenda-cross-b.headers"
    cross_status_a=$(http_get \
      "$public_base_url/api/v1/organizations/$org_a/scheduling/bookings/$booking_b" \
      "$PYMES_PILOT_BEARER_TOKEN_A_FILE" "$cross_a" "$cross_headers_a")
    cross_status_b=$(http_get \
      "$public_base_url/api/v1/organizations/$org_b/scheduling/bookings/$booking_a" \
      "$PYMES_PILOT_BEARER_TOKEN_B_FILE" "$cross_b" "$cross_headers_b")
    assert_status "$cross_status_a" 404 "Agenda tenant A cross-tenant lookup"
    assert_status "$cross_status_b" 404 "Agenda tenant B cross-tenant lookup"
    safe_a=$(jq -c --arg ref "$(sha_ref "$PYMES_PILOT_BOOKING_A")" '
      {
        booking_ref: $ref,
        status,
        start_at,
        end_at,
        timezone,
        duration_minutes,
        participants,
        allocation_count: (.allocations | length),
        allocation_modes: ([.allocations[].mode] | unique | sort)
      }
    ' "$agenda_a")
    safe_b=$(jq -c --arg ref "$(sha_ref "$PYMES_PILOT_BOOKING_B")" '
      {
        booking_ref: $ref,
        status,
        start_at,
        end_at,
        timezone,
        duration_minutes,
        participants,
        allocation_count: (.allocations | length),
        allocation_modes: ([.allocations[].mode] | unique | sort)
      }
    ' "$agenda_b")
    pilot_record=$(jq -cn \
      --argjson feature_a "$feature_a" \
      --argjson feature_b "$feature_b" \
      --argjson booking_a "$safe_a" \
      --argjson booking_b "$safe_b" '
      {
        kind: "agenda",
        result: "passed",
        tenants: [
          {label: "tenant-a", feature: $feature_a, booking: $booking_a},
          {label: "tenant-b", feature: $feature_b, booking: $booking_b}
        ],
        isolation: {
          tenant_a_cannot_read_tenant_b_booking: true,
          tenant_b_cannot_read_tenant_a_booking: true,
          observed_http_status: 404
        }
      }
    ')
    ;;
  pergo)
    api_json=${service_json_by_role[api]}
    worker_json=${service_json_by_role[worker]}
    [[ "$(service_env_value PYMES_PERGO_ENABLED <<<"$api_json")" == "true" &&
       "$(service_env_value PYMES_PERGO_ENABLED <<<"$worker_json")" == "true" ]] ||
      fail "PerGo is not enabled in both API and worker"
    pergo_url=$(service_env_value PERGO_URL <<<"$worker_json")
    [[ "$pergo_url" =~ ^https://[A-Za-z0-9.-]+(:[0-9]+)?(/[A-Za-z0-9._~!$&()*+,;=:@%/-]*)?$ &&
       "$pergo_url" != *".invalid"* &&
       "$pergo_url" != *"localhost"* ]] ||
      fail "worker PerGo URL is not a real HTTPS endpoint"
    pergo_audience=$(service_env_value PYMES_PERGO_AUDIENCE <<<"$worker_json")
    [[ "$pergo_audience" =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ &&
       "$pergo_audience" != *".invalid"* &&
       "$pergo_audience" != *"localhost"* ]] ||
      fail "worker PerGo audience is not a real exact HTTPS origin"
    pergo_channel=$(service_env_value PERGO_CHANNEL <<<"$worker_json")
    case "$pergo_channel" in
      whatsapp|whatsapp_cloud) ;;
      *) fail "worker uses a non-real PerGo channel" ;;
    esac
    [[ "$(service_env_value PERGO_ALLOW_GLOBAL_ROUTE_FALLBACK <<<"$worker_json")" == "false" ]] ||
      fail "worker global PerGo route fallback is enabled"
    feature=$(feature_probe "$PYMES_PILOT_ORGANIZATION_ID" \
      "$PYMES_PILOT_BEARER_TOKEN_FILE" whatsapp_enabled pilot)
    notification="$staging_dir/.notification"
    notification_headers="$staging_dir/.notification.headers"
    notification_status=$(http_get \
      "$(api_url "$PYMES_PILOT_ORGANIZATION_ID" \
        "notifications/$(urlencode "$PYMES_PILOT_NOTIFICATION_ID")")" \
      "$PYMES_PILOT_BEARER_TOKEN_FILE" "$notification" \
      "$notification_headers")
    assert_status "$notification_status" 200 "PerGo notification"
    jq -e --arg id "$PYMES_PILOT_NOTIFICATION_ID" '
      type == "object" and .id == $id and
      (.status | IN("delivered","read")) and
      (.external_message_id | type == "string" and length > 0) and
      ((.failure_code // "") == "")
    ' "$notification" >/dev/null ||
      fail "notification has not converged to a provider-confirmed delivery"
    safe_notification=$(jq -c \
      --arg notification_ref "$(sha_ref "$PYMES_PILOT_NOTIFICATION_ID")" '
      {
        notification_ref: $notification_ref,
        provider_message_ref: (
          .external_message_id |
          @base64 |
          "base64-sha-input:" + .
        ),
        kind,
        template_version,
        locale,
        status,
        send_at,
        created_at,
        updated_at
      }
    ' "$notification")
    # Replace the reversible intermediate with a one-way reference without
    # ever writing the provider ID to the bundle.
    provider_message_id=$(jq -r '.external_message_id' "$notification")
    safe_notification=$(jq -c \
      --arg provider_ref "$(sha_ref "$provider_message_id")" '
      .provider_message_ref = $provider_ref
    ' <<<"$safe_notification")
    pilot_record=$(jq -cn \
      --argjson feature "$feature" \
      --argjson notification "$safe_notification" \
      --arg endpoint_ref "$(sha_ref "$pergo_url")" \
      --arg audience_ref "$(sha_ref "$pergo_audience")" \
      --arg channel "$pergo_channel" '
      {
        kind: "pergo",
        result: "passed",
        feature: $feature,
        runtime: {
          enabled_in_api_and_worker: true,
          endpoint_ref: $endpoint_ref,
          serverless_identity_audience_configured: true,
          audience_ref: $audience_ref,
          channel: $channel,
          tenant_route_fallback: false
        },
        notification: $notification
      }
    ')
    ;;
  google)
    api_json=${service_json_by_role[api]}
    worker_json=${service_json_by_role[worker]}
    [[ "$(service_env_value PYMES_GOOGLE_CALENDAR_ENABLED <<<"$api_json")" == "true" &&
       "$(service_env_value PYMES_GOOGLE_CALENDAR_ENABLED <<<"$worker_json")" == "true" ]] ||
      fail "Google Calendar is not enabled in both API and worker"
    google_redirect=$(service_env_value PYMES_GOOGLE_REDIRECT_URL <<<"$api_json")
    [[ "$google_redirect" == \
      "$public_base_url/api/v1/calendars/google/oauth/callback" ]] ||
      fail "Google callback is not the exact same-origin BFF callback"
    expected_calendar_kms="projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/calendar-tokens"
    [[ "$(service_env_value PYMES_CALENDAR_KMS_KEY <<<"$api_json")" == \
       "$expected_calendar_kms" &&
       "$(service_env_value PYMES_CALENDAR_KMS_KEY <<<"$worker_json")" == \
       "$expected_calendar_kms" ]] ||
      fail "Google token KMS does not match the environment-scoped key"
    feature=$(feature_probe "$PYMES_PILOT_ORGANIZATION_ID" \
      "$PYMES_PILOT_BEARER_TOKEN_FILE" google_calendar_enabled pilot)
    connections="$staging_dir/.connections"
    connections_headers="$staging_dir/.connections.headers"
    connections_status=$(http_get \
      "$(api_url "$PYMES_PILOT_ORGANIZATION_ID" calendars/connections)" \
      "$PYMES_PILOT_BEARER_TOKEN_FILE" "$connections" \
      "$connections_headers")
    assert_status "$connections_status" 200 "Google connection"
    jq -e --arg id "$PYMES_PILOT_GOOGLE_CONNECTION_ID" '
      [
        .[] |
        select(
          .id == $id and
          .provider == "google" and
          .status == "active" and
          .calendar_connected == true and
          .meet_enabled == true
        )
      ] | length == 1
    ' "$connections" >/dev/null ||
      fail "the expected active Google/Meet connection is absent"
    event_id=$(python3 - \
      "$PYMES_PILOT_ORGANIZATION_ID" \
      "$PYMES_PILOT_GOOGLE_CONNECTION_ID" \
      "$PYMES_PILOT_BOOKING_ID" <<'PY'
import base64
import hashlib
import sys

digest = hashlib.sha256()
for part in ("event", *sys.argv[1:]):
    digest.update(b"\0")
    digest.update(part.encode("utf-8"))
print(base64.b32hexencode(digest.digest()).decode("ascii").rstrip("=").lower())
PY
    )
    [[ "$event_id" =~ ^[0-9a-v]{52}$ ]] ||
      fail "could not derive the deterministic Google event ID"
    google_calendar=$(urlencode "$PYMES_PILOT_GOOGLE_CALENDAR_ID")
    google_event=$(urlencode "$event_id")
    google_body="$staging_dir/.google-event"
    google_headers="$staging_dir/.google-event.headers"
    google_status=$(http_get \
      "https://www.googleapis.com/calendar/v3/calendars/${google_calendar}/events/${google_event}" \
      "$PYMES_PILOT_GOOGLE_TOKEN_FILE" "$google_body" "$google_headers")
    assert_status "$google_status" 200 "Google provider event"
    jq -e --arg id "$event_id" '
      type == "object" and .id == $id and .status == "confirmed" and
      .conferenceData.conferenceSolution.key.type == "hangoutsMeet" and
      (
        [
          .conferenceData.entryPoints[]? |
          select(
            .entryPointType == "video" and
            (.uri | startswith("https://meet.google.com/"))
          )
        ] | length == 1
      )
    ' "$google_body" >/dev/null ||
      fail "Google event does not contain one confirmed Meet projection"
    connection=$(jq -c --arg id "$PYMES_PILOT_GOOGLE_CONNECTION_ID" '
      [.[] | select(.id == $id)][0] |
      {
        status,
        calendar_connected,
        time_zone,
        free_busy_enabled,
        meet_enabled,
        version
      }
    ' "$connections")
    google_safe=$(jq -c \
      --arg event_ref "$(sha_ref "$event_id")" \
      --arg etag_ref "$(sha_ref "$(jq -r '.etag // ""' "$google_body")")" '
      {
        event_ref: $event_ref,
        etag_ref: $etag_ref,
        status,
        meet_solution: .conferenceData.conferenceSolution.key.type,
        video_entry_points: (
          [
            .conferenceData.entryPoints[]? |
            select(.entryPointType == "video")
          ] | length
        ),
        created,
        updated
      }
    ' "$google_body")
    pilot_record=$(jq -cn \
      --argjson feature "$feature" \
      --argjson connection "$connection" \
      --argjson provider_event "$google_safe" \
      --arg connection_ref "$(sha_ref "$PYMES_PILOT_GOOGLE_CONNECTION_ID")" \
      --arg booking_ref "$(sha_ref "$PYMES_PILOT_BOOKING_ID")" '
      {
        kind: "google",
        result: "passed",
        feature: $feature,
        runtime: {
          enabled_in_api_and_worker: true,
          callback_is_same_origin: true,
          token_kms_is_environment_scoped: true
        },
        connection_ref: $connection_ref,
        connection: $connection,
        booking_ref: $booking_ref,
        provider_event: $provider_event
      }
    ')
    ;;
  arca)
    fiscal_json=${service_json_by_role[fiscal]}
    [[ "$(service_env_value FISCAL_ADAPTER_MODE <<<"$fiscal_json")" == "arca" ]] ||
      fail "Fiscal Adapter is not running in ARCA mode"
    expected_fiscal_kms="projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/fiscal-vault"
    [[ "$(service_env_value FISCAL_KMS_KEY_NAME <<<"$fiscal_json")" == \
       "$expected_fiscal_kms" ]] ||
      fail "Fiscal vault KMS does not match the environment-scoped key"
    [[ -n "$(service_env_value FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN <<<"$fiscal_json")" ]] ||
      fail "Fiscal Adapter lacks the homologation issuer policy"
    feature=$(feature_probe "$PYMES_PILOT_ORGANIZATION_ID" \
      "$PYMES_PILOT_BEARER_TOKEN_FILE" fiscal_real_enabled pilot)
    credential="$staging_dir/.credential"
    credential_headers="$staging_dir/.credential.headers"
    credential_status=$(http_get \
      "$(api_url "$PYMES_PILOT_ORGANIZATION_ID" \
        "fiscal/credentials/$(urlencode "$PYMES_PILOT_FISCAL_CREDENTIAL_ID")")" \
      "$PYMES_PILOT_BEARER_TOKEN_FILE" "$credential" "$credential_headers")
    assert_status "$credential_status" 200 "ARCA homologation credential"
    jq -e --arg id "$PYMES_PILOT_FISCAL_CREDENTIAL_ID" '
      type == "object" and .id == $id and
      .environment == "homologation" and .status == "ready" and
      (.certificate_fingerprint | type == "string" and length > 0) and
      (.certificate_expires_at | type == "string" and length > 0)
    ' "$credential" >/dev/null ||
      fail "fiscal credential is not ready for homologation"
    sale="$staging_dir/.sale"
    sale_headers="$staging_dir/.sale.headers"
    sale_status=$(http_get \
      "$(api_url "$PYMES_PILOT_ORGANIZATION_ID" \
        "sales/$(urlencode "$PYMES_PILOT_SALE_ID")")" \
      "$PYMES_PILOT_BEARER_TOKEN_FILE" "$sale" "$sale_headers")
    assert_status "$sale_status" 200 "ARCA homologated sale"
    jq -e --arg id "$PYMES_PILOT_SALE_ID" '
      type == "object" and .id == $id and
      .fiscal_environment == "homologation" and
      (.status | IN("posted","partially_paid","paid")) and
      (.cae | type == "string" and test("^[0-9]{14}$")) and
      (.journal_entry_id | type == "string" and length > 0) and
      (.voucher.point_of_sale | type == "number" and . > 0) and
      (.voucher.voucher_number | type == "number" and . > 0)
    ' "$sale" >/dev/null ||
      fail "sale has not converged from ARCA authorization to one accounting entry"
    credential_safe=$(jq -c \
      --arg credential_ref "$(sha_ref "$PYMES_PILOT_FISCAL_CREDENTIAL_ID")" '
      {
        credential_ref: $credential_ref,
        environment,
        status,
        version,
        fingerprint_ref: (
          .certificate_fingerprint |
          @base64 |
          "base64-sha-input:" + .
        ),
        certificate_valid_from,
        certificate_expires_at
      }
    ' "$credential")
    fingerprint=$(jq -r '.certificate_fingerprint' "$credential")
    credential_safe=$(jq -c \
      --arg fingerprint_ref "$(sha_ref "$fingerprint")" '
      .fingerprint_ref = $fingerprint_ref
    ' <<<"$credential_safe")
    sale_safe=$(jq -c \
      --arg sale_ref "$(sha_ref "$PYMES_PILOT_SALE_ID")" \
      --arg cae_ref "$(sha_ref "$(jq -r '.cae' "$sale")")" \
      --arg journal_ref "$(sha_ref "$(jq -r '.journal_entry_id' "$sale")")" '
      {
        sale_ref: $sale_ref,
        fiscal_environment,
        status,
        voucher: .voucher,
        cae_ref: $cae_ref,
        journal_entry_ref: $journal_ref,
        snapshot_digest
      }
    ' "$sale")
    pilot_record=$(jq -cn \
      --argjson feature "$feature" \
      --argjson credential "$credential_safe" \
      --argjson sale "$sale_safe" '
      {
        kind: "arca",
        result: "passed",
        feature: $feature,
        runtime: {
          adapter_mode: "arca",
          vault_kms_is_environment_scoped: true,
          homologation_issuer_policy_present: true
        },
        credential: $credential,
        sale: $sale,
        evidence_boundary: {
          authorization_and_posting_observed_through_bff: true,
          exact_consultation_history_exposed_by_bff: false,
          note: "The bundle does not claim an ARCA exact-consultation history that the public API cannot independently expose."
        }
      }
    ')
    ;;
esac

observed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
collector_sha256=$(sha256sum "$0" | awk '{print $1}')
printf '%s\n' "$pilot_record" | jq -S . >"$staging_dir/pilot.json"
chmod 600 "$staging_dir/pilot.json"
pilot_sha256=$(sha256sum "$staging_dir/pilot.json" | awk '{print $1}')
cloud_records=$(jq -cS . <<<"$cloud_records")
manifest_record=$(jq -cnS \
  --arg environment "$environment" \
  --arg kind "$pilot_kind" \
  --arg observed_at "$observed_at" \
  --arg source_sha "$source_sha" \
  --arg release_manifest_sha256 "$release_manifest_sha256" \
  --arg collector_sha256 "$collector_sha256" \
  --arg pilot_sha256 "$pilot_sha256" \
  --arg project "$project" \
  --arg region "$region" \
  --arg public_origin "$public_base_url" \
  --arg release_marker "$actual_release_marker" \
  --argjson cloud_run "$cloud_records" '
  {
    schema_version: "pymes.pilot-evidence.v1",
    result: "passed",
    environment: $environment,
    pilot_kind: $kind,
    observed_at: $observed_at,
    source_sha: $source_sha,
    release_manifest_sha256: $release_manifest_sha256,
    collector_sha256: $collector_sha256,
    pilot_sha256: $pilot_sha256,
    target: {
      project: $project,
      region: $region,
      public_origin: $public_origin,
      release_marker: $release_marker
    },
    cloud_run: $cloud_run,
    redaction: {
      raw_http_responses_retained: false,
      bearer_tokens_retained: false,
      organization_ids_retained: false,
      customer_pii_retained: false,
      provider_payloads_retained: false
    }
  }
  ')
printf '%s\n' "$manifest_record" >"$staging_dir/manifest.json"
chmod 600 "$staging_dir/manifest.json"

cat >"$staging_dir/README.txt" <<'EOF'
Pymes v3 controlled-pilot evidence bundle.

manifest.json binds the observation to the exact source SHA, release-manifest
checksum, active Cloud Run revisions and collector checksum. pilot.json contains
only the integration-specific redacted assertions. checksums.sha256 detects any
later modification. Raw API/provider responses and bearer tokens are not kept.

This bundle proves only the claims represented in pilot.json. It is not evidence
for a provider behavior that the queried public interfaces cannot expose.
EOF
chmod 600 "$staging_dir/README.txt"

(
  cd "$staging_dir"
  sha256sum README.txt manifest.json pilot.json >checksums.sha256
  chmod 600 checksums.sha256
  sha256sum --check checksums.sha256 >/dev/null
)

# Remove every raw or credential-bearing staging artifact before publication.
find "$staging_dir" -maxdepth 1 -type f -name '.*' -delete
[[ "$(find "$staging_dir" -mindepth 1 -maxdepth 1 -type f | wc -l)" == "4" ]] ||
  fail "unexpected files remain in the evidence bundle"
if grep -ERa -n \
  'Authorization: Bearer|BEGIN (RSA )?PRIVATE KEY|BEGIN CERTIFICATE|csr_pem|customer_(name|email|phone)|recipient_ref' \
  "$staging_dir" >/dev/null; then
  fail "the redacted evidence bundle contains forbidden sensitive material"
fi
(
  cd "$staging_dir"
  sha256sum --check checksums.sha256 >/dev/null
)
mv -- "$staging_dir" "$evidence_dir"
published=true
trap - EXIT
echo "PILOT EVIDENCE COMPLETE kind=$pilot_kind environment=$environment source_sha=$source_sha bundle=$evidence_dir"
