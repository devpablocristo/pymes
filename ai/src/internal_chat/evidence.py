from __future__ import annotations

import copy
from dataclasses import dataclass, field
from datetime import UTC, date, datetime, timedelta
from typing import Any

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.internal_chat.routing import AnalysisScope, InternalRouteDecision


READ_ONLY_INTERNAL_TOOLS: frozenset[str] = frozenset(
    {
        "get_sales_summary",
        "get_sales_by_customer",
        "get_sales_by_payment",
        "get_sales_by_product",
        "get_sales_by_service",
        "get_recent_sales",
        "search_customers",
        "search_products",
        "search_services",
        "get_low_stock",
        "get_account_balances",
        "get_debtors",
        "get_purchases_summary",
        "list_procurement_requests",
        "search_suppliers",
        "get_bookings",
        "search_employees",
    }
)

MUTATING_INTERNAL_TOOLS: frozenset[str] = frozenset(
    {
        "create_quote",
        "create_sale",
        "generate_payment_link",
        "send_payment_info",
        "create_procurement_request",
        "submit_procurement_request",
        "book_scheduling",
        "check_availability",
        "complete_onboarding_step",
    }
)


@dataclass(frozen=True)
class EvidencePeriod:
    label: str
    from_date: str
    to_date: str

    def as_metadata(self) -> dict[str, str]:
        return {"label": self.label, "from": self.from_date, "to": self.to_date}


@dataclass(frozen=True)
class EvidenceCall:
    name: str
    data: Any


@dataclass(frozen=True)
class EvidencePacket:
    scope: AnalysisScope
    period: EvidencePeriod | None
    calls: list[EvidenceCall] = field(default_factory=list)

    @property
    def tools(self) -> list[str]:
        return [call.name for call in self.calls]

    @property
    def record_counts(self) -> dict[str, int]:
        return {call.name: _count_records(call.data) for call in self.calls}

    def metadata(self) -> dict[str, Any]:
        return {
            "tools": self.tools,
            "record_counts": self.record_counts,
            "period": self.period.as_metadata() if self.period is not None else None,
        }

    def prompt_payload(self) -> dict[str, Any]:
        return {
            "scope": self.scope,
            "period": self.period.as_metadata() if self.period is not None else None,
            "record_counts": self.record_counts,
            "tools": [{"name": call.name, "data": _trim_for_prompt(call.data)} for call in self.calls],
        }


class EvidenceUnavailable(RuntimeError):
    def __init__(self, *, tool_name: str, message: str) -> None:
        super().__init__(message)
        self.tool_name = tool_name


async def build_evidence_packet(
    *,
    backend_client: BackendClient,
    auth: AuthContext,
    decision: InternalRouteDecision,
    message: str,
) -> EvidencePacket:
    period = _resolve_period(message, decision.scope)
    calls: list[EvidenceCall] = []

    async def add(name: str, path: str, params: dict[str, Any] | None = None) -> None:
        if name not in READ_ONLY_INTERNAL_TOOLS or name in MUTATING_INTERNAL_TOOLS:
            raise EvidenceUnavailable(tool_name=name, message=f"tool_not_allowed:{name}")
        requester = getattr(backend_client, "request", None)
        if requester is None:
            raise EvidenceUnavailable(tool_name=name, message="backend_client_missing_request")
        try:
            data = await requester("GET", path, auth=auth, params=params or {})
        except Exception as exc:  # noqa: BLE001
            raise EvidenceUnavailable(tool_name=name, message=str(exc)) from exc
        calls.append(EvidenceCall(name=name, data=data))

    if decision.scope == "general":
        return EvidencePacket(scope=decision.scope, period=None, calls=[])

    if decision.scope in {"operations", "sales_collections"}:
        range_params = _range_params(period)
        await add("get_sales_summary", "/v1/reports/sales-summary", range_params)
        await add("get_sales_by_customer", "/v1/reports/sales-by-customer", range_params)
        await add("get_sales_by_payment", "/v1/reports/sales-by-payment", range_params)
        await add("get_recent_sales", "/v1/sales", {"limit": 10})
        await add("get_debtors", "/v1/accounts/debtors", {"limit": 20})
        await add("get_account_balances", "/v1/accounts", {"non_zero": "true", "limit": 20})
        if decision.scope == "sales_collections":
            return EvidencePacket(scope=decision.scope, period=period, calls=calls)
        await add("get_low_stock", "/v1/reports/low-stock")
        await add("list_procurement_requests", "/v1/procurement-requests", {"limit": 10})
        return EvidencePacket(scope=decision.scope, period=period, calls=calls)

    if decision.scope == "customers":
        await add("search_customers", "/v1/customers", {"limit": 20})
        await add("get_sales_by_customer", "/v1/reports/sales-by-customer", _range_params(period))
        return EvidencePacket(scope=decision.scope, period=period, calls=calls)

    if decision.scope == "products":
        await add("search_products", "/v1/products", {"limit": 20})
        await add("get_low_stock", "/v1/reports/low-stock")
        await add("get_sales_by_product", "/v1/reports/sales-by-product", _range_params(period))
        return EvidencePacket(scope=decision.scope, period=period, calls=calls)

    if decision.scope == "services":
        await add("search_services", "/v1/services", {"limit": 20})
        await add("get_sales_by_service", "/v1/reports/sales-by-service", _range_params(period))
        return EvidencePacket(scope=decision.scope, period=period, calls=calls)

    if decision.scope == "purchases":
        await add("get_purchases_summary", "/v1/purchases", {"limit": 20})
        await add("list_procurement_requests", "/v1/procurement-requests", {"limit": 20})
        await add("search_suppliers", "/v1/suppliers", {"limit": 20})
        return EvidencePacket(scope=decision.scope, period=None, calls=calls)

    if decision.scope == "scheduling":
        await add("get_bookings", "/v1/scheduling/bookings", _range_params(period))
        return EvidencePacket(scope=decision.scope, period=period, calls=calls)

    if decision.scope == "employees":
        await add("search_employees", "/v1/parties", {"role": "employee", "limit": 20})
        return EvidencePacket(scope=decision.scope, period=None, calls=calls)

    return EvidencePacket(scope=decision.scope, period=period, calls=calls)


def _resolve_period(message: str, scope: AnalysisScope) -> EvidencePeriod:
    today = datetime.now(UTC).date()
    normalized = message.lower()
    if "mes" in normalized:
        start = date(today.year, today.month, 1)
        return EvidencePeriod(label="mes", from_date=start.isoformat(), to_date=today.isoformat())
    if "hoy" in normalized:
        return EvidencePeriod(label="hoy", from_date=today.isoformat(), to_date=today.isoformat())
    if scope == "scheduling" and "mañana" in normalized:
        tomorrow = today + timedelta(days=1)
        return EvidencePeriod(label="mañana", from_date=tomorrow.isoformat(), to_date=tomorrow.isoformat())
    start = today - timedelta(days=today.weekday())
    return EvidencePeriod(label="semana", from_date=start.isoformat(), to_date=today.isoformat())


def _range_params(period: EvidencePeriod | None) -> dict[str, str]:
    if period is None:
        return {}
    return {"from": period.from_date, "to": period.to_date}


def _count_records(data: Any) -> int:
    if isinstance(data, list):
        return len(data)
    if not isinstance(data, dict):
        return 1 if data is not None else 0
    items = data.get("items")
    if isinstance(items, list):
        return len(items)
    nested = data.get("data")
    if isinstance(nested, dict):
        count = nested.get("count_sales")
        if isinstance(count, int):
            return count
        return 1 if nested else 0
    return 1 if data else 0


def _trim_for_prompt(value: Any, *, depth: int = 0) -> Any:
    if depth >= 4:
        return str(value)[:300]
    if isinstance(value, list):
        return [_trim_for_prompt(item, depth=depth + 1) for item in value[:12]]
    if isinstance(value, dict):
        trimmed: dict[str, Any] = {}
        for key, item in list(value.items())[:30]:
            trimmed[str(key)] = _trim_for_prompt(item, depth=depth + 1)
        return trimmed
    if isinstance(value, (str, int, float, bool)) or value is None:
        return value
    return copy.deepcopy(str(value))
