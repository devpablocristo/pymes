#!/usr/bin/env bash
set -uo pipefail

# Stateful fault-injection coverage for the release transaction. The test loads
# the transaction functions directly from cloud-run.sh and executes them with a
# fake Cloud Run control/data plane. It deliberately does not exercise the
# network, KMS, Secret Manager, build or GitHub preflights; those remain covered
# by their dedicated policy tests. The worker-release signal uses the real
# transaction function with logical time supplied by the fake PATH.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
deploy_script="$script_dir/cloud-run.sh"
fixture_dir="$script_dir/testdata/cloud-run-transaction"
fake_bin="$fixture_dir/bin"
# shellcheck source=release-candidate-tag.sh
source "$script_dir/release-candidate-tag.sh"

scratch_dir=$(mktemp -d)
cleanup() {
  rm -rf -- "$scratch_dir"
}
trap cleanup EXIT

export PATH="$fake_bin:$PATH"
export FAKE_GCLOUD_CALL_LOG="$scratch_dir/gcloud-calls.jsonl"
export FAKE_CURL_CALL_LOG="$scratch_dir/curl-calls.jsonl"
: >"$FAKE_GCLOUD_CALL_LOG"
: >"$FAKE_CURL_CALL_LOG"

failures=0

record_failure() {
  echo "FAIL cloud-run transaction: $*" >&2
  failures=$((failures + 1))
}

record_pass() {
  echo "PASS cloud-run transaction: $*"
}

extract_function() {
  local name="$1"
  awk -v signature="^${name}[(][)] [{]$" '
    $0 ~ signature { copying=1 }
    copying { print }
    copying && /^}$/ { exit }
  ' "$deploy_script"
}

load_real_function() {
  local name="$1" definition
  definition=$(extract_function "$name")
  if [[ -z "$definition" ]]; then
    echo "could not extract $name from $deploy_script" >&2
    exit 1
  fi
  eval "$definition"
}

for function_name in \
  gcloud_command \
  service_invoker_iam_check_enabled \
  ensure_service_invoker \
  existing_sidecars \
  append_sidecar_removal \
  create_cloud_run_environment_file \
  cleanup_release_secret_files \
  capture_previous_revision \
  revision_env_value \
  validate_release_baseline \
  record_candidate_revision \
  deploy \
  deploy_web \
  assert_active_revision \
  assert_worker_manual_scaling \
  service_absent \
  fail_close_service \
  service_was_promoted \
  restore_previous_api_tags \
  wait_for_tagged_api_ready \
  discover_candidate_revision \
  assert_tag_absent \
  assert_public_tag_url_revoked \
  remove_candidate_tag \
  remove_nonworker_candidate_tags \
  settle_release_tags \
  assert_revision_absent \
  assert_service_absent \
  quiesce_worker_candidate \
  rollback_on_exit \
  wait_for_worker_release_ready \
  promote_service; do
  load_real_function "$function_name"
done

project=pymes-dev-352318
region=us-central1
prefix=pymes-v3-stg
PYMES_RELEASE_SHA=2222222222222222222222222222222222222222
candidate_tag=$(pymes_release_candidate_tag "$PYMES_RELEASE_SHA")
dry_run=false
network=pymes-v3-serverless
subnet=pymes-v3-serverless
PYMES_CLOUDSQL_INSTANCE=pymes-dev-352318:us-central1:pymes
PYMES_DEPLOY_ENV=stg
deploy_stage=operational
PYMES_PREFLIGHT_TOKEN=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
web_release_marker="stg:${PYMES_RELEASE_SHA}:sha256:new"
api_sa="pymes-v3-api-stg@${project}.iam.gserviceaccount.com"
web_sa="pymes-v3-web-stg@${project}.iam.gserviceaccount.com"
worker_sa="pymes-v3-worker-stg@${project}.iam.gserviceaccount.com"
provision_sa="pymes-v3-provision-stg@${project}.iam.gserviceaccount.com"
fiscal_sa="pymes-v3-fiscal-stg@${project}.iam.gserviceaccount.com"
accounting_sa="pymes-v3-accounting-stg@${project}.iam.gserviceaccount.com"
accounting_admin_sa="pymes-v3-accounting-admin-stg@${project}.iam.gserviceaccount.com"
fiscal_service="$prefix-fiscal"
accounting_service="$prefix-accounting"
accounting_admin_service="$prefix-accounting-admin"
release_services=(
  "$fiscal_service"
  "$accounting_service"
  "$accounting_admin_service"
  "$prefix-worker"
  "$prefix-api"
  "$prefix-web"
)

reset_runtime() {
  unset previous_revisions previous_active_tags previous_all_tags
  unset previous_tag_urls candidate_revisions candidate_urls
  unset candidate_deploy_started service_existed promoted_services
  unset release_secret_files
  declare -gA previous_revisions=()
  declare -gA previous_active_tags=()
  declare -gA previous_all_tags=()
  declare -gA previous_tag_urls=()
  declare -gA candidate_revisions=()
  declare -gA candidate_urls=()
  declare -gA candidate_deploy_started=()
  declare -gA service_existed=()
  declare -ga promoted_services=()
  declare -ga release_secret_files=()
  promotion_started=false
  release_complete=false
  worker_scaling_paused=false
  previous_web_api_tag_mapping=
  previous_web_api_url=
  previous_web_api_token=
}

new_state() {
  local fixture="$1" scenario="$2"
  local state="$scratch_dir/${scenario}.json"
  cp "$fixture_dir/${fixture}.json" "$state"
  printf '%s' "$state"
}

set_fault() {
  local state="$1" match="$2" nth="$3" mode="$4"
  FAKE_GCLOUD_STATE="$state" \
    gcloud __fake__ set-fault "$match" "$nth" "$mode"
}

clear_fault() {
  local state="$1"
  FAKE_GCLOUD_STATE="$state" gcloud __fake__ clear-fault
}

set_worker_signal() {
  local state="$1" service="$2" revision="$3"
  FAKE_GCLOUD_STATE="$state" \
    gcloud __fake__ set-worker-signal "$service" "$revision"
}

capture_all() {
  local service
  for service in "${release_services[@]}"; do
    capture_previous_revision "$service"
  done
  validate_release_baseline
}

deploy_all_candidates() {
  deploy "$fiscal_service" "test.invalid/fiscal@sha256:new" "$fiscal_sa" \
    "" "PORT=8080" internal 0 private throttled direct
  deploy "$accounting_service" "test.invalid/accounting@sha256:new" \
    "$accounting_sa" "" "PORT=8080" internal 0 private throttled none
  deploy "$accounting_admin_service" \
    "test.invalid/accounting-admin@sha256:new" "$accounting_admin_sa" \
    "" "PORT=8080" internal 0 private throttled none
  deploy "$prefix-api" "test.invalid/api@sha256:new" "$api_sa" \
    "" "PORT=8080" all 0 public throttled direct
  deploy_web "$prefix-web" "test.invalid/web@sha256:new" "$web_sa" \
    "https://${candidate_tag}---${prefix}-api.${region}.run.fake"
  gcloud run services update "$prefix-worker" \
    --project="$project" --region="$region" --scaling=0 --quiet
  assert_worker_manual_scaling 0
  worker_scaling_paused=true
  deploy "$prefix-worker" "test.invalid/worker@sha256:new" "$worker_sa" \
    "" "PORT=8080" internal 0 private always direct disabled manual-zero
}

promote_all() {
  local service
  for service in \
    "$fiscal_service" \
    "$accounting_service" \
    "$accounting_admin_service" \
    "$prefix-api" \
    "$prefix-web" \
    "$prefix-worker"; do
    promote_service "$service"
  done
}

assert_state() {
  local expected="$1" state="$2"
  python3 "$fixture_dir/assert-state.py" "$expected" "$state"
}

assert_fault_triggered() {
  local state="$1"
  python3 - "$state" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    fault = json.load(stream).get("fault", {})
if int(fault.get("seen", 0)) < int(fault.get("nth", 1)):
    raise SystemExit(1)
PY
}

assert_old_api_tag_works() {
  local state="$1" status
  status=$(FAKE_GCLOUD_STATE="$state" curl --silent --show-error \
    --write-out='%{http_code}' \
    "https://c-1111111111111111---pymes-v3-stg-api.us-central1.run.fake" 2>/dev/null) ||
    return 1
  [[ "$status" == "200" ]]
}

assert_candidate_api_tag_is_retired() {
  local state="$1" status
  status=$(FAKE_GCLOUD_STATE="$state" curl --silent \
    --write-out='%{http_code}' \
    "https://${candidate_tag}---pymes-v3-stg-api.us-central1.run.fake" \
    2>/dev/null) || true
  [[ "$status" == "404" ]]
}

check_rollback_result() {
  local scenario="$1" expected="$2" state="$3" output="$4"
  local state_error="$scratch_dir/${scenario}.state.err"
  if assert_state "$expected" "$state" 2>"$state_error"; then
    if ! assert_candidate_api_tag_is_retired "$state"; then
      record_failure "$scenario left the candidate API URL reachable"
      return
    fi
    if [[ "$expected" == "established" ]] &&
      ! assert_old_api_tag_works "$state"; then
      record_failure "$scenario did not restore the previous API tag data plane"
      return
    fi
    if [[ "$expected" == "established" ]]; then
      record_pass "$scenario restored the exact established baseline"
    else
      record_pass "$scenario converged to the strict fail-closed inert state"
    fi
    return
  fi
  if grep -Fq "ROLLBACK COMPLETE" "$output"; then
    record_failure "$scenario printed ROLLBACK COMPLETE without convergence: $(tr '\n' ';' <"$state_error")"
  else
    record_failure "$scenario did not converge: $(tr '\n' ';' <"$state_error")"
  fi
}

test_list_error_is_not_absence() {
  local state output status
  state=$(new_state established list-error)
  output="$scratch_dir/list-error.out"
  set_fault "$state" "run services list" 1 before
  (
    set -Eeuo pipefail
    export FAKE_GCLOUD_STATE="$state"
    reset_runtime
    capture_previous_revision "$prefix-api"
  ) >"$output" 2>&1
  status=$?
  if (( status == 0 )); then
    record_failure "list error was interpreted as an absent first-deploy service"
  else
    record_pass "list error fails before mutation"
  fi
  if ! assert_state established "$state"; then
    record_failure "list error mutated the established baseline"
  fi
}

test_describe_error_is_not_absence() {
  local state output status
  state=$(new_state established describe-error)
  output="$scratch_dir/describe-error.out"
  set_fault "$state" "run services describe $prefix-api" 1 before
  (
    set -Eeuo pipefail
    export FAKE_GCLOUD_STATE="$state"
    reset_runtime
    capture_previous_revision "$prefix-api"
  ) >"$output" 2>&1
  status=$?
  if (( status == 0 )); then
    record_failure "describe error was interpreted as an absent first-deploy service"
  else
    record_pass "describe error fails before mutation"
  fi
  if ! assert_state established "$state"; then
    record_failure "describe error mutated the established baseline"
  fi
}

test_deploy_lost_response_cleanup() {
  local state output status
  state=$(new_state established deploy-lost-response)
  output="$scratch_dir/deploy-lost-response.out"
  (
    set -Eeuo pipefail
    export FAKE_GCLOUD_STATE="$state"
    reset_runtime
    capture_all
    set_fault "$state" "run services describe $prefix-api" 2 before
    trap rollback_on_exit EXIT
    deploy "$prefix-api" "test.invalid/api@sha256:new" "$api_sa" \
      "" "PORT=8080" all 0 public throttled direct
  ) >"$output" 2>&1
  status=$?
  if (( status == 0 )); then
    record_failure "deploy lost-response scenario unexpectedly succeeded"
  fi
  if ! assert_fault_triggered "$state"; then
    record_failure "deploy lost-response injection was not reached"
  fi
  check_rollback_result deploy-lost-response established "$state" "$output"
}

test_iam_binding_failure_is_pre_mutation() {
  local state output principal status
  state=$(new_state established iam-policy-failure)
  output="$scratch_dir/iam-policy-failure.out"
  principal="serviceAccount:$api_sa"
  set_fault "$state" \
    "run services add-iam-policy-binding $fiscal_service" 1 before
  (
    set -Eeuo pipefail
    export FAKE_GCLOUD_STATE="$state"
    reset_runtime
    ensure_service_invoker "$fiscal_service" "$principal"
  ) >"$output" 2>&1
  status=$?
  if (( status == 0 )); then
    record_failure "IAM binding failure was hidden"
  else
    record_pass "IAM binding failure aborts before mutation"
  fi
  if ! assert_state established "$state"; then
    record_failure "IAM binding failure mutated the baseline"
  fi
}

test_iam_set_policy_failure_never_claims_complete() {
  local state output status
  state=$(new_state inert iam-set-policy-failure)
  output="$scratch_dir/iam-set-policy-failure.out"
  (
    set -Eeuo pipefail
    export FAKE_GCLOUD_STATE="$state"
    reset_runtime
    capture_all
    trap rollback_on_exit EXIT
    deploy "$prefix-api" "test.invalid/api@sha256:new" "$api_sa" \
      "" "PORT=8080" all 0 public throttled direct
    promote_service "$prefix-api"
    set_fault "$state" \
      "run services set-iam-policy $prefix-api" 1 always-before
    false
  ) >"$output" 2>&1
  status=$?
  if (( status == 0 )); then
    record_failure "persistent IAM set-policy failure scenario unexpectedly succeeded"
  fi
  if ! assert_fault_triggered "$state"; then
    record_failure "persistent IAM set-policy failure injection was not reached"
  fi
  if grep -Fq "ROLLBACK COMPLETE" "$output"; then
    record_failure "persistent IAM set-policy failure falsely claimed ROLLBACK COMPLETE"
  elif ! grep -Fq "ROLLBACK INCOMPLETE" "$output"; then
    record_failure "persistent IAM set-policy failure omitted the incomplete rollback signal"
  elif ! assert_state iam-incomplete "$state"; then
    record_failure "persistent IAM set-policy fault left an unexpected state"
  elif ! assert_candidate_api_tag_is_retired "$state"; then
    record_failure "persistent IAM set-policy fault left the candidate API URL reachable"
  else
    record_pass "persistent IAM set-policy failure is fail-loud and never claims convergence"
  fi
}

test_promotion_failure_rollback() {
  local state output status
  state=$(new_state established promotion-failure)
  output="$scratch_dir/promotion-failure.out"
  (
    set -Eeuo pipefail
    export FAKE_GCLOUD_STATE="$state"
    reset_runtime
    capture_all
    trap rollback_on_exit EXIT
    deploy_all_candidates
    set_fault "$state" \
      "run services update-traffic $prefix-api" 1 after
    promote_all
  ) >"$output" 2>&1
  status=$?
  if (( status == 0 )); then
    record_failure "promotion failure scenario unexpectedly succeeded"
  fi
  if ! assert_fault_triggered "$state"; then
    record_failure "promotion failure injection was not reached"
  fi
  check_rollback_result promotion-failure established "$state" "$output"
}

test_settle_failure_rollback() {
  local state output status
  state=$(new_state established settle-failure)
  output="$scratch_dir/settle-failure.out"
  (
    set -Eeuo pipefail
    export FAKE_GCLOUD_STATE="$state"
    reset_runtime
    capture_all
    trap rollback_on_exit EXIT
    deploy_all_candidates
    set_worker_signal "$state" "$prefix-worker" \
      "${candidate_revisions[$prefix-worker]}"
    promote_all
    set_fault "$state" \
      "run services update-traffic $prefix-web" 1 after
    settle_release_tags
  ) >"$output" 2>&1
  status=$?
  if (( status == 0 )); then
    record_failure "settle failure scenario unexpectedly succeeded"
  fi
  if ! assert_fault_triggered "$state"; then
    record_failure "settle failure injection was not reached"
  fi
  check_rollback_result settle-failure established "$state" "$output"
}

test_inert_worker_signal_failure() {
  local state output calls_before calls_after status
  state=$(new_state inert inert-worker-signal)
  output="$scratch_dir/inert-worker-signal.out"
  calls_before=$(wc -l <"$FAKE_GCLOUD_CALL_LOG")
  (
    set -Eeuo pipefail
    export FAKE_GCLOUD_STATE="$state"
    reset_runtime
    capture_all
    trap rollback_on_exit EXIT
    deploy_all_candidates
    promote_all
  ) >"$output" 2>&1
  status=$?
  if (( status == 0 )); then
    record_failure "missing worker signal did not fail the release"
  fi
  calls_after=$(wc -l <"$FAKE_GCLOUD_CALL_LOG")
  if (( calls_after <= calls_before )) ||
    ! tail -n "$((calls_after - calls_before))" "$FAKE_GCLOUD_CALL_LOG" |
      grep -Fq '"logging", "read"'; then
    record_failure "missing worker signal scenario never queried the release signal"
  fi
  check_rollback_result inert-worker-signal inert "$state" "$output"
}

assert_capability_not_in_argv() {
  local log
  for log in "$FAKE_GCLOUD_CALL_LOG" "$FAKE_CURL_CALL_LOG"; do
    if grep -Fq "$PYMES_PREFLIGHT_TOKEN" "$log"; then
      record_failure "release capability leaked into argv log $(basename "$log")"
      return
    fi
  done
  record_pass "release capability never appears in gcloud or curl argv"
}

assert_secret_files_removed() {
  if ! python3 - "$FAKE_GCLOUD_CALL_LOG" "$FAKE_CURL_CALL_LOG" <<'PY'
import json
import pathlib
import sys

paths = []
for log_name, option in (
    (sys.argv[1], "--env-vars-file"),
    (sys.argv[2], "--config"),
):
    with open(log_name, encoding="utf-8") as stream:
        for raw_line in stream:
            arguments = json.loads(raw_line)
            for index, argument in enumerate(arguments):
                if argument.startswith(f"{option}="):
                    paths.append(pathlib.Path(argument.split("=", 1)[1]))
                elif argument == option and index + 1 < len(arguments):
                    paths.append(pathlib.Path(arguments[index + 1]))
remaining = [str(path) for path in paths if path.exists()]
if remaining:
    print("\n".join(remaining), file=sys.stderr)
    raise SystemExit(1)
if not paths:
    print("no sensitive transport files were exercised", file=sys.stderr)
    raise SystemExit(1)
PY
  then
    record_failure "one or more sensitive transport files survived the transaction"
    return
  fi
  record_pass "all temporary environment and curl capability files were removed"
}

test_list_error_is_not_absence
test_describe_error_is_not_absence
test_deploy_lost_response_cleanup
test_iam_binding_failure_is_pre_mutation
test_iam_set_policy_failure_never_claims_complete
test_promotion_failure_rollback
test_settle_failure_rollback
test_inert_worker_signal_failure
assert_capability_not_in_argv
assert_secret_files_removed

if (( failures > 0 )); then
  echo "cloud-run transaction tests failed: $failures" >&2
  exit 1
fi

echo "PASS cloud-run transaction stateful fault matrix"
