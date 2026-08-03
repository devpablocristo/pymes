#!/usr/bin/env bash
set -euo pipefail

# Cloud Run cannot consume Secret Manager regional secrets. This one-time,
# repeatable migration creates global secrets with one us-central1 replica,
# protected by the existing environment-specific regional CMEK keys. It copies
# only a missing latest value and never prints secret material.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=gcp-target-policy.sh
source "$script_dir/gcp-target-policy.sh"

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
pymes_require_canonical_project_region "$project" "$region"
migration_env=${PYMES_SECRET_MIGRATION_ENV:-all}
case "$migration_env" in
  stg|prd|all) ;;
  *) echo "PYMES_SECRET_MIGRATION_ENV must be stg, prd, or all" >&2; exit 2 ;;
esac
project_number=${PYMES_GCP_PROJECT_NUMBER:-$(gcloud projects describe "$project" --format='value(projectNumber)')}
: "${project_number:?could not resolve GCP project number}"
pymes_require_canonical_project_number "$project_number"
service_agent="service-${project_number}@gcp-sa-secretmanager.iam.gserviceaccount.com"
export CLOUDSDK_CORE_PROJECT="$project"
regional_secrets=$(CLOUDSDK_API_ENDPOINT_OVERRIDES_SECRETMANAGER="https://secretmanager.${region}.rep.googleapis.com/" \
  gcloud secrets list --location="$region" --project="$project" --format='value(name.basename())')

on_error() {
  local exit_code=$?
  printf 'secret migration failed at line %s (exit %s)\n' "$1" "$exit_code" >&2
  exit "$exit_code"
}
trap 'on_error "$LINENO"' ERR

regional_value_b64() {
  local secret="$1" metadata version
  if ! grep -Fxq "$secret" <<<"$regional_secrets"; then
    return
  fi
  metadata=$(CLOUDSDK_API_ENDPOINT_OVERRIDES_SECRETMANAGER="https://secretmanager.${region}.rep.googleapis.com/" \
    gcloud secrets versions list "$secret" --location="$region" --project="$project" \
      --format='value(name.basename(),state)' --sort-by='~name')
  version=$(awk 'toupper($2) == "ENABLED" { print $1; exit }' <<<"$metadata")
  if [[ -z "$version" ]]; then
    return
  fi
  CLOUDSDK_API_ENDPOINT_OVERRIDES_SECRETMANAGER="https://secretmanager.${region}.rep.googleapis.com/" \
    gcloud secrets versions access "$version" --secret="$secret" --location="$region" \
      --project="$project" | base64 --wrap=0
}

ensure_global_secret() {
  local environment="$1" secret="$2" key policy
  key="projects/${project}/locations/${region}/keyRings/pymes-v3-${environment}/cryptoKeys/secrets"
  if ! gcloud secrets describe "$secret" --project="$project" >/dev/null 2>&1; then
    policy=$(jq -nc --arg location "$region" --arg kms_key "$key" \
      '{userManaged:{replicas:[{location:$location,customerManagedEncryption:{kmsKeyName:$kms_key}}]}}')
    printf '%s' "$policy" | gcloud secrets create "$secret" --project="$project" \
      --labels="app=pymes-v3,env=${environment}" --replication-policy-file=- >/dev/null
  else
    gcloud secrets describe "$secret" --project="$project" --format=json | jq -e \
      --arg location "$region" --arg kms_key "$key" \
      '.replication.userManaged.replicas == [{"customerManagedEncryption":{"kmsKeyName":$kms_key},"location":$location}]' \
      >/dev/null
  fi
}

ensure_kms_access() {
  local environment="$1"
  gcloud kms keys add-iam-policy-binding secrets --project="$project" --location="$region" \
    --keyring="pymes-v3-${environment}" --member="serviceAccount:${service_agent}" \
    --role=roles/cloudkms.cryptoKeyEncrypterDecrypter >/dev/null
}

grant_accessor() {
  local secret="$1" service_account="$2"
  gcloud secrets add-iam-policy-binding "$secret" --project="$project" \
    --member="serviceAccount:${service_account}@${project}.iam.gserviceaccount.com" \
    --role=roles/secretmanager.secretAccessor >/dev/null
}

revoke_accessor() {
  local secret="$1" service_account="$2" member policy_json
  member="serviceAccount:${service_account}@${project}.iam.gserviceaccount.com"
  policy_json=$(gcloud secrets get-iam-policy "$secret" \
    --project="$project" --format=json) || {
    echo "could not audit obsolete accessor on $secret" >&2
    exit 1
  }
  if ! jq -e --arg member "$member" '
      any(
        .bindings[]?;
        .role == "roles/secretmanager.secretAccessor" and
        ((.members // []) | index($member) != null)
      )
    ' <<<"$policy_json" >/dev/null; then
    return
  fi
  gcloud secrets remove-iam-policy-binding "$secret" --project="$project" \
    --member="$member" --role=roles/secretmanager.secretAccessor --quiet >/dev/null
  policy_json=$(gcloud secrets get-iam-policy "$secret" \
    --project="$project" --format=json) || {
    echo "could not verify obsolete accessor removal on $secret" >&2
    exit 1
  }
  if jq -e --arg member "$member" '
    any(
      .bindings[]?;
      .role == "roles/secretmanager.secretAccessor" and
      ((.members // []) | index($member) != null)
    )
  ' <<<"$policy_json" >/dev/null; then
    echo "obsolete accessor remains on $secret: $member" >&2
    exit 1
  fi
}

assert_exact_accessors() {
  local secret="$1"
  shift
  local policy_json actual expected account
  policy_json=$(gcloud secrets get-iam-policy "$secret" \
    --project="$project" --format=json) || {
    echo "could not verify accessors on $secret" >&2
    exit 1
  }
  jq -e '
    all(
      .bindings[]?;
      .role != "roles/secretmanager.secretAccessor" or
      ((.condition // null) == null)
    )
  ' <<<"$policy_json" >/dev/null || {
    echo "conditional secret accessor is forbidden on $secret" >&2
    exit 1
  }
  actual=$(jq -r '
    [
      .bindings[]?
      | select(.role == "roles/secretmanager.secretAccessor")
      | (.members // [])[]
    ]
    | unique
    | sort
    | .[]
  ' <<<"$policy_json")
  expected=$(
    for account in "$@"; do
      printf 'serviceAccount:%s@%s.iam.gserviceaccount.com\n' \
        "$account" "$project"
    done | LC_ALL=C sort -u
  )
  if [[ "$actual" != "$expected" ]]; then
    echo "direct secret accessors differ from the exact allowlist on $secret" >&2
    echo "expected:" >&2
    printf '%s\n' "$expected" >&2
    echo "actual:" >&2
    printf '%s\n' "$actual" >&2
    exit 1
  fi
}

copy_latest_if_missing() {
  local secret="$1" data metadata
  metadata=$(gcloud secrets versions list "$secret" --project="$project" --format='value(state)')
  if grep -iqx enabled <<<"$metadata"; then
    return
  fi
  data=$(regional_value_b64 "$secret")
  if [[ -n "$data" ]]; then
    printf '%s' "$data" | base64 --decode | gcloud secrets versions add "$secret" --project="$project" --data-file=- >/dev/null
  fi
}

ensure_action_token_value() {
  local secret="$1" metadata
  metadata=$(gcloud secrets versions list "$secret" --project="$project" \
    --format='value(state)')
  if grep -iqx enabled <<<"$metadata"; then
    return
  fi
  command -v openssl >/dev/null || {
    echo "openssl is required to generate the scheduling action-token secret" >&2
    exit 1
  }
  # Generate directly into Secret Manager. The material is never stored in a
  # shell variable, temporary file, command line, or log.
  openssl rand 48 |
    base64 --wrap=0 |
    gcloud secrets versions add "$secret" --project="$project" \
      --data-file=- >/dev/null
}

disable_regional_versions() {
  local secret="$1" metadata version
  [[ "${PYMES_DISABLE_REGIONAL_AFTER_COPY:-false}" == "true" ]] || return 0
  grep -Fxq "$secret" <<<"$regional_secrets" || return 0
  metadata=$(CLOUDSDK_API_ENDPOINT_OVERRIDES_SECRETMANAGER="https://secretmanager.${region}.rep.googleapis.com/" \
    gcloud secrets versions list "$secret" --location="$region" --project="$project" \
      --format='value(name.basename(),state)')
  while read -r version state; do
    [[ -n "$version" && "${state^^}" == "ENABLED" ]] || continue
    CLOUDSDK_API_ENDPOINT_OVERRIDES_SECRETMANAGER="https://secretmanager.${region}.rep.googleapis.com/" \
      gcloud secrets versions disable "$version" --secret="$secret" --location="$region" \
        --project="$project" --quiet >/dev/null
  done <<<"$metadata"
}

if [[ "$migration_env" == "all" ]]; then
  environments=(stg prd)
else
  environments=("$migration_env")
fi

for environment in "${environments[@]}"; do
  prefix="pymes-v3-${environment}"
  ensure_kms_access "$environment"
  secrets=(
    "$prefix-clerk-secret-key"
    "$prefix-clerk-webhook-secret"
    "$prefix-scheduling-action-token-secret"
    "$prefix-pergo-api-key"
    "$prefix-pergo-webhook-secrets"
    "$prefix-google-client-secret"
    "$prefix-database-url"
    "$prefix-worker-database-url"
    "$prefix-migrate-database-url"
    "$prefix-fiscal-database-url"
    "$prefix-fiscal-migrate-database-url"
    "$prefix-accounting-database-url"
    "$prefix-accounting-admin-database-url"
    "$prefix-accounting-migrate-database-url"
  )
  for secret in "${secrets[@]}"; do
    ensure_global_secret "$environment" "$secret"
    copy_latest_if_missing "$secret"
    disable_regional_versions "$secret"
    printf 'configured global secret: %s\n' "$secret"
  done
  ensure_action_token_value "$prefix-scheduling-action-token-secret"

  grant_accessor "$prefix-clerk-secret-key" "pymes-v3-api-${environment}"
  grant_accessor "$prefix-clerk-webhook-secret" "pymes-v3-api-${environment}"
  grant_accessor "$prefix-scheduling-action-token-secret" "pymes-v3-api-${environment}"
  grant_accessor "$prefix-scheduling-action-token-secret" "pymes-v3-worker-${environment}"
  grant_accessor "$prefix-pergo-webhook-secrets" "pymes-v3-api-${environment}"
  grant_accessor "$prefix-google-client-secret" "pymes-v3-api-${environment}"
  grant_accessor "$prefix-google-client-secret" "pymes-v3-worker-${environment}"
  grant_accessor "$prefix-database-url" "pymes-v3-api-${environment}"
  grant_accessor "$prefix-database-url" "pymes-v3-provision-${environment}"
  revoke_accessor "$prefix-database-url" "pymes-v3-worker-${environment}"
  grant_accessor "$prefix-worker-database-url" "pymes-v3-worker-${environment}"
  grant_accessor "$prefix-pergo-api-key" "pymes-v3-worker-${environment}"
  grant_accessor "$prefix-fiscal-database-url" "pymes-v3-fiscal-${environment}"
  grant_accessor "$prefix-accounting-database-url" "pymes-v3-accounting-${environment}"
  grant_accessor "$prefix-accounting-admin-database-url" "pymes-v3-accounting-admin-${environment}"
  grant_accessor "$prefix-migrate-database-url" "pymes-v3-migrate-${environment}"
  grant_accessor "$prefix-fiscal-migrate-database-url" "pymes-v3-fiscal-migrate-${environment}"
  grant_accessor "$prefix-accounting-migrate-database-url" "pymes-v3-acct-migrate-${environment}"

  assert_exact_accessors "$prefix-clerk-secret-key" \
    "pymes-v3-api-${environment}"
  assert_exact_accessors "$prefix-clerk-webhook-secret" \
    "pymes-v3-api-${environment}"
  assert_exact_accessors "$prefix-scheduling-action-token-secret" \
    "pymes-v3-api-${environment}" "pymes-v3-worker-${environment}"
  assert_exact_accessors "$prefix-pergo-api-key" \
    "pymes-v3-worker-${environment}"
  assert_exact_accessors "$prefix-pergo-webhook-secrets" \
    "pymes-v3-api-${environment}"
  assert_exact_accessors "$prefix-google-client-secret" \
    "pymes-v3-api-${environment}" "pymes-v3-worker-${environment}"
  assert_exact_accessors "$prefix-database-url" \
    "pymes-v3-api-${environment}" "pymes-v3-provision-${environment}"
  assert_exact_accessors "$prefix-worker-database-url" \
    "pymes-v3-worker-${environment}"
  assert_exact_accessors "$prefix-migrate-database-url" \
    "pymes-v3-migrate-${environment}"
  assert_exact_accessors "$prefix-fiscal-database-url" \
    "pymes-v3-fiscal-${environment}"
  assert_exact_accessors "$prefix-fiscal-migrate-database-url" \
    "pymes-v3-fiscal-migrate-${environment}"
  assert_exact_accessors "$prefix-accounting-database-url" \
    "pymes-v3-accounting-${environment}"
  assert_exact_accessors "$prefix-accounting-admin-database-url" \
    "pymes-v3-accounting-admin-${environment}"
  assert_exact_accessors "$prefix-accounting-migrate-database-url" \
    "pymes-v3-acct-migrate-${environment}"
done

echo "migrated global Secret Manager containers for pymes-v3 ${migration_env}"
