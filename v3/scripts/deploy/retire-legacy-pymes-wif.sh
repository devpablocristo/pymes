#!/usr/bin/env bash
set -euo pipefail

# Removes only the legacy Pymes repository principal from the two historical
# service-account trust policies. The shared account remains enabled for its
# unrelated callers; the Pymes-only account is disabled, which is reversible.
# Apply is gated by an exact successful STG release through the new WIF boundary.
# Audit additionally requires a second successful STG release whose start is
# later than the durable DisableServiceAccount Cloud Audit Logs marker.

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
mode=${PYMES_LEGACY_WIF_MODE:-plan}
pre_canary_run_id=${PYMES_STG_CANARY_RUN_ID:-}
post_canary_run_id=${PYMES_STG_POST_RETIRE_CANARY_RUN_ID:-}
source_only=${PYMES_LEGACY_WIF_SOURCE_ONLY:-false}
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=release-authority-policy.sh
source "$script_dir/release-authority-policy.sh"
repository=devpablocristo/pymes
repository_id=1173650578
repository_owner_id=81805584
pool=pymes-v3-release-pool
provider=github
workflow_path=.github/workflows/v3-release.yml
workflow_ref="${repository}/${workflow_path}@refs/heads/main"
dedicated_account="pymes-github-actions-stg@${project}.iam.gserviceaccount.com"
shared_account="github-actions@${project}.iam.gserviceaccount.com"

repo_root=
release_workflow=
project_number=
legacy_principal=
reviewed_main_sha=
reviewed_tree_sha=

case "$source_only" in
  true|false) ;;
  *) echo "PYMES_LEGACY_WIF_SOURCE_ONLY must be true or false" >&2; exit 2 ;;
esac
if [[ "$source_only" == "true" && "${BASH_SOURCE[0]}" == "$0" ]]; then
  echo "PYMES_LEGACY_WIF_SOURCE_ONLY is test-only and requires sourcing the script" >&2
  exit 2
fi

if [[ "$source_only" != "true" ]]; then
  [[ "$project" == "pymes-dev-352318" ]] || {
    echo "legacy Pymes WIF retirement is restricted to pymes-dev-352318" >&2
    exit 2
  }
  [[ "$region" == "us-central1" ]] || {
    echo "legacy Pymes WIF retirement is restricted to us-central1" >&2
    exit 2
  }
  case "$mode" in
    plan|audit|apply) ;;
    *) echo "PYMES_LEGACY_WIF_MODE must be plan, audit or apply" >&2; exit 2 ;;
  esac

  if [[ "$mode" == "plan" ]]; then
    printf '%s\n' \
      "PLAN require pre/post STG canaries from the same exact reviewed main commit and release tree through ${pool}/${provider}" \
      "PLAN prove the dedicated provider, callers and effective authorization exactly" \
      "PLAN fail while any shared caller retains effective authority over a Pymes resource" \
      "PLAN prove the Pymes-only account has no other callers, user keys or attached workloads in any region" \
      "PLAN remove only the Pymes repository WIF principal from ${dedicated_account}" \
      "PLAN remove only the Pymes repository WIF principal from ${shared_account}" \
      "PLAN preserve every unrelated shared-account policy member exactly" \
      "PLAN disable the Pymes-only service account ${dedicated_account}" \
      "PLAN require a distinct successful STG canary started after both audited trust removals and the disable event" \
      "No IAM settings changed. Prepare/finalize the dedicated identities and run STG first."
    exit 0
  fi

  for command in date gcloud gh git jq timeout; do
    command -v "$command" >/dev/null 2>&1 || {
      echo "$command is required" >&2
      exit 1
    }
  done

  repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
  release_workflow="$repo_root/.github/workflows/v3-release.yml"
  "$script_dir/verify-github-environments.sh" all all-controls

  project_number=$(gcloud projects describe "$project" --format='value(projectNumber)')
  [[ "$project_number" == "884236221349" ]] || {
    echo "numeric GCP project identity differs from the reviewed Pymes project" >&2
    exit 1
  }
  legacy_principal="principalSet://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/github-actions-pool/attribute.repository/devpablocristo/pymes"
fi

canonical_account_bindings() {
  jq -cS '
    [
      .bindings[]?
      | {
          role: .role,
          condition: (.condition // null),
          members: ((.members // []) | sort)
        }
    ]
    | sort_by(.role, (.condition | tostring), (.members | join(",")))
  '
}

assert_exact_release_account_policy() {
  local account="$1"
  shift
  local policy_json actual expected
  policy_json=$(gcloud iam service-accounts get-iam-policy "$account" \
    --project="$project" --format=json)
  actual=$(canonical_account_bindings <<<"$policy_json")
  expected=$(printf '%s\n' "$@" |
    jq -RscS '
      split("\n")
      | map(select(length > 0))
      | [{
          role: "roles/iam.workloadIdentityUser",
          condition: null,
          members: (sort)
        }]
    ')
  [[ "$actual" == "$expected" ]] || {
    echo "dedicated release account has unexpected callers or bindings: $account" >&2
    echo "expected: $expected" >&2
    echo "actual:   $actual" >&2
    exit 1
  }
}

service_account_disabled_state() {
  local account="$1" state
  state=$(gcloud iam service-accounts describe "$account" \
    --project="$project" --format='value(disabled)') || {
    echo "could not read service-account lifecycle state: $account" >&2
    return 1
  }
  case "$state" in
    True|False) printf '%s\n' "$state" ;;
    *)
      echo "service-account lifecycle state is missing or malformed for $account: ${state:-<empty>}" >&2
      return 1
      ;;
  esac
}

assert_service_account_enabled() {
  local account="$1" description="$2" state
  state=$(service_account_disabled_state "$account") || return
  [[ "$state" == "False" ]] || {
    echo "$description must be enabled: $account" >&2
    return 1
  }
}

assert_service_account_disabled() {
  local account="$1" description="$2" state
  state=$(service_account_disabled_state "$account") || return
  [[ "$state" == "True" ]] || {
    echo "$description must be disabled: $account" >&2
    return 1
  }
}

verify_release_foundation() {
  local pool_json provider_json account account_emails
  local build_subject stg_subject principal_prefix
  local expected_condition
  pool_json=$(gcloud iam workload-identity-pools describe "$pool" \
    --project="$project" --location=global --format=json)
  jq -e \
    --arg name "projects/${project_number}/locations/global/workloadIdentityPools/${pool}" '
      .name == $name and
      .state == "ACTIVE" and
      .disabled != true
    ' <<<"$pool_json" >/dev/null || {
    echo "dedicated Pymes release WIF pool differs from the reviewed active pool" >&2
    exit 1
  }

  expected_condition="assertion.repository_id=='${repository_id}' && assertion.repository_owner_id=='${repository_owner_id}' && assertion.repository=='${repository}' && assertion.ref=='refs/heads/main' && assertion.ref_protected=='true' && assertion.workflow_ref=='${workflow_ref}' && assertion.event_name=='workflow_dispatch' && (assertion.sub=='repo:${repository}:ref:refs/heads/main' || assertion.sub=='repo:${repository}:environment:stg' || assertion.sub=='repo:${repository}:environment:prd')"
  provider_json=$(gcloud iam workload-identity-pools providers describe "$provider" \
    --project="$project" --location=global \
    --workload-identity-pool="$pool" --format=json)
  jq -e \
    --arg name "projects/${project_number}/locations/global/workloadIdentityPools/${pool}/providers/${provider}" \
    --arg condition "$expected_condition" '
      .name == $name and
      .state == "ACTIVE" and
      .disabled != true and
      .oidc.issuerUri == "https://token.actions.githubusercontent.com" and
      ((.oidc.allowedAudiences // []) | length) == 0 and
      .attributeCondition == $condition and
      .attributeMapping == {
        "google.subject": "assertion.sub",
        "attribute.repository": "assertion.repository",
        "attribute.repository_id": "assertion.repository_id",
        "attribute.repository_owner_id": "assertion.repository_owner_id",
        "attribute.ref": "assertion.ref",
        "attribute.ref_protected": "assertion.ref_protected",
        "attribute.workflow_ref": "assertion.workflow_ref",
        "attribute.event_name": "assertion.event_name"
      }
    ' <<<"$provider_json" >/dev/null || {
    echo "dedicated Pymes release WIF provider differs from the reviewed exact policy" >&2
    exit 1
  }

  for account in \
    "pymes-v3-gh-build@${project}.iam.gserviceaccount.com" \
    "pymes-v3-gh-deploy-stg@${project}.iam.gserviceaccount.com"; do
    assert_service_account_enabled \
      "$account" "dedicated release service account"
  done
  account_emails=$(gcloud iam service-accounts list \
    --project="$project" --format='value(email)')
  assert_prd_release_identity_absent "$account_emails"

  principal_prefix="principal://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${pool}/subject"
  build_subject="${principal_prefix}/repo:${repository}:ref:refs/heads/main"
  stg_subject="${principal_prefix}/repo:${repository}:environment:stg"
  assert_exact_release_account_policy \
    "pymes-v3-gh-build@${project}.iam.gserviceaccount.com" "$build_subject"
  assert_exact_release_account_policy \
    "pymes-v3-gh-deploy-stg@${project}.iam.gserviceaccount.com" "$stg_subject"
}

assert_prd_release_identity_absent() {
  local account_emails="$1"
  local prd_email="pymes-v3-gh-deploy-prd@${project}.iam.gserviceaccount.com"

  if grep -Fqx -- "$prd_email" <<<"$account_emails"; then
    echo "PRD release identity was provisioned before the STG legacy-WIF retirement closed" >&2
    return 1
  fi
}

verify_reviewed_release_source() {
  local local_head local_tree local_status remote_tree

  local_status=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)
  [[ -z "$local_status" ]] || {
    echo "legacy WIF retirement requires a clean reviewed Pymes checkout" >&2
    exit 1
  }
  local_head=$(git -C "$repo_root" rev-parse HEAD)
  reviewed_main_sha=$(gh api "repos/${repository}/git/ref/heads/main" --jq '.object.sha')
  [[ "$local_head" =~ ^[0-9a-f]{40}$ &&
     "$reviewed_main_sha" == "$local_head" ]] || {
    echo "local Pymes HEAD must equal the exact current GitHub main SHA" >&2
    exit 1
  }
  local_tree=$(git -C "$repo_root" rev-parse "${local_head}^{tree}")
  remote_tree=$(gh api "repos/${repository}/git/commits/${reviewed_main_sha}" \
    --jq '.tree.sha')
  [[ "$local_tree" =~ ^[0-9a-f]{40}$ &&
     "$remote_tree" == "$local_tree" ]] || {
    echo "local release helpers do not equal the exact reviewed GitHub main tree" >&2
    exit 1
  }
  reviewed_tree_sha=$local_tree
}

assert_canary_source_identity() {
  local head_sha="$1" tree_sha="$2" workflow_blob="$3"
  local expected_workflow_blob="$4"

  [[ "$head_sha" == "$reviewed_main_sha" ]] || {
    echo "STG canary did not execute the exact reviewed current main SHA" >&2
    return 1
  }
  [[ "$tree_sha" == "$reviewed_tree_sha" ]] || {
    echo "STG canary did not execute the exact reviewed release helper tree" >&2
    return 1
  }
  [[ "$workflow_blob" == "$expected_workflow_blob" ]] || {
    echo "STG canary did not execute the exact reviewed release workflow blob" >&2
    return 1
  }
}

assert_operational_canary_title() {
  local display_title="$1" head_sha="$2"
  local expected_title="Pymes V3 stg operational @ ${head_sha}"

  [[ "$display_title" == "$expected_title" ]] || {
    echo "STG canary must be the operational release stage for the exact reviewed SHA" >&2
    return 1
  }
}

verify_canary_run() {
  local run_id="$1" label="$2"
  local run_json jobs_json workflow_json workflow_content_json commit_json
  local head_sha display_title workflow_id remote_workflow_blob remote_tree
  local main_workflow_blob local_workflow_blob started_at completed_at
  [[ "$run_id" =~ ^[1-9][0-9]*$ ]] || {
    echo "$label STG canary run ID must be a positive GitHub Actions run ID" >&2
    exit 2
  }
  run_json=$(gh api "repos/${repository}/actions/runs/${run_id}")
  head_sha=$(jq -r '.head_sha // ""' <<<"$run_json")
  [[ "$head_sha" =~ ^[0-9a-f]{40}$ ]] || {
    echo "$label STG canary has no immutable source SHA" >&2
    exit 1
  }

  workflow_json=$(gh api "repos/${repository}/actions/workflows/v3-release.yml")
  workflow_id=$(jq -r '.id // 0' <<<"$workflow_json")
  [[ "$workflow_id" =~ ^[1-9][0-9]*$ ]] || {
    echo "could not resolve the canonical Pymes v3 release workflow ID" >&2
    exit 1
  }
  jq -e --arg path "$workflow_path" '
    .path == $path and .state == "active"
  ' <<<"$workflow_json" >/dev/null || {
    echo "canonical Pymes v3 release workflow is not active at the expected path" >&2
    exit 1
  }

  workflow_content_json=$(gh api \
    "repos/${repository}/contents/${workflow_path}?ref=${head_sha}")
  remote_workflow_blob=$(jq -r '
    select(.type == "file" and .path == ".github/workflows/v3-release.yml")
    | .sha // ""
  ' <<<"$workflow_content_json")
  main_workflow_blob=$(gh api \
    "repos/${repository}/contents/${workflow_path}?ref=${reviewed_main_sha}" --jq '.sha')
  local_workflow_blob=$(git -C "$repo_root" hash-object "$release_workflow")
  commit_json=$(gh api "repos/${repository}/git/commits/${head_sha}")
  remote_tree=$(jq -r '.tree.sha // ""' <<<"$commit_json")
  [[ "$remote_workflow_blob" =~ ^[0-9a-f]{40}$ &&
     "$main_workflow_blob" == "$local_workflow_blob" ]] || {
    echo "$label STG canary release workflow blob cannot be proven locally and remotely" >&2
    exit 1
  }
  assert_canary_source_identity \
    "$head_sha" "$remote_tree" "$remote_workflow_blob" "$main_workflow_blob" || {
    echo "$label STG canary source identity differs from the reviewed release" >&2
    exit 1
  }

  started_at=$(jq -r '.run_started_at // ""' <<<"$run_json")
  completed_at=$(jq -r '.updated_at // ""' <<<"$run_json")
  date -u -d "$started_at" +%s >/dev/null 2>&1 || {
    echo "$label STG canary has no valid start timestamp" >&2
    exit 1
  }
  date -u -d "$completed_at" +%s >/dev/null 2>&1 || {
    echo "$label STG canary has no valid completion timestamp" >&2
    exit 1
  }
  display_title=$(jq -r '.display_title // ""' <<<"$run_json")
  assert_operational_canary_title "$display_title" "$head_sha" || {
    echo "$label run is not an operational STG release" >&2
    exit 1
  }
  jq -e \
    --arg repository "$repository" \
    --argjson repository_id "$repository_id" \
    --argjson workflow_id "$workflow_id" '
      .repository.full_name == $repository and
      .repository.id == $repository_id and
      .head_repository.full_name == $repository and
      .head_repository.id == $repository_id and
      .workflow_id == $workflow_id and
      .path == ".github/workflows/v3-release.yml" and
      .event == "workflow_dispatch" and
      .head_branch == "main" and
      .status == "completed" and
      .conclusion == "success"
    ' <<<"$run_json" >/dev/null || {
      echo "$label run is not an exact successful STG Pymes v3 release from main" >&2
      exit 1
  }
  jobs_json=$(gh api "repos/${repository}/actions/runs/${run_id}/jobs?per_page=100")
  jq -e '
    (.jobs | any(
      .name == "Validate immutable release" and
      .status == "completed" and
      .conclusion == "success" and
      any(.steps[]?; .name == "Verify complete protected GitHub controls" and .conclusion == "success") and
      any(.steps[]?; .name == "Validate protected release configuration" and .conclusion == "success") and
      any(.steps[]?; .name == "Validate release workflow policy" and .conclusion == "success")
    )) and
    (.jobs | any(
      .name == "Build and attest immutable images" and
      .status == "completed" and
      .conclusion == "success" and
      any(.steps[]?; .name == "Validate isolated build identity" and .conclusion == "success") and
      any(.steps[]?; .name == "Authenticate immutable image builder" and .conclusion == "success") and
      any(.steps[]?; .name == "Build and push release digests" and .conclusion == "success") and
      any(.steps[]?; .name == "Upload digest manifest" and .conclusion == "success")
    )) and
    (.jobs | any(
      .name == "Deploy and verify exact release" and
      .status == "completed" and
      .conclusion == "success" and
      any(.steps[]?; .name == "Validate isolated deploy identity" and .conclusion == "success") and
      any(.steps[]?; .name == "Authenticate least-privilege deployer" and .conclusion == "success") and
      any(.steps[]?; .name == "Verify provenance, materials, and SBOMs" and .conclusion == "success") and
      any(.steps[]?; .name == "Deploy exact image digests" and .conclusion == "success") and
      any(.steps[]?; .name == "Verify deployed release" and .conclusion == "success")
    ))
  ' <<<"$jobs_json" >/dev/null || {
    echo "$label STG canary did not build, deploy and verify successfully" >&2
    exit 1
  }
  jq -cn \
    --arg sha "$head_sha" \
    --arg tree_sha "$remote_tree" \
    --arg started_at "$started_at" \
    --arg completed_at "$completed_at" \
    '{sha:$sha, tree_sha:$tree_sha, started_at:$started_at, completed_at:$completed_at}'
}

principal_roles() {
  local account="$1" policy_json
  policy_json=$(gcloud iam service-accounts get-iam-policy "$account" \
    --project="$project" --format=json)
  jq -r --arg principal "$legacy_principal" '
    .bindings[]?
    | select((.members // []) | index($principal) != null)
    | .role
  ' <<<"$policy_json" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u
}

assert_legacy_binding_shape() {
  local account="$1" policy_json="$2"
  jq -e --arg principal "$legacy_principal" '
    [
      .bindings[]?
      | select((.members // []) | index($principal) != null)
    ] as $bindings
    | ($bindings | length) <= 1 and
      all(
        $bindings[];
        .role == "roles/iam.workloadIdentityUser" and
        (.condition == null) and
        ((.members // []) | map(select(type == "string")) | length) ==
          ((.members // []) | length)
      )
  ' <<<"$policy_json" >/dev/null || {
    echo "legacy principal has an unexpected role, condition or duplicate binding on $account" >&2
    exit 1
  }
}

assert_legacy_binding_present() {
  local account="$1" policy_json="$2"
  jq -e --arg principal "$legacy_principal" '
    [
      .bindings[]?
      | select(
          .role == "roles/iam.workloadIdentityUser" and
          (.condition // null) == null and
          ((.members // []) | index($principal) != null)
        )
    ] | length == 1
  ' <<<"$policy_json" >/dev/null || {
    echo "apply requires one real legacy WIF binding to remove from $account" >&2
    return 1
  }
}

assert_dedicated_account_has_no_other_callers() {
  local policy_json bindings
  policy_json=$(gcloud iam service-accounts get-iam-policy "$dedicated_account" \
    --project="$project" --format=json)
  bindings=$(canonical_account_bindings <<<"$policy_json")
  jq -e --arg principal "$legacy_principal" '
    length == 0 or
    (
      length == 1 and
      .[0].role == "roles/iam.workloadIdentityUser" and
      .[0].condition == null and
      .[0].members == [$principal]
    )
  ' <<<"$bindings" >/dev/null || {
    echo "refusing to disable the legacy Pymes account because it has non-Pymes callers or roles" >&2
    echo "actual dedicated-account bindings: $bindings" >&2
    exit 1
  }
}

direct_project_roles() {
  local account="$1"
  gcloud projects get-iam-policy "$project" \
    --flatten='bindings[].members' \
    --filter="bindings.members=serviceAccount:${account}" \
    --format='value(bindings.role)' |
    sed '/^[[:space:]]*$/d' |
    LC_ALL=C sort -u
}

member_roles() {
  local policy_json="$1" member="$2"
  jq -r --arg member "$member" '
    .bindings[]?
    | select((.members // []) | index($member) != null)
    | .role
  ' <<<"$policy_json" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u
}

assert_exact_roles() {
  local description="$1" actual="$2"
  shift 2
  local expected
  expected=$(printf '%s\n' "$@" |
    sed '/^[[:space:]]*$/d' |
    LC_ALL=C sort -u)
  [[ "$actual" == "$expected" ]] || {
    echo "$description has unexpected direct roles" >&2
    echo "expected:" >&2
    printf '%s\n' "$expected" >&2
    echo "actual:" >&2
    printf '%s\n' "$actual" >&2
    exit 1
  }
}

assert_policy_member_roles() {
  local description="$1" policy_json="$2" member="$3"
  shift 3
  local actual expected
  actual=$(jq -cS --arg member "$member" '
    [
      .bindings[]?
      | select((.members // []) | index($member) != null)
      | {
          role: .role,
          condition: (.condition // null)
        }
    ]
    | sort_by(.role, (.condition | tostring))
  ' <<<"$policy_json")
  expected=$(printf '%s\n' "$@" | jq -RscS '
    split("\n")
    | map(select(length > 0))
    | map({role: ., condition: null})
    | sort_by(.role)
  ')
  [[ "$actual" == "$expected" ]] || {
    echo "$description has unexpected roles or conditions" >&2
    echo "expected: $expected" >&2
    echo "actual:   $actual" >&2
    exit 1
  }
}

release_secret_names() {
  local environment="$1"
  printf '%s\n' \
    "pymes-v3-${environment}-clerk-secret-key" \
    "pymes-v3-${environment}-clerk-webhook-secret" \
    "pymes-v3-${environment}-scheduling-action-token-secret" \
    "pymes-v3-${environment}-pergo-api-key" \
    "pymes-v3-${environment}-pergo-webhook-secrets" \
    "pymes-v3-${environment}-google-client-secret" \
    "pymes-v3-${environment}-database-url" \
    "pymes-v3-${environment}-worker-database-url" \
    "pymes-v3-${environment}-migrate-database-url" \
    "pymes-v3-${environment}-fiscal-database-url" \
    "pymes-v3-${environment}-fiscal-migrate-database-url" \
    "pymes-v3-${environment}-accounting-database-url" \
    "pymes-v3-${environment}-accounting-admin-database-url" \
    "pymes-v3-${environment}-accounting-migrate-database-url"
}

assert_exact_new_release_authorization() {
  local build_email build_member repository_json role_json
  local environment deploy_email deploy_member policy_json project_policy_json
  local component secret release_pool_assets constraint org_policy_json
  local project_iam_read_role kms_policy_read_role
  local -a runtime_components service_components job_components

  build_email="pymes-v3-gh-build@${project}.iam.gserviceaccount.com"
  build_member="serviceAccount:${build_email}"
  project_iam_read_role="projects/${project}/roles/pymesV3ReleaseProjectIamRead"
  kms_policy_read_role="projects/${project}/roles/pymesV3ReleaseKmsPolicyRead"
  runtime_components=(
    api web worker provision fiscal accounting accounting-admin
    migrate fiscal-migrate acct-migrate
  )
  service_components=(api web worker fiscal accounting accounting-admin)
  job_components=(
    migrate fiscal-migrate accounting-migrate accounting-grants provision-org
  )

  project_policy_json=$(gcloud projects get-iam-policy "$project" --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$project_policy_json" "legacy-close project IAM"
  release_pool_assets=$(pymes_search_release_pool_iam_assets "$project")
  pymes_validate_release_pool_iam_assets \
    "$release_pool_assets" stg exact
  for constraint in \
    iam.disableCrossProjectServiceAccountUsage \
    iam.disableServiceAccountKeyCreation; do
    org_policy_json=$(gcloud org-policies describe "constraints/${constraint}" \
      --project="$project" --effective --format=json)
    pymes_validate_enforced_boolean_org_policy \
      "$org_policy_json" "$constraint"
  done
  assert_policy_member_roles \
    "dedicated build project IAM" \
    "$project_policy_json" "$build_member" \
    roles/serviceusage.serviceUsageConsumer
  pymes_verify_release_account_not_attached \
    "$project" "$build_email" "dedicated release builder"

  role_json=$(gcloud iam roles describe pymesV3ReleaseProjectIamRead \
    --project="$project" --format=json)
  jq -e \
    --argjson expected "$(pymes_release_project_iam_read_permissions_json)" '
    (
      .includedPermissions | sort
    ) == $expected and
    .deleted != true
  ' <<<"$role_json" >/dev/null || {
    echo "dedicated project-IAM reader role differs from the reviewed role" >&2
    exit 1
  }
  role_json=$(gcloud iam roles describe pymesV3ReleaseKmsPolicyRead \
    --project="$project" --format=json)
  jq -e \
    --argjson expected "$(pymes_release_kms_policy_read_permissions_json)" '
    (.includedPermissions | sort) == $expected and
    .deleted != true
  ' <<<"$role_json" >/dev/null || {
    echo "dedicated KMS policy reader role differs from the reviewed role" >&2
    exit 1
  }

  repository_json=$(gcloud artifacts repositories get-iam-policy pymes \
    --project="$project" --location="$region" --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$repository_json" "legacy-close Artifact Registry IAM"
  assert_policy_member_roles \
    "dedicated builder Artifact Registry IAM" \
    "$repository_json" "$build_member" roles/artifactregistry.writer

  for environment in stg; do
    deploy_email="pymes-v3-gh-deploy-${environment}@${project}.iam.gserviceaccount.com"
    deploy_member="serviceAccount:${deploy_email}"
    pymes_verify_release_account_not_attached \
      "$project" "$deploy_email" "dedicated ${environment} release deployer"
    assert_policy_member_roles \
      "dedicated ${environment} deploy project IAM" \
      "$project_policy_json" "$deploy_member" \
      roles/serviceusage.serviceUsageConsumer \
      "$project_iam_read_role"
    assert_policy_member_roles \
      "dedicated ${environment} deploy Artifact Registry IAM" \
      "$repository_json" "$deploy_member" roles/artifactregistry.reader

    policy_json=$(gcloud kms keyrings get-iam-policy "pymes-v3-${environment}" \
      --project="$project" --location="$region" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy_json" "legacy-close KMS key-ring IAM"
    assert_policy_member_roles \
      "dedicated ${environment} deploy KMS key-ring IAM" \
      "$policy_json" "$deploy_member" "$kms_policy_read_role"
    policy_json=$(gcloud kms keys get-iam-policy internal-jwt-signing \
      --project="$project" --location="$region" \
      --keyring="pymes-v3-${environment}" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy_json" "legacy-close KMS signing-key IAM"
    assert_policy_member_roles \
      "dedicated ${environment} deploy signing-key IAM" \
      "$policy_json" "$deploy_member" roles/cloudkms.publicKeyViewer

    for component in "${runtime_components[@]}"; do
      policy_json=$(gcloud iam service-accounts get-iam-policy \
        "pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com" \
        --project="$project" --format=json)
      pymes_assert_policy_has_no_release_pool_members \
        "$policy_json" "legacy-close runtime identity IAM"
      assert_policy_member_roles \
        "dedicated ${environment} deploy runtime actAs ${component}" \
        "$policy_json" "$deploy_member" roles/iam.serviceAccountUser
    done
    while IFS= read -r secret; do
      policy_json=$(gcloud secrets get-iam-policy "$secret" \
        --project="$project" --format=json)
      pymes_assert_policy_has_no_release_pool_members \
        "$policy_json" "legacy-close secret IAM"
      assert_policy_member_roles \
        "dedicated ${environment} deploy secret metadata ${secret}" \
        "$policy_json" "$deploy_member" roles/secretmanager.viewer
    done < <(release_secret_names "$environment")
  done

  deploy_email="pymes-v3-gh-deploy-stg@${project}.iam.gserviceaccount.com"
  deploy_member="serviceAccount:${deploy_email}"
  for component in "${service_components[@]}"; do
    policy_json=$(gcloud run services get-iam-policy \
      "pymes-v3-stg-${component}" \
      --project="$project" --region="$region" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy_json" "legacy-close Cloud Run service IAM"
    assert_policy_member_roles \
      "dedicated STG deploy Cloud Run service ${component}" \
      "$policy_json" "$deploy_member" roles/run.admin
  done
  for component in "${job_components[@]}"; do
    policy_json=$(gcloud run jobs get-iam-policy \
      "pymes-v3-stg-${component}" \
      --project="$project" --region="$region" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy_json" "legacy-close Cloud Run job IAM"
    assert_policy_member_roles \
      "dedicated STG deploy Cloud Run job ${component}" \
      "$policy_json" "$deploy_member" roles/run.admin
  done

  assert_new_identity_effective_authorization \
    "serviceAccount:${build_email}" build
  assert_new_identity_effective_authorization \
    "serviceAccount:pymes-v3-gh-deploy-stg@${project}.iam.gserviceaccount.com" \
    deploy stg
}

filter_broad_roles() {
  grep -E \
    '^(roles/(owner|editor|[^/]*(Admin|admin)|artifactregistry\.(writer|createOnPushWriter)|cloudkms\.(cryptoKeyEncrypterDecrypter|signerVerifier)|cloudsql\.client|iam\.(serviceAccountDeleter|serviceAccountOpenIdTokenCreator|serviceAccountTokenCreator|serviceAccountUser)|run\.(developer|invoker)|secretmanager\.secretAccessor)|projects/[^/]+/roles/|organizations/[^/]+/roles/)' ||
    true
}

assert_policy_has_no_group_bindings() {
  local description="$1" policy_json="$2"
  local group_bindings
  group_bindings=$(jq -cS '
    [
      .bindings[]?
      | . as $binding
      | ($binding.members // [])[]?
      | select(startswith("group:"))
      | {
          role: $binding.role,
          member: .,
          condition: ($binding.condition // null)
        }
    ]
  ' <<<"$policy_json")
  [[ "$group_bindings" == "[]" ]] || {
    echo "BLOCKED: ${description} contains group IAM bindings whose membership cannot be proven by project-scoped Policy Analyzer" >&2
    echo "$group_bindings" >&2
    return 1
  }
}

assert_shared_account_has_no_broad_authority() {
  local roles broad_roles ancestors_json ancestor_type ancestor_id
  local policy_json inherited_roles inherited_broad
  roles=$(direct_project_roles "$shared_account")
  broad_roles=$(filter_broad_roles <<<"$roles")
  if [[ -n "$broad_roles" ]]; then
    broad_roles=$(sed 's/^/project: /' <<<"$broad_roles")
  fi

  ancestors_json=$(gcloud projects get-ancestors "$project" --format=json)
  while IFS=$'\t' read -r ancestor_type ancestor_id; do
    case "$ancestor_type" in
      folder)
        policy_json=$(gcloud resource-manager folders get-iam-policy \
          "$ancestor_id" --format=json)
        ;;
      organization)
        policy_json=$(gcloud organizations get-iam-policy \
          "$ancestor_id" --format=json)
        ;;
      *)
        continue
        ;;
    esac
    assert_policy_has_no_group_bindings \
      "${ancestor_type}:${ancestor_id} IAM policy" "$policy_json"
    inherited_roles=$(jq -r --arg member "serviceAccount:${shared_account}" '
      .bindings[]?
      | select((.members // []) | index($member) != null)
      | .role
    ' <<<"$policy_json" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
    inherited_broad=$(filter_broad_roles <<<"$inherited_roles")
    if [[ -n "$inherited_broad" ]]; then
      broad_roles+=$'\n'
      broad_roles+="$(sed "s/^/${ancestor_type}:${ancestor_id}: /" <<<"$inherited_broad")"
    fi
  done < <(jq -r '.[] | [.type, .id] | @tsv' <<<"$ancestors_json")

  broad_roles=$(sed '/^[[:space:]]*$/d' <<<"$broad_roles")
  [[ -z "$broad_roles" ]] || {
    echo "BLOCKED: the shared CI account still has broad project or inherited authority that bypasses the dedicated Pymes release boundary" >&2
    echo "No shared-account permission was modified automatically. Scope or remove these roles in a separately reviewed cross-product change:" >&2
    printf '%s\n' "$broad_roles" >&2
    exit 1
  }
}

dangerous_permissions=(
  artifactregistry.repositories.uploadArtifacts
  artifactregistry.repositories.setIamPolicy
  artifactregistry.repositories.delete
  artifactregistry.versions.delete
  artifactregistry.tags.update
  run.services.create
  run.services.update
  run.services.delete
  run.services.setIamPolicy
  run.jobs.create
  run.jobs.update
  run.jobs.delete
  run.jobs.run
  run.jobs.runWithOverrides
  run.jobs.setIamPolicy
  run.routes.invoke
  iam.serviceAccounts.actAs
  iam.serviceAccounts.delete
  iam.serviceAccounts.disable
  iam.serviceAccounts.enable
  iam.serviceAccounts.getAccessToken
  iam.serviceAccounts.getOpenIdToken
  iam.serviceAccounts.signBlob
  iam.serviceAccounts.signJwt
  iam.serviceAccounts.setIamPolicy
  iam.serviceAccounts.undelete
  secretmanager.versions.access
  secretmanager.versions.add
  secretmanager.versions.destroy
  secretmanager.versions.disable
  secretmanager.secrets.setIamPolicy
  secretmanager.secrets.update
  secretmanager.secrets.delete
  cloudkms.cryptoKeyVersions.useToDecrypt
  cloudkms.cryptoKeyVersions.useToEncrypt
  cloudkms.cryptoKeyVersions.useToSign
  cloudkms.cryptoKeys.setIamPolicy
  cloudkms.cryptoKeys.update
  cloudkms.cryptoKeyVersions.destroy
  cloudsql.instances.connect
  cloudsql.instances.update
  cloudsql.instances.delete
  cloudsql.users.update
  resourcemanager.projects.setIamPolicy
  serviceusage.services.enable
  serviceusage.services.disable
)

policy_analysis_for_identity() {
  local identity="$1"
  local offset chunk_json permissions_csv
  local -a analyses=()
  for ((offset = 0; offset < ${#dangerous_permissions[@]}; offset += 10)); do
    permissions_csv=$(
      IFS=,
      printf '%s' "${dangerous_permissions[*]:offset:10}"
    )
    chunk_json=$(timeout 90s gcloud asset analyze-iam-policy \
      --project="$project" \
      --identity="$identity" \
      --permissions="$permissions_csv" \
      --expand-groups \
      --expand-resources \
      --expand-roles \
      --analyze-service-account-impersonation \
      --execution-timeout=60s \
      --show-response \
      --format=json) || return
    analyses+=("$chunk_json")
  done
  printf '%s\n' "${analyses[@]}" | jq -cs '
    {
      fullyExplored: all(.[]; .fullyExplored == true),
      mainAnalysis: {
        fullyExplored: all(.[]; .mainAnalysis.fullyExplored == true),
        analysisResults: [
          .[].mainAnalysis.analysisResults[]?
        ]
      },
      serviceAccountImpersonationAnalysis: [
        .[].serviceAccountImpersonationAnalysis[]?
      ]
    }
  '
}

assert_policy_analysis_complete() {
  local description="$1" analysis_json="$2"
  jq -e '
    .fullyExplored == true and
    .mainAnalysis.fullyExplored == true and
    all(.mainAnalysis.analysisResults[]?; .fullyExplored == true) and
    all(
      .serviceAccountImpersonationAnalysis[]?;
      .fullyExplored == true and
      all(.analysisResults[]?; .fullyExplored == true)
    )
  ' <<<"$analysis_json" >/dev/null || {
    echo "$description IAM Policy Analyzer result is incomplete" >&2
    exit 1
  }
}

shared_authority_findings() {
  local analysis_json="$1"
  jq -cS \
    --arg project "$project" \
    --arg number "$project_number" \
    --arg region "$region" '
    def protected_resource:
      . == ("//cloudresourcemanager.googleapis.com/projects/" + $project) or
      startswith("//artifactregistry.googleapis.com/projects/" + $project + "/locations/" + $region + "/repositories/pymes") or
      test("^//run\\.googleapis\\.com/projects/" + $project + "/locations/[^/]+/(services|jobs)/pymes-v3-(stg|prd)-") or
      test("^//iam\\.googleapis\\.com/projects/" + $project + "/serviceAccounts/pymes-v3-.*@(pymes-dev-352318)\\.iam\\.gserviceaccount\\.com$") or
      test("^//secretmanager\\.googleapis\\.com/projects/" + $number + "(/locations/" + $region + ")?/secrets/pymes-v3-(stg|prd)-") or
      test("^//cloudkms\\.googleapis\\.com/projects/" + $project + "/locations/" + $region + "/keyRings/pymes-v3-(stg|prd)(/|$)") or
      startswith("//sqladmin.googleapis.com/projects/" + $project + "/instances/pymes-dev-db");
    [
      (
        .mainAnalysis.analysisResults[]?,
        .serviceAccountImpersonationAnalysis[]?.analysisResults[]?
      ) as $result
      | $result.accessControlLists[]? as $acl
      | $acl.accesses[]?
      | select(.permission? != null)
      | .permission as $permission
      | (
          ($acl.resources // [])
          | map(.fullResourceName)
          | if length == 0 then [$result.attachedResourceFullName] else . end
        )[] as $resource
      | select(
          ($result.attachedResourceFullName | protected_resource) or
          ($resource | protected_resource)
        )
      | {
          attached_resource: $result.attachedResourceFullName,
          effective_resource: $resource,
          permission: $permission,
          role: ($result.iamBinding.role // ""),
          members: (($result.iamBinding.members // []) | sort)
        }
    ]
    | unique_by(.attached_resource, .effective_resource, .permission, .role)
  ' <<<"$analysis_json"
}

assert_shared_account_has_no_effective_pymes_authority() {
  local analysis_json findings
  analysis_json=$(policy_analysis_for_identity "serviceAccount:${shared_account}") || {
    echo "IAM Policy Analyzer failed for the shared CI account" >&2
    exit 1
  }
  assert_policy_analysis_complete "shared CI account" "$analysis_json"
  findings=$(shared_authority_findings "$analysis_json")
  jq -e 'length == 0' <<<"$findings" >/dev/null || {
    echo "BLOCKED: shared CI callers retain effective authority over Pymes resources" >&2
    jq -r '.[] | "\(.permission) via \(.role) on \(.attached_resource) -> \(.effective_resource)"' \
      <<<"$findings" >&2
    exit 1
  }
}

new_identity_authority_findings() {
  local analysis_json="$1" identity_kind="$2" environment="${3:-}"
  jq -cS \
    --arg kind "$identity_kind" \
    --arg environment "$environment" \
    --arg project "$project" \
    --arg region "$region" '
    def allowed_service:
      [
        "api", "web", "worker", "fiscal", "accounting", "accounting-admin"
      ] as $components
      | . as $resource
      | any(
          $components[];
          $resource == (
            "//run.googleapis.com/projects/" + $project +
            "/locations/" + $region + "/services/pymes-v3-" +
            $environment + "-" + .
          )
        );
    def allowed_job:
      [
        "migrate", "fiscal-migrate", "accounting-migrate",
        "accounting-grants", "provision-org"
      ] as $components
      | . as $resource
      | any(
          $components[];
          $resource == (
            "//run.googleapis.com/projects/" + $project +
            "/locations/" + $region + "/jobs/pymes-v3-" +
            $environment + "-" + .
          )
        );
    def allowed_runtime:
      [
        "api", "web", "worker", "provision", "fiscal", "accounting",
        "accounting-admin", "migrate", "fiscal-migrate", "acct-migrate"
      ] as $components
      | . as $resource
      | any(
          $components[];
          $resource == (
            "//iam.googleapis.com/projects/" + $project +
            "/serviceAccounts/pymes-v3-" + . + "-" + $environment +
            "@" + $project + ".iam.gserviceaccount.com"
          )
        );
    [
      (
        .mainAnalysis.analysisResults[]?,
        .serviceAccountImpersonationAnalysis[]?.analysisResults[]?
      ) as $result
      | $result.accessControlLists[]? as $acl
      | $acl.accesses[]?
      | select(.permission? != null)
      | .permission as $permission
      | select(
          if $kind == "build" then
            (
              (
                $permission == "artifactregistry.repositories.uploadArtifacts" or
                $permission == "artifactregistry.tags.update"
              ) and
              $result.attachedResourceFullName == (
                "//artifactregistry.googleapis.com/projects/" + $project +
                "/locations/" + $region + "/repositories/pymes"
              )
            ) | not
          else
            (
              (
                ($permission | startswith("run.services.")) and
                ($result.attachedResourceFullName | allowed_service)
              ) or
              (
                ($permission | startswith("run.jobs.")) and
                ($result.attachedResourceFullName | allowed_job)
              ) or
              (
                $permission == "iam.serviceAccounts.actAs" and
                ($result.attachedResourceFullName | allowed_runtime)
              )
            ) | not
          end
        )
      | {
          attached_resource: $result.attachedResourceFullName,
          permission: $permission,
          role: ($result.iamBinding.role // "")
        }
    ]
    | unique_by(.attached_resource, .permission, .role)
  ' <<<"$analysis_json"
}

assert_new_identity_effective_authorization() {
  local identity="$1" identity_kind="$2" environment="${3:-}"
  local analysis_json findings
  analysis_json=$(policy_analysis_for_identity "$identity") || {
    echo "IAM Policy Analyzer failed for dedicated release identity $identity" >&2
    exit 1
  }
  assert_policy_analysis_complete "dedicated release identity $identity" "$analysis_json"
  findings=$(new_identity_authority_findings \
    "$analysis_json" "$identity_kind" "$environment")
  jq -e 'length == 0' <<<"$findings" >/dev/null || {
    echo "dedicated release identity has effective authority outside its exact allowlist: $identity" >&2
    jq -r '.[] | "\(.permission) via \(.role) on \(.attached_resource)"' \
      <<<"$findings" >&2
    exit 1
  }
}

policy_without_legacy_principal() {
  jq -cS --arg principal "$legacy_principal" '
    [
      .bindings[]?
      | {
          role: .role,
          condition: (.condition // null),
          members: ((.members // []) | map(select(. != $principal)) | sort)
        }
      | select((.members | length) > 0)
    ]
    | sort_by(.role, (.condition | tostring), (.members | join(",")))
  '
}

legacy_assets() {
  gcloud asset search-all-iam-policies \
    --scope="projects/${project}" \
    --query="policy:\"${legacy_principal}\"" \
    --format=json
}

assert_no_unexpected_legacy_assets() {
  local assets_json="$1" unexpected
  local dedicated_resource shared_resource
  dedicated_resource="//iam.googleapis.com/projects/${project}/serviceAccounts/${dedicated_account}"
  shared_resource="//iam.googleapis.com/projects/${project}/serviceAccounts/${shared_account}"
  unexpected=$(jq -r \
    --arg dedicated "$dedicated_resource" \
    --arg shared "$shared_resource" '
      .[].resource
      | select(. != $dedicated and . != $shared)
    ' <<<"$assets_json" | LC_ALL=C sort -u)
  [[ -z "$unexpected" ]] || {
    echo "legacy Pymes WIF principal exists outside the two reviewed service accounts:" >&2
    printf '%s\n' "$unexpected" >&2
    exit 1
  }
}

api_is_enabled() {
  local service="$1" result
  result=$(gcloud services list \
    --project="$project" --enabled \
    --filter="config.name=${service}" \
    --format='value(config.name)') || {
    echo "could not establish whether ${service} is enabled" >&2
    return 2
  }
  case "$result" in
    "$service") return 0 ;;
    "") return 1 ;;
    *)
      echo "unexpected Service Usage result while checking ${service}: ${result}" >&2
      return 2
      ;;
  esac
}

run_if_api_enabled() {
  local service="$1"
  shift
  local status
  if api_is_enabled "$service"; then
    "$@"
    return
  else
    status=$?
  fi
  [[ "$status" -eq 1 ]] || {
    echo "service inventory failed closed for ${service}" >&2
    exit 1
  }
  echo "cannot prove absence of retained workloads while ${service} is disabled" >&2
  exit 1
}

assert_cloud_asset_has_no_workload_reference() {
  local inventory_json unexpected own_count
  inventory_json=$(gcloud asset search-all-resources \
    --scope="projects/${project}" \
    --query="$dedicated_account" \
    --read-mask='name,assetType,location,additionalAttributes' \
    --format=json) || {
    echo "Cloud Asset workload inventory failed" >&2
    exit 1
  }
  jq -e 'type == "array"' <<<"$inventory_json" >/dev/null || {
    echo "Cloud Asset workload inventory returned malformed JSON" >&2
    exit 1
  }
  own_count=$(jq -r --arg account "$dedicated_account" '
    [
      .[]?
      | select(
          .assetType == "iam.googleapis.com/ServiceAccount" and
          ((.name // "") | endswith("/serviceAccounts/" + $account))
        )
    ] | length
  ' <<<"$inventory_json")
  [[ "$own_count" == "1" ]] || {
    echo "Cloud Asset did not return exactly the dedicated service-account asset; workload inventory is not complete" >&2
    exit 1
  }
  unexpected=$(jq -cS --arg account "$dedicated_account" '
    [
      .[]?
      | select(
          (
            .assetType == "iam.googleapis.com/ServiceAccount" and
            (.additionalAttributes.email // "") == $account and
            ((.name // "") | endswith("/serviceAccounts/" + $account))
          ) | not
        )
      | {
          name: (.name // ""),
          asset_type: (.assetType // ""),
          location: (.location // "")
        }
    ]
  ' <<<"$inventory_json")
  jq -e 'length == 0' <<<"$unexpected" >/dev/null || {
    echo "refusing to disable the legacy account; Cloud Asset found workload references in one or more regions:" >&2
    jq -r '.[] | "\(.asset_type) \(.location) \(.name)"' <<<"$unexpected" >&2
    exit 1
  }
}

assert_inventory_has_no_account() {
  local kind="$1" description="$2" inventory_json="$3" attached
  jq -e '
    type == "array" and
    all(.[]; type == "object")
  ' <<<"$inventory_json" >/dev/null || {
    echo "${description} inventory returned malformed JSON" >&2
    return 1
  }
  attached=$(jq -r --arg account "$dedicated_account" --arg kind "$kind" '
    .[]?
    | select(
        if $kind == "run-service" then
          (.spec.template.spec.serviceAccountName // "") == $account
        elif $kind == "run-job" then
          (.spec.template.spec.template.spec.serviceAccountName // "") == $account
        elif $kind == "run-revision" then
          (
            .spec.serviceAccountName //
            .spec.template.spec.serviceAccountName //
            ""
          ) == $account
        elif $kind == "compute" then
          any(.serviceAccounts[]?; (.email // "") == $account)
        elif $kind == "function" then
          (
            .serviceConfig.serviceAccountEmail //
            .serviceAccountEmail //
            ""
          ) == $account
        elif $kind == "build-trigger" then
          (
            (.serviceAccount // "") == $account or
            ((.serviceAccount // "") | endswith("/serviceAccounts/" + $account))
          )
        elif $kind == "pubsub-subscription" then
          (
            (.pushConfig.oidcToken.serviceAccountEmail // "") == $account or
            (.bigqueryConfig.serviceAccountEmail // "") == $account or
            (.cloudStorageConfig.serviceAccountEmail // "") == $account
          )
        else
          error("unknown inventory kind")
        end
      )
    | (
        .name //
        .metadata.name //
        .id //
        .description //
        "<unnamed-resource>"
      )
  ' <<<"$inventory_json") || {
    echo "${description} inventory could not be evaluated" >&2
    return 1
  }
  [[ -z "$attached" ]] || {
    echo "refusing to disable the legacy account; it is referenced by $description:" >&2
    printf '%s\n' "$attached" >&2
    exit 1
  }
}

list_all_cloud_run_services() {
  CLOUDSDK_RUN_REGION= gcloud run services list \
    --project="$project" --format=json
}

list_all_cloud_run_jobs() {
  CLOUDSDK_RUN_REGION= gcloud run jobs list \
    --project="$project" --format=json
}

list_all_cloud_run_revisions() {
  CLOUDSDK_RUN_REGION= gcloud run revisions list \
    --project="$project" --format=json
}

assert_no_user_keys_or_workloads() {
  local user_keys inventory_json

  user_keys=$(gcloud iam service-accounts keys list \
    --iam-account="$dedicated_account" --project="$project" \
    --managed-by=user --format='value(name)') || {
    echo "could not inventory user-managed keys for the legacy account" >&2
    return 1
  }
  [[ -z "$user_keys" ]] || {
    echo "refusing to disable a legacy account with user-managed keys" >&2
    return 1
  }

  assert_cloud_asset_has_no_workload_reference || return

  inventory_json=$(list_all_cloud_run_services) || {
    echo "could not inventory Cloud Run services in every region" >&2
    return 1
  }
  assert_inventory_has_no_account \
    run-service "Cloud Run services in any region" "$inventory_json" || return
  inventory_json=$(list_all_cloud_run_jobs) || {
    echo "could not inventory Cloud Run jobs in every region" >&2
    return 1
  }
  assert_inventory_has_no_account \
    run-job "Cloud Run jobs in any region" "$inventory_json" || return
  inventory_json=$(list_all_cloud_run_revisions) || {
    echo "could not inventory historical Cloud Run revisions in every region" >&2
    return 1
  }
  assert_inventory_has_no_account \
    run-revision "Cloud Run revisions in any region" "$inventory_json" || return

  run_if_api_enabled compute.googleapis.com \
    inventory_compute_workloads || return
  run_if_api_enabled cloudfunctions.googleapis.com \
    inventory_function_workloads || return
  run_if_api_enabled cloudbuild.googleapis.com \
    inventory_build_trigger_workloads || return
  run_if_api_enabled pubsub.googleapis.com \
    inventory_pubsub_workloads || return
}

assert_dedicated_release_accounts_keyless() {
  local account user_keys
  for account in \
    "pymes-v3-gh-build@${project}.iam.gserviceaccount.com" \
    "pymes-v3-gh-deploy-stg@${project}.iam.gserviceaccount.com"; do
    user_keys=$(gcloud iam service-accounts keys list \
      --iam-account="$account" --project="$project" \
      --managed-by=user --format='value(name)') || {
      echo "could not revalidate user-managed keys for dedicated release account: $account" >&2
      return 1
    }
    [[ -z "$user_keys" ]] || {
      echo "dedicated release account gained a user-managed key before legacy-WIF closure: $account" >&2
      return 1
    }
    pymes_verify_release_account_not_attached \
      "$project" "$account" "dedicated release account $account" || return
  done
}

assert_post_removal_policy_unchanged() {
  local account="$1" expected_bindings="$2" policy_json actual_bindings
  policy_json=$(gcloud iam service-accounts get-iam-policy "$account" \
    --project="$project" --format=json) || {
    echo "could not re-read post-removal IAM policy for $account" >&2
    return 1
  }
  actual_bindings=$(canonical_account_bindings <<<"$policy_json") || {
    echo "post-removal IAM policy is malformed for $account" >&2
    return 1
  }
  [[ "$actual_bindings" == "$expected_bindings" ]] || {
    echo "service-account IAM changed after the reviewed legacy trust removal: $account" >&2
    echo "expected: $expected_bindings" >&2
    echo "actual:   $actual_bindings" >&2
    return 1
  }
}

assert_immediate_disable_preconditions() {
  local expected_dedicated_policy="$1" expected_shared_policy="$2"

  # The canary can take long enough for the release boundary or the legacy
  # accounts to change. Re-prove every security boundary after both trust
  # writes, then place direct key/workload reads immediately before disable.
  verify_release_foundation || return
  assert_exact_new_release_authorization || return
  assert_shared_account_has_no_broad_authority || return
  assert_shared_account_has_no_effective_pymes_authority || return
  assert_post_removal_policy_unchanged \
    "$dedicated_account" "$expected_dedicated_policy" || return
  assert_post_removal_policy_unchanged \
    "$shared_account" "$expected_shared_policy" || return
  assert_service_account_enabled \
    "$dedicated_account" "legacy Pymes-only service account" || return
  assert_service_account_enabled \
    "$shared_account" "shared service account" || return
  assert_dedicated_release_accounts_keyless || return
  assert_no_user_keys_or_workloads || return
}

inventory_compute_workloads() {
  local inventory_json
  inventory_json=$(gcloud compute instances list \
    --project="$project" --format=json)
  assert_inventory_has_no_account \
    compute "Compute Engine and GKE instances" "$inventory_json"
}

inventory_function_workloads() {
  local inventory_json
  inventory_json=$(gcloud functions list \
    --project="$project" --regions=- --format=json)
  assert_inventory_has_no_account \
    function "Cloud Functions in any region" "$inventory_json"
}

inventory_build_trigger_workloads() {
  local inventory_json
  # Cloud Asset above is the all-location source of truth. These direct reads
  # add schema-specific defense in depth for the two locations Pymes uses.
  inventory_json=$(gcloud builds triggers list \
    --project="$project" --region=global --format=json)
  assert_inventory_has_no_account \
    build-trigger "Cloud Build triggers in global" "$inventory_json"
  inventory_json=$(gcloud builds triggers list \
    --project="$project" --region="$region" --format=json)
  assert_inventory_has_no_account \
    build-trigger "Cloud Build triggers in $region" "$inventory_json"
}

inventory_pubsub_workloads() {
  local inventory_json
  inventory_json=$(gcloud pubsub subscriptions list \
    --project="$project" --format=json)
  assert_inventory_has_no_account \
    pubsub-subscription "Pub/Sub subscriptions" "$inventory_json"
}

assert_asset_inventory_retired() {
  local attempt assets_json
  for attempt in {1..10}; do
    assets_json=$(legacy_assets)
    if jq -e 'length == 0' <<<"$assets_json" >/dev/null; then
      return
    fi
    [[ "$attempt" == "10" ]] || sleep 3
  done
  echo "Cloud Asset still finds legacy Pymes WIF trust after retirement" >&2
  jq -r '.[].resource' <<<"$assets_json" >&2
  exit 1
}

retirement_audit_events() {
  local after_timestamp=${1:-}
  local filter
  filter="logName=\"projects/${project}/logs/cloudaudit.googleapis.com%2Factivity\" AND resource.type=\"service_account\" AND protoPayload.serviceName=\"iam.googleapis.com\" AND (resource.labels.email_id=\"${dedicated_account}\" OR resource.labels.email_id=\"${shared_account}\") AND (protoPayload.methodName=\"google.iam.admin.v1.SetIAMPolicy\" OR protoPayload.methodName=\"google.iam.admin.v1.DisableServiceAccount\" OR protoPayload.methodName=\"google.iam.admin.v1.EnableServiceAccount\")"
  if [[ -n "$after_timestamp" ]]; then
    filter="${filter} AND timestamp>=\"${after_timestamp}\""
  fi
  timeout 20s gcloud logging read "$filter" \
    --project="$project" \
    --freshness=90d \
    --order=desc \
    --limit=200 \
    --format=json
}

build_retirement_marker() {
  local events_json="$1" marker
  marker=$(jq -cS \
    --arg dedicated "$dedicated_account" \
    --arg shared "$shared_account" \
    --arg principal "$legacy_principal" '
    def base_event:
      select(
        ((.protoPayload.status.code // 0) == 0) and
        ((.timestamp // .receiveTimestamp // "") | length > 0) and
        ((.insertId // "") | length > 0) and
        ((.protoPayload.authenticationInfo.principalEmail // "") | length > 0)
      );
    def policy_event($account):
      [
        .[]?
        | base_event
        | select(
            (.resource.labels.email_id // "") == $account and
            (.protoPayload.methodName // "") ==
              "google.iam.admin.v1.SetIAMPolicy" and
            ((.protoPayload.request.resource // "") |
              endswith("/serviceAccounts/" + $account)) and
            ((.protoPayload.request.policy // null) | type) == "object" and
            (((.protoPayload.request.policy.bindings // []) | type) == "array") and
            any(
              (
                .protoPayload.serviceData.policyDelta.bindingDeltas //
                .protoPayload.metadata.policyDelta.bindingDeltas //
                []
              )[];
              (.action // "") == "REMOVE" and
              (.role // "") == "roles/iam.workloadIdentityUser" and
              (.member // "") == $principal
            ) and
            all(
              .protoPayload.request.policy.bindings[]?.members[]?;
              . != $principal
            )
          )
        | {
            timestamp: (.timestamp // .receiveTimestamp),
            insert_id: .insertId,
            actor: .protoPayload.authenticationInfo.principalEmail,
            request_etag: (.protoPayload.request.policy.etag // "")
          }
      ]
      | sort_by(.timestamp)
      | last // null;
    def lifecycle_event:
      [
        .[]?
        | base_event
        | select(
            (.resource.labels.email_id // "") == $dedicated and
            (
              (.protoPayload.methodName // "") ==
                "google.iam.admin.v1.DisableServiceAccount" or
              (.protoPayload.methodName // "") ==
                "google.iam.admin.v1.EnableServiceAccount"
            )
          )
        | {
            timestamp: (.timestamp // .receiveTimestamp),
            insert_id: .insertId,
            actor: .protoPayload.authenticationInfo.principalEmail,
            method: .protoPayload.methodName
          }
      ]
      | sort_by(.timestamp)
      | last // null;
    {
      dedicated_policy: policy_event($dedicated),
      shared_policy: policy_event($shared),
      lifecycle: lifecycle_event
    }
  ' <<<"$events_json")
  jq -e '
    .dedicated_policy != null and
    .shared_policy != null and
    .lifecycle != null and
    .lifecycle.method == "google.iam.admin.v1.DisableServiceAccount" and
    .dedicated_policy.actor == .shared_policy.actor and
    .dedicated_policy.actor == .lifecycle.actor
  ' <<<"$marker" >/dev/null || {
    echo "Cloud Audit Logs do not prove both trust removals and the dedicated-account disable by one actor" >&2
    return 1
  }

  local dedicated_nanoseconds shared_nanoseconds lifecycle_nanoseconds
  dedicated_nanoseconds=$(date -u -d \
    "$(jq -r '.dedicated_policy.timestamp' <<<"$marker")" +%s%N)
  shared_nanoseconds=$(date -u -d \
    "$(jq -r '.shared_policy.timestamp' <<<"$marker")" +%s%N)
  lifecycle_nanoseconds=$(date -u -d \
    "$(jq -r '.lifecycle.timestamp' <<<"$marker")" +%s%N)
  (( dedicated_nanoseconds <= lifecycle_nanoseconds &&
     shared_nanoseconds <= lifecycle_nanoseconds )) || {
    echo "durable disable marker precedes one of the audited trust removals" >&2
    return 1
  }
  jq -cS '
    {
      timestamp: .lifecycle.timestamp,
      actor: .lifecycle.actor,
      dedicated_policy_event: .dedicated_policy.insert_id,
      shared_policy_event: .shared_policy.insert_id,
      disable_event: .lifecycle.insert_id
    }
  ' <<<"$marker"
}

latest_retirement_marker() {
  local after_timestamp=${1:-}
  local events_json
  events_json=$(retirement_audit_events "$after_timestamp") || {
    echo "Cloud Audit Logs cutover query failed or exceeded 20 seconds" >&2
    exit 1
  }
  build_retirement_marker "$events_json"
}

wait_for_retirement_marker() {
  local requested_after="$1" attempt marker
  for attempt in {1..4}; do
    if marker=$(latest_retirement_marker "$requested_after" 2>/dev/null); then
      printf '%s\n' "$marker"
      return
    fi
    [[ "$attempt" == "4" ]] || sleep 5
  done
  echo "Cloud Audit Logs did not expose both policy removals and the disable marker" >&2
  exit 1
}

assert_retired() {
  local account roles
  for account in "$dedicated_account" "$shared_account"; do
    roles=$(principal_roles "$account")
    [[ -z "$roles" ]] || {
      echo "legacy Pymes WIF principal is still trusted by $account:" >&2
      printf '%s\n' "$roles" >&2
      exit 1
    }
  done

  assert_service_account_disabled \
    "$dedicated_account" "legacy Pymes-only service account"
  assert_service_account_enabled \
    "$shared_account" "shared service account"
  # Re-read key inventories at the closure boundary. The earlier foundation
  # verification is intentionally insufficient because a key could be added
  # while canaries or the retirement mutation are running.
  assert_dedicated_release_accounts_keyless
  assert_no_user_keys_or_workloads
  assert_asset_inventory_retired
}

if [[ "$source_only" != "true" ]]; then
  verify_reviewed_release_source
  verify_release_foundation
  assert_exact_new_release_authorization
  assert_shared_account_has_no_broad_authority
  assert_shared_account_has_no_effective_pymes_authority
  assert_dedicated_account_has_no_other_callers
  pre_canary_result=$(verify_canary_run "$pre_canary_run_id" "pre-retirement")
  pre_canary_sha=$(jq -r '.sha' <<<"$pre_canary_result")

  if [[ "$mode" == "audit" ]]; then
    [[ "$post_canary_run_id" =~ ^[1-9][0-9]*$ &&
       "$post_canary_run_id" != "$pre_canary_run_id" ]] || {
      echo "audit requires a distinct PYMES_STG_POST_RETIRE_CANARY_RUN_ID" >&2
      exit 2
    }
    post_canary_result=$(verify_canary_run \
      "$post_canary_run_id" "post-retirement")
    post_canary_sha=$(jq -r '.sha' <<<"$post_canary_result")
    [[ "$post_canary_sha" == "$pre_canary_sha" &&
       "$post_canary_sha" == "$reviewed_main_sha" ]] || {
      echo "pre/post retirement canaries must use the same exact reviewed main SHA" >&2
      exit 1
    }
    pre_completed_at=$(jq -r '.completed_at' <<<"$pre_canary_result")
    post_started_at=$(jq -r '.started_at' <<<"$post_canary_result")
    retirement_marker=$(latest_retirement_marker "$pre_completed_at")
    retirement_timestamp=$(jq -r '.timestamp' <<<"$retirement_marker")
    retirement_nanoseconds=$(date -u -d "$retirement_timestamp" +%s%N)
    pre_completed_nanoseconds=$(date -u -d "$pre_completed_at" +%s%N)
    post_started_nanoseconds=$(date -u -d "$post_started_at" +%s%N)
    (( pre_completed_nanoseconds < retirement_nanoseconds )) || {
      echo "pre-retirement canary did not complete before the durable retirement marker" >&2
      exit 1
    }
    (( post_started_nanoseconds > retirement_nanoseconds )) || {
      echo "post-retirement canary did not start after both trust removals and account disable" >&2
      exit 1
    }
    assert_retired
    echo "Legacy Pymes WIF retired at ${retirement_timestamp}; exact STG release ${post_canary_sha} passed before and after the audited cutover"
    exit 0
  fi

  assert_no_user_keys_or_workloads
  assets_json=$(legacy_assets)
  assert_no_unexpected_legacy_assets "$assets_json"

  tmp_dir=$(mktemp -d)
  cleanup() {
    rm -rf -- "$tmp_dir"
  }
  trap cleanup EXIT INT TERM

  declare -A preserved_policies=()
  declare -A original_policies=()
  assert_service_account_enabled \
    "$dedicated_account" \
    "apply legacy account (DisableServiceAccount must be a real lifecycle transition)"
  assert_service_account_enabled \
    "$shared_account" "shared service account"
  for account in "$dedicated_account" "$shared_account"; do
    policy_json=$(gcloud iam service-accounts get-iam-policy "$account" \
      --project="$project" --format=json)
    original_policies["$account"]="$policy_json"
    assert_legacy_binding_shape "$account" "$policy_json"
    assert_legacy_binding_present "$account" "$policy_json"
    preserved_policies["$account"]=$(
      policy_without_legacy_principal <<<"$policy_json"
    )
  done

  retirement_requested_after=$(date -u +%Y-%m-%dT%H:%M:%S.%NZ)
  for account in "$dedicated_account" "$shared_account"; do
    policy_file="$tmp_dir/$(tr '@.' '__' <<<"$account").json"
    jq --arg principal "$legacy_principal" '
      .bindings = [
        .bindings[]?
        | .members = ((.members // []) | map(select(. != $principal)))
        | select((.members | length) > 0)
      ]
    ' <<<"${original_policies[$account]}" >"$policy_file"
    # Always write both reviewed policies. Besides making partial retries
    # deterministic, this creates the two durable SetIAMPolicy audit events
    # that bind the later post-canary to the complete trust cutover.
    gcloud iam service-accounts set-iam-policy "$account" "$policy_file" \
      --project="$project" --quiet >/dev/null
  done

  assert_immediate_disable_preconditions \
    "${preserved_policies[$dedicated_account]}" \
    "${preserved_policies[$shared_account]}"
  gcloud iam service-accounts disable "$dedicated_account" \
    --project="$project" --quiet

  for account in "$dedicated_account" "$shared_account"; do
    policy_json=$(gcloud iam service-accounts get-iam-policy "$account" \
      --project="$project" --format=json)
    actual_preserved=$(policy_without_legacy_principal <<<"$policy_json")
    [[ "$actual_preserved" == "${preserved_policies[$account]}" ]] || {
      echo "non-Pymes IAM members changed while retiring $account" >&2
      exit 1
    }
  done

  assert_retired
  retirement_marker=$(wait_for_retirement_marker "$retirement_requested_after")
  retirement_timestamp=$(jq -r '.timestamp' <<<"$retirement_marker")
  echo "Legacy Pymes WIF trust retired at ${retirement_timestamp}; run a second exact-SHA STG canary and then mode=audit"
fi
