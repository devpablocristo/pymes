#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=verify-release-authority.sh
source "$script_dir/verify-release-authority.sh"

member="serviceAccount:pymes-v3-gh-deploy-stg@pymes-dev-352318.iam.gserviceaccount.com"
custom_role="projects/pymes-dev-352318/roles/pymesV3ReleaseProjectIamRead"
project_policy() {
  local run_condition=${1:-null} extra_role=${2:-}
  jq -cn \
    --arg member "$member" \
    --arg custom_role "$custom_role" \
    --argjson run_condition "$run_condition" \
    --arg extra_role "$extra_role" '
      {
        bindings: (
          (
            [
              "roles/serviceusage.serviceUsageConsumer",
              $custom_role
            ]
            | map({role: ., members: [$member]})
          )
          + (
            if $run_condition == null then
              []
            else
              [{
                role: "roles/run.admin",
                members: [$member],
                condition: $run_condition
              }]
            end
          )
          + (
            if $extra_role == "" then
              []
            else
              [{role: $extra_role, members: [$member]}]
            end
          )
        )
      }
    '
}

expect_failure() {
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "FAIL: $description" >&2
    exit 1
  fi
}

validate_project_authority "$(project_policy)" stg bootstrap
validate_project_authority "$(project_policy)" stg operational

expect_failure \
  "bootstrap accepted project-wide Run Admin" \
  validate_project_authority \
  "$(project_policy '{"title":"temporary","description":"time only","expression":"request.time < timestamp(\"2026-01-01T01:00:00Z\")"}')" \
  stg bootstrap
expect_failure \
  "operational accepted project-wide Run Admin" \
  validate_project_authority \
  "$(project_policy null roles/run.admin)" \
  stg operational
expect_failure \
  "release accepted an extra project role" \
  validate_project_authority \
  "$(project_policy null roles/editor)" \
  stg bootstrap

empty_resource_policy='{"bindings":[]}'
exact_resource_policy=$(jq -cn --arg member "$member" '
  {bindings: [{role: "roles/run.admin", members: [$member]}]}
')
conditional_resource_policy=$(jq -cn --arg member "$member" '
  {
    bindings: [{
      role: "roles/run.admin",
      members: [$member],
      condition: {title: "unexpected", expression: "request.time < timestamp(\"2099-01-01T00:00:00Z\")"}
    }]
  }
')
validate_resource_authority \
  "$empty_resource_policy" "$member" absent "bootstrap fixture"
validate_resource_authority \
  "$exact_resource_policy" "$member" exact "operational fixture"
expect_failure \
  "bootstrap accepted resource-scoped authority before finalize" \
  validate_resource_authority \
  "$exact_resource_policy" "$member" absent "bootstrap fixture"
expect_failure \
  "operational accepted conditional resource authority" \
  validate_resource_authority \
  "$conditional_resource_policy" "$member" exact "operational fixture"

custom_role_json=$(jq -cn \
  --argjson permissions "$(pymes_release_project_iam_read_permissions_json)" '
    {
      name: "projects/pymes-dev-352318/roles/pymesV3ReleaseProjectIamRead",
      stage: "GA",
      includedPermissions: $permissions
    }
  ')
validate_custom_role_json "$custom_role_json"
expect_failure \
  "release accepted an expanded custom verifier role" \
  validate_custom_role_json \
  "$(jq '.includedPermissions += ["run.services.update"]' <<<"$custom_role_json")"

kms_role_json=$(jq -cn \
  --argjson permissions "$(pymes_release_kms_policy_read_permissions_json)" '
    {
      name: "projects/pymes-dev-352318/roles/pymesV3ReleaseKmsPolicyRead",
      stage: "GA",
      includedPermissions: $permissions
    }
  ')
validate_kms_custom_role_json "$kms_role_json"
expect_failure \
  "release accepted a KMS verifier role unable to describe the key ring" \
  validate_kms_custom_role_json \
  "$(jq 'del(.includedPermissions[] | select(. == "cloudkms.keyRings.get"))' <<<"$kms_role_json")"
expect_failure \
  "release accepted cryptographic use in the KMS verifier role" \
  validate_kms_custom_role_json \
  "$(jq '.includedPermissions += ["cloudkms.cryptoKeyVersions.useToDecrypt"]' <<<"$kms_role_json")"

organization_role_json=$(jq -cn \
  --arg name "$organization_iam_read_role" \
  --argjson permissions "$(pymes_release_organization_iam_read_permissions_json)" '
    {name: $name, stage: "GA", includedPermissions: $permissions}
  ')
validate_organization_iam_custom_role_json "$organization_role_json"
expect_failure \
  "release accepted an expanded organization-IAM reader role" \
  validate_organization_iam_custom_role_json \
  "$(jq '.includedPermissions += ["resourcemanager.organizations.setIamPolicy"]' <<<"$organization_role_json")"

folder_role_json=$(jq -cn \
  --arg name "$folder_iam_read_role" \
  --argjson permissions "$(pymes_release_folder_iam_read_permissions_json)" '
    {name: $name, stage: "GA", includedPermissions: $permissions}
  ')
validate_folder_iam_custom_role_json "$folder_role_json"
expect_failure \
  "release accepted a folder-IAM reader role unable to read the ancestor policy" \
  validate_folder_iam_custom_role_json \
  "$(jq '.includedPermissions = []' <<<"$folder_role_json")"

wif_pool_json='{
  "name":"projects/884236221349/locations/global/workloadIdentityPools/pymes-v3-release-pool",
  "state":"ACTIVE"
}'
validate_wif_pool_json "$wif_pool_json"
expect_failure \
  "release accepted a disabled WIF pool" \
  validate_wif_pool_json \
  "$(jq '.state = "DELETED"' <<<"$wif_pool_json")"

wif_provider_json=$(jq -cn \
  --arg condition "$attribute_condition" \
  --arg name "projects/884236221349/locations/global/workloadIdentityPools/pymes-v3-release-pool/providers/github" '
  {
    name: $name,
    oidc: {
      issuerUri: "https://token.actions.githubusercontent.com",
      allowedAudiences: []
    },
    attributeCondition: $condition,
    attributeMapping: {
      "google.subject": "assertion.sub",
      "attribute.repository": "assertion.repository",
      "attribute.repository_id": "assertion.repository_id",
      "attribute.repository_owner_id": "assertion.repository_owner_id",
      "attribute.ref": "assertion.ref",
      "attribute.ref_protected": "assertion.ref_protected",
      "attribute.workflow_ref": "assertion.workflow_ref",
      "attribute.event_name": "assertion.event_name"
    },
    state: "ACTIVE"
  }
')
validate_wif_provider_json "$wif_provider_json"
expect_failure \
  "release accepted a WIF provider from another pool" \
  validate_wif_provider_json \
  "$(jq '.name = "projects/884236221349/locations/global/workloadIdentityPools/other/providers/github"' <<<"$wif_provider_json")"
expect_failure \
  "release accepted a wider WIF condition" \
  validate_wif_provider_json \
  "$(jq '.attributeCondition = "true"' <<<"$wif_provider_json")"
expect_failure \
  "release accepted a custom WIF audience" \
  validate_wif_provider_json \
  "$(jq '.oidc.allowedAudiences = ["unexpected"]' <<<"$wif_provider_json")"

release_pool_assets=$(jq -cn \
  --arg build "$pymes_release_build_principal" \
  --arg stg "$pymes_release_stg_principal" '
  [
    {
      resource: "//iam.googleapis.com/projects/pymes-dev-352318/serviceAccounts/pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com",
      policy: {
        bindings: [{
          role: "roles/iam.workloadIdentityUser",
          members: [$build]
        }]
      }
    },
    {
      resource: "//iam.googleapis.com/projects/pymes-dev-352318/serviceAccounts/pymes-v3-gh-deploy-stg@pymes-dev-352318.iam.gserviceaccount.com",
      policy: {
        bindings: [{
          role: "roles/iam.workloadIdentityUser",
          members: [$stg]
        }]
      }
    }
  ]
')
pymes_validate_release_pool_iam_assets "$release_pool_assets" stg exact
pymes_validate_release_pool_iam_assets '[]' stg subset
expect_failure \
  "release accepted missing exact release-pool bindings" \
  pymes_validate_release_pool_iam_assets '[]' stg exact
expect_failure \
  "STG release accepted premature PRD WIF trust" \
  pymes_validate_release_pool_iam_assets \
  "$(jq --arg principal "$pymes_release_prd_principal" \
    --arg resource "//iam.googleapis.com/projects/pymes-dev-352318/serviceAccounts/pymes-v3-gh-deploy-prd@pymes-dev-352318.iam.gserviceaccount.com" '
      . + [{
        resource: $resource,
        policy: {
          bindings: [{
            role: "roles/iam.workloadIdentityUser",
            members: [$principal]
          }]
        }
      }]
    ' <<<"$release_pool_assets")" \
  stg exact
expect_failure \
  "PRD release accepted a missing STG trust root" \
  pymes_validate_release_pool_iam_assets '[]' prd subset
prd_release_pool_assets=$(jq \
  --arg principal "$pymes_release_prd_principal" \
  --arg resource "//iam.googleapis.com/projects/pymes-dev-352318/serviceAccounts/pymes-v3-gh-deploy-prd@pymes-dev-352318.iam.gserviceaccount.com" '
    . + [{
      resource: $resource,
      policy: {
        bindings: [{
          role: "roles/iam.workloadIdentityUser",
          members: [$principal]
        }]
      }
    }]
  ' <<<"$release_pool_assets")
pymes_validate_release_pool_iam_assets "$release_pool_assets" prd subset
pymes_validate_release_pool_iam_assets "$prd_release_pool_assets" prd exact
expect_failure \
  "PRD release accepted a missing environment trust" \
  pymes_validate_release_pool_iam_assets "$release_pool_assets" prd exact
expect_failure \
  "release accepted a direct release-pool principalSet on the project" \
  pymes_validate_release_pool_iam_assets \
  "$(jq --arg pool "$pymes_release_pool_path" '. += [{
    resource:"//cloudresourcemanager.googleapis.com/projects/pymes-dev-352318",
    policy:{bindings:[{
      role:"roles/owner",
      members:["principalSet://iam.googleapis.com/" + $pool + "/attribute.repository/devpablocristo/pymes"]
    }]}
  }]' <<<"$release_pool_assets")" stg exact
expect_failure \
  "release accepted the correct WIF subject on the wrong resource" \
  pymes_validate_release_pool_iam_assets \
  "$(jq '.[1].resource = "//artifactregistry.googleapis.com/projects/pymes-dev-352318/locations/us-central1/repositories/pymes"' <<<"$release_pool_assets")" \
  stg exact

no_pool_policy='{"bindings":[{"role":"roles/viewer","members":["user:reader@example.com"]}]}'
pymes_assert_policy_has_no_release_pool_members "$no_pool_policy" "policy fixture"
expect_failure \
  "direct policy accepted release-pool authority" \
  pymes_assert_policy_has_no_release_pool_members \
  "$(jq --arg principal "$pymes_release_stg_principal" '.bindings[0].members += [$principal]' <<<"$no_pool_policy")" \
  "policy fixture"

enforced_org_policy='{
  "name":"projects/pymes-dev-352318/policies/iam.disableCrossProjectServiceAccountUsage",
  "spec":{"rules":[{"enforce":true}]}
}'
pymes_validate_enforced_boolean_org_policy \
  "$enforced_org_policy" iam.disableCrossProjectServiceAccountUsage
expect_failure \
  "release accepted disabled cross-project service-account protection" \
  pymes_validate_enforced_boolean_org_policy \
  "$(jq '.spec.rules[0].enforce = false' <<<"$enforced_org_policy")" \
  iam.disableCrossProjectServiceAccountUsage

inventory_account=pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com
account_assets=$(jq -cn --arg account "$inventory_account" '
  [{
    assetType:"iam.googleapis.com/ServiceAccount",
    name:("//iam.googleapis.com/projects/pymes-dev-352318/serviceAccounts/" + $account)
  }]
')
pymes_validate_release_account_workload_inventory \
  "$account_assets" '[]' '[]' "$inventory_account" "builder fixture"
expect_failure \
  "release accepted an empty account attachment inventory" \
  pymes_validate_release_account_workload_inventory \
  '[]' '[]' '[]' "$inventory_account" "builder fixture"
expect_failure \
  "release accepted a builder attached to Cloud Run" \
  pymes_validate_release_account_workload_inventory \
  "$account_assets" \
  "$(jq -cn --arg account "$inventory_account" \
    '[{spec:{template:{spec:{serviceAccountName:$account}}}}]')" \
  '[]' "$inventory_account" "builder fixture"
expect_failure \
  "release accepted a builder attached to an old Cloud Run revision" \
  pymes_validate_release_account_workload_inventory \
  "$account_assets" '[]' '[]' "$inventory_account" "builder fixture" \
  "$(jq -cn --arg account "$inventory_account" \
    '[{spec:{serviceAccountName:$account}}]')"
expect_failure \
  "release accepted a non-service-account Cloud Asset reference" \
  pymes_validate_release_account_workload_inventory \
  "$(jq --arg account "$inventory_account" '. += [{
    assetType:"run.googleapis.com/Service",
    name:("//run.googleapis.com/projects/pymes-dev-352318/locations/us-central1/services/attached-" + $account)
  }]' <<<"$account_assets")" \
  '[]' '[]' "$inventory_account" "builder fixture"

ancestry_json='[
  {"type":"project","id":"pymes-dev-352318"},
  {"type":"folder","id":"673291958610"},
  {"type":"organization","id":"663017421195"}
]'
validate_project_ancestry_json "$ancestry_json"
expect_failure \
  "release accepted ancestry for another project" \
  validate_project_ancestry_json \
  "$(jq '.[0].id = "other-project"' <<<"$ancestry_json")"
expect_failure \
  "release accepted an unknown ancestry resource type" \
  validate_project_ancestry_json \
  "$(jq '.[1].type = "unknown"' <<<"$ancestry_json")"

build_member="serviceAccount:pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com"
ancestor_policy='{
  "bindings":[
    {
      "role":"organizations/663017421195/roles/pymesV3ReleaseFolderIamRead",
      "members":["serviceAccount:pymes-v3-gh-deploy-stg@pymes-dev-352318.iam.gserviceaccount.com"]
    },
    {"role":"roles/resourcemanager.folderAdmin","members":["user:softponti@gmail.com"]},
    {"role":"roles/viewer","members":["serviceAccount:unrelated@example.iam.gserviceaccount.com"]}
  ]
}'
validate_ancestor_policy \
  "$ancestor_policy" "$build_member" "$member" "$folder_iam_read_role" \
  "ancestor fixture"
expect_failure \
  "release accepted inherited builder authority" \
  validate_ancestor_policy \
  "$(jq --arg member "$build_member" '.bindings[0].members += [$member]' <<<"$ancestor_policy")" \
  "$build_member" "$member" "$folder_iam_read_role" "ancestor fixture"
expect_failure \
  "release accepted an unprovable ancestor group" \
  validate_ancestor_policy \
  "$(jq '.bindings[1].members += ["group:operators@example.com"]' <<<"$ancestor_policy")" \
  "$build_member" "$member" "$folder_iam_read_role" "ancestor fixture"
expect_failure \
  "release accepted public ancestor authority" \
  validate_ancestor_policy \
  "$(jq '.bindings[1].members += ["allAuthenticatedUsers"]' <<<"$ancestor_policy")" \
  "$build_member" "$member" "$folder_iam_read_role" "ancestor fixture"
expect_failure \
  "release accepted ancestor token-minting authority for another principal" \
  validate_ancestor_policy \
  "$(jq '.bindings += [{
    role:"roles/iam.serviceAccountTokenCreator",
    members:["serviceAccount:attacker@example.iam.gserviceaccount.com"]
  }]' <<<"$ancestor_policy")" \
  "$build_member" "$member" "$folder_iam_read_role" "ancestor fixture"
expect_failure \
  "release accepted inherited runtime-workload authority" \
  validate_ancestor_policy \
  "$(jq '.bindings[1].members += ["serviceAccount:pymes-v3-worker-stg@pymes-dev-352318.iam.gserviceaccount.com"]' <<<"$ancestor_policy")" \
  "$build_member" "$member" "$folder_iam_read_role" "ancestor fixture"
expect_failure \
  "release accepted a second ancestor role for the deployer" \
  validate_ancestor_policy \
  "$(jq --arg member "$member" '.bindings[1].members += [$member]' <<<"$ancestor_policy")" \
  "$build_member" "$member" "$folder_iam_read_role" "ancestor fixture"

build_email=pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com
build_principal="principal://iam.googleapis.com/projects/884236221349/locations/global/workloadIdentityPools/pymes-v3-release-pool/subject/${github_build_subject}"
build_account_json=$(jq -cn --arg email "$build_email" \
  '{email:$email,disabled:false}')
build_policy_json=$(jq -cn --arg principal "$build_principal" '
  {
    bindings: [{
      role: "roles/iam.workloadIdentityUser",
      members: [$principal]
    }]
  }
')
validate_release_account \
  "$build_account_json" "" "$build_policy_json" \
  "$build_email" "$build_principal" "builder fixture"
expect_failure \
  "release accepted a user-managed builder key" \
  validate_release_account \
  "$build_account_json" "projects/x/keys/key" "$build_policy_json" \
  "$build_email" "$build_principal" "builder fixture"
expect_failure \
  "release accepted a second WIF principal" \
  validate_release_account \
  "$build_account_json" "" \
  "$(jq '.bindings[0].members += ["principal://unexpected"]' <<<"$build_policy_json")" \
  "$build_email" "$build_principal" "builder fixture"

exact_reader_policy=$(jq -cn --arg member "$member" '
  {
    bindings: [{
      role: "roles/artifactregistry.reader",
      members: [$member]
    }]
  }
')
validate_exact_member_role \
  "$exact_reader_policy" "$member" roles/artifactregistry.reader \
  "repository fixture"
expect_failure \
  "release accepted a conditional repository role" \
  validate_exact_member_role \
  "$(jq '.bindings[0].condition={title:"temporary",expression:"true"}' <<<"$exact_reader_policy")" \
  "$member" roles/artifactregistry.reader "repository fixture"
expect_failure \
  "release accepted an additional repository role" \
  validate_exact_member_role \
  "$(jq '.bindings += [{role:"roles/artifactregistry.writer",members:[$member]}]' \
    --arg member "$member" <<<"$exact_reader_policy")" \
  "$member" roles/artifactregistry.reader "repository fixture"

runtime_account_policy=$(jq -cn --arg member "$member" '
  {
    bindings: [{
      role: "roles/iam.serviceAccountUser",
      members: [$member]
    }]
  }
')
validate_runtime_account_policy \
  "$runtime_account_policy" "$member" "runtime fixture"
expect_failure \
  "release accepted another runtime service-account caller" \
  validate_runtime_account_policy \
  "$(jq '.bindings[0].members += ["user:unexpected@example.com"]' <<<"$runtime_account_policy")" \
  "$member" "runtime fixture"
expect_failure \
  "release accepted a conditional runtime actAs binding" \
  validate_runtime_account_policy \
  "$(jq '.bindings[0].condition = {title:"bypass",expression:"true"}' <<<"$runtime_account_policy")" \
  "$member" "runtime fixture"

worker_runtime_pairs=$(runtime_effective_iam_allowlist worker stg)
grep -Fq $'//cloudresourcemanager.googleapis.com/projects/pymes-dev-352318\troles/cloudsql.client' \
  <<<"$worker_runtime_pairs"
grep -Fq $'//secretmanager.googleapis.com/projects/pymes-dev-352318/secrets/pymes-v3-stg-pergo-api-key\troles/secretmanager.secretAccessor' \
  <<<"$worker_runtime_pairs"
grep -Fq $'//cloudkms.googleapis.com/projects/pymes-dev-352318/locations/us-central1/keyRings/pymes-v3-stg/cryptoKeys/calendar-tokens\troles/cloudkms.cryptoKeyEncrypterDecrypter' \
  <<<"$worker_runtime_pairs"
grep -Fq $'//run.googleapis.com/projects/pymes-dev-352318/locations/us-central1/services/pymes-v3-stg-accounting\troles/run.invoker' \
  <<<"$worker_runtime_pairs"
if runtime_effective_iam_allowlist web stg | grep -q .; then
  echo "FAIL: stateless web runtime received effective authority" >&2
  exit 1
fi

effective_resource="//artifactregistry.googleapis.com/projects/pymes-dev-352318/locations/us-central1/repositories/pymes"
effective_analysis=$(jq -cn \
  --arg resource "$effective_resource" '
    {
      fullyExplored: true,
      mainAnalysis: {
        fullyExplored: true,
        analysisResults: [{
          fullyExplored: true,
          attachedResourceFullName: $resource,
          iamBinding: {role: "roles/artifactregistry.reader"},
          identityList: {groupEdges: []}
        }]
      },
      serviceAccountImpersonationAnalysis: []
    }
  ')
validate_effective_iam_analysis \
  "$effective_analysis" "effective fixture" \
  "$effective_resource"$'\t'"roles/artifactregistry.reader"
expect_failure \
  "release accepted effective authority outside the allowlist" \
  validate_effective_iam_analysis \
  "$(jq '.mainAnalysis.analysisResults[0].iamBinding.role = "roles/owner"' <<<"$effective_analysis")" \
  "effective fixture" \
  "$effective_resource"$'\t'"roles/artifactregistry.reader"
expect_failure \
  "release accepted incomplete effective IAM analysis" \
  validate_effective_iam_analysis \
  "$(jq '.fullyExplored = false' <<<"$effective_analysis")" \
  "effective fixture" \
  "$effective_resource"$'\t'"roles/artifactregistry.reader"
expect_failure \
  "release accepted group-derived effective IAM" \
  validate_effective_iam_analysis \
  "$(jq '.mainAnalysis.analysisResults[0].identityList.groupEdges = [{}]' <<<"$effective_analysis")" \
  "effective fixture" \
  "$effective_resource"$'\t'"roles/artifactregistry.reader"
expect_failure \
  "release accepted authority through service-account impersonation" \
  validate_effective_iam_analysis \
  "$(jq '.serviceAccountImpersonationAnalysis = [{
    fullyExplored:true,
    analysisResults:[{
      fullyExplored:true,
      attachedResourceFullName:"//cloudresourcemanager.googleapis.com/projects/pymes-dev-352318",
      iamBinding:{role:"roles/owner"},
      identityList:{groupEdges:[]}
    }]
  }]' <<<"$effective_analysis")" \
  "effective fixture" \
  "$effective_resource"$'\t'"roles/artifactregistry.reader"

bootstrap_service_policy=$(jq -cn --arg member "$member" '
  {bindings: [{role: "roles/run.admin", members: [$member]}]}
')
validate_complete_resource_authority \
  "$bootstrap_service_policy" "$member" service api stg bootstrap
expect_failure \
  "bootstrap accepted a public service invoker" \
  validate_complete_resource_authority \
  "$(jq '.bindings += [{role:"roles/run.invoker",members:["allUsers"]}]' <<<"$bootstrap_service_policy")" \
  "$member" service api stg bootstrap

fiscal_policy=$(jq -cn --arg member "$member" '
  {
    bindings: [
      {role: "roles/run.admin", members: [$member]},
      {
        role: "roles/run.invoker",
        members: [
          "serviceAccount:pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com",
          "serviceAccount:pymes-v3-worker-stg@pymes-dev-352318.iam.gserviceaccount.com"
        ]
      }
    ]
  }
')
validate_complete_resource_authority \
  "$fiscal_policy" "$member" service fiscal stg operational
validate_complete_resource_authority \
  "$fiscal_policy" "$member" service fiscal stg bootstrap
expect_failure \
  "operational accepted an unknown private invoker" \
  validate_complete_resource_authority \
  "$(jq '.bindings[1].members += ["serviceAccount:unknown@example.iam.gserviceaccount.com"]' <<<"$fiscal_policy")" \
  "$member" service fiscal stg operational
expect_failure \
  "operational accepted another resource administrator" \
  validate_complete_resource_authority \
  "$(jq '.bindings[0].members += ["user:other@example.com"]' <<<"$fiscal_policy")" \
  "$member" service fiscal stg operational

web_policy=$(jq -cn --arg member "$member" '
  {
    bindings: [
      {role: "roles/run.admin", members: [$member]},
      {role: "roles/run.invoker", members: ["allUsers"]}
    ]
  }
')
validate_complete_resource_authority \
  "$web_policy" "$member" service web stg operational
expect_failure \
  "bootstrap accepted the prior operational public projection" \
  validate_complete_resource_authority \
  "$web_policy" "$member" service web stg bootstrap
validate_complete_resource_authority \
  "$bootstrap_service_policy" "$member" service web stg operational
validate_complete_resource_authority \
  "$bootstrap_service_policy" "$member" job migrate stg operational

inverse_analysis_fixture() {
  local attached_resource="$1" effective_resource="$2" permission="$3"
  local identity="$4" role="$5"
  jq -cn \
    --arg attached "$attached_resource" \
    --arg effective "$effective_resource" \
    --arg permission "$permission" \
    --arg identity "$identity" \
    --arg role "$role" '
      {
        schemaVersion: 1,
        requestedScope: "projects/pymes-dev-352318",
        requestedPermissions: [$permission],
        responses: [{
          fullyExplored: true,
          mainAnalysis: {
            analysisQuery: {
              scope: "projects/pymes-dev-352318",
              accessSelector: {permissions: [$permission]},
              options: {
                analyzeServiceAccountImpersonation: true,
                expandGroups: true,
                expandResources: true,
                expandRoles: true,
                outputGroupEdges: true
              }
            },
            fullyExplored: true,
            analysisResults: [{
              fullyExplored: true,
              attachedResourceFullName: $attached,
              iamBinding: {role: $role, members: [$identity]},
              identityList: {
                identities: [{name: $identity}],
                groupEdges: []
              },
              accessControlLists: [{
                accesses: [{permission: $permission}],
                resources: [{fullResourceName: $effective}]
              }]
            }]
          },
          serviceAccountImpersonationAnalysis: []
        }]
      }
    '
}

inverse_run_resource="//run.googleapis.com/projects/pymes-dev-352318/locations/us-central1/services/pymes-v3-stg-api"
inverse_run_permission=run.services.update
inverse_deployer="serviceAccount:pymes-v3-gh-deploy-stg@pymes-dev-352318.iam.gserviceaccount.com"
inverse_run_analysis=$(inverse_analysis_fixture \
  "//cloudresourcemanager.googleapis.com/projects/pymes-dev-352318" \
  "$inverse_run_resource" "$inverse_run_permission" \
  "$inverse_deployer" roles/run.admin)
pymes_validate_inverse_permission_analysis \
  "$inverse_run_analysis" "inverse Run fixture" \
  pymes-dev-352318 884236221349 \
  "$inverse_run_resource"$'\t'"$inverse_run_permission" \
  -- \
  "$inverse_run_resource"$'\t'"$inverse_run_permission"$'\t'"$inverse_deployer"

expect_failure \
  "inverse analysis accepted project-wide Run Admin through another role holder" \
  pymes_validate_inverse_permission_analysis \
  "$(jq '
    .responses[0].mainAnalysis.analysisResults[0].iamBinding = {
      role:"projects/pymes-dev-352318/roles/customRunRelease",
      members:["serviceAccount:rogue@example.iam.gserviceaccount.com"]
    }
    | .responses[0].mainAnalysis.analysisResults[0].identityList.identities = [
        {name:"serviceAccount:rogue@example.iam.gserviceaccount.com"}
      ]
  ' <<<"$inverse_run_analysis")" \
  "inverse Run fixture" pymes-dev-352318 884236221349 \
  "$inverse_run_resource"$'\t'"$inverse_run_permission" \
  -- \
  "$inverse_run_resource"$'\t'"$inverse_run_permission"$'\t'"$inverse_deployer"

inverse_secret_resource="//secretmanager.googleapis.com/projects/pymes-dev-352318/secrets/pymes-v3-stg-database-url"
inverse_secret_permission=secretmanager.versions.access
inverse_api="serviceAccount:pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com"
inverse_secret_analysis=$(inverse_analysis_fixture \
  "//cloudresourcemanager.googleapis.com/projects/pymes-dev-352318" \
  "${inverse_secret_resource}/versions/1" "$inverse_secret_permission" \
  "$inverse_api" roles/secretmanager.secretAccessor)
pymes_validate_inverse_permission_analysis \
  "$inverse_secret_analysis" "inverse Secret fixture" \
  pymes-dev-352318 884236221349 \
  "$inverse_secret_resource"$'\t'"$inverse_secret_permission" \
  -- \
  "$inverse_secret_resource"$'\t'"$inverse_secret_permission"$'\t'"$inverse_api"
expect_failure \
  "inverse analysis accepted a cross-environment Secret accessor" \
  pymes_validate_inverse_permission_analysis \
  "$(jq '
    .responses[0].mainAnalysis.analysisResults[0].iamBinding = {
      role:"projects/pymes-dev-352318/roles/customSecretReader",
      members:["serviceAccount:pymes-v3-api-prd@pymes-dev-352318.iam.gserviceaccount.com"]
    }
    | .responses[0].mainAnalysis.analysisResults[0].identityList.identities = [
        {name:"serviceAccount:pymes-v3-api-prd@pymes-dev-352318.iam.gserviceaccount.com"}
      ]
  ' <<<"$inverse_secret_analysis")" \
  "inverse Secret fixture" pymes-dev-352318 884236221349 \
  "$inverse_secret_resource"$'\t'"$inverse_secret_permission" \
  -- \
  "$inverse_secret_resource"$'\t'"$inverse_secret_permission"$'\t'"$inverse_api"

inverse_kms_resource="//cloudkms.googleapis.com/projects/pymes-dev-352318/locations/us-central1/keyRings/pymes-v3-stg/cryptoKeys/internal-jwt-signing"
inverse_kms_permission=cloudkms.cryptoKeyVersions.useToSign
inverse_kms_analysis=$(inverse_analysis_fixture \
  "//cloudresourcemanager.googleapis.com/projects/pymes-dev-352318" \
  "${inverse_kms_resource}/cryptoKeyVersions/7" "$inverse_kms_permission" \
  "$inverse_api" roles/cloudkms.signer)
pymes_validate_inverse_permission_analysis \
  "$inverse_kms_analysis" "inverse KMS fixture" \
  pymes-dev-352318 884236221349 \
  "$inverse_kms_resource"$'\t'"$inverse_kms_permission" \
  -- \
  "$inverse_kms_resource"$'\t'"$inverse_kms_permission"$'\t'"$inverse_api"
expect_failure \
  "inverse analysis accepted cryptoOperator as an alternate KMS signer" \
  pymes_validate_inverse_permission_analysis \
  "$(jq '
    .responses[0].mainAnalysis.analysisResults[0].iamBinding = {
      role:"roles/cloudkms.cryptoOperator",
      members:["serviceAccount:rogue@example.iam.gserviceaccount.com"]
    }
    | .responses[0].mainAnalysis.analysisResults[0].identityList.identities = [
        {name:"serviceAccount:rogue@example.iam.gserviceaccount.com"}
      ]
  ' <<<"$inverse_kms_analysis")" \
  "inverse KMS fixture" pymes-dev-352318 884236221349 \
  "$inverse_kms_resource"$'\t'"$inverse_kms_permission" \
  -- \
  "$inverse_kms_resource"$'\t'"$inverse_kms_permission"$'\t'"$inverse_api"
expect_failure \
  "inverse analysis accepted group-derived authority on a protected resource" \
  pymes_validate_inverse_permission_analysis \
  "$(jq '
    .responses[0].mainAnalysis.analysisResults[0].identityList.groupEdges = [
      {source:"group:release@example.com",target:"serviceAccount:pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com"}
    ]
  ' <<<"$inverse_kms_analysis")" \
  "inverse KMS fixture" pymes-dev-352318 884236221349 \
  "$inverse_kms_resource"$'\t'"$inverse_kms_permission" \
  -- \
  "$inverse_kms_resource"$'\t'"$inverse_kms_permission"$'\t'"$inverse_api"
expect_failure \
  "inverse analysis accepted a partial Policy Analyzer response" \
  pymes_validate_inverse_permission_analysis \
  "$(jq '.responses[0].fullyExplored = false' <<<"$inverse_kms_analysis")" \
  "inverse KMS fixture" pymes-dev-352318 884236221349 \
  "$inverse_kms_resource"$'\t'"$inverse_kms_permission" \
  -- \
  "$inverse_kms_resource"$'\t'"$inverse_kms_permission"$'\t'"$inverse_api"

# Unrelated IAM in the shared project is deliberately outside this resource
# boundary. The inverse verifier constrains effective access to Pymes resources
# without canonicalizing every binding owned by another product.
pymes_validate_inverse_permission_analysis \
  "$(jq --arg resource "//run.googleapis.com/projects/pymes-dev-352318/locations/us-central1/services/shared-unrelated-runtime-stg" \
    '.responses[0].mainAnalysis.analysisResults[0].accessControlLists[0].resources[0].fullResourceName = $resource' \
    <<<"$inverse_run_analysis")" \
  "inverse shared-project fixture" pymes-dev-352318 884236221349 \
  "$inverse_run_resource"$'\t'"$inverse_run_permission" \
  --

inverse_injected_resource=
inverse_injected_permission=
inverse_injected_identity=
inverse_injected_role=
pymes_read_inverse_permission_analysis() {
  local fixture_project="$1" permissions_csv="$2"
  local permissions_json results_json
  permissions_json=$(tr ',' '\n' <<<"$permissions_csv" |
    jq -Rsc 'split("\n") | map(select(length > 0)) | unique | sort')
  results_json='[]'
  if [[ -n "$inverse_injected_resource" ]]; then
    results_json=$(jq -cn \
      --arg resource "$inverse_injected_resource" \
      --arg permission "$inverse_injected_permission" \
      --arg identity "$inverse_injected_identity" \
      --arg role "$inverse_injected_role" '
        [{
          fullyExplored: true,
          attachedResourceFullName:
            "//cloudresourcemanager.googleapis.com/projects/pymes-dev-352318",
          iamBinding: {role: $role, members: [$identity]},
          identityList: {
            identities: [{name: $identity}],
            groupEdges: []
          },
          accessControlLists: [{
            accesses: [{permission: $permission}],
            resources: [{fullResourceName: $resource}]
          }]
        }]
      ')
  fi
  jq -cn \
    --arg scope "projects/${fixture_project}" \
    --argjson permissions "$permissions_json" \
    --argjson results "$results_json" '
      {
        schemaVersion: 1,
        requestedScope: $scope,
        requestedPermissions: $permissions,
        responses: [{
          fullyExplored: true,
          mainAnalysis: {
            analysisQuery: {
              scope: $scope,
              accessSelector: {permissions: $permissions},
              options: {
                analyzeServiceAccountImpersonation: true,
                expandGroups: true,
                expandResources: true,
                expandRoles: true,
                outputGroupEdges: true
              }
            },
            fullyExplored: true,
            analysisResults: $results
          },
          serviceAccountImpersonationAnalysis: []
        }]
      }
    '
}

pymes_verify_release_inverse_authority \
  pymes-dev-352318 884236221349 us-central1 stg present pymes

inverse_bypass_cases=(
  "//cloudresourcemanager.googleapis.com/projects/pymes-dev-352318"$'\t'"resourcemanager.projects.setIamPolicy"$'\t'"projects/pymes-dev-352318/roles/customProjectAdmin"
  "//iam.googleapis.com/projects/pymes-dev-352318/serviceAccounts/pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com"$'\t'"iam.serviceAccounts.getAccessToken"$'\t'"roles/iam.serviceAccountTokenCreator"
  "//run.googleapis.com/projects/pymes-dev-352318/locations/us-central1/services/pymes-v3-stg-api"$'\t'"run.services.update"$'\t'"projects/pymes-dev-352318/roles/customRunRelease"
  "//secretmanager.googleapis.com/projects/pymes-dev-352318/secrets/pymes-v3-stg-database-url/versions/1"$'\t'"secretmanager.versions.access"$'\t'"projects/pymes-dev-352318/roles/customSecretReader"
  "//artifactregistry.googleapis.com/projects/pymes-dev-352318/locations/us-central1/repositories/pymes"$'\t'"artifactregistry.repositories.uploadArtifacts"$'\t'"projects/pymes-dev-352318/roles/customArtifactWriter"
  "//cloudkms.googleapis.com/projects/pymes-dev-352318/locations/us-central1/keyRings/pymes-v3-stg/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/1"$'\t'"cloudkms.cryptoKeyVersions.useToSign"$'\t'"roles/cloudkms.cryptoOperator"
)
inverse_injected_identity="serviceAccount:pymes-v3-api-prd@pymes-dev-352318.iam.gserviceaccount.com"
for inverse_case in "${inverse_bypass_cases[@]}"; do
  IFS=$'\t' read -r \
    inverse_injected_resource inverse_injected_permission inverse_injected_role \
    <<<"$inverse_case"
  expect_failure \
    "release boundary accepted alternate/custom or cross-environment effective permission: $inverse_injected_permission" \
    pymes_verify_release_inverse_authority \
    pymes-dev-352318 884236221349 us-central1 stg present pymes
done
expect_failure \
  "Cloud Run KMS-only boundary accepted alternate signer authority" \
  pymes_verify_release_inverse_authority \
  pymes-dev-352318 884236221349 us-central1 stg present pymes kms

echo "Release authority policy tests passed"
