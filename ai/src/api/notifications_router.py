from __future__ import annotations

import asyncio
from datetime import UTC, datetime
from typing import Literal
from uuid import uuid4

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field

from runtime import OUTPUT_KIND_INSIGHT_NOTIFICATION, SERVICE_KIND_INSIGHT
from runtime.logging import get_request_id, get_logger
from src.agents.catalog import COPILOT_AGENT_NAME
from src.api.deps import get_auth_context, get_backend_client, get_repository
from src.api.router import check_quota
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from src.insights.domain import CustomersRetentionInsight, InsightFilters, InventoryProfitInsight, SalesCollectionsInsight
from src.insights.repository import BackendInsightsRepository
from src.insights.service import InsightsService

router = APIRouter(prefix="/v1/notifications", tags=["notifications"])
logger = get_logger(__name__)


class NotificationsRequest(BaseModel):
    kind: Literal["insight"] = "insight"
    period: Literal["today", "week", "month"] = "month"
    compare: bool = True
    top_limit: int = Field(default=5, ge=1, le=10)


class NotificationChatContext(BaseModel):
    suggested_user_message: str
    scope: Literal["sales_collections", "inventory_profit", "customers_retention"]
    routed_agent: Literal["copilot"]


class NotificationItem(BaseModel):
    id: str
    title: str
    body: str
    kind: Literal["insight"]
    entity_type: Literal["insight"]
    entity_id: str
    chat_context: NotificationChatContext
    created_at: str


class NotificationsResponse(BaseModel):
    request_id: str
    service_kind: Literal["insight_service"] = Field(default=SERVICE_KIND_INSIGHT)
    output_kind: Literal["insight_notification"] = Field(default=OUTPUT_KIND_INSIGHT_NOTIFICATION)
    items: list[NotificationItem] = Field(default_factory=list)


def _period_label(period: str) -> str:
    labels = {
        "today": "hoy",
        "week": "esta semana",
        "month": "este mes",
    }
    return labels.get(period, "este período")


def _build_notification_item(
    *,
    scope: Literal["sales_collections", "inventory_profit", "customers_retention"],
    title: str,
    insight: SalesCollectionsInsight | InventoryProfitInsight | CustomersRetentionInsight,
    created_at: str,
    period: str,
) -> NotificationItem:
    return NotificationItem(
        id=f"insight:{scope}:{period}",
        title=title,
        body=insight.summary,
        kind="insight",
        entity_type="insight",
        entity_id=scope,
        chat_context=NotificationChatContext(
            suggested_user_message=f"Quiero entender {title.lower()} de {_period_label(period)}.",
            scope=scope,
            routed_agent=COPILOT_AGENT_NAME,
        ),
        created_at=created_at,
    )


def get_insights_service(backend_client: BackendClient = Depends(get_backend_client)) -> InsightsService:
    return InsightsService(BackendInsightsRepository(backend_client))


@router.post("", response_model=NotificationsResponse)
async def create_notifications(
    req: NotificationsRequest,
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    service: InsightsService = Depends(get_insights_service),
):
    request_id = get_request_id() or str(uuid4())
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
        ),
        _build_notification_item(
            scope="inventory_profit",
            title="Insight de inventario y rentabilidad",
            insight=inventory,
            created_at=created_at,
            period=req.period,
        ),
        _build_notification_item(
            scope="customers_retention",
            title="Insight de clientes y retención",
            insight=customers,
            created_at=created_at,
            period=req.period,
        ),
    ]
    await repo.track_usage(auth.org_id, tokens_in=0, tokens_out=0)
    logger.info(
        "notifications_insights_completed",
        request_id=request_id,
        org_id=auth.org_id,
        actor=auth.actor,
        notifications=len(items),
        period=req.period,
        compare=req.compare,
    )
    return NotificationsResponse(
        request_id=request_id,
        service_kind=SERVICE_KIND_INSIGHT,
        output_kind=OUTPUT_KIND_INSIGHT_NOTIFICATION,
        items=items,
    )
