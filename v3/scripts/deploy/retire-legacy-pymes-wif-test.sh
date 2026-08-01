#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
export PYMES_LEGACY_WIF_SOURCE_ONLY=true
# shellcheck source=retire-legacy-pymes-wif.sh
source "$script_dir/retire-legacy-pymes-wif.sh"

project=pymes-dev-352318
project_number=884236221349
region=us-central1
dedicated_account="dedicated@${project}.iam.gserviceaccount.com"
shared_account="shared@${project}.iam.gserviceaccount.com"
legacy_principal="principalSet://iam.googleapis.com/projects/${project_number}/legacy"
reviewed_main_sha=1111111111111111111111111111111111111111
reviewed_tree_sha=2222222222222222222222222222222222222222

tests=0

pass() {
  tests=$((tests + 1))
  printf 'ok %d - %s\n' "$tests" "$1"
}

expect_success() {
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    pass "$description"
  else
    echo "not ok - expected success: $description" >&2
    exit 1
  fi
}

expect_failure() {
  local description="$1"
  shift
  if ("$@") >/dev/null 2>&1; then
    echo "not ok - expected failure: $description" >&2
    exit 1
  fi
  pass "$description"
}

complete_analysis() {
  local attached_resource="${1:-}" permission="${2:-}" effective_resource="${3:-}"
  if [[ -z "$attached_resource" ]]; then
    printf '%s\n' \
      '{"fullyExplored":true,"mainAnalysis":{"fullyExplored":true,"analysisResults":[]},"serviceAccountImpersonationAnalysis":[]}'
    return
  fi
  jq -cn \
    --arg attached "$attached_resource" \
    --arg permission "$permission" \
    --arg resource "$effective_resource" '
    {
      fullyExplored: true,
      mainAnalysis: {
        fullyExplored: true,
        analysisResults: [{
          fullyExplored: true,
          attachedResourceFullName: $attached,
          iamBinding: {
            role: "roles/test.role",
            members: ["serviceAccount:test@example.com"]
          },
          accessControlLists: [{
            accesses: [{permission: $permission}],
            resources: [{fullResourceName: $resource}]
          }]
        }]
      },
      serviceAccountImpersonationAnalysis: []
    }
  '
}

impersonation_analysis() {
  local attached_resource="$1" permission="$2" effective_resource="$3"
  jq -cn \
    --arg attached "$attached_resource" \
    --arg permission "$permission" \
    --arg resource "$effective_resource" '
    {
      fullyExplored: true,
      mainAnalysis: {
        fullyExplored: true,
        analysisResults: []
      },
      serviceAccountImpersonationAnalysis: [{
        fullyExplored: true,
        analysisResults: [{
          fullyExplored: true,
          attachedResourceFullName: $attached,
          iamBinding: {
            role: "roles/test.impersonated",
            members: ["serviceAccount:intermediate@example.com"]
          },
          accessControlLists: [{
            accesses: [{permission: $permission}],
            resources: [{fullResourceName: $resource}]
          }]
        }]
      }]
    }
  '
}

analysis_fixture=$(complete_analysis)
policy_analysis_for_identity() {
  printf '%s\n' "$analysis_fixture"
}
expect_success \
  "Policy Analyzer clean result permits retirement" \
  assert_shared_account_has_no_effective_pymes_authority

analysis_fixture=$(complete_analysis \
  "//cloudresourcemanager.googleapis.com/projects/${project}" \
  run.services.update \
  "//run.googleapis.com/projects/${project}/locations/${region}/services/pymes-v3-stg-api")
expect_failure \
  "project-scoped shared authority over Pymes is rejected" \
  assert_shared_account_has_no_effective_pymes_authority

analysis_fixture=$(impersonation_analysis \
  "//cloudresourcemanager.googleapis.com/projects/${project}" \
  run.services.update \
  "//run.googleapis.com/projects/${project}/locations/${region}/services/pymes-v3-stg-api")
expect_failure \
  "shared authority reached through service-account impersonation is rejected" \
  assert_shared_account_has_no_effective_pymes_authority

analysis_fixture='{"fullyExplored":false,"mainAnalysis":{"fullyExplored":false,"analysisResults":[]},"serviceAccountImpersonationAnalysis":[]}'
expect_failure \
  "incomplete Policy Analyzer result fails closed" \
  assert_shared_account_has_no_effective_pymes_authority

analysis_fixture='{"fullyExplored":true,"mainAnalysis":{"fullyExplored":true,"analysisResults":[]},"serviceAccountImpersonationAnalysis":[{"fullyExplored":true,"analysisResults":[{"fullyExplored":false}]}]}'
expect_failure \
  "incomplete impersonated analysis result fails closed" \
  assert_shared_account_has_no_effective_pymes_authority

expect_success \
  "STG-first retirement accepts no PRD deployer" \
  assert_prd_release_identity_absent \
  $'pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com\npymes-v3-gh-deploy-stg@pymes-dev-352318.iam.gserviceaccount.com'
expect_failure \
  "premature PRD deployer blocks STG legacy retirement" \
  assert_prd_release_identity_absent \
  $'pymes-v3-gh-deploy-stg@pymes-dev-352318.iam.gserviceaccount.com\npymes-v3-gh-deploy-prd@pymes-dev-352318.iam.gserviceaccount.com'

ancestor_policy='{"bindings":[{"role":"roles/viewer","members":["serviceAccount:reader@example.com"]}]}'
expect_success \
  "ancestor policy without groups is auditable" \
  assert_policy_has_no_group_bindings "organization:123" "$ancestor_policy"
ancestor_policy='{"bindings":[{"role":"roles/viewer","members":["group:operators@example.com"]}]}'
expect_failure \
  "unverifiable ancestor group membership blocks retirement" \
  assert_policy_has_no_group_bindings "organization:123" "$ancestor_policy"

for dangerous_role in \
  roles/run.invoker \
  roles/iam.serviceAccountOpenIdTokenCreator \
  roles/iam.serviceAccountDeleter; do
  [[ "$(filter_broad_roles <<<"$dangerous_role")" == "$dangerous_role" ]] || {
    echo "not ok - broad-role filter omitted $dangerous_role" >&2
    exit 1
  }
done
pass "broad-role filter covers invocation, OIDC-token and lifecycle authority"

analysis_fixture=$(complete_analysis \
  "//run.googleapis.com/projects/${project}/locations/${region}/services/pymes-v3-stg-api" \
  run.services.update \
  "//run.googleapis.com/projects/${project}/locations/${region}/services/pymes-v3-stg-api")
expect_success \
  "STG deployer may update its exact allowlisted service" \
  assert_new_identity_effective_authorization \
  "serviceAccount:deploy-stg@example.com" deploy stg

analysis_fixture=$(complete_analysis \
  "//run.googleapis.com/projects/${project}/locations/${region}/services/pymes-v3-prd-api" \
  run.services.update \
  "//run.googleapis.com/projects/${project}/locations/${region}/services/pymes-v3-prd-api")
expect_failure \
  "STG deployer cannot update a PRD service" \
  assert_new_identity_effective_authorization \
  "serviceAccount:deploy-stg@example.com" deploy stg

analysis_fixture=$(impersonation_analysis \
  "//run.googleapis.com/projects/${project}/locations/${region}/services/pymes-v3-prd-api" \
  run.services.update \
  "//run.googleapis.com/projects/${project}/locations/${region}/services/pymes-v3-prd-api")
expect_failure \
  "STG deployer authority outside its allowlist through impersonation is rejected" \
  assert_new_identity_effective_authorization \
  "serviceAccount:deploy-stg@example.com" deploy stg

policy_json='{"bindings":[{"role":"roles/reader","members":["serviceAccount:allowed@example.com"]}]}'
expect_success \
  "exact unconditional IAM binding is accepted" \
  assert_policy_member_roles \
  "fixture policy" "$policy_json" "serviceAccount:allowed@example.com" \
  roles/reader
conditional_policy='{"bindings":[{"role":"roles/reader","members":["serviceAccount:allowed@example.com"],"condition":{"title":"bypass","expression":"true"}}]}'
expect_failure \
  "conditional replacement of an exact IAM binding is rejected" \
  assert_policy_member_roles \
  "fixture policy" "$conditional_policy" \
  "serviceAccount:allowed@example.com" roles/reader

marker_events() {
  local shared_timestamp="${1:-2026-08-01T10:00:02.100000000Z}"
  local disable_timestamp="${2:-2026-08-01T10:00:03.100000000Z}"
  local shared_members="${3:-[]}"
  local shared_actor="${4:-operator@example.com}"
  jq -cn \
    --arg dedicated "$dedicated_account" \
    --arg shared "$shared_account" \
    --arg principal "$legacy_principal" \
    --arg shared_timestamp "$shared_timestamp" \
    --arg disable_timestamp "$disable_timestamp" \
    --arg shared_actor "$shared_actor" \
    --argjson shared_members "$shared_members" '
    [
      {
        timestamp: "2026-08-01T10:00:01.100000000Z",
        insertId: "dedicated-policy",
        resource: {labels: {email_id: $dedicated}},
        protoPayload: {
          methodName: "google.iam.admin.v1.SetIAMPolicy",
          authenticationInfo: {principalEmail: "operator@example.com"},
          serviceData: {
            policyDelta: {
              bindingDeltas: [{
                action: "REMOVE",
                role: "roles/iam.workloadIdentityUser",
                member: $principal
              }]
            }
          },
          request: {
            resource: ("projects/example/serviceAccounts/" + $dedicated),
            policy: {etag: "a", bindings: []}
          }
        }
      },
      {
        timestamp: $shared_timestamp,
        insertId: "shared-policy",
        resource: {labels: {email_id: $shared}},
        protoPayload: {
          methodName: "google.iam.admin.v1.SetIAMPolicy",
          authenticationInfo: {principalEmail: $shared_actor},
          serviceData: {
            policyDelta: {
              bindingDeltas: [{
                action: "REMOVE",
                role: "roles/iam.workloadIdentityUser",
                member: $principal
              }]
            }
          },
          request: {
            resource: ("projects/example/serviceAccounts/" + $shared),
            policy: {
              etag: "b",
              bindings: [{
                role: "roles/iam.workloadIdentityUser",
                members: $shared_members
              }]
            }
          }
        }
      },
      {
        timestamp: $disable_timestamp,
        insertId: "disable",
        resource: {labels: {email_id: $dedicated}},
        protoPayload: {
          methodName: "google.iam.admin.v1.DisableServiceAccount",
          authenticationInfo: {principalEmail: "operator@example.com"}
        }
      }
    ]
  '
}

events=$(marker_events)
expect_success \
  "durable marker requires both policy writes before disable" \
  build_retirement_marker "$events"
events=$(marker_events \
  2026-08-01T10:00:04.100000000Z \
  2026-08-01T10:00:03.100000000Z)
expect_failure \
  "policy write after disable cannot form the cutover marker" \
  build_retirement_marker "$events"
events=$(marker_events \
  2026-08-01T10:00:02.100000000Z \
  2026-08-01T10:00:03.100000000Z \
  "[\"${legacy_principal}\"]")
expect_failure \
  "policy event retaining legacy trust cannot form the marker" \
  build_retirement_marker "$events"
events=$(marker_events \
  2026-08-01T10:00:02.100000000Z \
  2026-08-01T10:00:03.100000000Z \
  "[]" other@example.com)
expect_failure \
  "cutover events from different actors are rejected" \
  build_retirement_marker "$events"
events=$(marker_events)
events=$(jq '.[0].protoPayload.serviceData.policyDelta.bindingDeltas = []' \
  <<<"$events")
expect_failure \
  "no-op policy write cannot form a retirement marker" \
  build_retirement_marker "$events"

legacy_policy=$(jq -cn --arg principal "$legacy_principal" '
  {bindings:[{
    role:"roles/iam.workloadIdentityUser",
    members:[$principal]
  }]}
')
expect_success \
  "apply precondition requires an exact legacy binding" \
  assert_legacy_binding_present "$dedicated_account" "$legacy_policy"
expect_failure \
  "apply precondition rejects an already-clean policy" \
  assert_legacy_binding_present "$dedicated_account" '{"bindings":[]}'

expect_success \
  "canary source is bound to exact commit, tree and workflow" \
  assert_canary_source_identity \
  "$reviewed_main_sha" "$reviewed_tree_sha" \
  3333333333333333333333333333333333333333 \
  3333333333333333333333333333333333333333
expect_failure \
  "ancestor SHA with the same workflow is rejected" \
  assert_canary_source_identity \
  4444444444444444444444444444444444444444 \
  "$reviewed_tree_sha" \
  3333333333333333333333333333333333333333 \
  3333333333333333333333333333333333333333
expect_failure \
  "same workflow with a different helper tree is rejected" \
  assert_canary_source_identity \
  "$reviewed_main_sha" 5555555555555555555555555555555555555555 \
  3333333333333333333333333333333333333333 \
  3333333333333333333333333333333333333333
expect_success \
  "operational canary title is accepted for the exact reviewed SHA" \
  assert_operational_canary_title \
  "Pymes V3 stg operational @ ${reviewed_main_sha}" \
  "$reviewed_main_sha"
expect_failure \
  "bootstrap release cannot be used as the retirement canary" \
  assert_operational_canary_title \
  "Pymes V3 stg bootstrap @ ${reviewed_main_sha}" \
  "$reviewed_main_sha"

pymes_verify_release_account_not_attached() {
  return 0
}

gcloud() {
  case "$*" in
    "iam service-accounts keys list --iam-account=pymes-v3-gh-build@"*" --managed-by=user --format=value(name)") ;;
    "iam service-accounts keys list --iam-account=pymes-v3-gh-deploy-stg@"*" --managed-by=user --format=value(name)") ;;
    *) return 20 ;;
  esac
}
expect_success \
  "release key revalidation accepts keyless builder and STG deployer" \
  assert_dedicated_release_accounts_keyless

gcloud() {
  case "$*" in
    "iam service-accounts keys list --iam-account=pymes-v3-gh-build@"*" --managed-by=user --format=value(name)") ;;
    "iam service-accounts keys list --iam-account=pymes-v3-gh-deploy-stg@"*" --managed-by=user --format=value(name)") printf '%s\n' unexpected-user-key ;;
    *) return 21 ;;
  esac
}
expect_failure \
  "release key revalidation rejects a new STG deployer user-managed key" \
  assert_dedicated_release_accounts_keyless

service_account_state=False
service_account_state_status=0
gcloud() {
  case "$*" in
    "iam service-accounts describe "*) ;;
    *) return 22 ;;
  esac
  [[ "$service_account_state_status" == "0" ]] || return "$service_account_state_status"
  printf '%s\n' "$service_account_state"
}
expect_success \
  "exact False lifecycle state proves an account is enabled" \
  assert_service_account_enabled "$dedicated_account" "fixture account"
service_account_state=True
expect_success \
  "exact True lifecycle state proves an account is disabled" \
  assert_service_account_disabled "$dedicated_account" "fixture account"
service_account_state=
expect_failure \
  "missing lifecycle state fails closed" \
  assert_service_account_enabled "$dedicated_account" "fixture account"
service_account_state_status=23
expect_failure \
  "service-account lifecycle read failure fails closed" \
  assert_service_account_enabled "$dedicated_account" "fixture account"

gcloud() {
  return 17
}
expect_failure \
  "Service Usage read errors cannot be treated as disabled APIs" \
  run_if_api_enabled compute.googleapis.com true

gcloud() {
  [[ -v CLOUDSDK_RUN_REGION && -z "$CLOUDSDK_RUN_REGION" ]] || return 18
  case "$*" in
    "run services list "*) printf '%s\n' '[]' ;;
    "run jobs list "*) printf '%s\n' '[]' ;;
    "run revisions list "*) printf '%s\n' '[]' ;;
    *) return 19 ;;
  esac
}
expect_success \
  "Cloud Run service inventory clears inherited region configuration" \
  list_all_cloud_run_services
expect_success \
  "Cloud Run job inventory clears inherited region configuration" \
  list_all_cloud_run_jobs
expect_success \
  "Cloud Run revision inventory clears inherited region configuration" \
  list_all_cloud_run_revisions

revision_inventory=$(jq -cn --arg account "$dedicated_account" '
  [{
    metadata: {
      name: "pymes-v3-old-revision",
      namespace: "example"
    },
    spec: {
      serviceAccountName: $account
    }
  }]
')
expect_failure \
  "historical Cloud Run revision retaining the legacy identity blocks disable" \
  assert_inventory_has_no_account \
  run-revision "Cloud Run revisions in any region" "$revision_inventory"

revision_inventory=$(jq -cn --arg account "$dedicated_account" '
  [{
    metadata: {name: "pymes-v3-old-revision-alternate-schema"},
    spec: {
      template: {
        spec: {
          serviceAccountName: $account
        }
      }
    }
  }]
')
expect_failure \
  "alternate Cloud Run revision schema retaining the legacy identity blocks disable" \
  assert_inventory_has_no_account \
  run-revision "Cloud Run revisions in any region" "$revision_inventory"

revision_inventory='[{"metadata":{"name":"unrelated"},"spec":{"serviceAccountName":"other@example.com"}}]'
expect_success \
  "historical Cloud Run revision using another identity is accepted" \
  assert_inventory_has_no_account \
  run-revision "Cloud Run revisions in any region" "$revision_inventory"

expect_failure \
  "malformed Cloud Run revision inventory fails closed" \
  assert_inventory_has_no_account \
  run-revision "Cloud Run revisions in any region" '{}'

gcloud() {
  jq -cn --arg account "$dedicated_account" '
    [{
      name: ("//iam.googleapis.com/projects/example/serviceAccounts/" + $account),
      assetType: "iam.googleapis.com/ServiceAccount",
      location: "global",
      additionalAttributes: {email: $account}
    }]
  '
}
expect_success \
  "exact Cloud Asset service-account inventory is accepted" \
  assert_cloud_asset_has_no_workload_reference

gcloud() {
  jq -cn --arg account "$dedicated_account" '
    [{
      name: ("//run.googleapis.com/projects/example/locations/europe-west1/services/foreign"),
      assetType: "run.googleapis.com/Service",
      location: "europe-west1",
      additionalAttributes: {serviceAccount: $account}
    }]
  '
}
expect_failure \
  "Cloud Asset workload reference in another region blocks disable" \
  assert_cloud_asset_has_no_workload_reference

gcloud() {
  printf '%s\n' '[]'
}
expect_failure \
  "empty Cloud Asset workload inventory fails closed" \
  assert_cloud_asset_has_no_workload_reference

revision_fixture='[]'
assert_cloud_asset_has_no_workload_reference() {
  return 0
}
list_all_cloud_run_services() {
  printf '%s\n' '[]'
}
list_all_cloud_run_jobs() {
  printf '%s\n' '[]'
}
list_all_cloud_run_revisions() {
  printf '%s\n' "$revision_fixture"
}
run_if_api_enabled() {
  return 0
}
gcloud() {
  case "$*" in
    "iam service-accounts keys list --iam-account=${dedicated_account} "*) ;;
    *) return 25 ;;
  esac
}
expect_success \
  "complete direct workload inventory accepts no legacy references" \
  assert_no_user_keys_or_workloads

revision_fixture=$(jq -cn --arg account "$dedicated_account" '
  [{
    metadata: {name: "retained-old-revision"},
    spec: {serviceAccountName: $account}
  }]
')
expect_failure \
  "complete direct workload inventory rejects a retained old revision" \
  assert_no_user_keys_or_workloads

list_all_cloud_run_revisions() {
  return 26
}
expect_failure \
  "Cloud Run revision read errors fail the complete workload inventory closed" \
  assert_no_user_keys_or_workloads

precondition_trace=
precondition_failure=
record_precondition_gate() {
  local gate="$1"
  precondition_trace+="${precondition_trace:+ }${gate}"
  [[ "$precondition_failure" != "$gate" ]]
}
verify_release_foundation() {
  record_precondition_gate foundation
}
assert_exact_new_release_authorization() {
  record_precondition_gate exact-authorization
}
assert_shared_account_has_no_broad_authority() {
  record_precondition_gate shared-broad-authority
}
assert_shared_account_has_no_effective_pymes_authority() {
  record_precondition_gate shared-effective-authority
}
assert_post_removal_policy_unchanged() {
  case "$1" in
    "$dedicated_account")
      [[ "$2" == "dedicated-policy" ]] || return 27
      record_precondition_gate dedicated-policy
      ;;
    "$shared_account")
      [[ "$2" == "shared-policy" ]] || return 28
      record_precondition_gate shared-policy
      ;;
    *) return 29 ;;
  esac
}
assert_service_account_enabled() {
  case "$1" in
    "$dedicated_account") record_precondition_gate dedicated-enabled ;;
    "$shared_account") record_precondition_gate shared-enabled ;;
    *) return 30 ;;
  esac
}
assert_dedicated_release_accounts_keyless() {
  record_precondition_gate release-accounts-keyless
}
assert_no_user_keys_or_workloads() {
  record_precondition_gate legacy-workloads-empty
}
expect_success \
  "immediate disable gate revalidates every boundary after policy removal" \
  assert_immediate_disable_preconditions dedicated-policy shared-policy
expected_trace='foundation exact-authorization shared-broad-authority shared-effective-authority dedicated-policy shared-policy dedicated-enabled shared-enabled release-accounts-keyless legacy-workloads-empty'
[[ "$precondition_trace" == "$expected_trace" ]] || {
  echo "not ok - immediate disable preconditions ran in an unexpected order" >&2
  echo "expected: $expected_trace" >&2
  echo "actual:   $precondition_trace" >&2
  exit 1
}
pass "direct key and workload reads are the final gates before disable"

precondition_trace=
precondition_failure=shared-effective-authority
set +e
assert_immediate_disable_preconditions \
  dedicated-policy shared-policy >/dev/null 2>&1
precondition_status=$?
set -e
[[ "$precondition_status" -ne 0 ]] || {
  echo "not ok - immediate disable gate accepted a failed security revalidation" >&2
  exit 1
}
pass "immediate disable gate stops at the first failed security revalidation"
[[ "$precondition_trace" == \
  'foundation exact-authorization shared-broad-authority shared-effective-authority' ]] || {
  echo "not ok - failed immediate disable gate continued past its blocker" >&2
  exit 1
}
pass "failed immediate disable gate cannot continue to later checks"

expect_failure \
  "test-only source mode cannot bypass a direct retirement execution" \
  env PYMES_LEGACY_WIF_SOURCE_ONLY=true \
  bash "$script_dir/retire-legacy-pymes-wif.sh"

printf '1..%d\n' "$tests"
