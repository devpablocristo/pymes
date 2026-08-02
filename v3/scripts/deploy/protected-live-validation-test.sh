#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
fake_curl="$script_dir/testdata/protected-live-validation/curl"
tmp_dir=$(mktemp -d)
cleanup() {
  rm -rf -- "$tmp_dir"
}
trap cleanup EXIT INT TERM

organization_id=org_pilot
connection_id=11111111-1111-4111-8111-111111111111
booking_id=22222222-2222-4222-8222-222222222222
credential_id=fcred_A1b2C3d4E5f6G7h8I9j0K1l2
event_id=l3bknianhvt8e6hqjbbh6ih1dqv75b95lpgsdoo7k3m7bkhqnhfg
api_token=unit-test-placeholder00
google_token=ya29.pilot-access-token-value
argv_log="$tmp_dir/curl-argv.log"
url_log="$tmp_dir/curl-url.log"

touch "$argv_log" "$url_log"
chmod 600 "$argv_log" "$url_log"

printf '%s\n' \
  '[{"id":"11111111-1111-4111-8111-111111111111","provider":"google","status":"active","calendar_connected":true,"time_zone":"America/Argentina/Buenos_Aires","free_busy_enabled":false,"meet_enabled":true,"version":1}]' \
  >"$tmp_dir/google-connections.json"
printf '%s\n' \
  '{"id":"22222222-2222-4222-8222-222222222222","status":"confirmed","start_at":"2026-08-03T13:00:00Z","end_at":"2026-08-03T14:00:00Z","timezone":"America/Argentina/Buenos_Aires","version":1}' \
  >"$tmp_dir/booking.json"
printf '%s\n' \
  '{"id":"l3bknianhvt8e6hqjbbh6ih1dqv75b95lpgsdoo7k3m7bkhqnhfg","status":"confirmed","etag":"\"v1\"","start":{"dateTime":"2026-08-03T10:00:00-03:00"},"end":{"dateTime":"2026-08-03T11:00:00-03:00"},"extendedProperties":{"private":{"pymes_managed":"true","pymes_snapshot_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},"conferenceData":{"conferenceSolution":{"key":{"type":"hangoutsMeet"}},"createRequest":{"status":{"statusCode":"success"}},"entryPoints":[{"entryPointType":"video","uri":"https://meet.google.com/abc-defg-hij"}]}}' \
  >"$tmp_dir/google-event.json"
printf '%s\n' \
  '{"id":"fcred_A1b2C3d4E5f6G7h8I9j0K1l2","organization_id":"org_pilot","environment":"homologation","status":"ready","certificate_fingerprint":"sha256:opaque","certificate_expires_at":"2099-01-01T00:00:00Z","version":2}' \
  >"$tmp_dir/arca-credential.json"
validated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '{"organization_id":"org_pilot","credential_id":"fcred_A1b2C3d4E5f6G7h8I9j0K1l2","environment":"homologation","number":7,"enabled":true,"validated_at":"%s"}\n' \
  "$validated_at" >"$tmp_dir/arca-point.json"

export PYMES_CURL_BIN="$fake_curl"
export PYMES_TEST_CURL_ARGV_LOG="$argv_log"
export PYMES_TEST_CURL_URL_LOG="$url_log"
export PYMES_TEST_GOOGLE_CONNECTIONS_FIXTURE="$tmp_dir/google-connections.json"
export PYMES_TEST_BOOKING_FIXTURE="$tmp_dir/booking.json"
export PYMES_TEST_GOOGLE_EVENT_FIXTURE="$tmp_dir/google-event.json"
export PYMES_TEST_ARCA_CREDENTIAL_FIXTURE="$tmp_dir/arca-credential.json"
export PYMES_TEST_ARCA_POINT_FIXTURE="$tmp_dir/arca-point.json"
export PYMES_PUBLIC_BASE_URL=https://stg.pymes.example.com
export PYMES_LIVE_PILOT_API_TOKEN="$api_token"
export GITHUB_RUN_ID=123
export GITHUB_RUN_ATTEMPT=1

google_output=$(
  PYMES_GOOGLE_PILOT_ACCESS_TOKEN="$google_token" \
  PYMES_GOOGLE_PILOT_CALENDAR_ID=pilot-calendar@example.com \
  PYMES_PILOT_ORGANIZATION_ID="$organization_id" \
  PYMES_PILOT_CONNECTION_ID="$connection_id" \
  PYMES_PILOT_BOOKING_ID="$booking_id" \
  PYMES_PILOT_EXPECT_MEET=true \
    "$script_dir/google-live-validation.sh"
)
[[ "$google_output" == \
  "google-live-validation-ok organization=$organization_id connection=$connection_id booking=$booking_id meet=true" ]]
[[ "$(grep -c 'www.googleapis.com/calendar/v3/calendars/' "$url_log")" -eq 1 ]]
if grep -Fq "$api_token" "$argv_log" || grep -Fq "$google_token" "$argv_log"; then
  echo "protected credential reached curl argv" >&2
  exit 1
fi

arca_output=$(
  PYMES_PILOT_ORGANIZATION_ID="$organization_id" \
  PYMES_PILOT_FISCAL_CREDENTIAL_ID="$credential_id" \
  PYMES_PILOT_FISCAL_POINT_OF_SALE=7 \
    "$script_dir/arca-homologation-validation.sh"
)
[[ "$arca_output" == \
  "arca-homologation-validation-ok organization=$organization_id credential=$credential_id point_of_sale=7 emitted=false" ]]
[[ "$(grep -c '/points-of-sale/7/validate' "$url_log")" -eq 1 ]]

expect_failure() {
  local description=$1
  shift
  local output
  if output=$("$@" 2>&1); then
    echo "expected failure: $description" >&2
    exit 1
  fi
  if [[ "$output" == *must-not-leak* ]]; then
    echo "failure response body leaked: $description" >&2
    exit 1
  fi
}

expect_failure \
  "Google rejects non-deployed origin before transport" \
  env \
    PYMES_PUBLIC_BASE_URL=https://pilot.invalid \
    PYMES_GOOGLE_PILOT_ACCESS_TOKEN="$google_token" \
    PYMES_GOOGLE_PILOT_CALENDAR_ID=pilot-calendar@example.com \
    PYMES_PILOT_ORGANIZATION_ID="$organization_id" \
    PYMES_PILOT_CONNECTION_ID="$connection_id" \
    PYMES_PILOT_BOOKING_ID="$booking_id" \
    PYMES_PILOT_EXPECT_MEET=true \
    "$script_dir/google-live-validation.sh"

expect_failure \
  "Google provider response fails closed without leaking its body" \
  env \
    PYMES_TEST_FAIL_URL_PATTERN=www.googleapis.com \
    PYMES_TEST_FAILURE_BODY='{"secret":"must-not-leak"}' \
    PYMES_GOOGLE_PILOT_ACCESS_TOKEN="$google_token" \
    PYMES_GOOGLE_PILOT_CALENDAR_ID=pilot-calendar@example.com \
    PYMES_PILOT_ORGANIZATION_ID="$organization_id" \
    PYMES_PILOT_CONNECTION_ID="$connection_id" \
    PYMES_PILOT_BOOKING_ID="$booking_id" \
    PYMES_PILOT_EXPECT_MEET=true \
    "$script_dir/google-live-validation.sh"

production_credential="$tmp_dir/arca-production-credential.json"
printf '%s\n' \
  '{"id":"fcred_A1b2C3d4E5f6G7h8I9j0K1l2","organization_id":"org_pilot","environment":"production","status":"ready","certificate_fingerprint":"sha256:opaque","certificate_expires_at":"2099-01-01T00:00:00Z","version":2}' \
  >"$production_credential"
urls_before=$(wc -l <"$url_log")
expect_failure \
  "ARCA production credential cannot reach validation endpoint" \
  env \
    PYMES_TEST_ARCA_CREDENTIAL_FIXTURE="$production_credential" \
    PYMES_PILOT_ORGANIZATION_ID="$organization_id" \
    PYMES_PILOT_FISCAL_CREDENTIAL_ID="$credential_id" \
    PYMES_PILOT_FISCAL_POINT_OF_SALE=7 \
    "$script_dir/arca-homologation-validation.sh"
urls_after=$(wc -l <"$url_log")
[[ "$urls_after" -eq $((urls_before + 1)) ]] || {
  echo "ARCA production credential reached point-of-sale validation" >&2
  exit 1
}

expect_failure \
  "ARCA provider response fails closed without leaking its body" \
  env \
    PYMES_TEST_FAIL_URL_PATTERN=/points-of-sale/ \
    PYMES_TEST_FAILURE_BODY='{"secret":"must-not-leak"}' \
    PYMES_PILOT_ORGANIZATION_ID="$organization_id" \
    PYMES_PILOT_FISCAL_CREDENTIAL_ID="$credential_id" \
    PYMES_PILOT_FISCAL_POINT_OF_SALE=7 \
    "$script_dir/arca-homologation-validation.sh"

[[ "$event_id" == \
  l3bknianhvt8e6hqjbbh6ih1dqv75b95lpgsdoo7k3m7bkhqnhfg ]]
echo "protected live validation tests passed"
