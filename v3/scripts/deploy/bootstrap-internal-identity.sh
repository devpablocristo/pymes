#!/usr/bin/env bash
set -euo pipefail

# Idempotently prepares the asymmetric Ed25519 key used by the Pymes worker to
# sign private workload credentials. This script changes GCP only when an
# operator runs it; application deploys always pin a numeric key version.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=gcp-target-policy.sh
source "$script_dir/gcp-target-policy.sh"

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
pymes_require_canonical_project_region "$project" "$region"
bootstrap_env=${PYMES_KMS_BOOTSTRAP_ENV:-all}
case "$bootstrap_env" in
  stg|prd|all) ;;
  *) echo "PYMES_KMS_BOOTSTRAP_ENV must be stg, prd, or all" >&2; exit 2 ;;
esac

export CLOUDSDK_CORE_PROJECT="$project"
gcloud services enable cloudkms.googleapis.com --project="$project" >/dev/null

if [[ "$bootstrap_env" == "all" ]]; then
  environments=(stg prd)
else
  environments=("$bootstrap_env")
fi

for environment in "${environments[@]}"; do
  keyring="pymes-v3-${environment}"
  key="internal-jwt-signing"
  api="pymes-v3-api-${environment}@${project}.iam.gserviceaccount.com"
  worker="pymes-v3-worker-${environment}@${project}.iam.gserviceaccount.com"
  provisioner="pymes-v3-provision-${environment}@${project}.iam.gserviceaccount.com"

  for principal in "$api" "$worker" "$provisioner"; do
    account_id=${principal%%@*}
    if ! gcloud iam service-accounts describe "$principal" --project="$project" >/dev/null 2>&1; then
      gcloud iam service-accounts create "$account_id" --project="$project" \
        --display-name="Pymes v3 ${environment} ${account_id#pymes-v3-}"
    fi
  done

  if ! gcloud kms keyrings describe "$keyring" --project="$project" --location="$region" >/dev/null 2>&1; then
    gcloud kms keyrings create "$keyring" --project="$project" --location="$region"
  fi
  if ! gcloud kms keys describe "$key" --project="$project" --location="$region" --keyring="$keyring" >/dev/null 2>&1; then
    gcloud kms keys create "$key" \
      --project="$project" \
      --location="$region" \
      --keyring="$keyring" \
      --purpose=asymmetric-signing \
      --default-algorithm=ec-sign-ed25519 \
      --protection-level=software \
      --labels="app=pymes-v3,env=${environment},use=internal-identity"
  fi

  key_json=$(gcloud kms keys describe "$key" --project="$project" --location="$region" --keyring="$keyring" --format=json)
  jq -e '.purpose == "ASYMMETRIC_SIGN" and .versionTemplate.algorithm == "EC_SIGN_ED25519"' \
    <<<"$key_json" >/dev/null

  # Asymmetric KMS keys have no mutable "primary" pointer: callers must pin an
  # explicit CryptoKeyVersion. Resolve the newest enabled version, retrying the
  # short propagation window immediately after key creation.
  version_json='[]'
  for attempt in 1 2 3 4 5 6 7 8 9 10; do
    version_json=$(gcloud kms keys versions list \
      --project="$project" --location="$region" --keyring="$keyring" --key="$key" \
      --filter='state=ENABLED' --sort-by='~createTime' --limit=1 --format=json)
    if jq -e 'length == 1 and .[0].algorithm == "EC_SIGN_ED25519" and .[0].state == "ENABLED"' \
      <<<"$version_json" >/dev/null; then
      break
    fi
    sleep 2
  done
  jq -e 'length == 1 and .[0].algorithm == "EC_SIGN_ED25519" and .[0].state == "ENABLED"' \
    <<<"$version_json" >/dev/null

  for principal in "$api" "$worker" "$provisioner"; do
    gcloud kms keys add-iam-policy-binding "$key" \
      --project="$project" --location="$region" --keyring="$keyring" \
      --member="serviceAccount:${principal}" --role=roles/cloudkms.signer --quiet >/dev/null
    gcloud kms keys add-iam-policy-binding "$key" \
      --project="$project" --location="$region" --keyring="$keyring" \
      --member="serviceAccount:${principal}" --role=roles/cloudkms.publicKeyViewer --quiet >/dev/null
  done

  expected_members=$(printf '%s\n' \
    "serviceAccount:$api" \
    "serviceAccount:$worker" \
    "serviceAccount:$provisioner" | LC_ALL=C sort -u)
  for role in roles/cloudkms.signer roles/cloudkms.publicKeyViewer; do
    inherited_project=$(gcloud projects get-iam-policy "$project" \
      --flatten='bindings[].members' --filter="bindings.role=$role" \
      --format='value(bindings.members)' |
      sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
    inherited_keyring=$(gcloud kms keyrings get-iam-policy "$keyring" \
      --project="$project" --location="$region" \
      --flatten='bindings[].members' --filter="bindings.role=$role" \
      --format='value(bindings.members)' |
      sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
    if [[ -n "$inherited_project" || -n "$inherited_keyring" ]]; then
      echo "$role must not be inherited by $key from project or key-ring scope" >&2
      exit 1
    fi
    direct_members=$(gcloud kms keys get-iam-policy "$key" \
      --project="$project" --location="$region" --keyring="$keyring" \
      --flatten='bindings[].members' --filter="bindings.role=$role" \
      --format='value(bindings.members)' |
      sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
    if [[ "$direct_members" != "$expected_members" ]]; then
      echo "$key must grant $role to exactly API, worker and provisioner for $environment" >&2
      exit 1
    fi
  done

  version=$(jq -r '.[0].name' <<<"$version_json")
  case "$version" in
    projects/*/locations/*/keyRings/*/cryptoKeys/*/cryptoKeyVersions/[1-9]*) ;;
    *) echo "KMS did not return an explicit enabled primary version for ${environment}" >&2; exit 1 ;;
  esac
  printf 'PYMES_INTERNAL_KMS_KEY_VERSION_%s=%s\n' "${environment^^}" "$version"
done

echo "internal identity KMS ready; copy the selected explicit version into the deployment environment"
