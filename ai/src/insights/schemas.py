from __future__ import annotations

from datetime import date
from typing import Literal

from pydantic import BaseModel, Field, model_validator


class SalesCollectionsInsightQuery(BaseModel):
    period: Literal["today", "week", "month"] = "month"
    from_date: date | None = Field(default=None, alias="from")
    to_date: date | None = Field(default=None, alias="to")
    compare: bool = True
    top_limit: int = Field(default=5, ge=1, le=10)

    model_config = {"populate_by_name": True}

    @model_validator(mode="after")
    def validate_dates(self) -> "SalesCollectionsInsightQuery":
        if (self.from_date is None) != (self.to_date is None):
            raise ValueError("from and to must be provided together")
        if self.from_date is not None and self.to_date is not None and self.to_date < self.from_date:
            raise ValueError("to must be greater than or equal to from")
        return self


class InsightPeriodResponse(BaseModel):
    label: str
    from_date: date
    to_date: date


class InsightMetricResponse(BaseModel):
    key: str
    label: str
    unit: Literal["currency", "count", "percentage"]
    value: float
    previous_value: float | None = None
    delta: float | None = None
    delta_pct: float | None = None
    trend: Literal["up", "down", "flat", "unknown"]


class InsightHighlightResponse(BaseModel):
    severity: Literal["positive", "info", "warning"]
    title: str
    detail: str


class TopCustomerInsightResponse(BaseModel):
    customer_id: str | None = None
    customer_name: str
    total: float
    count: int
    share_pct: float


class PaymentMethodInsightResponse(BaseModel):
    payment_method: str
    total: float
    count: int
    share_pct: float


class DebtorInsightResponse(BaseModel):
    party_id: str
    party_name: str
    total_debt: float
    oldest_date: date | None = None


class ProductPerformanceInsightResponse(BaseModel):
    product_id: str | None = None
    product_name: str
    quantity: float
    revenue: float
    share_pct: float


class StockAlertInsightResponse(BaseModel):
    product_id: str
    product_name: str
    sku: str | None = None
    quantity: float
    min_quantity: float
    deficit: float


class SalesCollectionsInsightResponse(BaseModel):
    scope: Literal["sales_collections"]
    period: InsightPeriodResponse
    comparison_period: InsightPeriodResponse | None = None
    summary: str
    kpis: list[InsightMetricResponse]
    highlights: list[InsightHighlightResponse]
    recommendations: list[str]
    top_customers: list[TopCustomerInsightResponse]
    payment_mix: list[PaymentMethodInsightResponse]
    debtors: list[DebtorInsightResponse]


class InventoryProfitInsightResponse(BaseModel):
    scope: Literal["inventory_profit"]
    period: InsightPeriodResponse
    comparison_period: InsightPeriodResponse | None = None
    summary: str
    kpis: list[InsightMetricResponse]
    highlights: list[InsightHighlightResponse]
    recommendations: list[str]
    top_products: list[ProductPerformanceInsightResponse]
    low_stock: list[StockAlertInsightResponse]


class CustomersRetentionInsightResponse(BaseModel):
    scope: Literal["customers_retention"]
    period: InsightPeriodResponse
    comparison_period: InsightPeriodResponse | None = None
    summary: str
    kpis: list[InsightMetricResponse]
    highlights: list[InsightHighlightResponse]
    recommendations: list[str]
    top_customers: list[TopCustomerInsightResponse]
