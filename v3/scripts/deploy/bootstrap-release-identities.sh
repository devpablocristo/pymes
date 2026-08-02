#!/usr/bin/env bash
set -euo pipefail

# Creates a dedicated GitHub OIDC trust boundary and three release identities:
# one immutable builder plus one environment-scoped deployer for STG and PRD.
# It never reuses the project-wide/shared CI identity. The default is a
# read-only plan; setting PYMES_RELEASE_IDENTITY_APPLY=true is explicit consent
# to mutate IAM.

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
artifact_repository=${PYMES_ARTIFACT_REPOSITORY:-pymes}
expected_project=pymes-dev-352318
expected_project_number=884236221349
expected_region=us-central1
expected_artifact_repository=pymes
github_repository=devpablocristo/pymes
github_repository_id=1173650578
github_repository_owner_id=81805584
github_workflow_ref='devpablocristo/pymes/.github/workflows/v3-release.yml@refs/heads/main'
github_build_subject='repo:devpablocristo/pymes:ref:refs/heads/main'
github_stg_subject='repo:devpablocristo/pymes:environment:stg'
github_prd_subject='repo:devpablocristo/pymes:environment:prd'
pool=pymes-v3-release-pool
provider=github
apply=${PYMES_RELEASE_IDENTITY_APPLY:-false}
phase=${PYMES_RELEASE_IDENTITY_PHASE:-prepare}
target_environment=${PYMES_RELEASE_IDENTITY_ENV:-}
initial_seed_release_sha=${PYMES_INITIAL_SEED_RELEASE_SHA:-}
initial_seed_operator_email=${PYMES_INITIAL_SEED_OPERATOR_EMAIL:-}
initial_seed_started_at=${PYMES_INITIAL_SEED_STARTED_AT:-}
initial_seed_completed_at=${PYMES_INITIAL_SEED_COMPLETED_AT:-}
initial_seed_manifest=${PYMES_INITIAL_SEED_MANIFEST:-}
initial_seed_manifest_sha256=${PYMES_INITIAL_SEED_MANIFEST_SHA256:-}
release_identity_operator_email=${PYMES_RELEASE_IDENTITY_OPERATOR_EMAIL:-}
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=initial-seed-audit-bounds.sh
source "$script_dir/initial-seed-audit-bounds.sh"
# shellcheck source=release-authority-policy.sh
source "$script_dir/release-authority-policy.sh"
organization_iam_read_role="organizations/${pymes_release_expected_organization}/roles/pymesV3ReleaseOrganizationIamRead"
folder_iam_read_role="organizations/${pymes_release_expected_organization}/roles/pymesV3ReleaseFolderIamRead"

assert_direct_gcloud_auth() {
  local variable property value
  for variable in \
    CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT \
    CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE \
    CLOUDSDK_AUTH_ACCESS_TOKEN_FILE \
    CLOUDSDK_AUTH_LOGIN_CONFIG_FILE; do
    [[ -z "${!variable-}" ]] || {
      echo "release identity bootstrap forbids delegated or overridden gcloud credentials: $variable" >&2
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
      echo "release identity bootstrap forbids delegated or overridden gcloud credentials: $property" >&2
      return 1
    }
  done
}

verify_reviewed_release_source() {
  local repo_root local_head local_tree local_status remote_head remote_tree
  local repository_json active_account
  repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
  local_status=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)
  local_head=$(git -C "$repo_root" rev-parse HEAD)
  [[ "$(git -C "$repo_root" branch --show-current)" == "main" &&
     "$local_head" =~ ^[0-9a-f]{40}$ &&
     -z "$local_status" ]] || {
    echo "IAM mutation requires a clean Pymes main checkout" >&2
    exit 1
  }
  repository_json=$(gh api "repos/${github_repository}")
  jq -e \
    --arg repository "$github_repository" \
    --argjson repository_id "$github_repository_id" '
      .full_name == $repository and .id == $repository_id
    ' <<<"$repository_json" >/dev/null || {
    echo "GitHub repository identity differs from the reviewed Pymes repository" >&2
    exit 1
  }
  remote_head=$(gh api "repos/${github_repository}/git/ref/heads/main" \
    --jq '.object.sha')
  [[ "$remote_head" == "$local_head" ]] || {
    echo "local Pymes HEAD must equal the exact current GitHub main SHA" >&2
    exit 1
  }
  local_tree=$(git -C "$repo_root" rev-parse "${local_head}^{tree}")
  remote_tree=$(gh api "repos/${github_repository}/git/commits/${remote_head}" \
    --jq '.tree.sha')
  [[ "$local_tree" =~ ^[0-9a-f]{40}$ && "$remote_tree" == "$local_tree" ]] || {
    echo "local release helpers must equal the exact reviewed GitHub main tree" >&2
    exit 1
  }
  active_account=$(gcloud auth list \
    --filter=status:ACTIVE --format='value(account)')
  [[ "$release_identity_operator_email" == "softponti@gmail.com" &&
     "$active_account" == "$release_identity_operator_email" ]] || {
    echo "IAM mutation is restricted to the reviewed direct operator softponti@gmail.com" >&2
    exit 1
  }
  assert_direct_gcloud_auth
  echo "Reviewed IAM mutation source and operator verified: sha=${local_head} operator=${active_account}"
}

case "$apply" in
  true|false) ;;
  *) echo "PYMES_RELEASE_IDENTITY_APPLY must be true or false" >&2; exit 2 ;;
esac
case "$phase" in
  prepare|finalize|close) ;;
  *) echo "PYMES_RELEASE_IDENTITY_PHASE must be prepare, finalize or close" >&2; exit 2 ;;
esac
case "$phase" in
  prepare|finalize)
    case "$target_environment" in
      stg|prd) managed_environments=("$target_environment") ;;
      *) echo "$phase requires PYMES_RELEASE_IDENTITY_ENV=stg or prd" >&2; exit 2 ;;
    esac
    if [[ "$phase" == "finalize" && "$apply" == "true" ]]; then
      [[ "$initial_seed_release_sha" =~ ^[0-9a-f]{40}$ ]] || {
        echo "finalize requires PYMES_INITIAL_SEED_RELEASE_SHA from the reviewed human bootstrap" >&2
        exit 2
      }
      [[ "$initial_seed_operator_email" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]] || {
        echo "finalize requires PYMES_INITIAL_SEED_OPERATOR_EMAIL for Cloud Audit Logs evidence" >&2
        exit 2
      }
      [[ "$initial_seed_started_at" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] &&
        date -u -d "$initial_seed_started_at" +%s >/dev/null 2>&1 || {
        echo "finalize requires strict UTC RFC3339 PYMES_INITIAL_SEED_STARTED_AT for Cloud Audit Logs evidence" >&2
        exit 2
      }
      [[ "$initial_seed_completed_at" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] &&
        date -u -d "$initial_seed_completed_at" +%s >/dev/null 2>&1 || {
        echo "finalize requires strict UTC RFC3339 PYMES_INITIAL_SEED_COMPLETED_AT for a bounded Cloud Audit Logs window" >&2
        exit 2
      }
      [[ -f "$initial_seed_manifest" && ! -L "$initial_seed_manifest" ]] || {
        echo "finalize requires the regular immutable PYMES_INITIAL_SEED_MANIFEST used by the seed" >&2
        exit 2
      }
      [[ "$initial_seed_manifest_sha256" =~ ^[0-9a-f]{64}$ &&
         "$(sha256sum "$initial_seed_manifest" | awk '{print $1}')" == "$initial_seed_manifest_sha256" ]] || {
        echo "PYMES_INITIAL_SEED_MANIFEST_SHA256 does not match the immutable seed manifest" >&2
        exit 2
      }
    fi
    ;;
  close)
    [[ -z "$target_environment" || "$target_environment" == "all" ]] || {
      echo "close does not accept an environment target" >&2
      exit 2
    }
    target_environment=all
    managed_environments=()
    ;;
esac

build_accounts=(
  pymes-v3-gh-build
)
deploy_accounts=()
for environment in "${managed_environments[@]}"; do
  deploy_accounts+=("pymes-v3-gh-deploy-${environment}")
done

if [[ "$apply" == "false" ]]; then
  case "$phase:$target_environment" in
    prepare:stg|finalize:stg)
      plan_next_steps="Prepare STG without project Run Admin; the reviewed existing project Owner seeds the exact zero-traffic resources from a clean main checkout without receiving any new grant; audit that pre-existing authority; finalize resource-scoped IAM; then run the protected WIF bootstrap and operational canaries. The PRD deployer must not exist before close."
      ;;
    prepare:prd|finalize:prd)
      plan_next_steps="PRD is permitted only after the audited legacy-WIF close. Then prepare PRD without project Run Admin, perform and audit the reviewed human zero-traffic seed, finalize resource-scoped IAM, and run protected bootstrap before operational promotion."
      ;;
    close:all)
      plan_next_steps="Verify the post-retirement STG canary and apply close. Only after close may the PRD deployer be created."
      ;;
  esac
  cat <<EOF
PLAN project=${project}
PLAN WIF pool=${pool} provider=${provider}
PLAN trust repository=${github_repository} repository_id=${github_repository_id} owner_id=${github_repository_owner_id}
PLAN trust workflow_ref=${github_workflow_ref} event=workflow_dispatch ref=refs/heads/main ref_protected=true
PLAN build subject=${github_build_subject}
PLAN deploy subjects=${github_stg_subject},${github_prd_subject}
PLAN build identities=${build_accounts[*]}
PLAN deploy identities=${deploy_accounts[*]}
PLAN Artifact Registry repository=${region}/${artifact_repository}
PLAN phase=${phase} environment=${target_environment}
No IAM changes made. ${plan_next_steps}
EOF
  exit 0
fi

"$script_dir/verify-github-environments.sh" "$target_environment" all-controls

if [[ "$phase" == "close" ]]; then
  PYMES_GCP_PROJECT="$project" \
    PYMES_LEGACY_WIF_MODE=audit \
    "$script_dir/retire-legacy-pymes-wif.sh"
  echo "Pymes v3 release identity closure verified; no GCP settings changed"
  exit 0
fi

for command in date gcloud gh git jq; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "$command is required" >&2
    exit 1
  }
done
[[ "$project" == "$expected_project" ]] || {
  echo "release identity bootstrap is restricted to $expected_project" >&2
  exit 2
}
[[ "$region" == "$expected_region" ]] || {
  echo "release identity bootstrap is restricted to $expected_region" >&2
  exit 2
}
[[ "$artifact_repository" == "$expected_artifact_repository" ]] || {
  echo "release identity bootstrap is restricted to Artifact Registry repository $expected_artifact_repository" >&2
  exit 2
}
configured_project=$(gcloud config get-value project 2>/dev/null || true)
[[ "$configured_project" == "$expected_project" ]] || {
  echo "active gcloud project must be $expected_project before IAM bootstrap" >&2
  exit 1
}
project_json=$(gcloud projects describe "$project" --format=json)
jq -e \
  --arg project "$expected_project" \
  --arg number "$expected_project_number" '
    .projectId == $project and
    (.projectNumber | tostring) == $number and
    .lifecycleState == "ACTIVE"
  ' <<<"$project_json" >/dev/null || {
    echo "GCP project identity differs from the reviewed Pymes project" >&2
    exit 1
  }
project_number=$(jq -r '.projectNumber | tostring' <<<"$project_json")
repository_json=$(gcloud artifacts repositories describe "$artifact_repository" \
  --project="$project" --location="$region" --format=json)
jq -e \
  --arg name "projects/${expected_project}/locations/${expected_region}/repositories/${expected_artifact_repository}" '
    .name == $name and
    .format == "DOCKER" and
    (.mode // "STANDARD_REPOSITORY") == "STANDARD_REPOSITORY"
  ' <<<"$repository_json" >/dev/null || {
    echo "Artifact Registry identity, region or format differs from the reviewed release repository" >&2
    exit 1
  }
echo "GCP mutation preflight verified: project=${project} number=${project_number} region=${region} repository=${artifact_repository}"
verify_reviewed_release_source

if [[ "$phase" == "prepare" && "$target_environment" == "prd" ]]; then
  PYMES_GCP_PROJECT="$project" \
    PYMES_LEGACY_WIF_MODE=audit \
    "$script_dir/retire-legacy-pymes-wif.sh"
  echo "Audited STG cutover and legacy-WIF close verified before creating the PRD deployer"
fi

export CLOUDSDK_CORE_PROJECT="$project"
required_services=(
  artifactregistry.googleapis.com
  cloudasset.googleapis.com
  iam.googleapis.com
  iamcredentials.googleapis.com
  orgpolicy.googleapis.com
  run.googleapis.com
  sts.googleapis.com
)
enabled_services=$(gcloud services list --enabled \
  --project="$project" --format='value(config.name)')
missing_services=()
for required_service in "${required_services[@]}"; do
  grep -Fxq "$required_service" <<<"$enabled_services" ||
    missing_services+=("$required_service")
done
if ((${#missing_services[@]} > 0)); then
  if [[ "$phase" == "prepare" ]]; then
    gcloud services enable "${missing_services[@]}" \
      --project="$project" >/dev/null
  else
    printf 'finalize refuses to enable missing API: %s\n' \
      "${missing_services[@]}" >&2
    exit 1
  fi
fi

for constraint in \
  iam.disableCrossProjectServiceAccountUsage \
  iam.disableServiceAccountKeyCreation; do
  org_policy_json=
  org_policy_valid=false
  for attempt in 1 2 3 4 5 6; do
    if org_policy_json=$(gcloud org-policies describe \
        "constraints/${constraint}" --project="$project" \
        --effective --format=json 2>/dev/null) &&
      pymes_validate_enforced_boolean_org_policy \
        "$org_policy_json" "$constraint" 2>/dev/null; then
      org_policy_valid=true
      break
    fi
    [[ "$attempt" -eq 6 ]] || sleep 5
  done
  [[ "$org_policy_valid" == "true" ]] || {
    pymes_validate_enforced_boolean_org_policy \
      "$org_policy_json" "$constraint"
    exit 1
  }
done

release_pool_assets=$(pymes_search_release_pool_iam_assets "$project")
pymes_validate_release_pool_iam_assets \
  "$release_pool_assets" "$target_environment" subset
project_policy_json=$(gcloud projects get-iam-policy "$project" --format=json)
pymes_assert_policy_has_no_release_pool_members \
  "$project_policy_json" "release project IAM preflight"
repository_policy_json=$(gcloud artifacts repositories get-iam-policy \
  "$artifact_repository" --project="$project" --location="$region" \
  --format=json)
pymes_assert_policy_has_no_release_pool_members \
  "$repository_policy_json" "release Artifact Registry IAM preflight"

if ! gcloud iam workload-identity-pools describe "$pool" \
  --project="$project" --location=global >/dev/null 2>&1; then
  [[ "$phase" == "prepare" ]] || {
    echo "finalize requires the prepared WIF pool: $pool" >&2
    exit 1
  }
  gcloud iam workload-identity-pools create "$pool" \
      --project="$project" --location=global \
      --display-name="Pymes v3 release only"
fi
pool_json=$(gcloud iam workload-identity-pools describe "$pool" \
  --project="$project" --location=global --format=json)
jq -e \
  --arg name "projects/${project_number}/locations/global/workloadIdentityPools/${pool}" '
    .name == $name and
    .state == "ACTIVE" and
    .disabled != true
  ' <<<"$pool_json" >/dev/null || {
    echo "existing WIF pool is disabled or differs from the dedicated release pool" >&2
    exit 1
  }

attribute_mapping='google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.repository_id=assertion.repository_id,attribute.repository_owner_id=assertion.repository_owner_id,attribute.ref=assertion.ref,attribute.ref_protected=assertion.ref_protected,attribute.workflow_ref=assertion.workflow_ref,attribute.event_name=assertion.event_name'
attribute_condition="assertion.repository_id=='${github_repository_id}' && assertion.repository_owner_id=='${github_repository_owner_id}' && assertion.repository=='${github_repository}' && assertion.ref=='refs/heads/main' && assertion.ref_protected=='true' && assertion.workflow_ref=='${github_workflow_ref}' && assertion.event_name=='workflow_dispatch' && (assertion.sub=='${github_build_subject}' || assertion.sub=='${github_stg_subject}' || assertion.sub=='${github_prd_subject}')"
if ! gcloud iam workload-identity-pools providers describe "$provider" \
  --project="$project" --location=global --workload-identity-pool="$pool" >/dev/null 2>&1; then
  [[ "$phase" == "prepare" ]] || {
    echo "finalize requires the prepared WIF provider: $provider" >&2
    exit 1
  }
  gcloud iam workload-identity-pools providers create-oidc "$provider" \
    --project="$project" --location=global --workload-identity-pool="$pool" \
    --display-name="Pymes v3 GitHub releases" \
    --issuer-uri=https://token.actions.githubusercontent.com \
    --attribute-mapping="$attribute_mapping" \
    --attribute-condition="$attribute_condition"
fi

provider_json=$(gcloud iam workload-identity-pools providers describe "$provider" \
  --project="$project" --location=global --workload-identity-pool="$pool" --format=json)
jq -e \
  --arg name "projects/${project_number}/locations/global/workloadIdentityPools/${pool}/providers/${provider}" \
  --arg issuer "https://token.actions.githubusercontent.com" \
  --arg condition "$attribute_condition" \
  '
    .name == $name and
    .oidc.issuerUri == $issuer and
    .attributeCondition == $condition and
    .attributeMapping["google.subject"] == "assertion.sub" and
    .attributeMapping["attribute.repository"] == "assertion.repository" and
    .attributeMapping["attribute.repository_id"] == "assertion.repository_id" and
    .attributeMapping["attribute.repository_owner_id"] == "assertion.repository_owner_id" and
    .attributeMapping["attribute.ref"] == "assertion.ref" and
    .attributeMapping["attribute.ref_protected"] == "assertion.ref_protected" and
    .attributeMapping["attribute.workflow_ref"] == "assertion.workflow_ref" and
    .attributeMapping["attribute.event_name"] == "assertion.event_name" and
    (.attributeMapping | length) == 8 and
    ((.oidc.allowedAudiences // []) | length) == 0 and
    .state == "ACTIVE"
  ' <<<"$provider_json" >/dev/null ||
  {
    echo "existing WIF provider differs from the dedicated release policy" >&2
    exit 1
  }

project_iam_read_role="projects/${project}/roles/pymesV3ReleaseProjectIamRead"
kms_policy_read_role="projects/${project}/roles/pymesV3ReleaseKmsPolicyRead"
runtime_accounts=(
  api
  web
  worker
  provision
  fiscal
  accounting
  accounting-admin
  migrate
  fiscal-migrate
  acct-migrate
)
release_services=(
  api
  web
  worker
  fiscal
  accounting
  accounting-admin
)
release_jobs=(
  migrate
  fiscal-migrate
  accounting-migrate
  accounting-grants
  provision-org
)

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

ensure_service_account() {
  local account_id="$1" display_name="$2" email
  email="${account_id}@${project}.iam.gserviceaccount.com"
  if ! gcloud iam service-accounts describe "$email" --project="$project" >/dev/null 2>&1; then
    [[ "$phase" == "prepare" ]] || {
      echo "finalize requires prepared release identity: $email" >&2
      exit 1
    }
    gcloud iam service-accounts create "$account_id" \
      --project="$project" --display-name="$display_name"
  fi
}

ensure_service_account pymes-v3-gh-build \
  "Pymes v3 immutable image builder"
for environment in "${managed_environments[@]}"; do
  ensure_service_account "pymes-v3-gh-deploy-${environment}" \
    "Pymes v3 ${environment} Cloud Run deployer"
done

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

filtered_roles() {
  local policy_json="$1" member="$2"
  jq -r --arg member "$member" '
    .bindings[]?
    | select((.members // []) | index($member) != null)
    | .role
  ' <<<"$policy_json" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u
}

direct_project_roles() {
  local email="$1"
  gcloud projects get-iam-policy "$project" \
    --flatten='bindings[].members' \
    --filter="bindings.members=serviceAccount:${email}" \
    --format='value(bindings.role)' | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u
}

direct_repository_roles() {
  local email="$1"
  gcloud artifacts repositories get-iam-policy "$artifact_repository" \
    --project="$project" --location="$region" \
    --flatten='bindings[].members' \
    --filter="bindings.members=serviceAccount:${email}" \
    --format='value(bindings.role)' | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u
}

assert_exact_or_empty_member_role() {
  local description="$1" policy_json="$2" member="$3" expected_role="$4"
  local actual
  actual=$(filtered_roles "$policy_json" "$member")
  [[ -z "$actual" || "$actual" == "$expected_role" ]] || {
    echo "$description must grant the release deployer no role or exactly $expected_role" >&2
    echo "actual: $actual" >&2
    exit 1
  }
}

assert_roles_subset() {
  local description="$1" actual="$2"
  shift 2
  local role allowed candidate
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    allowed=false
    for candidate in "$@"; do
      if [[ "$role" == "$candidate" ]]; then
        allowed=true
        break
      fi
    done
    if [[ "$allowed" != "true" ]]; then
      echo "$description has prohibited pre-existing role: $role" >&2
      exit 1
    fi
  done <<<"$actual"
}

assert_effective_iam_subset() {
  local account_id="$1"
  shift
  local email ancestors_json analysis_json ancestor_type ancestor_id
  local expected_ancestor_role resource_audit_role
  local ancestor_policy actual_pairs allowed_pairs unexpected_pairs
  email="${account_id}@${project}.iam.gserviceaccount.com"
  ancestors_json=$(gcloud projects get-ancestors "$project" --format=json)
  jq -e \
    --arg project "$expected_project" \
    --arg folder "$pymes_release_expected_folder" \
    --arg organization "$pymes_release_expected_organization" '
    (
      map({type: .type, id: (.id | tostring)})
      | sort_by(.type, .id)
    ) == (
      [
        {type: "project", id: $project},
        {type: "folder", id: $folder},
        {type: "organization", id: $organization}
      ]
      | sort_by(.type, .id)
    )
  ' <<<"$ancestors_json" >/dev/null || {
    echo "cannot prove the complete project ancestry for effective IAM analysis" >&2
    exit 1
  }
  analysis_json=$(gcloud asset analyze-iam-policy \
    --project="$project" \
    --identity="serviceAccount:${email}" \
    --expand-groups \
    --expand-roles \
    --analyze-service-account-impersonation \
    --output-group-edges \
    --execution-timeout=120s \
    --show-response \
    --format=json)
  jq -e '
    .fullyExplored == true and
    .mainAnalysis.fullyExplored == true and
    all(.mainAnalysis.analysisResults[]?; .fullyExplored == true) and
    all(
      .serviceAccountImpersonationAnalysis[]?;
      .fullyExplored == true and
      all(.analysisResults[]?; .fullyExplored == true) and
      ((.analysisResults // []) | length) == 0
    ) and
    all(
      (
        .mainAnalysis.analysisResults[]?,
        .serviceAccountImpersonationAnalysis[]?.analysisResults[]?
      );
      ((.identityList.groupEdges // []) | length) == 0
    )
  ' <<<"$analysis_json" >/dev/null || {
    echo "effective IAM analysis was incomplete or found indirect Google Group membership for $email" >&2
    exit 1
  }

  while IFS=$'\t' read -r ancestor_type ancestor_id; do
    case "$ancestor_type" in
      project)
        continue
        ;;
      folder)
        ancestor_policy=$(gcloud resource-manager folders get-iam-policy \
          "$ancestor_id" --format=json)
        resource_audit_role=$folder_iam_read_role
        ;;
      organization)
        ancestor_policy=$(gcloud organizations get-iam-policy \
          "$ancestor_id" --format=json)
        resource_audit_role=$organization_iam_read_role
        ;;
      *)
        echo "unsupported project ancestor type: $ancestor_type" >&2
        exit 1
        ;;
    esac
    expected_ancestor_role=
    if [[ "$account_id" == pymes-v3-gh-deploy-* ]]; then
      expected_ancestor_role=$resource_audit_role
    fi
    jq -e \
      --arg member "serviceAccount:${email}" \
      --arg expected_role "$expected_ancestor_role" \
      --arg resource_audit_role "$resource_audit_role" \
      --argjson policy_bindings "$(jq '.bindings // []' <<<"$ancestor_policy")" '
      (
        [
          .bindings[]?
          | select((.members // []) | index($member) != null)
          | {
              role: .role,
              condition: (.condition // null),
              occurrences: ([.members[] | select(. == $member)] | length)
            }
        ] as $direct
        | if $expected_role == "" then
            ($direct | length) == 0
          else
            ($direct == [] or $direct == [{
              role: $expected_role,
              condition: null,
              occurrences: 1
            }])
          end
      ) and
      all(
        .bindings[]?;
        all(
          .members[]?;
          if startswith("serviceAccount:pymes-v3-gh-deploy-") then
            (
              . == "serviceAccount:pymes-v3-gh-deploy-stg@pymes-dev-352318.iam.gserviceaccount.com" or
              . == "serviceAccount:pymes-v3-gh-deploy-prd@pymes-dev-352318.iam.gserviceaccount.com"
            ) and
            (. as $release_member | any(
              $policy_bindings[]?;
              .role == $resource_audit_role and
              (.condition // null) == null and
              ((.members // []) | index($release_member) != null)
            ))
          elif startswith("serviceAccount:pymes-v3-") then
            false
          elif . == "user:softponti@gmail.com" then
            true
          else
            (
              startswith("user:") or startswith("serviceAccount:")
            ) and
            (. as $other_member | all(
              $policy_bindings[]?;
              if ((.members // []) | index($other_member) != null) then
                .role == "roles/viewer" or
                .role == "roles/browser" or
                .role == "roles/iam.securityReviewer" or
                .role == "roles/resourcemanager.organizationViewer" or
                .role == "roles/resourcemanager.folderViewer"
              else
                true
              end
            ))
          end
        )
      )
    ' <<<"$ancestor_policy" >/dev/null || {
      echo "ancestor IAM contains authority outside the exact audit reader, or group/domain/public authority that cannot be proven unrelated to $email: ${ancestor_type}/${ancestor_id}" >&2
      exit 1
    }
  done < <(jq -r '.[] | [.type, (.id | tostring)] | @tsv' <<<"$ancestors_json")

  actual_pairs=$(jq -r '
    [
      (
        .mainAnalysis.analysisResults[]?,
        .serviceAccountImpersonationAnalysis[]?.analysisResults[]?
      )
      | [.attachedResourceFullName, .iamBinding.role]
      | @tsv
    ]
    | unique
    | sort
    | .[]
  ' <<<"$analysis_json")
  allowed_pairs=$(printf '%s\n' "$@" |
    sed '/^[[:space:]]*$/d' |
    LC_ALL=C sort -u)
  unexpected_pairs=$(comm -23 \
    <(printf '%s\n' "$actual_pairs" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u) \
    <(printf '%s\n' "$allowed_pairs" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u))
  [[ -z "$unexpected_pairs" ]] || {
    echo "effective IAM analysis found roles outside the reviewed resource allowlist for $email" >&2
    printf '%s\n' "$unexpected_pairs" >&2
    exit 1
  }
}

preflight_deploy_resources() {
  local environment="$1" deploy_email="$2" secret component runtime_email
  local policy_json keyring signing_key service job runtime_json kms_key
  local member="serviceAccount:${deploy_email}"
  local release_secrets=()
  mapfile -t release_secrets < <(release_secret_names "$environment")

  for secret in "${release_secrets[@]}"; do
    gcloud secrets describe "$secret" --project="$project" >/dev/null || {
      echo "missing release secret container before IAM grants: $secret" >&2
      exit 1
    }
    policy_json=$(gcloud secrets get-iam-policy "$secret" \
      --project="$project" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy_json" "release secret $secret IAM preflight"
    assert_exact_or_empty_member_role \
      "$deploy_email on secret $secret" \
      "$policy_json" "$member" roles/secretmanager.viewer
  done

  for component in "${runtime_accounts[@]}"; do
    runtime_email="pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
    if ! runtime_json=$(gcloud iam service-accounts describe "$runtime_email" \
      --project="$project" --format=json); then
      echo "runtime identity is missing, disabled, or unexpected before IAM grants: $runtime_email" >&2
      exit 1
    fi
    jq -e --arg email "$runtime_email" '
      .email == $email and .disabled != true
    ' <<<"$runtime_json" >/dev/null || {
      echo "runtime identity is missing, disabled, or unexpected before IAM grants: $runtime_email" >&2
      exit 1
    }
    policy_json=$(gcloud iam service-accounts get-iam-policy "$runtime_email" \
      --project="$project" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy_json" "runtime identity $runtime_email IAM preflight"
    assert_exact_or_empty_member_role \
      "$deploy_email on runtime identity $runtime_email" \
      "$policy_json" "$member" roles/iam.serviceAccountUser
  done

  keyring="pymes-v3-${environment}"
  gcloud kms keyrings describe "$keyring" \
    --project="$project" --location="$region" >/dev/null || {
      echo "missing release KMS keyring before IAM grants: $keyring" >&2
      exit 1
    }
  policy_json=$(gcloud kms keyrings get-iam-policy "$keyring" \
    --project="$project" --location="$region" --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$policy_json" "release KMS keyring $keyring IAM preflight"
  assert_exact_or_empty_member_role \
    "$deploy_email on KMS keyring $keyring" \
    "$policy_json" "$member" "$kms_policy_read_role"

  signing_key=internal-jwt-signing
  gcloud kms keys describe "$signing_key" \
    --project="$project" --location="$region" --keyring="$keyring" >/dev/null || {
      echo "missing release signing key before IAM grants: $keyring/$signing_key" >&2
      exit 1
    }
  policy_json=$(gcloud kms keys get-iam-policy "$signing_key" \
    --project="$project" --location="$region" --keyring="$keyring" --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$policy_json" "release signing key $keyring/$signing_key IAM preflight"
  assert_exact_or_empty_member_role \
    "$deploy_email on signing key $keyring/$signing_key" \
    "$policy_json" "$member" roles/cloudkms.publicKeyViewer
  for kms_key in secrets calendar-tokens fiscal-vault; do
    gcloud kms keys describe "$kms_key" \
      --project="$project" --location="$region" --keyring="$keyring" >/dev/null || {
      echo "missing release data key before IAM grants: $keyring/$kms_key" >&2
      exit 1
    }
    policy_json=$(gcloud kms keys get-iam-policy "$kms_key" \
      --project="$project" --location="$region" --keyring="$keyring" \
      --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy_json" "release KMS key $keyring/$kms_key IAM preflight"
  done

  if [[ "$phase" == "finalize" ]]; then
    for component in "${release_services[@]}"; do
      service="pymes-v3-${environment}-${component}"
      gcloud run services describe "$service" \
        --project="$project" --region="$region" >/dev/null || {
          echo "missing initial Cloud Run service before IAM grants: $service" >&2
          exit 1
        }
      policy_json=$(gcloud run services get-iam-policy "$service" \
        --project="$project" --region="$region" --format=json)
      pymes_assert_policy_has_no_release_pool_members \
        "$policy_json" "release Cloud Run service $service IAM preflight"
      assert_exact_or_empty_member_role \
        "$deploy_email on Cloud Run service $service" \
        "$policy_json" "$member" roles/run.admin
    done
    for component in "${release_jobs[@]}"; do
      job="pymes-v3-${environment}-${component}"
      gcloud run jobs describe "$job" \
        --project="$project" --region="$region" >/dev/null || {
          echo "missing initial Cloud Run job before IAM grants: $job" >&2
          exit 1
        }
      policy_json=$(gcloud run jobs get-iam-policy "$job" \
        --project="$project" --region="$region" --format=json)
      pymes_assert_policy_has_no_release_pool_members \
        "$policy_json" "release Cloud Run job $job IAM preflight"
      assert_exact_or_empty_member_role \
        "$deploy_email on Cloud Run job $job" \
        "$policy_json" "$member" roles/run.admin
    done
  fi
}

verify_initial_cloud_run_resources() {
  local environment="$1" release_sha="$2" manifest_release
  manifest_release=$(sed -n 's/^PYMES_SOURCE_SHA=//p' "$initial_seed_manifest")
  [[ "$(grep -c '^PYMES_SOURCE_SHA=' "$initial_seed_manifest")" -eq 1 &&
     "$manifest_release" == "$release_sha" ]] || {
    echo "initial seed manifest does not bind exactly to the reviewed Pymes SHA" >&2
    exit 1
  }
  PYMES_CLOUD_RUN_SEED_APPLY=false \
    PYMES_CLOUD_RUN_SEED_VERIFY=true \
    PYMES_CLOUD_RUN_SEED_ENV="$environment" \
    PYMES_CLOUD_RUN_SEED_MANIFEST="$initial_seed_manifest" \
    "$script_dir/seed-cloud-run-resources.sh"
}

verify_initial_seed_audit() {
  local environment="$1" release_sha="$2" repo_root local_sha
  local started_epoch completed_epoch now_epoch
  local audit_window_end_at filter audit_scope
  local logs_first_json logs_json project_policy
  local canonical_first canonical_second allowed_resources_json
  local allowed_service_accounts_json component resource resource_number
  local service_account_email service_account_resource
  local audit_log_limit=1001
  local audit_end_grace_seconds=120
  local audit_min_settle_seconds=600
  local audit_stability_wait_seconds=20
  repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
  local_sha=$(git -C "$repo_root" rev-parse HEAD)
  [[ "$(git -C "$repo_root" branch --show-current)" == "main" &&
     "$local_sha" == "$release_sha" &&
     -z "$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)" ]] || {
    echo "finalize requires a clean main checkout at the exact inert-seed SHA" >&2
    exit 1
  }
  [[ "$initial_seed_operator_email" == "softponti@gmail.com" ]] || {
    echo "finalize accepts only the reviewed existing project Owner softponti@gmail.com as initial seed operator" >&2
    exit 1
  }
  started_epoch=$(date -u -d "$initial_seed_started_at" +%s)
  completed_epoch=$(date -u -d "$initial_seed_completed_at" +%s)
  now_epoch=$(date -u +%s)
  (( started_epoch <= completed_epoch &&
     completed_epoch - started_epoch <= 14400 &&
     completed_epoch <= now_epoch &&
     now_epoch - started_epoch <= 172800 )) || {
    echo "initial seed audit window must be ordered, last at most four hours and start within the last 48 hours" >&2
    exit 1
  }
  (( now_epoch - completed_epoch >= audit_min_settle_seconds )) || {
    echo "wait at least ${audit_min_settle_seconds}s after PYMES_INITIAL_SEED_COMPLETED_AT before finalizing so Admin Activity can settle" >&2
    exit 1
  }
  audit_window_end_at=$(
    initial_seed_audit_end_at \
      "$initial_seed_completed_at" "$audit_end_grace_seconds"
  )
  project_policy=$(gcloud projects get-iam-policy "$project" --format=json)
  jq -e --arg member "user:${initial_seed_operator_email}" '
    any(
      .bindings[]?;
      .role == "roles/owner" and
      .condition == null and
      ((.members // []) | index($member) != null)
    )
  ' <<<"$project_policy" >/dev/null || {
    echo "seed operator no longer matches the reviewed pre-existing direct project Owner authority" >&2
    exit 1
  }

  allowed_resources_json=$(
    {
      for component in "${release_services[@]}"; do
        printf 'projects/%s/locations/%s/services/pymes-v3-%s-%s\n' \
          "$project" "$region" "$environment" "$component"
      done
      for component in "${release_jobs[@]}"; do
        printf 'projects/%s/locations/%s/jobs/pymes-v3-%s-%s\n' \
          "$project" "$region" "$environment" "$component"
      done
    } | jq -Rsc 'split("\n") | map(select(length > 0))'
  )
  allowed_service_accounts_json=$(
    {
      for component in "${runtime_accounts[@]}"; do
        printf 'pymes-v3-%s-%s@%s.iam.gserviceaccount.com\n' \
          "$component" "$environment" "$project"
      done
    } | jq -Rsc 'split("\n") | map(select(length > 0))'
  )
  audit_scope="protoPayload.authenticationInfo.principalEmail=\"${initial_seed_operator_email}\" OR protoPayload.authenticationInfo.serviceAccountDelegationInfo.firstPartyPrincipal.principalEmail=\"${initial_seed_operator_email}\""
  while IFS= read -r resource; do
    resource_number=${resource/projects\/${project}\//projects\/${project_number}\/}
    audit_scope+=" OR protoPayload.resourceName=\"${resource}\""
    audit_scope+=" OR protoPayload.resourceName=\"${resource_number}\""
    audit_scope+=" OR protoPayload.resourceName=\"//run.googleapis.com/${resource}\""
    audit_scope+=" OR protoPayload.resourceName=\"//run.googleapis.com/${resource_number}\""
  done < <(jq -r '.[]' <<<"$allowed_resources_json")
  while IFS= read -r service_account_email; do
    for service_account_resource in \
      "projects/-/serviceAccounts/${service_account_email}" \
      "projects/${project}/serviceAccounts/${service_account_email}" \
      "projects/${project_number}/serviceAccounts/${service_account_email}"; do
      audit_scope+=" OR protoPayload.resourceName=\"${service_account_resource}\""
      audit_scope+=" OR protoPayload.resourceName=\"//iam.googleapis.com/${service_account_resource}\""
    done
  done < <(jq -r '.[]' <<<"$allowed_service_accounts_json")
  filter="logName=\"projects/${project}/logs/cloudaudit.googleapis.com%2Factivity\" AND timestamp>=\"${initial_seed_started_at}\" AND timestamp<\"${audit_window_end_at}\" AND (${audit_scope})"
  logs_first_json=$(gcloud logging read "$filter" \
    --project="$project" --limit="$audit_log_limit" --order=asc --format=json)
  sleep "$audit_stability_wait_seconds"
  logs_json=$(gcloud logging read "$filter" \
    --project="$project" --limit="$audit_log_limit" --order=asc --format=json)
  canonical_first=$(jq -cS 'sort_by(.insertId // "", .timestamp // "")' \
    <<<"$logs_first_json")
  canonical_second=$(jq -cS 'sort_by(.insertId // "", .timestamp // "")' \
    <<<"$logs_json")
  [[ "$canonical_first" == "$canonical_second" ]] || {
    echo "Cloud Audit Logs changed during the ${audit_stability_wait_seconds}s stability read; wait and rerun finalize" >&2
    exit 1
  }
  jq -e \
    --argjson allowed_resources "$allowed_resources_json" \
    --argjson allowed_sas "$allowed_service_accounts_json" \
    --arg project "$project" \
    --arg project_number "$project_number" \
    --arg region "$region" \
    --arg operator "$initial_seed_operator_email" \
    --argjson limit "$audit_log_limit" \
    -f "$script_dir/initial-seed-audit.jq" \
    <<<"$logs_json" >/dev/null || {
    echo "Cloud Audit Logs do not prove that the reviewed Owner touched only all 6+5 allowlisted seed resources and their exact runtime service accounts" >&2
    exit 1
  }
  echo "Initial seed audit verified: operator=${initial_seed_operator_email} authority=pre-existing-owner resources=11 runtime_act_as=exact end_grace=${audit_end_grace_seconds}s settled=${audit_min_settle_seconds}s stable_read=${audit_stability_wait_seconds}s unrelated_mutations=0"
}

assert_release_account_preflight() {
  local account_id="$1" expected_principal="$2" repository_role="$3"
  shift 3
  local email account_json user_keys policy_json actual_policy expected_policy
  local project_roles repository_roles
  email="${account_id}@${project}.iam.gserviceaccount.com"
  account_json=$(gcloud iam service-accounts describe "$email" \
    --project="$project" --format=json)
  jq -e --arg email "$email" '
    .email == $email and
    .disabled != true
  ' <<<"$account_json" >/dev/null || {
    echo "release service account is disabled or has unexpected identity: $email" >&2
    exit 1
  }
  user_keys=$(gcloud iam service-accounts keys list \
    --iam-account="$email" --project="$project" \
    --managed-by=user --format='value(name)')
  [[ -z "$user_keys" ]] || {
    echo "release service account has prohibited user-managed keys: $email" >&2
    exit 1
  }
  pymes_verify_release_account_not_attached \
    "$project" "$email" "release identity $email"

  policy_json=$(gcloud iam service-accounts get-iam-policy "$email" \
    --project="$project" --format=json)
  actual_policy=$(canonical_account_bindings <<<"$policy_json")
  expected_policy=$(jq -cnS --arg principal "$expected_principal" '
    [{
      role: "roles/iam.workloadIdentityUser",
      condition: null,
      members: [$principal]
    }]
  ')
  [[ "$actual_policy" == "[]" || "$actual_policy" == "$expected_policy" ]] || {
    echo "release service account policy must be empty or exactly the reviewed WIF binding: $email" >&2
    exit 1
  }

  project_roles=$(direct_project_roles "$email")
  assert_roles_subset "$email project IAM preflight" "$project_roles" "$@"
  repository_roles=$(direct_repository_roles "$email")
  assert_roles_subset "$email Artifact Registry preflight" "$repository_roles" "$repository_role"
}

build_principal="principal://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${pool}/subject/${github_build_subject}"
assert_release_account_preflight \
  pymes-v3-gh-build \
  "$build_principal" \
  roles/artifactregistry.writer \
  roles/serviceusage.serviceUsageConsumer
build_allowed_pairs=(
  "//cloudresourcemanager.googleapis.com/projects/${project}"$'\t'"roles/serviceusage.serviceUsageConsumer"
  "//artifactregistry.googleapis.com/projects/${project}/locations/${region}/repositories/${artifact_repository}"$'\t'"roles/artifactregistry.writer"
  "//artifactregistry.googleapis.com/projects/${project_number}/locations/${region}/repositories/${artifact_repository}"$'\t'"roles/artifactregistry.writer"
)
assert_effective_iam_subset pymes-v3-gh-build "${build_allowed_pairs[@]}"
for environment in "${managed_environments[@]}"; do
  case "$environment" in
    stg) deploy_subject=$github_stg_subject ;;
    prd) deploy_subject=$github_prd_subject ;;
  esac
  deploy_principal="principal://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${pool}/subject/${deploy_subject}"
  deploy_email="pymes-v3-gh-deploy-${environment}@${project}.iam.gserviceaccount.com"
  assert_release_account_preflight \
    "pymes-v3-gh-deploy-${environment}" \
    "$deploy_principal" \
    roles/artifactregistry.reader \
    roles/serviceusage.serviceUsageConsumer \
    "projects/${project}/roles/pymesV3ReleaseProjectIamRead"
  deploy_allowed_pairs=(
    "//cloudresourcemanager.googleapis.com/projects/${project}"$'\t'"roles/serviceusage.serviceUsageConsumer"
    "//cloudresourcemanager.googleapis.com/projects/${project}"$'\t'"projects/${project}/roles/pymesV3ReleaseProjectIamRead"
    "//artifactregistry.googleapis.com/projects/${project}/locations/${region}/repositories/${artifact_repository}"$'\t'"roles/artifactregistry.reader"
    "//artifactregistry.googleapis.com/projects/${project_number}/locations/${region}/repositories/${artifact_repository}"$'\t'"roles/artifactregistry.reader"
    "//cloudkms.googleapis.com/projects/${project}/locations/${region}/keyRings/pymes-v3-${environment}"$'\t'"${kms_policy_read_role}"
    "//cloudkms.googleapis.com/projects/${project_number}/locations/${region}/keyRings/pymes-v3-${environment}"$'\t'"${kms_policy_read_role}"
    "//cloudkms.googleapis.com/projects/${project}/locations/${region}/keyRings/pymes-v3-${environment}/cryptoKeys/internal-jwt-signing"$'\t'"roles/cloudkms.publicKeyViewer"
    "//cloudkms.googleapis.com/projects/${project_number}/locations/${region}/keyRings/pymes-v3-${environment}/cryptoKeys/internal-jwt-signing"$'\t'"roles/cloudkms.publicKeyViewer"
  )
  mapfile -t release_secrets < <(release_secret_names "$environment")
  for secret in "${release_secrets[@]}"; do
    deploy_allowed_pairs+=(
      "//secretmanager.googleapis.com/projects/${project}/secrets/${secret}"$'\t'"roles/secretmanager.viewer"
      "//secretmanager.googleapis.com/projects/${project_number}/secrets/${secret}"$'\t'"roles/secretmanager.viewer"
    )
  done
  for component in "${runtime_accounts[@]}"; do
    runtime_email="pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
    deploy_allowed_pairs+=(
      "//iam.googleapis.com/projects/${project}/serviceAccounts/${runtime_email}"$'\t'"roles/iam.serviceAccountUser"
      "//iam.googleapis.com/projects/${project_number}/serviceAccounts/${runtime_email}"$'\t'"roles/iam.serviceAccountUser"
    )
  done
  if [[ "$phase" == "finalize" ]]; then
    for component in "${release_services[@]}"; do
      service="pymes-v3-${environment}-${component}"
      deploy_allowed_pairs+=(
        "//run.googleapis.com/projects/${project}/locations/${region}/services/${service}"$'\t'"roles/run.admin"
        "//run.googleapis.com/projects/${project_number}/locations/${region}/services/${service}"$'\t'"roles/run.admin"
      )
    done
    for component in "${release_jobs[@]}"; do
      job="pymes-v3-${environment}-${component}"
      deploy_allowed_pairs+=(
        "//run.googleapis.com/projects/${project}/locations/${region}/jobs/${job}"$'\t'"roles/run.admin"
        "//run.googleapis.com/projects/${project_number}/locations/${region}/jobs/${job}"$'\t'"roles/run.admin"
      )
    done
  fi
  assert_effective_iam_subset "pymes-v3-gh-deploy-${environment}" "${deploy_allowed_pairs[@]}"
  preflight_deploy_resources "$environment" "$deploy_email"
  if [[ "$phase" == "finalize" ]]; then
    verify_initial_seed_audit \
      "$environment" "$initial_seed_release_sha"
    verify_initial_cloud_run_resources \
      "$environment" "$initial_seed_release_sha"
  fi
done
inverse_run_mode=absent
if [[ "$phase" == "finalize" ]]; then
  inverse_run_mode=present
fi
pymes_verify_release_inverse_authority \
  "$project" "$project_number" "$region" "$target_environment" \
  "$inverse_run_mode" "$artifact_repository"
echo "Release service-account preflight verified: enabled, keyless, exact trust, project-effective and inverse-permission IAM plus fail-closed ancestor policies, and exact planned-resource roles"

grant_project_role() {
  local account_id="$1" role="$2"
  gcloud projects add-iam-policy-binding "$project" \
    --member="serviceAccount:${account_id}@${project}.iam.gserviceaccount.com" \
    --role="$role" --condition=None --quiet >/dev/null
}

grant_repository_role() {
  local account_id="$1" role="$2"
  gcloud artifacts repositories add-iam-policy-binding "$artifact_repository" \
    --project="$project" --location="$region" \
    --member="serviceAccount:${account_id}@${project}.iam.gserviceaccount.com" \
    --role="$role" --condition=None --quiet >/dev/null
}

grant_secret_metadata_reader() {
  local secret="$1" account_id="$2"
  gcloud secrets describe "$secret" --project="$project" >/dev/null ||
    {
      echo "missing release secret container: $secret" >&2
      exit 1
    }
  gcloud secrets add-iam-policy-binding "$secret" \
    --project="$project" \
    --member="serviceAccount:${account_id}@${project}.iam.gserviceaccount.com" \
    --role=roles/secretmanager.viewer \
    --condition=None --quiet >/dev/null
}

ensure_organization_custom_role() {
  local role_id="$1" title="$2" description="$3"
  local permissions_csv="$4" expected_permissions_json="$5"
  local role_json
  if ! gcloud iam roles describe "$role_id" \
    --organization="$pymes_release_expected_organization" >/dev/null 2>&1; then
    [[ "$phase" == "prepare" ]] || {
      echo "finalize requires prepared organization custom role $role_id" >&2
      exit 1
    }
    gcloud iam roles create "$role_id" \
      --organization="$pymes_release_expected_organization" \
      --title="$title" \
      --description="$description" \
      --stage=GA \
      --permissions="$permissions_csv"
  else
    role_json=$(gcloud iam roles describe "$role_id" \
      --organization="$pymes_release_expected_organization" --format=json)
    if ! jq -e \
      --arg name "organizations/${pymes_release_expected_organization}/roles/${role_id}" \
      --argjson expected "$expected_permissions_json" '
        .name == $name and
        .deleted != true and
        (.stage // "GA") == "GA" and
        (.includedPermissions | sort) == $expected
      ' <<<"$role_json" >/dev/null; then
      [[ "$phase" == "prepare" ]] || {
        echo "existing organization custom role is broader or different: $role_id" >&2
        exit 1
      }
      gcloud iam roles update "$role_id" \
        --organization="$pymes_release_expected_organization" \
        --title="$title" \
        --description="$description" \
        --stage=GA \
        --permissions="$permissions_csv"
    fi
  fi
  role_json=$(gcloud iam roles describe "$role_id" \
    --organization="$pymes_release_expected_organization" --format=json)
  jq -e \
    --arg name "organizations/${pymes_release_expected_organization}/roles/${role_id}" \
    --argjson expected "$expected_permissions_json" '
      .name == $name and
      .deleted != true and
      (.stage // "GA") == "GA" and
      (.includedPermissions | sort) == $expected
    ' <<<"$role_json" >/dev/null || {
      echo "organization custom role did not converge: $role_id" >&2
      exit 1
    }
}

if ! gcloud iam roles describe pymesV3ReleaseProjectIamRead \
  --project="$project" >/dev/null 2>&1; then
  [[ "$phase" == "prepare" ]] || {
    echo "finalize requires prepared custom role pymesV3ReleaseProjectIamRead" >&2
    exit 1
  }
  gcloud iam roles create pymesV3ReleaseProjectIamRead \
    --project="$project" \
    --title="Pymes v3 release preflight read" \
    --description="Read only exact SQL, network and IAM metadata needed by the release gate" \
    --stage=GA \
    --permissions="$(pymes_release_project_iam_read_permissions_csv)"
else
  role_json=$(gcloud iam roles describe pymesV3ReleaseProjectIamRead \
    --project="$project" --format=json)
  if ! jq -e \
    --argjson expected "$(pymes_release_project_iam_read_permissions_json)" '
    (
      .includedPermissions | sort
    ) == $expected and
    .deleted != true
  ' <<<"$role_json" >/dev/null; then
    [[ "$phase" == "prepare" ]] || {
      echo "existing custom release verifier role is broader or different" >&2
      exit 1
    }
    gcloud iam roles update pymesV3ReleaseProjectIamRead \
      --project="$project" \
      --title="Pymes v3 release preflight read" \
      --description="Read only exact SQL, network and IAM metadata needed by the release gate" \
      --stage=GA \
      --permissions="$(pymes_release_project_iam_read_permissions_csv)"
  fi
fi
role_json=$(gcloud iam roles describe pymesV3ReleaseProjectIamRead \
  --project="$project" --format=json)
jq -e \
  --argjson expected "$(pymes_release_project_iam_read_permissions_json)" '
    .name == "projects/pymes-dev-352318/roles/pymesV3ReleaseProjectIamRead" and
    .deleted != true and
    (.stage // "GA") == "GA" and
    (.includedPermissions | sort) == $expected
  ' <<<"$role_json" >/dev/null || {
    echo "custom release verifier role did not converge to the reviewed permission set" >&2
    exit 1
  }
if ! gcloud iam roles describe pymesV3ReleaseKmsPolicyRead \
  --project="$project" >/dev/null 2>&1; then
  [[ "$phase" == "prepare" ]] || {
    echo "finalize requires prepared custom role pymesV3ReleaseKmsPolicyRead" >&2
    exit 1
  }
  gcloud iam roles create pymesV3ReleaseKmsPolicyRead \
    --project="$project" \
    --title="Pymes v3 release KMS policy read" \
    --description="Read exact KMS key metadata and inherited/direct IAM without cryptographic use" \
    --stage=GA \
    --permissions="$(pymes_release_kms_policy_read_permissions_csv)"
else
  role_json=$(gcloud iam roles describe pymesV3ReleaseKmsPolicyRead \
    --project="$project" --format=json)
  if ! jq -e \
    --argjson expected "$(pymes_release_kms_policy_read_permissions_json)" '
    (.includedPermissions | sort) == $expected and
    .deleted != true
  ' <<<"$role_json" >/dev/null; then
    [[ "$phase" == "prepare" ]] || {
      echo "existing custom KMS policy reader role is broader or different" >&2
      exit 1
    }
    gcloud iam roles update pymesV3ReleaseKmsPolicyRead \
      --project="$project" \
      --title="Pymes v3 release KMS policy read" \
      --description="Read exact KMS key metadata and inherited/direct IAM without cryptographic use" \
      --stage=GA \
      --permissions="$(pymes_release_kms_policy_read_permissions_csv)"
  fi
fi
role_json=$(gcloud iam roles describe pymesV3ReleaseKmsPolicyRead \
  --project="$project" --format=json)
jq -e \
  --argjson expected "$(pymes_release_kms_policy_read_permissions_json)" '
    .name == "projects/pymes-dev-352318/roles/pymesV3ReleaseKmsPolicyRead" and
    .deleted != true and
    (.stage // "GA") == "GA" and
    (.includedPermissions | sort) == $expected
  ' <<<"$role_json" >/dev/null || {
    echo "custom KMS policy reader role did not converge to the reviewed permission set" >&2
    exit 1
  }

ensure_organization_custom_role \
  pymesV3ReleaseOrganizationIamRead \
  "Pymes v3 release organization IAM read" \
  "Read only the organization IAM policy and exact release audit role definitions" \
  "$(pymes_release_organization_iam_read_permissions_csv)" \
  "$(pymes_release_organization_iam_read_permissions_json)"
ensure_organization_custom_role \
  pymesV3ReleaseFolderIamRead \
  "Pymes v3 release folder IAM read" \
  "Read only the exact shared-folder IAM policy for inherited-authority checks" \
  "$(pymes_release_folder_iam_read_permissions_csv)" \
  "$(pymes_release_folder_iam_read_permissions_json)"

build_id=pymes-v3-gh-build
build_email="${build_id}@${project}.iam.gserviceaccount.com"
build_principal="principal://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${pool}/subject/${github_build_subject}"
gcloud iam service-accounts add-iam-policy-binding "$build_email" \
  --project="$project" \
  --member="$build_principal" \
  --role=roles/iam.workloadIdentityUser \
  --condition=None --quiet >/dev/null
grant_project_role "$build_id" roles/serviceusage.serviceUsageConsumer
grant_repository_role "$build_id" roles/artifactregistry.writer

for environment in "${managed_environments[@]}"; do
  deploy_id="pymes-v3-gh-deploy-${environment}"
  deploy_email="${deploy_id}@${project}.iam.gserviceaccount.com"
  case "$environment" in
    stg) deploy_subject=$github_stg_subject ;;
    prd) deploy_subject=$github_prd_subject ;;
  esac
  deploy_principal="principal://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${pool}/subject/${deploy_subject}"
  gcloud iam service-accounts add-iam-policy-binding "$deploy_email" \
    --project="$project" \
    --member="$deploy_principal" \
    --role=roles/iam.workloadIdentityUser \
    --condition=None --quiet >/dev/null

  for role in \
    roles/serviceusage.serviceUsageConsumer \
    "$project_iam_read_role"; do
    grant_project_role "$deploy_id" "$role"
  done
  grant_repository_role "$deploy_id" roles/artifactregistry.reader
  gcloud organizations add-iam-policy-binding \
    "$pymes_release_expected_organization" \
    --member="serviceAccount:${deploy_email}" \
    --role="$organization_iam_read_role" \
    --condition=None --quiet >/dev/null
  gcloud resource-manager folders add-iam-policy-binding \
    "$pymes_release_expected_folder" \
    --member="serviceAccount:${deploy_email}" \
    --role="$folder_iam_read_role" \
    --condition=None --quiet >/dev/null

  mapfile -t release_secrets < <(release_secret_names "$environment")
  for secret in "${release_secrets[@]}"; do
    grant_secret_metadata_reader "$secret" "$deploy_id"
  done

  for component in "${runtime_accounts[@]}"; do
    runtime_email="pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
    gcloud iam service-accounts describe "$runtime_email" --project="$project" >/dev/null ||
      {
        echo "missing runtime identity: $runtime_email" >&2
        exit 1
      }
    gcloud iam service-accounts add-iam-policy-binding "$runtime_email" \
      --project="$project" \
      --member="serviceAccount:${deploy_email}" \
      --role=roles/iam.serviceAccountUser \
      --condition=None --quiet >/dev/null
  done

  gcloud kms keys add-iam-policy-binding internal-jwt-signing \
    --project="$project" --location="$region" --keyring="pymes-v3-${environment}" \
    --member="serviceAccount:${deploy_email}" \
    --role=roles/cloudkms.publicKeyViewer \
    --condition=None --quiet >/dev/null
  gcloud kms keyrings add-iam-policy-binding "pymes-v3-${environment}" \
    --project="$project" --location="$region" \
    --member="serviceAccount:${deploy_email}" \
    --role="$kms_policy_read_role" \
    --condition=None --quiet >/dev/null
done

if [[ "$phase" == "finalize" ]]; then
  for environment in "${managed_environments[@]}"; do
    deploy_email="pymes-v3-gh-deploy-${environment}@${project}.iam.gserviceaccount.com"
    for component in "${release_services[@]}"; do
      service="pymes-v3-${environment}-${component}"
      gcloud run services describe "$service" \
        --project="$project" --region="$region" >/dev/null ||
        {
          echo "missing initial Cloud Run service: $service" >&2
          echo "run seed-cloud-run-resources.sh with the reviewed Owner, then rerun phase=finalize" >&2
          exit 1
        }
      gcloud run services add-iam-policy-binding "$service" \
        --project="$project" --region="$region" \
        --member="serviceAccount:${deploy_email}" \
        --role=roles/run.admin \
        --condition=None --quiet >/dev/null
    done
    for component in "${release_jobs[@]}"; do
      job="pymes-v3-${environment}-${component}"
      gcloud run jobs describe "$job" \
        --project="$project" --region="$region" >/dev/null ||
        {
          echo "missing initial Cloud Run job: $job" >&2
          echo "run seed-cloud-run-resources.sh with the reviewed Owner, then rerun phase=finalize" >&2
          exit 1
        }
      gcloud run jobs add-iam-policy-binding "$job" \
        --project="$project" --region="$region" \
        --member="serviceAccount:${deploy_email}" \
        --role=roles/run.admin \
        --condition=None --quiet >/dev/null
    done
  done
else
  cat <<EOF
Release identity preparation is complete without project-wide roles/run.admin.
Initial inert seed:
  1. The reviewed existing project Owner builds and verifies the exact digest manifest.
  2. That Owner runs seed-cloud-run-resources.sh for ${target_environment}; it creates
     only six internal zero-instance services and five never-executed jobs.
  3. Audit the exact 11 Cloud Run mutations and rerun this script with
     PYMES_RELEASE_IDENTITY_PHASE=finalize and
     PYMES_RELEASE_IDENTITY_ENV=${target_environment}, plus the printed
     PYMES_INITIAL_SEED_* evidence, including the exact manifest and its SHA-256.
Finalize grants roles/run.admin only on the six services and five jobs in the selected environment.
The protected workflow can run only after finalize; it never receives project-wide Run Admin.
EOF
fi

role_members() {
  local policy_json="$1" role="$2"
  jq -r --arg role "$role" '
    .bindings[]?
    | select(.role == $role)
    | .members[]?
  ' <<<"$policy_json" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u
}

assert_exact_roles() {
  local description="$1" actual="$2"
  shift 2
  local expected
  expected=$(printf '%s\n' "$@" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ "$actual" != "$expected" ]]; then
    echo "$description has unexpected direct roles" >&2
    echo "expected:" >&2
    printf '%s\n' "$expected" >&2
    echo "actual:" >&2
    printf '%s\n' "$actual" >&2
    exit 1
  fi
}

assert_project_roles() {
  local account_id="$1"
  shift
  local actual
  actual=$(direct_project_roles "${account_id}@${project}.iam.gserviceaccount.com")
  assert_exact_roles "$account_id project IAM" "$actual" "$@"
}

legacy_release_email="pymes-github-actions-stg@${project}.iam.gserviceaccount.com"
if gcloud iam service-accounts describe "$legacy_release_email" \
  --project="$project" >/dev/null 2>&1; then
  legacy_roles=$(direct_project_roles "$legacy_release_email")
  legacy_prohibited=$(grep -E \
    'roles/(artifactregistry\\.writer|iam\\.serviceAccountUser|run\\.admin|secretmanager\\.(admin|secretAccessor))' \
    <<<"$legacy_roles" || true)
  if [[ -n "$legacy_prohibited" ]]; then
    legacy_disabled=$(gcloud iam service-accounts describe "$legacy_release_email" \
      --project="$project" --format='value(disabled)')
    if [[ "$legacy_disabled" != "True" ]]; then
      echo "TRANSITION WARNING retire legacy Pymes WIF trust and disable $legacy_release_email before phase=close" >&2
    fi
  fi
fi

assert_project_roles pymes-v3-gh-build \
  roles/serviceusage.serviceUsageConsumer
project_policy_json=$(gcloud projects get-iam-policy "$project" --format=json)
pymes_assert_policy_has_no_release_pool_members \
  "$project_policy_json" "release project IAM postcondition"
build_email="pymes-v3-gh-build@${project}.iam.gserviceaccount.com"
pymes_verify_release_account_not_attached \
  "$project" "$build_email" "release builder postcondition"
build_policy=$(gcloud iam service-accounts get-iam-policy "$build_email" \
  --project="$project" --format=json)
build_wif_members="principal://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${pool}/subject/${github_build_subject}"
actual_build_wif_members=$(role_members "$build_policy" roles/iam.workloadIdentityUser)
[[ "$actual_build_wif_members" == "$build_wif_members" ]] ||
  {
    echo "$build_email has unexpected WIF principals" >&2
    exit 1
  }
for environment in "${managed_environments[@]}"; do
  assert_project_roles "pymes-v3-gh-deploy-${environment}" \
    roles/serviceusage.serviceUsageConsumer \
    "$project_iam_read_role"

  build_email="pymes-v3-gh-build@${project}.iam.gserviceaccount.com"
  deploy_email="pymes-v3-gh-deploy-${environment}@${project}.iam.gserviceaccount.com"
  pymes_verify_release_account_not_attached \
    "$project" "$deploy_email" "release deployer postcondition"
  case "$environment" in
    stg) deploy_subject=$github_stg_subject ;;
    prd) deploy_subject=$github_prd_subject ;;
  esac
  deploy_wif_principal="principal://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${pool}/subject/${deploy_subject}"
  deploy_policy=$(gcloud iam service-accounts get-iam-policy "$deploy_email" \
    --project="$project" --format=json)
  roles=$(filtered_roles "$deploy_policy" "$deploy_wif_principal")
  assert_exact_roles "$deploy_email WIF trust" "$roles" roles/iam.workloadIdentityUser
  deploy_wif_members=$(role_members "$deploy_policy" roles/iam.workloadIdentityUser)
  [[ "$deploy_wif_members" == "$deploy_wif_principal" ]] ||
    {
      echo "$deploy_email has unexpected WIF principals" >&2
      exit 1
    }

  repository_policy=$(gcloud artifacts repositories get-iam-policy "$artifact_repository" \
    --project="$project" --location="$region" --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$repository_policy" "release Artifact Registry IAM postcondition"
  roles=$(filtered_roles "$repository_policy" "serviceAccount:$build_email")
  assert_exact_roles "$build_email Artifact Registry" "$roles" roles/artifactregistry.writer
  roles=$(filtered_roles "$repository_policy" "serviceAccount:$deploy_email")
  assert_exact_roles "$deploy_email Artifact Registry" "$roles" roles/artifactregistry.reader

  organization_policy=$(gcloud organizations get-iam-policy \
    "$pymes_release_expected_organization" --format=json)
  roles=$(filtered_roles "$organization_policy" "serviceAccount:$deploy_email")
  assert_exact_roles \
    "$deploy_email organization IAM audit" "$roles" \
    "$organization_iam_read_role"
  folder_policy=$(gcloud resource-manager folders get-iam-policy \
    "$pymes_release_expected_folder" --format=json)
  roles=$(filtered_roles "$folder_policy" "serviceAccount:$deploy_email")
  assert_exact_roles \
    "$deploy_email folder IAM audit" "$roles" \
    "$folder_iam_read_role"

  keyring_policy=$(gcloud kms keyrings get-iam-policy "pymes-v3-${environment}" \
    --project="$project" --location="$region" --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$keyring_policy" "$deploy_email KMS key-ring postcondition"
  roles=$(filtered_roles "$keyring_policy" "serviceAccount:$deploy_email")
  assert_exact_roles "$deploy_email KMS key-ring" "$roles" "$kms_policy_read_role"
  signing_policy=$(gcloud kms keys get-iam-policy internal-jwt-signing \
    --project="$project" --location="$region" --keyring="pymes-v3-${environment}" \
    --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$signing_policy" "$deploy_email internal signing key postcondition"
  roles=$(filtered_roles "$signing_policy" "serviceAccount:$deploy_email")
  assert_exact_roles "$deploy_email internal signing key" "$roles" roles/cloudkms.publicKeyViewer
  for kms_key in secrets calendar-tokens fiscal-vault; do
    kms_policy=$(gcloud kms keys get-iam-policy "$kms_key" \
      --project="$project" --location="$region" \
      --keyring="pymes-v3-${environment}" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$kms_policy" "$deploy_email KMS key $kms_key postcondition"
  done

  for component in "${runtime_accounts[@]}"; do
    runtime_email="pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
    runtime_policy=$(gcloud iam service-accounts get-iam-policy "$runtime_email" \
      --project="$project" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$runtime_policy" "$runtime_email IAM postcondition"
    roles=$(filtered_roles "$runtime_policy" "serviceAccount:$deploy_email")
    assert_exact_roles "$deploy_email on $runtime_email" "$roles" roles/iam.serviceAccountUser
  done
  mapfile -t release_secrets < <(release_secret_names "$environment")
  for secret in "${release_secrets[@]}"; do
    secret_policy=$(gcloud secrets get-iam-policy "$secret" \
      --project="$project" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$secret_policy" "$secret IAM postcondition"
    roles=$(filtered_roles "$secret_policy" "serviceAccount:$deploy_email")
    assert_exact_roles "$deploy_email on $secret" "$roles" roles/secretmanager.viewer
  done
  if [[ "$phase" == "finalize" && "$environment" == "$target_environment" ]]; then
    for component in "${release_services[@]}"; do
      service="pymes-v3-${environment}-${component}"
      service_policy=$(gcloud run services get-iam-policy "$service" \
        --project="$project" --region="$region" --format=json)
      pymes_assert_policy_has_no_release_pool_members \
        "$service_policy" "$service IAM postcondition"
      roles=$(filtered_roles "$service_policy" "serviceAccount:$deploy_email")
      assert_exact_roles "$deploy_email on $service" "$roles" roles/run.admin
    done
    for component in "${release_jobs[@]}"; do
      job="pymes-v3-${environment}-${component}"
      job_policy=$(gcloud run jobs get-iam-policy "$job" \
        --project="$project" --region="$region" --format=json)
      pymes_assert_policy_has_no_release_pool_members \
        "$job_policy" "$job IAM postcondition"
      roles=$(filtered_roles "$job_policy" "serviceAccount:$deploy_email")
      assert_exact_roles "$deploy_email on $job" "$roles" roles/run.admin
    done
  fi
done

release_pool_assets=$(pymes_search_release_pool_iam_assets "$project")
pymes_validate_release_pool_iam_assets \
  "$release_pool_assets" "$target_environment" exact

provider_resource="projects/${project_number}/locations/global/workloadIdentityPools/${pool}/providers/${provider}"
for environment in "${managed_environments[@]}"; do
  cat <<EOF
Reviewed release identity for ${environment}:
  provider=${provider_resource}
  build_service_account=pymes-v3-gh-build@${project}.iam.gserviceaccount.com
  deploy_service_account=pymes-v3-gh-deploy-${environment}@${project}.iam.gserviceaccount.com
These values are source-controlled constants in v3-release.yml. Do not duplicate
or override them with GitHub repository/environment variables.
EOF
done
