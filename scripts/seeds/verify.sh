#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"
# shellcheck source=scripts/seeds/seed_contract.sh
source "$ROOT_DIR/scripts/seeds/seed_contract.sh"

SEED_VERIFY_API_KEY="${SEED_VERIFY_API_KEY:-${VITE_API_KEY:-psk_local_admin}}"
SEED_VERIFY_CORE_URL="${SEED_VERIFY_CORE_URL:-${VITE_API_URL:-http://localhost:8100}}"
SEED_VERIFY_WORKSHOPS_URL="${SEED_VERIFY_WORKSHOPS_URL:-${VITE_WORKSHOPS_API_URL:-http://localhost:8282}}"
SEED_VERIFY_PROFESSIONALS_URL="${SEED_VERIFY_PROFESSIONALS_URL:-${VITE_PROFESSIONALS_API_URL:-http://localhost:8181}}"
SEED_VERIFY_RESTAURANTS_URL="${SEED_VERIFY_RESTAURANTS_URL:-${VITE_RESTAURANTS_API_URL:-http://localhost:8484}}"
SEED_VERIFY_MEDICAL_URL="${SEED_VERIFY_MEDICAL_URL:-${VITE_MEDICAL_API_URL:-http://localhost:8585}}"
CLEAR_MODE=0
if [[ "${1:-}" == "--cleared" ]]; then
  CLEAR_MODE=1
fi

ensure_pymes_seed_db_ready
require_seed_org_external_id
TARGET_ORG_UUID="$(resolve_target_org_uuid)"

week_from="$(date -u -d '1 day ago' +%Y-%m-%dT00:00:00Z)"
week_to="$(date -u -d '14 days' +%Y-%m-%dT23:59:59Z)"
failures=0

check_min() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  local context="$4"

  if [[ ! "$actual" =~ ^[0-9]+$ ]]; then
    printf 'FAIL %s expected>=%s got=%s %s\n' "$name" "$expected" "$actual" "$context" >&2
    failures=$((failures + 1))
    return
  fi
  if (( actual < expected )); then
    printf 'FAIL %s expected>=%s got=%s %s\n' "$name" "$expected" "$actual" "$context" >&2
    failures=$((failures + 1))
    return
  fi
  printf 'OK   %s got=%s %s\n' "$name" "$actual" "$context"
}

check_empty() {
  local name="$1"
  local actual="$2"
  local context="$3"

  if [[ ! "$actual" =~ ^[0-9]+$ ]]; then
    printf 'FAIL %s expected=0 got=%s %s\n' "$name" "$actual" "$context" >&2
    failures=$((failures + 1))
    return
  fi
  if (( actual != 0 )); then
    printf 'FAIL %s expected=0 got=%s %s\n' "$name" "$actual" "$context" >&2
    failures=$((failures + 1))
    return
  fi
  printf 'OK   %s got=0 %s\n' "$name" "$context"
}

query_count() {
  local sql="$1"
  sql="${sql//__ORG_ID__/$TARGET_ORG_UUID}"
  host_pymes_psql -Atq -v ON_ERROR_STOP=1 -c "$sql" | tr -d '[:space:]'
}

api_count() {
  local url="$1"
  curl -fsS -H "X-API-Key: $SEED_VERIFY_API_KEY" "$url" \
    | jq -r 'if type == "object" and (.items | type) == "array" then (.items | length) else "invalid_response" end'
}

if (( CLEAR_MODE == 1 )); then
  printf 'Verificando seed-clear DB org=%s\n' "$TARGET_ORG_UUID"
else
  printf 'Verificando seeds DB org=%s\n' "$TARGET_ORG_UUID"
fi
for check in "${SEED_DB_CHECKS[@]}"; do
  IFS='|' read -r name expected sql <<<"$check"
  actual="$(query_count "$sql")"
  if (( CLEAR_MODE == 1 )); then
    check_empty "$name" "$actual" "db"
  else
    check_min "$name" "$expected" "$actual" "db"
  fi
done

if (( CLEAR_MODE == 1 )); then
  check_min "bootstrapOrg" 1 "$(query_count "SELECT count(*) FROM tenants WHERE id = '__ORG_ID__'::uuid")" "db"
  check_min "bootstrapMembers" 1 "$(query_count "SELECT count(*) FROM tenant_memberships WHERE tenant_id = '__ORG_ID__'::uuid")" "db"
  printf 'SKIP API checks (--cleared)\n'
elif [[ "${SEED_VERIFY_SKIP_API:-}" == "1" ]]; then
  printf 'SKIP API checks (SEED_VERIFY_SKIP_API=1)\n'
else
  if ! command -v jq >/dev/null 2>&1; then
    echo "FAIL api verifier requires jq" >&2
    exit 1
  fi

  printf 'Verificando seeds API\n'
  for check in "${SEED_API_CHECKS[@]}"; do
    IFS='|' read -r name expected base_var path <<<"$check"
    base_url="${!base_var:-}"
    path="${path//__FROM__/$week_from}"
    path="${path//__TO__/$week_to}"
    url="${base_url}${path}"
    if ! actual="$(api_count "$url" 2>/tmp/pymes-seed-verify-curl.err)"; then
      err="$(tr '\n' ' ' </tmp/pymes-seed-verify-curl.err | sed 's/[[:space:]]*$//')"
      printf 'FAIL %s expected>=%s got=request_error endpoint=%s error=%s\n' "$name" "$expected" "$url" "$err" >&2
      failures=$((failures + 1))
      continue
    fi
    check_min "$name" "$expected" "$actual" "endpoint=$url"
  done
fi

if (( failures > 0 )); then
  printf 'Seed verify failed: %s check(s) failed.\n' "$failures" >&2
  exit 1
fi

echo "Seed verify OK."
