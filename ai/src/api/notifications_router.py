from __future__ import annotations

import asyncio
from datetime import UTC, datetime
from typing import Literal
from uuid import uuid4

from fastapi import APIRouter, Depends, Request
from pydantic import BaseModel, Field

from runtime.logging import get_request_id, get_logger
from src.agents.catalog import COPILOT_AGENT_NAME
from src.api.deps import get_auth_context, get_backend_client, get_repository
from src.api.quota import check_quota
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from src.insights.domain import CustomersRetentionInsight, InsightFilters, InventoryProfitInsight, SalesCollectionsInsight
from src.insights.repository import BackendInsightsRepository
from src.insights.service import InsightsService
from src.localization import LanguageCode, resolve_preferred_language
from src.runtime_contracts import OUTPUT_KIND_INSIGHT_NOTIFICATION, SERVICE_KIND_INSIGHT

router = APIRouter(prefix="/v1/notifications", tags=["notifications"])
logger = get_logger(__name__)


class NotificationsRequest(BaseModel):
    kind: Literal["insight"] = "insight"
    period: Literal["today", "week", "month"] = "month"
    compare: bool = True
    top_limit: int = Field(default=5, ge=1, le=10)
    preferred_language: LanguageCode | None = Field(
        default=None,
        description=(
            "Idioma preferido para el contenido generado por el servicio. Hoy el backend "
            "normaliza sobre `es|en` y, si falta traducción, responde en español."
        ),
    )


class NotificationChatContext(BaseModel):
    suggested_user_message: str
    scope: Literal["sales_collections", "inventory_profit", "customers_retention"]
    routed_agent: Literal["copilot"]
    content_language: LanguageCode = "es"


class NotificationItem(BaseModel):
    id: str
    title: str
    body: str
    kind: Literal["insight"]
    entity_type: Literal["insight"]
    entity_id: str
    content_language: LanguageCode = "es"
    chat_context: NotificationChatContext
    created_at: str


class NotificationsResponse(BaseModel):
    request_id: str
    service_kind: Literal["insight_service"] = Field(default=SERVICE_KIND_INSIGHT)
    output_kind: Literal["insight_notification"] = Field(default=OUTPUT_KIND_INSIGHT_NOTIFICATION)
    content_language: LanguageCode = Field(
        default="es",
        description="Idioma efectivo del contenido visible devuelto por este lote de notificaciones.",
    )
    items: list[NotificationItem] = Field(default_factory=list)


def _period_label(period: str) -> str:
    labels = {
        "today": "hoy",
        "week": "esta semana",
        "month": "este mes",
    }
    return labels.get(period, "este período")


def _format_metric_value(metric) -> str:
    if metric.unit == "currency":
        return f"${metric.value:,.2f}"
    if metric.unit == "percentage":
        return f"{metric.value:.2f}%"
    return f"{int(metric.value):,}"


def _build_notification_body(
    *,
    scope: Literal["sales_collections", "inventory_profit", "customers_retention"],
    insight: SalesCollectionsInsight | InventoryProfitInsight | CustomersRetentionInsight,
    period: str,
) -> str:
    facts = [f"{metric.label}: {_format_metric_value(metric)}" for metric in insight.kpis[:2]]

    if scope == "sales_collections" and insight.top_customers:
        facts.append(f"Cliente destacado: {insight.top_customers[0].customer_name}")
    elif scope == "inventory_profit":
        facts.append(f"Alertas de stock: {len(insight.low_stock)}")
    elif scope == "customers_retention" and insight.top_customers:
        facts.append(f"Cliente recurrente: {insight.top_customers[0].customer_name}")

    if not facts:
        return f"Hay una actualización factual para {_period_label(period)}."
    return f"{_period_label(period).capitalize()}: " + " · ".join(facts) + "."


def _build_notification_item(
    *,
    scope: Literal["sales_collections", "inventory_profit", "customers_retention"],
    title: str,
    insight: SalesCollectionsInsight | InventoryProfitInsight | CustomersRetentionInsight,
    created_at: str,
    period: str,
    content_language: LanguageCode,
) -> NotificationItem:
    return NotificationItem(
        id=f"insight:{scope}:{period}",
        title=title,
        body=_build_notification_body(scope=scope, insight=insight, period=period),
        kind="insight",
        entity_type="insight",
        entity_id=scope,
        content_language=content_language,
        chat_context=NotificationChatContext(
            suggested_user_message=f"Quiero entender {title.lower()} de {_period_label(period)}.",
            scope=scope,
            routed_agent=COPILOT_AGENT_NAME,
            content_language=content_language,
        ),
        created_at=created_at,
    )


def get_insights_service(backend_client: BackendClient = Depends(get_backend_client)) -> InsightsService:
    return InsightsService(BackendInsightsRepository(backend_client))


async def _persist_notification_items(
    backend_client: BackendClient,
    auth: AuthContext,
    items: list[NotificationItem],
) -> list[NotificationItem]:
    async def _persist_one(item: NotificationItem) -> NotificationItem:
        try:
            payload = await backend_client.request(
                "POST",
                "/v1/internal/v1/in-app-notifications",
                include_internal=True,
                json={
                    "id": item.id,
                    "org_id": auth.org_id,
                    "actor": auth.actor,
                    "title": item.title,
                    "body": item.body,
                    "kind": item.kind,
                    "entity_type": item.entity_type,
                    "entity_id": item.entity_id,
                    "chat_context": item.chat_context.model_dump(mode="json"),
                },
            )
        except Exception as exc:  # pragma: no cover - best effort path, covered by request assertions
            logger.warning(
                "notifications_in_app_persist_failed",
                org_id=auth.org_id,
                actor=auth.actor,
                notification_scope=item.entity_id,
                error=str(exc),
            )
            return item

        persisted_id = str(payload.get("id") or item.id)
        created_at = str(payload.get("created_at") or item.created_at)
        return item.model_copy(update={"id": persisted_id, "created_at": created_at})

    return list(await asyncio.gather(*[_persist_one(item) for item in items]))


@router.post("", response_model=NotificationsResponse)
async def create_notifications(
    req: NotificationsRequest,
    request: Request,
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    service: InsightsService = Depends(get_insights_service),
    backend_client: BackendClient = Depends(get_backend_client),
):
    request_id = get_request_id() or str(uuid4())
    preferred_language = resolve_preferred_language(
        req.preferred_language,
        accept_language=request.headers.get("Accept-Language"),
    )
    # Diseño listo para i18n end-to-end: aceptamos preferencia ahora,
    # pero hasta que exista catálogo `en` el contenido efectivo sigue en español.
    effective_content_language: LanguageCode = "es"
    await check_quota(repo, auth.org_id, mode="internal")
    filters = InsightFilters(period=req.period, compare=req.compare, top_limit=req.top_limit)
    sales, inventory, customers = await asyncio.gather(
        service.build_sales_collections_insight(auth=auth, filters=filters),
        service.build_inventory_profit_insight(auth=auth, filters=filters),
        service.build_customers_retention_insight(auth=auth, filters=filters),
    )
    created_at = datetime.now(UTC).isoformat()
    items = [
        _build_notification_item(
            scope="sales_collections",
            title="Insight de ventas y cobranzas",
            insight=sales,
            created_at=created_at,
            period=req.period,
            content_language=effective_content_language,
        ),
        _build_notification_item(
            scope="inventory_profit",
            title="Insight de inventario y rentabilidad",
            insight=inventory,
            created_at=created_at,
            period=req.period,
            content_language=effective_content_language,
        ),
        _build_notification_item(
            scope="customers_retention",
            title="Insight de clientes y retención",
            insight=customers,
            created_at=created_at,
            period=req.period,
            content_language=effective_content_language,
        ),
    ]
    items = await _persist_notification_items(backend_client, auth, items)
    await repo.track_usage(auth.org_id, tokens_in=0, tokens_out=0)
    logger.info(
        "notifications_insights_completed",
        request_id=request_id,
        org_id=auth.org_id,
        actor=auth.actor,
        notifications=len(items),
        period=req.period,
        compare=req.compare,
        preferred_language=preferred_language,
    )
    return NotificationsResponse(
        request_id=request_id,
        service_kind=SERVICE_KIND_INSIGHT,
        output_kind=OUTPUT_KIND_INSIGHT_NOTIFICATION,
        content_language=effective_content_language,
        items=items,
    )
