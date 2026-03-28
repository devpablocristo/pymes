from __future__ import annotations

from datetime import date
from typing import Literal

from pydantic import BaseModel, Field


class InsightPeriod(BaseModel):
    label: str
    from_date: date
    to_date: date


class InsightFilters(BaseModel):
    period: str = "month"
    from_date: date | None = None
    to_date: date | None = None
    compare: bool = True
    top_limit: int = Field(default=5, ge=1, le=10)


class SalesSummarySnapshot(BaseModel):
    total_sales: float = 0.0
    count_sales: int = 0
    average_ticket: float = 0.0


class CashflowSummarySnapshot(BaseModel):
    total_income: float = 0.0
    total_expense: float = 0.0
    balance: float = 0.0


class TopCustomerSnapshot(BaseModel):
    customer_id: str | None = None
    customer_name: str
    total: float = 0.0
    count: int = 0


class PaymentMethodSnapshot(BaseModel):
    payment_method: str
    total: float = 0.0
    count: int = 0


class DebtorSnapshot(BaseModel):
    party_id: str
    party_name: str
    total_debt: float = 0.0
    oldest_date: date | None = None


class SalesByProductSnapshot(BaseModel):
    product_id: str | None = None
    product_name: str
    quantity: float = 0.0
    revenue: float = 0.0


class InventoryValuationSnapshot(BaseModel):
    product_id: str
    product_name: str
    sku: str | None = None
    quantity: float = 0.0
    cost_price: float = 0.0
    valuation: float = 0.0


class LowStockSnapshot(BaseModel):
    product_id: str
    product_name: str
    sku: str | None = None
    quantity: float = 0.0
    min_quantity: float = 0.0


class ProfitMarginSnapshot(BaseModel):
    revenue: float = 0.0
    cost: float = 0.0
    gross_profit: float = 0.0
    margin_pct: float = 0.0


class CustomerBaseSnapshot(BaseModel):
    total: int = 0


class InsightMetric(BaseModel):
    key: str
    label: str
    unit: Literal["currency", "count", "percentage"]
    value: float
    previous_value: float | None = None
    delta: float | None = None
    delta_pct: float | None = None
    trend: Literal["up", "down", "flat", "unknown"] = "unknown"


class InsightHighlight(BaseModel):
    severity: Literal["positive", "info", "warning"]
    title: str
    detail: str


class TopCustomerInsight(BaseModel):
    customer_id: str | None = None
    customer_name: str
    total: float
    count: int
    share_pct: float


class PaymentMethodInsight(BaseModel):
    payment_method: str
    total: float
    count: int
    share_pct: float


class DebtorInsight(BaseModel):
    party_id: str
    party_name: str
    total_debt: float
    oldest_date: date | None = None


class ProductPerformanceInsight(BaseModel):
    product_id: str | None = None
    product_name: str
    quantity: float
    revenue: float
    share_pct: float


class StockAlertInsight(BaseModel):
    product_id: str
    product_name: str
    sku: str | None = None
    quantity: float
    min_quantity: float
    deficit: float


class SalesCollectionsInsight(BaseModel):
    scope: Literal["sales_collections"] = "sales_collections"
    period: InsightPeriod
    comparison_period: InsightPeriod | None = None
    summary: str
    kpis: list[InsightMetric]
    highlights: list[InsightHighlight]
    recommendations: list[str]
    top_customers: list[TopCustomerInsight]
    payment_mix: list[PaymentMethodInsight]
    debtors: list[DebtorInsight]


class InventoryProfitInsight(BaseModel):
    scope: Literal["inventory_profit"] = "inventory_profit"
    period: InsightPeriod
    comparison_period: InsightPeriod | None = None
    summary: str
    kpis: list[InsightMetric]
    highlights: list[InsightHighlight]
    recommendations: list[str]
    top_products: list[ProductPerformanceInsight]
    low_stock: list[StockAlertInsight]


class CustomersRetentionInsight(BaseModel):
    scope: Literal["customers_retention"] = "customers_retention"
    period: InsightPeriod
    comparison_period: InsightPeriod | None = None
    summary: str
    kpis: list[InsightMetric]
    highlights: list[InsightHighlight]
    recommendations: list[str]
    top_customers: list[TopCustomerInsight]
