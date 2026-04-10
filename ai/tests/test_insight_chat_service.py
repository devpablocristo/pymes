from __future__ import annotations

from datetime import date

import pytest

from src.agents.insight_chat_service import (
    _match_insight_request,
    build_insight_chat_response_for_scope,
    build_internal_insight_evidence,
    maybe_build_insight_chat_response,
)
from src.backend_client.auth import AuthContext
from src.insights.domain import InsightFilters, InsightMetric, InsightPeriod, SalesCollectionsInsight


def _auth() -> AuthContext:
    return AuthContext(
        tenant_id="org-123",
        actor="user-1",
        role="admin",
        scopes=["admin:console:write"],
        mode="jwt",
    )


def _sales_insight() -> SalesCollectionsInsight:
    return SalesCollectionsInsight(
        period=InsightPeriod(label="Este mes", from_date=date(2026, 3, 1), to_date=date(2026, 3, 31)),
        summary="Ventas arriba 12% este mes.",
        kpis=[
            InsightMetric(
                key="sales",
                label="Ventas",
                unit="currency",
                value=120000,
                previous_value=107000,
                delta=13000,
                delta_pct=12.1,
                trend="up",
            )
        ],
        highlights=[],
        recommendations=["Mantener seguimiento semanal."],
        top_customers=[],
        payment_mix=[],
        debtors=[],
    )


def test_match_insight_request_detects_generic_business_summary() -> None:
    request = _match_insight_request("Como viene el negocio este mes?")

    assert request is not None
    assert request.scope == "sales_collections"
    assert request.period == "month"
    assert request.compare is True


def test_match_insight_request_detects_general_business_panorama_weekly() -> None:
    request = _match_insight_request("Dame un panorama general de la empresa esta semana")

    assert request is not None
    assert request.scope == "sales_collections"
    assert request.period == "week"


def test_match_insight_request_detects_sales_question_without_period() -> None:
    request = _match_insight_request("Como van las ventas?")

    assert request is not None
    assert request.scope == "sales_collections"
    assert request.period == "month"


def test_match_insight_request_ignores_specific_status_request() -> None:
    request = _match_insight_request("Cual es el estado del cobro 1234?")

    assert request is None


def test_match_insight_request_ignores_specific_status_request_without_numeric_reference() -> None:
    request = _match_insight_request("Cual es el estado del cobro de Maria?")

    assert request is None


def test_build_internal_insight_evidence_is_serializable() -> None:
    evidence = build_internal_insight_evidence(
        insight=_sales_insight(),
        filters=InsightFilters(period="month", compare=True, top_limit=5),
        notification_id="notif-123",
        computed_at="2026-04-10T12:00:00Z",
    )

    payload = evidence.model_dump(mode="json")

    assert payload["source"] == "insight_handoff"
    assert payload["notification_id"] == "notif-123"
    assert payload["scope"] == "sales_collections"
    assert payload["period"] == "month"
    assert payload["compare"] is True
    assert payload["top_limit"] == 5
    assert payload["summary"] == "Ventas arriba 12% este mes."
    assert payload["computed_at"] == "2026-04-10T12:00:00Z"
    assert payload["current_period"]["from_date"] == "2026-03-01"
    assert payload["kpis"][0]["label"] == "Ventas"


def test_build_internal_insight_evidence_collects_entity_ids() -> None:
    insight = _sales_insight().model_copy(
        update={
            "top_customers": [
                {"customer_id": "cust-1", "customer_name": "Acme", "total": 450.0, "count": 4, "share_pct": 45.0}
            ],
            "debtors": [
                {"party_id": "party-1", "party_name": "Cliente Uno", "total_debt": 350.0, "oldest_date": None}
            ],
        }
    )
    evidence = build_internal_insight_evidence(
        insight=insight,
        filters=InsightFilters(period="week", compare=False, top_limit=3),
        notification_id="notif-999",
        computed_at="2026-04-10T12:00:00Z",
    )

    assert evidence.entity_ids == ["cust-1", "party-1"]


@pytest.mark.asyncio
async def test_maybe_build_insight_chat_response_handles_generic_business_query(monkeypatch) -> None:
    async def fake_build_sales_collections_insight(self, *, auth, filters):  # noqa: ANN001
        assert auth.org_id == "org-123"
        assert filters.period == "month"
        return _sales_insight()

    monkeypatch.setattr(
        "src.agents.insight_chat_service.InsightsService.build_sales_collections_insight",
        fake_build_sales_collections_insight,
    )

    response = await maybe_build_insight_chat_response(
        backend_client=object(),  # type: ignore[arg-type]
        auth=_auth(),
        user_message="Como viene el negocio este mes?",
    )

    assert response is not None
    assert response.reply == "Ventas arriba 12% este mes."
    assert response.blocks[0]["type"] == "insight_card"
    assert response.blocks[1]["type"] == "kpi_group"


@pytest.mark.asyncio
async def test_build_insight_chat_response_for_scope_matches_legacy_insight_shape(monkeypatch) -> None:
    async def fake_build_sales_collections_insight(self, *, auth, filters):  # noqa: ANN001
        assert auth.org_id == "org-123"
        assert filters.period == "month"
        assert filters.compare is True
        assert filters.top_limit == 5
        return _sales_insight()

    monkeypatch.setattr(
        "src.agents.insight_chat_service.InsightsService.build_sales_collections_insight",
        fake_build_sales_collections_insight,
    )

    legacy_response = await maybe_build_insight_chat_response(
        backend_client=object(),  # type: ignore[arg-type]
        auth=_auth(),
        user_message="Como viene el negocio este mes?",
    )
    handoff_response = await build_insight_chat_response_for_scope(
        backend_client=object(),  # type: ignore[arg-type]
        auth=_auth(),
        scope="sales_collections",
        period="month",
        compare=True,
        top_limit=5,
    )

    assert legacy_response is not None
    assert handoff_response is not None
    assert handoff_response.reply == legacy_response.reply
    assert handoff_response.blocks == legacy_response.blocks
    assert handoff_response.blocks[0]["type"] == "insight_card"
    assert handoff_response.blocks[1]["type"] == "kpi_group"
    assert handoff_response.blocks[2]["type"] == "table"


@pytest.mark.asyncio
async def test_maybe_build_insight_chat_response_returns_none_when_insight_fails(monkeypatch) -> None:
    async def fake_build_sales_collections_insight(self, *, auth, filters):  # noqa: ANN001, ARG001
        raise RuntimeError("boom")

    monkeypatch.setattr(
        "src.agents.insight_chat_service.InsightsService.build_sales_collections_insight",
        fake_build_sales_collections_insight,
    )

    response = await maybe_build_insight_chat_response(
        backend_client=object(),  # type: ignore[arg-type]
        auth=_auth(),
        user_message="Como viene el negocio este mes?",
    )

    assert response is None
