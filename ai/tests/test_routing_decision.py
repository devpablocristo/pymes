from __future__ import annotations

from dataclasses import FrozenInstanceError

import pytest

from src.routing import RoutingDecision


def test_routing_decision_keeps_contract_fields() -> None:
    decision = RoutingDecision(
        handler_kind="insight_lane",
        target="sales_collections",
        reason="structured_handoff",
        extras={"notification_id": "notif-123"},
    )

    assert decision.handler_kind == "insight_lane"
    assert decision.target == "sales_collections"
    assert decision.reason == "structured_handoff"
    assert decision.extras == {"notification_id": "notif-123"}


def test_routing_decision_is_frozen() -> None:
    decision = RoutingDecision(
        handler_kind="orchestrator",
        target="general",
        reason="no_deterministic_match",
    )

    with pytest.raises(FrozenInstanceError):
        decision.target = "sales"  # type: ignore[misc]
