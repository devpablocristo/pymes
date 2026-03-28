from __future__ import annotations

from fastapi import APIRouter, Depends, Query
from fastapi.exceptions import RequestValidationError
from pydantic import ValidationError

from runtime.logging import get_logger
from src.api.deps import get_auth_context, get_backend_client, get_repository
from src.api.router import check_quota
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from src.insights.domain import InsightFilters
from src.insights.repository import BackendInsightsRepository
from src.insights.schemas import (
    CustomersRetentionInsightResponse,
    InventoryProfitInsightResponse,
    SalesCollectionsInsightQuery,
    SalesCollectionsInsightResponse,
)
from src.insights.service import InsightsService

router = APIRouter(prefix="/v1/insights", tags=["insights"])
logger = get_logger(__name__)


def get_insights_service(backend_client: BackendClient = Depends(get_backend_client)) -> InsightsService:
    return InsightsService(BackendInsightsRepository(backend_client))


def get_sales_collections_query(
    period: str = Query(default="month"),
    from_date: str | None = Query(default=None, alias="from"),
    to_date: str | None = Query(default=None, alias="to"),
    compare: bool = Query(default=True),
    top_limit: int = Query(default=5, ge=1, le=10),
) -> SalesCollectionsInsightQuery:
    try:
        return SalesCollectionsInsightQuery.model_validate(
            {
                "period": period,
                "from": from_date,
                "to": to_date,
                "compare": compare,
                "top_limit": top_limit,
            }
        )
    except ValidationError as exc:
        raise RequestValidationError(exc.errors()) from exc


@router.get("/sales-collections", response_model=SalesCollectionsInsightResponse)
async def get_sales_collections_insight(
    query: SalesCollectionsInsightQuery = Depends(get_sales_collections_query),
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    service: InsightsService = Depends(get_insights_service),
):
    await check_quota(repo, auth.org_id, mode="internal")
    insight = await service.build_sales_collections_insight(
        auth=auth,
        filters=InsightFilters(
            period=query.period,
            from_date=query.from_date,
            to_date=query.to_date,
            compare=query.compare,
            top_limit=query.top_limit,
        ),
    )
    await repo.track_usage(auth.org_id, tokens_in=0, tokens_out=0)
    logger.info(
        "insights_sales_collections_completed",
        org_id=auth.org_id,
        actor=auth.actor,
        period=query.period if query.from_date is None else "custom",
        compare=query.compare,
    )
    return SalesCollectionsInsightResponse.model_validate(insight.model_dump())


@router.get("/inventory-profit", response_model=InventoryProfitInsightResponse)
async def get_inventory_profit_insight(
    query: SalesCollectionsInsightQuery = Depends(get_sales_collections_query),
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    service: InsightsService = Depends(get_insights_service),
):
    await check_quota(repo, auth.org_id, mode="internal")
    insight = await service.build_inventory_profit_insight(
        auth=auth,
        filters=InsightFilters(
            period=query.period,
            from_date=query.from_date,
            to_date=query.to_date,
            compare=query.compare,
            top_limit=query.top_limit,
        ),
    )
    await repo.track_usage(auth.org_id, tokens_in=0, tokens_out=0)
    logger.info(
        "insights_inventory_profit_completed",
        org_id=auth.org_id,
        actor=auth.actor,
        period=query.period if query.from_date is None else "custom",
        compare=query.compare,
    )
    return InventoryProfitInsightResponse.model_validate(insight.model_dump())


@router.get("/customers-retention", response_model=CustomersRetentionInsightResponse)
async def get_customers_retention_insight(
    query: SalesCollectionsInsightQuery = Depends(get_sales_collections_query),
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    service: InsightsService = Depends(get_insights_service),
):
    await check_quota(repo, auth.org_id, mode="internal")
    insight = await service.build_customers_retention_insight(
        auth=auth,
        filters=InsightFilters(
            period=query.period,
            from_date=query.from_date,
            to_date=query.to_date,
            compare=query.compare,
            top_limit=query.top_limit,
        ),
    )
    await repo.track_usage(auth.org_id, tokens_in=0, tokens_out=0)
    logger.info(
        "insights_customers_retention_completed",
        org_id=auth.org_id,
        actor=auth.actor,
        period=query.period if query.from_date is None else "custom",
        compare=query.compare,
    )
    return CustomersRetentionInsightResponse.model_validate(insight.model_dump())
