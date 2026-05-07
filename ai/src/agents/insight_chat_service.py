from __future__ import annotations

from datetime import UTC, datetime
from dataclasses import dataclass
from typing import Any, Literal

from pydantic import BaseModel, Field
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from runtime.chat.blocks import (
    build_insight_card_block,
    build_kpi_group_block,
    build_table_block,
)
from src.insights.domain import (
    CustomersRetentionInsight,
    InsightFilters,
    InsightMetric,
    InsightPeriod,
    InventoryProfitInsight,
    SalesCollectionsInsight,
)
from src.insights.repository import BackendInsightsRepository
from src.insights.service import InsightsService
from runtime.logging import get_logger

logger = get_logger(__name__)

@dataclass(frozen=True)
class InsightChatResponse:
    reply: str
    blocks: list[dict[str, Any]]
    insight_evidence: InternalInsightEvidence | None = None


ResolvedInsight = SalesCollectionsInsight | InventoryProfitInsight | CustomersRetentionInsight


class InsightEvidencePeriod(BaseModel):
    label: str
    from_date: str
    to_date: str


class InsightEvidenceKPI(BaseModel):
    key: str
    label: str
    unit: Literal["currency", "count", "percentage"]
    value: float
    previous_value: float | None = None
    delta: float | None = None
    delta_pct: float | None = None
    trend: Literal["up", "down", "flat", "unknown"] = "unknown"


class InsightEvidenceHighlight(BaseModel):
    severity: Literal["positive", "info", "warning"]
    title: str
    detail: str


class InternalInsightEvidence(BaseModel):
    source: Literal["insight_handoff"] = "insight_handoff"
    notification_id: str | None = None
    scope: Literal["sales_collections", "inventory_profit", "customers_retention"]
    period: Literal["today", "week", "month"]
    compare: bool = True
    top_limit: int = Field(default=5, ge=1, le=10)
    computed_at: str
    summary: str
    current_period: InsightEvidencePeriod
    comparison_period: InsightEvidencePeriod | None = None
    kpis: list[InsightEvidenceKPI] = Field(default_factory=list)
    highlights: list[InsightEvidenceHighlight] = Field(default_factory=list)
    recommendations: list[str] = Field(default_factory=list)
    entity_ids: list[str] = Field(default_factory=list)


def _format_scope_label(scope: str, filters: InsightFilters) -> str:
    scope_labels = {
        "sales_collections": "Ventas y cobranzas",
        "inventory_profit": "Inventario y rentabilidad",
        "customers_retention": "Clientes y retención",
    }
    period_labels = {
        "today": "hoy",
        "week": "esta semana",
        "month": "este mes",
    }
    return f"{scope_labels.get(scope, 'Insight')} · {period_labels.get(filters.period, 'este período')}"


def _format_metric_value(metric: InsightMetric) -> str:
    if metric.unit == "currency":
        return f"${metric.value:,.2f}"
    if metric.unit == "percentage":
        return f"{metric.value:.1f}%"
    return f"{metric.value:,.0f}"


def _format_metric_context(metric: InsightMetric) -> str | None:
    if metric.delta_pct is None:
        return None
    return f"{metric.delta_pct:+.1f}% vs período anterior"


def _build_kpi_items(metrics: list[InsightMetric]) -> list[dict[str, str]]:
    items: list[dict[str, str]] = []
    for metric in metrics:
        item: dict[str, str] = {
            "label": metric.label,
            "value": _format_metric_value(metric),
        }
        if metric.trend:
            item["trend"] = metric.trend
        context = _format_metric_context(metric)
        if context:
            item["context"] = context
        items.append(item)
    return items


def _build_sales_collections_blocks(insight: SalesCollectionsInsight, filters: InsightFilters) -> list[dict[str, Any]]:
    return [
        build_insight_card_block(
            title="Ventas y cobranzas",
            summary=insight.summary,
            scope=_format_scope_label(insight.scope, filters),
            highlights=[{"label": metric.label, "value": _format_metric_value(metric)} for metric in insight.kpis[:3]],
            recommendations=insight.recommendations,
        ),
        build_kpi_group_block(title="KPIs clave", items=_build_kpi_items(insight.kpis)),
        build_table_block(
            title="Top clientes",
            columns=["Cliente", "Total", "Operaciones", "Participación"],
            rows=[
                [item.customer_name, f"${item.total:,.2f}", str(item.count), f"{item.share_pct:.1f}%"]
                for item in insight.top_customers
            ],
            empty_state="No hay clientes destacados para este período.",
        ),
        build_table_block(
            title="Mix de cobros",
            columns=["Medio", "Total", "Operaciones", "Participación"],
            rows=[
                [item.payment_method, f"${item.total:,.2f}", str(item.count), f"{item.share_pct:.1f}%"]
                for item in insight.payment_mix
            ],
            empty_state="No hay medios de cobro registrados para este período.",
        ),
        build_table_block(
            title="Deudores",
            columns=["Cliente", "Deuda", "Más antigua"],
            rows=[
                [item.party_name, f"${item.total_debt:,.2f}", item.oldest_date.isoformat() if item.oldest_date else "-"]
                for item in insight.debtors
            ],
            empty_state="No hay deuda pendiente abierta.",
        ),
    ]


def _build_inventory_profit_blocks(insight: InventoryProfitInsight, filters: InsightFilters) -> list[dict[str, Any]]:
    return [
        build_insight_card_block(
            title="Inventario y rentabilidad",
            summary=insight.summary,
            scope=_format_scope_label(insight.scope, filters),
            highlights=[{"label": metric.label, "value": _format_metric_value(metric)} for metric in insight.kpis[:3]],
            recommendations=insight.recommendations,
        ),
        build_kpi_group_block(title="KPIs clave", items=_build_kpi_items(insight.kpis)),
        build_table_block(
            title="Productos con mejor desempeño",
            columns=["Producto", "Ingresos", "Cantidad", "Participación"],
            rows=[
                [item.product_name, f"${item.revenue:,.2f}", f"{item.quantity:,.0f}", f"{item.share_pct:.1f}%"]
                for item in insight.top_products
            ],
            empty_state="No hay productos destacados para este período.",
        ),
        build_table_block(
            title="Alertas de stock",
            columns=["Producto", "Stock", "Mínimo", "Faltante"],
            rows=[
                [item.product_name, f"{item.quantity:,.0f}", f"{item.min_quantity:,.0f}", f"{item.deficit:,.0f}"]
                for item in insight.low_stock
            ],
            empty_state="No hay alertas de stock.",
        ),
    ]


def _build_customers_retention_blocks(insight: CustomersRetentionInsight, filters: InsightFilters) -> list[dict[str, Any]]:
    return [
        build_insight_card_block(
            title="Clientes y retención",
            summary=insight.summary,
            scope=_format_scope_label(insight.scope, filters),
            highlights=[{"label": metric.label, "value": _format_metric_value(metric)} for metric in insight.kpis[:3]],
            recommendations=insight.recommendations,
        ),
        build_kpi_group_block(title="KPIs clave", items=_build_kpi_items(insight.kpis)),
        build_table_block(
            title="Clientes con más recurrencia",
            columns=["Cliente", "Total", "Compras", "Participación"],
            rows=[
                [item.customer_name, f"${item.total:,.2f}", str(item.count), f"{item.share_pct:.1f}%"]
                for item in insight.top_customers
            ],
            empty_state="No hay clientes destacados para este período.",
        ),
    ]


def _serialize_period(period: InsightPeriod | None) -> InsightEvidencePeriod | None:
    if period is None:
        return None
    return InsightEvidencePeriod(
        label=period.label,
        from_date=period.from_date.isoformat(),
        to_date=period.to_date.isoformat(),
    )


def _collect_insight_entity_ids(insight: ResolvedInsight) -> list[str]:
    def _item_value(item: Any, key: str) -> str | None:
        if isinstance(item, dict):
            value = item.get(key)
        else:
            value = getattr(item, key, None)
        if isinstance(value, str) and value.strip():
            return value
        return None

    entity_ids: list[str] = []
    if isinstance(insight, (SalesCollectionsInsight, CustomersRetentionInsight)):
        for item in insight.top_customers:
            if customer_id := _item_value(item, "customer_id"):
                if customer_id not in entity_ids:
                    entity_ids.append(customer_id)
    if isinstance(insight, InventoryProfitInsight):
        for item in insight.top_products:
            if product_id := _item_value(item, "product_id"):
                if product_id not in entity_ids:
                    entity_ids.append(product_id)
        for item in insight.low_stock:
            if product_id := _item_value(item, "product_id"):
                if product_id not in entity_ids:
                    entity_ids.append(product_id)
    if isinstance(insight, SalesCollectionsInsight):
        for item in insight.debtors:
            if party_id := _item_value(item, "party_id"):
                if party_id not in entity_ids:
                    entity_ids.append(party_id)
    return entity_ids


def build_internal_insight_evidence(
    *,
    insight: ResolvedInsight,
    filters: InsightFilters,
    notification_id: str | None = None,
    computed_at: str | None = None,
) -> InternalInsightEvidence:
    return InternalInsightEvidence(
        notification_id=notification_id,
        scope=insight.scope,
        period=filters.period,
        compare=filters.compare,
        top_limit=filters.top_limit,
        computed_at=computed_at or datetime.now(UTC).isoformat(),
        summary=insight.summary,
        current_period=_serialize_period(insight.period),
        comparison_period=_serialize_period(insight.comparison_period),
        kpis=[InsightEvidenceKPI.model_validate(metric.model_dump(mode="json")) for metric in insight.kpis],
        highlights=[InsightEvidenceHighlight.model_validate(item.model_dump(mode="json")) for item in insight.highlights],
        recommendations=list(insight.recommendations),
        entity_ids=_collect_insight_entity_ids(insight),
    )


def _build_insight_chat_response_from_insight(*, insight: ResolvedInsight, filters: InsightFilters) -> InsightChatResponse:
    return _build_insight_chat_response_with_evidence(
        insight=insight,
        filters=filters,
        notification_id=None,
    )


def _build_insight_chat_response_with_evidence(
    *,
    insight: ResolvedInsight,
    filters: InsightFilters,
    notification_id: str | None,
) -> InsightChatResponse:
    if isinstance(insight, SalesCollectionsInsight):
        blocks = _build_sales_collections_blocks(insight, filters)
    elif isinstance(insight, InventoryProfitInsight):
        blocks = _build_inventory_profit_blocks(insight, filters)
    else:
        blocks = _build_customers_retention_blocks(insight, filters)
    return InsightChatResponse(
        reply=insight.summary,
        blocks=blocks,
        insight_evidence=build_internal_insight_evidence(
            insight=insight,
            filters=filters,
            notification_id=notification_id,
        ),
    )


async def build_insight_chat_response_for_scope(
    *,
    backend_client: BackendClient,
    auth: AuthContext,
    scope: str,
    period: str,
    compare: bool,
    top_limit: int,
    notification_id: str | None = None,
) -> InsightChatResponse | None:
    filters = InsightFilters(period=period, compare=compare, top_limit=top_limit)
    service = InsightsService(BackendInsightsRepository(backend_client))

    try:
        if scope == "sales_collections":
            insight = await service.build_sales_collections_insight(auth=auth, filters=filters)
            return _build_insight_chat_response_with_evidence(
                insight=insight,
                filters=filters,
                notification_id=notification_id,
            )
        if scope == "inventory_profit":
            insight = await service.build_inventory_profit_insight(auth=auth, filters=filters)
            return _build_insight_chat_response_with_evidence(
                insight=insight,
                filters=filters,
                notification_id=notification_id,
            )
        if scope == "customers_retention":
            insight = await service.build_customers_retention_insight(auth=auth, filters=filters)
            return _build_insight_chat_response_with_evidence(
                insight=insight,
                filters=filters,
                notification_id=notification_id,
            )
    except Exception as exc:  # noqa: BLE001
        logger.warning("insight_chat_failed", tenant_id=auth.tenant_id, error=str(exc))
        return None

    return None
