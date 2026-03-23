"""Gate de gobernanza: evalúa acciones contra Nexus Review antes de ejecutarlas."""
from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Any

from src.review_client.client import ReviewClient

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class ReviewDecision:
    allowed: bool
    decision: str  # allow | deny | require_approval
    request_id: str | None = None
    reason: str | None = None
    approval_id: str | None = None


# Mapeo de nombre de tool a action_type en Review
GOVERNED_ACTIONS: dict[str, str] = {
    "book_appointment": "appointment.book",
    "cancel_appointment": "appointment.cancel",
    "reschedule_appointment": "appointment.reschedule",
    "create_sale": "sale.create",
    "create_quote": "quote.create",
    "generate_payment_link": "payment_link.generate",
    "apply_discount": "discount.apply",
    "create_cash_movement": "cashflow.movement",
    "prepare_purchase_draft": "purchase.draft",
    "create_procurement_request": "procurement.request",
    "submit_procurement_request": "procurement.submit",
    "send_bulk_notification": "notification.bulk_send",
}

# Tools de solo lectura — nunca necesitan Review
READ_ONLY_TOOLS: frozenset[str] = frozenset({
    "search_customers",
    "search_products",
    "search_suppliers",
    "get_sales_summary",
    "get_low_stock",
    "get_stock_level",
    "get_account_balances",
    "get_debtors",
    "get_appointments",
    "get_my_appointments",
    "get_payment_status",
    "get_cashflow_summary",
    "get_purchases",
    "get_recurring_expenses",
    "get_exchange_rates",
    "search_help",
    "get_work_order",
    "list_work_orders",
    "list_vehicles",
    "list_services",
    "get_business_info",
    "get_public_services",
    "check_availability",
    "get_quotes",
    "get_recent_sales",
    "get_quote_payment_link",
    "request_quote",
    "get_procurement_request",
    "list_procurement_requests",
    "remember_fact",
    "apply_business_profile",
    "update_business_info",
    "complete_onboarding_step",
})


async def evaluate_action(
    review_client: ReviewClient,
    tool_name: str,
    tool_args: dict[str, Any],
    org_id: str,
    context: str = "",
) -> ReviewDecision:
    """Evalúa si una acción de tool debe proceder, consultando a Review si es necesario."""

    # Lectura pura → siempre permitir sin consultar Review
    if tool_name in READ_ONLY_TOOLS:
        return ReviewDecision(allowed=True, decision="allow")

    # Acción gobernada → consultar Review
    action_type = GOVERNED_ACTIONS.get(tool_name)
    if action_type is None:
        # Tool no reconocido → fallback conservador
        logger.warning("review_gate_unknown_tool", extra={"tool_name": tool_name})
        return ReviewDecision(allowed=False, decision="require_approval", reason="Tool no reconocido en la política de gobernanza")

    target_resource = str(tool_args.get("id", tool_args.get("reference_id", ""))).strip()
    params: dict[str, Any] = {}
    # Extraer parámetros relevantes para las políticas CEL
    if "percentage" in tool_args:
        params["percentage"] = tool_args["percentage"]
    if "amount" in tool_args:
        params["amount"] = tool_args["amount"]
    if "days_from_now" in tool_args:
        params["days_from_now"] = tool_args["days_from_now"]
    if "duration" in tool_args:
        params["duration"] = tool_args["duration"]
    params["org_id"] = org_id
    params["tool_name"] = tool_name

    reason = f"AI tool {tool_name} solicitado en conversación"

    resp = await review_client.submit_request(
        action_type=action_type,
        target_system="pymes",
        target_resource=target_resource,
        params=params,
        reason=reason,
        context=context,
    )

    if resp.decision == "allow" or resp.decision == "allowed" or resp.decision == "approved":
        return ReviewDecision(
            allowed=True,
            decision="allow",
            request_id=resp.request_id or None,
        )

    if resp.decision == "deny" or resp.decision == "denied" or resp.decision == "rejected":
        return ReviewDecision(
            allowed=False,
            decision="deny",
            request_id=resp.request_id or None,
            reason=resp.decision_reason,
        )

    # require_approval o fallback
    return ReviewDecision(
        allowed=False,
        decision="require_approval",
        request_id=resp.request_id or None,
        reason=resp.decision_reason,
        approval_id=resp.approval_id,
    )
