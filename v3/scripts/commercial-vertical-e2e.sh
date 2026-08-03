#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
tmp_dir=$(mktemp -d)
cleanup() {
  if test -d "$tmp_dir"; then
    find "$tmp_dir" -type f -exec chmod 600 {} \; 2>/dev/null || true
    rm -r "$tmp_dir"
  fi
}
trap cleanup EXIT INT TERM

for command in curl docker go jq node openssl psql; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "$command is required" >&2
    exit 1
  }
done

private_key="$tmp_dir/clerk-private.pem"
public_key="$tmp_dir/clerk-public.pem"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$private_key" >/dev/null 2>&1
openssl pkey -in "$private_key" -pubout -out "$public_key" >/dev/null 2>&1
chmod 600 "$private_key" "$public_key"

PYMES_CLERK_JWT_KEY=$(sed -n '1,200p' "$public_key")
export PYMES_CLERK_JWT_KEY
export PYMES_CLERK_ISSUER=https://clerk.local
export PYMES_CLERK_AUDIENCE=pymes-v3
export PYMES_CLERK_AUTHORIZED_PARTIES=http://localhost:18080
PYMES_CLERK_WEBHOOK_SECRET="whsec_$(openssl rand -base64 32 | tr -d '\n')"
export PYMES_CLERK_WEBHOOK_SECRET

cd "$root_dir"
docker compose up -d --build --wait

api_url=http://127.0.0.1:18080
fiscal_database_url=${FISCAL_DATABASE_TEST_URL:-postgresql://fiscal@127.0.0.1:55435/pymes_fiscal?sslmode=disable}
fiscal_database_password=${FISCAL_DATABASE_TEST_PASSWORD:-fiscal}
accounting_database_url=${ACCOUNTING_DATABASE_TEST_URL:-postgresql://accounting@127.0.0.1:55436/pymes_accounting?sslmode=disable}
run_suffix="$(date -u +%Y%m%d%H%M%S)-$$"
issue_date=$(date -u +%F)
external_org_a="org_e2e_a_$run_suffix"
external_org_b="org_e2e_b_$run_suffix"
external_org_pending="org_e2e_pending_$run_suffix"
organization_a="org_$(printf '%s' "$external_org_a" | tr '-' '_')"
organization_b="org_$(printf '%s' "$external_org_b" | tr '-' '_')"
organization_pending="org_$(printf '%s' "$external_org_pending" | tr '-' '_')"
user_a="user_e2e_a_$run_suffix"
user_b="user_e2e_b_$run_suffix"
user_pending="user_e2e_pending_$run_suffix"

webhook() {
  event_id=$1
  payload=$2
  timestamp=$(date +%s)
  signature=$(printf '%s' "$payload" |
    node "$root_dir/scripts/e2e-auth.mjs" webhook \
      "$PYMES_CLERK_WEBHOOK_SECRET" "$event_id" "$timestamp" -)
  status=$(curl -sS -o "$tmp_dir/webhook-response" -w '%{http_code}' \
    -X POST "$api_url/api/v1/webhooks/clerk" \
    -H 'Content-Type: application/json' \
    -H "svix-id: $event_id" \
    -H "svix-timestamp: $timestamp" \
    -H "svix-signature: $signature" \
    --data-binary "$payload")
  if test "$status" != 204; then
    echo "Clerk webhook failed: status=$status body=$(sed -n '1,5p' "$tmp_dir/webhook-response")" >&2
    exit 1
  fi
}

project_identity() {
  external_org=$1
  user=$2
  label=$3
  now_ms="$(date +%s)000"
  organization_payload=$(jq -cn \
    --arg id "$external_org" --arg name "E2E $label" --arg slug "e2e-$label-$run_suffix" \
    --argjson now "$now_ms" \
    '{data:{id:$id,name:$name,slug:$slug,created_at:$now,updated_at:$now},object:"event",type:"organization.created",timestamp:$now,instance_id:"ins_e2e"}')
  webhook "evt-org-$label-$run_suffix" "$organization_payload"
  membership_payload=$(jq -cn \
    --arg org "$external_org" --arg user "$user" --arg name "E2E $label" \
    --arg slug "e2e-$label-$run_suffix" --argjson now "$now_ms" \
    '{data:{id:("orgmem_"+$user),role:"org:admin",permissions:["org:members:read","org:members:manage"],organization:{id:$org,name:$name,slug:$slug},public_user_data:{user_id:$user,identifier:"opaque-e2e"},created_at:$now,updated_at:$now},object:"event",type:"organizationMembership.created",timestamp:$now,instance_id:"ins_e2e"}')
  webhook "evt-member-$label-$run_suffix" "$membership_payload"
}

provision() {
  internal_org=$1
  external_org=$2
  label=$3
  (
    cd "$root_dir/backend"
    PYMES_DATABASE_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable' \
    ACCOUNTING_PROVISIONING_URL='http://127.0.0.1:18087' \
    PYMES_ALLOW_INSECURE_LOCAL_SERVICES=true \
    PYMES_ENVIRONMENT=development \
    PYMES_INTERNAL_SIGNING_SEED_B64='AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA=' \
    PYMES_INTERNAL_ISSUER=pymes-v3 \
    PYMES_INTERNAL_KEY_ID=local-dev-1 \
      go run ./cmd/provision-org \
      --id "$internal_org" \
      --name "E2E $label" \
      --slug "e2e-$label-$run_suffix" \
      --clerk-organization-id "$external_org" >/dev/null
  )
}

project_identity "$external_org_a" "$user_a" a
project_identity "$external_org_b" "$user_b" b
project_identity "$external_org_pending" "$user_pending" pending
provision "$organization_a" "$external_org_a" a
provision "$organization_a" "$external_org_a" a
provision "$organization_b" "$external_org_b" b

token_a=$(node "$root_dir/scripts/e2e-auth.mjs" session "$private_key" \
  "$PYMES_CLERK_ISSUER" "$PYMES_CLERK_AUDIENCE" "$PYMES_CLERK_AUTHORIZED_PARTIES" \
  "$user_a" "$external_org_a")
token_b=$(node "$root_dir/scripts/e2e-auth.mjs" session "$private_key" \
  "$PYMES_CLERK_ISSUER" "$PYMES_CLERK_AUDIENCE" "$PYMES_CLERK_AUTHORIZED_PARTIES" \
  "$user_b" "$external_org_b")
token_pending=$(node "$root_dir/scripts/e2e-auth.mjs" session "$private_key" \
  "$PYMES_CLERK_ISSUER" "$PYMES_CLERK_AUDIENCE" "$PYMES_CLERK_AUTHORIZED_PARTIES" \
  "$user_pending" "$external_org_pending")

post_json() {
  post_token=$1
  post_path=$2
  post_key=$3
  post_body=$4
  post_output=$5
  post_expected=${6:-201}
  post_status=$(curl -sS -o "$post_output" -w '%{http_code}' \
    -X POST "$api_url$post_path" \
    -H "Authorization: Bearer $post_token" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: $post_key" \
    -H 'X-Source-Version: 1' \
    -H "X-Request-ID: request-$post_key" \
    -H "X-Correlation-ID: correlation-$post_key" \
    --data-binary "$post_body")
  if test "$post_status" != "$post_expected"; then
    echo "POST $post_path failed: expected=$post_expected status=$post_status body=$(sed -n '1,20p' "$post_output")" >&2
    exit 1
  fi
}

get_json() {
  get_token=$1
  get_path=$2
  get_output=$3
  get_expected=${4:-200}
  get_status=$(curl -sS -o "$get_output" -w '%{http_code}' \
    -H "Authorization: Bearer $get_token" "$api_url$get_path")
  if test "$get_status" != "$get_expected"; then
    echo "GET $get_path failed: expected=$get_expected status=$get_status body=$(sed -n '1,20p' "$get_output")" >&2
    exit 1
  fi
}

enable_fiscal_mock_for_tenant() {
  feature_token=$1
  feature_organization=$2
  feature_output=$3
  feature_status=$(curl -sS -o "$feature_output" -w '%{http_code}' \
    -X PUT \
    "$api_url/api/v1/organizations/$feature_organization/features" \
    -H "Authorization: Bearer $feature_token" \
    -H 'Content-Type: application/json' \
    --data-binary '{
      "scheduling_enabled":false,
      "whatsapp_enabled":false,
      "google_calendar_enabled":false,
      "fiscal_real_enabled":true,
      "expected_version":1
    }')
  if test "$feature_status" != 200; then
    echo "enable fiscal feature failed: status=$feature_status body=$(sed -n '1,20p' "$feature_output")" >&2
    exit 1
  fi
}

enable_fiscal_mock_for_tenant \
  "$token_a" "$organization_a" "$tmp_dir/features-a"
enable_fiscal_mock_for_tenant \
  "$token_b" "$organization_b" "$tmp_dir/features-b"

wait_status() {
  wait_token=$1
  wait_path=$2
  wait_expected=$3
  wait_output=$4
  wait_attempt=0
  while test "$wait_attempt" -lt 240; do
    get_json "$wait_token" "$wait_path" "$wait_output"
    wait_actual=$(jq -r '.status // empty' "$wait_output")
    if test "$wait_actual" = "$wait_expected"; then
      return 0
    fi
    case "$wait_actual" in
      fiscal_rejected|accounting_adjustment_required)
        echo "resource entered terminal/unexpected state $wait_actual: $(sed -n '1,20p' "$wait_output")" >&2
        return 1
        ;;
    esac
    wait_attempt=$((wait_attempt + 1))
    sleep 0.5
  done
  echo "resource did not reach $wait_expected: $(sed -n '1,20p' "$wait_output")" >&2
  return 1
}

party_payload() {
  id=$1
  kind=$2
  name=$3
  jq -cn --arg id "$id" --arg kind "$kind" --arg name "$name" \
    '{id:$id,kind:$kind,display_name:$name}'
}

sale_payload() {
  id=$1
  recipient=$2
  document_type=$3
  amount=$4
  currency=$5
  rate=$6
  net=$7
  vat=$8
  exempt=$9
  source_id=${10:-}
  jq -cn \
    --arg id "$id" --arg recipient "$recipient" --arg document_type "$document_type" \
    --arg amount "$amount" --arg currency "$currency" --arg rate "$rate" \
    --arg net "$net" --arg vat "$vat" --arg exempt "$exempt" \
    --arg issue_date "$issue_date" --arg source_id "$source_id" \
    '{
      id:$id,recipient_ref:$recipient,point_of_sale:1,document_type:$document_type,
      amount:$amount,currency:$currency,credential_ref:"credential/mock/e2e",
      fiscal:{
        environment:"homologation",issue_date:$issue_date,
        totals:{net:$net,vat:$vat,exempt:$exempt,total:$amount},
        recipient:{document_type:"CUIT",document_number:"opaque",vat_condition:"responsable_inscripto"},
        lines:[{description:"e2e",quantity:"1",unit_price:$net,vat_rate:(if $vat == "0" then "0" else "21" end),net:$net}]
      }
    }
    | if $rate != "" then .exchange_rate=$rate else . end
    | if $source_id != "" then .source_document_id=$source_id else . end'
}

purchase_payload() {
  id=$1
  supplier=$2
  currency=$3
  rate=$4
  jq -cn \
    --arg id "$id" --arg supplier "$supplier" --arg currency "$currency" \
    --arg rate "$rate" --arg issue_date "$issue_date" \
    '{
      id:$id,supplier_ref:$supplier,external_document_ref:("supplier-"+$id),
      issue_date:$issue_date,amount:"666.00",currency:$currency,
      net_amount:"600.00",exempt_amount:"0",
      vat_breakdown:[
        {rate:"0",base_amount:"100.00",tax_amount:"0"},
        {rate:"2.5",base_amount:"100.00",tax_amount:"2.50"},
        {rate:"5",base_amount:"100.00",tax_amount:"5.00"},
        {rate:"10.5",base_amount:"100.00",tax_amount:"10.50"},
        {rate:"21",base_amount:"100.00",tax_amount:"21.00"},
        {rate:"27",base_amount:"100.00",tax_amount:"27.00"}
      ]
    } | if $rate != "" then .exchange_rate=$rate else . end'
}

customer_a="customer_$run_suffix"
supplier_a="supplier_$run_suffix"
customer_b="customer_b_$run_suffix"
post_json "$token_a" "/api/v1/organizations/$organization_a/parties" "party-customer-$run_suffix" \
  "$(party_payload "$customer_a" customer "Customer A")" "$tmp_dir/party-a"
post_json "$token_a" "/api/v1/organizations/$organization_a/parties" "party-supplier-$run_suffix" \
  "$(party_payload "$supplier_a" supplier "Supplier A")" "$tmp_dir/party-supplier"
post_json "$token_b" "/api/v1/organizations/$organization_b/parties" "party-b-$run_suffix" \
  "$(party_payload "$customer_b" customer "Customer B")" "$tmp_dir/party-b"
post_json "$token_pending" "/api/v1/organizations/$organization_pending/parties" \
  "party-pending-$run_suffix" \
  "$(party_payload "pending_$run_suffix" customer "Pending")" "$tmp_dir/party-pending" 422
jq -e '.code == "ORG_NOT_PROVISIONED"' "$tmp_dir/party-pending" >/dev/null

get_json "$token_a" "/api/v1/organizations/$organization_b/parties/$customer_b" \
  "$tmp_dir/cross-tenant" 403

invalid_fiscal_status=$(curl -sS -o "$tmp_dir/invalid-fiscal-credential" -w '%{http_code}' \
  -X POST "http://127.0.0.1:18081/internal/v1/organizations/$organization_a/authorizations" \
  -H 'Authorization: Bearer invalid' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: invalid-credential' \
  -H 'X-Correlation-ID: invalid-credential' \
  --data-binary '{}')
if test "$invalid_fiscal_status" != 401; then
  echo "Fiscal accepted an invalid internal credential: $invalid_fiscal_status" >&2
  exit 1
fi

sale_a="sale_a_$run_suffix"
sale_b="sale_b_$run_suffix"
sale_c="sale_c_$run_suffix"
payload_sale_a=$(sale_payload "$sale_a" "$customer_a" FA "121.00" ARS "" "100.00" "21.00" "0")
payload_sale_b=$(sale_payload "$sale_b" "$customer_a" FB "12.10" USD "1000.123456" "10.00" "2.10" "0")
payload_sale_c=$(sale_payload "$sale_c" "$customer_a" FC "121.00" EUR "1200.654321" "100.00" "21.00" "0")

post_json "$token_a" "/api/v1/organizations/$organization_a/sales" "sale-a-$run_suffix" \
  "$payload_sale_a" "$tmp_dir/sale-a-created"
post_json "$token_a" "/api/v1/organizations/$organization_a/sales" "sale-a-$run_suffix" \
  "$payload_sale_a" "$tmp_dir/sale-a-replayed"
if test "$(jq -r '.voucher.voucher_number' "$tmp_dir/sale-a-created")" != \
  "$(jq -r '.voucher.voucher_number' "$tmp_dir/sale-a-replayed")"; then
  echo "idempotent replay reserved another voucher" >&2
  exit 1
fi
changed_sale_a=$(printf '%s' "$payload_sale_a" | jq -c '.amount="122.00"')
post_json "$token_a" "/api/v1/organizations/$organization_a/sales" "sale-a-$run_suffix" \
  "$changed_sale_a" "$tmp_dir/sale-a-conflict" 409

post_json "$token_a" "/api/v1/organizations/$organization_a/sales" "sale-b-$run_suffix" \
  "$payload_sale_b" "$tmp_dir/sale-b-created"
post_json "$token_a" "/api/v1/organizations/$organization_a/sales" "sale-c-$run_suffix" \
  "$payload_sale_c" "$tmp_dir/sale-c-created"

wait_status "$token_a" "/api/v1/organizations/$organization_a/sales/$sale_a" posted "$tmp_dir/sale-a"
wait_status "$token_a" "/api/v1/organizations/$organization_a/sales/$sale_b" posted "$tmp_dir/sale-b"
wait_status "$token_a" "/api/v1/organizations/$organization_a/sales/$sale_c" posted "$tmp_dir/sale-c"
for result in "$tmp_dir/sale-a" "$tmp_dir/sale-b" "$tmp_dir/sale-c"; do
  jq -e '.cae != null and .journal_entry_id != null and .open_item_id != null' "$result" >/dev/null
done

uncertain_sale="sale_uncertain_$run_suffix"
FISCAL_MOCK_SCENARIO=response_lost_after_processing \
  docker compose up -d --force-recreate --wait fiscal-fake
post_json "$token_a" "/api/v1/organizations/$organization_a/sales" "sale-uncertain-$run_suffix" \
  "$(sale_payload "$uncertain_sale" "$customer_a" FA "24.20" ARS "" "20.00" "4.20" "0")" \
  "$tmp_dir/uncertain-created"
wait_status "$token_a" "/api/v1/organizations/$organization_a/sales/$uncertain_sale" posted \
  "$tmp_dir/uncertain-posted"
docker compose up -d --force-recreate --wait fiscal-fake

credit_note="sale_nca_$run_suffix"
debit_note="sale_nda_$run_suffix"
post_json "$token_a" "/api/v1/organizations/$organization_a/sales" "sale-nca-$run_suffix" \
  "$(sale_payload "$credit_note" "$customer_a" NCA "12.10" ARS "" "10.00" "2.10" "0" "$sale_a")" \
  "$tmp_dir/nca-created"
post_json "$token_a" "/api/v1/organizations/$organization_a/sales" "sale-nda-$run_suffix" \
  "$(sale_payload "$debit_note" "$customer_a" NDA "1.21" ARS "" "1.00" "0.21" "0" "$sale_a")" \
  "$tmp_dir/nda-created"
wait_status "$token_a" "/api/v1/organizations/$organization_a/sales/$credit_note" posted "$tmp_dir/nca"
wait_status "$token_a" "/api/v1/organizations/$organization_a/sales/$debit_note" posted "$tmp_dir/nda"

purchase_ars="purchase_ars_$run_suffix"
purchase_usd="purchase_usd_$run_suffix"
post_json "$token_a" "/api/v1/organizations/$organization_a/purchases" "purchase-ars-$run_suffix" \
  "$(purchase_payload "$purchase_ars" "$supplier_a" ARS "")" "$tmp_dir/purchase-ars-created"
post_json "$token_a" "/api/v1/organizations/$organization_a/purchases" "purchase-usd-$run_suffix" \
  "$(purchase_payload "$purchase_usd" "$supplier_a" USD "1000.123456")" "$tmp_dir/purchase-usd-created"
wait_status "$token_a" "/api/v1/organizations/$organization_a/purchases/$purchase_ars" posted "$tmp_dir/purchase-ars"
wait_status "$token_a" "/api/v1/organizations/$organization_a/purchases/$purchase_usd" posted "$tmp_dir/purchase-usd"

purchase_recovery="purchase_recovery_$run_suffix"
docker compose stop accounting
post_json "$token_a" "/api/v1/organizations/$organization_a/purchases" "purchase-recovery-$run_suffix" \
  "$(purchase_payload "$purchase_recovery" "$supplier_a" ARS "")" "$tmp_dir/purchase-recovery-created"
docker compose up -d --wait accounting
wait_status "$token_a" "/api/v1/organizations/$organization_a/purchases/$purchase_recovery" posted \
  "$tmp_dir/purchase-recovery"

payment_receipt="receipt_$run_suffix"
receipt_payload=$(jq -cn \
  --arg id "$payment_receipt" --arg party "$customer_a" --arg sale "$sale_a" \
  '{id:$id,direction:"receipt",party_ref:$party,amount:"50.00",currency:"ARS",applications:[{id:("application_"+$id),document_kind:"sale",document_id:$sale,amount:"50.00"}]}')
post_json "$token_a" "/api/v1/organizations/$organization_a/payments" "receipt-$run_suffix" \
  "$receipt_payload" "$tmp_dir/receipt-created"
wait_status "$token_a" "/api/v1/organizations/$organization_a/payments/$payment_receipt" posted "$tmp_dir/receipt"
wait_status "$token_a" "/api/v1/organizations/$organization_a/sales/$sale_a" partially_paid "$tmp_dir/sale-a-partial"

payment_disbursement="disbursement_$run_suffix"
disbursement_payload=$(jq -cn \
  --arg id "$payment_disbursement" --arg party "$supplier_a" --arg purchase "$purchase_ars" \
  '{id:$id,direction:"disbursement",party_ref:$party,amount:"50.00",currency:"ARS",applications:[{id:("application_"+$id),document_kind:"purchase",document_id:$purchase,amount:"50.00"}]}')
post_json "$token_a" "/api/v1/organizations/$organization_a/payments" "disbursement-$run_suffix" \
  "$disbursement_payload" "$tmp_dir/disbursement-created"
wait_status "$token_a" "/api/v1/organizations/$organization_a/payments/$payment_disbursement" posted "$tmp_dir/disbursement"
wait_status "$token_a" "/api/v1/organizations/$organization_a/purchases/$purchase_ars" partially_paid "$tmp_dir/purchase-ars-partial"

reversal_id="reversal_$run_suffix"
reversal_payload=$(jq -cn \
  --arg id "$reversal_id" --arg purchase "$purchase_usd" \
  --arg effective_at "${issue_date}T12:00:00Z" \
  '{id:$id,document_kind:"purchase",document_id:$purchase,effective_at:$effective_at,reason:"E2E reversal"}')
post_json "$token_a" "/api/v1/organizations/$organization_a/reversals" "reversal-$run_suffix" \
  "$reversal_payload" "$tmp_dir/reversal-created"
wait_status "$token_a" "/api/v1/organizations/$organization_a/purchases/$purchase_usd" reversed "$tmp_dir/purchase-usd-reversed"

fiscal_count=$(printf '%s\n' \
  "SELECT count(*) FROM fiscal.requests WHERE organization_id = :'organization_id';" |
  PGPASSWORD="$fiscal_database_password" \
  psql "$fiscal_database_url" -X -v ON_ERROR_STOP=1 \
    -v organization_id="$organization_a" -At)
if test "$fiscal_count" != 7; then
  echo "expected seven unique fiscal requests, got $fiscal_count" >&2
  exit 1
fi

duplicate_journals=$(printf '%s\n' \
  "SELECT count(*)-count(DISTINCT result->>'journal_entry_id') FROM public.pymes_accounting_commands WHERE organization_id = :'organization_id' AND result ? 'journal_entry_id';" |
  psql "$accounting_database_url" -X -v ON_ERROR_STOP=1 \
    -v organization_id="$organization_a" -At)
if test "$duplicate_journals" != 0; then
  echo "accounting commands converged to duplicate journal entries" >&2
  exit 1
fi

schema_pair=$(printf '%s\n' \
  "SELECT count(DISTINCT schema_name) FROM public.pymes_accounting_organizations WHERE organization_id IN (:'organization_id', :'second_organization_id');" |
  psql "$accounting_database_url" -X -v ON_ERROR_STOP=1 \
    -v organization_id="$organization_a" \
    -v second_organization_id="$organization_b" -At)
if test "$schema_pair" != 2; then
  echo "accounting organizations do not have distinct schemas" >&2
  exit 1
fi

echo "commercial vertical E2E passed: A/B/C, NC/ND, VAT, FX, purchases, partial payments, reversal, idempotency and tenant isolation"
