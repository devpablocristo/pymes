from __future__ import annotations

from dataclasses import dataclass
from typing import Literal

from src.backend_client.auth import AuthContext

AgentMode = Literal["external_sales", "internal_sales", "internal_procurement"]
Channel = Literal["web_public", "whatsapp", "api", "embedded", "internal_ui"]


@dataclass(frozen=True)
class CommercialPolicy:
    agent_mode: AgentMode
    channel: Channel
    allowed_tools: frozenset[str]
    confirm_required_tools: frozenset[str]
    max_tool_calls: int
    tool_timeout_seconds: int
    total_timeout_seconds: int

    def allows(self, tool_name: str) -> bool:
        return tool_name in self.allowed_tools

    def requires_confirmation(self, tool_name: str) -> bool:
        return tool_name in self.confirm_required_tools


EXTERNAL_SALES_TOOLS = frozenset(
    {
        "get_business_info",
        "get_public_services",
        "check_availability",
        "get_my_bookings",
        "request_quote",
        "get_quote_payment_link",
        "book_scheduling",
    }
)

INTERNAL_SALES_BASE_TOOLS = frozenset(
    {
        "search_customers",
        "search_products",
        "get_low_stock",
        "get_stock_level",
        "get_quotes",
        "create_quote",
        "create_sale",
        "generate_payment_link",
        "get_payment_status",
        "send_payment_info",
        "get_account_balances",
        "get_recent_sales",
    }
)

INTERNAL_PROCUREMENT_BASE_TOOLS = frozenset(
    {
        "search_suppliers",
        "search_products",
        "get_low_stock",
        "get_stock_level",
        "get_purchases",
        "prepare_purchase_draft",
        "list_procurement_requests",
        "create_procurement_request",
        "get_procurement_request",
        "submit_procurement_request",
    }
)

# Contador / finanzas: visibilidad del circuito sin crear ni enviar solicitudes.
INTERNAL_PROCUREMENT_ACCOUNTANT_TOOLS = frozenset(
    {
        "list_procurement_requests",
        "get_procurement_request",
        "get_purchases",
    }
)

ROLE_INTERNAL_SALES: dict[str, frozenset[str]] = {
    "admin": INTERNAL_SALES_BASE_TOOLS,
    "seller": frozenset(
        {
            "search_customers",
            "search_products",
            "get_low_stock",
            "get_stock_level",
            "get_quotes",
            "create_quote",
            "create_sale",
            "generate_payment_link",
            "get_payment_status",
            "send_payment_info",
            "get_recent_sales",
        }
    ),
    "cashier": frozenset(
        {
            "search_customers",
            "search_products",
            "get_stock_level",
            "create_sale",
            "generate_payment_link",
            "get_payment_status",
            "send_payment_info",
            "get_recent_sales",
            "get_account_balances",
        }
    ),
}

ROLE_INTERNAL_PROCUREMENT: dict[str, frozenset[str]] = {
    "admin": INTERNAL_PROCUREMENT_BASE_TOOLS,
    "warehouse": INTERNAL_PROCUREMENT_BASE_TOOLS,
    "accountant": INTERNAL_PROCUREMENT_ACCOUNTANT_TOOLS,
}

MODULE_REQUIREMENTS: dict[str, frozenset[str]] = {
    "search_customers": frozenset({"customers"}),
    "search_products": frozenset({"products"}),
    "get_low_stock": frozenset({"inventory", "products"}),
    "get_stock_level": frozenset({"inventory", "products"}),
    "get_quotes": frozenset({"quotes"}),
    "create_quote": frozenset({"quotes"}),
    "create_sale": frozenset({"sales"}),
    "generate_payment_link": frozenset({"sales", "quotes", "paymentgateway"}),
    "get_payment_status": frozenset({"sales", "quotes", "paymentgateway"}),
    "send_payment_info": frozenset({"sales", "whatsapp"}),
    "get_account_balances": frozenset({"accounts"}),
    "get_recent_sales": frozenset({"sales"}),
    "search_suppliers": frozenset({"suppliers"}),
    "get_purchases": frozenset({"purchases"}),
    "prepare_purchase_draft": frozenset({"purchases", "inventory", "products"}),
    "list_procurement_requests": frozenset({"purchases"}),
    "create_procurement_request": frozenset({"purchases"}),
    "get_procurement_request": frozenset({"purchases"}),
    "submit_procurement_request": frozenset({"purchases"}),
}

EXTERNAL_CONFIRM_REQUIRED = frozenset({"book_scheduling"})
INTERNAL_SALES_CONFIRM_REQUIRED = frozenset({"create_quote", "create_sale", "generate_payment_link"})
INTERNAL_PROCUREMENT_CONFIRM_REQUIRED = frozenset()


def _filter_by_modules(tools: frozenset[str], modules_active: list[str]) -> frozenset[str]:
    if not modules_active:
        return tools
    active = {item.strip().lower() for item in modules_active if str(item).strip()}
    allowed: set[str] = set()
    for tool_name in tools:
        required = MODULE_REQUIREMENTS.get(tool_name)
        if required is None or required.intersection(active):
            allowed.add(tool_name)
    return frozenset(allowed)


def build_external_sales_policy(channel: Channel = "web_public") -> CommercialPolicy:
    return CommercialPolicy(
        agent_mode="external_sales",
        channel=channel,
        allowed_tools=EXTERNAL_SALES_TOOLS,
        confirm_required_tools=EXTERNAL_CONFIRM_REQUIRED,
        max_tool_calls=5,
        tool_timeout_seconds=8,
        total_timeout_seconds=30,
    )


def build_internal_sales_policy(auth: AuthContext, modules_active: list[str], channel: Channel = "internal_ui") -> CommercialPolicy:
    base = ROLE_INTERNAL_SALES.get(auth.role.strip().lower(), frozenset())
    return CommercialPolicy(
        agent_mode="internal_sales",
        channel=channel,
        allowed_tools=_filter_by_modules(base, modules_active),
        confirm_required_tools=INTERNAL_SALES_CONFIRM_REQUIRED.intersection(_filter_by_modules(base, modules_active)),
        max_tool_calls=6,
        tool_timeout_seconds=10,
        total_timeout_seconds=45,
    )


def build_internal_procurement_policy(auth: AuthContext, modules_active: list[str], channel: Channel = "internal_ui") -> CommercialPolicy:
    base = ROLE_INTERNAL_PROCUREMENT.get(auth.role.strip().lower(), frozenset())
    return CommercialPolicy(
        agent_mode="internal_procurement",
        channel=channel,
        allowed_tools=_filter_by_modules(base, modules_active),
        confirm_required_tools=INTERNAL_PROCUREMENT_CONFIRM_REQUIRED,
        max_tool_calls=8,
        tool_timeout_seconds=10,
        total_timeout_seconds=45,
    )
