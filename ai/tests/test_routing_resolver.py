from __future__ import annotations

import pytest

from src.api.chat_contract import ChatHandoff
from src.routing import TurnContext
from src.routing.resolve import resolve_routing_decision


@pytest.mark.asyncio
async def test_resolve_routing_decision_prefers_handoff_over_explicit_domain_hint() -> None:
    decision = await resolve_routing_decision(
        TurnContext(
            message="hola",
            route_hint="sales",
            route_hint_source="explicit",
            handoff=ChatHandoff(
                source="in_app_notification",
                notification_id="notif-123",
                insight_scope="inventory_profit",
                period="month",
                compare=False,
                top_limit=3,
            ),
            handoff_is_structured_insight=True,
            handoff_is_valid=True,
        )
    )

    assert decision.handler_kind == "insight_lane"
    assert decision.target == "inventory_profit"
    assert decision.reason == "structured_handoff"


@pytest.mark.asyncio
async def test_resolve_routing_decision_ignores_invalid_handoff_without_legacy_match() -> None:
    decision = await resolve_routing_decision(
        TurnContext(
            message="hola",
            route_hint="insight_chat",
            route_hint_source="explicit",
            handoff=ChatHandoff(
                source="in_app_notification",
                notification_id="notif-404",
                insight_scope="sales_collections",
                period="month",
                compare=True,
                top_limit=5,
            ),
            handoff_is_structured_insight=True,
            handoff_is_valid=False,
            handoff_validation_reason="notification_not_found",
            legacy_insight_request=None,
            legacy_insight_match=False,
        )
    )

    assert decision.handler_kind == "orchestrator"
    assert decision.target == "general"
    assert decision.reason == "no_deterministic_match"
