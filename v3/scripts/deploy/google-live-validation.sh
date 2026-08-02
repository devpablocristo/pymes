#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077

api_origin=${PYMES_PUBLIC_BASE_URL:-}
api_token=${PYMES_LIVE_PILOT_API_TOKEN:-}
google_token=${PYMES_GOOGLE_PILOT_ACCESS_TOKEN:-}
calendar_id=${PYMES_GOOGLE_PILOT_CALENDAR_ID:-}
organization_id=${PYMES_PILOT_ORGANIZATION_ID:-}
connection_id=${PYMES_PILOT_CONNECTION_ID:-}
booking_id=${PYMES_PILOT_BOOKING_ID:-}
expected_meet=${PYMES_PILOT_EXPECT_MEET:-}
curl_bin=${PYMES_CURL_BIN:-curl}

uuid_pattern='^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
token_pattern='^[A-Za-z0-9._~+/=-]{20,8192}$'

fail() {
  echo "Google live validation failed: $*" >&2
  exit 1
}

for command in jq python3 stat mktemp; do
  command -v "$command" >/dev/null 2>&1 ||
    fail "$command is required"
done
if [[ "$curl_bin" == */* ]]; then
  [[ "$curl_bin" == /* && -x "$curl_bin" ]] ||
    fail "PYMES_CURL_BIN must be an executable absolute path"
else
  command -v "$curl_bin" >/dev/null 2>&1 ||
    fail "$curl_bin is required"
fi

[[ "$api_origin" =~ ^https://[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]] ||
  fail "PYMES_PUBLIC_BASE_URL must be an HTTPS origin without path or port"
[[ "$api_origin" != "https://localhost" &&
   "$api_origin" != https://127.* &&
   "$api_origin" != *.invalid ]] ||
  fail "PYMES_PUBLIC_BASE_URL must identify a deployed environment"
[[ "$organization_id" =~ ^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$ ]] ||
  fail "PYMES_PILOT_ORGANIZATION_ID is invalid"
[[ "$connection_id" =~ $uuid_pattern ]] ||
  fail "PYMES_PILOT_CONNECTION_ID must be a canonical lowercase UUID"
[[ "$booking_id" =~ $uuid_pattern ]] ||
  fail "PYMES_PILOT_BOOKING_ID must be a canonical lowercase UUID"
case "$expected_meet" in
  true|false) ;;
  *) fail "PYMES_PILOT_EXPECT_MEET must be true or false" ;;
esac
[[ "$api_token" =~ $token_pattern ]] ||
  fail "PYMES_LIVE_PILOT_API_TOKEN is missing or malformed"
[[ "$google_token" =~ $token_pattern ]] ||
  fail "PYMES_GOOGLE_PILOT_ACCESS_TOKEN is missing or malformed"
[[ -n "$calendar_id" && ${#calendar_id} -le 512 &&
   "$calendar_id" != *$'\n'* && "$calendar_id" != *$'\r'* &&
   "$calendar_id" != *'"'* && "$calendar_id" != *\\* ]] ||
  fail "PYMES_GOOGLE_PILOT_CALENDAR_ID is missing or malformed"

tmp_dir=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/pymes-google-live.XXXXXX")
cleanup() {
  rm -rf -- "$tmp_dir"
}
trap cleanup EXIT INT TERM

request_json() {
  local output_variable=$1 method=$2 url=$3 token=$4 payload=${5:-}
  local request_id=$6 expected_status=$7
  local config_file response_file payload_file status response

  config_file=$(mktemp "$tmp_dir/curl-config.XXXXXX")
  response_file=$(mktemp "$tmp_dir/response.XXXXXX")
  chmod 600 "$config_file" "$response_file"
  {
    printf '%s\n' \
      'silent' \
      'show-error' \
      'proto = "=https"' \
      'tlsv1.2' \
      'connect-timeout = 10' \
      'max-time = 45' \
      'max-filesize = 1048576' \
      "request = \"${method}\"" \
      "url = \"${url}\"" \
      'header = "Accept: application/json"' \
      'header = "Cache-Control: no-store"'
    printf 'header = "Authorization: Bearer %s"\n' "$token"
    printf 'header = "X-Request-ID: %s"\n' "$request_id"
    if [[ -n "$payload" ]]; then
      payload_file=$(mktemp "$tmp_dir/payload.XXXXXX")
      chmod 600 "$payload_file"
      printf '%s' "$payload" >"$payload_file"
      printf '%s\n' \
        'header = "Content-Type: application/json"' \
        "data-binary = \"@${payload_file}\""
    fi
  } >"$config_file"
  [[ "$(stat -c '%a' "$config_file")" == "600" ]] ||
    fail "curl credential file permissions are not 0600"

  if ! status=$(
    "$curl_bin" \
      --config "$config_file" \
      --output "$response_file" \
      --write-out '%{http_code}'
  ); then
    fail "HTTPS request transport failed"
  fi
  [[ "$status" == "$expected_status" ]] ||
    fail "HTTPS request returned status $status, expected $expected_status"
  [[ -s "$response_file" ]] ||
    fail "HTTPS request returned an empty body"
  response=$(<"$response_file")
  jq -e . >/dev/null <<<"$response" ||
    fail "HTTPS response is not valid JSON"
  printf -v "$output_variable" '%s' "$response"
}

request_prefix="pilot-google-${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-1}"
connections_json=
request_json \
  connections_json \
  GET \
  "${api_origin}/api/v1/organizations/${organization_id}/calendars/connections" \
  "$api_token" \
  "" \
  "${request_prefix}-connections" \
  200

jq -e \
  --arg connection_id "$connection_id" \
  --argjson expected_meet "$expected_meet" '
    type == "array" and
    ([.[] | select(.id == $connection_id)] | length) == 1 and
    (
      .[] | select(.id == $connection_id) |
      .provider == "google" and
      .status == "active" and
      .calendar_connected == true and
      .meet_enabled == $expected_meet and
      (.version | type == "number" and . >= 1)
    )
  ' >/dev/null <<<"$connections_json" ||
  fail "Pymes does not expose the exact active Google connection"

booking_json=
request_json \
  booking_json \
  GET \
  "${api_origin}/api/v1/organizations/${organization_id}/scheduling/bookings/${booking_id}" \
  "$api_token" \
  "" \
  "${request_prefix}-booking" \
  200

jq -e \
  --arg booking_id "$booking_id" '
    .id == $booking_id and
    (.status == "confirmed" or .status == "checked_in" or .status == "completed") and
    (.start_at | type == "string") and
    (.end_at | type == "string") and
    (.timezone | type == "string" and length > 0) and
    (.version | type == "number" and . >= 1)
  ' >/dev/null <<<"$booking_json" ||
  fail "Pymes booking is not an eligible synced booking"

event_id=$(
  python3 -c '
import base64
import hashlib
import sys

digest = hashlib.sha256()
for part in ("event", *sys.argv[1:]):
    digest.update(b"\0")
    digest.update(part.encode("utf-8"))
print(base64.b32hexencode(digest.digest()).decode("ascii").rstrip("=").lower())
' "$organization_id" "$connection_id" "$booking_id"
)
[[ "$event_id" =~ ^[0-9a-v]{52}$ ]] ||
  fail "deterministic Google event ID could not be derived"
encoded_calendar_id=$(jq -rn --arg value "$calendar_id" '$value | @uri')

event_json=
request_json \
  event_json \
  GET \
  "https://www.googleapis.com/calendar/v3/calendars/${encoded_calendar_id}/events/${event_id}?conferenceDataVersion=1&alwaysIncludeEmail=false" \
  "$google_token" \
  "" \
  "${request_prefix}-provider" \
  200

jq -e \
  --arg event_id "$event_id" '
    .id == $event_id and
    .status != "cancelled" and
    (.etag | type == "string" and length > 0) and
    .extendedProperties.private.pymes_managed == "true" and
    (
      .extendedProperties.private.pymes_snapshot_digest |
      type == "string" and test("^[0-9a-f]{64}$")
    ) and
    (.start.dateTime | type == "string") and
    (.end.dateTime | type == "string")
  ' >/dev/null <<<"$event_json" ||
  fail "Google event does not match the Pymes-managed projection contract"

booking_start=$(jq -er '.start_at' <<<"$booking_json")
booking_end=$(jq -er '.end_at' <<<"$booking_json")
event_start=$(jq -er '.start.dateTime' <<<"$event_json")
event_end=$(jq -er '.end.dateTime' <<<"$event_json")
python3 -c '
from datetime import datetime
import sys

def instant(value):
    return datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()

if instant(sys.argv[1]) != instant(sys.argv[2]) or instant(sys.argv[3]) != instant(sys.argv[4]):
    raise SystemExit(1)
' "$booking_start" "$event_start" "$booking_end" "$event_end" ||
  fail "Google event interval differs from the Pymes booking"

if [[ "$expected_meet" == "true" ]]; then
  jq -e '
    .conferenceData.conferenceSolution.key.type == "hangoutsMeet" and
    .conferenceData.createRequest.status.statusCode == "success" and
    any(
      .conferenceData.entryPoints[]?;
      .entryPointType == "video" and
      (.uri | type == "string" and test("^https://meet\\.google\\.com/[a-z-]+$"))
    )
  ' >/dev/null <<<"$event_json" ||
    fail "Google event does not contain the expected completed Meet conference"
else
  jq -e '
    ([.conferenceData.entryPoints[]? | select(.entryPointType == "video")] | length) == 0
  ' >/dev/null <<<"$event_json" ||
    fail "Google event contains an unexpected video conference"
fi

printf 'google-live-validation-ok organization=%s connection=%s booking=%s meet=%s\n' \
  "$organization_id" "$connection_id" "$booking_id" "$expected_meet"
