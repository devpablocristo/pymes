#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077

api_origin=${PYMES_PUBLIC_BASE_URL:-}
api_token=${PYMES_LIVE_PILOT_API_TOKEN:-}
organization_id=${PYMES_PILOT_ORGANIZATION_ID:-}
credential_id=${PYMES_PILOT_FISCAL_CREDENTIAL_ID:-}
point_of_sale=${PYMES_PILOT_FISCAL_POINT_OF_SALE:-}
curl_bin=${PYMES_CURL_BIN:-curl}

credential_id_pattern='^fcred_[A-Za-z0-9_-]{8,80}$'
token_pattern='^[A-Za-z0-9._~+/=-]{20,8192}$'

fail() {
  echo "ARCA homologation validation failed: $*" >&2
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
[[ "$credential_id" =~ $credential_id_pattern ]] ||
  fail "PYMES_PILOT_FISCAL_CREDENTIAL_ID must be an opaque fcred_ identifier"
[[ "$point_of_sale" =~ ^[1-9][0-9]{0,4}$ ]] ||
  fail "PYMES_PILOT_FISCAL_POINT_OF_SALE must be between 1 and 99999"
((10#$point_of_sale <= 99999)) ||
  fail "PYMES_PILOT_FISCAL_POINT_OF_SALE must be between 1 and 99999"
[[ "$api_token" =~ $token_pattern ]] ||
  fail "PYMES_LIVE_PILOT_API_TOKEN is missing or malformed"

tmp_dir=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/pymes-arca-live.XXXXXX")
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
      'max-time = 60' \
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

request_prefix="pilot-arca-${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-1}"
credential_json=
request_json \
  credential_json \
  GET \
  "${api_origin}/api/v1/organizations/${organization_id}/fiscal/credentials/${credential_id}" \
  "$api_token" \
  "" \
  "${request_prefix}-credential" \
  200

jq -e \
  --arg organization_id "$organization_id" \
  --arg credential_id "$credential_id" '
    .id == $credential_id and
    .organization_id == $organization_id and
    .environment == "homologation" and
    .status == "ready" and
    (.certificate_fingerprint | type == "string" and length > 0) and
    (.certificate_expires_at | type == "string" and length > 0) and
    (.version | type == "number" and . >= 1)
  ' >/dev/null <<<"$credential_json" ||
  fail "credential is not a ready homologation credential for the organization"

certificate_expires_at=$(jq -er '.certificate_expires_at' <<<"$credential_json")
python3 -c '
from datetime import datetime, timezone
import sys

expires = datetime.fromisoformat(sys.argv[1].replace("Z", "+00:00"))
if expires <= datetime.now(timezone.utc):
    raise SystemExit(1)
' "$certificate_expires_at" ||
  fail "homologation certificate is expired"

validation_started_at=$(python3 -c 'import time; print(int(time.time()))')
point_json=
request_json \
  point_json \
  POST \
  "${api_origin}/api/v1/organizations/${organization_id}/fiscal/credentials/${credential_id}/points-of-sale/${point_of_sale}/validate" \
  "$api_token" \
  '{"enabled":true}' \
  "${request_prefix}-validate" \
  200

jq -e \
  --arg organization_id "$organization_id" \
  --arg credential_id "$credential_id" \
  --argjson point_of_sale "$point_of_sale" '
    .organization_id == $organization_id and
    .credential_id == $credential_id and
    .environment == "homologation" and
    .number == $point_of_sale and
    .enabled == true and
    (.validated_at | type == "string" and length > 0)
  ' >/dev/null <<<"$point_json" ||
  fail "WSAA/WSFE validation result does not match the requested homologation point"

validated_at=$(jq -er '.validated_at' <<<"$point_json")
python3 -c '
from datetime import datetime
import sys
import time

validated = datetime.fromisoformat(sys.argv[1].replace("Z", "+00:00")).timestamp()
started = int(sys.argv[2])
now = time.time()
if validated < started - 300 or validated > now + 300:
    raise SystemExit(1)
' "$validated_at" "$validation_started_at" ||
  fail "homologation validation timestamp is outside the current run"

printf 'arca-homologation-validation-ok organization=%s credential=%s point_of_sale=%s emitted=false\n' \
  "$organization_id" "$credential_id" "$point_of_sale"
