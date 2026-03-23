#!/usr/bin/env bash
# Crea action types, políticas default y delegation en Nexus Review
# para atención al cliente gobernada.
#
# Uso: REVIEW_URL=http://localhost:18084 REVIEW_API_KEY=nexus-review-admin-dev-key bash scripts/seed-review-policies.sh
set -euo pipefail

REVIEW_URL="${REVIEW_URL:-http://localhost:18084}"
API_KEY="${REVIEW_API_KEY:-nexus-review-admin-dev-key}"

post() {
  local path="$1"
  local body="$2"
  curl -s -X POST "${REVIEW_URL}${path}" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: ${API_KEY}" \
    -d "$body"
  echo
}

echo "=== Creando action types ==="

# Transversales
post "/v1/action-types" '{"name":"appointment.book","risk_class":"low","enabled":true}'
post "/v1/action-types" '{"name":"appointment.reschedule","risk_class":"low","enabled":true}'
post "/v1/action-types" '{"name":"appointment.cancel","risk_class":"medium","enabled":true}'
post "/v1/action-types" '{"name":"discount.apply","risk_class":"medium","enabled":true}'
post "/v1/action-types" '{"name":"payment_link.generate","risk_class":"low","enabled":true}'
post "/v1/action-types" '{"name":"refund.create","risk_class":"high","enabled":true}'
post "/v1/action-types" '{"name":"sale.create","risk_class":"medium","enabled":true}'
post "/v1/action-types" '{"name":"quote.create","risk_class":"low","enabled":true}'
post "/v1/action-types" '{"name":"notification.send","risk_class":"low","enabled":true}'
post "/v1/action-types" '{"name":"notification.bulk_send","risk_class":"medium","enabled":true}'
post "/v1/action-types" '{"name":"cashflow.movement","risk_class":"medium","enabled":true}'
post "/v1/action-types" '{"name":"purchase.draft","risk_class":"low","enabled":true}'
post "/v1/action-types" '{"name":"procurement.request","risk_class":"low","enabled":true}'
post "/v1/action-types" '{"name":"procurement.submit","risk_class":"medium","enabled":true}'

# Workshops
post "/v1/action-types" '{"name":"work_order.update_status","risk_class":"low","enabled":true}'
post "/v1/action-types" '{"name":"work_order.delay_notify","risk_class":"low","enabled":true}'
post "/v1/action-types" '{"name":"vehicle.service_reminder","risk_class":"low","enabled":true}'

echo ""
echo "=== Creando políticas default (taller) ==="

# Agendar turno → siempre permitir
post "/v1/policies" '{
  "name":"appointment-book-allow",
  "action_type":"appointment.book",
  "expression":"request.action_type == \"appointment.book\"",
  "effect":"allow",
  "mode":"enforced"
}'

# Reagendar turno → siempre permitir
post "/v1/policies" '{
  "name":"appointment-reschedule-allow",
  "action_type":"appointment.reschedule",
  "expression":"request.action_type == \"appointment.reschedule\"",
  "effect":"allow",
  "mode":"enforced"
}'

# Cancelar turno → requiere aprobación
post "/v1/policies" '{
  "name":"appointment-cancel-approval",
  "action_type":"appointment.cancel",
  "expression":"request.action_type == \"appointment.cancel\"",
  "effect":"require_approval",
  "mode":"enforced"
}'

# Descuento <= 10% → permitir
post "/v1/policies" '{
  "name":"discount-auto-lte-10",
  "action_type":"discount.apply",
  "expression":"request.action_type == \"discount.apply\" && double(request.params.percentage) <= 10.0",
  "effect":"allow",
  "mode":"enforced"
}'

# Descuento > 10% → requiere aprobación
post "/v1/policies" '{
  "name":"discount-approval-gt-10",
  "action_type":"discount.apply",
  "expression":"request.action_type == \"discount.apply\" && double(request.params.percentage) > 10.0",
  "effect":"require_approval",
  "mode":"enforced"
}'

# Link de pago → permitir
post "/v1/policies" '{
  "name":"payment-link-allow",
  "action_type":"payment_link.generate",
  "expression":"request.action_type == \"payment_link.generate\"",
  "effect":"allow",
  "mode":"enforced"
}'

# Reembolso → denegar (solo humano)
post "/v1/policies" '{
  "name":"refund-deny",
  "action_type":"refund.create",
  "expression":"request.action_type == \"refund.create\"",
  "effect":"deny",
  "mode":"enforced"
}'

# Presupuesto → permitir
post "/v1/policies" '{
  "name":"quote-create-allow",
  "action_type":"quote.create",
  "expression":"request.action_type == \"quote.create\"",
  "effect":"allow",
  "mode":"enforced"
}'

# Venta → requiere aprobación
post "/v1/policies" '{
  "name":"sale-create-approval",
  "action_type":"sale.create",
  "expression":"request.action_type == \"sale.create\"",
  "effect":"require_approval",
  "mode":"enforced"
}'

# Envío masivo → requiere aprobación
post "/v1/policies" '{
  "name":"bulk-notification-approval",
  "action_type":"notification.bulk_send",
  "expression":"request.action_type == \"notification.bulk_send\"",
  "effect":"require_approval",
  "mode":"enforced"
}'

# Watchers: notificaciones y recordatorios → permitir
post "/v1/policies" '{
  "name":"notification-send-allow",
  "action_type":"notification.send",
  "expression":"request.action_type == \"notification.send\"",
  "effect":"allow",
  "mode":"enforced"
}'

post "/v1/policies" '{
  "name":"delay-notify-allow",
  "action_type":"work_order.delay_notify",
  "expression":"request.action_type == \"work_order.delay_notify\"",
  "effect":"allow",
  "mode":"enforced"
}'

post "/v1/policies" '{
  "name":"service-reminder-allow",
  "action_type":"vehicle.service_reminder",
  "expression":"request.action_type == \"vehicle.service_reminder\"",
  "effect":"allow",
  "mode":"enforced"
}'

echo ""
echo "=== Creando delegation para pymes-ai ==="

post "/v1/delegations" '{
  "owner_id":"system",
  "owner_type":"service",
  "agent_id":"pymes-ai",
  "agent_type":"service",
  "allowed_action_types":[
    "appointment.book","appointment.reschedule","appointment.cancel",
    "discount.apply","payment_link.generate","refund.create",
    "sale.create","quote.create","cashflow.movement",
    "purchase.draft","procurement.request","procurement.submit",
    "notification.send","notification.bulk_send"
  ],
  "max_risk_class":"high",
  "purpose":"Atencion al cliente via WhatsApp y chat",
  "enabled":true
}'

echo ""
echo "=== Creando delegation para nexus_companion ==="

post "/v1/delegations" '{
  "owner_id":"system",
  "owner_type":"service",
  "agent_id":"nexus_companion",
  "agent_type":"service",
  "allowed_action_types":[
    "work_order.delay_notify","notification.send",
    "vehicle.service_reminder","notification.bulk_send"
  ],
  "max_risk_class":"medium",
  "purpose":"Watchers proactivos: OTs demoradas, turnos, stock, clientes inactivos",
  "enabled":true
}'

echo ""
echo "=== Seed completo ==="
