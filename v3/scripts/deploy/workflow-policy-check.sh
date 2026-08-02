#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
ci_workflow="$repo_root/.github/workflows/v3-ci.yml"
release_workflow="$repo_root/.github/workflows/v3-release.yml"
dependency_pin="$repo_root/.github/dependencies/open-accounting.env"
pymes_dockerfile="$repo_root/v3/Dockerfile"
build_script="$repo_root/v3/scripts/deploy/build-push-images.sh"
deploy_script="$repo_root/v3/scripts/deploy/cloud-run.sh"
verify_script="$repo_root/v3/scripts/deploy/verify-cloud-run.sh"
identity_script="$repo_root/v3/scripts/deploy/bootstrap-release-identities.sh"
seed_script="$repo_root/v3/scripts/deploy/seed-cloud-run-resources.sh"
seed_test="$repo_root/v3/scripts/deploy/seed-cloud-run-resources-test.sh"
seed_audit_policy="$repo_root/v3/scripts/deploy/initial-seed-audit.jq"
seed_audit_bounds="$repo_root/v3/scripts/deploy/initial-seed-audit-bounds.sh"
release_authority_policy="$repo_root/v3/scripts/deploy/release-authority-policy.sh"
authority_verifier="$repo_root/v3/scripts/deploy/verify-release-authority.sh"
authority_test="$repo_root/v3/scripts/deploy/verify-release-authority-test.sh"
legacy_wif_retirement="$repo_root/v3/scripts/deploy/retire-legacy-pymes-wif.sh"
legacy_wif_test="$repo_root/v3/scripts/deploy/retire-legacy-pymes-wif-test.sh"
github_environment_bootstrap="$repo_root/v3/scripts/deploy/bootstrap-github-environments.sh"
github_environment_verifier="$repo_root/v3/scripts/deploy/verify-github-environments.sh"
structured_policy="$repo_root/v3/scripts/deploy/workflowpolicy/main.go"
structured_policy_test="$repo_root/v3/scripts/deploy/workflowpolicy/main_test.go"

for file in \
  "$ci_workflow" \
  "$release_workflow" \
  "$dependency_pin" \
  "$pymes_dockerfile" \
  "$build_script" \
  "$deploy_script" \
  "$verify_script" \
  "$identity_script" \
  "$seed_script" \
  "$seed_test" \
  "$seed_audit_policy" \
  "$seed_audit_bounds" \
  "$release_authority_policy" \
  "$authority_verifier" \
  "$authority_test" \
  "$legacy_wif_retirement" \
  "$legacy_wif_test" \
  "$github_environment_bootstrap" \
  "$github_environment_verifier" \
  "$structured_policy" \
  "$structured_policy_test"; do
  [[ -f "$file" ]] || {
    echo "missing release-policy file: $file" >&2
    exit 1
  }
done
bash -n "$identity_script" "$deploy_script" "$seed_script" "$seed_test" "$seed_audit_bounds" \
  "$release_authority_policy" \
  "$authority_verifier" "$authority_test"

fail() {
  echo "release workflow policy violation: $*" >&2
  exit 1
}

check_action_pins() {
  local workflow="$1" line action owner
  while IFS= read -r line; do
    action=$(sed -E 's/^[[:space:]]*(-[[:space:]]+)?uses:[[:space:]]*([^[:space:]#]+).*$/\2/' <<<"$line")
    [[ "$action" =~ ^([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)@([0-9a-f]{40})$ ]] ||
      fail "$(basename "$workflow") contains a mutable or invalid action reference: $action"
    owner=${BASH_REMATCH[1]}
    case "$owner" in
      actions|docker|google-github-actions) ;;
      *) fail "$(basename "$workflow") uses non-allowlisted action owner: $owner" ;;
    esac
  done < <(grep -E '^[[:space:]]*(-[[:space:]]+)?uses:' "$workflow")
}

release_is_manual_only() {
  local workflow="$1"
  grep -Fq 'workflow_dispatch:' "$workflow" &&
    ! grep -Eq '^[[:space:]]+(push|pull_request|pull_request_target|schedule):' "$workflow"
}

check_docker_base_pins() {
  local dockerfile="$1" line keyword reference alias_keyword alias
  declare -A stages=()
  while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*[Ff][Rr][Oo][Mm][[:space:]]+ ]] || continue
    read -r keyword reference alias_keyword alias <<<"$line"
    [[ "$reference" != --platform=* ]] ||
      fail "$(basename "$dockerfile") hides platform selection in FROM"
    if [[ -z "${stages[$reference]:-}" &&
          ! "$reference" =~ @sha256:[0-9a-f]{64}$ ]]; then
      fail "$(basename "$dockerfile") contains mutable external base: $reference"
    fi
    if [[ "${alias_keyword,,}" == "as" ]]; then
      [[ "$alias" =~ ^[A-Za-z0-9_.-]+$ ]] ||
        fail "$(basename "$dockerfile") contains an invalid stage alias"
      stages["$alias"]=1
    fi
  done <"$dockerfile"
}

check_action_pins "$ci_workflow"
check_action_pins "$release_workflow"
check_docker_base_pins "$pymes_dockerfile"

release_is_manual_only "$release_workflow" ||
  fail "release must be manually dispatched and have no automatic trigger"
grep -Fq '[[ "${GITHUB_REF}" == "refs/heads/main" ]]' "$release_workflow" ||
  fail "release does not fail closed outside main"
grep -Fq 'Pymes V3 CI has no successful push run for exact SHA' "$release_workflow" ||
  fail "release does not gate on exact-SHA V3 CI"
grep -Fq '.head_sha == $sha' "$release_workflow" ||
  fail "release CI gate does not compare the exact SHA"
grep -Fq '[[ "${PRODUCTION_CONFIRMATION}" == "DEPLOY_PRD" ]]' "$release_workflow" ||
  fail "production does not require explicit typed confirmation"
grep -Fq '[[ "${REQUESTED_ENVIRONMENT}" == "stg" ]]' "$release_workflow" ||
  fail "bootstrap release stage is not restricted to STG"
grep -Fq 'PYMES_DEPLOY_STAGE: ${{ inputs.deploy_stage }}' "$release_workflow" ||
  fail "release does not pass the reviewed deploy stage to Cloud Run"
grep -Fq 'PYMES_CLOUD_RUN_VERIFY_PHASE=pretraffic' "$deploy_script" ||
  fail "Cloud Run transaction does not verify the candidate before traffic"
grep -Fq 'PYMES_CLOUD_RUN_VERIFY_PHASE=active' "$deploy_script" ||
  fail "Cloud Run transaction does not verify the active release before commit"
if grep -Fq 'name: Verify deployed release' "$release_workflow"; then
  fail "release verification must remain inside cloud-run.sh rollback transaction"
fi
if grep -Fq 'paths:' "$ci_workflow"; then
  fail "required V3 CI must run for every pull request and main push"
fi
grep -Fq 'name: Pymes V3 validate' "$ci_workflow" ||
  fail "V3 CI required-check name is not explicit and stable"
grep -Fq '.github/scripts/check-legacy-frozen.sh "${BASE_SHA}" "${HEAD_SHA}"' "$ci_workflow" ||
  fail "required V3 CI does not enforce immutable v1/v2"
grep -Fq 'fetch-depth: 0' "$ci_workflow" ||
  fail "required V3 CI cannot establish the complete legacy diff baseline"
grep -Fq 'verify-github-environments.sh all all-controls' "$release_workflow" ||
  fail "release does not audit both protected GitHub environments before authentication"
grep -Fq 'secrets.PYMES_GITHUB_RELEASE_AUDIT_TOKEN' "$release_workflow" ||
  fail "release does not use the protected full-control audit credential"
grep -Fq 'run-name: Pymes V3 ${{ inputs.environment }} ${{ inputs.deploy_stage }} @ ${{ github.sha }}' "$release_workflow" ||
  fail "release run name does not bind environment, deployment stage, and exact source SHA"
for proof in \
  '.protection.required_status_checks.enforcement_level == "everyone"' \
  '.required_status_checks.checks[0].context == "Pymes V3 validate"' \
  '.required_pull_request_reviews.require_last_push_approval == true' \
  '.enforce_admins.enabled == true' \
  '.deployment_branch_policy.custom_branch_policies == true' \
  '.branch_policies[0].name == "main"' \
  '.can_admins_bypass == false' \
  '.prevent_self_review == true'; do
  grep -Fq "$proof" "$github_environment_verifier" ||
    fail "GitHub environment verifier lacks proof: $proof"
done
grep -Fq 'PYMES_PRD_REVIEWER_IDS' "$github_environment_bootstrap" ||
  fail "GitHub environment bootstrap does not require explicit PRD reviewers"
grep -Fq 'contexts: []' "$github_environment_bootstrap" ||
  fail "GitHub branch-protection payload omits the required contexts field"
if grep -Fq 'dismissal_restrictions' "$github_environment_bootstrap"; then
  fail "GitHub branch-protection payload must omit dismissal_restrictions for a user-owned repository"
fi
grep -Fq '"repos/${repository}/branches/main/protection"' "$github_environment_bootstrap" ||
  fail "GitHub release-control bootstrap does not configure main protection"
grep -Fq "refusing to delete or overwrite unexpected" "$github_environment_bootstrap" ||
  fail "GitHub environment bootstrap may overwrite unexpected branch policies"
grep -Fq 'found unknown or conflicting protection rules' "$github_environment_bootstrap" ||
  fail "GitHub environment bootstrap may overwrite unknown protection rules"
for proof in \
  'repository_id=1173650578' \
  'repository_owner_id=81805584' \
  '.permissions.admin == true' \
  '.status == "404"' \
  'could not prove absence or ownership' \
  'preflight_main_only_policy "$environment"' \
  'GitHub mutation preflight verified:'; do
  grep -Fq "$proof" "$github_environment_bootstrap" ||
    fail "GitHub environment bootstrap lacks mutation preflight proof: $proof"
done
github_preflight_line=$(grep -nF 'GitHub mutation preflight verified:' "$github_environment_bootstrap" | head -1 | cut -d: -f1)
github_first_mutation_line=$(grep -nE 'gh api --method (PUT|POST|PATCH|DELETE)' "$github_environment_bootstrap" | head -1 | cut -d: -f1)
[[ "$github_preflight_line" =~ ^[0-9]+$ &&
   "$github_first_mutation_line" =~ ^[0-9]+$ &&
   "$github_preflight_line" -lt "$github_first_mutation_line" ]] ||
  fail "GitHub identity and environment checks must complete before the first mutation"
grep -Fq 'reviewers differ from PYMES_PRD_REVIEWER_IDS' "$github_environment_verifier" ||
  fail "GitHub environment verifier does not enforce the exact approved PRD reviewer set"
grep -Fq '.restrictions == null' "$github_environment_verifier" ||
  fail "GitHub branch-protection verifier permits actor restrictions"
grep -Fq '.allow_fork_syncing.enabled == false' "$github_environment_verifier" ||
  fail "GitHub branch-protection verifier permits fork-sync bypass"
grep -Fq '.required_pull_request_reviews.bypass_pull_request_allowances' "$github_environment_verifier" ||
  fail "GitHub branch-protection verifier does not reject review bypass allowances"
grep -Fq '"$script_dir/verify-github-environments.sh" "$target_environment" all-controls' "$identity_script" ||
  fail "WIF bootstrap does not verify the selected GitHub environment before mutating GCP"

grep -Fq 'path: .deps/open-accounting' "$ci_workflow" ||
  fail "CI Open Accounting checkout is not isolated under .deps"
grep -Fq 'path: .deps/open-accounting' "$release_workflow" ||
  fail "release Open Accounting checkout is not isolated under .deps"
grep -Fq 'git status --porcelain=v1 --untracked-files=all' "$ci_workflow" ||
  fail "CI does not prove a clean Open Accounting checkout"
grep -Fq '[[ -z "$(git status --porcelain=v1 --untracked-files=all)" ]]' "$release_workflow" ||
  fail "release does not prove a clean Pymes checkout"
grep -Fq 'git -C "$accounting_context" status --porcelain=v1 --untracked-files=all' "$build_script" ||
  fail "image builder does not prove a clean Open Accounting checkout"
[[ "$(sed -n 's/^OPEN_ACCOUNTING_REPOSITORY=//p' "$dependency_pin")" == "devpablocristo/open-accounting" ]] ||
  fail "Open Accounting dependency is not fixed to the reviewed fork"
[[ "$(sed -n 's/^OPEN_ACCOUNTING_REPOSITORY_ID=//p' "$dependency_pin")" == "1317775856" ]] ||
  fail "Open Accounting dependency is not fixed to repository_id 1317775856"
[[ "$(grep -c '^OPEN_ACCOUNTING_REPOSITORY=' "$dependency_pin")" -eq 1 &&
   "$(grep -c '^OPEN_ACCOUNTING_REPOSITORY_ID=' "$dependency_pin")" -eq 1 &&
   "$(grep -c '^OPEN_ACCOUNTING_REF=' "$dependency_pin")" -eq 1 ]] ||
  fail "Open Accounting dependency pin has duplicate or missing identity fields"
for workflow in "$ci_workflow" "$release_workflow"; do
  grep -Fq '[[ "${repository}" == "devpablocristo/open-accounting" ]]' "$workflow" ||
    fail "$(basename "$workflow") does not reject a different Open Accounting owner/repository"
  grep -Fq '[[ "${repository_id}" == "1317775856" ]]' "$workflow" ||
    fail "$(basename "$workflow") does not reject a different Open Accounting repository_id"
  grep -Fq '.full_name == $repository and .id == $repository_id' "$workflow" ||
    fail "$(basename "$workflow") does not verify Open Accounting identity against GitHub"
done
grep -Fq 'verify_dockerfile_base_pins "$accounting_dockerfile"' "$build_script" ||
  fail "image builder does not fail closed on mutable Open Accounting bases"
grep -Fq './v3/scripts/deploy/build-push-images.sh verify-source-pins' "$ci_workflow" ||
  fail "required V3 CI does not validate every release Docker base pin"

grep -Fq 'workloadIdentityPools/pymes-v3-release-pool/providers/github' "$release_workflow" ||
  fail "release does not reject the old shared WIF pool"
[[ "$(grep -Fc 'workload_identity_provider: projects/884236221349/locations/global/workloadIdentityPools/pymes-v3-release-pool/providers/github' "$release_workflow")" -eq 3 ]] ||
  fail "pre-build audit, build and deploy must require the exact numeric release provider"
grep -Fq '[[ "${DEPLOY_SERVICE_ACCOUNT}" != "${BUILD_SERVICE_ACCOUNT}" ]]' "$release_workflow" ||
  fail "release does not prove build/deploy identity separation"
[[ "$(grep -Fc 'service_account: pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com' "$release_workflow")" -eq 1 ]] ||
  fail "build must authenticate exactly once as the source-controlled Pymes builder"
[[ "$(grep -Fc 'service_account: pymes-v3-gh-deploy-${{ inputs.environment }}@pymes-dev-352318.iam.gserviceaccount.com' "$release_workflow")" -eq 2 ]] ||
  fail "authority audit and deploy must authenticate exactly as the source-controlled environment deployer"
if grep -Eq 'vars\\.PYMES_(GCP_PROJECT|GCP_REGION|ARTIFACT_REPOSITORY|BUILD_|DEPLOY_)' "$release_workflow"; then
  fail "fixed GCP release targets and identities must not come from mutable GitHub variables"
fi
for target in \
  'PYMES_GCP_PROJECT: pymes-dev-352318' \
  'PYMES_GCP_PROJECT_NUMBER: "884236221349"' \
  'PYMES_GCP_REGION: us-central1' \
  'PYMES_ARTIFACT_REPOSITORY: pymes'; do
  grep -Fq "$target" "$release_workflow" ||
    fail "release omits fixed target: $target"
done
[[ "$(grep -Fc 'name: Reverify protected GitHub controls before deploy identity' "$release_workflow")" -eq 1 ]] ||
  fail "release must repeat the full GitHub audit exactly once before deploy authentication"
pre_deploy_audit_line=$(grep -nF 'name: Reverify protected GitHub controls before deploy identity' "$release_workflow" | cut -d: -f1)
deploy_auth_line=$(grep -nF 'name: Authenticate least-privilege deployer' "$release_workflow" | cut -d: -f1)
[[ "$pre_deploy_audit_line" =~ ^[0-9]+$ &&
   "$deploy_auth_line" =~ ^[0-9]+$ &&
   "$pre_deploy_audit_line" -lt "$deploy_auth_line" ]] ||
  fail "the full GitHub audit must execute immediately before deploy authentication"
[[ "$(grep -Fc 'name: Reverify protected GitHub controls before authority audit identity' "$release_workflow")" -eq 1 ]] ||
  fail "release must repeat the full GitHub audit exactly once before pre-build authority authentication"
pre_build_audit_line=$(grep -nF 'name: Reverify protected GitHub controls before authority audit identity' "$release_workflow" | cut -d: -f1)
pre_build_auth_line=$(grep -nF 'name: Authenticate pre-build authority auditor' "$release_workflow" | cut -d: -f1)
pre_build_authority_line=$(grep -nF 'name: Verify complete release authority before builder' "$release_workflow" | cut -d: -f1)
build_auth_line=$(grep -nF 'name: Authenticate immutable image builder' "$release_workflow" | cut -d: -f1)
build_rerun_guard_line=$(grep -nF 'name: Reject standalone build reruns' "$release_workflow" | cut -d: -f1)
deploy_rerun_guard_line=$(grep -nF 'name: Reject standalone deploy reruns' "$release_workflow" | cut -d: -f1)
deploy_checkout_line=$(grep -nF 'name: Checkout exact Pymes source' "$release_workflow" | tail -1 | cut -d: -f1)
[[ "$pre_build_audit_line" =~ ^[0-9]+$ &&
   "$pre_build_auth_line" =~ ^[0-9]+$ &&
   "$pre_build_authority_line" =~ ^[0-9]+$ &&
   "$build_rerun_guard_line" =~ ^[0-9]+$ &&
   "$build_auth_line" =~ ^[0-9]+$ &&
   "$pre_build_audit_line" -lt "$pre_build_auth_line" &&
   "$pre_build_auth_line" -lt "$pre_build_authority_line" &&
   "$pre_build_authority_line" -lt "$build_auth_line" &&
   "$build_rerun_guard_line" -lt "$build_auth_line" ]] ||
  fail "complete authority must be audited through protected WIF before builder authentication"
[[ "$(grep -Fc '[[ "${GITHUB_RUN_ATTEMPT}" == "1" ]]' "$release_workflow")" -eq 2 ]] ||
  fail "build and deploy must each reject every rerun attempt"
[[ "$(grep -Fc 'dispatch a new release workflow' "$release_workflow")" -eq 2 ]] ||
  fail "build and deploy rerun guards must require a fresh release dispatch"
[[ "$(grep -Fc 'name: Reject standalone deploy reruns' "$release_workflow")" -eq 1 &&
   "$deploy_rerun_guard_line" =~ ^[0-9]+$ &&
   "$deploy_checkout_line" =~ ^[0-9]+$ &&
   "$deploy_auth_line" =~ ^[0-9]+$ &&
   "$deploy_rerun_guard_line" -lt "$deploy_checkout_line" &&
   "$deploy_rerun_guard_line" -lt "$deploy_auth_line" ]] ||
  fail "deploy rerun guard must execute once before any checkout or authentication"

if grep -Eq '(^|[[:space:]])(source|\\.)[[:space:]]+.*pymes-v3-images' "$release_workflow"; then
  fail "release manifest must never be sourced as shell code"
fi
grep -Fq 'release manifest contains non-allowlisted key' "$release_workflow" ||
  fail "release manifest import is not allowlisted"
grep -Fq '>>"${GITHUB_ENV}"' "$release_workflow" ||
  fail "validated manifest values are not passed through GITHUB_ENV"
grep -Fq 'PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN:' "$release_workflow" ||
  fail "release omits the homologation ARCA issuer policy"
grep -Fq 'PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN:' "$release_workflow" ||
  fail "release omits the production ARCA issuer policy"
if grep -Eq 'PYMES_FISCAL_ARCA_(WSAA_URL|WSFE_URL|ALLOWED_ISSUERS)' "$release_workflow"; then
  fail "release contains obsolete Fiscal ARCA configuration"
fi
if grep -Fq 'VITE_API_BASE_URL' "$release_workflow" ||
  grep -Fq 'VITE_API_BASE_URL' "$build_script"; then
  fail "Web release must use the runtime same-origin API proxy"
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
for key in "${manifest_keys[@]}"; do
  grep -Fq "$key" "$build_script" ||
    fail "image manifest builder omits $key"
  grep -Fq "[$key]=" "$release_workflow" ||
    fail "release manifest allowlist omits $key"
done
if grep -Eq '(^|[/:_-])latest([[:space:]\"'\''@]|$)' "$build_script"; then
  fail "image builder must not publish or consume latest tags"
fi
grep -Fq '@${digest}' "$build_script" ||
  fail "image builder does not emit digest-only references"
grep -Fq -- '--provenance=mode=max' "$build_script" ||
  fail "image builder does not publish maximum provenance"
grep -Fq -- '--sbom=true' "$build_script" ||
  fail "image builder does not publish an SBOM"
for proof in \
  'verify_image_attestations' \
  '{{json .Provenance}}' \
  '{{json .SBOM}}' \
  '.manifest.digest == $digest' \
  'https://mobyproject.org/buildkit@v1' \
  'pinned_digest_for_base' \
  'contains("digest=" + $material_digest)' \
  'provenance omits pinned base material' \
  'SPDXRef-DOCUMENT' \
  'io.pymes.release.pymes-revision' \
  'io.pymes.release.open-accounting-revision' \
  'io.pymes.release.pymes-repository-id' \
  'io.pymes.release.open-accounting-repository-id'; do
  grep -Fq "$proof" "$build_script" ||
    fail "image attestation verifier lacks proof: $proof"
done
grep -Fq './v3/scripts/deploy/build-push-images.sh verify-attestations' "$release_workflow" ||
  fail "deploy does not re-verify provenance, materials, and SBOMs"
attestation_line=$(grep -nF './v3/scripts/deploy/build-push-images.sh verify-attestations' "$release_workflow" | cut -d: -f1)
deploy_line=$(grep -nF 'run: ./scripts/deploy/cloud-run.sh' "$release_workflow" | cut -d: -f1)
[[ "$attestation_line" =~ ^[0-9]+$ && "$deploy_line" =~ ^[0-9]+$ &&
   "$attestation_line" -lt "$deploy_line" ]] ||
  fail "attestation verification must execute before Cloud Run deployment"

if grep -Eiq 'axis' "$legacy_wif_retirement"; then
  fail "legacy Pymes WIF retirement must not reference Axis"
fi
grep -Fq 'PYMES_LEGACY_WIF_MODE=audit' "$identity_script" ||
  fail "release close phase does not audit retired legacy Pymes WIF trust"
grep -Fq '"$script_dir/retire-legacy-pymes-wif.sh"' "$identity_script" ||
  fail "release close phase does not invoke the dedicated legacy WIF retirement audit"
retirement_gate_line=$(awk '
  $0 ~ /^if \[\[ "\$phase" == "close" \]\]; then$/ { in_close=1; next }
  in_close && /PYMES_LEGACY_WIF_MODE=audit/ { saw_audit=1 }
  in_close && /retire-legacy-pymes-wif\.sh/ && saw_audit { print NR; exit }
  in_close && /^fi$/ { in_close=0; saw_audit=0 }
' "$identity_script")
retirement_exit_line=$(awk '
  $0 ~ /^if \[\[ "\$phase" == "close" \]\]; then$/ { in_close=1; next }
  in_close && /exit 0/ { print NR; exit }
  in_close && /^fi$/ { in_close=0 }
' "$identity_script")
services_enable_line=$(grep -nF 'gcloud services enable ' "$identity_script" | head -1 | cut -d: -f1)
[[ "$retirement_gate_line" =~ ^[0-9]+$ &&
   "$retirement_exit_line" =~ ^[0-9]+$ &&
   "$services_enable_line" =~ ^[0-9]+$ &&
   "$retirement_gate_line" -lt "$retirement_exit_line" &&
   "$retirement_exit_line" -lt "$services_enable_line" ]] ||
  fail "close must audit legacy Pymes WIF retirement before any GCP mutation"
bash "$legacy_wif_test" >/dev/null ||
  fail "legacy Pymes WIF retirement behavioral safety tests failed"

grep -Fq 'pool=pymes-v3-release-pool' "$identity_script" ||
  fail "release identities do not use a dedicated pool"
for claim in \
  "github_repository_id=1173650578" \
  "github_repository_owner_id=81805584" \
  "attribute.repository_id=assertion.repository_id" \
  "attribute.repository_owner_id=assertion.repository_owner_id" \
  "attribute.ref=assertion.ref" \
  "attribute.ref_protected=assertion.ref_protected" \
  "attribute.workflow_ref=assertion.workflow_ref" \
  "attribute.event_name=assertion.event_name" \
  "github_workflow_ref='devpablocristo/pymes/.github/workflows/v3-release.yml@refs/heads/main'" \
  "github_build_subject='repo:devpablocristo/pymes:ref:refs/heads/main'" \
  "github_stg_subject='repo:devpablocristo/pymes:environment:stg'" \
  "github_prd_subject='repo:devpablocristo/pymes:environment:prd'" \
  "assertion.repository_id=='\${github_repository_id}'" \
  "assertion.repository_owner_id=='\${github_repository_owner_id}'" \
  "assertion.ref_protected=='true'" \
  "assertion.workflow_ref=='\${github_workflow_ref}'" \
  "assertion.event_name=='workflow_dispatch'" \
  "assertion.sub=='\${github_build_subject}'" \
  "assertion.sub=='\${github_stg_subject}'" \
  "assertion.sub=='\${github_prd_subject}'"; do
  grep -Fq "$claim" "$identity_script" ||
    fail "dedicated WIF trust omits exact claim: $claim"
done
grep -Fq "assertion.repository=='" "$identity_script" ||
  fail "WIF provider is not constrained to one repository"
grep -Fq "assertion.ref=='refs/heads/main'" "$identity_script" ||
  fail "WIF provider is not constrained to main"
if grep -Fq 'attribute.environment' "$identity_script"; then
  fail "WIF trust must use exact OIDC subjects, not a broad environment attribute"
fi
[[ "$(grep -Fo "assertion.sub=='" "$identity_script" | wc -l | tr -d '[:space:]')" -eq 3 ]] ||
  fail "WIF provider must allow exactly the build, STG and PRD OIDC subjects"
grep -Fq 'google.subject=assertion.sub' "$identity_script" ||
  fail "WIF provider does not map the exact GitHub OIDC subject"
grep -Fq '(.attributeMapping | length) == 8' "$identity_script" ||
  fail "WIF provider does not reject additional claim mappings"
grep -Fq 'principal://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${pool}/subject/${github_build_subject}' "$identity_script" ||
  fail "build identity is not bound to the exact main-ref subject"
grep -Fq 'principal://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${pool}/subject/${deploy_subject}' "$identity_script" ||
  fail "deploy identities are not bound to exact protected-environment subjects"
grep -Fq 'PYMES_RELEASE_IDENTITY_ENV' "$identity_script" ||
  fail "release identity finalization is not explicitly environment-scoped"
grep -Fq '"$phase requires PYMES_RELEASE_IDENTITY_ENV=stg or prd"' "$identity_script" ||
  fail "release identity preparation/finalization does not require one exact environment"
if grep -Fq 'prepare manages both environments' "$identity_script" ||
  grep -Fq 'managed_environments=(stg prd)' "$identity_script"; then
  fail "release identity bootstrap still couples STG and PRD"
fi
grep -Fq 'The PRD deployer must not exist before close.' "$identity_script" ||
  fail "release identity plan does not preserve the STG-first cutover sequence"
grep -Fq 'PRD is permitted only after the audited legacy-WIF close.' "$identity_script" ||
  fail "release identity plan permits preparing PRD before the audited cutover"
prd_close_gate_line=$(awk '
  /^if \[\[ "\$phase" == "prepare" && "\$target_environment" == "prd" \]\]; then$/ {
    in_prd_prepare=1
    next
  }
  in_prd_prepare && /PYMES_LEGACY_WIF_MODE=audit/ { saw_audit=1 }
  in_prd_prepare && /retire-legacy-pymes-wif\.sh/ && saw_audit { print NR; exit }
  in_prd_prepare && /^fi$/ { in_prd_prepare=0; saw_audit=0 }
' "$identity_script")
[[ "$prd_close_gate_line" =~ ^[0-9]+$ ]] ||
  fail "PRD prepare does not audit the completed STG legacy-WIF cutover"
grep -Fq 'GCP mutation preflight verified:' "$identity_script" ||
  fail "release identity bootstrap does not prove the immutable GCP target before mutation"
for proof in \
  'expected_project=pymes-dev-352318' \
  'expected_project_number=884236221349' \
  'expected_region=us-central1' \
  'expected_artifact_repository=pymes' \
  '.projectId == $project' \
  '(.projectNumber | tostring) == $number' \
  '.lifecycleState == "ACTIVE"' \
  '.format == "DOCKER"' \
  '(.mode // "STANDARD_REPOSITORY") == "STANDARD_REPOSITORY"'; do
  grep -Fq "$proof" "$identity_script" ||
    fail "GCP mutation preflight lacks immutable target proof: $proof"
done
gcp_preflight_line=$(grep -nF 'GCP mutation preflight verified:' "$identity_script" | head -1 | cut -d: -f1)
reviewed_source_line=$(grep -nF 'verify_reviewed_release_source' "$identity_script" | tail -1 | cut -d: -f1)
services_enable_line=$(grep -nF 'gcloud services enable ' "$identity_script" | head -1 | cut -d: -f1)
[[ "$gcp_preflight_line" =~ ^[0-9]+$ &&
   "$reviewed_source_line" =~ ^[0-9]+$ &&
   "$services_enable_line" =~ ^[0-9]+$ &&
   "$gcp_preflight_line" -lt "$reviewed_source_line" &&
   "$reviewed_source_line" -lt "$services_enable_line" ]] ||
  fail "GCP identity and reviewed-source checks must complete before the first release mutation"
[[ "$prd_close_gate_line" -lt "$services_enable_line" ]] ||
  fail "PRD close audit must complete before the first release mutation"
for proof in \
  'PYMES_RELEASE_IDENTITY_OPERATOR_EMAIL' \
  'assert_direct_gcloud_auth' \
  'CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT' \
  'auth/impersonate_service_account' \
  'IAM mutation requires a clean Pymes main checkout' \
  'local Pymes HEAD must equal the exact current GitHub main SHA' \
  'local release helpers must equal the exact reviewed GitHub main tree' \
  'IAM mutation is restricted to the reviewed direct operator softponti@gmail.com' \
  'pymes_validate_release_pool_iam_assets' \
  'pymes_verify_release_account_not_attached' \
  'iam.disableCrossProjectServiceAccountUsage' \
  'iam.disableServiceAccountKeyCreation' \
  'pymesV3ReleaseOrganizationIamRead' \
  'pymesV3ReleaseFolderIamRead' \
  'assert_release_account_preflight' \
  '.disabled != true' \
  '--managed-by=user' \
  'policy must be empty or exactly the reviewed WIF binding' \
  'assert_roles_subset' \
  'assert_effective_iam_subset' \
  'pymes_verify_release_inverse_authority' \
  'gcloud asset analyze-iam-policy' \
  '--expand-groups' \
  '--expand-roles' \
  '--output-group-edges' \
  '.fullyExplored == true' \
  'preflight_deploy_resources' \
  'assert_exact_or_empty_member_role' \
  'missing release secret container before IAM grants' \
  'runtime identity is missing, disabled, or unexpected before IAM grants' \
  'missing release KMS keyring before IAM grants' \
  'missing initial Cloud Run service before IAM grants' \
  'missing initial Cloud Run job before IAM grants' \
  'Release service-account preflight verified:'; do
  grep -Fq -- "$proof" "$identity_script" ||
    fail "release account preflight lacks proof: $proof"
done
[[ "$(grep -Fc '"$release_pool_assets" "$target_environment" exact' "$identity_script")" -eq 2 ]] ||
  fail "release-pool trust must be exact both before and after IAM mutation"
if grep -Fq '"$release_pool_assets" "$target_environment" subset' "$identity_script"; then
  fail "release-pool postcondition must not accept missing or premature environment trust"
fi
for proof in \
  'pymes_read_inverse_permission_analysis' \
  'pymes_validate_inverse_permission_analysis' \
  '--permissions="$chunk_csv"' \
  '--expand-resources' \
  'requestedPermissions' \
  'identityList.identities' \
  'resourcemanager.projects.setIamPolicy' \
  'iam.serviceAccounts.getAccessToken' \
  'run.services.update' \
  'secretmanager.versions.access' \
  'artifactregistry.repositories.uploadArtifacts' \
  'cloudkms.cryptoKeyVersions.useToSign'; do
  grep -Fq -- "$proof" "$release_authority_policy" ||
    fail "inverse effective-permission boundary lacks proof: $proof"
done
release_account_preflight_line=$(grep -nF 'Release service-account preflight verified:' "$identity_script" | head -1 | cut -d: -f1)
inverse_preflight_line=$(grep -nF 'pymes_verify_release_inverse_authority' "$identity_script" | tail -1 | cut -d: -f1)
first_release_grant_line=$(grep -nF 'gcloud iam service-accounts add-iam-policy-binding "$build_email"' "$identity_script" | head -1 | cut -d: -f1)
[[ "$release_account_preflight_line" =~ ^[0-9]+$ &&
   "$inverse_preflight_line" =~ ^[0-9]+$ &&
   "$first_release_grant_line" =~ ^[0-9]+$ &&
   "$inverse_preflight_line" -lt "$release_account_preflight_line" &&
   "$release_account_preflight_line" -lt "$first_release_grant_line" ]] ||
  fail "inverse authority and release service accounts must be validated before the first privilege grant"
grep -Fq 'roles/artifactregistry.writer' "$identity_script" ||
  fail "build identity lacks repository-scoped write"
grep -Fq 'roles/artifactregistry.reader' "$identity_script" ||
  fail "deploy identity lacks repository-scoped read"
grep -Fq 'pymes-v3-gh-build' "$identity_script" ||
  fail "release builder identity name differs from the reviewed contract"
grep -Fq 'pymes-v3-gh-deploy-' "$identity_script" ||
  fail "environment deployer identity names differ from the reviewed contract"
grep -Fq 'grant_secret_metadata_reader' "$identity_script" ||
  fail "deploy identity does not read Secret Manager metadata at secret scope"
grep -Fq 'kms keyrings add-iam-policy-binding' "$identity_script" ||
  fail "deploy identity does not read KMS policy at environment key-ring scope"
grep -Fq 'pymesV3ReleaseKmsPolicyRead' "$identity_script" ||
  fail "deploy identity lacks the minimal KMS IAM-policy reader"
if grep -Eq 'grant_project_role[[:space:]]+"?\\$?build[^\\n]*roles/(run\\.admin|artifactregistry\\.writer|owner|editor)' "$identity_script"; then
  fail "build identity receives a broad deployment/project role"
fi
if grep -Eq 'grant_project_role[[:space:]]+"?\\$?deploy[^\\n]*roles/secretmanager\\.viewer' "$identity_script"; then
  fail "deploy identity reads Secret Manager metadata at project scope"
fi
if grep -Eq 'grant_project_role[^[:cntrl:]]*roles/(owner|editor|iam\\.serviceAccountAdmin|secretmanager\\.admin|run\\.admin)' "$identity_script"; then
  fail "release bootstrap includes a prohibited broad role"
fi
grep -Fq 'gcloud run services add-iam-policy-binding' "$identity_script" ||
  fail "Cloud Run admin is not scoped to initialized services"
grep -Fq 'gcloud run jobs add-iam-policy-binding' "$identity_script" ||
  fail "Cloud Run admin is not scoped to initialized jobs"
if grep -Eq 'roles/(run\\.viewer|cloudsql\\.viewer|compute\\.networkViewer)' "$identity_script"; then
  fail "steady-state deployer retains a broad project metadata viewer role"
fi
for permission in \
  artifactregistry.repositories.getIamPolicy \
  cloudasset.assets.analyzeIamPolicy \
  cloudsql.instances.get \
  compute.routers.get \
  compute.subnetworks.get \
  iam.serviceAccountKeys.list \
  iam.serviceAccounts.getIamPolicy \
  iam.workloadIdentityPoolProviders.get \
  iam.workloadIdentityPools.get; do
  grep -Fq "$permission" "$release_authority_policy" ||
    fail "minimal release preflight role omits $permission"
done
grep -Fq 'without project-wide roles/run.admin' "$identity_script" ||
  fail "initial bootstrap does not explicitly avoid permanent project Run Admin"
for proof in \
  'PYMES_INITIAL_SEED_OPERATOR_EMAIL' \
  'PYMES_INITIAL_SEED_COMPLETED_AT' \
  'PYMES_INITIAL_SEED_MANIFEST_SHA256' \
  'gcloud logging read' \
  'audit_end_grace_seconds=120' \
  'audit_min_settle_seconds=600' \
  'audit_stability_wait_seconds=20' \
  'unrelated_mutations=0' \
  'authority=pre-existing-owner' \
  'allowed_resources_json' \
  'allowed_service_accounts_json' \
  'serviceAccountDelegationInfo.firstPartyPrincipal.principalEmail' \
  'runtime_act_as=exact' \
  'verify_initial_cloud_run_resources'; do
  grep -Fq "$proof" "$identity_script" ||
    fail "initial seed finalization lacks audited evidence: $proof"
done
for proof in \
  'assert_direct_gcloud_auth' \
  'CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT' \
  'auth/impersonate_service_account'; do
  grep -Fq "$proof" "$seed_script" ||
    fail "initial seed does not reject delegated gcloud credentials: $proof"
done
for proof in \
  'iam.serviceAccounts.actAs' \
  'permissionType // "ADMIN_WRITE"' \
  'allowed_service_account_resource' \
  'allowed_run_resource' \
  'valid_run_mutation'; do
  grep -Fq "$proof" "$seed_audit_policy" ||
    fail "initial seed audit policy omits exact actAs control: $proof"
done
for proof in \
  'PYMES_CLOUD_RUN_SEED_APPLY' \
  'softponti@gmail.com' \
  'build-push-images.sh" verify-attestations' \
  'validate_github_release_state' \
  '--scaling=0' \
  '--no-traffic' \
  '--ingress=internal' \
  '--no-deploy-health-check' \
  '--clear-secrets' \
  '--clear-cloudsql-instances' \
  '--clear-network' \
  'gcloud run jobs executions list' \
  'No Cloud Run resources changed.'; do
  grep -Fq -- "$proof" "$seed_script" ||
    fail "inert Cloud Run seed lacks proof: $proof"
done
if grep -Eq -- '--execute-now|gcloud run jobs execute|--allow-unauthenticated|--no-allow-unauthenticated' "$seed_script"; then
  fail "inert Cloud Run seed must never execute a job or mutate Cloud Run IAM"
fi
grep -Fq 'release workflow forbids every project-scoped roles/run.admin binding' "$authority_verifier" ||
  fail "protected workflow may accept project-scoped Cloud Run administration"
grep -Fq 'resource_run_admin=exact' "$authority_verifier" ||
  fail "protected workflow does not require finalized resource-scoped authority"
for proof in \
  'validate_custom_role_json' \
  'validate_kms_custom_role_json' \
  'validate_wif_pool_json' \
  'validate_wif_provider_json' \
  'validate_release_account' \
  'validate_exact_member_role' \
  'validate_effective_iam_analysis' \
  'pymes_verify_release_inverse_authority' \
  'pymes_validate_release_pool_iam_assets' \
  'pymes_verify_release_account_not_attached' \
  'pymes_validate_enforced_boolean_org_policy' \
  'iam.disableCrossProjectServiceAccountUsage' \
  'iam.disableServiceAccountKeyCreation' \
  '--analyze-service-account-impersonation' \
  'serviceAccountImpersonationAnalysis' \
  'runtime_effective_iam_allowlist' \
  'validate_runtime_account_policy' \
  'runtime_iam=allowlisted' \
  'gcloud projects get-ancestors' \
  'release authority verifier must run as the exact environment deployer' \
  '--managed-by=user' \
  'read_effective_iam_analysis "$build_email"' \
  'read_effective_iam_analysis "$deploy_email"' \
  'release_secret_names' \
  'runtime_accounts' \
  'builder=keyless-exact' \
  'validate_complete_resource_authority' \
  'allowed seed or prior-stage state'; do
  grep -Fq -- "$proof" "$authority_verifier" ||
    fail "protected workflow does not revalidate complete release authority: $proof"
done
grep -Fq 'inverse_permissions=allowlisted' "$authority_verifier" ||
  fail "protected workflow does not report the inverse effective-permission gate"
grep -Fq 'source "$script_dir/release-authority-policy.sh"' "$deploy_script" ||
  fail "Cloud Run deployment does not load the shared inverse-authority policy"
grep -Fq 'pymes_verify_release_inverse_authority' "$deploy_script" ||
  fail "Cloud Run deployment can bypass the inverse effective-permission gate"
bash "$seed_test" >/dev/null ||
  fail "inert Cloud Run seed policy tests failed"
bash "$authority_test" >/dev/null ||
  fail "release authority policy tests failed"
grep -Fq 'TRANSITION WARNING retire legacy Pymes WIF trust' "$identity_script" ||
  fail "release bootstrap does not surface the temporary legacy authority"

for proof in \
  'does not run the exact release digest' \
  'current revision has the wrong release marker' \
  'latest created revision is not ready' \
  'min/max scale differs' \
  'CPU throttling differs' \
  'roles/run.invoker differs' \
  'JWKS does not match' \
  'does not bind $env_name to $secret_name exactly once' \
  'Clerk audience differs' \
  'uses network' \
  'Cloud SQL attachment differs' \
  'task count/max retries differs from 1/0' \
  'public readiness redirected outside HTTPS' \
  'public readiness does not expose the exact deployed Web release marker' \
  'public same-origin API proxy returned HTTP' \
  'public same-origin API proxy did not return the canonical FORBIDDEN JSON' \
  'public same-origin API proxy does not expose the exact Web release marker'; do
  grep -Fq "$proof" "$verify_script" ||
    fail "post-deploy verifier lacks proof: $proof"
done
grep -Fq ': "${PYMES_VPC_NETWORK:?set PYMES_VPC_NETWORK explicitly}"' "$verify_script" ||
  fail "post-deploy verifier must require the exact VPC network"
grep -Fq ': "${PYMES_VPC_SUBNET:?set PYMES_VPC_SUBNET explicitly}"' "$verify_script" ||
  fail "post-deploy verifier must require the exact VPC subnet"
[[ $(grep -Fc 'verify_secret_refs job ' "$verify_script") -eq 5 ]] ||
  fail "post-deploy verifier must allowlist secrets for all five Cloud Run jobs"
grep -Fq 'PYMES_SCHEDULING_ACTION_TOKEN_SECRET=${prefix}-scheduling-action-token-secret' "$verify_script" ||
  fail "post-deploy verifier does not bind the action-token secret exactly"

run_actionlint() {
  if command -v actionlint >/dev/null 2>&1; then
    actionlint "$@"
  else
    (
      cd "$repo_root"
      GOWORK=off go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 "$@"
    )
  fi
}

run_structured_policy() {
  (
    cd "$repo_root/v3/backend"
    GOWORK=off go run "$structured_policy" "$@"
  )
}

run_structured_policy_tests() {
  (
    cd "$repo_root/v3/backend"
    GOWORK=off go test "$structured_policy" "$structured_policy_test"
  )
}

run_structured_policy_tests
run_structured_policy "$ci_workflow" "$release_workflow"
run_actionlint "$ci_workflow" "$release_workflow"

# Exercise the fail-closed controls with disposable negative fixtures. This
# prevents a future refactor from preserving only the happy-path appearance.
fixture_directory=$(mktemp -d)
cleanup_fixtures() {
  rm -rf -- "$fixture_directory"
}
trap cleanup_fixtures EXIT

mutable_action_fixture="$fixture_directory/mutable-action.yml"
sed '0,/@[0-9a-f]\{40\}/s//@v7/' "$release_workflow" >"$mutable_action_fixture"
if (check_action_pins "$mutable_action_fixture") >/dev/null 2>&1; then
  fail "negative fixture: mutable action reference was accepted"
fi

mutable_base_fixture="$fixture_directory/mutable-base.Dockerfile"
sed '0,/@sha256:[0-9a-f]\{64\}/s///' "$pymes_dockerfile" >"$mutable_base_fixture"
if (check_docker_base_pins "$mutable_base_fixture") >/dev/null 2>&1; then
  fail "negative fixture: mutable Docker base was accepted"
fi

automatic_release_fixture="$fixture_directory/automatic-release.yml"
sed '0,/workflow_dispatch:/s//push:/' "$release_workflow" >"$automatic_release_fixture"
if release_is_manual_only "$automatic_release_fixture"; then
  fail "negative fixture: automatic release trigger was accepted"
fi

structured_bypass_fixture="$fixture_directory/structured-build-environment.yml"
sed '/^  build:$/a\
    environment:\
      name: ${{ inputs.environment }}' \
  "$release_workflow" >"$structured_bypass_fixture"
if run_structured_policy "$ci_workflow" "$structured_bypass_fixture" >/dev/null 2>&1; then
  fail "negative fixture: protected environment leakage into build was accepted"
fi

unsafe_stage_default_fixture="$fixture_directory/unsafe-stage-default.yml"
sed '0,/default: operational/s//default: bootstrap/' \
  "$release_workflow" >"$unsafe_stage_default_fixture"
if run_structured_policy "$ci_workflow" "$unsafe_stage_default_fixture" >/dev/null 2>&1; then
  fail "negative fixture: bootstrap was accepted as the default deploy stage"
fi

misordered_deploy_guard_fixture="$fixture_directory/misordered-deploy-guard.yml"
sed '/^      - name: Reject standalone deploy reruns$/i\
      - name: Unsafe checkout before deploy rerun guard\
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1' \
  "$release_workflow" >"$misordered_deploy_guard_fixture"
if run_structured_policy "$ci_workflow" "$misordered_deploy_guard_fixture" >/dev/null 2>&1; then
  fail "negative fixture: deploy checkout before the rerun guard was accepted"
fi

inline_mutable_action_fixture="$fixture_directory/inline-mutable-action.yml"
awk '
  {
    print
    if (!inserted && $0 == "    steps:") {
      print "      - {name: Inline mutable action, uses: actions/checkout@main}"
      inserted=1
    }
  }
' "$release_workflow" >"$inline_mutable_action_fixture"
if run_structured_policy "$ci_workflow" "$inline_mutable_action_fixture" >/dev/null 2>&1; then
  fail "negative fixture: inline mutable action reference was accepted"
fi

redirected_target_fixture="$fixture_directory/redirected-target.yml"
sed '0,/PYMES_GCP_PROJECT: pymes-dev-352318/s//PYMES_GCP_PROJECT: unrelated-project/' \
  "$release_workflow" >"$redirected_target_fixture"
if run_structured_policy "$ci_workflow" "$redirected_target_fixture" >/dev/null 2>&1; then
  fail "negative fixture: redirected GCP release target was accepted"
fi

invalid_yaml_fixture="$fixture_directory/invalid-workflow.yml"
cp -- "$release_workflow" "$invalid_yaml_fixture"
printf '\ninvalid: [unterminated\n' >>"$invalid_yaml_fixture"
if run_actionlint "$invalid_yaml_fixture" >/dev/null 2>&1; then
  fail "negative fixture: malformed workflow YAML was accepted"
fi

echo "GitHub Actions and immutable release policy verified"
