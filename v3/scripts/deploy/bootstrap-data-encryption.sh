#!/usr/bin/env bash
set -euo pipefail

# Idempotently prepares the environment-scoped symmetric keys used by Secret
# Manager, Calendar token envelope encryption and the Fiscal credential vault.
# It intentionally fails when an inherited or unexpected direct
# cryptoKeyEncrypterDecrypter grant would weaken workload isolation.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=gcp-target-policy.sh
source "$script_dir/gcp-target-policy.sh"

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
pymes_require_canonical_project_region "$project" "$region"
bootstrap_env=${PYMES_DATA_KMS_BOOTSTRAP_ENV:-all}
case "$bootstrap_env" in
  stg|prd|all) ;;
  *) echo "PYMES_DATA_KMS_BOOTSTRAP_ENV must be stg, prd, or all" >&2; exit 2 ;;
esac

rotation_period=7776000s
next_rotation_time=$(date -u -d '+90 days' '+%Y-%m-%dT%H:%M:%SZ')
project_number=${PYMES_GCP_PROJECT_NUMBER:-$(gcloud projects describe "$project" --format='value(projectNumber)')}
: "${project_number:?could not resolve GCP project number}"
pymes_require_canonical_project_number "$project_number"
secret_manager_agent="service-${project_number}@gcp-sa-secretmanager.iam.gserviceaccount.com"

export CLOUDSDK_CORE_PROJECT="$project"
gcloud services enable cloudkms.googleapis.com secretmanager.googleapis.com \
  --project="$project" >/dev/null
if ! gcloud iam service-accounts describe "$secret_manager_agent" \
  --project="$project" >/dev/null 2>&1; then
  gcloud beta services identity create \
    --service=secretmanager.googleapis.com \
    --project="$project" >/dev/null
fi

if [[ "$bootstrap_env" == "all" ]]; then
  environments=(stg prd)
else
  environments=("$bootstrap_env")
fi

ensure_service_account() {
  local principal="$1" environment="$2" account_id
  account_id=${principal%%@*}
  if ! gcloud iam service-accounts describe "$principal" \
    --project="$project" >/dev/null 2>&1; then
    gcloud iam service-accounts create "$account_id" \
      --project="$project" \
      --display-name="Pymes v3 ${environment} ${account_id#pymes-v3-}"
  fi
}

inherited_members() {
  local scope="$1" keyring="$2"
  if [[ "$scope" == "project" ]]; then
    gcloud projects get-iam-policy "$project" \
      --flatten='bindings[].members' \
      --filter='bindings.role=roles/cloudkms.cryptoKeyEncrypterDecrypter' \
      --format='value(bindings.members)'
    return
  fi
  gcloud kms keyrings get-iam-policy "$keyring" \
    --project="$project" --location="$region" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/cloudkms.cryptoKeyEncrypterDecrypter' \
    --format='value(bindings.members)'
}

ensure_key() {
  local environment="$1" keyring="$2" key="$3" expected_members="$4"
  local key_json primary direct_members inherited_project inherited_keyring
  if ! gcloud kms keys describe "$key" \
    --project="$project" --location="$region" \
    --keyring="$keyring" >/dev/null 2>&1; then
    gcloud kms keys create "$key" \
      --project="$project" \
      --location="$region" \
      --keyring="$keyring" \
      --purpose=encryption \
      --default-algorithm=google-symmetric-encryption \
      --rotation-period="$rotation_period" \
      --next-rotation-time="$next_rotation_time" \
      --protection-level=software \
      --labels="app=pymes-v3,env=${environment},use=${key}"
  else
    key_json=$(gcloud kms keys describe "$key" \
      --project="$project" --location="$region" \
      --keyring="$keyring" --format=json)
    if ! jq -e --arg rotation "$rotation_period" \
      '.rotationPeriod == $rotation and ((.nextRotationTime // "") | length) > 0' \
      <<<"$key_json" >/dev/null; then
      gcloud kms keys update "$key" \
        --project="$project" \
        --location="$region" \
        --keyring="$keyring" \
        --rotation-period="$rotation_period" \
        --next-rotation-time="$next_rotation_time" >/dev/null
    fi
  fi

  while IFS= read -r member; do
    [[ -n "$member" ]] || continue
    gcloud kms keys add-iam-policy-binding "$key" \
      --project="$project" --location="$region" --keyring="$keyring" \
      --member="$member" \
      --role=roles/cloudkms.cryptoKeyEncrypterDecrypter \
      --quiet >/dev/null
  done <<<"$expected_members"

  inherited_project=$(inherited_members project "$keyring" |
    sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  inherited_keyring=$(inherited_members keyring "$keyring" |
    sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ -n "$inherited_project" || -n "$inherited_keyring" ]]; then
    echo "roles/cloudkms.cryptoKeyEncrypterDecrypter for $key must not be inherited from project or key-ring scope" >&2
    exit 1
  fi

  direct_members=$(gcloud kms keys get-iam-policy "$key" \
    --project="$project" --location="$region" --keyring="$keyring" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/cloudkms.cryptoKeyEncrypterDecrypter' \
    --format='value(bindings.members)' |
    sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ "$direct_members" != "$expected_members" ]]; then
    echo "$key has unexpected direct cryptoKeyEncrypterDecrypter principals" >&2
    exit 1
  fi

  key_json=$(gcloud kms keys describe "$key" \
    --project="$project" --location="$region" \
    --keyring="$keyring" --format=json)
  jq -e \
    --arg rotation "$rotation_period" \
    '
      .purpose == "ENCRYPT_DECRYPT" and
      .versionTemplate.algorithm == "GOOGLE_SYMMETRIC_ENCRYPTION" and
      .primary.state == "ENABLED" and
      .primary.algorithm == "GOOGLE_SYMMETRIC_ENCRYPTION" and
      .rotationPeriod == $rotation and
      ((.nextRotationTime // "") | length) > 0
    ' <<<"$key_json" >/dev/null
  primary=$(jq -r '.primary.name' <<<"$key_json")
  printf 'KMS key=%s environment=%s primary=%s rotation=%s iam=exact\n' \
    "$key" "$environment" "$primary" "$rotation_period"
}

for environment in "${environments[@]}"; do
  keyring="pymes-v3-${environment}"
  api="pymes-v3-api-${environment}@${project}.iam.gserviceaccount.com"
  worker="pymes-v3-worker-${environment}@${project}.iam.gserviceaccount.com"
  fiscal="pymes-v3-fiscal-${environment}@${project}.iam.gserviceaccount.com"

  for principal in "$api" "$worker" "$fiscal"; do
    ensure_service_account "$principal" "$environment"
  done
  if ! gcloud kms keyrings describe "$keyring" \
    --project="$project" --location="$region" >/dev/null 2>&1; then
    gcloud kms keyrings create "$keyring" \
      --project="$project" --location="$region"
  fi

  secret_members="serviceAccount:$secret_manager_agent"
  calendar_members=$(printf '%s\n' \
    "serviceAccount:$api" \
    "serviceAccount:$worker" | LC_ALL=C sort -u)
  fiscal_members="serviceAccount:$fiscal"

  ensure_key "$environment" "$keyring" secrets "$secret_members"
  ensure_key "$environment" "$keyring" calendar-tokens "$calendar_members"
  ensure_key "$environment" "$keyring" fiscal-vault "$fiscal_members"
done

echo "data-encryption KMS ready for pymes-v3 ${bootstrap_env}"
