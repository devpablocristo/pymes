from __future__ import annotations

from collections.abc import Awaitable
from datetime import UTC, date, datetime, timedelta
from typing import Literal, TypeVar

from runtime.logging import get_logger
from src.insights.domain import (
    CashflowSummarySnapshot,
    DebtorSnapshot,
    DebtorInsight,
    InsightFilters,
    InsightHighlight,
    InsightMetric,
    InsightPeriod,
    LowStockSnapshot,
    PaymentMethodInsight,
    PaymentMethodSnapshot,
    ProductPerformanceInsight,
    ProfitMarginSnapshot,
    SalesByProductSnapshot,
    SalesSummarySnapshot,
    StockAlertInsight,
    TopCustomerInsight,
    TopCustomerSnapshot,
)

logger = get_logger(__name__)
T = TypeVar("T")

_PERIOD_LABELS = {
    "today": "hoy",
    "week": "esta semana",
    "month": "este mes",
    "custom": "este período",
    "previous_period": "el período anterior",
}

_PAYMENT_METHOD_LABELS = {
    "cash": "Efectivo",
    "card": "Tarjeta",
    "credit_card": "Tarjeta de crédito",
    "debit_card": "Tarjeta de débito",
    "transfer": "Transferencia",
    "bank_transfer": "Transferencia bancaria",
    "wallet": "Billetera virtual",
    "mercado_pago": "Mercado Pago",
    "check": "Cheque",
    "account": "Cuenta corriente",
    "account_balance": "Cuenta corriente",
    "other": "Otro",
    "unknown": "Sin especificar",
}


class InsightComputationMixin:
    def _leading_period_phrase(self, period: InsightPeriod) -> str:
        label = period.label.strip()
        if not label:
            return "Este período"
        if label.startswith(("hoy", "esta ", "este ", "el ")):
            return label[:1].upper() + label[1:]
        return f"En {label}"

    def _resolve_period(self, filters: InsightFilters) -> InsightPeriod:
        if filters.from_date is not None and filters.to_date is not None:
            return InsightPeriod(label=_PERIOD_LABELS["custom"], from_date=filters.from_date, to_date=filters.to_date)

        today = datetime.now(UTC).date()
        period_key = (filters.period or "month").strip().lower()
        if period_key == "today":
            start = today
        elif period_key == "week":
            start = today - timedelta(days=today.weekday())
        else:
            period_key = "month"
            start = date(today.year, today.month, 1)
        return InsightPeriod(label=_PERIOD_LABELS[period_key], from_date=start, to_date=today)

    def _resolve_comparison_period(self, period: InsightPeriod) -> InsightPeriod:
        span_days = (period.to_date - period.from_date).days + 1
        comparison_to = period.from_date - timedelta(days=1)
        comparison_from = comparison_to - timedelta(days=span_days - 1)
        return InsightPeriod(
            label=_PERIOD_LABELS["previous_period"],
            from_date=comparison_from,
            to_date=comparison_to,
        )

    def _metric(
        self,
        *,
        key: str,
        label: str,
        unit: Literal["currency", "count", "percentage"],
        value: float,
        previous_value: float | None,
    ) -> InsightMetric:
        delta = None if previous_value is None else value - previous_value
        delta_pct = self._pct_change(value, previous_value)
        return InsightMetric(
            key=key,
            label=label,
            unit=unit,
            value=value,
            previous_value=previous_value,
            delta=delta,
            delta_pct=delta_pct,
            trend=self._trend(delta),
        )

    def _build_top_customers(
        self,
        customers: list[TopCustomerSnapshot],
        total_sales: float,
        limit: int,
    ) -> list[TopCustomerInsight]:
        ranked = sorted(customers, key=lambda item: (item.total, item.count), reverse=True)
        base = total_sales if total_sales > 0 else sum(item.total for item in ranked)
        items: list[TopCustomerInsight] = []
        for customer in ranked[:limit]:
            share = 0.0 if base <= 0 else (customer.total / base) * 100
            items.append(
                TopCustomerInsight(
                    customer_id=customer.customer_id,
                    customer_name=customer.customer_name,
                    total=customer.total,
                    count=customer.count,
                    share_pct=round(share, 2),
                )
            )
        return items

    def _build_payment_mix(
        self,
        payment_mix: list[PaymentMethodSnapshot],
        limit: int,
    ) -> list[PaymentMethodInsight]:
        ranked = sorted(payment_mix, key=lambda item: (item.total, item.count), reverse=True)
        base = sum(item.total for item in ranked)
        items: list[PaymentMethodInsight] = []
        for payment in ranked[:limit]:
            share = 0.0 if base <= 0 else (payment.total / base) * 100
            items.append(
                PaymentMethodInsight(
                    payment_method=self._human_payment_method_label(payment.payment_method),
                    total=payment.total,
                    count=payment.count,
                    share_pct=round(share, 2),
                )
            )
        return items

    def _human_payment_method_label(self, payment_method: str) -> str:
        normalized = payment_method.strip().lower()
        if not normalized:
            return "Sin especificar"
        return _PAYMENT_METHOD_LABELS.get(normalized, payment_method.replace("_", " ").capitalize())

    def _build_debtors(self, debtors: list[DebtorSnapshot], limit: int) -> list[DebtorInsight]:
        ranked = sorted(debtors, key=lambda debtor: (-debtor.total_debt, debtor.oldest_date))
        return [
            DebtorInsight(
                party_id=debtor.party_id,
                party_name=debtor.party_name,
                total_debt=debtor.total_debt,
                oldest_date=debtor.oldest_date,
            )
            for debtor in ranked[:limit]
        ]

    def _build_top_products(
        self,
        products: list[SalesByProductSnapshot],
        limit: int,
    ) -> list[ProductPerformanceInsight]:
        ranked = sorted(products, key=lambda item: (item.revenue, item.quantity), reverse=True)
        base = sum(item.revenue for item in ranked)
        items: list[ProductPerformanceInsight] = []
        for product in ranked[:limit]:
            share = 0.0 if base <= 0 else (product.revenue / base) * 100
            items.append(
                ProductPerformanceInsight(
                    product_id=product.product_id,
                    product_name=product.product_name,
                    quantity=product.quantity,
                    revenue=product.revenue,
                    share_pct=round(share, 2),
                )
            )
        return items

    def _build_low_stock_alerts(
        self,
        low_stock_items: list[LowStockSnapshot],
        limit: int,
    ) -> list[StockAlertInsight]:
        return [
            StockAlertInsight(
                product_id=item.product_id,
                product_name=item.product_name,
                sku=item.sku,
                quantity=item.quantity,
                min_quantity=item.min_quantity,
                deficit=max(item.min_quantity - item.quantity, 0.0),
            )
            for item in low_stock_items[:limit]
        ]

    def _build_retention_top_customers(
        self,
        customers: list[TopCustomerSnapshot],
        limit: int,
    ) -> list[TopCustomerInsight]:
        ranked = sorted(customers, key=lambda item: (item.count, item.total), reverse=True)
        base = sum(item.total for item in ranked)
        items: list[TopCustomerInsight] = []
        for customer in ranked[:limit]:
            share = 0.0 if base <= 0 else (customer.total / base) * 100
            items.append(
                TopCustomerInsight(
                    customer_id=customer.customer_id,
                    customer_name=customer.customer_name,
                    total=customer.total,
                    count=customer.count,
                    share_pct=round(share, 2),
                )
            )
        return items

    def _build_summary(
        self,
        *,
        period: InsightPeriod,
        current_sales: SalesSummarySnapshot,
        previous_sales: SalesSummarySnapshot | None,
        current_cashflow: CashflowSummarySnapshot,
        total_debt: float,
    ) -> str:
        sales_delta_pct = self._pct_change(current_sales.total_sales, previous_sales.total_sales if previous_sales else None)
        delta_fragment = ""
        if sales_delta_pct is not None:
            delta_fragment = f", con una variación de {sales_delta_pct:+.1f}% vs el período anterior"
        return (
            f"{self._leading_period_phrase(period)} acumulás {self._money(current_sales.total_sales)} en ventas "
            f"repartidas en {current_sales.count_sales} operaciones, con ticket promedio de "
            f"{self._money(current_sales.average_ticket)}{delta_fragment}. "
            f"El balance de caja del período es {self._money(current_cashflow.balance)} "
            f"y la deuda pendiente abierta suma {self._money(total_debt)}."
        )

    def _build_inventory_summary(
        self,
        *,
        period: InsightPeriod,
        current_margin: ProfitMarginSnapshot,
        previous_margin: ProfitMarginSnapshot | None,
        inventory_total: float,
        low_stock_count: int,
    ) -> str:
        margin_delta_pct = self._pct_change(
            current_margin.margin_pct,
            previous_margin.margin_pct if previous_margin is not None else None,
        )
        delta_fragment = ""
        if margin_delta_pct is not None:
            delta_fragment = f", con variación de {margin_delta_pct:+.1f}% vs el período anterior"
        return (
            f"{self._leading_period_phrase(period)} el margen bruto es {current_margin.margin_pct:.1f}% "
            f"sobre ingresos por {self._money(current_margin.revenue)}, con ganancia bruta de "
            f"{self._money(current_margin.gross_profit)}{delta_fragment}. "
            f"La valuación actual del inventario es {self._money(inventory_total)} "
            f"y hay {low_stock_count} alertas de stock."
        )

    def _build_customers_summary(
        self,
        *,
        period: InsightPeriod,
        customer_base: int,
        active_customers: int,
        repeat_customers: int,
        repeat_rate: float,
        previous_repeat_rate: float | None,
        inactive_customers: int,
    ) -> str:
        delta_fragment = ""
        repeat_rate_delta = self._pct_change(repeat_rate, previous_repeat_rate)
        if repeat_rate_delta is not None:
            delta_fragment = f", con variación de {repeat_rate_delta:+.1f}% vs el período anterior"
        return (
            f"{self._leading_period_phrase(period)} la base total es de {customer_base} clientes. "
            f"Estuvieron activos {active_customers}, de los cuales {repeat_customers} son recurrentes "
            f"({repeat_rate:.1f}%){delta_fragment}. "
            f"Quedan {inactive_customers} clientes sin actividad en el período."
        )

    def _build_highlights(
        self,
        *,
        current_sales: SalesSummarySnapshot,
        previous_sales: SalesSummarySnapshot | None,
        current_cashflow: CashflowSummarySnapshot,
        total_debt: float,
        top_customer_share: float,
    ) -> list[InsightHighlight]:
        highlights: list[InsightHighlight] = []
        sales_delta_pct = self._pct_change(current_sales.total_sales, previous_sales.total_sales if previous_sales else None)
        if sales_delta_pct is not None:
            if sales_delta_pct >= 10:
                highlights.append(
                    InsightHighlight(
                        severity="positive",
                        title="Ventas en crecimiento",
                        detail=f"Las ventas subieron {sales_delta_pct:.1f}% contra el período comparable.",
                    )
                )
            elif sales_delta_pct <= -10:
                highlights.append(
                    InsightHighlight(
                        severity="warning",
                        title="Caída de ventas",
                        detail=f"Las ventas bajaron {abs(sales_delta_pct):.1f}% contra el período comparable.",
                    )
                )

        if current_cashflow.balance < 0:
            highlights.append(
                InsightHighlight(
                    severity="warning",
                    title="Caja en negativo",
                    detail=f"El período cierra con balance {self._money(current_cashflow.balance)}.",
                )
            )
        else:
            highlights.append(
                InsightHighlight(
                    severity="info",
                    title="Caja positiva",
                    detail=f"El balance operativo del período es {self._money(current_cashflow.balance)}.",
                )
            )

        if total_debt > 0:
            highlights.append(
                InsightHighlight(
                    severity="warning" if total_debt >= current_sales.total_sales * 0.3 else "info",
                    title="Exposición a cobranzas",
                    detail=f"La deuda pendiente representa {self._ratio_pct(total_debt, current_sales.total_sales):.1f}% de las ventas del período.",
                )
            )

        if top_customer_share >= 35:
            highlights.append(
                InsightHighlight(
                    severity="warning",
                    title="Concentración de ingresos",
                    detail=f"El principal cliente explica {top_customer_share:.1f}% de la facturación del período.",
                )
            )

        return highlights[:4]

    def _build_inventory_highlights(
        self,
        *,
        current_margin: ProfitMarginSnapshot,
        previous_margin: ProfitMarginSnapshot | None,
        inventory_total: float,
        low_stock_count: int,
        top_product_share: float,
    ) -> list[InsightHighlight]:
        highlights: list[InsightHighlight] = []
        margin_delta_pct = self._pct_change(
            current_margin.margin_pct,
            previous_margin.margin_pct if previous_margin is not None else None,
        )
        if margin_delta_pct is not None:
            if margin_delta_pct >= 10:
                highlights.append(
                    InsightHighlight(
                        severity="positive",
                        title="Margen en mejora",
                        detail=f"El margen bruto mejoró {margin_delta_pct:.1f}% frente al período comparable.",
                    )
                )
            elif margin_delta_pct <= -10:
                highlights.append(
                    InsightHighlight(
                        severity="warning",
                        title="Margen deteriorado",
                        detail=f"El margen bruto cayó {abs(margin_delta_pct):.1f}% frente al período comparable.",
                    )
                )

        if current_margin.margin_pct < 20:
            highlights.append(
                InsightHighlight(
                    severity="warning",
                    title="Rentabilidad ajustada",
                    detail=f"El margen bruto actual es {current_margin.margin_pct:.1f}%, por debajo de un umbral saludable.",
                )
            )
        else:
            highlights.append(
                InsightHighlight(
                    severity="info",
                    title="Inventario valorizado",
                    detail=f"El stock actual representa {self._money(inventory_total)} inmovilizados en inventario.",
                )
            )

        if low_stock_count > 0:
            highlights.append(
                InsightHighlight(
                    severity="warning",
                    title="Reposición pendiente",
                    detail=f"Hay {low_stock_count} productos por debajo del stock mínimo.",
                )
            )

        if top_product_share >= 45:
            highlights.append(
                InsightHighlight(
                    severity="warning",
                    title="Dependencia de un producto",
                    detail=f"El producto líder concentra {top_product_share:.1f}% de la facturación analizada.",
                )
            )

        return highlights[:4]

    def _build_customers_highlights(
        self,
        *,
        customer_base: int,
        active_customers: int,
        previous_active_customers: int | None,
        repeat_rate: float,
        inactive_customers: int,
        concentration_pct: float,
    ) -> list[InsightHighlight]:
        highlights: list[InsightHighlight] = []
        active_delta_pct = self._pct_change(float(active_customers), float(previous_active_customers) if previous_active_customers is not None else None)
        if active_delta_pct is not None:
            if active_delta_pct >= 10:
                highlights.append(
                    InsightHighlight(
                        severity="positive",
                        title="Más clientes activos",
                        detail=f"La actividad de clientes subió {active_delta_pct:.1f}% frente al período comparable.",
                    )
                )
            elif active_delta_pct <= -10:
                highlights.append(
                    InsightHighlight(
                        severity="warning",
                        title="Menos clientes activos",
                        detail=f"La actividad de clientes cayó {abs(active_delta_pct):.1f}% frente al período comparable.",
                    )
                )

        if repeat_rate >= 40:
            highlights.append(
                InsightHighlight(
                    severity="positive",
                    title="Buena recurrencia",
                    detail=f"La tasa de recurrencia actual es {repeat_rate:.1f}% de los clientes activos.",
                )
            )
        elif repeat_rate <= 20 and active_customers > 0:
            highlights.append(
                InsightHighlight(
                    severity="warning",
                    title="Recurrencia baja",
                    detail=f"Solo {repeat_rate:.1f}% de los clientes activos repitieron compra en el período.",
                )
            )

        inactive_share = self._share(inactive_customers, customer_base)
        if inactive_share >= 50 and customer_base > 0:
            highlights.append(
                InsightHighlight(
                    severity="warning",
                    title="Base inactiva alta",
                    detail=f"El {inactive_share:.1f}% de la base no tuvo actividad en el período.",
                )
            )

        if concentration_pct >= 35:
            highlights.append(
                InsightHighlight(
                    severity="warning",
                    title="Facturación concentrada",
                    detail=f"El principal cliente concentra {concentration_pct:.1f}% del ingreso entre clientes activos.",
                )
            )

        return highlights[:4]

    def _build_recommendations(
        self,
        *,
        current_sales: SalesSummarySnapshot,
        previous_sales: SalesSummarySnapshot | None,
        current_cashflow: CashflowSummarySnapshot,
        total_debt: float,
        top_customer_share: float,
    ) -> list[str]:
        recommendations: list[str] = []
        sales_delta_pct = self._pct_change(current_sales.total_sales, previous_sales.total_sales if previous_sales else None)

        if sales_delta_pct is not None and sales_delta_pct <= -10:
            recommendations.append("Revisar la caída de ventas por canal, vendedor o producto para frenar el desvío esta semana.")
        if current_cashflow.balance < 0:
            recommendations.append("Priorizar egresos críticos y reprogramar pagos no urgentes hasta recuperar caja positiva.")
        if total_debt >= current_sales.total_sales * 0.3 and total_debt > 0:
            recommendations.append("Activar una campaña corta de cobranzas sobre los deudores principales para bajar exposición.")
        if top_customer_share >= 35:
            recommendations.append("Diversificar la facturación con acciones comerciales sobre clientes medianos para reducir concentración.")
        if not recommendations:
            recommendations.append("Mantener seguimiento semanal de ventas, caja y deudores para sostener la tendencia actual.")

        return recommendations[:4]

    def _build_inventory_recommendations(
        self,
        *,
        current_margin: ProfitMarginSnapshot,
        low_stock_count: int,
        top_product_share: float,
    ) -> list[str]:
        recommendations: list[str] = []
        if current_margin.margin_pct < 20:
            recommendations.append("Revisar costos y precios de los productos más vendidos para recuperar margen bruto.")
        if low_stock_count > 0:
            recommendations.append("Priorizar reposición de los productos críticos para evitar quiebres de stock.")
        if top_product_share >= 45:
            recommendations.append("Desarrollar demanda en productos secundarios para reducir dependencia del SKU líder.")
        if not recommendations:
            recommendations.append("Mantener control semanal de margen, rotación y alertas de stock para sostener rentabilidad.")
        return recommendations[:4]

    def _build_customers_recommendations(
        self,
        *,
        customer_base: int,
        repeat_rate: float,
        inactive_customers: int,
        concentration_pct: float,
    ) -> list[str]:
        recommendations: list[str] = []
        if repeat_rate <= 20 and customer_base > 0:
            recommendations.append("Lanzar una acción de fidelización sobre clientes de una sola compra para mejorar recurrencia.")
        if inactive_customers > max(customer_base * 0.4, 10):
            recommendations.append("Activar campañas de reactivación sobre la base inactiva del período.")
        if concentration_pct >= 35:
            recommendations.append("Expandir ventas sobre clientes medios para reducir dependencia del principal cliente.")
        if not recommendations:
            recommendations.append("Mantener seguimiento mensual de clientes activos, repetición y reactivación.")
        return recommendations[:4]

    @staticmethod
    def _pct_change(value: float, previous_value: float | None) -> float | None:
        if previous_value is None:
            return None
        if abs(previous_value) < 1e-9:
            if abs(value) < 1e-9:
                return 0.0
            return None
        return round(((value - previous_value) / abs(previous_value)) * 100, 2)

    @staticmethod
    def _ratio_pct(value: float, base: float) -> float:
        if abs(base) < 1e-9:
            return 0.0
        return round((value / base) * 100, 2)

    @staticmethod
    def _share(value: int | float, base: int | float) -> float:
        if abs(float(base)) < 1e-9:
            return 0.0
        return round((float(value) / float(base)) * 100, 2)

    @staticmethod
    def _top_share(customers: list[TopCustomerSnapshot]) -> float:
        if not customers:
            return 0.0
        total = sum(item.total for item in customers)
        if abs(total) < 1e-9:
            return 0.0
        top = max(item.total for item in customers)
        return round((top / total) * 100, 2)

    @staticmethod
    def _trend(delta: float | None) -> str:
        if delta is None:
            return "unknown"
        if abs(delta) < 1e-9:
            return "flat"
        return "up" if delta > 0 else "down"

    @staticmethod
    def _money(value: float) -> str:
        return f"${value:,.2f}"

    async def _optional(self, *, source: str, operation: Awaitable[T], default: T) -> T:
        try:
            return await operation
        except Exception as exc:  # noqa: BLE001
            logger.warning("insights_optional_source_failed", source=source, error=str(exc))
            return default
