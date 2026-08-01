#!/usr/bin/env bash
set -euo pipefail

# Verifies the authority used by the protected release workflow. Neither the
# zero-traffic bootstrap nor an operational release may receive project-wide
# Cloud Run administration in the shared project. Both stages run only after
# the initial resources have been seeded by a reviewed human operator and the
# deployer has received resource-scoped administration through finalize.

expected_project=pymes-dev-352318
expected_project_number=884236221349
expected_region=us-central1
artifact_repository=pymes
pool=pymes-v3-release-pool
provider=github
github_repository=devpablocristo/pymes
github_repository_id=1173650578
github_repository_owner_id=81805584
github_workflow_ref='devpablocristo/pymes/.github/workflows/v3-release.yml@refs/heads/main'
github_build_subject='repo:devpablocristo/pymes:ref:refs/heads/main'
github_stg_subject='repo:devpablocristo/pymes:environment:stg'
github_prd_subject='repo:devpablocristo/pymes:environment:prd'
attribute_condition="assertion.repository_id=='${github_repository_id}' && assertion.repository_owner_id=='${github_repository_owner_id}' && assertion.repository=='${github_repository}' && assertion.ref=='refs/heads/main' && assertion.ref_protected=='true' && assertion.workflow_ref=='${github_workflow_ref}' && assertion.event_name=='workflow_dispatch' && (assertion.sub=='${github_build_subject}' || assertion.sub=='${github_stg_subject}' || assertion.sub=='${github_prd_subject}')"
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=release-authority-policy.sh
source "$script_dir/release-authority-policy.sh"
project_iam_read_role="projects/${expected_project}/roles/pymesV3ReleaseProjectIamRead"
kms_policy_read_role="projects/${expected_project}/roles/pymesV3ReleaseKmsPolicyRead"
organization_iam_read_role="organizations/${pymes_release_expected_organization}/roles/pymesV3ReleaseOrganizationIamRead"
folder_iam_read_role="organizations/${pymes_release_expected_organization}/roles/pymesV3ReleaseFolderIamRead"
runtime_accounts=(
  api web worker provision fiscal accounting accounting-admin
  migrate fiscal-migrate acct-migrate
)
release_services=(api web worker fiscal accounting accounting-admin)
release_jobs=(migrate fiscal-migrate accounting-migrate accounting-grants provision-org)

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

validate_project_authority() {
  local policy_json="$1" environment="$2" stage="$3"
  local member custom_role
  member="serviceAccount:pymes-v3-gh-deploy-${environment}@${expected_project}.iam.gserviceaccount.com"
  custom_role="projects/${expected_project}/roles/pymesV3ReleaseProjectIamRead"

  jq -e \
    --arg member "$member" \
    --arg custom_role "$custom_role" '
      [
        .bindings[]?
        | select((.members // []) | index($member) != null)
      ] as $bindings
      | (
          [
            $bindings[]
            | select(.role != "roles/run.admin")
            | select(.condition == null)
            | .role
          ] | sort
        ) == (
          [
            "roles/serviceusage.serviceUsageConsumer",
            $custom_role
          ] | sort
        )
      and all(
        $bindings[];
        .condition == null
      )
    ' <<<"$policy_json" >/dev/null || {
      echo "release deployer project roles differ from the reviewed base allowlist" >&2
      return 1
    }

  case "$stage" in bootstrap|operational) ;; *)
    echo "unsupported release authority stage: $stage" >&2
    return 1
  esac
  if jq -e \
    --arg member "$member" '
      any(
        .bindings[]?;
        .role == "roles/run.admin" and
        ((.members // []) | index($member) != null)
      )
    ' <<<"$policy_json" >/dev/null; then
    echo "release workflow forbids every project-scoped roles/run.admin binding" >&2
    return 1
  fi
}

validate_resource_authority() {
  local policy_json="$1" member="$2" expected_state="$3" description="$4"
  local count
  count=$(jq -er \
    --arg member "$member" '
      [
        .bindings[]?
        | select((.members // []) | index($member) != null)
      ] as $bindings
      | select(all($bindings[]; .role == "roles/run.admin" and .condition == null))
      | $bindings
      | length
    ' <<<"$policy_json") || {
    echo "$description has a conditional or non-reviewed deployer role" >&2
    return 1
  }
  case "$expected_state" in
    absent)
      [[ "$count" == "0" ]] || {
        echo "$description must not grant resource roles before finalize" >&2
        return 1
      }
      ;;
    exact)
      [[ "$count" == "1" ]] || {
        echo "$description must grant exactly one resource-scoped roles/run.admin" >&2
        return 1
      }
      ;;
    *)
      echo "unsupported resource authority state: $expected_state" >&2
      return 1
      ;;
  esac
}

validate_custom_role_json() {
  local role_json="$1"
  jq -e \
    --argjson expected "$(pymes_release_project_iam_read_permissions_json)" '
    .name == "projects/pymes-dev-352318/roles/pymesV3ReleaseProjectIamRead" and
    .deleted != true and
    (.stage // "GA") == "GA" and
    (.includedPermissions | sort) == $expected
  ' <<<"$role_json" >/dev/null || {
    echo "release project verifier custom role differs from the reviewed permission set" >&2
    return 1
  }
}

validate_kms_custom_role_json() {
  local role_json="$1"
  jq -e \
    --argjson expected "$(pymes_release_kms_policy_read_permissions_json)" '
    .name == "projects/pymes-dev-352318/roles/pymesV3ReleaseKmsPolicyRead" and
    .deleted != true and
    (.stage // "GA") == "GA" and
    (.includedPermissions | sort) == $expected
  ' <<<"$role_json" >/dev/null || {
    echo "release KMS verifier custom role differs from the reviewed permission set" >&2
    return 1
  }
}

validate_organization_iam_custom_role_json() {
  local role_json="$1"
  jq -e \
    --arg name "$organization_iam_read_role" \
    --argjson expected "$(pymes_release_organization_iam_read_permissions_json)" '
    .name == $name and
    .deleted != true and
    (.stage // "GA") == "GA" and
    (.includedPermissions | sort) == $expected
  ' <<<"$role_json" >/dev/null || {
    echo "release organization-IAM verifier role differs from the reviewed permission set" >&2
    return 1
  }
}

validate_folder_iam_custom_role_json() {
  local role_json="$1"
  jq -e \
    --arg name "$folder_iam_read_role" \
    --argjson expected "$(pymes_release_folder_iam_read_permissions_json)" '
    .name == $name and
    .deleted != true and
    (.stage // "GA") == "GA" and
    (.includedPermissions | sort) == $expected
  ' <<<"$role_json" >/dev/null || {
    echo "release folder-IAM verifier role differs from the reviewed permission set" >&2
    return 1
  }
}

validate_wif_pool_json() {
  local pool_json="$1"
  jq -e \
    --arg name "projects/${expected_project_number}/locations/global/workloadIdentityPools/${pool}" '
      .name == $name and
      .state == "ACTIVE" and
      .disabled != true
    ' <<<"$pool_json" >/dev/null || {
      echo "release WIF pool is disabled or differs from the reviewed pool" >&2
      return 1
    }
}

validate_wif_provider_json() {
  local provider_json="$1"
  jq -e \
    --arg condition "$attribute_condition" \
    --arg name "projects/${expected_project_number}/locations/global/workloadIdentityPools/${pool}/providers/${provider}" '
      .name == $name and
      .oidc.issuerUri == "https://token.actions.githubusercontent.com" and
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
      .state == "ACTIVE" and
      .disabled != true
    ' <<<"$provider_json" >/dev/null || {
      echo "release WIF provider differs from the reviewed GitHub boundary" >&2
      return 1
    }
}

validate_project_ancestry_json() {
  local ancestry_json="$1"
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
    ' <<<"$ancestry_json" >/dev/null || {
      echo "release verifier cannot prove the complete expected project ancestry" >&2
      return 1
    }
}

validate_ancestor_policy() {
  local policy_json="$1" build_member="$2" deploy_member="$3"
  local expected_deploy_role="$4" description="$5"
  jq -e \
    --arg build_member "$build_member" \
    --arg deploy_member "$deploy_member" \
    --arg expected_deploy_role "$expected_deploy_role" '
      [
        .bindings[]?
        | select((.members // []) | index($deploy_member) != null)
        | {
            role: .role,
            condition: (.condition // null),
            occurrences: ([.members[] | select(. == $deploy_member)] | length)
          }
      ] == [{
        role: $expected_deploy_role,
        condition: null,
        occurrences: 1
      }] and
      all(
        .bindings[]?;
        all(
          .members[]?;
          . != $build_member and
          (
            if startswith("serviceAccount:pymes-v3-gh-deploy-") then
              (
                . == "serviceAccount:pymes-v3-gh-deploy-stg@pymes-dev-352318.iam.gserviceaccount.com" or
                . == "serviceAccount:pymes-v3-gh-deploy-prd@pymes-dev-352318.iam.gserviceaccount.com"
              ) and
              (. as $release_member | any(
                  $policy_bindings[]?;
                  .role == $expected_deploy_role and
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
      )
    ' --argjson policy_bindings "$(jq '.bindings // []' <<<"$policy_json")" \
    <<<"$policy_json" >/dev/null || {
      echo "$description contains release authority or group, domain, public, principal-set, or otherwise unprovable inherited authority" >&2
      return 1
    }
}

canonical_policy_bindings() {
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

validate_release_account() {
  local account_json="$1" user_keys="$2" policy_json="$3"
  local email="$4" principal="$5" description="$6"
  local actual expected
  jq -e --arg email "$email" '
    .email == $email and .disabled != true
  ' <<<"$account_json" >/dev/null || {
    echo "$description is missing, disabled, or has an unexpected identity" >&2
    return 1
  }
  [[ -z "$user_keys" ]] || {
    echo "$description has a prohibited user-managed key" >&2
    return 1
  }
  actual=$(canonical_policy_bindings <<<"$policy_json")
  expected=$(jq -cnS --arg principal "$principal" '
    [{
      role: "roles/iam.workloadIdentityUser",
      condition: null,
      members: [$principal]
    }]
  ')
  [[ "$actual" == "$expected" ]] || {
    echo "$description WIF trust is not the single reviewed subject" >&2
    return 1
  }
}

validate_exact_member_role() {
  local policy_json="$1" member="$2" role="$3" description="$4"
  jq -e --arg member "$member" --arg role "$role" '
    [
      .bindings[]?
      | select((.members // []) | index($member) != null)
      | {role: .role, condition: (.condition // null)}
    ] == [{role: $role, condition: null}]
  ' <<<"$policy_json" >/dev/null || {
    echo "$description does not grant exactly one unconditional $role" >&2
    return 1
  }
}

validate_runtime_account_policy() {
  local policy_json="$1" deploy_member="$2" description="$3"
  local actual expected
  actual=$(canonical_policy_bindings <<<"$policy_json")
  expected=$(jq -cnS --arg member "$deploy_member" '
    [{
      role: "roles/iam.serviceAccountUser",
      condition: null,
      members: [$member]
    }]
  ')
  [[ "$actual" == "$expected" ]] || {
    echo "$description IAM policy is not the single reviewed deployer actAs binding" >&2
    return 1
  }
}

emit_project_runtime_pair() {
  local role="$1"
  printf '%s\t%s\n' \
    "//cloudresourcemanager.googleapis.com/projects/${expected_project}" "$role" \
    "//cloudresourcemanager.googleapis.com/projects/${expected_project_number}" "$role"
}

emit_runtime_pair() {
  local service="$1" resource_suffix="$2" role="$3"
  printf '%s\t%s\n' \
    "//${service}.googleapis.com/projects/${expected_project}/${resource_suffix}" "$role" \
    "//${service}.googleapis.com/projects/${expected_project_number}/${resource_suffix}" "$role"
}

runtime_effective_iam_allowlist() {
  local component="$1" environment="$2"
  local prefix="pymes-v3-${environment}" keyring="pymes-v3-${environment}"
  local secret target
  local -a secrets=() kms_grants=() run_targets=()

  [[ "$component" == "web" ]] ||
    emit_project_runtime_pair roles/cloudsql.client

  case "$component" in
    api)
      secrets=(
        clerk-secret-key clerk-webhook-secret scheduling-action-token-secret
        pergo-webhook-secrets google-client-secret database-url
      )
      kms_grants=(
        "calendar-tokens|roles/cloudkms.cryptoKeyEncrypterDecrypter"
        "internal-jwt-signing|roles/cloudkms.signer"
        "internal-jwt-signing|roles/cloudkms.publicKeyViewer"
      )
      run_targets=(fiscal)
      ;;
    web)
      ;;
    worker)
      secrets=(
        scheduling-action-token-secret google-client-secret
        worker-database-url pergo-api-key
      )
      kms_grants=(
        "calendar-tokens|roles/cloudkms.cryptoKeyEncrypterDecrypter"
        "internal-jwt-signing|roles/cloudkms.signer"
        "internal-jwt-signing|roles/cloudkms.publicKeyViewer"
      )
      run_targets=(fiscal accounting)
      ;;
    provision)
      secrets=(database-url)
      kms_grants=(
        "internal-jwt-signing|roles/cloudkms.signer"
        "internal-jwt-signing|roles/cloudkms.publicKeyViewer"
      )
      run_targets=(accounting-admin)
      ;;
    fiscal)
      secrets=(fiscal-database-url)
      kms_grants=(
        "fiscal-vault|roles/cloudkms.cryptoKeyEncrypterDecrypter"
      )
      ;;
    accounting)
      secrets=(accounting-database-url)
      ;;
    accounting-admin)
      secrets=(accounting-admin-database-url)
      ;;
    migrate)
      secrets=(migrate-database-url)
      ;;
    fiscal-migrate)
      secrets=(fiscal-migrate-database-url)
      ;;
    acct-migrate)
      secrets=(accounting-migrate-database-url)
      ;;
    *)
      echo "unsupported runtime authority component: $component" >&2
      return 1
      ;;
  esac

  for secret in "${secrets[@]}"; do
    emit_runtime_pair secretmanager "secrets/${prefix}-${secret}" \
      roles/secretmanager.secretAccessor
  done
  for target in "${kms_grants[@]}"; do
    emit_runtime_pair cloudkms \
      "locations/${expected_region}/keyRings/${keyring}/cryptoKeys/${target%%|*}" \
      "${target#*|}"
  done
  for target in "${run_targets[@]}"; do
    emit_runtime_pair run \
      "locations/${expected_region}/services/${prefix}-${target}" \
      roles/run.invoker
  done
}

validate_effective_iam_analysis() {
  local analysis_json="$1" description="$2"
  shift 2
  local actual_pairs allowed_pairs unexpected_pairs
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
    echo "$description effective IAM analysis is incomplete or group-derived" >&2
    return 1
  }
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
    echo "$description has effective IAM outside the reviewed resource allowlist" >&2
    printf '%s\n' "$unexpected_pairs" >&2
    return 1
  }
}

read_effective_iam_analysis() {
  local email="$1"
  gcloud asset analyze-iam-policy \
    --project="$expected_project" \
    --identity="serviceAccount:${email}" \
    --expand-groups \
    --expand-roles \
    --analyze-service-account-impersonation \
    --output-group-edges \
    --execution-timeout=120s \
    --show-response \
    --format=json
}

validate_complete_resource_authority() {
  local policy_json="$1" member="$2" kind="$3" component="$4"
  local environment="$5" stage="$6" base projected actual allow_projected=false
  local -a invokers=()
  if [[ "$kind" == "service" ]]; then
    case "$component" in
      api|web)
        invokers=(allUsers)
        [[ "$stage" == "operational" ]] && allow_projected=true
        ;;
      fiscal)
        invokers=(
          "serviceAccount:pymes-v3-api-${environment}@${expected_project}.iam.gserviceaccount.com"
          "serviceAccount:pymes-v3-worker-${environment}@${expected_project}.iam.gserviceaccount.com"
        )
        allow_projected=true
        ;;
      accounting)
        invokers=(
          "serviceAccount:pymes-v3-worker-${environment}@${expected_project}.iam.gserviceaccount.com"
        )
        allow_projected=true
        ;;
      accounting-admin)
        invokers=(
          "serviceAccount:pymes-v3-provision-${environment}@${expected_project}.iam.gserviceaccount.com"
        )
        allow_projected=true
        ;;
      worker)
        ;;
      *)
        echo "unsupported Cloud Run service component: $component" >&2
        return 1
        ;;
    esac
  elif [[ "$kind" != "job" && "$kind" != "service" ]]; then
    echo "unsupported Cloud Run resource kind: $kind" >&2
    return 1
  fi

  base=$(jq -cn --arg member "$member" '
    [{role: "roles/run.admin", condition: null, members: [$member]}]
  ')
  projected=$(printf '%s\n' "${invokers[@]:-}" |
    jq -Rsc --arg member "$member" '
      (split("\n") | map(select(length > 0)) | sort) as $invokers
      | (
          [{role: "roles/run.admin", condition: null, members: [$member]}]
          + (
              if ($invokers | length) == 0 then
                []
              else
                [{role: "roles/run.invoker", condition: null, members: $invokers}]
              end
            )
        )
      | sort_by(.role)
    ')
  actual=$(jq -cS '
    [
      .bindings[]?
      | {
          role: .role,
          condition: (.condition // null),
          members: ((.members // []) | sort)
        }
    ] | sort_by(.role)
  ' <<<"$policy_json")
  if [[ "$actual" == "$(jq -cS . <<<"$base")" ]]; then
    return 0
  fi
  [[ "$allow_projected" == "true" &&
     "$actual" == "$(jq -cS . <<<"$projected")" ]] || {
    echo "Cloud Run $kind $component IAM is not an allowed seed or prior-stage state" >&2
    return 1
  }
}

main() {
  local project=${PYMES_GCP_PROJECT:-}
  local region=${PYMES_GCP_REGION:-}
  local environment=${PYMES_DEPLOY_ENV:-}
  local stage=${PYMES_DEPLOY_STAGE:-}
  local build_email build_member build_principal deploy_email deploy_principal
  local deploy_subject member project_policy repository_policy component name
  local policy role_json pool_json provider_json account_json account_policy
  local user_keys runtime_email runtime_json keyring secret analysis_json kms_key
  local ancestry_json ancestor_type ancestor_id ancestor_policy expected_ancestor_role
  local release_pool_assets
  local active_account org_policy_json constraint
  local -a release_secrets build_allowed_pairs deploy_allowed_pairs runtime_pairs

  [[ "$project" == "$expected_project" ]] || {
    echo "release authority verification is restricted to $expected_project" >&2
    exit 2
  }
  [[ "$region" == "$expected_region" ]] || {
    echo "release authority verification is restricted to $expected_region" >&2
    exit 2
  }
  case "$environment" in stg|prd) ;; *)
    echo "PYMES_DEPLOY_ENV must be stg or prd" >&2
    exit 2
  esac
  case "$stage" in bootstrap|operational) ;; *)
    echo "PYMES_DEPLOY_STAGE must be bootstrap or operational" >&2
    exit 2
  esac
  for command in gcloud jq; do
    command -v "$command" >/dev/null 2>&1 || {
      echo "$command is required" >&2
      exit 1
    }
  done

  for constraint in \
    iam.disableCrossProjectServiceAccountUsage \
    iam.disableServiceAccountKeyCreation; do
    org_policy_json=$(gcloud org-policies describe "constraints/${constraint}" \
      --project="$project" --effective --format=json)
    pymes_validate_enforced_boolean_org_policy \
      "$org_policy_json" "$constraint"
  done

  pool_json=$(gcloud iam workload-identity-pools describe "$pool" \
    --project="$project" --location=global --format=json)
  validate_wif_pool_json "$pool_json"
  provider_json=$(gcloud iam workload-identity-pools providers describe "$provider" \
    --project="$project" --location=global \
    --workload-identity-pool="$pool" --format=json)
  validate_wif_provider_json "$provider_json"
  release_pool_assets=$(pymes_search_release_pool_iam_assets "$project")
  pymes_validate_release_pool_iam_assets \
    "$release_pool_assets" "$environment" exact

  project_policy=$(gcloud projects get-iam-policy "$project" --format=json)
  repository_policy=$(gcloud artifacts repositories get-iam-policy \
    "$artifact_repository" --project="$project" --location="$region" \
    --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$project_policy" "release project IAM"
  pymes_assert_policy_has_no_release_pool_members \
    "$repository_policy" "release Artifact Registry IAM"

  build_email="pymes-v3-gh-build@${project}.iam.gserviceaccount.com"
  build_member="serviceAccount:${build_email}"
  build_principal="principal://iam.googleapis.com/projects/${expected_project_number}/locations/global/workloadIdentityPools/${pool}/subject/${github_build_subject}"
  account_json=$(gcloud iam service-accounts describe "$build_email" \
    --project="$project" --format=json)
  user_keys=$(gcloud iam service-accounts keys list \
    --iam-account="$build_email" --project="$project" \
    --managed-by=user --format='value(name)')
  account_policy=$(gcloud iam service-accounts get-iam-policy "$build_email" \
    --project="$project" --format=json)
  validate_release_account \
    "$account_json" "$user_keys" "$account_policy" \
    "$build_email" "$build_principal" "release builder"
  pymes_verify_release_account_not_attached \
    "$project" "$build_email" "release builder"
  validate_exact_member_role \
    "$project_policy" "$build_member" \
    roles/serviceusage.serviceUsageConsumer \
    "release builder project IAM"
  validate_exact_member_role \
    "$repository_policy" "$build_member" \
    roles/artifactregistry.writer \
    "release builder Artifact Registry IAM"

  deploy_email="pymes-v3-gh-deploy-${environment}@${project}.iam.gserviceaccount.com"
  member="serviceAccount:${deploy_email}"
  active_account=$(gcloud auth list \
    --filter=status:ACTIVE --format='value(account)')
  [[ "$active_account" == "$deploy_email" ]] || {
    echo "release authority verifier must run as the exact environment deployer" >&2
    exit 1
  }
  case "$environment" in
    stg) deploy_subject=$github_stg_subject ;;
    prd) deploy_subject=$github_prd_subject ;;
  esac
  deploy_principal="principal://iam.googleapis.com/projects/${expected_project_number}/locations/global/workloadIdentityPools/${pool}/subject/${deploy_subject}"
  account_json=$(gcloud iam service-accounts describe "$deploy_email" \
    --project="$project" --format=json)
  user_keys=$(gcloud iam service-accounts keys list \
    --iam-account="$deploy_email" --project="$project" \
    --managed-by=user --format='value(name)')
  account_policy=$(gcloud iam service-accounts get-iam-policy "$deploy_email" \
    --project="$project" --format=json)
  validate_release_account \
    "$account_json" "$user_keys" "$account_policy" \
    "$deploy_email" "$deploy_principal" "release deployer"
  pymes_verify_release_account_not_attached \
    "$project" "$deploy_email" "release deployer"
  validate_project_authority "$project_policy" "$environment" "$stage"
  validate_exact_member_role \
    "$repository_policy" "$member" \
    roles/artifactregistry.reader \
    "release deployer Artifact Registry IAM"
  role_json=$(gcloud iam roles describe pymesV3ReleaseProjectIamRead \
    --project="$project" --format=json)
  validate_custom_role_json "$role_json"
  role_json=$(gcloud iam roles describe pymesV3ReleaseKmsPolicyRead \
    --project="$project" --format=json)
  validate_kms_custom_role_json "$role_json"
  role_json=$(gcloud iam roles describe pymesV3ReleaseOrganizationIamRead \
    --organization="$pymes_release_expected_organization" --format=json)
  validate_organization_iam_custom_role_json "$role_json"
  role_json=$(gcloud iam roles describe pymesV3ReleaseFolderIamRead \
    --organization="$pymes_release_expected_organization" --format=json)
  validate_folder_iam_custom_role_json "$role_json"

  ancestry_json=$(gcloud projects get-ancestors "$project" --format=json)
  validate_project_ancestry_json "$ancestry_json"
  while IFS=$'\t' read -r ancestor_type ancestor_id; do
    case "$ancestor_type" in
      project)
        continue
        ;;
      folder)
        ancestor_policy=$(gcloud resource-manager folders get-iam-policy \
          "$ancestor_id" --format=json)
        expected_ancestor_role=$folder_iam_read_role
        ;;
      organization)
        ancestor_policy=$(gcloud organizations get-iam-policy \
          "$ancestor_id" --format=json)
        expected_ancestor_role=$organization_iam_read_role
        ;;
      *)
        echo "unsupported project ancestor type: $ancestor_type" >&2
        exit 1
        ;;
    esac
    validate_ancestor_policy \
      "$ancestor_policy" "$build_member" "$member" "$expected_ancestor_role" \
      "release ancestor ${ancestor_type}/${ancestor_id}"
  done < <(jq -r '.[] | [.type, (.id | tostring)] | @tsv' <<<"$ancestry_json")

  mapfile -t release_secrets < <(release_secret_names "$environment")
  for secret in "${release_secrets[@]}"; do
    gcloud secrets describe "$secret" --project="$project" >/dev/null
    policy=$(gcloud secrets get-iam-policy "$secret" \
      --project="$project" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy" "release secret $secret IAM"
    validate_exact_member_role \
      "$policy" "$member" roles/secretmanager.viewer \
      "release deployer on secret $secret"
  done

  for component in "${runtime_accounts[@]}"; do
    runtime_email="pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
    runtime_json=$(gcloud iam service-accounts describe "$runtime_email" \
      --project="$project" --format=json)
    jq -e --arg email "$runtime_email" '
      .email == $email and .disabled != true
    ' <<<"$runtime_json" >/dev/null || {
      echo "runtime identity is missing, disabled, or unexpected: $runtime_email" >&2
      exit 1
    }
    user_keys=$(gcloud iam service-accounts keys list \
      --iam-account="$runtime_email" --project="$project" \
      --managed-by=user --format='value(name)')
    [[ -z "$user_keys" ]] || {
      echo "runtime identity has a prohibited user-managed key: $runtime_email" >&2
      exit 1
    }
    policy=$(gcloud iam service-accounts get-iam-policy "$runtime_email" \
      --project="$project" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy" "runtime identity $runtime_email IAM"
    validate_runtime_account_policy \
      "$policy" "$member" "runtime identity $runtime_email"
  done

  keyring="pymes-v3-${environment}"
  gcloud kms keyrings describe "$keyring" \
    --project="$project" --location="$region" >/dev/null
  policy=$(gcloud kms keyrings get-iam-policy "$keyring" \
    --project="$project" --location="$region" --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$policy" "release KMS keyring $keyring IAM"
  validate_exact_member_role \
    "$policy" "$member" "$kms_policy_read_role" \
    "release deployer on KMS keyring $keyring"
  gcloud kms keys describe internal-jwt-signing \
    --project="$project" --location="$region" --keyring="$keyring" >/dev/null
  policy=$(gcloud kms keys get-iam-policy internal-jwt-signing \
    --project="$project" --location="$region" --keyring="$keyring" \
    --format=json)
  pymes_assert_policy_has_no_release_pool_members \
    "$policy" "release KMS signing key IAM"
  validate_exact_member_role \
    "$policy" "$member" roles/cloudkms.publicKeyViewer \
    "release deployer on KMS signing key"
  for kms_key in secrets calendar-tokens fiscal-vault; do
    gcloud kms keys describe "$kms_key" \
      --project="$project" --location="$region" --keyring="$keyring" >/dev/null
    policy=$(gcloud kms keys get-iam-policy "$kms_key" \
      --project="$project" --location="$region" --keyring="$keyring" \
      --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy" "release KMS key $kms_key IAM"
  done

  for component in "${release_services[@]}"; do
    name="pymes-v3-${environment}-${component}"
    if ! gcloud run services describe "$name" \
      --project="$project" --region="$region" >/dev/null 2>&1; then
      echo "release workflow is missing finalized Cloud Run service $name" >&2
      exit 1
    fi
    policy=$(gcloud run services get-iam-policy "$name" \
      --project="$project" --region="$region" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy" "release Cloud Run service $name IAM"
    validate_complete_resource_authority \
      "$policy" "$member" service "$component" "$environment" "$stage"
  done
  for component in "${release_jobs[@]}"; do
    name="pymes-v3-${environment}-${component}"
    if ! gcloud run jobs describe "$name" \
      --project="$project" --region="$region" >/dev/null 2>&1; then
      echo "release workflow is missing finalized Cloud Run job $name" >&2
      exit 1
    fi
    policy=$(gcloud run jobs get-iam-policy "$name" \
      --project="$project" --region="$region" --format=json)
    pymes_assert_policy_has_no_release_pool_members \
      "$policy" "release Cloud Run job $name IAM"
    validate_complete_resource_authority \
      "$policy" "$member" job "$component" "$environment" "$stage"
  done

  pymes_verify_release_inverse_authority \
    "$project" "$expected_project_number" "$region" "$environment" present \
    "$artifact_repository"

  build_allowed_pairs=(
    "//cloudresourcemanager.googleapis.com/projects/${project}"$'\t'"roles/serviceusage.serviceUsageConsumer"
    "//cloudresourcemanager.googleapis.com/projects/${expected_project_number}"$'\t'"roles/serviceusage.serviceUsageConsumer"
    "//artifactregistry.googleapis.com/projects/${project}/locations/${region}/repositories/${artifact_repository}"$'\t'"roles/artifactregistry.writer"
    "//artifactregistry.googleapis.com/projects/${expected_project_number}/locations/${region}/repositories/${artifact_repository}"$'\t'"roles/artifactregistry.writer"
  )
  analysis_json=$(read_effective_iam_analysis "$build_email")
  validate_effective_iam_analysis \
    "$analysis_json" "release builder" "${build_allowed_pairs[@]}"

  deploy_allowed_pairs=(
    "//cloudresourcemanager.googleapis.com/projects/${project}"$'\t'"roles/serviceusage.serviceUsageConsumer"
    "//cloudresourcemanager.googleapis.com/projects/${expected_project_number}"$'\t'"roles/serviceusage.serviceUsageConsumer"
    "//cloudresourcemanager.googleapis.com/projects/${project}"$'\t'"${project_iam_read_role}"
    "//cloudresourcemanager.googleapis.com/projects/${expected_project_number}"$'\t'"${project_iam_read_role}"
    "//artifactregistry.googleapis.com/projects/${project}/locations/${region}/repositories/${artifact_repository}"$'\t'"roles/artifactregistry.reader"
    "//artifactregistry.googleapis.com/projects/${expected_project_number}/locations/${region}/repositories/${artifact_repository}"$'\t'"roles/artifactregistry.reader"
    "//cloudkms.googleapis.com/projects/${project}/locations/${region}/keyRings/${keyring}"$'\t'"${kms_policy_read_role}"
    "//cloudkms.googleapis.com/projects/${expected_project_number}/locations/${region}/keyRings/${keyring}"$'\t'"${kms_policy_read_role}"
    "//cloudkms.googleapis.com/projects/${project}/locations/${region}/keyRings/${keyring}/cryptoKeys/internal-jwt-signing"$'\t'"roles/cloudkms.publicKeyViewer"
    "//cloudkms.googleapis.com/projects/${expected_project_number}/locations/${region}/keyRings/${keyring}/cryptoKeys/internal-jwt-signing"$'\t'"roles/cloudkms.publicKeyViewer"
  )
  for secret in "${release_secrets[@]}"; do
    deploy_allowed_pairs+=(
      "//secretmanager.googleapis.com/projects/${project}/secrets/${secret}"$'\t'"roles/secretmanager.viewer"
      "//secretmanager.googleapis.com/projects/${expected_project_number}/secrets/${secret}"$'\t'"roles/secretmanager.viewer"
    )
  done
  for component in "${runtime_accounts[@]}"; do
    runtime_email="pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
    deploy_allowed_pairs+=(
      "//iam.googleapis.com/projects/${project}/serviceAccounts/${runtime_email}"$'\t'"roles/iam.serviceAccountUser"
      "//iam.googleapis.com/projects/${expected_project_number}/serviceAccounts/${runtime_email}"$'\t'"roles/iam.serviceAccountUser"
      "//iam.googleapis.com/projects/-/serviceAccounts/${runtime_email}"$'\t'"roles/iam.serviceAccountUser"
    )
  done
  for component in "${release_services[@]}"; do
    name="pymes-v3-${environment}-${component}"
    deploy_allowed_pairs+=(
      "//run.googleapis.com/projects/${project}/locations/${region}/services/${name}"$'\t'"roles/run.admin"
      "//run.googleapis.com/projects/${expected_project_number}/locations/${region}/services/${name}"$'\t'"roles/run.admin"
    )
  done
  for component in "${release_jobs[@]}"; do
    name="pymes-v3-${environment}-${component}"
    deploy_allowed_pairs+=(
      "//run.googleapis.com/projects/${project}/locations/${region}/jobs/${name}"$'\t'"roles/run.admin"
      "//run.googleapis.com/projects/${expected_project_number}/locations/${region}/jobs/${name}"$'\t'"roles/run.admin"
    )
  done
  analysis_json=$(read_effective_iam_analysis "$deploy_email")
  validate_effective_iam_analysis \
    "$analysis_json" "release deployer" "${deploy_allowed_pairs[@]}"

  for component in "${runtime_accounts[@]}"; do
    runtime_email="pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
    mapfile -t runtime_pairs < <(
      runtime_effective_iam_allowlist "$component" "$environment"
    )
    analysis_json=$(read_effective_iam_analysis "$runtime_email")
    validate_effective_iam_analysis \
      "$analysis_json" "runtime identity $runtime_email" "${runtime_pairs[@]}"
  done

  echo "Release authority verified: environment=${environment} stage=${stage} builder=keyless-exact wif=exact effective_iam=allowlisted inverse_permissions=allowlisted runtime_iam=allowlisted project_run_admin=absent resource_run_admin=exact"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
