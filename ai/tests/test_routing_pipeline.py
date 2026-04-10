from __future__ import annotations

from types import SimpleNamespace

import pytest

from src.api.chat_contract import ChatHandoff
from src.agents.insight_chat_service import InsightChatMatch
from src.routing import TurnContext
from src.routing.resolve import resolve_routing_decision


"""Routing pipeline tests stay pure on purpose.

They do not call LLMs, InsightsService, backend clients, or run_routed_agent.
All external behavior must be precomputed into TurnContext so the table stays fast
and deterministic.
"""


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("name", "context", "expected_kind", "expected_target", "expected_reason", "expected_extras"),
    [
        (
            "menu_before_other_rules",
            TurnContext(
                message="menu",
                route_hint="sales",
                route_hint_source="explicit",
                is_menu_request=True,
            ),
            "static_reply",
            "route_menu",
            "menu_request",
            {},
        ),
        (
            "ambiguous_without_hint",
            TurnContext(
                message="como viene?",
                is_ambiguous_query=True,
            ),
            "static_reply",
            "route_clarification",
            "ambiguous_query",
            {},
        ),
        (
            "structured_handoff",
            TurnContext(
                message="hola",
                handoff=ChatHandoff(
                    source="in_app_notification",
                    notification_id="notif-123",
                    insight_scope="sales_collections",
                    period="week",
                    compare=True,
                    top_limit=5,
                ),
                handoff_is_structured_insight=True,
                handoff_is_valid=True,
            ),
            "insight_lane",
            "sales_collections",
            "structured_handoff",
            {
                "source": "in_app_notification",
                "notification_id": "notif-123",
                "period": "week",
                "compare": True,
                "top_limit": 5,
            },
        ),
        (
            "explicit_domain_hint",
            TurnContext(
                message="mostrame ventas",
                route_hint="sales",
                route_hint_source="explicit",
            ),
            "direct_agent",
            "sales",
            "explicit_route_hint",
            {},
        ),
        (
            "legacy_insight_chat_with_match",
            TurnContext(
                message="como vienen las ventas esta semana",
                route_hint="insight_chat",
                route_hint_source="explicit",
                legacy_insight_request=InsightChatMatch(
                    scope="sales_collections",
                    period="week",
                    compare=True,
                ),
                legacy_insight_match=True,
            ),
            "insight_lane",
            "sales_collections",
            "legacy_insight_hint",
            {
                "source": "insight_chat_legacy_match",
                "notification_id": None,
                "period": "week",
                "compare": True,
                "top_limit": 5,
            },
        ),
        (
            "insight_chat_without_match",
            TurnContext(
                message="hola",
                route_hint="insight_chat",
                route_hint_source="explicit",
                legacy_insight_request=None,
                legacy_insight_match=False,
            ),
            "orchestrator",
            "general",
            "no_deterministic_match",
            {},
        ),
        (
            "no_hint_no_handoff",
            TurnContext(
                message="hola",
            ),
            "orchestrator",
            "general",
            "no_deterministic_match",
            {},
        ),
        (
            "inferred_domain_route",
            TurnContext(
                message="listame los clientes",
                route_hint="customers",
                route_hint_source="inferred",
            ),
            "direct_agent",
            "customers",
            "explicit_route_hint",
            {},
        ),
    ],
)
async def test_resolve_routing_decision_pipeline_table(
    name: str,
    context: TurnContext,
    expected_kind: str,
    expected_target: str,
    expected_reason: str,
    expected_extras: dict[str, object],
) -> None:
    _ = name
    decision = await resolve_routing_decision(context)

    assert decision.handler_kind == expected_kind
    assert decision.target == expected_target
    assert decision.reason == expected_reason
    assert decision.extras == expected_extras


@pytest.mark.asyncio
async def test_resolve_routing_decision_keeps_handoff_before_explicit_domain_hint() -> None:
    context = TurnContext(
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

    decision = await resolve_routing_decision(context)

    assert decision.handler_kind == "insight_lane"
    assert decision.target == "inventory_profit"
    assert decision.reason == "structured_handoff"


@pytest.mark.asyncio
async def test_resolve_routing_decision_ignores_invalid_handoff_and_keeps_explicit_domain_hint() -> None:
    context = TurnContext(
        message="mostrame ventas",
        route_hint="sales",
        route_hint_source="explicit",
        handoff=SimpleNamespace(
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
    )

    decision = await resolve_routing_decision(context)

    assert decision.handler_kind == "direct_agent"
    assert decision.target == "sales"
    assert decision.reason == "explicit_route_hint"
