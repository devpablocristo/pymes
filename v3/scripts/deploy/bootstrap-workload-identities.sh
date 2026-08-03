#!/usr/bin/env bash
set -euo pipefail

# Creates only environment-scoped workload identities inside the existing
# shared Pymes project. Service accounts are free IAM principals; they do not
# create projects, networks, SQL instances or always-on workloads.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=gcp-target-policy.sh
source "$script_dir/gcp-target-policy.sh"

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
pymes_require_canonical_project "$project"
bootstrap_env=${PYMES_IDENTITY_BOOTSTRAP_ENV:-all}
case "$bootstrap_env" in
  stg|prd|all) ;;
  *) echo "PYMES_IDENTITY_BOOTSTRAP_ENV must be stg, prd, or all" >&2; exit 2 ;;
esac
export CLOUDSDK_CORE_PROJECT="$project"

if [[ "$bootstrap_env" == "all" ]]; then
  environments=(stg prd)
else
  environments=("$bootstrap_env")
fi

ensure_account() {
  local account_id="$1" display_name="$2" email attempt
  email="${account_id}@${project}.iam.gserviceaccount.com"
  if ! gcloud iam service-accounts describe "$email" --project="$project" >/dev/null 2>&1; then
    gcloud iam service-accounts create "$account_id" --project="$project" \
      --display-name="$display_name"
  fi
  # IAM propagation after service-account creation is eventually consistent.
  # Retry only this idempotent grant; never recreate or broaden the principal.
  for attempt in 1 2 3 4 5 6 7 8 9 10; do
    if gcloud projects add-iam-policy-binding "$project" \
      --member="serviceAccount:${email}" \
      --role=roles/cloudsql.client \
      --condition=None \
      --quiet >/dev/null 2>&1; then
      return
    fi
    sleep 2
  done
  gcloud projects add-iam-policy-binding "$project" \
    --member="serviceAccount:${email}" \
    --role=roles/cloudsql.client \
    --condition=None \
    --quiet >/dev/null
}

ensure_stateless_account() {
  local account_id="$1" display_name="$2" email
  email="${account_id}@${project}.iam.gserviceaccount.com"
  if ! gcloud iam service-accounts describe "$email" --project="$project" >/dev/null 2>&1; then
    gcloud iam service-accounts create "$account_id" --project="$project" \
      --display-name="$display_name"
  fi
}

for environment in "${environments[@]}"; do
  ensure_account "pymes-v3-api-${environment}" "Pymes v3 API ${environment}"
  # The static web serves immutable assets only. It deliberately receives no
  # Cloud SQL role, secrets or private-service permissions.
  ensure_stateless_account "pymes-v3-web-${environment}" "Pymes v3 Web ${environment}"
  ensure_account "pymes-v3-worker-${environment}" "Pymes v3 worker ${environment}"
  ensure_account "pymes-v3-provision-${environment}" "Pymes v3 provisioner ${environment}"
  ensure_account "pymes-v3-fiscal-${environment}" "Pymes v3 Fiscal ${environment}"
  ensure_account "pymes-v3-accounting-${environment}" "Pymes v3 Accounting ${environment}"
  ensure_account "pymes-v3-accounting-admin-${environment}" "Pymes v3 Accounting admin ${environment}"
  ensure_account "pymes-v3-migrate-${environment}" "Pymes v3 migration ${environment}"
  ensure_account "pymes-v3-fiscal-migrate-${environment}" "Pymes v3 Fiscal migration ${environment}"
  ensure_account "pymes-v3-acct-migrate-${environment}" "Pymes v3 Accounting migration ${environment}"
done

printf 'workload identities ready in shared project %s for %s\n' "$project" "$bootstrap_env"
