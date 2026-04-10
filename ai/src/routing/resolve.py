from __future__ import annotations

from src.agents.catalog import INSIGHT_CHAT_AGENT_NAME, PRODUCT_AGENT_NAME
from src.routing.context import TurnContext
from src.routing.decision import RoutingDecision


async def resolve_routing_decision(context: TurnContext) -> RoutingDecision:
    """Resolve deterministic routing before any LLM orchestration."""

    # Routing stages are intentionally ordered to match the architecture baseline:
    # 1) hard UI/static rules, 2) structured insight handoff, 3) explicit domain hint,
    # 4) legacy insight_chat hint, 5) orchestrator fallback.
    normalized_route_hint = str(context.route_hint or "").strip().lower() or None

    if normalized_route_hint != INSIGHT_CHAT_AGENT_NAME and context.is_menu_request:
        return RoutingDecision(
            handler_kind="static_reply",
            target="route_menu",
            reason="menu_request",
        )

    if normalized_route_hint is None and context.is_ambiguous_query:
        return RoutingDecision(
            handler_kind="static_reply",
            target="route_clarification",
            reason="ambiguous_query",
        )

    if context.handoff_is_structured_insight and context.handoff_is_valid and context.handoff is not None:
        return RoutingDecision(
            handler_kind="insight_lane",
            target=str(context.handoff.insight_scope),
            reason="structured_handoff",
            extras={
                "source": context.handoff.source,
                "notification_id": context.handoff.notification_id,
                "period": context.handoff.period,
                "compare": context.handoff.compare,
                "top_limit": context.handoff.top_limit,
            },
        )

    if normalized_route_hint not in {None, INSIGHT_CHAT_AGENT_NAME, PRODUCT_AGENT_NAME}:
        return RoutingDecision(
            handler_kind="direct_agent",
            target=normalized_route_hint,
            reason="explicit_route_hint",
        )

    if normalized_route_hint == INSIGHT_CHAT_AGENT_NAME and context.legacy_insight_match:
        legacy_request = context.legacy_insight_request
        return RoutingDecision(
            handler_kind="insight_lane",
            target=str(legacy_request.scope),
            reason="legacy_insight_hint",
            extras={
                "source": "insight_chat_legacy_match",
                "notification_id": None,
                "period": legacy_request.period,
                "compare": legacy_request.compare,
                "top_limit": 5,
            },
        )

    return RoutingDecision(
        handler_kind="orchestrator",
        target="general",
        reason="no_deterministic_match",
    )
