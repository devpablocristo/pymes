#!/usr/bin/env bash
set -euo pipefail

# Deploys Pymes v3 into the shared GCP project. It deliberately refuses to
# start incomplete services: all referenced Cloud Run-compatible secrets must have an
# enabled version first. Fiscal remains in mock mode until the ARCA phase.

: "${PYMES_DEPLOY_ENV:?set PYMES_DEPLOY_ENV to stg or prd}"
case "$PYMES_DEPLOY_ENV" in stg|prd) ;; *) echo "PYMES_DEPLOY_ENV must be stg or prd" >&2; exit 2 ;; esac

: "${PYMES_API_IMAGE:?set PYMES_API_IMAGE}"
: "${PYMES_WEB_IMAGE:?set PYMES_WEB_IMAGE (built with the target environment API URL and Clerk publishable key)}"
: "${PYMES_WORKER_IMAGE:?set PYMES_WORKER_IMAGE}"
: "${PYMES_FISCAL_IMAGE:?set PYMES_FISCAL_IMAGE}"
: "${PYMES_ACCOUNTING_IMAGE:?set PYMES_ACCOUNTING_IMAGE}"
: "${PYMES_ACCOUNTING_ADMIN_IMAGE:?set PYMES_ACCOUNTING_ADMIN_IMAGE}"
: "${PYMES_PROVISION_IMAGE:?set PYMES_PROVISION_IMAGE}"
: "${PYMES_MIGRATE_IMAGE:?set PYMES_MIGRATE_IMAGE}"
: "${PYMES_FISCAL_MIGRATE_IMAGE:?set PYMES_FISCAL_MIGRATE_IMAGE}"
: "${PYMES_ACCOUNTING_MIGRATE_IMAGE:?set PYMES_ACCOUNTING_MIGRATE_IMAGE}"
: "${PYMES_CLOUDSQL_INSTANCE:?set PYMES_CLOUDSQL_INSTANCE (project:region:instance)}"
: "${PYMES_CLERK_ISSUER:?set PYMES_CLERK_ISSUER}"
: "${PYMES_CLERK_AUTHORIZED_PARTIES:?set PYMES_CLERK_AUTHORIZED_PARTIES}"
: "${PYMES_INTERNAL_KMS_KEY_VERSION:?set PYMES_INTERNAL_KMS_KEY_VERSION to an explicit EC_SIGN_ED25519 CryptoKeyVersion resource}"

dry_run=${PYMES_CLOUD_RUN_DRY_RUN:-false}
case "$dry_run" in
  true|false) ;;
  *) echo "PYMES_CLOUD_RUN_DRY_RUN must be true or false" >&2; exit 2 ;;
esac

pergo_enabled=${PYMES_PERGO_ENABLED:-false}
case "$pergo_enabled" in
  true|false) ;;
  *) echo "PYMES_PERGO_ENABLED must be true or false" >&2; exit 2 ;;
esac
pergo_url=${PYMES_PERGO_URL:-}
pergo_workspace_id=${PYMES_PERGO_WORKSPACE_ID:-}
pergo_channel=${PYMES_PERGO_CHANNEL:-whatsapp}
if [[ "$pergo_enabled" == "true" ]]; then
  : "${pergo_url:?set PYMES_PERGO_URL when PerGo is enabled}"
  : "${pergo_workspace_id:?set PYMES_PERGO_WORKSPACE_ID when PerGo is enabled}"
  case "$pergo_channel" in
    whatsapp|whatsapp_cloud) ;;
    *) echo "PYMES_PERGO_CHANNEL must be whatsapp or whatsapp_cloud" >&2; exit 2 ;;
  esac
  if [[ "$pergo_url" != https://* ||
        "$pergo_url" == *"|"* ||
        "$pergo_url" == *","* ||
        "$pergo_url" == *"@"* ||
        "$pergo_url" == *"?"* ||
        "$pergo_url" == *"#"* ]]; then
    echo "PYMES_PERGO_URL must be an explicit HTTPS URL without credentials, query, fragment or Cloud Run delimiters" >&2
    exit 2
  fi
fi

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
prefix="pymes-v3-${PYMES_DEPLOY_ENV}"
export CLOUDSDK_CORE_PROJECT="$project"
export PYMES_INTERNAL_KMS_KEY_VERSION
export PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS="${PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS:-}"
google_calendar_enabled=${PYMES_GOOGLE_CALENDAR_ENABLED:-false}
api_sa="pymes-v3-api-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
web_sa="pymes-v3-web-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
worker_sa="pymes-v3-worker-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
provision_sa="pymes-v3-provision-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
fiscal_sa="pymes-v3-fiscal-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
accounting_sa="pymes-v3-accounting-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
accounting_admin_sa="pymes-v3-accounting-admin-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
migrate_sa="pymes-v3-migrate-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
fiscal_migrate_sa="pymes-v3-fiscal-migrate-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
accounting_migrate_sa="pymes-v3-acct-migrate-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"

case "$google_calendar_enabled" in
  true|false) ;;
  *) echo "PYMES_GOOGLE_CALENDAR_ENABLED must be true or false" >&2; exit 2 ;;
esac
calendar_environment="|PYMES_GOOGLE_CALENDAR_ENABLED=false"
google_client_secret_name=
if [[ "$google_calendar_enabled" == "true" ]]; then
  : "${PYMES_GOOGLE_CLIENT_ID:?set the environment-specific Google OAuth client ID}"
  : "${PYMES_GOOGLE_REDIRECT_URL:?set the single global BFF Google OAuth callback}"
  : "${PYMES_CALENDAR_KMS_KEY:?set the environment-specific Calendar token CryptoKey}"
  google_redirect_pattern='^https://[^/@|?#]+/api/v1/calendars/google/oauth/callback$'
  if [[ ! "$PYMES_GOOGLE_REDIRECT_URL" =~ $google_redirect_pattern ]]; then
    echo "PYMES_GOOGLE_REDIRECT_URL must be the global HTTPS BFF callback /api/v1/calendars/google/oauth/callback" >&2
    exit 2
  fi
  for value in "$PYMES_GOOGLE_CLIENT_ID" "$PYMES_GOOGLE_REDIRECT_URL" "$PYMES_CALENDAR_KMS_KEY"; do
    if [[ "$value" == *"|"* || "$value" == *$'\n'* ]]; then
      echo "Google Calendar environment values must not contain Cloud Run delimiters or newlines" >&2
      exit 2
    fi
  done
  expected_calendar_kms_key="projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/calendar-tokens"
  if [[ "$PYMES_CALENDAR_KMS_KEY" != "$expected_calendar_kms_key" ]]; then
    echo "PYMES_CALENDAR_KMS_KEY must be $expected_calendar_kms_key" >&2
    exit 2
  fi
  google_client_secret_name="$prefix-google-client-secret"
  calendar_environment="|PYMES_GOOGLE_CALENDAR_ENABLED=true|PYMES_GOOGLE_CLIENT_ID=$PYMES_GOOGLE_CLIENT_ID|PYMES_GOOGLE_REDIRECT_URL=$PYMES_GOOGLE_REDIRECT_URL|PYMES_CALENDAR_KMS_KEY=$PYMES_CALENDAR_KMS_KEY"
  echo "GOOGLE_CALENDAR status=enabled callback=global kms=environment-scoped"
else
  for variable in PYMES_GOOGLE_CLIENT_ID PYMES_GOOGLE_REDIRECT_URL PYMES_CALENDAR_KMS_KEY; do
    if [[ -n "${!variable:-}" ]]; then
      echo "$variable requires PYMES_GOOGLE_CALENDAR_ENABLED=true" >&2
      exit 2
    fi
  done
  echo "GOOGLE_CALENDAR status=disabled callback=unset"
fi

version_pattern="^projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/[1-9][0-9]*$"
if [[ ! "$PYMES_INTERNAL_KMS_KEY_VERSION" =~ $version_pattern ]]; then
  echo "PYMES_INTERNAL_KMS_KEY_VERSION must pin a numeric version in the ${prefix} key ring" >&2
  exit 2
fi
IFS=',' read -r -a overlap_versions <<<"$PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS"
for version in "${overlap_versions[@]}"; do
  [[ -z "$version" ]] && continue
  if [[ ! "$version" =~ $version_pattern ]]; then
    echo "every PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS entry must pin a numeric version in the ${prefix} key ring" >&2
    exit 2
  fi
done

tracing_environment=
tracing_endpoint=${OTEL_EXPORTER_OTLP_ENDPOINT:-}
if [[ -n "$tracing_endpoint" ]]; then
  tracing_exporter=${PYMES_TRACING_EXPORTER:-otlp}
  tracing_sample_ratio=${PYMES_TRACE_SAMPLE_RATIO:-0.1}
  if [[ "$tracing_exporter" != "otlp" ]]; then
    echo "PYMES_TRACING_EXPORTER must be otlp when OTEL_EXPORTER_OTLP_ENDPOINT is set" >&2
    exit 2
  fi
  if [[ "$tracing_endpoint" == *"|"* || "$tracing_endpoint" == *$'\n'* || "$tracing_endpoint" == *"@"* || "$tracing_endpoint" == *"?"* || "$tracing_endpoint" == *"#"* ]]; then
    echo "OTEL_EXPORTER_OTLP_ENDPOINT must be an explicit endpoint without credentials, query, fragment or Cloud Run env delimiters" >&2
    exit 2
  fi
  if ! awk -v ratio="$tracing_sample_ratio" 'BEGIN { exit !(ratio ~ /^[0-9]+([.][0-9]+)?$/ && ratio > 0 && ratio <= 1) }'; then
    echo "PYMES_TRACE_SAMPLE_RATIO must be greater than zero and at most one" >&2
    exit 2
  fi
  tracing_environment="|PYMES_TRACING_EXPORTER=otlp|OTEL_EXPORTER_OTLP_ENDPOINT=$tracing_endpoint|PYMES_TRACE_SAMPLE_RATIO=$tracing_sample_ratio"
  echo "TRACING status=configured exporter=otlp endpoint=explicit sample_ratio=$tracing_sample_ratio"
else
  echo "TRACING status=pending exporter=none endpoint=unset"
fi

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
if [[ "$dry_run" == "true" ]]; then
  : "${PYMES_INTERNAL_JWKS_JSON:?set PYMES_INTERNAL_JWKS_JSON for dry-run validation}"
  resolved_internal_jwks=$PYMES_INTERNAL_JWKS_JSON
else
  resolved_internal_jwks=$(cd "$script_dir/../../backend" && go run ./cmd/internal-jwks)
fi
if [[ -n "${PYMES_INTERNAL_JWKS_JSON:-}" && "$PYMES_INTERNAL_JWKS_JSON" != "$resolved_internal_jwks" ]]; then
  echo "PYMES_INTERNAL_JWKS_JSON does not match the selected KMS key versions" >&2
  exit 2
fi
PYMES_INTERNAL_JWKS_JSON=$resolved_internal_jwks
case "$PYMES_INTERNAL_JWKS_JSON" in
  *'|'*) echo "PYMES_INTERNAL_JWKS_JSON must not contain the Cloud Run env delimiter |" >&2; exit 2 ;;
esac

gcloud_command() {
  if [[ "$dry_run" == "true" ]]; then
    printf 'DRY-RUN'
    printf ' %q' gcloud "$@"
    printf '\n'
    return
  fi
  gcloud "$@"
}

service_url() {
  local service="$1"
  if [[ "$dry_run" == "true" ]]; then
    printf 'https://%s.%s.run.internal.invalid' "$service" "$region"
    return
  fi
  gcloud run services describe "$service" --region="$region" --format='value(status.url)'
}

ensure_service_invoker() {
  local service="$1" expected_principal="$2"
  echo "SECURITY IAM service=$service required_invoker=$expected_principal"
  if [[ "$dry_run" == "true" ]]; then
    gcloud_command run services add-iam-policy-binding "$service" \
      --region="$region" --member="$expected_principal" \
      --role=roles/run.invoker --quiet
    return
  fi
  gcloud_command run services add-iam-policy-binding "$service" \
    --region="$region" --member="$expected_principal" \
    --role=roles/run.invoker --quiet >/dev/null
}

verify_no_project_invokers() {
  local invokers
  if [[ "$dry_run" == "true" ]]; then
    echo "SECURITY PROJECT project=$project direct_roles/run.invoker=none"
    return
  fi
  invokers=$(gcloud projects get-iam-policy "$project" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/run.invoker' \
    --format='value(bindings.members)' | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ -n "$invokers" ]]; then
    echo "shared project $project must not grant roles/run.invoker at project scope; got: $invokers" >&2
    echo "move each reviewed caller to the target Cloud Run service policy, then rerun this deployment" >&2
    exit 1
  fi
  echo "SECURITY PROJECT project=$project direct_roles/run.invoker=none"
}

verify_private_service() {
  local service="$1" expected_principal="$2" ingress invokers
  if [[ "$dry_run" == "true" ]]; then
    echo "SECURITY SERVICE service=$service ingress=internal unauthenticated=denied sole_invoker=$expected_principal"
    return
  fi

  ingress=$(gcloud run services describe "$service" --region="$region" \
    --format='value(metadata.annotations."run.googleapis.com/ingress")')
  if [[ "$ingress" != "internal" ]]; then
    echo "private service $service has unexpected ingress: ${ingress:-unset}" >&2
    exit 1
  fi
  invokers=$(gcloud run services get-iam-policy "$service" --region="$region" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/run.invoker' \
    --format='value(bindings.members)' | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ "$invokers" != "$expected_principal" ]]; then
    echo "private service $service must have exactly one roles/run.invoker member: $expected_principal; got: ${invokers:-none}" >&2
    echo "remove unexpected invokers explicitly after reviewing their owners, then rerun this deployment" >&2
    exit 1
  fi
  echo "SECURITY SERVICE service=$service ingress=internal unauthenticated=denied sole_invoker=$expected_principal"
}

require_secret() {
  local secret="$1"
  local metadata version state
  if [[ "$dry_run" == "true" ]]; then
    secret_versions["$secret"]="DRY_RUN"
    return
  fi
  metadata=$(gcloud_command secrets versions describe latest --secret="$secret" --format='value(name.basename(),state)' 2>/dev/null || true)
  read -r version state <<<"$metadata"
  if [[ -z "$version" || "$state" != "ENABLED" ]]; then
    echo "required secret has no enabled version: $secret" >&2
    exit 1
  fi
  secret_versions["$secret"]="$version"
}

declare -A secret_versions=()
required_secrets=(
  "$prefix-clerk-secret-key" "$prefix-clerk-webhook-secret" \
  "$prefix-database-url" "$prefix-worker-database-url" \
  "$prefix-migrate-database-url" "$prefix-fiscal-database-url" "$prefix-fiscal-migrate-database-url" \
  "$prefix-accounting-database-url" "$prefix-accounting-admin-database-url" \
  "$prefix-accounting-migrate-database-url"
)
if [[ "$pergo_enabled" == "true" ]]; then
  required_secrets+=(
    "$prefix-pergo-api-key"
    "$prefix-pergo-webhook-secrets"
  )
fi
for secret in "${required_secrets[@]}"; do
  require_secret "$secret"
done
if [[ -n "$google_client_secret_name" ]]; then
  require_secret "$google_client_secret_name"
fi

secret_ref() {
  printf '%s:%s' "$1" "${secret_versions[$1]}"
}

deploy() {
  local service="$1" image="$2" service_account="$3" secrets="$4" environment="$5" ingress="$6" min_instances="$7" access="$8" cpu="$9" network="${10}"
  local -a arguments=(
    --region="$region" --image="$image" --service-account="$service_account"
    --ingress="$ingress" --min="$min_instances" --max=1
    --set-cloudsql-instances="$PYMES_CLOUDSQL_INSTANCE"
    --set-secrets="$secrets" --set-env-vars="^|^$environment" --quiet
    --deploy-health-check
    --startup-probe=httpGet.path=/readyz,httpGet.port=8080,initialDelaySeconds=0,timeoutSeconds=2,periodSeconds=5,failureThreshold=12
    --readiness-probe=httpGet.path=/readyz,httpGet.port=8080,timeoutSeconds=2,periodSeconds=5,failureThreshold=3,successThreshold=1
    --liveness-probe=httpGet.path=/healthz,httpGet.port=8080,initialDelaySeconds=5,timeoutSeconds=2,periodSeconds=30,failureThreshold=3
  )
  if [[ "$access" == "public" ]]; then
    arguments+=(--allow-unauthenticated)
  else
    arguments+=(--no-allow-unauthenticated)
  fi
  if [[ "$cpu" == "always" ]]; then
    arguments+=(--no-cpu-throttling)
  else
    arguments+=(--cpu-throttling)
  fi
  if [[ "$network" == "direct" ]]; then
    arguments+=(--network=default --subnet=default --vpc-egress=all-traffic)
  fi
  gcloud_command run deploy "$service" "${arguments[@]}"
}

deploy_web() {
  local service="$1" image="$2" service_account="$3"
  gcloud_command run deploy "$service" \
    --region="$region" --image="$image" --service-account="$service_account" \
    --ingress=all --min=0 --max=1 --cpu-throttling \
    --allow-unauthenticated --quiet \
    --deploy-health-check \
    --startup-probe=httpGet.path=/readyz,httpGet.port=8080,initialDelaySeconds=0,timeoutSeconds=2,periodSeconds=5,failureThreshold=12 \
    --readiness-probe=httpGet.path=/readyz,httpGet.port=8080,timeoutSeconds=2,periodSeconds=5,failureThreshold=3,successThreshold=1 \
    --liveness-probe=httpGet.path=/healthz,httpGet.port=8080,initialDelaySeconds=5,timeoutSeconds=2,periodSeconds=30,failureThreshold=3
}

migrate() {
  local job="$1" image="$2" service_account="$3" secrets="$4"
  gcloud_command run jobs deploy "$job" \
    --region="$region" --image="$image" --service-account="$service_account" \
    --set-cloudsql-instances="$PYMES_CLOUDSQL_INSTANCE" --set-secrets="$secrets" \
    --tasks=1 --max-retries=0 --execute-now --wait --quiet
}

run_job() {
  local job="$1" image="$2" service_account="$3" secrets="$4" environment="$5"
  gcloud_command run jobs deploy "$job" \
    --region="$region" --image="$image" --service-account="$service_account" \
    --set-cloudsql-instances="$PYMES_CLOUDSQL_INSTANCE" --set-secrets="$secrets" \
    --set-env-vars="^|^$environment" \
    --tasks=1 --max-retries=0 --execute-now --wait --quiet
}

deploy_job_template() {
  local job="$1" image="$2" service_account="$3" secrets="$4" environment="$5"
  gcloud_command run jobs deploy "$job" \
    --region="$region" --image="$image" --service-account="$service_account" \
    --set-cloudsql-instances="$PYMES_CLOUDSQL_INSTANCE" --set-secrets="$secrets" \
    --set-env-vars="^|^$environment" \
    --network=default --subnet=default --vpc-egress=all-traffic \
    --tasks=1 --max-retries=0 --quiet
}

migrate "$prefix-migrate" "$PYMES_MIGRATE_IMAGE" "$migrate_sa" \
  "PYMES_DATABASE_URL=$(secret_ref "$prefix-migrate-database-url")"
migrate "$prefix-fiscal-migrate" "$PYMES_FISCAL_MIGRATE_IMAGE" "$fiscal_migrate_sa" \
  "FISCAL_DATABASE_URL=$(secret_ref "$prefix-fiscal-migrate-database-url")"
migrate "$prefix-accounting-migrate" "$PYMES_ACCOUNTING_MIGRATE_IMAGE" "$accounting_migrate_sa" \
  "DATABASE_URL=$(secret_ref "$prefix-accounting-migrate-database-url")"
run_job "$prefix-accounting-grants" "$PYMES_ACCOUNTING_ADMIN_IMAGE" \
  "$accounting_admin_sa" \
  "ACCOUNTING_ADMIN_DATABASE_URL=$(secret_ref "$prefix-accounting-admin-database-url")" \
  "ACCOUNTING_ADMIN_OPERATION=sync-runtime-grants|ACCOUNTING_RUNTIME_ROLE=pymes_v3_accounting_${PYMES_DEPLOY_ENV}|ACCOUNTING_OWNER_ROLE=pymes_v3_accounting_owner_${PYMES_DEPLOY_ENV}"

fiscal_service="$prefix-fiscal"
accounting_service="$prefix-accounting"
accounting_admin_service="$prefix-accounting-admin"
deploy "$fiscal_service" "$PYMES_FISCAL_IMAGE" "$fiscal_sa" \
  "FISCAL_DATABASE_URL=$(secret_ref "$prefix-fiscal-database-url")" \
  "FISCAL_ADAPTER_MODE=mock|FISCAL_MOCK_SCENARIO=authorized|PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_JWKS_JSON=$PYMES_INTERNAL_JWKS_JSON|PORT=8080" internal 0 private throttled none
deploy "$accounting_service" "$PYMES_ACCOUNTING_IMAGE" "$accounting_sa" \
  "ACCOUNTING_DATABASE_URL=$(secret_ref "$prefix-accounting-database-url")" \
  "PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_JWKS_JSON=$PYMES_INTERNAL_JWKS_JSON|PORT=8080" internal 0 private throttled none
deploy "$accounting_admin_service" "$PYMES_ACCOUNTING_ADMIN_IMAGE" \
  "$accounting_admin_sa" \
  "ACCOUNTING_ADMIN_DATABASE_URL=$(secret_ref "$prefix-accounting-admin-database-url")" \
  "ACCOUNTING_ADMIN_OPERATION=serve|ACCOUNTING_RUNTIME_ROLE=pymes_v3_accounting_${PYMES_DEPLOY_ENV}|ACCOUNTING_OWNER_ROLE=pymes_v3_accounting_owner_${PYMES_DEPLOY_ENV}|PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_JWKS_JSON=$PYMES_INTERNAL_JWKS_JSON|PORT=8080" \
  internal 0 private throttled none

worker_principal="serviceAccount:$worker_sa"
provision_principal="serviceAccount:$provision_sa"
verify_no_project_invokers
ensure_service_invoker "$fiscal_service" "$worker_principal"
ensure_service_invoker "$accounting_service" "$worker_principal"
ensure_service_invoker "$accounting_admin_service" "$provision_principal"
verify_private_service "$fiscal_service" "$worker_principal"
verify_private_service "$accounting_service" "$worker_principal"
verify_private_service "$accounting_admin_service" "$provision_principal"

fiscal_url=$(service_url "$fiscal_service")
accounting_url=$(service_url "$accounting_service")
accounting_admin_url=$(service_url "$accounting_admin_service")
api_secrets="PYMES_CLERK_SECRET_KEY=$(secret_ref "$prefix-clerk-secret-key"),PYMES_CLERK_WEBHOOK_SECRET=$(secret_ref "$prefix-clerk-webhook-secret"),PYMES_DATABASE_URL=$(secret_ref "$prefix-database-url")"
worker_secrets="PYMES_DATABASE_URL=$(secret_ref "$prefix-worker-database-url")"
if [[ -n "$google_client_secret_name" ]]; then
  google_client_secret_ref=$(secret_ref "$google_client_secret_name")
  api_secrets="$api_secrets,PYMES_GOOGLE_CLIENT_SECRET=$google_client_secret_ref"
  worker_secrets="$worker_secrets,PYMES_GOOGLE_CLIENT_SECRET=$google_client_secret_ref"
fi
api_environment="PYMES_ENVIRONMENT=production|PYMES_CLERK_ISSUER=$PYMES_CLERK_ISSUER|PYMES_CLERK_AUDIENCE=pymes-v3|PYMES_CLERK_AUTHORIZED_PARTIES=$PYMES_CLERK_AUTHORIZED_PARTIES|PYMES_HTTP_ADDR=:8080${calendar_environment}${tracing_environment}"
worker_environment="FISCAL_ADAPTER_URL=$fiscal_url|ACCOUNTING_URL=$accounting_url|PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_KMS_KEY_VERSION=$PYMES_INTERNAL_KMS_KEY_VERSION|PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS=$PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS|PYMES_WORKER_HTTP_ADDR=:8080|PYMES_WORKER_INTERVAL_MS=250|PYMES_WORKER_METRICS_INTERVAL=60s${calendar_environment}${tracing_environment}"
if [[ "$pergo_enabled" == "true" ]]; then
  api_secrets+=",PERGO_WEBHOOK_SECRETS=$(secret_ref "$prefix-pergo-webhook-secrets")"
  api_environment+="|PYMES_PERGO_ENABLED=true|PERGO_WORKSPACE_ID=$pergo_workspace_id"
  worker_secrets+=",PERGO_API_KEY=$(secret_ref "$prefix-pergo-api-key")"
  worker_environment+="|PYMES_PERGO_ENABLED=true|PERGO_URL=$pergo_url|PERGO_WORKSPACE_ID=$pergo_workspace_id|PERGO_CHANNEL=$pergo_channel|PERGO_TIMEOUT=5s"
else
  api_environment+="|PYMES_PERGO_ENABLED=false"
  worker_environment+="|PYMES_PERGO_ENABLED=false"
fi
deploy "$prefix-api" "$PYMES_API_IMAGE" "$api_sa" \
  "$api_secrets" \
  "$api_environment" all 0 public throttled none
deploy_web "$prefix-web" "$PYMES_WEB_IMAGE" "$web_sa"
deploy "$prefix-worker" "$PYMES_WORKER_IMAGE" "$worker_sa" \
  "$worker_secrets" \
  "$worker_environment" internal 1 private always direct

deploy_job_template "$prefix-provision-org" "$PYMES_PROVISION_IMAGE" \
  "$provision_sa" \
  "PYMES_DATABASE_URL=$(secret_ref "$prefix-database-url")" \
  "ACCOUNTING_PROVISIONING_URL=$accounting_admin_url|PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_KMS_KEY_VERSION=$PYMES_INTERNAL_KMS_KEY_VERSION|PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS=$PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS"

echo "deployed $prefix in shared project $project; configure Clerk webhook for $(service_url "$prefix-api")/api/v1/webhooks/clerk"
if [[ "$pergo_enabled" == "true" ]]; then
  echo "configure PerGo workspace=$pergo_workspace_id callback=$(service_url "$prefix-api")/api/v1/webhooks/pergo"
fi
echo "web: $(service_url "$prefix-web") (must be present in Clerk authorized parties and callbacks)"
echo "organization provisioning job: $prefix-provision-org (execute with explicit --id, --name, --slug and --clerk-organization-id args)"
