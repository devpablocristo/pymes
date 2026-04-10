from __future__ import annotations

from datetime import date

from fastapi import FastAPI
from fastapi.testclient import TestClient

from runtime.contexts import AuthContext
from src.api.deps import get_auth_context, get_backend_client, get_repository
from src.api.notifications_router import get_insights_service, router
import src.api.notifications_router as notifications_router_module
from src.insights.domain import (
    CustomersRetentionInsight,
    DebtorInsight,
    InsightFilters,
    InsightHighlight,
    InsightMetric,
    InsightPeriod,
    InventoryProfitInsight,
    PaymentMethodInsight,
    ProductPerformanceInsight,
    SalesCollectionsInsight,
    StockAlertInsight,
    TopCustomerInsight,
)


class StubRepo:
    def __init__(self) -> None:
        self.tracked_usage: list[tuple[int, int]] = []

    async def get_plan_code(self, _org_id: str) -> str:
        return "starter"

    async def get_month_usage(self, _org_id: str, _year: int, _month: int) -> dict[str, int]:
        return {"queries": 0, "tokens_input": 0, "tokens_output": 0}

    async def track_usage(self, _org_id: str, tokens_in: int, tokens_out: int) -> None:
        self.tracked_usage.append((tokens_in, tokens_out))


class StubInsightsService:
    def __init__(self) -> None:
        self.calls: list[tuple[str, InsightFilters]] = []

    async def build_sales_collections_insight(
        self,
        *,
        auth: AuthContext,
        filters: InsightFilters,
    ) -> SalesCollectionsInsight:
        _ = auth
        self.calls.append(("sales_collections", filters))
        return SalesCollectionsInsight(
            period=InsightPeriod(label="month", from_date=date(2026, 3, 1), to_date=date(2026, 3, 31)),
            comparison_period=InsightPeriod(label="previous_period", from_date=date(2026, 2, 1), to_date=date(2026, 2, 28)),
            summary="Resumen generado",
            kpis=[
                InsightMetric(
                    key="total_sales",
                    label="Ventas",
                    unit="currency",
                    value=1000.0,
                    previous_value=800.0,
                    delta=200.0,
                    delta_pct=25.0,
                    trend="up",
                )
            ],
            highlights=[InsightHighlight(severity="positive", title="Ventas en crecimiento", detail="Suben 25%.")],
            recommendations=["Seguir monitoreando ventas y cobranzas."],
            top_customers=[
                TopCustomerInsight(
                    customer_id="c1",
                    customer_name="Acme",
                    total=450.0,
                    count=4,
                    share_pct=45.0,
                )
            ],
            payment_mix=[
                PaymentMethodInsight(
                    payment_method="cash",
                    total=600.0,
                    count=6,
                    share_pct=60.0,
                )
            ],
            debtors=[
                DebtorInsight(
                    party_id="p1",
                    party_name="Cliente Uno",
                    total_debt=350.0,
                    oldest_date=date(2026, 2, 14),
                )
            ],
        )

    async def build_inventory_profit_insight(
        self,
        *,
        auth: AuthContext,
        filters: InsightFilters,
    ) -> InventoryProfitInsight:
        _ = auth
        self.calls.append(("inventory_profit", filters))
        return InventoryProfitInsight(
            period=InsightPeriod(label="month", from_date=date(2026, 3, 1), to_date=date(2026, 3, 31)),
            comparison_period=InsightPeriod(label="previous_period", from_date=date(2026, 2, 1), to_date=date(2026, 2, 28)),
            summary="Resumen de inventario",
            kpis=[
                InsightMetric(
                    key="margin_pct",
                    label="Margen bruto",
                    unit="percentage",
                    value=41.67,
                    previous_value=35.0,
                    delta=6.67,
                    delta_pct=19.06,
                    trend="up",
                )
            ],
            highlights=[InsightHighlight(severity="warning", title="Reposición pendiente", detail="Hay 1 producto crítico.")],
            recommendations=["Priorizar reposición de productos críticos."],
            top_products=[
                ProductPerformanceInsight(
                    product_id="p1",
                    product_name="Producto A",
                    quantity=15.0,
                    revenue=900.0,
                    share_pct=75.0,
                )
            ],
            low_stock=[
                StockAlertInsight(
                    product_id="p2",
                    product_name="Producto B",
                    sku="B-1",
                    quantity=2.0,
                    min_quantity=5.0,
                    deficit=3.0,
                )
            ],
        )

    async def build_customers_retention_insight(
        self,
        *,
        auth: AuthContext,
        filters: InsightFilters,
    ) -> CustomersRetentionInsight:
        _ = auth
        self.calls.append(("customers_retention", filters))
        return CustomersRetentionInsight(
            period=InsightPeriod(label="month", from_date=date(2026, 3, 1), to_date=date(2026, 3, 31)),
            comparison_period=InsightPeriod(label="previous_period", from_date=date(2026, 2, 1), to_date=date(2026, 2, 28)),
            summary="Resumen de clientes",
            kpis=[
                InsightMetric(
                    key="repeat_rate_pct",
                    label="Tasa de recurrencia",
                    unit="percentage",
                    value=50.0,
                    previous_value=0.0,
                    delta=50.0,
                    delta_pct=None,
                    trend="up",
                )
            ],
            highlights=[InsightHighlight(severity="warning", title="Base inactiva alta", detail="Hay 18 sin actividad.")],
            recommendations=["Activar campaña de reactivación."],
            top_customers=[
                TopCustomerInsight(
                    customer_id="c1",
                    customer_name="Acme",
                    total=450.0,
                    count=4,
                    share_pct=64.29,
                )
            ],
        )


class StubBackendClient:
    def __init__(self) -> None:
        self.calls: list[tuple[str, str, bool, dict[str, object]]] = []

    async def request(self, method: str, path: str, auth=None, *, include_internal: bool = False, **kwargs):
        _ = auth
        payload = kwargs.get("json")
        payload_dict = payload if isinstance(payload, dict) else {}
        self.calls.append((method, path, include_internal, payload_dict))
        return {
            "id": f"persisted-{len(self.calls)}",
            "created_at": "2026-04-01T12:00:00Z",
        }


def create_client(service: StubInsightsService, repo: StubRepo, backend_client: StubBackendClient | None = None) -> TestClient:
    app = FastAPI()
    app.include_router(router)
    app.dependency_overrides[get_repository] = lambda: repo
    app.dependency_overrides[get_auth_context] = lambda: AuthContext(
        tenant_id="org-123",
        actor="user-1",
        role="admin",
        scopes=["reports:read"],
        mode="jwt",
    )
    app.dependency_overrides[get_insights_service] = lambda: service
    app.dependency_overrides[get_backend_client] = lambda: backend_client or StubBackendClient()
    return TestClient(app)


def test_sales_collections_router_returns_response() -> None:
    service = StubInsightsService()
    repo = StubRepo()
    backend_client = StubBackendClient()
    client = create_client(service, repo, backend_client)
    notifications_router_module.get_request_id = lambda: "req-notif-1"  # type: ignore[assignment]

    response = client.post("/v1/notifications", json={"period": "month", "compare": True, "top_limit": 3})

    assert response.status_code == 200
    payload = response.json()
    assert payload["request_id"] == "req-notif-1"
    assert payload["service_kind"] == "insight_service"
    assert payload["output_kind"] == "insight_notification"
    assert payload["content_language"] == "es"
    assert len(payload["items"]) == 3
    assert payload["items"][0]["id"] == "persisted-1"
    assert payload["items"][0]["kind"] == "insight"
    assert payload["items"][0]["entity_type"] == "insight"
    assert payload["items"][0]["entity_id"] == "sales_collections"
    assert payload["items"][0]["content_language"] == "es"
    assert payload["items"][0]["title"] == "Insight de ventas y cobranzas"
    assert payload["items"][0]["body"] == "Este mes: Ventas: $1,000.00 · Cliente destacado: Acme."
    assert payload["items"][0]["chat_context"]["scope"] == "sales_collections"
    assert payload["items"][0]["chat_context"]["routed_agent"] == "insight_chat"
    assert payload["items"][0]["chat_context"]["content_language"] == "es"
    assert service.calls[0][0] == "sales_collections"
    assert service.calls[0][1].period == "month"
    assert service.calls[0][1].top_limit == 3
    assert service.calls[1][0] == "inventory_profit"
    assert service.calls[2][0] == "customers_retention"
    assert repo.tracked_usage == [(0, 0)]
    assert len(backend_client.calls) == 3
    assert backend_client.calls[0][0] == "POST"
    assert backend_client.calls[0][1] == "/v1/internal/v1/in-app-notifications"
    assert backend_client.calls[0][2] is True
    assert backend_client.calls[0][3]["id"] == "insight:sales_collections:month"
    assert backend_client.calls[0][3]["actor"] == "user-1"
    assert backend_client.calls[0][3]["org_id"] == "org-123"


def test_notifications_router_validates_top_limit() -> None:
    service = StubInsightsService()
    client = create_client(service, StubRepo())

    response = client.post("/v1/notifications", json={"top_limit": 0})

    assert response.status_code == 422


def test_notifications_router_supports_period_and_compare() -> None:
    service = StubInsightsService()
    repo = StubRepo()
    backend_client = StubBackendClient()
    client = create_client(service, repo, backend_client)
    notifications_router_module.get_request_id = lambda: "req-notif-2"  # type: ignore[assignment]

    response = client.post("/v1/notifications", json={"period": "week", "compare": False, "top_limit": 2})

    assert response.status_code == 200
    assert response.json()["request_id"] == "req-notif-2"
    assert [call[1].period for call in service.calls] == ["week", "week", "week"]
    assert [call[1].compare for call in service.calls] == [False, False, False]
    assert [call[1].top_limit for call in service.calls] == [2, 2, 2]
    assert repo.tracked_usage == [(0, 0)]
    assert len(backend_client.calls) == 3
