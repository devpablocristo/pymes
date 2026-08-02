#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
subject="$script_dir/rollback-cloud-run.sh"
fixture_dir="$script_dir/testdata/rollback-cloud-run"

release_sha=1111111111111111111111111111111111111111
accounting_sha=ad1c182093986279aac7fb6582f7788202112a78
token=2222222222222222222222222222222222222222222222222222222222222222
project=pymes-dev-352318
region=us-central1
api_service=pymes-v3-stg-api
web_service=pymes-v3-stg-web
api_service_account="pymes-v3-api-stg@${project}.iam.gserviceaccount.com"
web_service_account="pymes-v3-web-stg@${project}.iam.gserviceaccount.com"
api_revision=pymes-v3-stg-api-00042-good
web_revision=pymes-v3-stg-web-00042-good
candidate_tag="c-${release_sha:0:16}"
api_origin="https://${api_service}-fakehash-uscentral1.a.run.app"
api_tag_origin="https://${candidate_tag}---${api_origin#https://}"
api_digest=$(printf 'a%.0s' {1..64})
web_digest=$(printf 'b%.0s' {1..64})
api_image="${region}-docker.pkg.dev/${project}/pymes/pymes-v3-api@sha256:${api_digest}"
web_image="${region}-docker.pkg.dev/${project}/pymes/pymes-v3-web-stg@sha256:${web_digest}"
web_marker="stg:${release_sha}:sha256:${web_digest}"
registry="${region}-docker.pkg.dev/${project}/pymes"
accounting_context=${ACCOUNTING_BUILD_CONTEXT:-"$script_dir/../../../../open-accounting"}
accounting_context=$(cd -- "$accounting_context" && pwd)

fail() {
  echo "rollback-cloud-run test failed: $*" >&2
  exit 1
}

make_revision() {
  local state="$1" service="$2" revision="$3" image="$4" role="$5"
  local selected_token=${6:-$token}
  local selected_upstream=${7:-$api_tag_origin}
  local service_account env_json
  if [[ "$role" == "api" ]]; then
    service_account=$api_service_account
    env_json=$(jq -cn \
      --arg tag "$candidate_tag" \
      --arg token "$selected_token" '
        [
          {name: "PYMES_PREFLIGHT_TAG", value: $tag},
          {name: "PYMES_PREFLIGHT_TOKEN", value: $token}
        ]
      ')
  else
    service_account=$web_service_account
    env_json=$(jq -cn \
      --arg tag "$candidate_tag" \
      --arg token "$selected_token" \
      --arg upstream "$selected_upstream" \
      --arg marker "$web_marker" '
        [
          {name: "PYMES_PREFLIGHT_TAG", value: $tag},
          {name: "PYMES_PREFLIGHT_TOKEN", value: $token},
          {name: "PYMES_API_UPSTREAM", value: $upstream},
          {name: "PYMES_RELEASE_MARKER", value: $marker}
        ]
      ')
  fi
  jq -n \
    --arg name "$revision" \
    --arg service "$service" \
    --arg release "$release_sha" \
    --arg image "$image" \
    --arg service_account "$service_account" \
    --argjson env "$env_json" '
      {
        metadata: {
          name: $name,
          labels: {
            app: "pymes-v3",
            env: "stg",
            "pymes-v3-release": $release,
            "serving.knative.dev/service": $service
          }
        },
        spec: {
          serviceAccountName: $service_account,
          containers: [
            {
              image: $image,
              env: $env
            }
          ]
        },
        status: {
          conditions: [
            {type: "Ready", status: "True"}
          ]
        }
      }
    ' >"$state/revisions/$revision.json"
}

test_wrong_revision_identity_fails_before_mutation() {
  local state output
  state=$(new_state)
  output="$state/output"
  jq '.spec.serviceAccountName = "unexpected@pymes-dev-352318.iam.gserviceaccount.com"' \
    "$state/revisions/$api_revision.json" \
    >"$state/revisions/$api_revision.tmp"
  mv "$state/revisions/$api_revision.tmp" \
    "$state/revisions/$api_revision.json"
  if run_subject "$state" "$output"; then
    fail "API revision with an unexpected workload identity was accepted"
  fi
  ! grep -Fq 'update-traffic' "$state/commands.log" ||
    fail "unexpected revision identity mutated Cloud Run"
  rm -rf -- "$state"
}

make_list() {
  local state="$1" service="$2"
  shift 2
  printf '%s\n' "$@" |
    jq -Rn \
      --arg release "$release_sha" \
      '[inputs | {
        metadata: {
          name: .,
          labels: {"pymes-v3-release": $release}
        }
      }]' >"$state/lists/$service.json"
}

make_manifest() {
  local state="$1"
  {
    printf 'PYMES_RELEASE_ENV=stg\n'
    printf 'PYMES_SOURCE_SHA=%s\n' "$release_sha"
    printf 'PYMES_OPEN_ACCOUNTING_SOURCE_SHA=%s\n' "$accounting_sha"
    printf 'PYMES_API_IMAGE=%s\n' "$api_image"
    printf 'PYMES_WEB_IMAGE=%s\n' "$web_image"
    printf 'PYMES_WORKER_IMAGE=%s/pymes-v3-worker@sha256:%064d\n' "$registry" 3
    printf 'PYMES_FISCAL_IMAGE=%s/pymes-v3-fiscal@sha256:%064d\n' "$registry" 4
    printf 'PYMES_ACCOUNTING_IMAGE=%s/pymes-v3-accounting@sha256:%064d\n' "$registry" 5
    printf 'PYMES_ACCOUNTING_ADMIN_IMAGE=%s/pymes-v3-accounting-admin@sha256:%064d\n' "$registry" 6
    printf 'PYMES_PROVISION_IMAGE=%s/pymes-v3-provision@sha256:%064d\n' "$registry" 7
    printf 'PYMES_MIGRATE_IMAGE=%s/pymes-v3-migrate@sha256:%064d\n' "$registry" 8
    printf 'PYMES_FISCAL_MIGRATE_IMAGE=%s/pymes-v3-fiscal-migrate@sha256:%064d\n' "$registry" 9
    printf 'PYMES_ACCOUNTING_MIGRATE_IMAGE=%s/pymes-v3-accounting-migrate@sha256:%064d\n' "$registry" 0
  } >"$state/release-manifest.env"
}

new_state() {
  local state
  state=$(mktemp -d)
  mkdir -p "$state/bin" "$state/lists" "$state/revisions"
  ln -s "$fixture_dir/gcloud" "$state/bin/gcloud"
  ln -s "$fixture_dir/curl" "$state/bin/curl"
  ln -s "$fixture_dir/git" "$state/bin/git"
  ln -s "$fixture_dir/docker" "$state/bin/docker"
  : >"$state/commands.log"
  printf '%s' pymes-v3-stg-api-00041-current >"$state/active-$api_service"
  printf '%s' pymes-v3-stg-web-00041-current >"$state/active-$web_service"
  printf 'current-api=pymes-v3-stg-api-00041-current\n' >"$state/tags-$api_service"
  printf 'current-web=pymes-v3-stg-web-00041-current\n' >"$state/tags-$web_service"
  printf 'false' >"$state/invoker-disabled-$api_service"
  printf 'false' >"$state/invoker-disabled-$web_service"
  printf 'all' >"$state/ingress-$api_service"
  printf 'all' >"$state/ingress-$web_service"
  for service in "$api_service" "$web_service"; do
    jq -n '{
      bindings: [
        {
          role: "roles/run.invoker",
          members: ["allUsers"]
        }
      ]
    }' >"$state/policy-$service.json"
  done
  make_revision "$state" "$api_service" "$api_revision" "$api_image" api
  make_revision "$state" "$web_service" "$web_revision" "$web_image" web
  make_list "$state" "$api_service" "$api_revision"
  make_list "$state" "$web_service" "$web_revision"
  make_manifest "$state"
  printf '%s' "$state"
}

run_subject() {
  local state="$1" output="$2"
  local manifest_checksum
  shift 2
  manifest_checksum=$(sha256sum "$state/release-manifest.env" | awk '{print $1}')
  env \
    PATH="$state/bin:$PATH" \
    FAKE_STATE_DIR="$state" \
    FAKE_API_SERVICE="$api_service" \
    FAKE_WEB_SERVICE="$web_service" \
    FAKE_TOKEN="$token" \
    FAKE_RELEASE_SHA="$release_sha" \
    FAKE_ACCOUNTING_SHA="$accounting_sha" \
    FAKE_ACCOUNTING_CONTEXT="$accounting_context" \
    PYMES_DEPLOY_ENV=stg \
    PYMES_ROLLBACK_RELEASE_SHA="$release_sha" \
    PYMES_ROLLBACK_IMAGE_MANIFEST="$state/release-manifest.env" \
    PYMES_ROLLBACK_IMAGE_MANIFEST_SHA256="$manifest_checksum" \
    OPEN_ACCOUNTING_CONTEXT="$accounting_context" \
    PYMES_GCP_PROJECT="$project" \
    PYMES_GCP_REGION="$region" \
    "$@" \
    "$subject" >"$output" 2>&1
}

assert_no_traffic_mutation() {
  local state="$1" service="$2"
  ! grep -F "update-traffic $service" "$state/commands.log" |
    grep -Fq -- '--to-revisions=' ||
    fail "traffic for $service moved unexpectedly"
}

assert_mutation_order() {
  local state="$1" tag_line curl_line web_line api_line
  tag_line=$(grep -nF -- "--update-tags=${candidate_tag}=${api_revision}" \
    "$state/commands.log" | head -1 | cut -d: -f1)
  curl_line=$(grep -nF "CURL ${api_tag_origin}/readyz" \
    "$state/commands.log" | head -1 | cut -d: -f1)
  web_line=$(grep -nF "update-traffic $web_service" "$state/commands.log" |
    grep -F -- "--to-revisions=${web_revision}=100" |
    head -1 | cut -d: -f1)
  api_line=$(grep -nF "update-traffic $api_service" "$state/commands.log" |
    grep -F -- "--to-revisions=${api_revision}=100" |
    head -1 | cut -d: -f1)
  [[ "$tag_line" =~ ^[0-9]+$ &&
      "$curl_line" =~ ^[0-9]+$ &&
      "$web_line" =~ ^[0-9]+$ &&
      "$api_line" =~ ^[0-9]+$ &&
      "$tag_line" -lt "$curl_line" &&
      "$curl_line" -lt "$web_line" &&
      "$web_line" -lt "$api_line" ]] ||
    fail "rollback did not prove the API tag before moving Web and API traffic"
}

test_manifest_checksum_mismatch_fails_before_cloud_run() {
  local state output wrong_checksum
  state=$(new_state)
  output="$state/output"
  wrong_checksum=$(printf 'f%.0s' {1..64})
  if run_subject "$state" "$output" \
    PYMES_ROLLBACK_IMAGE_MANIFEST_SHA256="$wrong_checksum"; then
    fail "manifest with a mismatched recorded checksum was accepted"
  fi
  ! grep -Fq 'GCLOUD' "$state/commands.log" ||
    fail "checksum mismatch reached the Cloud Run control plane"
  rm -rf -- "$state"
}

test_noncanonical_manifest_image_fails_before_cloud_run() {
  local state output
  state=$(new_state)
  output="$state/output"
  sed \
    's#pymes/pymes-v3-api@#other/pymes-v3-api@#' \
    "$state/release-manifest.env" >"$state/release-manifest.tmp"
  mv "$state/release-manifest.tmp" "$state/release-manifest.env"
  if run_subject "$state" "$output"; then
    fail "manifest image outside the canonical repository was accepted"
  fi
  ! grep -Fq 'GCLOUD' "$state/commands.log" ||
    fail "noncanonical manifest reached the Cloud Run control plane"
  rm -rf -- "$state"
}

test_invalid_attestation_fails_before_cloud_run() {
  local state output
  state=$(new_state)
  output="$state/output"
  if run_subject "$state" "$output" FAKE_ATTESTATIONS_VALID=false; then
    fail "image with invalid release metadata/attestation was accepted"
  fi
  ! grep -Fq 'GCLOUD' "$state/commands.log" ||
    fail "invalid attestation reached the Cloud Run control plane"
  rm -rf -- "$state"
}

test_revision_image_not_in_manifest_fails_before_mutation() {
  local state output other_digest other_image
  state=$(new_state)
  output="$state/output"
  other_digest=$(printf 'c%.0s' {1..64})
  other_image="${registry}/pymes-v3-api@sha256:${other_digest}"
  jq --arg image "$other_image" \
    '.spec.containers[0].image = $image' \
    "$state/revisions/$api_revision.json" \
    >"$state/revisions/$api_revision.tmp"
  mv "$state/revisions/$api_revision.tmp" \
    "$state/revisions/$api_revision.json"
  if run_subject "$state" "$output"; then
    fail "revision image differing from the release manifest was accepted"
  fi
  ! grep -Fq 'update-traffic' "$state/commands.log" ||
    fail "revision/manifest mismatch mutated Cloud Run"
  rm -rf -- "$state"
}

test_success_and_final_policy() {
  local state output
  state=$(new_state)
  output="$state/output"
  run_subject "$state" "$output" ||
    fail "valid rollback failed: $(<"$output")"

  [[ "$(<"$state/active-$web_service")" == "$web_revision" ]] ||
    fail "Web target was not activated"
  [[ "$(<"$state/active-$api_service")" == "$api_revision" ]] ||
    fail "API target was not activated"
  [[ "$(<"$state/tags-$api_service")" == "${candidate_tag}=${api_revision}" ]] ||
    fail "API did not retain exactly the rollback tag"
  [[ ! -s "$state/tags-$web_service" ]] ||
    fail "Web retained revision tags"
  grep -Fq 'database=unchanged' "$output" ||
    fail "rollback did not state that the database is unchanged"
  assert_mutation_order "$state"

  if grep '^GCLOUD' "$state/commands.log" |
    grep -Fv -- "--project=$project" >/dev/null; then
    fail "a gcloud command omitted the explicit project"
  fi
  if grep '^GCLOUD' "$state/commands.log" |
    grep -Fv -- "--region=$region" >/dev/null; then
    fail "a gcloud command omitted the explicit region"
  fi
  if grep -Fq "$token" "$output" ||
    grep -Fq "$token" "$state/commands.log"; then
    fail "preflight capability leaked to logs"
  fi
  rm -rf -- "$state"
}

test_invalid_pair_fails_before_mutation() {
  local state output mismatched_token
  state=$(new_state)
  output="$state/output"
  mismatched_token=$(printf '3%.0s' {1..64})
  make_revision "$state" "$web_service" "$web_revision" "$web_image" web \
    "$mismatched_token"
  if run_subject "$state" "$output"; then
    fail "mismatched Web/API capability was accepted"
  fi
  ! grep -Fq 'update-traffic' "$state/commands.log" ||
    fail "invalid pair mutated Cloud Run"
  rm -rf -- "$state"
}

test_duplicate_release_revision_fails_before_mutation() {
  local state output duplicate
  state=$(new_state)
  output="$state/output"
  duplicate=pymes-v3-stg-api-00040-duplicate
  make_revision "$state" "$api_service" "$duplicate" "$api_image" api
  make_list "$state" "$api_service" "$api_revision" "$duplicate"
  if run_subject "$state" "$output"; then
    fail "duplicate release revisions were accepted"
  fi
  ! grep -Fq 'update-traffic' "$state/commands.log" ||
    fail "duplicate revision discovery mutated Cloud Run"
  rm -rf -- "$state"
}

test_disabled_invoker_iam_fails_before_mutation() {
  local state output
  state=$(new_state)
  output="$state/output"
  printf 'true' >"$state/invoker-disabled-$api_service"
  if run_subject "$state" "$output"; then
    fail "disabled API invoker IAM check was accepted"
  fi
  ! grep -Fq 'update-traffic' "$state/commands.log" ||
    fail "disabled invoker IAM check mutated Cloud Run"
  rm -rf -- "$state"
}

test_unexpected_invoker_policy_fails_before_mutation() {
  local state output
  state=$(new_state)
  output="$state/output"
  jq -n '{
    bindings: [
      {
        role: "roles/run.invoker",
        members: [
          "allUsers",
          "serviceAccount:unexpected@example.iam.gserviceaccount.com"
        ]
      }
    ]
  }' >"$state/policy-$web_service.json"
  if run_subject "$state" "$output"; then
    fail "unexpected Web invoker policy was accepted"
  fi
  ! grep -Fq 'update-traffic' "$state/commands.log" ||
    fail "unexpected invoker policy mutated Cloud Run"
  rm -rf -- "$state"
}

test_applied_timeout_recovers_by_readback() {
  local state output
  state=$(new_state)
  output="$state/output"
  run_subject "$state" "$output" FAKE_TAG_TIMEOUT_MODE=applied ||
    fail "applied tag timeout did not recover: $(<"$output")"
  grep -Fq 'readback proved the requested state' "$output" ||
    fail "applied timeout was not reported as readback recovery"
  assert_mutation_order "$state"
  rm -rf -- "$state"
}

test_unapplied_timeout_never_moves_web() {
  local state output
  state=$(new_state)
  output="$state/output"
  if run_subject "$state" "$output" FAKE_TAG_TIMEOUT_MODE=not-applied; then
    fail "unapplied tag timeout was accepted"
  fi
  assert_no_traffic_mutation "$state" "$web_service"
  assert_no_traffic_mutation "$state" "$api_service"
  rm -rf -- "$state"
}

test_api_probe_timeout_never_moves_web() {
  local state output
  state=$(new_state)
  output="$state/output"
  if run_subject "$state" "$output" FAKE_CURL_TIMEOUT=tagged; then
    fail "tagged API timeout was accepted"
  fi
  assert_no_traffic_mutation "$state" "$web_service"
  assert_no_traffic_mutation "$state" "$api_service"
  grep -Fq "${candidate_tag}=${api_revision}" "$state/tags-$api_service" ||
    fail "probe timeout did not preserve the viable API tag for inspection"
  rm -rf -- "$state"
}

test_manifest_checksum_mismatch_fails_before_cloud_run
test_noncanonical_manifest_image_fails_before_cloud_run
test_invalid_attestation_fails_before_cloud_run
test_revision_image_not_in_manifest_fails_before_mutation
test_success_and_final_policy
test_invalid_pair_fails_before_mutation
test_duplicate_release_revision_fails_before_mutation
test_wrong_revision_identity_fails_before_mutation
test_disabled_invoker_iam_fails_before_mutation
test_unexpected_invoker_policy_fails_before_mutation
test_applied_timeout_recovers_by_readback
test_unapplied_timeout_never_moves_web
test_api_probe_timeout_never_moves_web

echo "rollback-cloud-run tests passed"
