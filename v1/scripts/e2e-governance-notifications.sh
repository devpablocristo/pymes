#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
E2E_BASE_URL="${BASE_URL:-}"
E2E_GOVERNANCE_URL="${E2E_GOVERNANCE_URL:-}"
E2E_API_KEY="${API_KEY:-}"
E2E_GOVERNANCE_API_KEY="${GOVERNANCE_API_KEY:-}"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

DECISION="${1:-approve}"
E2E_MODE="${E2E_MODE:-host}"
case "$E2E_MODE" in
  host)
    BASE_URL="${E2E_BASE_URL:-${PYMES_BASE_URL_HOST:-http://localhost:8100}}"
    GOVERNANCE_URL="${E2E_GOVERNANCE_URL:-${GOVERNANCE_URL_HOST:-http://localhost:18084}}"
    ;;
  compose)
    BASE_URL="${E2E_BASE_URL:-${PYMES_BASE_URL_COMPOSE:-http://cp-backend:8080}}"
    GOVERNANCE_URL="${E2E_GOVERNANCE_URL:-${GOVERNANCE_URL_COMPOSE:-http://host.docker.internal:18084}}"
    ;;
  ci)
    BASE_URL="${E2E_BASE_URL:-${PYMES_BASE_URL_CI:-http://localhost:8100}}"
    GOVERNANCE_URL="${E2E_GOVERNANCE_URL:-${GOVERNANCE_URL_CI:-http://localhost:18084}}"
    ;;
  *)
    echo "E2E_MODE debe ser host, compose o ci" >&2
    exit 1
    ;;
esac
API_KEY="${E2E_API_KEY:-${API_KEY:-psk_local_admin}}"
GOVERNANCE_API_KEY="${E2E_GOVERNANCE_API_KEY:-${GOVERNANCE_API_KEY:-nexus-admin-dev-key}}"
if [[ -z "${TENANT_ID:-}" ]]; then
  ensure_pymes_seed_db_ready
  TENANT_ID="$(resolve_target_tenant_uuid)"
fi
REQUESTER_ID="${REQUESTER_ID:-e2e-governance-tester}"
ACTION_TYPE="${ACTION_TYPE:-e2e.notification.bulk_send}"
POLICY_NAME="${POLICY_NAME:-e2e-require-approval-governance-notifications}"
CALLBACK_TIMEOUT_SECONDS="${CALLBACK_TIMEOUT_SECONDS:-12}"
POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-1}"
COMPOSE_CMD="${COMPOSE_CMD:-docker compose}"

case "$DECISION" in
  approve|reject) ;;
  *)
    echo "Uso: $0 [approve|reject]" >&2
    exit 1
    ;;
esac

red()   { printf "\033[31m%s\033[0m" "$1"; }
green() { printf "\033[32m%s\033[0m" "$1"; }
bold()  { printf "\033[1m%s\033[0m" "$1"; }

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Falta comando requerido: $cmd" >&2
    exit 1
  fi
}

http_request() {
  local __body_var="$1"
  local __status_var="$2"
  local method="$3"
  local url="$4"
  shift 4

  local response body status
  response="$(curl -sS -w $'\n%{http_code}' -X "$method" "$url" "$@")"
  status="${response##*$'\n'}"
  body="${response%$'\n'*}"

  printf -v "$__body_var" '%s' "$body"
  printf -v "$__status_var" '%s' "$status"
}

assert_status() {
  local name="$1"
  local status="$2"
  local expected="$3"
  local body="${4:-}"
  if [[ "$status" == "$expected" ]]; then
    printf "  %-56s %s (%s)\n" "$name" "$(green "PASS")" "$status"
    return
  fi
  printf "  %-56s %s expected=%s got=%s\n" "$name" "$(red "FAIL")" "$expected" "$status" >&2
  if [[ -n "$body" ]]; then
    printf "    body: %.300s\n" "$body" >&2
  fi
  exit 1
}

json_find_action_type_id() {
  local body="$1"
  local tenant_id="$2"
  printf '%s' "$body" | python3 -c '
import json, sys
target = sys.argv[1]
tenant_id = sys.argv[2]
data = json.load(sys.stdin).get("data", [])
for item in data:
    org_id = item.get("org_id") or ""
    if item.get("name") == target and org_id in ("", tenant_id):
        print(item.get("id", ""))
        break
' "$ACTION_TYPE" "$tenant_id"
}

json_find_delegation_id() {
  local body="$1"
  local tenant_id="$2"
  printf '%s' "$body" | python3 -c '
import json, sys
requester_id = sys.argv[1]
action_type = sys.argv[2]
tenant_id = sys.argv[3]
data = json.load(sys.stdin).get("data", [])
for item in data:
    org_id = item.get("org_id") or ""
    if item.get("agent_id") == requester_id and action_type in (item.get("allowed_action_types") or []) and org_id in ("", tenant_id):
        print(item.get("id", ""))
        break
' "$REQUESTER_ID" "$ACTION_TYPE" "$tenant_id"
}

json_find_policy_info() {
  local body="$1"
  local tenant_id="$2"
  printf '%s' "$body" | python3 -c '
import json, sys
name = sys.argv[1]
tenant_id = sys.argv[2]
data = json.load(sys.stdin).get("data", [])
for item in data:
    org_id = item.get("org_id") or ""
    if item.get("name") == name and org_id in ("", tenant_id):
        enabled = "true" if item.get("enabled") else "false"
        print("{}\t{}".format(item.get("id", ""), enabled))
        break
' "$POLICY_NAME" "$tenant_id"
}

json_get_submit_fields() {
  local body="$1"
  printf '%s' "$body" | python3 -c '
import json, sys
data = json.load(sys.stdin)
approval = data.get("approval") or {}
print("\t".join([
    data.get("request_id", ""),
    data.get("status", ""),
    approval.get("id", ""),
]))
'
}

json_has_pending_approval() {
  local body="$1"
  printf '%s' "$body" | python3 -c '
import json, sys
approval_id = sys.argv[1]
data = json.load(sys.stdin).get("data", [])
for item in data:
    if item.get("id") == approval_id:
        print("1")
        break
else:
    print("0")
' "$2"
}

db_query() {
  local sql="$1"
  if command -v psql >/dev/null 2>&1; then
    if host_pymes_psql -At -F '|' -c "$sql" 2>/dev/null; then
      return 0
    fi
  fi
  local -a compose_cmd_parts=()
  read -r -a compose_cmd_parts <<<"$COMPOSE_CMD"
  "${compose_cmd_parts[@]}" exec -T postgres psql -U postgres -d pymes -At -F '|' -c "$sql"
}

wait_for_db_notification_state() {
  local approval_id="$1"
  local expected="$2"
  # Usar epoch en lugar de SECONDS: en CI los subshells (docker compose exec) y el runtime del script
  # hacen poco fiable el contador interno de bash para ventanas largas.
  local start_ts deadline_ts now_ts
  start_ts=$(date +%s)
  deadline_ts=$((start_ts + CALLBACK_TIMEOUT_SECONDS))

  while true; do
    now_ts=$(date +%s)
    if (( now_ts >= deadline_ts )); then
      break
    fi
    local row count read_at
    row="$(db_query "SELECT COUNT(*), COALESCE(MAX(read_at)::text, '') FROM pymes_in_app_notifications WHERE entity_type = 'governance_approval' AND entity_id = '${approval_id}';")"
    row="$(printf '%s' "$row" | tr -d '\r')"
    IFS='|' read -r count read_at <<<"$row"
    if [[ "$row" == "$count" ]]; then
      read_at=""
    fi
    case "$expected" in
      unread)
        if [[ "${count:-0}" != "0" && -z "$read_at" ]]; then
          return 0
        fi
        ;;
      read)
        if [[ "${count:-0}" != "0" && -n "$read_at" ]]; then
          return 0
        fi
        ;;
    esac
    sleep "$POLL_INTERVAL_SECONDS"
  done

  return 1
}

print_section() {
  echo ""
  bold "▸ $1"
  echo ""
}

require_cmd curl
require_cmd python3
require_cmd docker

TARGET_RESOURCE="e2e-target-$(date +%s)"
NOTE="E2E ${DECISION} governance notification"

echo ""
bold "═══════════════════════════════════════════════════"
echo ""
bold "  Governance Notifications — E2E"
echo ""
bold "  Decision: $DECISION"
echo ""
bold "  cp-backend: $BASE_URL"
echo ""
bold "  governance: $GOVERNANCE_URL"
echo ""
bold "═══════════════════════════════════════════════════"
echo ""

print_section "Health"
http_request BODY STATUS GET "$GOVERNANCE_URL/readyz"
assert_status "GET governance /readyz" "$STATUS" "200" "$BODY"
http_request BODY STATUS GET "$BASE_URL/healthz"
assert_status "GET cp-backend /healthz" "$STATUS" "200" "$BODY"

print_section "Fixtures"
http_request BODY STATUS GET "$GOVERNANCE_URL/v1/action-types?org_id=$TENANT_ID" -H "X-API-Key: $GOVERNANCE_API_KEY"
assert_status "GET governance action types" "$STATUS" "200" "$BODY"
ACTION_TYPE_ID="$(json_find_action_type_id "$BODY" "$TENANT_ID")"
if [[ -z "$ACTION_TYPE_ID" ]]; then
  http_request BODY STATUS POST "$GOVERNANCE_URL/v1/action-types" \
    -H "X-API-Key: $GOVERNANCE_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary "{\"org_id\":\"$TENANT_ID\",\"name\":\"$ACTION_TYPE\",\"description\":\"E2E approval test action\",\"risk_class\":\"medium\"}"
  assert_status "POST governance action type" "$STATUS" "201" "$BODY"
else
  printf "  %-56s %s (%s)\n" "action type fixture already present" "$(green "PASS")" "$ACTION_TYPE_ID"
fi

http_request BODY STATUS GET "$GOVERNANCE_URL/v1/delegations" -H "X-API-Key: $GOVERNANCE_API_KEY"
assert_status "GET governance delegations" "$STATUS" "200" "$BODY"
DELEGATION_ID="$(json_find_delegation_id "$BODY" "$TENANT_ID")"
if [[ -z "$DELEGATION_ID" ]]; then
  http_request BODY STATUS POST "$GOVERNANCE_URL/v1/delegations" \
    -H "X-API-Key: $GOVERNANCE_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary "{\"owner_id\":\"pymes-platform\",\"owner_type\":\"service\",\"agent_id\":\"$REQUESTER_ID\",\"agent_type\":\"service\",\"allowed_action_types\":[\"$ACTION_TYPE\"],\"allowed_resources\":[],\"purpose\":\"E2E governance notifications\",\"max_risk_class\":\"high\"}"
  assert_status "POST governance delegation" "$STATUS" "201" "$BODY"
else
  printf "  %-56s %s (%s)\n" "delegation fixture already present" "$(green "PASS")" "$DELEGATION_ID"
fi

http_request BODY STATUS GET "$GOVERNANCE_URL/v1/policies" -H "X-API-Key: $GOVERNANCE_API_KEY"
assert_status "GET governance policies" "$STATUS" "200" "$BODY"
POLICY_INFO="$(json_find_policy_info "$BODY" "$TENANT_ID")"
POLICY_ID="${POLICY_INFO%%$'\t'*}"
POLICY_ENABLED="${POLICY_INFO#*$'\t'}"
if [[ -z "$POLICY_ID" ]]; then
  http_request BODY STATUS POST "$GOVERNANCE_URL/v1/policies" \
    -H "X-API-Key: $GOVERNANCE_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary "{\"name\":\"$POLICY_NAME\",\"description\":\"E2E governance notifications\",\"action_type\":\"$ACTION_TYPE\",\"expression\":\"request.action_type == \\\"$ACTION_TYPE\\\"\",\"effect\":\"require_approval\",\"mode\":\"enforced\",\"enabled\":true}"
  assert_status "POST governance policy" "$STATUS" "201" "$BODY"
elif [[ "$POLICY_ENABLED" != "true" ]]; then
  http_request BODY STATUS PATCH "$GOVERNANCE_URL/v1/policies/$POLICY_ID" \
    -H "X-API-Key: $GOVERNANCE_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary '{"enabled":true}'
  assert_status "PATCH governance policy enabled=true" "$STATUS" "200" "$BODY"
else
  printf "  %-56s %s (%s)\n" "policy fixture already enabled" "$(green "PASS")" "$POLICY_ID"
fi

print_section "Submit"
http_request BODY STATUS POST "$GOVERNANCE_URL/v1/requests" \
  -H "X-API-Key: $GOVERNANCE_API_KEY" \
  -H "Content-Type: application/json" \
  --data-binary "{\"requester_type\":\"service\",\"requester_id\":\"$REQUESTER_ID\",\"requester_name\":\"E2E Governance Tester\",\"action_type\":\"$ACTION_TYPE\",\"target_system\":\"pymes\",\"target_resource\":\"$TARGET_RESOURCE\",\"params\":{\"tenant_id\":\"$TENANT_ID\",\"org_id\":\"$TENANT_ID\"},\"reason\":\"E2E governance inbox verification\",\"context\":\"e2e-governance-notifications\"}"
assert_status "POST governance request" "$STATUS" "201" "$BODY"
IFS=$'\t' read -r REQUEST_ID REQUEST_STATUS APPROVAL_ID <<<"$(json_get_submit_fields "$BODY")"
if [[ "$REQUEST_STATUS" != "pending_approval" || -z "$APPROVAL_ID" || -z "$REQUEST_ID" ]]; then
  echo "Respuesta inesperada al crear request: $BODY" >&2
  exit 1
fi
printf "  %-56s %s (%s)\n" "request pending approval created" "$(green "PASS")" "$REQUEST_ID"
printf "  %-56s %s (%s)\n" "approval id captured" "$(green "PASS")" "$APPROVAL_ID"

print_section "Pending Callback"
if wait_for_db_notification_state "$APPROVAL_ID" unread; then
  printf "  %-56s %s (<= %ss)\n" "callback persisted unread inbox notification" "$(green "PASS")" "$CALLBACK_TIMEOUT_SECONDS"
else
  printf "  %-56s %s (> %ss)\n" "callback persisted unread inbox notification" "$(red "FAIL")" "$CALLBACK_TIMEOUT_SECONDS" >&2
  exit 1
fi

http_request BODY STATUS GET "$BASE_URL/v1/governance/approvals/pending" -H "X-API-Key: $API_KEY"
assert_status "GET cp-backend /v1/governance/approvals/pending" "$STATUS" "200" "$BODY"
if [[ "$(json_has_pending_approval "$BODY" "$APPROVAL_ID")" != "1" ]]; then
  echo "La approval no apareció en el proxy de pending approvals" >&2
  exit 1
fi
printf "  %-56s %s\n" "governance proxy lists the pending approval" "$(green "PASS")"

print_section "Resolve"
http_request BODY STATUS POST "$BASE_URL/v1/governance/approvals/$APPROVAL_ID/$DECISION" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  --data-binary "{\"note\":\"$NOTE\"}"
assert_status "POST cp-backend approval $DECISION" "$STATUS" "200" "$BODY"

if wait_for_db_notification_state "$APPROVAL_ID" read; then
  printf "  %-56s %s (<= %ss)\n" "resolution callback marked inbox notification read" "$(green "PASS")" "$CALLBACK_TIMEOUT_SECONDS"
else
  printf "  %-56s %s (> %ss)\n" "resolution callback marked inbox notification read" "$(red "FAIL")" "$CALLBACK_TIMEOUT_SECONDS" >&2
  exit 1
fi

http_request BODY STATUS GET "$BASE_URL/v1/governance/approvals/pending" -H "X-API-Key: $API_KEY"
assert_status "GET cp-backend /v1/governance/approvals/pending after resolve" "$STATUS" "200" "$BODY"
if [[ "$(json_has_pending_approval "$BODY" "$APPROVAL_ID")" != "0" ]]; then
  echo "La approval resuelta sigue pendiente en Governance" >&2
  exit 1
fi
printf "  %-56s %s\n" "governance proxy no longer lists resolved approval" "$(green "PASS")"

echo ""
bold "Resultado: PASS"
echo ""
