from __future__ import annotations

import asyncio

from src.backend_client.auth import AuthContext
from src.insights.domain import (
    CashflowSummarySnapshot,
    CustomerBaseSnapshot,
    CustomersRetentionInsight,
    InsightFilters,
    InventoryProfitInsight,
    ProfitMarginSnapshot,
    SalesCollectionsInsight,
    SalesSummarySnapshot,
    TopCustomerSnapshot,
)
from src.insights.repository import InsightsRepository
from src.insights.service_helpers import InsightComputationMixin


class InsightsService(InsightComputationMixin):
    def __init__(self, repository: InsightsRepository) -> None:
        self._repository = repository

    async def build_sales_collections_insight(
        self,
        *,
        auth: AuthContext,
        filters: InsightFilters,
    ) -> SalesCollectionsInsight:
        period = self._resolve_period(filters)
        comparison_period = self._resolve_comparison_period(period) if filters.compare else None

        current_sales, current_cashflow = await asyncio.gather(
            self._repository.get_sales_summary(auth, from_date=period.from_date, to_date=period.to_date),
            self._repository.get_cashflow_summary(auth, from_date=period.from_date, to_date=period.to_date),
        )
        customers, payment_mix, debtors = await asyncio.gather(
            self._optional(
                source="sales_by_customer",
                operation=self._repository.get_sales_by_customer(
                    auth,
                    from_date=period.from_date,
                    to_date=period.to_date,
                ),
                default=[],
            ),
            self._optional(
                source="sales_by_payment",
                operation=self._repository.get_sales_by_payment(
                    auth,
                    from_date=period.from_date,
                    to_date=period.to_date,
                ),
                default=[],
            ),
            self._optional(
                source="debtors",
                operation=self._repository.get_debtors(auth, limit=filters.top_limit),
                default=[],
            ),
        )

        previous_sales: SalesSummarySnapshot | None = None
        previous_cashflow: CashflowSummarySnapshot | None = None
        if comparison_period is not None:
            previous_sales, previous_cashflow = await asyncio.gather(
                self._repository.get_sales_summary(
                    auth,
                    from_date=comparison_period.from_date,
                    to_date=comparison_period.to_date,
                ),
                self._repository.get_cashflow_summary(
                    auth,
                    from_date=comparison_period.from_date,
                    to_date=comparison_period.to_date,
                ),
            )

        top_customers = self._build_top_customers(customers, current_sales.total_sales, filters.top_limit)
        payment_distribution = self._build_payment_mix(payment_mix, filters.top_limit)
        debtor_items = self._build_debtors(debtors, filters.top_limit)
        total_debt = sum(item.total_debt for item in debtor_items)
        top_customer_share = top_customers[0].share_pct if top_customers else 0.0

        kpis = [
            self._metric(
                key="total_sales",
                label="Ventas",
                unit="currency",
                value=current_sales.total_sales,
                previous_value=previous_sales.total_sales if previous_sales is not None else None,
            ),
            self._metric(
                key="sales_count",
                label="Operaciones",
                unit="count",
                value=float(current_sales.count_sales),
                previous_value=float(previous_sales.count_sales) if previous_sales is not None else None,
            ),
            self._metric(
                key="average_ticket",
                label="Ticket promedio",
                unit="currency",
                value=current_sales.average_ticket,
                previous_value=previous_sales.average_ticket if previous_sales is not None else None,
            ),
            self._metric(
                key="cash_balance",
                label="Balance de caja",
                unit="currency",
                value=current_cashflow.balance,
                previous_value=previous_cashflow.balance if previous_cashflow is not None else None,
            ),
            self._metric(
                key="pending_debt",
                label="Deuda pendiente",
                unit="currency",
                value=total_debt,
                previous_value=None,
            ),
        ]

        highlights = self._build_highlights(
            current_sales=current_sales,
            previous_sales=previous_sales,
            current_cashflow=current_cashflow,
            total_debt=total_debt,
            top_customer_share=top_customer_share,
        )
        recommendations = self._build_recommendations(
            current_sales=current_sales,
            previous_sales=previous_sales,
            current_cashflow=current_cashflow,
            total_debt=total_debt,
            top_customer_share=top_customer_share,
        )

        return SalesCollectionsInsight(
            period=period,
            comparison_period=comparison_period,
            summary=self._build_summary(
                period=period,
                current_sales=current_sales,
                previous_sales=previous_sales,
                current_cashflow=current_cashflow,
                total_debt=total_debt,
            ),
            kpis=kpis,
            highlights=highlights,
            recommendations=recommendations,
            top_customers=top_customers,
            payment_mix=payment_distribution,
            debtors=debtor_items,
        )

    async def build_inventory_profit_insight(
        self,
        *,
        auth: AuthContext,
        filters: InsightFilters,
    ) -> InventoryProfitInsight:
        period = self._resolve_period(filters)
        comparison_period = self._resolve_comparison_period(period) if filters.compare else None

        current_margin = await self._repository.get_profit_margin(
            auth,
            from_date=period.from_date,
            to_date=period.to_date,
        )
        inventory_items, low_stock_items, top_products_raw = await asyncio.gather(
            self._optional(
                source="inventory_valuation",
                operation=self._repository.get_inventory_valuation(auth),
                default=[],
            ),
            self._optional(
                source="low_stock",
                operation=self._repository.get_low_stock(auth),
                default=[],
            ),
            self._optional(
                source="sales_by_product",
                operation=self._repository.get_sales_by_product(
                    auth,
                    from_date=period.from_date,
                    to_date=period.to_date,
                ),
                default=[],
            ),
        )

        previous_margin: ProfitMarginSnapshot | None = None
        if comparison_period is not None:
            previous_margin = await self._optional(
                source="previous_profit_margin",
                operation=self._repository.get_profit_margin(
                    auth,
                    from_date=comparison_period.from_date,
                    to_date=comparison_period.to_date,
                ),
                default=ProfitMarginSnapshot(),
            )

        inventory_total = sum(item.valuation for item in inventory_items)
        low_stock_alerts = self._build_low_stock_alerts(low_stock_items, filters.top_limit)
        top_products = self._build_top_products(top_products_raw, filters.top_limit)
        top_product_share = top_products[0].share_pct if top_products else 0.0

        kpis = [
            self._metric(
                key="inventory_valuation",
                label="Valuación de inventario",
                unit="currency",
                value=inventory_total,
                previous_value=None,
            ),
            self._metric(
                key="gross_profit",
                label="Ganancia bruta",
                unit="currency",
                value=current_margin.gross_profit,
                previous_value=previous_margin.gross_profit if previous_margin is not None else None,
            ),
            self._metric(
                key="margin_pct",
                label="Margen bruto",
                unit="percentage",
                value=current_margin.margin_pct,
                previous_value=previous_margin.margin_pct if previous_margin is not None else None,
            ),
            self._metric(
                key="revenue",
                label="Ingresos analizados",
                unit="currency",
                value=current_margin.revenue,
                previous_value=previous_margin.revenue if previous_margin is not None else None,
            ),
            self._metric(
                key="low_stock_count",
                label="Alertas de stock",
                unit="count",
                value=float(len(low_stock_alerts)),
                previous_value=None,
            ),
        ]

        return InventoryProfitInsight(
            period=period,
            comparison_period=comparison_period,
            summary=self._build_inventory_summary(
                period=period,
                current_margin=current_margin,
                previous_margin=previous_margin,
                inventory_total=inventory_total,
                low_stock_count=len(low_stock_alerts),
            ),
            kpis=kpis,
            highlights=self._build_inventory_highlights(
                current_margin=current_margin,
                previous_margin=previous_margin,
                inventory_total=inventory_total,
                low_stock_count=len(low_stock_alerts),
                top_product_share=top_product_share,
            ),
            recommendations=self._build_inventory_recommendations(
                current_margin=current_margin,
                low_stock_count=len(low_stock_alerts),
                top_product_share=top_product_share,
            ),
            top_products=top_products,
            low_stock=low_stock_alerts,
        )

    async def build_customers_retention_insight(
        self,
        *,
        auth: AuthContext,
        filters: InsightFilters,
    ) -> CustomersRetentionInsight:
        period = self._resolve_period(filters)
        comparison_period = self._resolve_comparison_period(period) if filters.compare else None

        customer_base, current_customers = await asyncio.gather(
            self._optional(
                source="customers_total",
                operation=self._repository.get_customers_total(auth),
                default=CustomerBaseSnapshot(),
            ),
            self._optional(
                source="sales_by_customer",
                operation=self._repository.get_sales_by_customer(
                    auth,
                    from_date=period.from_date,
                    to_date=period.to_date,
                ),
                default=[],
            ),
        )

        previous_customers: list[TopCustomerSnapshot] = []
        if comparison_period is not None:
            previous_customers = await self._optional(
                source="previous_sales_by_customer",
                operation=self._repository.get_sales_by_customer(
                    auth,
                    from_date=comparison_period.from_date,
                    to_date=comparison_period.to_date,
                ),
                default=[],
            )

        active_customers = len(current_customers)
        previous_active_customers = len(previous_customers) if comparison_period is not None else None
        repeat_customers = sum(1 for item in current_customers if item.count > 1)
        previous_repeat_customers = (
            sum(1 for item in previous_customers if item.count > 1) if comparison_period is not None else None
        )
        inactive_customers = max(customer_base.total - active_customers, 0)
        repeat_rate = self._share(repeat_customers, active_customers)
        previous_repeat_rate = (
            self._share(previous_repeat_customers or 0, previous_active_customers or 0)
            if comparison_period is not None
            else None
        )
        concentration_pct = self._top_share(current_customers)
        top_customers = self._build_retention_top_customers(current_customers, filters.top_limit)

        kpis = [
            self._metric(
                key="customer_base",
                label="Base de clientes",
                unit="count",
                value=float(customer_base.total),
                previous_value=None,
            ),
            self._metric(
                key="active_customers",
                label="Clientes activos",
                unit="count",
                value=float(active_customers),
                previous_value=float(previous_active_customers) if previous_active_customers is not None else None,
            ),
            self._metric(
                key="repeat_customers",
                label="Clientes recurrentes",
                unit="count",
                value=float(repeat_customers),
                previous_value=float(previous_repeat_customers) if previous_repeat_customers is not None else None,
            ),
            self._metric(
                key="repeat_rate_pct",
                label="Tasa de recurrencia",
                unit="percentage",
                value=repeat_rate,
                previous_value=previous_repeat_rate,
            ),
            self._metric(
                key="inactive_customers",
                label="Clientes sin actividad",
                unit="count",
                value=float(inactive_customers),
                previous_value=None,
            ),
        ]

        return CustomersRetentionInsight(
            period=period,
            comparison_period=comparison_period,
            summary=self._build_customers_summary(
                period=period,
                customer_base=customer_base.total,
                active_customers=active_customers,
                repeat_customers=repeat_customers,
                repeat_rate=repeat_rate,
                previous_repeat_rate=previous_repeat_rate,
                inactive_customers=inactive_customers,
            ),
            kpis=kpis,
            highlights=self._build_customers_highlights(
                customer_base=customer_base.total,
                active_customers=active_customers,
                previous_active_customers=previous_active_customers,
                repeat_rate=repeat_rate,
                inactive_customers=inactive_customers,
                concentration_pct=concentration_pct,
            ),
            recommendations=self._build_customers_recommendations(
                customer_base=customer_base.total,
                repeat_rate=repeat_rate,
                inactive_customers=inactive_customers,
                concentration_pct=concentration_pct,
            ),
            top_customers=top_customers,
        )
