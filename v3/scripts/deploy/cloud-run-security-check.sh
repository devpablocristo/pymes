#!/usr/bin/env bash
set -euo pipefail

# Local-only deployment policy gate. It executes cloud-run.sh in dry-run mode for
# both shared-project environments and never invokes gcloud or resolves secrets.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
deploy_script="$script_dir/cloud-run.sh"
verify_script="$script_dir/verify-cloud-run.sh"
candidate_tag_policy="$script_dir/release-candidate-tag.sh"
project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
scratch_dir=$(mktemp -d)
printf -v test_preflight_token '%064x' 1
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
  local environment="$1" output="$2" endpoint="${3:-}" google_enabled="${4:-false}" google_redirect_override="${5:-}" pergo="${6:-false}" fiscal_mode="${7:-mock}"
  local prefix="pymes-v3-$environment"
  local deploy_stage="${PYMES_TEST_DEPLOY_STAGE:-operational}"
  local test_dry_run="${PYMES_TEST_CLOUD_RUN_DRY_RUN:-true}"
  local webhook_lifecycle="${PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE:-}"
  local active_services="${PYMES_TEST_ACTIVE_SERVICES:-}"
  local kms_version="projects/$project/locations/$region/keyRings/$prefix/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/1"
  local calendar_kms_key="projects/$project/locations/$region/keyRings/$prefix/cryptoKeys/calendar-tokens"
  local public_base="${9:-}"
  if [[ -z "$public_base" ]]; then
    if [[ "$deploy_stage" == "bootstrap" ]]; then
      public_base=https://pymes-v3-stg-bootstrap.invalid
    else
      public_base="https://pymes.$environment.dry-run.invalid"
    fi
  fi
  local google_redirect="${google_redirect_override:-$public_base/api/v1/calendars/google/oauth/callback}"
  local digest
  digest=$(printf '%064d' 0)
  local api_image="${8:-test.invalid/pymes-api@sha256:$digest}"
  local authorized_parties="${10:-$public_base}"
  local release_sha="${11:-1111111111111111111111111111111111111111}"
  local subnet_cidr="${12:-10.120.0.0/24}"
  local arca_patterns="${13:-true}"
  local default_pergo_workspace
  case "$environment" in
    stg) default_pergo_workspace=11111111-1111-4111-8111-111111111111 ;;
    prd) default_pergo_workspace=22222222-2222-4222-8222-222222222222 ;;
  esac
  local pergo_workspace="${14:-$default_pergo_workspace}"
  local -a common_environment=(
    "PYMES_CLOUD_RUN_DRY_RUN=$test_dry_run"
    "PYMES_DEPLOY_ENV=$environment"
    "PYMES_DEPLOY_STAGE=$deploy_stage"
    "PYMES_GCP_PROJECT=$project"
    "PYMES_GCP_REGION=$region"
    "PYMES_API_IMAGE=$api_image"
    "PYMES_WEB_IMAGE=test.invalid/pymes-web@sha256:$digest"
    "PYMES_WORKER_IMAGE=test.invalid/pymes-worker@sha256:$digest"
    "PYMES_FISCAL_IMAGE=test.invalid/pymes-fiscal@sha256:$digest"
    "PYMES_ACCOUNTING_IMAGE=test.invalid/pymes-accounting@sha256:$digest"
    "PYMES_ACCOUNTING_ADMIN_IMAGE=test.invalid/pymes-accounting-admin@sha256:$digest"
    "PYMES_PROVISION_IMAGE=test.invalid/pymes-provision@sha256:$digest"
    "PYMES_MIGRATE_IMAGE=test.invalid/pymes-migrate@sha256:$digest"
    "PYMES_FISCAL_MIGRATE_IMAGE=test.invalid/pymes-fiscal-migrate@sha256:$digest"
    "PYMES_ACCOUNTING_MIGRATE_IMAGE=test.invalid/pymes-accounting-migrate@sha256:$digest"
    "PYMES_CLOUDSQL_INSTANCE=$project:$region:pymes-v3-dry-run"
    "PYMES_CLERK_ISSUER=https://clerk.dry-run.invalid"
    "PYMES_CLERK_AUTHORIZED_PARTIES=$authorized_parties"
    "PYMES_PUBLIC_BASE_URL=$public_base"
    "PYMES_RELEASE_SHA=$release_sha"
    "PYMES_PREFLIGHT_TOKEN=$test_preflight_token"
    "PYMES_INTERNAL_KMS_KEY_VERSION=$kms_version"
    "PYMES_GOOGLE_CALENDAR_ENABLED=$google_enabled"
    "PYMES_FISCAL_MODE=$fiscal_mode"
    "PYMES_VPC_NETWORK=default"
    "PYMES_VPC_SUBNET=pymes-v3-serverless"
    "PYMES_VPC_SUBNET_CIDR=$subnet_cidr"
    "PYMES_VPC_NAT_ROUTER=pymes-v3-serverless"
    "PYMES_VPC_NAT_NAME=pymes-v3-serverless"
    'PYMES_INTERNAL_JWKS_JSON={"keys":[{"kid":"dry-run-public-key"}]}'
  )
  if [[ -n "$webhook_lifecycle" ]]; then
    common_environment+=(
      "PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN=$webhook_lifecycle"
    )
  fi
  if [[ -n "$active_services" ]]; then
    common_environment+=(
      "PYMES_CLOUD_RUN_ACTIVE_SERVICES_DRY_RUN=$active_services"
    )
  fi
  if [[ "$fiscal_mode" == "arca" && "$arca_patterns" == "true" ]]; then
    common_environment+=(
      "PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN=ARCA Homologacion"
      "PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN=ARCA Produccion"
    )
  fi
  if [[ "$pergo" == "true" ]]; then
    common_environment+=(
      "PYMES_PERGO_ENABLED=true"
      "PYMES_PERGO_URL=https://pergo.$environment.dry-run.invalid"
      "PYMES_PERGO_WORKSPACE_ID=$pergo_workspace"
      "PYMES_PERGO_CHANNEL=whatsapp_cloud"
    )
  fi
  if [[ "$google_enabled" == "true" ]]; then
    common_environment+=(
      "PYMES_GOOGLE_CLIENT_ID=client-$environment.apps.googleusercontent.com"
      "PYMES_GOOGLE_REDIRECT_URL=$google_redirect"
      "PYMES_CALENDAR_KMS_KEY=$calendar_kms_key"
    )
  fi

  if [[ -n "$endpoint" ]]; then
    env -u GOOGLE_APPLICATION_CREDENTIALS \
      -u PYMES_DEPLOY_STAGE \
      -u PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN \
      -u PYMES_CLOUD_RUN_ACTIVE_SERVICES_DRY_RUN \
      -u PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN \
      -u PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN \
      -u PYMES_GOOGLE_CLIENT_ID \
      -u PYMES_GOOGLE_REDIRECT_URL \
      -u PYMES_CALENDAR_KMS_KEY \
      "PATH=$guarded_path" \
      "${common_environment[@]}" \
      PYMES_TRACING_EXPORTER=otlp \
      "OTEL_EXPORTER_OTLP_ENDPOINT=$endpoint" \
      PYMES_TRACE_SAMPLE_RATIO=0.25 \
      "$deploy_script" >"$output"
  else
    env -u GOOGLE_APPLICATION_CREDENTIALS \
      -u PYMES_DEPLOY_STAGE \
      -u PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN \
      -u PYMES_CLOUD_RUN_ACTIVE_SERVICES_DRY_RUN \
      -u PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN \
      -u PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN \
      -u PYMES_TRACING_EXPORTER \
      -u OTEL_EXPORTER_OTLP_ENDPOINT \
      -u PYMES_TRACE_SAMPLE_RATIO \
      -u PYMES_GOOGLE_CLIENT_ID \
      -u PYMES_GOOGLE_REDIRECT_URL \
      -u PYMES_CALENDAR_KMS_KEY \
      "PATH=$guarded_path" \
      "${common_environment[@]}" \
      "$deploy_script" >"$output"
  fi
}

check_pergo_environment() {
  local environment="$1" output="$2"
  local prefix="pymes-v3-$environment"
  local api_line worker_line workspace
  case "$environment" in
    stg) workspace=11111111-1111-4111-8111-111111111111 ;;
    prd) workspace=22222222-2222-4222-8222-222222222222 ;;
  esac

  api_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-api " "$output") ||
    fail "missing PerGo-enabled API deploy command"
  worker_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-worker " "$output") ||
    fail "missing PerGo-enabled worker deploy command"

  [[ "$api_line" == *"PYMES_PERGO_ENABLED=true"* &&
     "$api_line" == *"PERGO_WORKSPACE_ID=$workspace"* &&
     "$api_line" == *"PERGO_WEBHOOK_SECRETS=$prefix-pergo-webhook-secrets:DRY_RUN"* ]] ||
    fail "API is missing PerGo webhook configuration"
  [[ "$api_line" != *"PERGO_API_KEY="* &&
     "$api_line" != *"PERGO_URL="* ]] ||
    fail "API received PerGo delivery credentials"

  [[ "$worker_line" == *"PYMES_PERGO_ENABLED=true"* &&
     "$worker_line" == *"PERGO_URL=https://pergo.$environment.dry-run.invalid"* &&
     "$worker_line" == *"PERGO_WORKSPACE_ID=$workspace"* &&
     "$worker_line" == *"PERGO_CHANNEL=whatsapp_cloud"* &&
     "$worker_line" == *"PERGO_ALLOW_GLOBAL_ROUTE_FALLBACK=false"* &&
     "$worker_line" == *"PERGO_API_KEY=$prefix-pergo-api-key:DRY_RUN"* ]] ||
    fail "worker is missing PerGo delivery configuration"
  [[ "$worker_line" != *"PERGO_WEBHOOK_SECRETS="* ]] ||
    fail "worker received PerGo webhook verification secret"

  require_text "$output" \
    "configure PerGo workspace=$workspace callback=https://pymes.$environment.dry-run.invalid/api/v1/webhooks/pergo"
}

check_environment() {
  local environment="$1" output="$2" traced_output="$3"
  local prefix="pymes-v3-$environment"
  local api_principal="serviceAccount:pymes-v3-api-$environment@$project.iam.gserviceaccount.com"
  local worker_principal="serviceAccount:pymes-v3-worker-$environment@$project.iam.gserviceaccount.com"
  local provision_principal="serviceAccount:pymes-v3-provision-$environment@$project.iam.gserviceaccount.com"
  local service deploy_line expected_web_marker

  if grep -Fq -- "$test_preflight_token" "$output"; then
    fail "Cloud Run dry-run output exposed the preflight capability"
  fi

  require_text "$output" "TRACING status=pending exporter=none endpoint=unset"
  require_text "$output" "GOOGLE_CALENDAR status=disabled callback=unset"
  require_text "$output" "FISCAL mode=mock kms=environment-scoped"
  require_text "$output" "DEPLOY STAGE stage=operational environment=$environment promotion=enabled"
  require_text "$output" "SECURITY CLERK_WEBHOOK_SECRET secret=$prefix-clerk-webhook-secret lifecycle=unset stage=operational source=dry-run-simulation"
  require_text "$output" "SECURITY NETWORK network=default subnet=pymes-v3-serverless cidr=10.120.0.0/24 private_google_access=true public_nat=pymes-v3-serverless/pymes-v3-serverless vpc_egress=all-traffic"
  require_text "$output" "SECURITY KMS key=secrets primary=enabled rotation=90d direct_crypto_principals=serviceAccount:service-DRY_RUN@gcp-sa-secretmanager.iam.gserviceaccount.com inherited_crypto_principals=none"
  require_text "$output" "SECURITY KMS key=calendar-tokens primary=enabled rotation=90d direct_crypto_principals=serviceAccount:pymes-v3-api-$environment@$project.iam.gserviceaccount.com,serviceAccount:pymes-v3-worker-$environment@$project.iam.gserviceaccount.com inherited_crypto_principals=none"
  require_text "$output" "SECURITY KMS key=fiscal-vault primary=enabled rotation=90d direct_crypto_principals=serviceAccount:pymes-v3-fiscal-$environment@$project.iam.gserviceaccount.com inherited_crypto_principals=none"
  require_text "$output" "SECURITY KMS key=internal-jwt-signing versions=enabled-ed25519 direct_signers=api,worker,provisioner direct_public_viewers=api,worker,provisioner,deployer inherited_principals=none"
  require_count "$output" "--labels=app=pymes-v3\\,env=$environment\\,pymes-v3-release=1111111111111111111111111111111111111111" 11
  require_count "$output" "--command=" 11
  require_count "$output" "--args=" 11
  require_count "$output" "--clear-volumes" 11
  require_count "$output" "--clear-volume-mounts" 11
  require_count "$output" "--no-traffic" 6
  require_count "$output" "--tag=c-1111111111111111" 6
  require_count "$output" "--invoker-iam-check" 6
  require_count "$output" "DRY-RUN gcloud run services update-traffic" 12
  require_text "$output" \
    "DRY-RUN gcloud run services update-traffic $prefix-api --project=$project --region=$region --set-tags=c-1111111111111111=$prefix-api-dry-run-11111111 --quiet"
  require_count "$output" " --clear-tags --quiet" 5
  require_text "$output" \
    "RELEASE TAGS service=$prefix-api policy=current-api-only tag=c-1111111111111111 revision=$prefix-api-dry-run-11111111"
  for service in \
    "$prefix-fiscal" \
    "$prefix-accounting" \
    "$prefix-accounting-admin" \
    "$prefix-worker" \
    "$prefix-web"; do
    require_text "$output" "RELEASE TAGS service=$service policy=none"
  done
  require_count "$output" "ROLLBACK PLAN service=$prefix-" 6
  require_text "$output" "PRETRAFFIC VERIFY release=1111111111111111111111111111111111111111 tag=c-1111111111111111 traffic=0 shape=exact same_origin=no-redirects"
  require_text "$output" "ACTIVE VERIFY release=1111111111111111111111111111111111111111 traffic=exact-100 tags=api-current-only same_origin=no-redirects"
  require_count "$output" "PYMES_GOOGLE_CALENDAR_ENABLED=false" 2
  require_count "$output" "PYMES_SCHEDULING_ACTION_TOKEN_SECRET=$prefix-scheduling-action-token-secret:DRY_RUN" 2
  forbid_text "$output" "PYMES_TRACING_EXPORTER="
  forbid_text "$output" "OTEL_EXPORTER_OTLP_ENDPOINT="
  forbid_text "$output" "PYMES_TRACE_SAMPLE_RATIO="
  forbid_text "$output" "$prefix-google-client-secret"
  forbid_text "$output" "$prefix-pergo-api-key"
  forbid_text "$output" "$prefix-pergo-webhook-secrets"
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

  require_text "$output" "SECURITY SERVICE service=$prefix-fiscal ingress=internal unauthenticated=denied exact_invokers=$api_principal,$worker_principal"
  require_text "$output" "SECURITY SERVICE service=$prefix-accounting ingress=internal unauthenticated=denied exact_invokers=$worker_principal"
  require_text "$output" "SECURITY SERVICE service=$prefix-accounting-admin ingress=internal unauthenticated=denied exact_invokers=$provision_principal"
  require_text "$output" "SECURITY SERVICE service=$prefix-worker ingress=internal unauthenticated=denied exact_invokers=none"

  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-api " "$output") ||
    fail "missing deploy command for $prefix-api"
  [[ "$deploy_line" == *"--ingress=all"* && "$deploy_line" == *"--allow-unauthenticated"* ]] ||
    fail "the BFF is not publicly reachable"
  [[ "$deploy_line" == *"FISCAL_ADAPTER_URL=https://$prefix-fiscal.$region.run.internal.invalid"* &&
     "$deploy_line" == *"PYMES_INTERNAL_KMS_KEY_VERSION="* &&
     "$deploy_line" == *"--network=default"* &&
     "$deploy_line" == *"--subnet=pymes-v3-serverless"* &&
     "$deploy_line" == *"--vpc-egress=all-traffic"* ]] ||
    fail "the BFF is missing private fiscal access through Direct VPC and workload identity"
  [[ "$deploy_line" != *"ACCOUNTING_URL="* && "$deploy_line" != *"ACCOUNTING_PROVISIONING_URL="* ]] ||
    fail "the public BFF received an accounting downstream URL"

  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-web " "$output") ||
    fail "missing deploy command for $prefix-web"
  [[ "$deploy_line" == *"--ingress=all"* && "$deploy_line" == *"--allow-unauthenticated"* ]] ||
    fail "the static web is not publicly reachable"
  [[ "$deploy_line" == *"--min=0"* && "$deploy_line" == *"--cpu-throttling"* ]] ||
    fail "the static web does not scale to zero"
  [[ "$deploy_line" != *"--set-cloudsql-instances="* && "$deploy_line" != *"--set-secrets="* ]] ||
    fail "the static web received database or secret access"
  [[ "$deploy_line" == *"PYMES_API_UPSTREAM=https://c-1111111111111111---$prefix-api.$region.run.internal.invalid"* ]] ||
    fail "the static web is missing the same-origin BFF proxy upstream"
  [[ "$deploy_line" == *"PYMES_PREFLIGHT_TAG=c-1111111111111111"* &&
     "$deploy_line" == *"PYMES_PREFLIGHT_TOKEN=\\[REDACTED\\]"* ]] ||
    fail "the static web is missing the authenticated pretraffic gate"
  [[ "$deploy_line" != *"$test_preflight_token"* ]] ||
    fail "the Cloud Run dry-run exposed the preflight capability"
  expected_web_marker="$environment:1111111111111111111111111111111111111111:sha256:$(printf '%064d' 0)"
  [[ "$deploy_line" == *"PYMES_RELEASE_MARKER=$expected_web_marker"* ]] ||
    fail "the static web is missing the exact environment/source/digest release marker"
  [[ "$deploy_line" != *"FISCAL_ADAPTER_URL="* && "$deploy_line" != *"ACCOUNTING_URL="* && "$deploy_line" != *"ACCOUNTING_PROVISIONING_URL="* ]] ||
    fail "the static web received a private downstream URL"

  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-worker " "$output") ||
    fail "missing deploy command for $prefix-worker"
  [[ "$deploy_line" == *"FISCAL_ADAPTER_URL=https://$prefix-fiscal.$region.run.internal.invalid"* ]] ||
    fail "worker is missing the fiscal private URL"
  [[ "$deploy_line" == *"ACCOUNTING_URL=https://$prefix-accounting.$region.run.internal.invalid"* ]] ||
    fail "worker is missing the accounting private URL"
  [[ "$deploy_line" == *"--network=default"* &&
     "$deploy_line" == *"--subnet=pymes-v3-serverless"* &&
     "$deploy_line" == *"--vpc-egress=all-traffic"* ]] ||
    fail "worker is missing Direct VPC all-traffic egress"
  [[ "$deploy_line" == *"--scaling=0"* &&
     "$deploy_line" == *"--min-instances=0"* &&
     "$deploy_line" == *"--no-deploy-health-check"* &&
     "$deploy_line" == *"--no-cpu-throttling"* ]] ||
    fail "worker candidate can start before operational promotion"
  require_text "$output" \
    "DRY-RUN gcloud run services update $prefix-worker --project=$project --region=$region --scaling=0 --quiet"
  require_text "$output" \
    "DRY-RUN gcloud run services update $prefix-worker --project=$project --region=$region --scaling=1 --quiet"
  require_text "$output" \
    "WORKER QUIESCENCE PLAN revision=$prefix-worker-dry-run-11111111 revision_min=0 deployment_health_check=disabled pretraffic_scaling=0 promotion_scaling=1 rollback_scaling=0"
  local web_promotion_line worker_promotion_line
  web_promotion_line=$(grep -nF \
    "DRY-RUN gcloud run services update-traffic $prefix-web --project=$project --region=$region --to-revisions=$prefix-web-dry-run-11111111=100 --quiet" "$output" |
    cut -d: -f1)
  worker_promotion_line=$(grep -nF \
    "DRY-RUN gcloud run services update-traffic $prefix-worker --project=$project --region=$region --to-revisions=$prefix-worker-dry-run-11111111=100 --quiet" "$output" |
    cut -d: -f1)
  [[ "$web_promotion_line" =~ ^[0-9]+$ &&
     "$worker_promotion_line" =~ ^[0-9]+$ &&
     "$worker_promotion_line" -gt "$web_promotion_line" ]] ||
    fail "worker must be the last promoted service"

  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-fiscal " "$output") ||
    fail "missing fiscal deploy command"
  [[ "$deploy_line" == *"FISCAL_ADAPTER_MODE=mock"* &&
     "$deploy_line" == *"FISCAL_KMS_KEY_NAME=projects/$project/locations/$region/keyRings/$prefix/cryptoKeys/fiscal-vault"* &&
     "$deploy_line" != *"FISCAL_LOCAL_KMS_KEY_B64"* &&
     "$deploy_line" == *"--network=default"* &&
     "$deploy_line" == *"--subnet=pymes-v3-serverless"* &&
     "$deploy_line" == *"--vpc-egress=all-traffic"* ]] ||
    fail "production fiscal mock is not fail-closed behind Cloud KMS and Direct VPC"

  deploy_line=$(grep -F -- "DRY-RUN gcloud run jobs deploy $prefix-provision-org " "$output") ||
    fail "missing provision job command"
  [[ "$deploy_line" == *"ACCOUNTING_PROVISIONING_URL=https://$prefix-accounting-admin.$region.run.internal.invalid"* ]] ||
    fail "provisioner is missing the accounting-admin private URL"
  [[ "$deploy_line" == *"--network=default"* &&
     "$deploy_line" == *"--subnet=pymes-v3-serverless"* &&
     "$deploy_line" == *"--vpc-egress=all-traffic"* ]] ||
    fail "provisioner is missing Direct VPC all-traffic egress"

  require_text "$output" \
    "configure Clerk webhook for https://pymes.$environment.dry-run.invalid/api/v1/webhooks/clerk"

  require_text "$traced_output" "TRACING status=configured exporter=otlp endpoint=explicit sample_ratio=0.25"
  require_count "$traced_output" "PYMES_TRACING_EXPORTER=otlp" 2
  require_count "$traced_output" "OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-collector.$environment.dry-run.invalid:4318" 2
  require_count "$traced_output" "PYMES_TRACE_SAMPLE_RATIO=0.25" 2
}

check_bootstrap_environment() {
  local output="$1"
  local prefix=pymes-v3-stg

  require_text "$output" \
    "DEPLOY STAGE stage=bootstrap environment=stg fiscal=mock pergo=false google=false promotion=disabled"
  require_text "$output" \
    "SECURITY CLERK_WEBHOOK_SECRET secret=$prefix-clerk-webhook-secret lifecycle=bootstrap-temporary stage=bootstrap source=dry-run-simulation"
  require_count "$output" "--no-traffic" 6
  require_count "$output" "--tag=c-1111111111111111" 6
  require_text "$output" \
    "PRETRAFFIC VERIFY release=1111111111111111111111111111111111111111 tag=c-1111111111111111 traffic=0 shape=exact same_origin=no-redirects"
  require_text "$output" \
    "BOOTSTRAP COMPLETE environment=stg release=1111111111111111111111111111111111111111 traffic=0 ingress=internal unauthenticated=denied worker_min=0 tag_public_access=denied promotion=skipped"
  require_text "$output" \
    "bootstrap web service URL: https://$prefix-web.us-central1.run.internal.invalid"
  local service deploy_line
  for service in "$prefix-api" "$prefix-web"; do
    deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $service " "$output") ||
      fail "missing bootstrap deploy command for $service"
    [[ "$deploy_line" == *"--ingress=internal"* &&
       "$deploy_line" == *"--no-allow-unauthenticated"* &&
       "$deploy_line" != *" --allow-unauthenticated"* ]] ||
      fail "$service bootstrap candidate is publicly invocable through its tag"
  done
  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-worker " "$output") ||
    fail "missing bootstrap deploy command for $prefix-worker"
  [[ "$deploy_line" == *"--scaling=0"* &&
     "$deploy_line" == *"--min-instances=0"* &&
     "$deploy_line" == *"--no-deploy-health-check"* &&
     "$deploy_line" == *"--no-allow-unauthenticated"* ]] ||
    fail "bootstrap worker could start or be invoked before operational promotion"
  require_count "$output" "DRY-RUN gcloud run services update-traffic" 6
  require_count "$output" " --clear-tags --quiet" 6
  require_count "$output" "BOOTSTRAP TAGS service=$prefix-" 6
  forbid_text "$output" "--to-revisions="
  forbid_text "$output" "--set-tags="
  forbid_text "$output" "--update-tags="
  forbid_text "$output" "RELEASE PROMOTED"
  forbid_text "$output" "ACTIVE VERIFY"
  forbid_text "$output" "ROLLBACK PLAN"
}

check_google_environment() {
  local environment="$1" output="$2"
  local prefix="pymes-v3-$environment"
  local redirect="https://pymes.$environment.dry-run.invalid/api/v1/calendars/google/oauth/callback"
  local kms_key="projects/$project/locations/$region/keyRings/$prefix/cryptoKeys/calendar-tokens"
  local deploy_line

  require_text "$output" "GOOGLE_CALENDAR status=enabled callback=same-origin kms=environment-scoped"
  require_count "$output" "PYMES_GOOGLE_CALENDAR_ENABLED=true" 2
  require_count "$output" "PYMES_GOOGLE_CLIENT_ID=client-$environment.apps.googleusercontent.com" 2
  require_count "$output" "PYMES_GOOGLE_REDIRECT_URL=$redirect" 2
  require_count "$output" "PYMES_CALENDAR_KMS_KEY=$kms_key" 2
  require_count "$output" "PYMES_GOOGLE_CLIENT_SECRET=$prefix-google-client-secret:DRY_RUN" 2

  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-web " "$output") ||
    fail "missing deploy command for $prefix-web"
  [[ "$deploy_line" != *"PYMES_GOOGLE_"* &&
    "$deploy_line" != *"PYMES_CALENDAR_KMS_KEY"* &&
    "$deploy_line" != *"google-client-secret"* ]] ||
    fail "the static web received Google OAuth configuration"
}

check_fiscal_arca_environment() {
  local environment="$1" output="$2"
  local prefix="pymes-v3-$environment"
  local deploy_line
  require_text "$output" "FISCAL mode=arca kms=environment-scoped"
  deploy_line=$(grep -F -- "DRY-RUN gcloud run deploy $prefix-fiscal " "$output") ||
    fail "missing ARCA fiscal deploy command"
  [[ "$deploy_line" == *"FISCAL_ADAPTER_MODE=arca"* &&
     "$deploy_line" == *"FISCAL_KMS_KEY_NAME=projects/$project/locations/$region/keyRings/$prefix/cryptoKeys/fiscal-vault"* &&
     "$deploy_line" == *"FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN=ARCA\\ Homologacion"* &&
     "$deploy_line" == *"FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN=ARCA\\ Produccion"* &&
     "$deploy_line" != *"FISCAL_MOCK_SCENARIO="* &&
     "$deploy_line" != *"FISCAL_LOCAL_KMS_KEY_B64"* ]] ||
    fail "ARCA mode is not isolated behind the fiscal KMS vault and reviewed issuer policies"
}

for environment in stg prd; do
  output="$scratch_dir/$environment.out"
  traced_output="$scratch_dir/$environment-traced.out"
  pergo_output="$scratch_dir/$environment-pergo.out"
  google_output="$scratch_dir/$environment-google.out"
  arca_output="$scratch_dir/$environment-arca.out"
  run_dry "$environment" "$output"
  run_dry "$environment" "$traced_output" "https://otel-collector.$environment.dry-run.invalid:4318"
  run_dry "$environment" "$pergo_output" "" false "" true
  run_dry "$environment" "$google_output" "" true
  run_dry "$environment" "$arca_output" "" false "" false arca
  check_environment "$environment" "$output" "$traced_output"
  check_pergo_environment "$environment" "$pergo_output"
  check_google_environment "$environment" "$google_output"
  check_fiscal_arca_environment "$environment" "$arca_output"
  forbid_text "$output" "fiscal-credential"
  echo "PASS cloud-run security dry-run environment=$environment resources_created=0"
done

bootstrap_output="$scratch_dir/stg-bootstrap.out"
PYMES_TEST_DEPLOY_STAGE=bootstrap \
PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE=bootstrap-temporary \
  run_dry stg "$bootstrap_output"
check_bootstrap_environment "$bootstrap_output"
echo "PASS Clerk bootstrap environment=stg traffic=0 promotion=disabled"

if PYMES_TEST_DEPLOY_STAGE=invalid \
  run_dry stg "$scratch_dir/invalid-deploy-stage.out" \
  2>"$scratch_dir/invalid-deploy-stage.err"; then
  fail "an unknown deploy stage was accepted"
fi
require_text "$scratch_dir/invalid-deploy-stage.err" \
  "PYMES_DEPLOY_STAGE must be bootstrap or operational"
echo "PASS Clerk deploy_stage invalid=rejected"

if PYMES_TEST_DEPLOY_STAGE=bootstrap \
  PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE=bootstrap-temporary \
  run_dry prd "$scratch_dir/bootstrap-prd.out" \
  2>"$scratch_dir/bootstrap-prd.err"; then
  fail "bootstrap was accepted for PRD"
fi
require_text "$scratch_dir/bootstrap-prd.err" \
  "PYMES_DEPLOY_STAGE=bootstrap is allowed only with PYMES_DEPLOY_ENV=stg"
echo "PASS Clerk bootstrap environment=prd rejected=true"

if PYMES_TEST_DEPLOY_STAGE=bootstrap \
  PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE=bootstrap-temporary \
  PYMES_TEST_ACTIVE_SERVICES=pymes-v3-stg-web \
  run_dry stg "$scratch_dir/bootstrap-active-service.out" \
  2>"$scratch_dir/bootstrap-active-service.err"; then
  fail "bootstrap accepted an STG service that already had active traffic"
fi
require_text "$scratch_dir/bootstrap-active-service.err" \
  "bootstrap refuses service pymes-v3-stg-web because it already has active traffic"
echo "PASS Clerk bootstrap active_stg=rejected"

if PYMES_TEST_DEPLOY_STAGE=bootstrap \
  run_dry stg "$scratch_dir/bootstrap-unlabeled.out" \
  2>"$scratch_dir/bootstrap-unlabeled.err"; then
  fail "bootstrap accepted a Clerk webhook secret without its temporary lifecycle label"
fi
require_text "$scratch_dir/bootstrap-unlabeled.err" \
  "must have label lifecycle=bootstrap-temporary during bootstrap"
echo "PASS Clerk bootstrap temporary_label=required"

if PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE=bootstrap-temporary \
  run_dry stg "$scratch_dir/operational-temporary.out" \
  2>"$scratch_dir/operational-temporary.err"; then
  fail "operational deployment accepted the temporary Clerk webhook lifecycle"
fi
require_text "$scratch_dir/operational-temporary.err" \
  "still has lifecycle=bootstrap-temporary"
echo "PASS Clerk operational temporary_label=rejected"

if PYMES_TEST_DEPLOY_STAGE=bootstrap \
  PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE=bootstrap-temporary \
  run_dry stg "$scratch_dir/bootstrap-arca.out" "" false "" false arca \
  2>"$scratch_dir/bootstrap-arca.err"; then
  fail "bootstrap accepted ARCA mode"
fi
require_text "$scratch_dir/bootstrap-arca.err" \
  "bootstrap requires PYMES_FISCAL_MODE=mock"
echo "PASS Clerk bootstrap fiscal_arca=rejected"

if PYMES_TEST_DEPLOY_STAGE=bootstrap \
  PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE=bootstrap-temporary \
  run_dry stg "$scratch_dir/bootstrap-pergo.out" "" false "" true \
  2>"$scratch_dir/bootstrap-pergo.err"; then
  fail "bootstrap accepted enabled PerGo"
fi
require_text "$scratch_dir/bootstrap-pergo.err" \
  "bootstrap requires PYMES_PERGO_ENABLED=false"
echo "PASS Clerk bootstrap pergo_enabled=rejected"

if PYMES_TEST_DEPLOY_STAGE=bootstrap \
  PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE=bootstrap-temporary \
  run_dry stg "$scratch_dir/bootstrap-google.out" "" true \
  2>"$scratch_dir/bootstrap-google.err"; then
  fail "bootstrap accepted enabled Google Calendar"
fi
require_text "$scratch_dir/bootstrap-google.err" \
  "bootstrap requires PYMES_GOOGLE_CALENDAR_ENABLED=false"
echo "PASS Clerk bootstrap google_enabled=rejected"

if PYMES_TEST_DEPLOY_STAGE=bootstrap \
  PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE=bootstrap-temporary \
  run_dry stg "$scratch_dir/bootstrap-origin.out" "" false "" false mock "" \
  "https://pymes.stg.dry-run.invalid" \
  2>"$scratch_dir/bootstrap-origin.err"; then
  fail "bootstrap accepted an operational public origin"
fi
require_text "$scratch_dir/bootstrap-origin.err" \
  "bootstrap must use only the reserved fail-closed origin"
echo "PASS Clerk bootstrap public_origin=reserved"

if PYMES_TEST_DEPLOY_STAGE=bootstrap \
  PYMES_TEST_CLOUD_RUN_DRY_RUN=false \
  PYMES_TEST_CLERK_WEBHOOK_LIFECYCLE=bootstrap-temporary \
  run_dry stg "$scratch_dir/live-lifecycle-simulation.out" \
  2>"$scratch_dir/live-lifecycle-simulation.err"; then
  fail "live deployment accepted a simulated Clerk secret lifecycle"
fi
require_text "$scratch_dir/live-lifecycle-simulation.err" \
  "is permitted only with PYMES_CLOUD_RUN_DRY_RUN=true"
echo "PASS Clerk lifecycle simulation=dry-run-only"

if run_dry stg "$scratch_dir/credential-endpoint.out" \
  "https://embedded-token@otel-collector.stg.dry-run.invalid:4318" \
  2>"$scratch_dir/credential-endpoint.err"; then
  fail "an OTLP endpoint with embedded credentials was accepted"
fi
require_text "$scratch_dir/credential-endpoint.err" \
  "OTEL_EXPORTER_OTLP_ENDPOINT must be an explicit endpoint without credentials"
echo "PASS cloud-run tracing endpoint credentials=rejected"

if run_dry stg "$scratch_dir/tenant-google-callback.out" "" true \
  "https://api.stg.dry-run.invalid/api/v1/organizations/org-a/calendars/google/oauth/callback" \
  2>"$scratch_dir/tenant-google-callback.err"; then
  fail "a tenant-specific Google OAuth callback was accepted"
fi
require_text "$scratch_dir/tenant-google-callback.err" \
  "PYMES_GOOGLE_REDIRECT_URL must equal the public same-origin callback"
echo "PASS Google Calendar callback scope=same-origin tenant_callback=rejected"

if run_dry stg "$scratch_dir/mutable-image.out" "" false "" false mock \
  "test.invalid/pymes-api:latest" \
  2>"$scratch_dir/mutable-image.err"; then
  fail "a mutable image tag was accepted"
fi
require_text "$scratch_dir/mutable-image.err" \
  "PYMES_API_IMAGE must be an immutable image reference pinned by @sha256"
echo "PASS Cloud Run image policy mutable_tag=rejected"

if run_dry stg "$scratch_dir/missing-origin.out" "" false "" false mock "" \
  "https://pymes.stg.dry-run.invalid" \
  "https://another.stg.dry-run.invalid" \
  2>"$scratch_dir/missing-origin.err"; then
  fail "a public origin absent from Clerk authorized parties was accepted"
fi
require_text "$scratch_dir/missing-origin.err" \
  "PYMES_CLERK_AUTHORIZED_PARTIES must include PYMES_PUBLIC_BASE_URL exactly"
echo "PASS public same-origin Clerk_authorized_party=required"

if run_dry stg "$scratch_dir/release-sha.out" "" false "" false mock "" "" "" \
  "not-a-commit" \
  2>"$scratch_dir/release-sha.err"; then
  fail "an invalid release SHA was accepted"
fi
require_text "$scratch_dir/release-sha.err" \
  "PYMES_RELEASE_SHA must be exactly 40 lowercase hexadecimal characters"
echo "PASS release provenance invalid_sha=rejected"

if run_dry stg "$scratch_dir/small-subnet.out" "" false "" false mock "" "" "" "" \
  "10.120.0.0/28" \
  2>"$scratch_dir/small-subnet.err"; then
  fail "an undersized Direct VPC subnet was accepted"
fi
require_text "$scratch_dir/small-subnet.err" \
  "PYMES_VPC_SUBNET_CIDR prefix must be between /20 and /26"
echo "PASS Direct VPC CIDR unsupported_prefix=rejected"

if run_dry stg "$scratch_dir/public-subnet.out" "" false "" false mock "" "" "" "" \
  "203.0.113.0/24" \
  2>"$scratch_dir/public-subnet.err"; then
  fail "a public Direct VPC subnet was accepted"
fi
require_text "$scratch_dir/public-subnet.err" \
  "PYMES_VPC_SUBNET_CIDR must use an RFC1918 private IPv4 range"
echo "PASS Direct VPC CIDR public_range=rejected"

if run_dry stg "$scratch_dir/unaligned-subnet.out" "" false "" false mock "" "" "" "" \
  "10.120.0.1/24" \
  2>"$scratch_dir/unaligned-subnet.err"; then
  fail "an unaligned Direct VPC subnet was accepted"
fi
require_text "$scratch_dir/unaligned-subnet.err" \
  "PYMES_VPC_SUBNET_CIDR must be aligned to its prefix"
echo "PASS Direct VPC CIDR unaligned_range=rejected"

if run_dry stg "$scratch_dir/invalid-fiscal-mode.out" "" false "" false invalid \
  2>"$scratch_dir/invalid-fiscal-mode.err"; then
  fail "an invalid fiscal mode was accepted"
fi
require_text "$scratch_dir/invalid-fiscal-mode.err" \
  "PYMES_FISCAL_MODE must be mock or arca"
echo "PASS Fiscal mode invalid=rejected"

if run_dry stg "$scratch_dir/arca-without-policy.out" "" false "" false arca "" "" "" "" "" false \
  2>"$scratch_dir/arca-without-policy.err"; then
  fail "ARCA mode without reviewed issuer policies was accepted"
fi
require_text "$scratch_dir/arca-without-policy.err" \
  "set the reviewed ARCA homologation certificate issuer pattern"
echo "PASS Fiscal ARCA issuer_policy=required"

if run_dry stg "$scratch_dir/pergo-workspace-pipe.out" "" false "" true mock "" "" "" "" "" true \
  "11111111-1111-4111-8111-111111111111|attacker" \
  2>"$scratch_dir/pergo-workspace-pipe.err"; then
  fail "a PerGo workspace ID containing a pipe was accepted"
fi
require_text "$scratch_dir/pergo-workspace-pipe.err" \
  "PYMES_PERGO_WORKSPACE_ID must be one canonical lowercase UUIDv4"
echo "PASS PerGo workspace ID pipe=rejected"

if run_dry stg "$scratch_dir/pergo-workspace-crlf.out" "" false "" true mock "" "" "" "" "" true \
  $'11111111-1111-4111-8111-111111111111\r\nattacker' \
  2>"$scratch_dir/pergo-workspace-crlf.err"; then
  fail "a PerGo workspace ID containing CRLF was accepted"
fi
require_text "$scratch_dir/pergo-workspace-crlf.err" \
  "PYMES_PERGO_WORKSPACE_ID must be one canonical lowercase UUIDv4"
echo "PASS PerGo workspace ID CRLF=rejected"

network_plan="$scratch_dir/network-plan.out"
PATH="$guarded_path" "$script_dir/bootstrap-network-egress.sh" >"$network_plan"
require_text "$network_plan" "PLAN ONLY resources_created=0"
require_text "$network_plan" "COST recurring Cloud NAT gateway"
echo "PASS network bootstrap default=read-only cost_gate=required"

if env "PATH=$guarded_path" PYMES_VPC_SUBNET_CIDR=203.0.113.0/24 \
  "$script_dir/bootstrap-network-egress.sh" \
  >"$scratch_dir/network-public.out" \
  2>"$scratch_dir/network-public.err"; then
  fail "network bootstrap accepted a public subnet range"
fi
require_text "$scratch_dir/network-public.err" \
  "PYMES_VPC_SUBNET_CIDR must use an RFC1918 private IPv4 range"
echo "PASS network bootstrap public_cidr=rejected"

if env "PATH=$guarded_path" PYMES_VPC_SUBNET_CIDR=10.120.0.1/24 \
  "$script_dir/bootstrap-network-egress.sh" \
  >"$scratch_dir/network-unaligned.out" \
  2>"$scratch_dir/network-unaligned.err"; then
  fail "network bootstrap accepted an unaligned subnet range"
fi
require_text "$scratch_dir/network-unaligned.err" \
  "PYMES_VPC_SUBNET_CIDR must be aligned to its prefix"
echo "PASS network bootstrap unaligned_cidr=rejected"

if env "PATH=$guarded_path" PYMES_NETWORK_BOOTSTRAP_APPLY=true \
  "$script_dir/bootstrap-network-egress.sh" \
  >"$scratch_dir/network-unacknowledged.out" \
  2>"$scratch_dir/network-unacknowledged.err"; then
  fail "network bootstrap applied without explicit recurring-cost acknowledgement"
fi
require_text "$scratch_dir/network-unacknowledged.err" \
  "PYMES_NETWORK_COST_ACK=I_ACCEPT_RECURRING_CLOUD_NAT_COST"
echo "PASS network bootstrap apply_without_cost_ack=rejected"

data_bootstrap="$script_dir/bootstrap-data-encryption.sh"
identity_bootstrap="$script_dir/bootstrap-internal-identity.sh"
secret_migration="$script_dir/migrate-regional-secrets.sh"
bash -n "$data_bootstrap" "$identity_bootstrap" "$secret_migration" \
  "$script_dir/bootstrap-network-egress.sh" "$candidate_tag_policy" \
  "$deploy_script" "$verify_script"
derived_tag=$(bash -c \
  'source "$1"; pymes_release_candidate_tag "$2"' \
  _ "$candidate_tag_policy" 1111111111111111111111111111111111111111)
[[ "$derived_tag" == "c-1111111111111111" ]] ||
  fail "candidate tag is not the canonical c-<16 hex> derivation"
bash -c \
  'source "$1"; pymes_validate_cloud_run_tagged_url "$2" "$3" "$4"' \
  _ "$candidate_tag_policy" \
  "https://c-1111111111111111---pymes-v3-stg-accounting-admin-abcde-uc.a.run.app" \
  "c-1111111111111111" "pymes-v3-stg-accounting-admin" ||
  fail "candidate tag policy rejected a DNS-safe Cloud Run tagged URL"
if bash -c \
  'source "$1"; pymes_validate_cloud_run_tagged_url "$2" "$3" "$4"' \
  _ "$candidate_tag_policy" \
  "https://c-1111111111111111---pymes-v3-stg-accounting-admin-abcdefghijklmnop-uc.a.run.app" \
  "c-1111111111111111" "pymes-v3-stg-accounting-admin" \
  >"$scratch_dir/candidate-tag-too-long.out" \
  2>"$scratch_dir/candidate-tag-too-long.err"; then
  fail "candidate tag policy accepted a tagged hostname with a DNS label over 63 characters"
fi
require_text "$scratch_dir/candidate-tag-too-long.err" \
  "Cloud Run tagged hostname contains an invalid DNS label"
echo "PASS Cloud Run candidate tag deterministic=c-16hex dns_label=validated"
require_text "$deploy_script" "--no-traffic"
require_text "$deploy_script" "--min-instances=0"
require_text "$deploy_script" "--no-deploy-health-check"
require_text "$deploy_script" "--scaling=0"
require_text "$deploy_script" "--scaling=1"
require_text "$deploy_script" 'rollback_on_exit()'
require_text "$deploy_script" 'ROLLBACK PRETRAFFIC'
require_text "$deploy_script" 'quiesce_worker_candidate()'
require_text "$deploy_script" 'settle_release_tags()'
require_text "$deploy_script" 'restore_previous_api_tags()'
require_text "$deploy_script" 'validate_release_baseline()'
require_text "$deploy_script" 'candidate_deploy_started["$service"]=true'
require_text "$deploy_script" 'service inventory returned $count exact entries'
require_text "$deploy_script" 'services set-iam-policy "$service" "$policy_file"'
require_text "$deploy_script" 'fail_closed_verified=true'
require_text "$deploy_script" 'wait_for_worker_release_ready()'
require_text "$deploy_script" 'jsonPayload.event=\"worker_release_ready\"'
require_text "$deploy_script" 'error("unsafe active tag")'
require_text "$deploy_script" 'active Web baseline does not point to the exact active tagged API revision'
require_text "$deploy_script" '--invoker-iam-check'
require_text "$deploy_script" '--set-tags="$candidate_tag=$candidate"'
require_text "$deploy_script" '--clear-tags'
require_text "$deploy_script" '--remove-tags="$candidate_tag"'
require_text "$deploy_script" 'assert_worker_manual_scaling 0'
require_text "$deploy_script" 'assert_worker_manual_scaling 1'
require_text "$deploy_script" 'gcloud run revisions delete "$candidate"'
require_text "$deploy_script" 'gcloud run services delete "$service"'
require_text "$deploy_script" 'PYMES_CLOUD_RUN_VERIFY_PHASE=pretraffic'
require_text "$deploy_script" 'lifecycle=bootstrap-temporary during bootstrap'
require_text "$deploy_script" 'tag_public_access=denied promotion=skipped'
require_text "$verify_script" 'environment variable names differ from the exact allowlist'
require_text "$verify_script" 'length == 1'
require_text "$verify_script" 'bootstrap verification is restricted to PYMES_CLOUD_RUN_VERIFY_PHASE=pretraffic'
require_text "$verify_script" 'operational verification is forbidden'
require_text "$verify_script" "--max-redirs 0"
require_text "$verify_script" 'returned a redirect Location header'
require_text "$verify_script" 'tag_public_access=denied'
require_text "$verify_script" 'candidate revision min scale must remain zero'
require_text "$verify_script" 'manual scaling differs from'
require_text "$verify_script" '($tagged | length) == 1'
require_text "$verify_script" '($tagged | length) == 0'
require_text "$verify_script" 'exact settled tag policy'
require_text "$verify_script" 'Cloud Run invoker IAM check disabled'
require_text "$verify_script" 'unauthenticated candidate Web gate'
require_text "$verify_script" 'active tagged API gate'
trap_install_line=$(grep -nF 'trap rollback_on_exit EXIT' "$deploy_script" |
  head -1 | cut -d: -f1)
first_migration_line=$(grep -nF 'migrate "$prefix-migrate"' "$deploy_script" |
  head -1 | cut -d: -f1)
[[ "$trap_install_line" =~ ^[0-9]+$ &&
   "$first_migration_line" =~ ^[0-9]+$ &&
   "$trap_install_line" -lt "$first_migration_line" ]] ||
  fail "operational pretraffic failures are not covered by rollback cleanup"
bootstrap_exit_line=$(grep -nF 'bootstrap release ${PYMES_RELEASE_SHA}' "$verify_script" | cut -d: -f1)
http_probe_line=$(grep -nF 'headers=$(mktemp)' "$verify_script" | head -1 | cut -d: -f1)
[[ "$bootstrap_exit_line" =~ ^[0-9]+$ &&
   "$http_probe_line" =~ ^[0-9]+$ &&
   "$bootstrap_exit_line" -lt "$http_probe_line" ]] ||
  fail "bootstrap verifier may probe a tag URL before proving public access is denied"
if sed -n '/^http_exact()/,/^}/p' "$verify_script" |
  grep -Eq -- '(^|[[:space:]])(--location(-trusted)?|-L)([=[:space:]]|$)'; then
  fail "the exact HTTP verifier follows redirects"
fi
require_text "$data_bootstrap" "rotation_period=7776000s"
require_text "$data_bootstrap" "ensure_key \"\$environment\" \"\$keyring\" secrets"
require_text "$data_bootstrap" "ensure_key \"\$environment\" \"\$keyring\" calendar-tokens"
require_text "$data_bootstrap" "ensure_key \"\$environment\" \"\$keyring\" fiscal-vault"
require_text "$data_bootstrap" "must not be inherited from project or key-ring scope"
require_text "$identity_bootstrap" 'api="pymes-v3-api-${environment}@${project}.iam.gserviceaccount.com"'
require_text "$secret_migration" '"$prefix-scheduling-action-token-secret"'
require_text "$secret_migration" '"$prefix-google-client-secret"'
forbid_text "$secret_migration" 'fiscal-credential'
echo "PASS bootstrap policy KMS=rotated-and-exact secrets=least-privilege"
