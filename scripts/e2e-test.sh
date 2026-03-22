#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────
# E2E tests for control-plane API
# Requires: docker compose up (cp-backend con PYMES_SEED_DEMO → API key psk_local_admin)
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

assert_condition() {
    local name="$1" ok="$2" detail="${3:-}"
    TOTAL=$((TOTAL + 1))
    if [ "$ok" = "1" ]; then
        PASS=$((PASS + 1))
        printf "  %-50s %s %s\n" "$name" "$(green "PASS")" "$detail"
    else
        FAIL=$((FAIL + 1))
        printf "  %-50s %s %s\n" "$name" "$(red "FAIL")" "$detail"
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

# ── Core Negocio (Prompt 01) ──
echo ""
bold "▸ Core Negocio"
TS="$(date +%s)"

assert_status_capture "POST /v1/customers (create)" POST "$BASE_URL/v1/customers" 201 \
    -d "{\"type\":\"person\",\"name\":\"E2E Cliente $TS\",\"email\":\"e2e-cliente-$TS@local.dev\"}"
CUSTOMER_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")

assert_status_capture "POST /v1/products (create)" POST "$BASE_URL/v1/products" 201 \
    -d "{\"type\":\"product\",\"sku\":\"E2E-SKU-$TS\",\"name\":\"E2E Producto $TS\",\"price\":100,\"cost_price\":50,\"track_stock\":true}"
PRODUCT_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")

if [ -n "$PRODUCT_ID" ] && [ "$PRODUCT_ID" != "" ]; then
    assert_status "POST /v1/inventory/:product_id/adjust" POST "$BASE_URL/v1/inventory/$PRODUCT_ID/adjust" 200 \
        -d '{"quantity":20,"notes":"e2e initial stock"}' > /dev/null
fi

assert_status_capture "POST /v1/sales (create)" POST "$BASE_URL/v1/sales" 201 \
    -d "{\"customer_id\":\"$CUSTOMER_ID\",\"customer_name\":\"E2E Cliente $TS\",\"payment_method\":\"cash\",\"items\":[{\"product_id\":\"$PRODUCT_ID\",\"description\":\"E2E Producto $TS\",\"quantity\":2,\"unit_price\":100}]}"
SALE_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")

if [ -n "$PRODUCT_ID" ] && [ "$PRODUCT_ID" != "" ]; then
    assert_status_capture "GET /v1/inventory/:product_id" GET "$BASE_URL/v1/inventory/$PRODUCT_ID" 200
    STOCK_QTY=$(echo "$BODY" | python3 -c "import sys,json; v=json.load(sys.stdin).get('quantity',-999); print(v)" 2>/dev/null || echo "-999")
    if [ "$STOCK_QTY" = "18" ] || [ "$STOCK_QTY" = "18.0" ]; then
        assert_condition "stock decremented by sale (20 -> 18)" 1 "quantity=$STOCK_QTY"
    else
        assert_condition "stock decremented by sale (20 -> 18)" 0 "quantity=$STOCK_QTY"
    fi
fi

assert_status_capture "GET /v1/cashflow?type=income" GET "$BASE_URL/v1/cashflow?type=income&limit=50" 200
HAS_SALE_CASH=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(1 if any((it.get('reference_type')=='sale' and it.get('type')=='income') for it in d.get('items',[])) else 0)" 2>/dev/null || echo "0")
assert_condition "cashflow has sale income movement" "$HAS_SALE_CASH"

assert_status_capture "POST /v1/quotes (create)" POST "$BASE_URL/v1/quotes" 201 \
    -d "{\"customer_id\":\"$CUSTOMER_ID\",\"customer_name\":\"E2E Cliente $TS\",\"items\":[{\"product_id\":\"$PRODUCT_ID\",\"description\":\"E2E Producto $TS\",\"quantity\":1,\"unit_price\":100}]}"
QUOTE_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")

if [ -n "$QUOTE_ID" ] && [ "$QUOTE_ID" != "" ]; then
    assert_status_capture "POST /v1/quotes/:id/to-sale" POST "$BASE_URL/v1/quotes/$QUOTE_ID/to-sale" 200 \
        -d '{"payment_method":"transfer"}'
    assert_status_capture "GET /v1/quotes/:id after to-sale" GET "$BASE_URL/v1/quotes/$QUOTE_ID" 200
    QUOTE_STATUS=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null || echo "")
    if [ "$QUOTE_STATUS" = "accepted" ]; then
        assert_condition "quote accepted after to-sale" 1 "status=$QUOTE_STATUS"
    else
        assert_condition "quote accepted after to-sale" 0 "status=$QUOTE_STATUS"
    fi
fi

assert_status "GET /v1/reports/sales-summary" GET "$BASE_URL/v1/reports/sales-summary?from=2020-01-01&to=2099-12-31" 200 > /dev/null
assert_status "GET /v1/reports/sales-by-product" GET "$BASE_URL/v1/reports/sales-by-product?from=2020-01-01&to=2099-12-31" 200 > /dev/null
assert_status "GET /v1/reports/profit-margin" GET "$BASE_URL/v1/reports/profit-margin?from=2020-01-01&to=2099-12-31" 200 > /dev/null

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
