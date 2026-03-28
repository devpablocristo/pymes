from __future__ import annotations

from datetime import date
from typing import Any, Protocol, TypeVar

from pydantic import ValidationError

from runtime.logging import get_logger
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.insights.domain import (
    CashflowSummarySnapshot,
    CustomerBaseSnapshot,
    DebtorSnapshot,
    InventoryValuationSnapshot,
    LowStockSnapshot,
    PaymentMethodSnapshot,
    ProfitMarginSnapshot,
    SalesByProductSnapshot,
    SalesSummarySnapshot,
    TopCustomerSnapshot,
)

logger = get_logger(__name__)


class InsightsRepository(Protocol):
    async def get_sales_summary(self, auth: AuthContext, *, from_date: date, to_date: date) -> SalesSummarySnapshot: ...

    async def get_cashflow_summary(self, auth: AuthContext, *, from_date: date, to_date: date) -> CashflowSummarySnapshot: ...

    async def get_sales_by_customer(
        self, auth: AuthContext, *, from_date: date, to_date: date
    ) -> list[TopCustomerSnapshot]: ...

    async def get_sales_by_payment(
        self, auth: AuthContext, *, from_date: date, to_date: date
    ) -> list[PaymentMethodSnapshot]: ...

    async def get_debtors(self, auth: AuthContext, *, limit: int) -> list[DebtorSnapshot]: ...

    async def get_sales_by_product(
        self, auth: AuthContext, *, from_date: date, to_date: date
    ) -> list[SalesByProductSnapshot]: ...

    async def get_inventory_valuation(self, auth: AuthContext) -> list[InventoryValuationSnapshot]: ...

    async def get_low_stock(self, auth: AuthContext) -> list[LowStockSnapshot]: ...

    async def get_profit_margin(self, auth: AuthContext, *, from_date: date, to_date: date) -> ProfitMarginSnapshot: ...

    async def get_customers_total(self, auth: AuthContext) -> CustomerBaseSnapshot: ...


class BackendInsightsRepository:
    def __init__(self, client: BackendClient) -> None:
        self._client = client

    async def get_sales_summary(self, auth: AuthContext, *, from_date: date, to_date: date) -> SalesSummarySnapshot:
        payload = await self._client.request(
            "GET",
            "/v1/reports/sales-summary",
            auth=auth,
            params={"from": from_date.isoformat(), "to": to_date.isoformat()},
        )
        return self._parse_model(SalesSummarySnapshot, self._nested_dict(payload, "data"))

    async def get_cashflow_summary(self, auth: AuthContext, *, from_date: date, to_date: date) -> CashflowSummarySnapshot:
        payload = await self._client.request(
            "GET",
            "/v1/reports/cashflow-summary",
            auth=auth,
            params={"from": from_date.isoformat(), "to": to_date.isoformat()},
        )
        return self._parse_model(CashflowSummarySnapshot, self._nested_dict(payload, "data"))

    async def get_sales_by_customer(
        self, auth: AuthContext, *, from_date: date, to_date: date
    ) -> list[TopCustomerSnapshot]:
        payload = await self._client.request(
            "GET",
            "/v1/reports/sales-by-customer",
            auth=auth,
            params={"from": from_date.isoformat(), "to": to_date.isoformat()},
        )
        return self._parse_list(TopCustomerSnapshot, self._nested_list(payload, "items"))

    async def get_sales_by_payment(
        self, auth: AuthContext, *, from_date: date, to_date: date
    ) -> list[PaymentMethodSnapshot]:
        payload = await self._client.request(
            "GET",
            "/v1/reports/sales-by-payment",
            auth=auth,
            params={"from": from_date.isoformat(), "to": to_date.isoformat()},
        )
        return self._parse_list(PaymentMethodSnapshot, self._nested_list(payload, "items"))

    async def get_debtors(self, auth: AuthContext, *, limit: int) -> list[DebtorSnapshot]:
        payload = await self._client.request(
            "GET",
            "/v1/accounts/debtors",
            auth=auth,
            params={"limit": max(1, min(limit, 20))},
        )
        return self._parse_list(DebtorSnapshot, self._nested_list(payload, "items"))

    async def get_sales_by_product(
        self, auth: AuthContext, *, from_date: date, to_date: date
    ) -> list[SalesByProductSnapshot]:
        payload = await self._client.request(
            "GET",
            "/v1/reports/sales-by-product",
            auth=auth,
            params={"from": from_date.isoformat(), "to": to_date.isoformat()},
        )
        return self._parse_list(SalesByProductSnapshot, self._nested_list(payload, "items"))

    async def get_inventory_valuation(self, auth: AuthContext) -> list[InventoryValuationSnapshot]:
        payload = await self._client.request(
            "GET",
            "/v1/reports/inventory-valuation",
            auth=auth,
        )
        return self._parse_list(InventoryValuationSnapshot, self._nested_list(payload, "items"))

    async def get_low_stock(self, auth: AuthContext) -> list[LowStockSnapshot]:
        payload = await self._client.request(
            "GET",
            "/v1/reports/low-stock",
            auth=auth,
        )
        return self._parse_list(LowStockSnapshot, self._nested_list(payload, "items"))

    async def get_profit_margin(self, auth: AuthContext, *, from_date: date, to_date: date) -> ProfitMarginSnapshot:
        payload = await self._client.request(
            "GET",
            "/v1/reports/profit-margin",
            auth=auth,
            params={"from": from_date.isoformat(), "to": to_date.isoformat()},
        )
        return self._parse_model(ProfitMarginSnapshot, self._nested_dict(payload, "data"))

    async def get_customers_total(self, auth: AuthContext) -> CustomerBaseSnapshot:
        payload = await self._client.request(
            "GET",
            "/v1/customers",
            auth=auth,
            params={"limit": 1},
        )
        total = payload.get("total", 0)
        if isinstance(total, bool):
            total = 0
        if not isinstance(total, int):
            try:
                total = int(total)
            except (TypeError, ValueError):
                total = 0
        return CustomerBaseSnapshot(total=max(0, total))

    @staticmethod
    def _nested_dict(payload: dict[str, Any], key: str) -> dict[str, Any]:
        value = payload.get(key)
        if not isinstance(value, dict):
            return {}
        return value

    @staticmethod
    def _nested_list(payload: dict[str, Any], key: str) -> list[dict[str, Any]]:
        value = payload.get(key)
        if not isinstance(value, list):
            return []
        return [item for item in value if isinstance(item, dict)]

    @staticmethod
    def _parse_model(model: type[ModelT], payload: dict[str, Any]) -> ModelT:
        try:
            return model.model_validate(payload)
        except ValidationError as exc:
            logger.warning(
                "insights_payload_invalid",
                model=model.__name__,
                errors=exc.errors(),
            )
            raise ValueError(f"invalid payload for {model.__name__}") from exc

    @classmethod
    def _parse_list(cls, model: type[ModelT], payload: list[dict[str, Any]]) -> list[ModelT]:
        items: list[ModelT] = []
        for raw in payload:
            try:
                items.append(model.model_validate(raw))
            except ValidationError as exc:
                logger.warning(
                    "insights_list_item_invalid",
                    model=model.__name__,
                    errors=exc.errors(),
                )
                continue
        return items


ModelT = TypeVar("ModelT")
