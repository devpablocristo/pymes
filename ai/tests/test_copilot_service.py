from __future__ import annotations

from datetime import date

import pytest

from src.agents.copilot_service import _match_insight_request, maybe_build_copilot_response
from src.backend_client.auth import AuthContext
from src.insights.domain import InsightMetric, InsightPeriod, SalesCollectionsInsight


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


@pytest.mark.asyncio
async def test_maybe_build_copilot_response_handles_generic_business_query(monkeypatch) -> None:
    async def fake_build_sales_collections_insight(self, *, auth, filters):  # noqa: ANN001
        assert auth.org_id == "org-123"
        assert filters.period == "month"
        return _sales_insight()

    monkeypatch.setattr(
        "src.agents.copilot_service.InsightsService.build_sales_collections_insight",
        fake_build_sales_collections_insight,
    )

    response = await maybe_build_copilot_response(
        backend_client=object(),  # type: ignore[arg-type]
        auth=_auth(),
        user_message="Como viene el negocio este mes?",
    )

    assert response is not None
    assert response.reply == "Ventas arriba 12% este mes."
    assert response.blocks[0]["type"] == "insight_card"
    assert response.blocks[1]["type"] == "kpi_group"


@pytest.mark.asyncio
async def test_maybe_build_copilot_response_returns_none_when_insight_fails(monkeypatch) -> None:
    async def fake_build_sales_collections_insight(self, *, auth, filters):  # noqa: ANN001, ARG001
        raise RuntimeError("boom")

    monkeypatch.setattr(
        "src.agents.copilot_service.InsightsService.build_sales_collections_insight",
        fake_build_sales_collections_insight,
    )

    response = await maybe_build_copilot_response(
        backend_client=object(),  # type: ignore[arg-type]
        auth=_auth(),
        user_message="Como viene el negocio este mes?",
    )

    assert response is None
