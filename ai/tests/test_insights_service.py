from __future__ import annotations

from datetime import date

import pytest

from runtime.contexts import AuthContext
from src.insights.domain import (
    CashflowSummarySnapshot,
    CustomerBaseSnapshot,
    DebtorSnapshot,
    InsightFilters,
    InventoryValuationSnapshot,
    LowStockSnapshot,
    PaymentMethodSnapshot,
    ProfitMarginSnapshot,
    SalesByProductSnapshot,
    SalesSummarySnapshot,
    TopCustomerSnapshot,
)
from src.insights.service import InsightsService


class StubInsightsRepository:
    async def get_sales_summary(self, _auth: AuthContext, *, from_date: date, to_date: date) -> SalesSummarySnapshot:
        _ = to_date
        if from_date == date(2026, 3, 1):
            return SalesSummarySnapshot(total_sales=1000.0, count_sales=10, average_ticket=100.0)
        return SalesSummarySnapshot(total_sales=800.0, count_sales=8, average_ticket=100.0)

    async def get_cashflow_summary(self, _auth: AuthContext, *, from_date: date, to_date: date) -> CashflowSummarySnapshot:
        _ = to_date
        if from_date == date(2026, 3, 1):
            return CashflowSummarySnapshot(total_income=1200.0, total_expense=900.0, balance=300.0)
        return CashflowSummarySnapshot(total_income=1000.0, total_expense=950.0, balance=50.0)

    async def get_sales_by_customer(self, _auth: AuthContext, *, from_date: date, to_date: date) -> list[TopCustomerSnapshot]:
        _ = from_date, to_date
        return [
            TopCustomerSnapshot(customer_id="c2", customer_name="Beta", total=250.0, count=2),
            TopCustomerSnapshot(customer_id="c1", customer_name="Acme", total=450.0, count=4),
        ]

    async def get_sales_by_payment(self, _auth: AuthContext, *, from_date: date, to_date: date) -> list[PaymentMethodSnapshot]:
        _ = from_date, to_date
        return [
            PaymentMethodSnapshot(payment_method="transfer", total=400.0, count=4),
            PaymentMethodSnapshot(payment_method="cash", total=600.0, count=6),
        ]

    async def get_debtors(self, _auth: AuthContext, *, limit: int) -> list[DebtorSnapshot]:
        return [
            DebtorSnapshot(party_id="p2", party_name="Cliente Dos", total_debt=100.0, oldest_date=date(2026, 2, 20)),
            DebtorSnapshot(party_id="p1", party_name="Cliente Uno", total_debt=350.0, oldest_date=date(2026, 2, 14)),
        ][:limit]

    async def get_sales_by_product(
        self, _auth: AuthContext, *, from_date: date, to_date: date
    ) -> list[SalesByProductSnapshot]:
        _ = from_date, to_date
        return [
            SalesByProductSnapshot(product_id="p2", product_name="Producto B", quantity=5.0, revenue=300.0),
            SalesByProductSnapshot(product_id="p1", product_name="Producto A", quantity=15.0, revenue=900.0),
        ]

    async def get_inventory_valuation(self, _auth: AuthContext) -> list[InventoryValuationSnapshot]:
        return [
            InventoryValuationSnapshot(
                product_id="p1",
                product_name="Producto A",
                sku="A-1",
                quantity=20.0,
                cost_price=30.0,
                valuation=600.0,
            ),
            InventoryValuationSnapshot(
                product_id="p2",
                product_name="Producto B",
                sku="B-1",
                quantity=10.0,
                cost_price=25.0,
                valuation=250.0,
            ),
        ]

    async def get_low_stock(self, _auth: AuthContext) -> list[LowStockSnapshot]:
        return [
            LowStockSnapshot(product_id="p2", product_name="Producto B", sku="B-1", quantity=2.0, min_quantity=5.0)
        ]

    async def get_profit_margin(self, _auth: AuthContext, *, from_date: date, to_date: date) -> ProfitMarginSnapshot:
        _ = to_date
        if from_date == date(2026, 3, 1):
            return ProfitMarginSnapshot(revenue=1200.0, cost=700.0, gross_profit=500.0, margin_pct=41.67)
        return ProfitMarginSnapshot(revenue=1000.0, cost=650.0, gross_profit=350.0, margin_pct=35.0)

    async def get_customers_total(self, _auth: AuthContext) -> CustomerBaseSnapshot:
        return CustomerBaseSnapshot(total=20)


@pytest.mark.asyncio
async def test_build_sales_collections_insight_returns_structured_summary() -> None:
    service = InsightsService(StubInsightsRepository())
    auth = AuthContext(
        tenant_id="org-123",
        actor="user-1",
        role="admin",
        scopes=["reports:read"],
        mode="jwt",
    )

    result = await service.build_sales_collections_insight(
        auth=auth,
        filters=InsightFilters(
            from_date=date(2026, 3, 1),
            to_date=date(2026, 3, 31),
            compare=True,
            top_limit=5,
        ),
    )

    assert result.scope == "sales_collections"
    assert result.period.label == "este período"
    assert result.comparison_period is not None
    assert "variación de +25.0%" in result.summary
    assert result.kpis[0].key == "total_sales"
    assert result.kpis[0].delta_pct == 25.0
    assert result.top_customers[0].customer_name == "Acme"
    assert result.top_customers[0].share_pct == 45.0
    assert result.payment_mix[0].payment_method == "Efectivo"
    assert result.payment_mix[0].share_pct == 60.0
    assert result.debtors[0].party_name == "Cliente Uno"
    assert any(item.title == "Concentración de ingresos" for item in result.highlights)
    assert any("cobranzas" in item.lower() for item in result.recommendations)


@pytest.mark.asyncio
async def test_build_inventory_profit_insight_returns_inventory_and_margin_view() -> None:
    service = InsightsService(StubInsightsRepository())
    auth = AuthContext(
        tenant_id="org-123",
        actor="user-1",
        role="admin",
        scopes=["reports:read"],
        mode="jwt",
    )

    result = await service.build_inventory_profit_insight(
        auth=auth,
        filters=InsightFilters(
            from_date=date(2026, 3, 1),
            to_date=date(2026, 3, 31),
            compare=True,
            top_limit=5,
        ),
    )

    assert result.scope == "inventory_profit"
    assert result.comparison_period is not None
    assert "margen bruto es 41.7%" in result.summary
    assert result.kpis[0].key == "inventory_valuation"
    assert result.kpis[1].delta == 150.0
    assert result.kpis[2].delta_pct == 19.06
    assert result.top_products[0].product_name == "Producto A"
    assert result.top_products[0].share_pct == 75.0
    assert result.low_stock[0].deficit == 3.0
    assert any(item.title == "Reposición pendiente" for item in result.highlights)


@pytest.mark.asyncio
async def test_build_customers_retention_insight_returns_customer_base_and_repeat_rate() -> None:
    service = InsightsService(StubInsightsRepository())
    auth = AuthContext(
        tenant_id="org-123",
        actor="user-1",
        role="admin",
        scopes=["reports:read"],
        mode="jwt",
    )

    result = await service.build_customers_retention_insight(
        auth=auth,
        filters=InsightFilters(
            from_date=date(2026, 3, 1),
            to_date=date(2026, 3, 31),
            compare=True,
            top_limit=5,
        ),
    )

    assert result.scope == "customers_retention"
    assert result.comparison_period is not None
    assert "la base total es de 20 clientes" in result.summary
    assert result.kpis[0].key == "customer_base"
    assert result.kpis[1].value == 2.0
    assert result.kpis[2].value == 2.0
    assert result.kpis[3].value == 100.0
    assert result.kpis[4].value == 18.0
    assert result.top_customers[0].customer_name == "Acme"
    assert any(item.title == "Base inactiva alta" for item in result.highlights)
    assert any("reactivación" in item.lower() for item in result.recommendations)
