#!/usr/bin/env bash
set -euo pipefail

# Local-only deployment policy gate. It executes cloud-run.sh in dry-run mode for
# both shared-project environments and never invokes gcloud or resolves secrets.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
deploy_script="$script_dir/cloud-run.sh"
project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
scratch_dir=$(mktemp -d)
trap 'rm -rf -- "$scratch_dir"' EXIT
mkdir -p "$scratch_dir/bin"
printf '%s\n' '#!/usr/bin/env bash' 'echo "gcloud must not run during the deployment dry-run" >&2' 'exit 97' >"$scratch_dir/bin/gcloud"
chmod +x "$scratch_dir/bin/gcloud"
guarded_path="$scratch_dir/bin:$PATH"

fail() {
  echo "cloud-run security check failed: $*" >&2
  exit 1
}

require_text() {
  local file="$1" text="$2"
  grep -F -- "$text" "$file" >/dev/null ||
    fail "missing expected text in $(basename "$file"): $text"
}

forbid_text() {
  local file="$1" text="$2"
  if grep -F -- "$text" "$file" >/dev/null; then
    fail "unexpected text in $(basename "$file"): $text"
  fi
}

require_count() {
  local file="$1" text="$2" expected="$3" actual
  actual=$(grep -oF -- "$text" "$file" | wc -l)
  [[ "$actual" == "$expected" ]] ||
    fail "expected $expected occurrences of $text in $(basename "$file"), got $actual"
}

run_dry() {
  local environment="$1" output="$2" endpoint="${3:-}" pergo="${4:-false}"
  local prefix="pymes-v3-$environment"
  local kms_version="projects/$project/locations/$region/keyRings/$prefix/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/1"
  local -a common_environment=(
    "PYMES_CLOUD_RUN_DRY_RUN=true"
    "PYMES_DEPLOY_ENV=$environment"
    "PYMES_GCP_PROJECT=$project"
    "PYMES_GCP_REGION=$region"
    "PYMES_API_IMAGE=test.invalid/pymes-api:dry-run"
    "PYMES_WEB_IMAGE=test.invalid/pymes-web:dry-run"
    "PYMES_WORKER_IMAGE=test.invalid/pymes-worker:dry-run"
    "PYMES_FISCAL_IMAGE=test.invalid/pymes-fiscal:dry-run"
    "PYMES_ACCOUNTING_IMAGE=test.invalid/pymes-accounting:dry-run"
    "PYMES_ACCOUNTING_ADMIN_IMAGE=test.invalid/pymes-accounting-admin:dry-run"
    "PYMES_PROVISION_IMAGE=test.invalid/pymes-provision:dry-run"
    "PYMES_MIGRATE_IMAGE=test.invalid/pymes-migrate:dry-run"
    "PYMES_FISCAL_MIGRATE_IMAGE=test.invalid/pymes-fiscal-migrate:dry-run"
    "PYMES_ACCOUNTING_MIGRATE_IMAGE=test.invalid/pymes-accounting-migrate:dry-run"
    "PYMES_CLOUDSQL_INSTANCE=$project:$region:pymes-v3-dry-run"
    "PYMES_CLERK_ISSUER=https://clerk.dry-run.invalid"
    "PYMES_CLERK_AUTHORIZED_PARTIES=https://pymes.dry-run.invalid"
    "PYMES_INTERNAL_KMS_KEY_VERSION=$kms_version"
    'PYMES_INTERNAL_JWKS_JSON={"keys":[{"kid":"dry-run-public-key"}]}'
  )
  if [[ "$pergo" == "true" ]]; then
    common_environment+=(
      "PYMES_PERGO_ENABLED=true"
      "PYMES_PERGO_URL=https://pergo.$environment.dry-run.invalid"
      "PYMES_PERGO_WORKSPACE_ID=pymes-$environment"
      "PYMES_PERGO_CHANNEL=whatsapp_cloud"
    )
  fi

  if [[ -n "$endpoint" ]]; then
    env -u GOOGLE_APPLICATION_CREDENTIALS \
      "PATH=$guarded_path" \
      "${common_environment[@]}" \
      PYMES_TRACING_EXPORTER=otlp \
      "OTEL_EXPORTER_OTLP_ENDPOINT=$endpoint" \
      PYMES_TRACE_SAMPLE_RATIO=0.25 \
      "$deploy_script" >"$output"
  else
    env -u GOOGLE_APPLICATION_CREDENTIALS \
      -u PYMES_TRACING_EXPORTER \
      -u OTEL_EXPORTER_OTLP_ENDPOINT \
      -u PYMES_TRACE_SAMPLE_RATIO \
      "PATH=$guarded_path" \
      "${common_environment[@]}" \
      "$deploy_script" >"$output"
  fi
}

check_pergo_environment() {
  local environment="$1" output="$2"
  local prefix="pymes-v3-$environment"
  local api_line worker_line

  api_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-api " "$output") ||
    fail "missing PerGo-enabled API deploy command"
  worker_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-worker " "$output") ||
    fail "missing PerGo-enabled worker deploy command"

  [[ "$api_line" == *"PYMES_PERGO_ENABLED=true"* &&
     "$api_line" == *"PERGO_WORKSPACE_ID=pymes-$environment"* &&
     "$api_line" == *"PERGO_WEBHOOK_SECRETS=$prefix-pergo-webhook-secrets:DRY_RUN"* ]] ||
    fail "API is missing PerGo webhook configuration"
  [[ "$api_line" != *"PERGO_API_KEY="* &&
     "$api_line" != *"PERGO_URL="* ]] ||
    fail "API received PerGo delivery credentials"

  [[ "$worker_line" == *"PYMES_PERGO_ENABLED=true"* &&
     "$worker_line" == *"PERGO_URL=https://pergo.$environment.dry-run.invalid"* &&
     "$worker_line" == *"PERGO_WORKSPACE_ID=pymes-$environment"* &&
     "$worker_line" == *"PERGO_CHANNEL=whatsapp_cloud"* &&
     "$worker_line" == *"PERGO_API_KEY=$prefix-pergo-api-key:DRY_RUN"* ]] ||
    fail "worker is missing PerGo delivery configuration"
  [[ "$worker_line" != *"PERGO_WEBHOOK_SECRETS="* ]] ||
    fail "worker received PerGo webhook verification secret"

  require_text "$output" \
    "configure PerGo workspace=pymes-$environment callback=https://$prefix-api.$region.run.internal.invalid/api/v1/webhooks/pergo"
}

check_environment() {
  local environment="$1" output="$2" traced_output="$3"
  local prefix="pymes-v3-$environment"
  local worker_principal="serviceAccount:pymes-v3-worker-$environment@$project.iam.gserviceaccount.com"
  local provision_principal="serviceAccount:pymes-v3-provision-$environment@$project.iam.gserviceaccount.com"
  local service deploy_line

  require_text "$output" "TRACING status=pending exporter=none endpoint=unset"
  forbid_text "$output" "PYMES_TRACING_EXPORTER="
  forbid_text "$output" "OTEL_EXPORTER_OTLP_ENDPOINT="
  forbid_text "$output" "PYMES_TRACE_SAMPLE_RATIO="
  forbid_text "$output" "allUsers"
  forbid_text "$output" "allAuthenticatedUsers"
  require_text "$output" "SECURITY PROJECT project=$project direct_roles/run.invoker=none"

  for service in "$prefix-fiscal" "$prefix-accounting" "$prefix-accounting-admin"; do
    deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $service " "$output") ||
      fail "missing deploy command for $service"
    [[ "$deploy_line" == *"--ingress=internal"* ]] ||
      fail "$service is not internal-only"
    [[ "$deploy_line" == *"--no-allow-unauthenticated"* ]] ||
      fail "$service permits unauthenticated invocation"
    [[ "$deploy_line" != *" --allow-unauthenticated"* ]] ||
      fail "$service contains a public access flag"
  done

  require_text "$output" "SECURITY SERVICE service=$prefix-fiscal ingress=internal unauthenticated=denied sole_invoker=$worker_principal"
  require_text "$output" "SECURITY SERVICE service=$prefix-accounting ingress=internal unauthenticated=denied sole_invoker=$worker_principal"
  require_text "$output" "SECURITY SERVICE service=$prefix-accounting-admin ingress=internal unauthenticated=denied sole_invoker=$provision_principal"

  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-api " "$output") ||
    fail "missing deploy command for $prefix-api"
  [[ "$deploy_line" == *"--ingress=all"* && "$deploy_line" == *"--allow-unauthenticated"* ]] ||
    fail "the BFF is not publicly reachable"
  [[ "$deploy_line" != *"FISCAL_ADAPTER_URL="* && "$deploy_line" != *"ACCOUNTING_URL="* && "$deploy_line" != *"ACCOUNTING_PROVISIONING_URL="* ]] ||
    fail "the public BFF received a private downstream URL"

  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-web " "$output") ||
    fail "missing deploy command for $prefix-web"
  [[ "$deploy_line" == *"--ingress=all"* && "$deploy_line" == *"--allow-unauthenticated"* ]] ||
    fail "the static web is not publicly reachable"
  [[ "$deploy_line" == *"--min=0"* && "$deploy_line" == *"--cpu-throttling"* ]] ||
    fail "the static web does not scale to zero"
  [[ "$deploy_line" != *"--set-cloudsql-instances="* && "$deploy_line" != *"--set-secrets="* ]] ||
    fail "the static web received database or secret access"
  [[ "$deploy_line" != *"FISCAL_ADAPTER_URL="* && "$deploy_line" != *"ACCOUNTING_URL="* && "$deploy_line" != *"ACCOUNTING_PROVISIONING_URL="* ]] ||
    fail "the static web received a private downstream URL"

  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-worker " "$output") ||
    fail "missing deploy command for $prefix-worker"
  [[ "$deploy_line" == *"FISCAL_ADAPTER_URL=https://$prefix-fiscal.$region.run.internal.invalid"* ]] ||
    fail "worker is missing the fiscal private URL"
  [[ "$deploy_line" == *"ACCOUNTING_URL=https://$prefix-accounting.$region.run.internal.invalid"* ]] ||
    fail "worker is missing the accounting private URL"

  deploy_line=$(grep -F -- "DRY-RUN gcloud run jobs deploy $prefix-provision-org " "$output") ||
    fail "missing provision job command"
  [[ "$deploy_line" == *"ACCOUNTING_PROVISIONING_URL=https://$prefix-accounting-admin.$region.run.internal.invalid"* ]] ||
    fail "provisioner is missing the accounting-admin private URL"

  require_text "$traced_output" "TRACING status=configured exporter=otlp endpoint=explicit sample_ratio=0.25"
  require_count "$traced_output" "PYMES_TRACING_EXPORTER=otlp" 2
  require_count "$traced_output" "OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-collector.$environment.dry-run.invalid:4318" 2
  require_count "$traced_output" "PYMES_TRACE_SAMPLE_RATIO=0.25" 2
}

for environment in stg prd; do
  output="$scratch_dir/$environment.out"
  traced_output="$scratch_dir/$environment-traced.out"
  pergo_output="$scratch_dir/$environment-pergo.out"
  run_dry "$environment" "$output"
  run_dry "$environment" "$traced_output" "https://otel-collector.$environment.dry-run.invalid:4318"
  run_dry "$environment" "$pergo_output" "" true
  check_environment "$environment" "$output" "$traced_output"
  check_pergo_environment "$environment" "$pergo_output"
  echo "PASS cloud-run security dry-run environment=$environment resources_created=0"
done

if run_dry stg "$scratch_dir/credential-endpoint.out" \
  "https://embedded-token@otel-collector.stg.dry-run.invalid:4318" \
  2>"$scratch_dir/credential-endpoint.err"; then
  fail "an OTLP endpoint with embedded credentials was accepted"
fi
require_text "$scratch_dir/credential-endpoint.err" \
  "OTEL_EXPORTER_OTLP_ENDPOINT must be an explicit endpoint without credentials"
echo "PASS cloud-run tracing endpoint credentials=rejected"
