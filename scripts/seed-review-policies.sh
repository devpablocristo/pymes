#!/usr/bin/env bash
# Seed de action types, policies y delegations en Nexus Review para la integración con Pymes.
# Ejecutar una vez con Review corriendo: bash scripts/seed-review-for-pymes.sh
set -euo pipefail

REVIEW_URL="${REVIEW_URL:-http://localhost:18084}"
API_KEY="${NEXUS_REVIEW_ADMIN_API_KEY:-nexus-review-admin-dev-key}"

post() {
  local path="$1"
  local body="$2"
  curl -s -X POST "${REVIEW_URL}${path}" \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "${body}"
  echo
}

echo "=== Seeding action types ==="

# Transversales
post /v1/action-types '{"name":"appointment.book","risk_class":"low","enabled":true}'
post /v1/action-types '{"name":"appointment.reschedule","risk_class":"low","enabled":true}'
post /v1/action-types '{"name":"appointment.cancel","risk_class":"medium","enabled":true}'
post /v1/action-types '{"name":"discount.apply","risk_class":"medium","enabled":true}'
post /v1/action-types '{"name":"payment_link.generate","risk_class":"low","enabled":true}'
post /v1/action-types '{"name":"refund.create","risk_class":"high","enabled":true}'
post /v1/action-types '{"name":"notification.send","risk_class":"low","enabled":true}'
post /v1/action-types '{"name":"notification.bulk_send","risk_class":"medium","enabled":true}'
post /v1/action-types '{"name":"sale.create","risk_class":"low","enabled":true}'
post /v1/action-types '{"name":"quote.create","risk_class":"low","enabled":true}'
post /v1/action-types '{"name":"cashflow.movement","risk_class":"medium","enabled":true}'

# Workshops
post /v1/action-types '{"name":"work_order.delay_notify","risk_class":"low","enabled":true}'
post /v1/action-types '{"name":"vehicle.service_reminder","risk_class":"low","enabled":true}'

# Procurement
post /v1/action-types '{"name":"purchase.draft","risk_class":"low","enabled":true}'
post /v1/action-types '{"name":"procurement.request","risk_class":"medium","enabled":true}'
post /v1/action-types '{"name":"procurement.submit","risk_class":"medium","enabled":true}'

echo
echo "=== Seeding default policies ==="

# Políticas conservadoras por defecto
post /v1/policies '{
  "name":"auto-allow-appointment-book",
  "action_type":"appointment.book",
  "expression":"request.action_type == \"appointment.book\"",
  "effect":"allow",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"auto-allow-appointment-reschedule",
  "action_type":"appointment.reschedule",
  "expression":"request.action_type == \"appointment.reschedule\"",
  "effect":"allow",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"require-approval-appointment-cancel",
  "action_type":"appointment.cancel",
  "expression":"request.action_type == \"appointment.cancel\"",
  "effect":"require_approval",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"auto-allow-small-discount",
  "action_type":"discount.apply",
  "expression":"request.action_type == \"discount.apply\" && double(request.params.percentage) <= 10.0",
  "effect":"allow",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"require-approval-large-discount",
  "action_type":"discount.apply",
  "expression":"request.action_type == \"discount.apply\" && double(request.params.percentage) > 10.0",
  "effect":"require_approval",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"deny-refund",
  "action_type":"refund.create",
  "expression":"request.action_type == \"refund.create\"",
  "effect":"deny",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"auto-allow-payment-link",
  "action_type":"payment_link.generate",
  "expression":"request.action_type == \"payment_link.generate\"",
  "effect":"allow",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"auto-allow-notification",
  "action_type":"notification.send",
  "expression":"request.action_type == \"notification.send\"",
  "effect":"allow",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"require-approval-bulk-notification",
  "action_type":"notification.bulk_send",
  "expression":"request.action_type == \"notification.bulk_send\"",
  "effect":"require_approval",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"auto-allow-sale",
  "action_type":"sale.create",
  "expression":"request.action_type == \"sale.create\"",
  "effect":"allow",
  "mode":"enforced"
}'

post /v1/policies '{
  "name":"auto-allow-quote",
  "action_type":"quote.create",
  "expression":"request.action_type == \"quote.create\"",
  "effect":"allow",
  "mode":"enforced"
}'

echo
echo "=== Seeding delegation for pymes-ai ==="

post /v1/delegations '{
  "owner_id":"pymes-platform",
  "owner_type":"service",
  "agent_id":"pymes-ai",
  "agent_type":"service",
  "allowed_action_types":[],
  "allowed_resources":[],
  "purpose":"Pymes AI Service — atención al cliente y operaciones gobernadas",
  "max_risk_class":"high",
  "enabled":true
}'

echo
echo "=== Done. Review seeded for Pymes integration ==="
