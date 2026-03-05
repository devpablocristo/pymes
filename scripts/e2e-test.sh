#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────
# E2E tests for control-plane API
# Requires: docker compose up running
# Usage: ./scripts/e2e-test.sh [BASE_URL]
# ─────────────────────────────────────────────

BASE_URL="${1:-http://localhost:8100}"
API_KEY="psk_local_admin"
ORG_ID="00000000-0000-0000-0000-000000000001"

PASS=0
FAIL=0
TOTAL=0

red()   { printf "\033[31m%s\033[0m" "$1"; }
green() { printf "\033[32m%s\033[0m" "$1"; }
bold()  { printf "\033[1m%s\033[0m" "$1"; }

assert_status() {
    local name="$1" method="$2" url="$3" expected="$4"
    shift 4
    TOTAL=$((TOTAL + 1))

    local response status body
    response=$(curl -s -w "\n%{http_code}" -X "$method" "$url" \
        -H "X-API-Key: $API_KEY" \
        -H "Content-Type: application/json" \
        "$@" 2>&1)
    status=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')

    if [ "$status" = "$expected" ]; then
        PASS=$((PASS + 1))
        printf "  %-50s %s %s\n" "$name" "$(green "PASS")" "($status)"
    else
        FAIL=$((FAIL + 1))
        printf "  %-50s %s expected=%s got=%s\n" "$name" "$(red "FAIL")" "$expected" "$status"
        printf "    body: %.200s\n" "$body"
    fi
    echo "$body"
}

assert_status_noauth() {
    local name="$1" method="$2" url="$3" expected="$4"
    shift 4
    TOTAL=$((TOTAL + 1))

    local response status body
    response=$(curl -s -w "\n%{http_code}" -X "$method" "$url" \
        -H "Content-Type: application/json" \
        "$@" 2>&1)
    status=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')

    if [ "$status" = "$expected" ]; then
        PASS=$((PASS + 1))
        printf "  %-50s %s %s\n" "$name" "$(green "PASS")" "($status)"
    else
        FAIL=$((FAIL + 1))
        printf "  %-50s %s expected=%s got=%s\n" "$name" "$(red "FAIL")" "$expected" "$status"
        printf "    body: %.200s\n" "$body"
    fi
}

assert_json_field() {
    local name="$1" body="$2" field="$3" expected="$4"
    TOTAL=$((TOTAL + 1))

    local actual
    actual=$(echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin).get('$field',''))" 2>/dev/null || echo "PARSE_ERROR")

    if [ "$actual" = "$expected" ]; then
        PASS=$((PASS + 1))
        printf "  %-50s %s %s=%s\n" "$name" "$(green "PASS")" "$field" "$actual"
    else
        FAIL=$((FAIL + 1))
        printf "  %-50s %s %s expected=%s got=%s\n" "$name" "$(red "FAIL")" "$field" "$expected" "$actual"
    fi
}

# Capture body from assert_status for further checks
BODY=""
assert_status_capture() {
    local name="$1" method="$2" url="$3" expected="$4"
    shift 4

    local response status
    response=$(curl -s -w "\n%{http_code}" -X "$method" "$url" \
        -H "X-API-Key: $API_KEY" \
        -H "Content-Type: application/json" \
        "$@" 2>&1)
    status=$(echo "$response" | tail -1)
    BODY=$(echo "$response" | sed '$d')
    TOTAL=$((TOTAL + 1))

    if [ "$status" = "$expected" ]; then
        PASS=$((PASS + 1))
        printf "  %-50s %s %s\n" "$name" "$(green "PASS")" "($status)"
    else
        FAIL=$((FAIL + 1))
        printf "  %-50s %s expected=%s got=%s\n" "$name" "$(red "FAIL")" "$expected" "$status"
        printf "    body: %.200s\n" "$BODY"
    fi
}

echo ""
bold "═══════════════════════════════════════════════════"
echo ""
bold "  Control Plane — E2E Tests"
echo ""
bold "  Target: $BASE_URL"
echo ""
bold "═══════════════════════════════════════════════════"
echo ""

# ── Health ──
echo ""
bold "▸ Health"
assert_status_capture "GET /healthz" GET "$BASE_URL/healthz" 200
assert_json_field "healthz.status = ok" "$BODY" "status" "ok"

# ── Auth ──
echo ""
bold "▸ Authentication"
assert_status_noauth "GET without key → 401" GET "$BASE_URL/v1/users/me" 401
assert_status_noauth "GET with bad key → 401" GET "$BASE_URL/v1/users/me" 401 -H "X-API-Key: bad_key"

# ── Users ──
echo ""
bold "▸ Users"
assert_status_capture "GET /v1/users/me" GET "$BASE_URL/v1/users/me" 200

# ── Admin ──
echo ""
bold "▸ Admin"
assert_status_capture "GET /v1/admin/tenant-settings" GET "$BASE_URL/v1/admin/tenant-settings" 200
assert_json_field "tenant plan_code = starter" "$BODY" "plan_code" "starter"
assert_json_field "tenant org_id matches seed" "$BODY" "org_id" "$ORG_ID"

assert_status_capture "GET /v1/admin/bootstrap" GET "$BASE_URL/v1/admin/bootstrap" 200

assert_status_capture "PUT /v1/admin/tenant-settings" PUT "$BASE_URL/v1/admin/tenant-settings" 200 \
    -d '{"plan_code":"growth","hard_limits":{"max_users":50}}'
assert_json_field "updated plan_code = growth" "$BODY" "plan_code" "growth"

# restore
assert_status "PUT restore plan to starter" PUT "$BASE_URL/v1/admin/tenant-settings" 200 \
    -d '{"plan_code":"starter","hard_limits":{}}' > /dev/null

assert_status_capture "GET /v1/admin/activity" GET "$BASE_URL/v1/admin/activity" 200

# ── API Keys ──
echo ""
bold "▸ API Keys"
assert_status_capture "GET /v1/orgs/:org_id/api-keys" GET "$BASE_URL/v1/orgs/$ORG_ID/api-keys" 200

assert_status_capture "POST /v1/orgs/:org_id/api-keys (create)" POST "$BASE_URL/v1/orgs/$ORG_ID/api-keys" 201 \
    -d '{"name":"e2e-test-key","scopes":["read"]}'
NEW_KEY_ID=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('key',{}).get('id',''))" 2>/dev/null || echo "")
NEW_RAW_KEY=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('raw_key',''))" 2>/dev/null || echo "")

if [ -n "$NEW_KEY_ID" ] && [ "$NEW_KEY_ID" != "" ]; then
    TOTAL=$((TOTAL + 1))
    PASS=$((PASS + 1))
    printf "  %-50s %s id=%s\n" "created key has id" "$(green "PASS")" "${NEW_KEY_ID:0:8}..."

    assert_status_capture "POST rotate key" POST "$BASE_URL/v1/orgs/$ORG_ID/api-keys/$NEW_KEY_ID/rotate" 200

    assert_status "DELETE /v1/orgs/:org_id/api-keys/:id" DELETE "$BASE_URL/v1/orgs/$ORG_ID/api-keys/$NEW_KEY_ID" 204 > /dev/null
else
    TOTAL=$((TOTAL + 1))
    FAIL=$((FAIL + 1))
    printf "  %-50s %s\n" "created key has id" "$(red "FAIL")"
fi

# ── Members ──
echo ""
bold "▸ Members"
assert_status_capture "GET /v1/orgs/:org_id/members" GET "$BASE_URL/v1/orgs/$ORG_ID/members" 200

# ── Audit ──
echo ""
bold "▸ Audit"
assert_status_capture "GET /v1/audit" GET "$BASE_URL/v1/audit" 200
assert_status "GET /v1/audit/export" GET "$BASE_URL/v1/audit/export" 200 > /dev/null

# ── Notifications ──
echo ""
bold "▸ Notifications"
assert_status_capture "GET /v1/notifications/preferences" GET "$BASE_URL/v1/notifications/preferences" 200

assert_status "PUT /v1/notifications/preferences" PUT "$BASE_URL/v1/notifications/preferences" 200 \
    -d '{"notification_type":"billing","channel":"email","enabled":true}' > /dev/null

# ── Billing ──
echo ""
bold "▸ Billing"
assert_status_capture "GET /v1/billing/status" GET "$BASE_URL/v1/billing/status" 200

# ── Orgs (public) ──
echo ""
bold "▸ Orgs"
assert_status_capture "POST /v1/orgs" POST "$BASE_URL/v1/orgs" 201 \
    -d "{\"name\":\"E2E Org $(date +%s)\",\"slug\":\"e2e-$(date +%s)\",\"actor\":\"e2e-test\"}"

# ── CORS ──
echo ""
bold "▸ CORS"
CORS_HEADERS=$(curl -s -I -X OPTIONS "$BASE_URL/v1/users/me" \
    -H "Origin: http://localhost:5180" \
    -H "Access-Control-Request-Method: GET" 2>&1)
TOTAL=$((TOTAL + 1))
if echo "$CORS_HEADERS" | grep -qi "access-control-allow-origin"; then
    PASS=$((PASS + 1))
    printf "  %-50s %s\n" "OPTIONS returns CORS headers" "$(green "PASS")"
else
    FAIL=$((FAIL + 1))
    printf "  %-50s %s\n" "OPTIONS returns CORS headers" "$(red "FAIL")"
fi

# ── Summary ──
echo ""
bold "═══════════════════════════════════════════════════"
echo ""
if [ "$FAIL" -eq 0 ]; then
    printf "  $(green "ALL PASSED"): %d/%d tests\n" "$PASS" "$TOTAL"
else
    printf "  $(red "FAILED"): %d passed, %d failed, %d total\n" "$PASS" "$FAIL" "$TOTAL"
fi
echo ""
bold "═══════════════════════════════════════════════════"
echo ""

exit "$FAIL"
