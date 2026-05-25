#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

E2E_MODE="${E2E_MODE:-host}"
case "$E2E_MODE" in
  host)
    BASE_URL="${E2E_BASE_URL:-${PYMES_BASE_URL_HOST:-http://localhost:8100}}"
    NEXUS_URL="${E2E_NEXUS_URL:-${GOVERNANCE_URL_HOST:-http://localhost:18084}}"
    COMPANION_URL="${E2E_COMPANION_URL:-${COMPANION_URL_HOST:-http://localhost:18085}}"
    ;;
  compose)
    BASE_URL="${E2E_BASE_URL:-${PYMES_BASE_URL_COMPOSE:-http://cp-backend:8080}}"
    NEXUS_URL="${E2E_NEXUS_URL:-${GOVERNANCE_URL_COMPOSE:-http://host.docker.internal:18084}}"
    COMPANION_URL="${E2E_COMPANION_URL:-${COMPANION_URL_COMPOSE:-http://host.docker.internal:18085}}"
    ;;
  ci)
    BASE_URL="${E2E_BASE_URL:-${PYMES_BASE_URL_CI:-http://localhost:8100}}"
    NEXUS_URL="${E2E_NEXUS_URL:-${GOVERNANCE_URL_CI:-http://localhost:18084}}"
    COMPANION_URL="${E2E_COMPANION_URL:-${COMPANION_URL_CI:-http://localhost:18085}}"
    ;;
  *)
    echo "E2E_MODE debe ser host, compose o ci" >&2
    exit 1
    ;;
esac

API_KEY="${API_KEY:-psk_local_admin}"
NEXUS_API_KEY="${GOVERNANCE_API_KEY:-nexus-admin-dev-key}"
COMPANION_JWT_SECRET="${COMPANION_INTERNAL_JWT_SECRET:-axis-dev-internal-jwt-secret-change-me}"
CALLBACK_TOKEN="${GOVERNANCE_CALLBACK_TOKEN:-local-nexus-callback-token}"
NEXUS_ORG_HEADER="${NEXUS_ORG_HEADER:-X-"Org"-ID}"
TENANT_ID="${TENANT_ID:-}"
if [[ -z "$TENANT_ID" ]]; then
  ensure_pymes_seed_db_ready
  TENANT_ID="$(resolve_target_tenant_uuid)"
fi

http_request() {
  local __body_var="$1"
  local __status_var="$2"
  local method="$3"
  local url="$4"
  shift 4
  local response body status
  response="$(curl -sS --max-time 10 -w $'\n%{http_code}' -X "$method" "$url" "$@")"
  status="${response##*$'\n'}"
  body="${response%$'\n'*}"
  printf -v "$__body_var" '%s' "$body"
  printf -v "$__status_var" '%s' "$status"
}

assert_status() {
  local name="$1"
  local got="$2"
  local want="$3"
  local body="${4:-}"
  if [[ "$got" == "$want" ]]; then
    printf "  %-62s PASS (%s)\n" "$name" "$got"
    return
  fi
  printf "  %-62s FAIL expected=%s got=%s\n" "$name" "$want" "$got" >&2
  [[ -n "$body" ]] && printf "    body: %.400s\n" "$body" >&2
  exit 1
}

jwt() {
  local org_id="$1"
  local surface="${2:-pymes}"
  python3 - "$COMPANION_JWT_SECRET" "$org_id" "$surface" <<'PY'
import base64, hashlib, hmac, json, sys, time
secret, org_id, surface = sys.argv[1:4]
now = int(time.time())
header = {"alg": "HS256", "typ": "JWT"}
claims = {
    "iss": "axis-bff",
    "aud": "companion",
    "sub": "pymes-e2e",
    "org_id": org_id,
    "actor_id": "pymes-e2e",
    "actor_type": "service",
    "role": "service",
    "scope": "companion:tasks:write companion:tasks:read companion:watchers:read companion:watchers:write",
    "service_principal": True,
    "product_surface": surface,
    "iat": now,
    "nbf": now - 30,
    "exp": now + 300,
}
def b64(obj):
    return base64.urlsafe_b64encode(json.dumps(obj, separators=(",", ":")).encode()).rstrip(b"=").decode()
signing = f"{b64(header)}.{b64(claims)}"
sig = base64.urlsafe_b64encode(hmac.new(secret.encode(), signing.encode(), hashlib.sha256).digest()).rstrip(b"=").decode()
print(f"{signing}.{sig}")
PY
}

callback_signature() {
  local timestamp="$1"
  local body="$2"
  python3 - "$CALLBACK_TOKEN" "$timestamp" "$body" <<'PY'
import hashlib, hmac, sys
token, timestamp, body = sys.argv[1:4]
print("sha256=" + hmac.new(token.encode(), (timestamp + "." + body).encode(), hashlib.sha256).hexdigest())
PY
}

stale_timestamp() {
  python3 - <<'PY'
from datetime import datetime, timedelta, timezone
print((datetime.now(timezone.utc) - timedelta(minutes=10)).isoformat(timespec="seconds").replace("+00:00", "Z"))
PY
}

ensure_action_type_fixture() {
  local action_body action_status
  http_request action_body action_status GET "$NEXUS_URL/v1/action-types?org_id=$TENANT_ID" -H "X-API-Key: $NEXUS_API_KEY"
  assert_status "Nexus action type fixture list" "$action_status" "200" "$action_body"
  if ACTION_TYPES_BODY="$action_body" python3 - e2e.notification.bulk_send <<'PY'
import json, os, sys
name = sys.argv[1]
items = json.loads(os.environ.get("ACTION_TYPES_BODY", "{}")).get("data", [])
raise SystemExit(0 if any(item.get("name") == name for item in items) else 1)
PY
  then
    printf "  %-62s PASS\n" "Nexus action type fixture present"
    return
  fi
  http_request action_body action_status POST "$NEXUS_URL/v1/action-types" \
    -H "X-API-Key: $NEXUS_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary '{"org_id":"'"$TENANT_ID"'","name":"e2e.notification.bulk_send","description":"Axis negative E2E fixture","risk_class":"medium"}'
  assert_status "Nexus action type fixture create" "$action_status" "201" "$action_body"
}

echo ""
echo "=== Axis/Pymes negative live contract E2E ==="
echo "  pymes:     $BASE_URL"
echo "  nexus:     $NEXUS_URL"
echo "  companion: $COMPANION_URL"
echo ""

http_request BODY STATUS GET "$BASE_URL/healthz"
assert_status "Pymes health" "$STATUS" "200" "$BODY"
http_request BODY STATUS GET "$NEXUS_URL/readyz"
assert_status "Nexus ready" "$STATUS" "200" "$BODY"
http_request BODY STATUS GET "$COMPANION_URL/readyz"
assert_status "Companion ready" "$STATUS" "200" "$BODY"

http_request BODY STATUS POST "$BASE_URL/v1/ai/chat" -H "Content-Type: application/json" --data-binary '{"message":"hola sin auth"}'
assert_status "Pymes Companion proxy rejects missing auth" "$STATUS" "401" "$BODY"

http_request BODY STATUS POST "$BASE_URL/v1/ai/chat" -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" --data-binary '{"message":""}'
assert_status "Pymes Companion proxy rejects malformed/empty chat" "$STATUS" "400" "$BODY"

http_request BODY STATUS GET "$BASE_URL/v1/ai/nope" -H "X-API-Key: $API_KEY"
assert_status "Pymes Companion proxy unknown route is 404" "$STATUS" "404" "$BODY"

token="$(jwt "$TENANT_ID" pymes)"
http_request BODY STATUS POST "$COMPANION_URL/v1/customer-messaging/inbound" -H "Content-Type: application/json" --data-binary '{"org_id":"'"$TENANT_ID"'","phone_number_id":"phone-e2e","from_phone":"5491112345678","message":"sin auth"}'
assert_status "Companion customer messaging rejects missing auth" "$STATUS" "401" "$BODY"

http_request BODY STATUS POST "$COMPANION_URL/v1/customer-messaging/inbound" -H "Authorization: Bearer invalid.jwt.token" -H "Content-Type: application/json" --data-binary '{"org_id":"'"$TENANT_ID"'","phone_number_id":"phone-e2e","from_phone":"5491112345678","message":"bad jwt"}'
assert_status "Companion customer messaging rejects invalid JWT" "$STATUS" "401" "$BODY"

wrong_surface="$(jwt "$TENANT_ID" other)"
http_request BODY STATUS POST "$COMPANION_URL/v1/customer-messaging/inbound" -H "Authorization: Bearer $wrong_surface" -H "Content-Type: application/json" --data-binary '{"org_id":"'"$TENANT_ID"'","phone_number_id":"phone-e2e","from_phone":"5491112345678","message":"wrong surface"}'
assert_status "Companion customer messaging rejects wrong product surface" "$STATUS" "403" "$BODY"

http_request BODY STATUS POST "$COMPANION_URL/v1/customer-messaging/inbound" -H "Authorization: Bearer $token" -H "Content-Type: application/json" --data-binary '{"org_id":"'"$TENANT_ID"'","phone_number_id":"phone-e2e","message":"missing from_phone"}'
assert_status "Companion customer messaging validates required fields" "$STATUS" "400" "$BODY"

http_request BODY STATUS POST "$COMPANION_URL/v1/customer-messaging/inbound" -H "Authorization: Bearer $token" -H "Content-Type: application/json" --data-binary '{"org_id":"'"$TENANT_ID"'","phone_number_id":"phone-e2e","from_phone":"5491112345678","message":"smoke inbound","message_id":"wamid-e2e-'"$(date +%s)"'"}'
assert_status "Companion customer messaging live smoke succeeds" "$STATUS" "200" "$BODY"

old_companion_inbound="/v1/"internal"/customer-messaging/inbound"
http_request BODY STATUS POST "$COMPANION_URL$old_companion_inbound" -H "Authorization: Bearer $token" -H "Content-Type: application/json" --data-binary '{}'
assert_status "Companion old internal customer messaging route is not exposed" "$STATUS" "404" "$BODY"

http_request BODY STATUS POST "$NEXUS_URL/v1/requests" -H "X-API-Key: $NEXUS_API_KEY" -H "Content-Type: application/json" --data-binary '{bad json'
assert_status "Nexus rejects malformed request JSON" "$STATUS" "400" "$BODY"

ensure_action_type_fixture

strict_binding='{"schema_version":"tool_intent.v1","org_id":"'"$TENANT_ID"'","actor_id":"pymes-e2e","actor_type":"service","product_surface":"pymes","run_id":"run-e2e","tool_invocation_id":"tool-e2e","connector_id":"pymes.e2e","capability_id":"e2e.notification.bulk_send","operation":"invoke","target_system":"pymes","target_resource":"e2e-target","payload_hash":"hash-e2e","idempotency_key":"idem-e2e"}'
http_request BODY STATUS POST "$NEXUS_URL/v1/requests/simulate" -H "X-API-Key: $NEXUS_API_KEY" -H "$NEXUS_ORG_HEADER: $TENANT_ID" -H "Content-Type: application/json" --data-binary '{"requester_type":"service","requester_id":"pymes-e2e","action_type":"e2e.notification.bulk_send","target_system":"pymes","target_resource":"e2e-target","action_binding":'"$strict_binding"',"params":{"action_binding":'"$strict_binding"'}}'
assert_status "Nexus accepts strict action_binding simulation" "$STATUS" "200" "$BODY"

bad_binding='{"schema_version":"tool_intent.v1","org_id":"other-org","actor_id":"pymes-e2e","actor_type":"service","product_surface":"pymes","run_id":"run-e2e","tool_invocation_id":"tool-e2e","connector_id":"pymes.e2e","capability_id":"e2e.notification.bulk_send","operation":"invoke","target_system":"pymes","target_resource":"e2e-target","payload_hash":"hash-e2e","idempotency_key":"idem-e2e"}'
http_request BODY STATUS POST "$NEXUS_URL/v1/requests/simulate" -H "X-API-Key: $NEXUS_API_KEY" -H "$NEXUS_ORG_HEADER: $TENANT_ID" -H "Content-Type: application/json" --data-binary '{"requester_type":"service","requester_id":"pymes-e2e","action_type":"e2e.notification.bulk_send","target_system":"pymes","target_resource":"e2e-target","action_binding":'"$bad_binding"',"params":{"action_binding":'"$bad_binding"'}}'
assert_status "Nexus rejects org-mismatched action_binding" "$STATUS" "400" "$BODY"

callback_body='{"event":"approval_pending","approval_id":"tamper-e2e"}'
http_request BODY STATUS POST "$BASE_URL/v1/internal/v1/governance-callback" -H "Content-Type: application/json" -H "X-Nexus-Callback-Timestamp: 2026-05-25T10:00:00Z" -H "X-Nexus-Callback-Signature: sha256=bad" --data-binary "$callback_body"
assert_status "Pymes rejects tampered Nexus callback signature" "$STATUS" "401" "$BODY"

stale_ts="$(stale_timestamp)"
stale_sig="$(callback_signature "$stale_ts" "$callback_body")"
http_request BODY STATUS POST "$BASE_URL/v1/internal/v1/governance-callback" -H "Content-Type: application/json" -H "X-Nexus-Callback-Timestamp: $stale_ts" -H "X-Nexus-Callback-Signature: $stale_sig" --data-binary "$callback_body"
assert_status "Pymes rejects stale Nexus callback replay" "$STATUS" "401" "$BODY"

idem="axis-negative-$(date +%s)"
request_body='{"requester_type":"service","requester_id":"pymes-e2e","action_type":"e2e.notification.bulk_send","target_system":"pymes","target_resource":"idem-target","params":{"org_id":"'"$TENANT_ID"'"}}'
http_request BODY STATUS POST "$NEXUS_URL/v1/requests" -H "X-API-Key: $NEXUS_API_KEY" -H "$NEXUS_ORG_HEADER: $TENANT_ID" -H "Idempotency-Key: $idem" -H "Content-Type: application/json" --data-binary "$request_body"
assert_status "Nexus idempotency first submit" "$STATUS" "201" "$BODY"
http_request BODY STATUS POST "$NEXUS_URL/v1/requests" -H "X-API-Key: $NEXUS_API_KEY" -H "$NEXUS_ORG_HEADER: $TENANT_ID" -H "Idempotency-Key: $idem" -H "Content-Type: application/json" --data-binary "$request_body"
assert_status "Nexus idempotency same payload replay" "$STATUS" "201" "$BODY"
http_request BODY STATUS POST "$NEXUS_URL/v1/requests" -H "X-API-Key: $NEXUS_API_KEY" -H "$NEXUS_ORG_HEADER: $TENANT_ID" -H "Idempotency-Key: $idem" -H "Content-Type: application/json" --data-binary '{"requester_type":"service","requester_id":"pymes-e2e","action_type":"e2e.notification.bulk_send","target_system":"pymes","target_resource":"different","params":{"org_id":"'"$TENANT_ID"'"}}'
assert_status "Nexus idempotency rejects changed payload" "$STATUS" "409" "$BODY"

echo ""
echo "=== Axis/Pymes negative live contract E2E passed ==="
