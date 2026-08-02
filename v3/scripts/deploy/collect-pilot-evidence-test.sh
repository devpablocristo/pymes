#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077
trap 'echo "pilot evidence test failed at line $LINENO" >&2' ERR

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
subject="$script_dir/collect-pilot-evidence.sh"
fixture_dir="$script_dir/testdata/pilot-evidence"
repo_root=$(cd -- "$script_dir/../../.." && pwd)
scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
mkdir -p "$scratch/bin"
cp -- "$fixture_dir/gcloud" "$scratch/bin/gcloud"
cp -- "$fixture_dir/curl" "$scratch/bin/curl"
chmod 700 "$scratch/bin/gcloud" "$scratch/bin/curl"

source_sha=$(git -C "$repo_root" rev-parse HEAD)
manifest="$scratch/release.env"
call_log="$scratch/calls.log"
public_base=https://pymes-stg.pilot.example
digest_a=$(printf 'a%.0s' {1..64})
digest_b=$(printf 'b%.0s' {1..64})
digest_c=$(printf 'c%.0s' {1..64})
digest_d=$(printf 'd%.0s' {1..64})
digest_e=$(printf 'e%.0s' {1..64})
digest_f=$(printf 'f%.0s' {1..64})

{
  printf 'PYMES_RELEASE_ENV=stg\n'
  printf 'PYMES_SOURCE_SHA=%s\n' "$source_sha"
  printf 'PYMES_FISCAL_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-fiscal@sha256:%s\n' "$digest_a"
  printf 'PYMES_ACCOUNTING_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-accounting@sha256:%s\n' "$digest_b"
  printf 'PYMES_ACCOUNTING_ADMIN_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-accounting-admin@sha256:%s\n' "$digest_c"
  printf 'PYMES_WORKER_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-worker@sha256:%s\n' "$digest_d"
  printf 'PYMES_API_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-api@sha256:%s\n' "$digest_e"
  printf 'PYMES_WEB_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-web-stg@sha256:%s\n' "$digest_f"
} >"$manifest"
chmod 600 "$manifest"
manifest_sha=$(sha256sum "$manifest" | awk '{print $1}')

api_token_a='test-api-token-tenant-a'
api_token_b='test-api-token-tenant-b'
api_token='unit-test-placeholder00'
google_token='ya29.google-provider-token-test'
printf '%s\n' "$api_token_a" >"$scratch/api-a.token"
printf '%s\n' "$api_token_b" >"$scratch/api-b.token"
printf '%s\n' "$api_token" >"$scratch/api.token"
printf '%s\n' "$google_token" >"$scratch/google.token"
chmod 600 "$scratch"/*.token

google_event_id=$(python3 - \
  org-google \
  33333333-3333-4333-8333-333333333333 \
  55555555-5555-4555-8555-555555555555 <<'PY'
import base64
import hashlib
import sys

digest = hashlib.sha256()
for part in ("event", *sys.argv[1:]):
    digest.update(b"\0")
    digest.update(part.encode())
print(base64.b32hexencode(digest.digest()).decode().rstrip("=").lower())
PY
)

common=(
  "PATH=$scratch/bin:$PATH"
  "FAKE_CALL_LOG=$call_log"
  "FAKE_SOURCE_SHA=$source_sha"
  "FAKE_RELEASE_MANIFEST=$manifest"
  "FAKE_PUBLIC_BASE_URL=$public_base"
  "FAKE_API_TOKEN_A=$api_token_a"
  "FAKE_API_TOKEN_B=$api_token_b"
  "FAKE_API_TOKEN=$api_token"
  "FAKE_GOOGLE_TOKEN=$google_token"
  "FAKE_GOOGLE_EVENT_ID=$google_event_id"
  "PYMES_PILOT_ENV=stg"
  "PYMES_PILOT_SOURCE_SHA=$source_sha"
  "PYMES_PILOT_PUBLIC_BASE_URL=$public_base"
  "PYMES_PILOT_RELEASE_MANIFEST=$manifest"
  "PYMES_PILOT_RELEASE_MANIFEST_SHA256=$manifest_sha"
)

fail() {
  echo "pilot evidence test failed: $*" >&2
  exit 1
}

expect_failure() {
  local description="$1"
  shift
  if "$@" >"$scratch/failure.out" 2>&1; then
    fail "$description"
  fi
}

assert_bundle() {
  local bundle="$1" kind="$2"
  [[ -d "$bundle" && ! -L "$bundle" ]] ||
    fail "$kind bundle is absent"
  [[ "$(stat -c '%a' "$bundle")" == "700" ]] ||
    fail "$kind bundle mode is not 0700"
  (
    cd "$bundle"
    sha256sum --check checksums.sha256 >/dev/null
  ) || fail "$kind bundle checksums do not verify"
  jq -e \
    --arg kind "$kind" \
    --arg sha "$source_sha" \
    --arg manifest_sha "$manifest_sha" '
    .schema_version == "pymes.pilot-evidence.v1" and
    .result == "passed" and .pilot_kind == $kind and
    .source_sha == $sha and
    .release_manifest_sha256 == $manifest_sha and
    (.cloud_run | length == 6) and
    ([.cloud_run[].traffic_percent] | all(. == 100)) and
    .redaction.raw_http_responses_retained == false and
    .redaction.bearer_tokens_retained == false
  ' "$bundle/manifest.json" >/dev/null ||
    fail "$kind manifest is incomplete"
  jq -e --arg kind "$kind" \
    '.kind == $kind and .result == "passed"' \
    "$bundle/pilot.json" >/dev/null ||
    fail "$kind pilot record is incomplete"
  if grep -Ra -F \
    -e "$api_token_a" \
    -e "$api_token_b" \
    -e "$api_token" \
    -e "$google_token" \
    -e 'Alice Secret' \
    -e 'Bob Secret' \
    -e 'customer@example.invalid' \
    -e 'Secret Company SA' \
    -e '20123456789' \
    -e 'provider-secret-message-id' \
    -e 'customer-secret' \
    "$bundle" >/dev/null; then
    fail "$kind bundle retained sensitive source material"
  fi
}

run_agenda() {
  local bundle="$scratch/agenda-evidence"
  env "${common[@]}" \
    PYMES_PILOT_KIND=agenda \
    "PYMES_PILOT_CONFIRMATION=COLLECT_STG_AGENDA_${source_sha}" \
    "PYMES_PILOT_EVIDENCE_DIR=$bundle" \
    PYMES_PILOT_ORGANIZATION_A=org-a \
    PYMES_PILOT_ORGANIZATION_B=org-b \
    PYMES_PILOT_BOOKING_A=11111111-1111-4111-8111-111111111111 \
    PYMES_PILOT_BOOKING_B=22222222-2222-4222-8222-222222222222 \
    "PYMES_PILOT_BEARER_TOKEN_A_FILE=$scratch/api-a.token" \
    "PYMES_PILOT_BEARER_TOKEN_B_FILE=$scratch/api-b.token" \
    "$subject" >/dev/null
  assert_bundle "$bundle" agenda
  jq -e '
    (.tenants | length == 2) and
    .isolation.tenant_a_cannot_read_tenant_b_booking == true and
    .isolation.tenant_b_cannot_read_tenant_a_booking == true
  ' "$bundle/pilot.json" >/dev/null ||
    fail "Agenda evidence does not prove two-tenant isolation"
}

run_pergo() {
  local bundle="$scratch/pergo-evidence"
  env "${common[@]}" \
    PYMES_PILOT_KIND=pergo \
    "PYMES_PILOT_CONFIRMATION=COLLECT_STG_PERGO_${source_sha}" \
    "PYMES_PILOT_EVIDENCE_DIR=$bundle" \
    PYMES_PILOT_ORGANIZATION_ID=org-pergo \
    PYMES_PILOT_NOTIFICATION_ID=notification-pilot \
    "PYMES_PILOT_BEARER_TOKEN_FILE=$scratch/api.token" \
    "$subject" >/dev/null
  assert_bundle "$bundle" pergo
  jq -e '
    .runtime.channel == "whatsapp_cloud" and
    .runtime.tenant_route_fallback == false and
    .runtime.serverless_identity_audience_configured == true and
    (.runtime.audience_ref | startswith("sha256:")) and
    .runtime.audience_ref != .runtime.endpoint_ref and
    .notification.status == "delivered"
  ' "$bundle/pilot.json" >/dev/null ||
    fail "PerGo evidence does not prove private provider delivery identity"
}

expect_pergo_audience_failure() {
  local audience="$1" label="$2"
  local bundle="$scratch/refused-pergo-audience-$label"
  expect_failure "PerGo accepted an unsafe $label workload audience" \
    env "${common[@]}" \
      "FAKE_PERGO_AUDIENCE=$audience" \
      PYMES_PILOT_KIND=pergo \
      "PYMES_PILOT_CONFIRMATION=COLLECT_STG_PERGO_${source_sha}" \
      "PYMES_PILOT_EVIDENCE_DIR=$bundle" \
      PYMES_PILOT_ORGANIZATION_ID=org-pergo \
      PYMES_PILOT_NOTIFICATION_ID=notification-pilot \
      "PYMES_PILOT_BEARER_TOKEN_FILE=$scratch/api.token" \
      "$subject"
  [[ ! -e "$bundle" ]] ||
    fail "unsafe $label PerGo audience produced a bundle"
}

run_google() {
  local bundle="$scratch/google-evidence"
  env "${common[@]}" \
    PYMES_PILOT_KIND=google \
    "PYMES_PILOT_CONFIRMATION=COLLECT_STG_GOOGLE_${source_sha}" \
    "PYMES_PILOT_PROVIDER_CONFIRMATION=READ_GOOGLE_STG_${source_sha}" \
    "PYMES_PILOT_EVIDENCE_DIR=$bundle" \
    PYMES_PILOT_ORGANIZATION_ID=org-google \
    PYMES_PILOT_GOOGLE_CONNECTION_ID=33333333-3333-4333-8333-333333333333 \
    PYMES_PILOT_BOOKING_ID=55555555-5555-4555-8555-555555555555 \
    PYMES_PILOT_GOOGLE_CALENDAR_ID=controlled-pilot@example.invalid \
    "PYMES_PILOT_BEARER_TOKEN_FILE=$scratch/api.token" \
    "PYMES_PILOT_GOOGLE_TOKEN_FILE=$scratch/google.token" \
    "$subject" >/dev/null
  assert_bundle "$bundle" google
  jq -e '
    .connection.status == "active" and
    .provider_event.meet_solution == "hangoutsMeet" and
    .provider_event.video_entry_points == 1
  ' "$bundle/pilot.json" >/dev/null ||
    fail "Google evidence does not prove the Calendar/Meet projection"
}

run_arca() {
  local bundle="$scratch/arca-evidence"
  env "${common[@]}" \
    PYMES_PILOT_KIND=arca \
    "PYMES_PILOT_CONFIRMATION=COLLECT_STG_ARCA_${source_sha}" \
    "PYMES_PILOT_EVIDENCE_DIR=$bundle" \
    PYMES_PILOT_ORGANIZATION_ID=org-arca \
    PYMES_PILOT_FISCAL_CREDENTIAL_ID=fcred_Z9y8X7w6V5u4T3s2R1q0P9o8 \
    PYMES_PILOT_SALE_ID=sale-pilot \
    "PYMES_PILOT_BEARER_TOKEN_FILE=$scratch/api.token" \
    "$subject" >/dev/null
  assert_bundle "$bundle" arca
  jq -e '
    .runtime.adapter_mode == "arca" and
    .credential.environment == "homologation" and
    .sale.status == "posted" and
    .evidence_boundary.exact_consultation_history_exposed_by_bff == false
  ' "$bundle/pilot.json" >/dev/null ||
    fail "ARCA evidence overclaims or lacks the safe terminal projection"
}

: >"$call_log"
expect_failure "missing confirmation reached a live adapter" \
  env "${common[@]}" \
    PYMES_PILOT_KIND=pergo \
    PYMES_PILOT_CONFIRMATION=WRONG \
    "PYMES_PILOT_EVIDENCE_DIR=$scratch/refused-confirmation" \
    PYMES_PILOT_ORGANIZATION_ID=org-pergo \
    PYMES_PILOT_NOTIFICATION_ID=notification-pilot \
    "PYMES_PILOT_BEARER_TOKEN_FILE=$scratch/api.token" \
    "$subject"
[[ ! -s "$call_log" && ! -e "$scratch/refused-confirmation" ]] ||
  fail "missing confirmation contacted an adapter or published evidence"

: >"$call_log"
expect_failure "manifest checksum mismatch reached a live adapter" \
  env "${common[@]}" \
    PYMES_PILOT_RELEASE_MANIFEST_SHA256="$(printf '0%.0s' {1..64})" \
    PYMES_PILOT_KIND=pergo \
    "PYMES_PILOT_CONFIRMATION=COLLECT_STG_PERGO_${source_sha}" \
    "PYMES_PILOT_EVIDENCE_DIR=$scratch/refused-manifest" \
    PYMES_PILOT_ORGANIZATION_ID=org-pergo \
    PYMES_PILOT_NOTIFICATION_ID=notification-pilot \
    "PYMES_PILOT_BEARER_TOKEN_FILE=$scratch/api.token" \
    "$subject"
[[ ! -s "$call_log" && ! -e "$scratch/refused-manifest" ]] ||
  fail "bad manifest contacted an adapter or published evidence"

: >"$call_log"
chmod 644 "$scratch/api.token"
expect_failure "insecure token mode reached a live adapter" \
  env "${common[@]}" \
    PYMES_PILOT_KIND=pergo \
    "PYMES_PILOT_CONFIRMATION=COLLECT_STG_PERGO_${source_sha}" \
    "PYMES_PILOT_EVIDENCE_DIR=$scratch/refused-token" \
    PYMES_PILOT_ORGANIZATION_ID=org-pergo \
    PYMES_PILOT_NOTIFICATION_ID=notification-pilot \
    "PYMES_PILOT_BEARER_TOKEN_FILE=$scratch/api.token" \
    "$subject"
chmod 600 "$scratch/api.token"
[[ ! -s "$call_log" && ! -e "$scratch/refused-token" ]] ||
  fail "insecure token contacted an adapter or published evidence"

: >"$call_log"
run_agenda
run_pergo
run_google
run_arca
expect_pergo_audience_failure "" missing
expect_pergo_audience_failure \
  "https://pergo-audience.pilot.example/tenant" path

expect_failure "a non-delivered PerGo result published evidence" \
  env "${common[@]}" \
    FAKE_NOTIFICATION_STATUS=sent \
    PYMES_PILOT_KIND=pergo \
    "PYMES_PILOT_CONFIRMATION=COLLECT_STG_PERGO_${source_sha}" \
    "PYMES_PILOT_EVIDENCE_DIR=$scratch/refused-pergo-state" \
    PYMES_PILOT_ORGANIZATION_ID=org-pergo \
    PYMES_PILOT_NOTIFICATION_ID=notification-pilot \
    "PYMES_PILOT_BEARER_TOKEN_FILE=$scratch/api.token" \
    "$subject"
[[ ! -e "$scratch/refused-pergo-state" ]] ||
  fail "non-delivered PerGo state produced a bundle"
if find "$scratch" -maxdepth 1 -name '.pymes-pilot-evidence.*' | grep -q .; then
  fail "a failed collection retained raw staging data"
fi

expect_failure "a mismatched active revision published evidence" \
  env "${common[@]}" \
    FAKE_REVISION_SHA=9999999999999999999999999999999999999999 \
    PYMES_PILOT_KIND=arca \
    "PYMES_PILOT_CONFIRMATION=COLLECT_STG_ARCA_${source_sha}" \
    "PYMES_PILOT_EVIDENCE_DIR=$scratch/refused-revision" \
    PYMES_PILOT_ORGANIZATION_ID=org-arca \
    PYMES_PILOT_FISCAL_CREDENTIAL_ID=fcred_Z9y8X7w6V5u4T3s2R1q0P9o8 \
    PYMES_PILOT_SALE_ID=sale-pilot \
    "PYMES_PILOT_BEARER_TOKEN_FILE=$scratch/api.token" \
    "$subject"
[[ ! -e "$scratch/refused-revision" ]] ||
  fail "mismatched revision produced a bundle"

printf '\n' >>"$scratch/agenda-evidence/pilot.json"
if (
  cd "$scratch/agenda-evidence"
  sha256sum --check checksums.sha256 >/dev/null 2>&1
); then
  fail "bundle tampering was not detected"
fi

echo "pilot evidence tests passed"
