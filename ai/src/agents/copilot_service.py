from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.chat_blocks import (
    build_insight_card_block,
    build_kpi_group_block,
    build_table_block,
)
from src.insights.domain import (
    CustomersRetentionInsight,
    InsightFilters,
    InsightMetric,
    InventoryProfitInsight,
    SalesCollectionsInsight,
)
from src.insights.repository import BackendInsightsRepository
from src.insights.service import InsightsService
from runtime.logging import get_logger

logger = get_logger(__name__)

_ANALYTICS_HINTS = (
    "insight",
    "insights",
    "como viene",
    "cómo viene",
    "como van",
    "cómo van",
    "como vamos",
    "cómo vamos",
    "entender",
    "explica",
    "explicame",
    "explicá",
    "explicar",
    "resumi",
    "resumí",
    "resumen",
    "analiza",
    "analizá",
    "analisis",
    "análisis",
    "indicador",
    "indicadores",
    "kpi",
    "kpis",
    "metric",
    "métrica",
    "métricas",
    "tendencia",
    "tendencias",
    "performance",
    "rendimiento",
    "panorama",
    "reporte",
    "reportes",
    "salud del negocio",
    "resumen general",
    "panorama general",
    "estado",
)
_PERIOD_HINTS = (" hoy", " mes", " semana", " mensual", " semanal", " diario", " diaria")
_GENERIC_BUSINESS_SCOPE_HINTS = (
    "negocio",
    "empresa",
    "comercio",
    "local",
    "operacion",
    "operación",
    "resultados",
    "resultado",
    "general",
    "global",
)
_OPERATIONAL_HINTS = (
    "crear ",
    "crea ",
    "registr",
    "cargar ",
    "agregar ",
    "actualizar ",
    "modificar ",
    "eliminar ",
    "borrar ",
    "vender ",
    "cobrar ",
    "comprar ",
    "emitir ",
    "generar ",
    "armar ",
    "hacer ",
)
_SPECIFIC_STATUS_HINTS = (
    "estado del",
    "estado de la",
    "estado de ",
    "seguimiento del",
    "seguimiento de la",
    "seguimiento de ",
)
_SPECIFIC_ENTITY_HINTS = (
    " cobro ",
    " pago ",
    " venta ",
    " presupuesto ",
    " factura ",
    " orden ",
    " compra ",
    " solicitud ",
)
_REFERENCE_HINTS = ("#", " id ", " nro ", " n° ", " numero ", " número ")


@dataclass(frozen=True)
class CopilotResponse:
    reply: str
    blocks: list[dict[str, Any]]


@dataclass(frozen=True)
class _InsightRequest:
    scope: str
    period: str
    compare: bool


def _has_analytics_intent(text: str) -> bool:
    padded = f" {text} "
    if any(hint in padded for hint in _ANALYTICS_HINTS):
        return True
    return any(hint in padded for hint in _PERIOD_HINTS)


def _has_operational_intent(text: str) -> bool:
    padded = f" {text} "
    return any(hint in padded for hint in _OPERATIONAL_HINTS)


def _looks_like_specific_status_request(text: str) -> bool:
    padded = f" {text} "
    if not any(hint in padded for hint in _SPECIFIC_STATUS_HINTS):
        return False
    if not any(entity in padded for entity in _SPECIFIC_ENTITY_HINTS):
        return False
    return True


def _match_insight_request(message: str) -> _InsightRequest | None:
    text = message.strip().lower()
    if not text:
        return None
    if _has_operational_intent(text) and not _has_analytics_intent(text):
        return None
    if _looks_like_specific_status_request(text):
        return None
    if not _has_analytics_intent(text):
        return None

    period = "month"
    if " hoy" in f" {text}" or text.startswith("hoy"):
        period = "today"
    elif "semana" in text or "semanal" in text:
        period = "week"

    compare = not any(flag in text for flag in ("sin comparar", "sin comparacion", "sin comparación"))

    if "cliente" in text and any(
        hint in text
        for hint in ("retencion", "retención", "recurrencia", "recurrent", "fideliz", "reactiv", "churn")
    ):
        return _InsightRequest(scope="customers_retention", period=period, compare=compare)

    if any(hint in text for hint in ("inventario", "stock", "margen", "rentabilidad")):
        return _InsightRequest(scope="inventory_profit", period=period, compare=compare)

    if any(
        hint in text
        for hint in ("ventas", "venta", "cobros", "cobro", "cobranza", "cobranzas", "caja", "facturacion", "facturación", "deuda", "deudores")
    ):
        return _InsightRequest(scope="sales_collections", period=period, compare=compare)

    if any(hint in text for hint in _GENERIC_BUSINESS_SCOPE_HINTS):
        return _InsightRequest(scope="sales_collections", period=period, compare=compare)

    return None


def looks_like_copilot_insight_request(message: str) -> bool:
    return _match_insight_request(message) is not None


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


async def maybe_build_copilot_response(
    *,
    backend_client: BackendClient,
    auth: AuthContext,
    user_message: str,
    preferred_language: str | None = None,
) -> CopilotResponse | None:
    _ = preferred_language
    request = _match_insight_request(user_message)
    if request is None:
        return None

    filters = InsightFilters(period=request.period, compare=request.compare, top_limit=5)
    service = InsightsService(BackendInsightsRepository(backend_client))

    try:
        if request.scope == "sales_collections":
            insight = await service.build_sales_collections_insight(auth=auth, filters=filters)
            return CopilotResponse(reply=insight.summary, blocks=_build_sales_collections_blocks(insight, filters))
        if request.scope == "inventory_profit":
            insight = await service.build_inventory_profit_insight(auth=auth, filters=filters)
            return CopilotResponse(reply=insight.summary, blocks=_build_inventory_profit_blocks(insight, filters))
        if request.scope == "customers_retention":
            insight = await service.build_customers_retention_insight(auth=auth, filters=filters)
            return CopilotResponse(reply=insight.summary, blocks=_build_customers_retention_blocks(insight, filters))
    except Exception as exc:  # noqa: BLE001
        logger.warning("internal_copilot_failed", org_id=auth.org_id, error=str(exc))
        return None

    return None
