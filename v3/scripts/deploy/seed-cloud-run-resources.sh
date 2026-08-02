#!/usr/bin/env bash
set -euo pipefail

# Seeds only the six Cloud Run services and five jobs that Pymes v3 needs so
# IAM can subsequently be granted at resource scope. It never executes a job,
# attaches a secret/database/network, or starts a service instance. The default
# is a read-only plan; apply requires a clean main checkout, a strict immutable
# release manifest, verified attestations, and explicit operator consent.

expected_project=pymes-dev-352318
expected_region=us-central1
expected_repository=pymes
apply=${PYMES_CLOUD_RUN_SEED_APPLY:-false}
verify=${PYMES_CLOUD_RUN_SEED_VERIFY:-false}
environment=${PYMES_CLOUD_RUN_SEED_ENV:-}
manifest_file=${PYMES_CLOUD_RUN_SEED_MANIFEST:-}
operator_email=${PYMES_CLOUD_RUN_SEED_OPERATOR_EMAIL:-}
operator_ack=${PYMES_CLOUD_RUN_SEED_ACK:-}
accounting_context=${OPEN_ACCOUNTING_CONTEXT:-.deps/open-accounting}
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
if [[ "$accounting_context" != /* ]]; then
  accounting_context="${repo_root}/${accounting_context}"
fi

manifest_keys=(
  PYMES_RELEASE_ENV
  PYMES_SOURCE_SHA
  PYMES_OPEN_ACCOUNTING_SOURCE_SHA
  PYMES_API_IMAGE
  PYMES_WEB_IMAGE
  PYMES_WORKER_IMAGE
  PYMES_FISCAL_IMAGE
  PYMES_ACCOUNTING_IMAGE
  PYMES_ACCOUNTING_ADMIN_IMAGE
  PYMES_PROVISION_IMAGE
  PYMES_MIGRATE_IMAGE
  PYMES_FISCAL_MIGRATE_IMAGE
  PYMES_ACCOUNTING_MIGRATE_IMAGE
)
service_components=(api web worker fiscal accounting accounting-admin)
job_components=(migrate fiscal-migrate accounting-migrate accounting-grants provision-org)
declare -A manifest_values=()
declare -A service_images=()
declare -A service_accounts=()
declare -A job_images=()
declare -A job_accounts=()

fail() {
  echo "$*" >&2
  return 1
}

validate_manifest() {
  local file="$1" expected_environment="$2" line key value allowed
  local digest_pattern registry_prefix
  [[ -f "$file" && ! -L "$file" ]] ||
    fail "seed manifest must be one regular non-symlink file" || return 1
  manifest_values=()
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -n "$line" && "$line" == *=* && "$line" != *$'\r'* ]] ||
      fail "seed manifest contains a malformed line" || return 1
    key=${line%%=*}
    value=${line#*=}
    allowed=false
    for expected_key in "${manifest_keys[@]}"; do
      if [[ "$key" == "$expected_key" ]]; then
        allowed=true
        break
      fi
    done
    [[ "$allowed" == "true" ]] ||
      fail "seed manifest contains non-allowlisted key: $key" || return 1
    [[ -z "${manifest_values[$key]+present}" ]] ||
      fail "seed manifest contains duplicate key: $key" || return 1
    [[ -n "$value" && "$value" != *[[:space:]]* ]] ||
      fail "seed manifest value is empty or contains whitespace: $key" || return 1
    manifest_values["$key"]=$value
  done <"$file"
  for key in "${manifest_keys[@]}"; do
    [[ -n "${manifest_values[$key]:-}" ]] ||
      fail "seed manifest is missing $key" || return 1
  done
  [[ "${#manifest_values[@]}" -eq "${#manifest_keys[@]}" ]] ||
    fail "seed manifest key count differs from the reviewed contract" || return 1
  [[ "${manifest_values[PYMES_RELEASE_ENV]}" == "$expected_environment" ]] ||
    fail "seed manifest environment differs from $expected_environment" || return 1
  [[ "${manifest_values[PYMES_SOURCE_SHA]}" =~ ^[0-9a-f]{40}$ &&
     "${manifest_values[PYMES_OPEN_ACCOUNTING_SOURCE_SHA]}" =~ ^[0-9a-f]{40}$ ]] ||
    fail "seed manifest source revisions must be full lowercase SHAs" || return 1

  registry_prefix="${expected_region}-docker.pkg.dev/${expected_project}/${expected_repository}/"
  digest_pattern="^${registry_prefix}[a-z0-9-]+@sha256:[0-9a-f]{64}$"
  for key in "${manifest_keys[@]}"; do
    [[ "$key" == PYMES_*_IMAGE ]] || continue
    [[ "${manifest_values[$key]}" =~ $digest_pattern ]] ||
      fail "$key is not an immutable image in the reviewed Artifact Registry" || return 1
  done
}

validate_github_release_state() {
  local repository_json="$1" branch_json="$2" runs_json="$3" release_sha="$4"
  jq -e \
    --arg sha "$release_sha" '
      .full_name == "devpablocristo/pymes" and
      .id == 1173650578 and
      .default_branch == "main" and
      .archived == false
    ' <<<"$repository_json" >/dev/null || {
    echo "GitHub repository identity differs from the reviewed Pymes source" >&2
    return 1
  }
  jq -e \
    --arg sha "$release_sha" '
      .name == "main" and
      .protected == true and
      .commit.sha == $sha
    ' <<<"$branch_json" >/dev/null || {
    echo "initial seed source is not the exact protected GitHub main head" >&2
    return 1
  }
  jq -e \
    --arg sha "$release_sha" '
      .workflow_runs
      | any(
          .head_sha == $sha and
          .head_branch == "main" and
          .event == "push" and
          .status == "completed" and
          .conclusion == "success"
        )
    ' <<<"$runs_json" >/dev/null || {
    echo "initial seed source has no successful Pymes V3 CI push run" >&2
    return 1
  }
}

configure_resources() {
  local prefix="pymes-v3-${environment}"
  service_images=(
    [api]="${manifest_values[PYMES_API_IMAGE]}"
    [web]="${manifest_values[PYMES_WEB_IMAGE]}"
    [worker]="${manifest_values[PYMES_WORKER_IMAGE]}"
    [fiscal]="${manifest_values[PYMES_FISCAL_IMAGE]}"
    [accounting]="${manifest_values[PYMES_ACCOUNTING_IMAGE]}"
    [accounting-admin]="${manifest_values[PYMES_ACCOUNTING_ADMIN_IMAGE]}"
  )
  service_accounts=(
    [api]="pymes-v3-api-${environment}@${expected_project}.iam.gserviceaccount.com"
    [web]="pymes-v3-web-${environment}@${expected_project}.iam.gserviceaccount.com"
    [worker]="pymes-v3-worker-${environment}@${expected_project}.iam.gserviceaccount.com"
    [fiscal]="pymes-v3-fiscal-${environment}@${expected_project}.iam.gserviceaccount.com"
    [accounting]="pymes-v3-accounting-${environment}@${expected_project}.iam.gserviceaccount.com"
    [accounting-admin]="pymes-v3-accounting-admin-${environment}@${expected_project}.iam.gserviceaccount.com"
  )
  job_images=(
    [migrate]="${manifest_values[PYMES_MIGRATE_IMAGE]}"
    [fiscal-migrate]="${manifest_values[PYMES_FISCAL_MIGRATE_IMAGE]}"
    [accounting-migrate]="${manifest_values[PYMES_ACCOUNTING_MIGRATE_IMAGE]}"
    [accounting-grants]="${manifest_values[PYMES_ACCOUNTING_ADMIN_IMAGE]}"
    [provision-org]="${manifest_values[PYMES_PROVISION_IMAGE]}"
  )
  job_accounts=(
    [migrate]="pymes-v3-migrate-${environment}@${expected_project}.iam.gserviceaccount.com"
    [fiscal-migrate]="pymes-v3-fiscal-migrate-${environment}@${expected_project}.iam.gserviceaccount.com"
    [accounting-migrate]="pymes-v3-acct-migrate-${environment}@${expected_project}.iam.gserviceaccount.com"
    [accounting-grants]="pymes-v3-accounting-admin-${environment}@${expected_project}.iam.gserviceaccount.com"
    [provision-org]="pymes-v3-provision-${environment}@${expected_project}.iam.gserviceaccount.com"
  )
  : "$prefix"
}

verify_seed_service_json() {
  local json="$1" name="$2" expected_environment="$3" release_sha="$4"
  local expected_image="$5" expected_account="$6"
  jq -e \
    --arg name "$name" \
    --arg environment "$expected_environment" \
    --arg release_sha "$release_sha" \
    --arg image "$expected_image" \
    --arg account "$expected_account" '
      .metadata.name == $name and
      .metadata.labels.app == "pymes-v3" and
      .metadata.labels.env == $environment and
      .metadata.labels["pymes-v3-seed"] == "true" and
      .metadata.labels["pymes-v3-release"] == $release_sha and
      .metadata.annotations["run.googleapis.com/ingress"] == "internal" and
      all(
        (.status.traffic // .status.trafficStatuses // [])[];
        (.percent // 0) == 0
      ) and
      (
        .metadata.annotations["run.googleapis.com/scalingMode"] //
        .scaling.scalingMode //
        ""
        | ascii_downcase
      ) == "manual" and
      (
        .metadata.annotations["run.googleapis.com/manualInstanceCount"] //
        .scaling.manualInstanceCount //
        -1
        | tonumber
      ) == 0 and
      (
        (.spec.template.metadata.annotations["autoscaling.knative.dev/minScale"] // "0") == "0"
      ) and
      (
        .spec.template.spec.serviceAccountName //
        .template.serviceAccount //
        ""
      ) == $account and
      (
        .spec.template.spec.containers //
        .template.containers //
        []
      ) as $containers |
      ($containers | length) == 1 and
      $containers[0].image == $image and
      (($containers[0].env // []) | length) == 0 and
      (($containers[0].envFrom // []) | length) == 0 and
      (($containers[0].command // []) | length) == 0 and
      (($containers[0].args // []) | length) == 0 and
      (($containers[0].volumeMounts // []) | length) == 0 and
      (($containers[0].dependsOn // []) | length) == 0 and
      (
        .spec.template.spec.volumes //
        .template.volumes //
        []
        | length
      ) == 0 and
      (
        .spec.template.metadata.annotations["run.googleapis.com/cloudsql-instances"] //
        .template.annotations["run.googleapis.com/cloudsql-instances"] //
        ""
      ) == "" and
      (
        .spec.template.metadata.annotations["run.googleapis.com/network-interfaces"] //
        .template.annotations["run.googleapis.com/network-interfaces"] //
        ""
      ) == "" and
      (
        .spec.template.metadata.annotations["run.googleapis.com/vpc-access-egress"] //
        .template.annotations["run.googleapis.com/vpc-access-egress"] //
        ""
      ) == "" and
      (
        .spec.template.metadata.annotations["run.googleapis.com/vpc-access-connector"] //
        .template.annotations["run.googleapis.com/vpc-access-connector"] //
        .template.vpcAccess.connector //
        ""
      ) == "" and
      (
        .template.vpcAccess.networkInterfaces //
        []
        | length
      ) == 0
    ' <<<"$json" >/dev/null
}

verify_seed_job_json() {
  local json="$1" name="$2" expected_environment="$3" release_sha="$4"
  local expected_image="$5" expected_account="$6"
  jq -e \
    --arg name "$name" \
    --arg environment "$expected_environment" \
    --arg release_sha "$release_sha" \
    --arg image "$expected_image" \
    --arg account "$expected_account" '
      .metadata.name == $name and
      .metadata.labels.app == "pymes-v3" and
      .metadata.labels.env == $environment and
      .metadata.labels["pymes-v3-seed"] == "true" and
      .metadata.labels["pymes-v3-release"] == $release_sha and
      (
        .spec.template.spec.taskCount //
        .template.taskCount //
        1
        | tonumber
      ) == 1 and
      (
        .spec.template.spec.template.spec.maxRetries //
        .template.template.maxRetries //
        -1
        | tonumber
      ) == 0 and
      (
        .spec.template.spec.template.spec.serviceAccountName //
        .template.template.serviceAccount //
        ""
      ) == $account and
      (
        .spec.template.spec.template.spec.containers //
        .template.template.containers //
        []
      ) as $containers |
      ($containers | length) == 1 and
      $containers[0].image == $image and
      (($containers[0].env // []) | length) == 0 and
      (($containers[0].envFrom // []) | length) == 0 and
      (($containers[0].command // []) | length) == 0 and
      (($containers[0].args // []) | length) == 0 and
      (($containers[0].volumeMounts // []) | length) == 0 and
      (($containers[0].dependsOn // []) | length) == 0 and
      (
        .spec.template.spec.template.spec.volumes //
        .template.template.volumes //
        []
        | length
      ) == 0 and
      (
        .spec.template.spec.template.metadata.annotations["run.googleapis.com/cloudsql-instances"] //
        .template.template.annotations["run.googleapis.com/cloudsql-instances"] //
        ""
      ) == "" and
      (
        .spec.template.spec.template.metadata.annotations["run.googleapis.com/network-interfaces"] //
        .template.template.annotations["run.googleapis.com/network-interfaces"] //
        ""
      ) == "" and
      (
        .spec.template.spec.template.metadata.annotations["run.googleapis.com/vpc-access-egress"] //
        .template.template.annotations["run.googleapis.com/vpc-access-egress"] //
        ""
      ) == "" and
      (
        .spec.template.spec.template.metadata.annotations["run.googleapis.com/vpc-access-connector"] //
        .template.template.annotations["run.googleapis.com/vpc-access-connector"] //
        .template.template.vpcAccess.connector //
        ""
      ) == "" and
      (
        .template.template.vpcAccess.networkInterfaces //
        []
        | length
      ) == 0
    ' <<<"$json" >/dev/null
}

verify_existing_or_absent() {
  local require_presence="$1" component name json policy executions
  local service_names job_names
  local release_sha=${manifest_values[PYMES_SOURCE_SHA]}
  service_names=$(gcloud run services list \
    --project="$expected_project" --region="$expected_region" \
    --format='value(metadata.name)')
  job_names=$(gcloud run jobs list \
    --project="$expected_project" --region="$expected_region" \
    --format='value(metadata.name)')
  for component in "${service_components[@]}"; do
    name="pymes-v3-${environment}-${component}"
    if ! grep -Fxq "$name" <<<"$service_names"; then
      [[ "$require_presence" == "false" ]] ||
        fail "required inert seed service is absent: $name" || return 1
      continue
    fi
    json=$(gcloud run services describe "$name" \
      --project="$expected_project" --region="$expected_region" \
      --format=json)
    verify_seed_service_json \
      "$json" "$name" "$environment" "$release_sha" \
      "${service_images[$component]}" "${service_accounts[$component]}" ||
      fail "existing service is not the exact inert seed: $name" || return 1
    policy=$(gcloud run services get-iam-policy "$name" \
      --project="$expected_project" --region="$expected_region" --format=json)
    jq -e '(.bindings // []) | length == 0' <<<"$policy" >/dev/null ||
      fail "existing seed service IAM policy is not empty: $name" || return 1
  done
  for component in "${job_components[@]}"; do
    name="pymes-v3-${environment}-${component}"
    if ! grep -Fxq "$name" <<<"$job_names"; then
      [[ "$require_presence" == "false" ]] ||
        fail "required inert seed job is absent: $name" || return 1
      continue
    fi
    json=$(gcloud run jobs describe "$name" \
      --project="$expected_project" --region="$expected_region" \
      --format=json)
    verify_seed_job_json \
      "$json" "$name" "$environment" "$release_sha" \
      "${job_images[$component]}" "${job_accounts[$component]}" ||
      fail "existing job is not the exact inert seed: $name" || return 1
    policy=$(gcloud run jobs get-iam-policy "$name" \
      --project="$expected_project" --region="$expected_region" --format=json)
    jq -e '(.bindings // []) | length == 0' <<<"$policy" >/dev/null ||
      fail "existing seed job IAM policy is not empty: $name" || return 1
    executions=$(gcloud run jobs executions list --job="$name" \
      --project="$expected_project" --region="$expected_region" --format=json)
    jq -e 'length == 0' <<<"$executions" >/dev/null ||
      fail "existing seed job has already been executed: $name" || return 1
  done
}

seed_service() {
  local component="$1" name
  name="pymes-v3-${environment}-${component}"
  gcloud run deploy "$name" \
    --project="$expected_project" \
    --region="$expected_region" \
    --image="${service_images[$component]}" \
    --service-account="${service_accounts[$component]}" \
    --labels="app=pymes-v3,env=${environment},pymes-v3-seed=true,pymes-v3-release=${manifest_values[PYMES_SOURCE_SHA]}" \
    --ingress=internal \
    --scaling=0 \
    --no-traffic \
    --no-deploy-health-check \
    --clear-cloudsql-instances \
    --clear-env-vars \
    --clear-secrets \
    --clear-network \
    --clear-volumes \
    --clear-volume-mounts \
    --command="" \
    --args="" \
    --quiet
}

seed_job() {
  local component="$1" name
  name="pymes-v3-${environment}-${component}"
  gcloud run jobs deploy "$name" \
    --project="$expected_project" \
    --region="$expected_region" \
    --image="${job_images[$component]}" \
    --service-account="${job_accounts[$component]}" \
    --labels="app=pymes-v3,env=${environment},pymes-v3-seed=true,pymes-v3-release=${manifest_values[PYMES_SOURCE_SHA]}" \
    --tasks=1 \
    --max-retries=0 \
    --clear-cloudsql-instances \
    --clear-env-vars \
    --clear-secrets \
    --clear-network \
    --clear-volumes \
    --clear-volume-mounts \
    --command="" \
    --args="" \
    --quiet
}

assert_direct_gcloud_auth() {
  local variable property value
  for variable in \
    CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT \
    CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE \
    CLOUDSDK_AUTH_ACCESS_TOKEN_FILE \
    CLOUDSDK_AUTH_LOGIN_CONFIG_FILE; do
    [[ -z "${!variable-}" ]] || {
      echo "initial seed forbids delegated or overridden gcloud credentials: $variable" >&2
      return 1
    }
  done
  for property in \
    auth/impersonate_service_account \
    auth/credential_file_override \
    auth/access_token_file \
    auth/login_config_file; do
    value=$(gcloud config get-value "$property" 2>/dev/null) || {
      echo "could not verify direct gcloud credential property: $property" >&2
      return 1
    }
    [[ -z "$value" || "$value" == "(unset)" ]] || {
      echo "initial seed forbids delegated or overridden gcloud credentials: $property" >&2
      return 1
    }
  done
}

main() {
  local active_account expected_ack manifest_sha
  local seed_started_at seed_completed_at component
  local repository_json branch_json runs_json
  case "$apply" in true|false) ;; *)
    echo "PYMES_CLOUD_RUN_SEED_APPLY must be true or false" >&2
    exit 2
  esac
  case "$verify" in true|false) ;; *)
    echo "PYMES_CLOUD_RUN_SEED_VERIFY must be true or false" >&2
    exit 2
  esac
  if [[ "$apply" == "true" && "$verify" == "true" ]]; then
    echo "seed apply and verify modes are mutually exclusive" >&2
    exit 2
  fi
  case "$environment" in stg|prd) ;; *)
    echo "PYMES_CLOUD_RUN_SEED_ENV must be stg or prd" >&2
    exit 2
  esac
  [[ -n "$manifest_file" ]] || {
    echo "set PYMES_CLOUD_RUN_SEED_MANIFEST to the downloaded immutable manifest" >&2
    exit 2
  }
  validate_manifest "$manifest_file" "$environment"
  configure_resources

  if [[ "$apply" == "false" && "$verify" == "false" ]]; then
    echo "PLAN seed environment=${environment} project=${expected_project} region=${expected_region}"
    echo "PLAN source_sha=${manifest_values[PYMES_SOURCE_SHA]} accounting_sha=${manifest_values[PYMES_OPEN_ACCOUNTING_SOURCE_SHA]}"
    printf 'PLAN service=%s image=%s scaling=0 traffic=0 ingress=internal secrets=none database=none network=none\n' \
      "pymes-v3-${environment}-api" "${service_images[api]}"
    for component in "${service_components[@]:1}"; do
      printf 'PLAN service=%s image=%s scaling=0 traffic=0 ingress=internal secrets=none database=none network=none\n' \
        "pymes-v3-${environment}-${component}" "${service_images[$component]}"
    done
    for component in "${job_components[@]}"; do
      printf 'PLAN job=%s image=%s execute=false secrets=none database=none network=none\n' \
        "pymes-v3-${environment}-${component}" "${job_images[$component]}"
    done
    echo "No Cloud Run resources changed."
    exit 0
  fi

  for command in gcloud jq sha256sum; do
    command -v "$command" >/dev/null 2>&1 || {
      echo "$command is required" >&2
      exit 1
    }
  done
  if [[ "$verify" == "true" ]]; then
    verify_existing_or_absent true
    echo "Cloud Run inert seed verified: environment=${environment} release=${manifest_values[PYMES_SOURCE_SHA]} services=6 jobs=5 traffic=0 executions=0"
    exit 0
  fi
  for command in docker gh git; do
    command -v "$command" >/dev/null 2>&1 || {
      echo "$command is required" >&2
      exit 1
    }
  done
  [[ "$(git -C "$repo_root" branch --show-current)" == "main" &&
     "$(git -C "$repo_root" rev-parse HEAD)" == "${manifest_values[PYMES_SOURCE_SHA]}" &&
     -z "$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)" ]] || {
    echo "seed apply requires a clean main checkout at the manifest Pymes SHA" >&2
    exit 1
  }
  [[ -d "$accounting_context/.git" ]] || {
    echo "seed apply requires the pinned Open Accounting checkout at $accounting_context" >&2
    exit 1
  }
  [[ "$(git -C "$accounting_context" rev-parse HEAD)" == "${manifest_values[PYMES_OPEN_ACCOUNTING_SOURCE_SHA]}" &&
     -z "$(git -C "$accounting_context" status --porcelain=v1 --untracked-files=all)" ]] || {
    echo "seed apply requires a clean Open Accounting checkout at the manifest SHA" >&2
    exit 1
  }
  active_account=$(gcloud auth list \
    --filter=status:ACTIVE --format='value(account)')
  [[ -n "$operator_email" && "$active_account" == "$operator_email" ]] || {
    echo "active gcloud account must equal PYMES_CLOUD_RUN_SEED_OPERATOR_EMAIL" >&2
    exit 1
  }
  [[ "$operator_email" == "softponti@gmail.com" ]] || {
    echo "initial seed is restricted to the reviewed existing project Owner softponti@gmail.com" >&2
    exit 1
  }
  assert_direct_gcloud_auth
  [[ "$(gcloud config get-value project 2>/dev/null)" == "$expected_project" ]] || {
    echo "active gcloud project must be $expected_project" >&2
    exit 1
  }
  expected_ack="SEED_${environment^^}_${manifest_values[PYMES_SOURCE_SHA]}"
  [[ "$operator_ack" == "$expected_ack" ]] || {
    echo "set PYMES_CLOUD_RUN_SEED_ACK=$expected_ack" >&2
    exit 2
  }

  repository_json=$(gh api --method GET repos/devpablocristo/pymes)
  branch_json=$(gh api --method GET repos/devpablocristo/pymes/branches/main)
  runs_json=$(gh api --method GET \
    repos/devpablocristo/pymes/actions/workflows/v3-ci.yml/runs \
    -f head_sha="${manifest_values[PYMES_SOURCE_SHA]}" \
    -f branch=main \
    -f event=push \
    -f status=success \
    -f per_page=100)
  validate_github_release_state \
    "$repository_json" "$branch_json" "$runs_json" \
    "${manifest_values[PYMES_SOURCE_SHA]}"

  for key in "${manifest_keys[@]}"; do
    export "$key=${manifest_values[$key]}"
  done
  (
    cd "$repo_root"
    PYMES_RELEASE_ENV="$environment" \
      PYMES_GCP_PROJECT="$expected_project" \
      PYMES_GCP_REGION="$expected_region" \
      PYMES_ARTIFACT_REPOSITORY="$expected_repository" \
      PYMES_SOURCE_SHA="${manifest_values[PYMES_SOURCE_SHA]}" \
      OPEN_ACCOUNTING_CONTEXT="$accounting_context" \
      "$script_dir/build-push-images.sh" verify-attestations
  )

  seed_started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  verify_existing_or_absent false
  for component in "${service_components[@]}"; do
    seed_service "$component"
  done
  for component in "${job_components[@]}"; do
    seed_job "$component"
  done
  verify_existing_or_absent true

  seed_completed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  manifest_sha=$(sha256sum "$manifest_file" | awk '{print $1}')
  echo "CLOUD RUN SEED COMPLETE environment=${environment} release=${manifest_values[PYMES_SOURCE_SHA]} operator=${operator_email} services=6 jobs=5 traffic=0 executions=0"
  echo "Finalize evidence:"
  echo "  PYMES_INITIAL_SEED_RELEASE_SHA=${manifest_values[PYMES_SOURCE_SHA]}"
  echo "  PYMES_INITIAL_SEED_OPERATOR_EMAIL=${operator_email}"
  echo "  PYMES_INITIAL_SEED_STARTED_AT=${seed_started_at}"
  echo "  PYMES_INITIAL_SEED_COMPLETED_AT=${seed_completed_at}"
  printf '  PYMES_INITIAL_SEED_MANIFEST=%q\n' "$manifest_file"
  echo "  PYMES_INITIAL_SEED_MANIFEST_SHA256=${manifest_sha}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
