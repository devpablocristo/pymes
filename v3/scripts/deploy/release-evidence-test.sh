#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=bootstrap-release-evidence.sh
source "$script_dir/bootstrap-release-evidence.sh"
# shellcheck source=retain-release-manifest.sh
source "$script_dir/retain-release-manifest.sh"

scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT

sha=0123456789abcdef0123456789abcdef01234567
accounting_sha=89abcdef0123456789abcdef0123456789abcdef
digest=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
fake_bucket_exists=false
fake_bucket_locked=false
fake_bucket_bad_region=false
fake_legacy_bucket_reader=false
fake_object_creator=false
fake_object_viewer=false
fake_evidence_iam_reader=false
fake_iam_read_role_exists=false
fake_iam_read_role_broader=false
fake_account=softponti@gmail.com
fake_impersonation='(unset)'
fake_object_exists=false
fake_commands="$scratch/commands.log"
fake_object="$scratch/retained-object"

fail_test() {
  echo "release evidence test failed: $*" >&2
  exit 1
}

expect_failure() {
  local description="$1"
  shift
  if "$@" >"$scratch/expected-failure.log" 2>&1; then
    fail_test "$description"
  fi
}

bucket_json() {
  local environment=${PYMES_RELEASE_EVIDENCE_ENV:-stg}
  local region=US-CENTRAL1
  [[ "$fake_bucket_bad_region" == false ]] || region=EUROPE-WEST1
  jq -n \
    --arg name "pymes-v3-release-evidence-${environment}-884236221349" \
    --arg region "$region" \
    --arg environment "$environment" \
    --argjson locked "$fake_bucket_locked" '
      {
        name: $name,
        projectNumber: "884236221349",
        location: $region,
        storageClass: "STANDARD",
        iamConfiguration: {
          uniformBucketLevelAccess: {enabled: true},
          publicAccessPrevention: "enforced"
        },
        versioning: {enabled: false},
        lifecycle: {rule: []},
        retentionPolicy: {
          retentionPeriod: "31557600",
          isLocked: $locked
        },
        labels: {
          app: "pymes-v3",
          component: "release-evidence",
          environment: $environment,
          managed_by: "pymes-v3"
        }
      }
    '
}

policy_json() {
  jq -n \
    --arg member \
      "serviceAccount:pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com" \
    --arg iam_read_role "$PYMES_RELEASE_EVIDENCE_IAM_READ_ROLE" \
    --argjson legacy_bucket_reader "$fake_legacy_bucket_reader" \
    --argjson object_creator "$fake_object_creator" \
    --argjson object_viewer "$fake_object_viewer" \
    --argjson evidence_iam_reader "$fake_evidence_iam_reader" '
      {
        bindings: (
          (if $legacy_bucket_reader then [{
            role: "roles/storage.legacyBucketReader",
            members: [$member]
          }] else [] end) +
          (if $object_creator then [{
            role: "roles/storage.objectCreator",
            members: [$member]
          }] else [] end) +
          (if $object_viewer then [{
            role: "roles/storage.objectViewer",
            members: [$member]
          }] else [] end) +
          (if $evidence_iam_reader then [{
            role: $iam_read_role,
            members: [$member]
          }] else [] end)
        )
      }
    '
}

iam_read_role_json() {
  jq -n \
    --arg name "$PYMES_RELEASE_EVIDENCE_IAM_READ_ROLE" \
    --argjson broader "$fake_iam_read_role_broader" '
      {
        name: $name,
        stage: "GA",
        includedPermissions: (
          ["storage.buckets.getIamPolicy"] +
          (if $broader then ["storage.buckets.setIamPolicy"] else [] end)
        )
      }
    '
}

gcloud() {
  {
    printf 'gcloud'
    printf ' %q' "$@"
    printf '\n'
  } >>"$fake_commands"
  case "$1 ${2:-} ${3:-}" in
    "config get-value account")
      echo "$fake_account"
      ;;
    "config get-value auth/impersonate_service_account")
      echo "$fake_impersonation"
      ;;
    "projects describe pymes-dev-352318")
      echo '{"projectId":"pymes-dev-352318","projectNumber":"884236221349","lifecycleState":"ACTIVE"}'
      ;;
    "iam roles describe")
      [[ "$fake_iam_read_role_exists" == true ]] || return 1
      iam_read_role_json
      ;;
    "iam roles create")
      [[ "$fake_iam_read_role_exists" == false ]] || return 1
      fake_iam_read_role_exists=true
      ;;
    "storage buckets describe")
      [[ "$fake_bucket_exists" == true ]] || return 1
      bucket_json
      ;;
    "storage buckets create")
      [[ "$fake_bucket_exists" == false ]] || return 1
      fake_bucket_exists=true
      ;;
    "storage buckets update")
      if [[ " $* " == *" --lock-retention-period "* ]]; then
        fake_bucket_locked=true
      fi
      ;;
    "storage buckets add-iam-policy-binding")
      case " $* " in
        *" --role=roles/storage.legacyBucketReader "*)
          fake_legacy_bucket_reader=true
          ;;
        *" --role=roles/storage.objectCreator "*)
          fake_object_creator=true
          ;;
        *" --role=roles/storage.objectViewer "*)
          fake_object_viewer=true
          ;;
        *" --role=${PYMES_RELEASE_EVIDENCE_IAM_READ_ROLE} "*)
          fake_evidence_iam_reader=true
          ;;
        *)
          return 1
          ;;
      esac
      ;;
    "storage buckets get-iam-policy")
      if [[ "$fake_account" == "$PYMES_RELEASE_EVIDENCE_BUILDER" ]]; then
        [[ "$fake_iam_read_role_exists" == true &&
           "$fake_evidence_iam_reader" == true ]] || return 1
      fi
      policy_json
      ;;
    "storage cp "*)
      local source=${3:-} destination=${4:-}
      if [[ "$source" == gs://* ]]; then
        source=${source%#1}
        [[ -f "$fake_object" ]] || return 1
        cp "$fake_object" "$destination"
      else
        [[ "$fake_object_exists" == false ]] || return 1
        cp "$source" "$fake_object"
        fake_object_exists=true
      fi
      ;;
    "storage objects describe")
      [[ "$fake_object_exists" == true ]] || return 1
      local environment=${PYMES_RELEASE_EVIDENCE_ENV}
      local source_sha=${PYMES_RELEASE_EVIDENCE_SOURCE_SHA}
      local selected_accounting_sha=${PYMES_RELEASE_EVIDENCE_ACCOUNTING_SHA}
      local run_id=${PYMES_RELEASE_EVIDENCE_RUN_ID}
      local checksum size name
      checksum=$(sha256sum "$fake_object" | awk '{print $1}')
      size=$(stat -c '%s' "$fake_object")
      name="releases/${environment}/${source_sha}/github-run-${run_id}/pymes-v3-images.env"
      jq -n \
        --arg bucket "pymes-v3-release-evidence-${environment}-884236221349" \
        --arg name "$name" \
        --arg environment "$environment" \
        --arg source_sha "$source_sha" \
        --arg accounting_sha "$selected_accounting_sha" \
        --arg checksum "$checksum" \
        --arg run_id "$run_id" \
        --arg size "$size" '
          {
            bucket: $bucket,
            name: $name,
            generation: "1",
            size: $size,
            contentType: "text/plain",
            retentionExpirationTime: "2027-08-01T00:00:00Z",
            metadata: {
              environment: $environment,
              source_sha: $source_sha,
              open_accounting_sha: $accounting_sha,
              manifest_sha256: $checksum,
              github_run_id: $run_id,
              github_run_attempt: "1"
            }
          }
        '
      ;;
    *)
      return 1
      ;;
  esac
}

write_manifest() {
  local destination="$1" extra=${2:-}
  {
    printf 'PYMES_RELEASE_ENV=stg\n'
    printf 'PYMES_SOURCE_SHA=%s\n' "$sha"
    printf 'PYMES_OPEN_ACCOUNTING_SOURCE_SHA=%s\n' "$accounting_sha"
    printf 'PYMES_API_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-api@sha256:%s\n' "$digest"
    printf 'PYMES_WEB_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-web-stg@sha256:%s\n' "$digest"
    for image in \
      worker:pymes-v3-worker \
      fiscal:pymes-v3-fiscal \
      accounting:pymes-v3-accounting \
      accounting_admin:pymes-v3-accounting-admin \
      provision:pymes-v3-provision \
      migrate:pymes-v3-migrate \
      fiscal_migrate:pymes-v3-fiscal-migrate \
      accounting_migrate:pymes-v3-accounting-migrate; do
      key=${image%%:*}
      name=${image#*:}
      printf 'PYMES_%s_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/%s@sha256:%s\n' \
        "${key^^}" "$name" "$digest"
    done
    [[ -z "$extra" ]] || printf '%s\n' "$extra"
  } >"$destination"
}

run_bootstrap() {
  PYMES_RELEASE_EVIDENCE_ENV=stg \
  PYMES_RELEASE_EVIDENCE_MODE="$1" \
  PYMES_RELEASE_EVIDENCE_LOCK_CONFIRMATION="${2:-}" \
    bootstrap_release_evidence_main
}

run_retain() {
  local selected_run_id="$1"
  PYMES_RELEASE_EVIDENCE_ENV=stg \
  PYMES_RELEASE_EVIDENCE_SOURCE_SHA="$sha" \
  PYMES_RELEASE_EVIDENCE_ACCOUNTING_SHA="$accounting_sha" \
  PYMES_RELEASE_EVIDENCE_RUN_ID="$selected_run_id" \
  PYMES_RELEASE_EVIDENCE_RUN_ATTEMPT=1 \
  PYMES_RELEASE_EVIDENCE_MANIFEST="$manifest" \
  PYMES_RELEASE_EVIDENCE_RECEIPT="$receipt" \
    retain_release_manifest_main
}

exact_iam_read_role=$(iam_read_role_json)
pymes_release_evidence_validate_iam_read_role "$exact_iam_read_role"
expect_failure \
  "IAM reader validator accepted an absent permission" \
  pymes_release_evidence_validate_iam_read_role \
  "$(jq '.includedPermissions = []' <<<"$exact_iam_read_role")"
expect_failure \
  "IAM reader validator accepted a broader permission set" \
  pymes_release_evidence_validate_iam_read_role \
  "$(jq '.includedPermissions += ["storage.buckets.setIamPolicy"]' \
    <<<"$exact_iam_read_role")"

: >"$fake_commands"
run_bootstrap plan >/dev/null
[[ ! -s "$fake_commands" ]] ||
  fail_test "plan mode called gcloud"

run_bootstrap apply >/dev/null
[[ "$fake_bucket_exists" == true &&
   "$fake_iam_read_role_exists" == true &&
   "$fake_legacy_bucket_reader" == true &&
   "$fake_object_creator" == true &&
   "$fake_object_viewer" == true &&
   "$fake_evidence_iam_reader" == true ]] ||
  fail_test "apply did not prepare the bucket and writer"
[[ "$fake_bucket_locked" == false ]] ||
  fail_test "apply irreversibly locked the bucket"
[[ "$(grep -Fc \
  "gcloud storage buckets add-iam-policy-binding gs://pymes-v3-release-evidence-stg-884236221349 --member=serviceAccount:pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com --role=projects/pymes-dev-352318/roles/pymesV3ReleaseEvidenceIamRead" \
  "$fake_commands")" -eq 1 ]] ||
  fail_test "apply did not bind the IAM reader only on the evidence bucket"
if grep -Eq \
  '^gcloud (projects|organizations|resource-manager folders) .*iam-policy-binding' \
  "$fake_commands"; then
  fail_test "apply granted the evidence reader outside its target bucket"
fi

fake_iam_read_role_broader=true
expect_failure \
  "apply accepted a broader pre-existing IAM reader role" \
  run_bootstrap apply
fake_iam_read_role_broader=false

expect_failure "verify accepted an unlocked bucket" run_bootstrap verify
fake_iam_read_role_exists=false
expect_failure "verify accepted an absent IAM reader role" run_bootstrap verify
fake_iam_read_role_exists=true
fake_evidence_iam_reader=false
expect_failure \
  "lock accepted a missing bucket-scoped IAM reader binding" \
  run_bootstrap lock \
  "LOCK_RELEASE_EVIDENCE_STG_pymes-v3-release-evidence-stg-884236221349"
fake_evidence_iam_reader=true
expect_failure "lock accepted no irreversible confirmation" run_bootstrap lock
[[ "$fake_bucket_locked" == false ]] ||
  fail_test "failed lock mutated retention"
lock_ack="LOCK_RELEASE_EVIDENCE_STG_pymes-v3-release-evidence-stg-884236221349"
run_bootstrap lock "$lock_ack" >/dev/null
[[ "$fake_bucket_locked" == true ]] ||
  fail_test "confirmed lock did not converge"
run_bootstrap verify >/dev/null

fake_iam_read_role_broader=true
expect_failure "verify accepted a broader IAM reader role" run_bootstrap verify
fake_iam_read_role_broader=false
fake_evidence_iam_reader=false
expect_failure \
  "verify accepted a missing bucket-scoped IAM reader binding" \
  run_bootstrap verify
fake_evidence_iam_reader=true
fake_bucket_bad_region=true
expect_failure "verify accepted a bucket in another region" run_bootstrap verify
fake_bucket_bad_region=false

manifest="$scratch/manifest.env"
receipt="$scratch/receipt.json"
write_manifest "$manifest"
fake_account="$PYMES_RELEASE_EVIDENCE_BUILDER"
: >"$fake_commands"
run_retain 123456 >/dev/null
[[ "$(grep -Fc \
  "gcloud storage buckets get-iam-policy gs://pymes-v3-release-evidence-stg-884236221349 --format=json" \
  "$fake_commands")" -eq 1 ]] ||
  fail_test "publisher did not validate live evidence-bucket IAM exactly once"
jq -e \
  --arg sha "$sha" \
  --arg checksum "$(sha256sum "$manifest" | awk '{print $1}')" '
    .schema == "pymes-v3-release-evidence-v1" and
    .source_sha == $sha and
    .manifest_sha256 == $checksum and
    .generation == "1"
  ' "$receipt" >/dev/null
[[ "$(stat -c '%a' "$receipt")" == 600 ]] ||
  fail_test "receipt permissions are not private"

rm -f "$receipt"
expect_failure "create-only publication overwrote an existing object" \
  run_retain 123456

fake_object_exists=false
rm -f "$fake_object" "$receipt"
write_manifest "$manifest" 'UNREVIEWED=value'
commands_before=$(wc -l <"$fake_commands")
expect_failure "publisher accepted an unknown manifest key" \
  run_retain 123457
commands_after=$(wc -l <"$fake_commands")
[[ "$commands_before" == "$commands_after" ]] ||
  fail_test "invalid manifest reached gcloud"

echo "release evidence tests passed: plan-only bootstrap, bucket-scoped IAM reader, irreversible lock guard, exact IAM, create-only retention and receipt verification"
