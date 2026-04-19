from __future__ import annotations

from collections.abc import Iterable

COMMERCIAL_EXTERNAL_SALES_TOOLS = frozenset(
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

COMMERCIAL_INTERNAL_SALES_BASE_TOOLS = frozenset(
    {
        "search_customers",
        "search_products",
        "search_services",
        "get_service",
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

COMMERCIAL_INTERNAL_PROCUREMENT_BASE_TOOLS = frozenset(
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

COMMERCIAL_INTERNAL_PROCUREMENT_ACCOUNTANT_TOOLS = frozenset(
    {
        "list_procurement_requests",
        "get_procurement_request",
        "get_purchases",
    }
)

COMMERCIAL_INTERNAL_SALES_ROLE_TOOLS: dict[str, frozenset[str]] = {
    "admin": COMMERCIAL_INTERNAL_SALES_BASE_TOOLS,
    "seller": frozenset(
        {
            "search_customers",
            "search_products",
            "search_services",
            "get_service",
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

COMMERCIAL_INTERNAL_PROCUREMENT_ROLE_TOOLS: dict[str, frozenset[str]] = {
    "admin": COMMERCIAL_INTERNAL_PROCUREMENT_BASE_TOOLS,
    "warehouse": COMMERCIAL_INTERNAL_PROCUREMENT_BASE_TOOLS,
    "accountant": COMMERCIAL_INTERNAL_PROCUREMENT_ACCOUNTANT_TOOLS,
}

LEGACY_INTERNAL_ROLE_TOOLS: dict[str, str | frozenset[str]] = {
    "admin": "*",
    "seller": frozenset(
        {
            "search_customers",
            "search_products",
            "search_services",
            "get_service",
            "get_bookings",
            "check_availability",
            "book_scheduling",
            "get_quotes",
            "create_quote",
            "create_sale",
            "get_account_balances",
            "get_low_stock",
            "generate_payment_link",
            "get_payment_status",
            "send_payment_info",
            "search_help",
        }
    ),
    "cashier": frozenset(
        {
            "get_recent_sales",
            "create_sale",
            "get_cashflow_summary",
            "create_cash_movement",
            "search_customers",
            "get_account_balances",
            "get_bookings",
            "generate_payment_link",
            "get_payment_status",
            "send_payment_info",
            "search_help",
        }
    ),
    "accountant": frozenset(
        {
            "get_sales_summary",
            "get_cashflow_summary",
            "get_account_balances",
            "get_purchases",
            "get_recurring_expenses",
            "get_debtors",
            "get_exchange_rates",
            "get_payment_status",
            "search_help",
        }
    ),
    "warehouse": frozenset(
        {
            "search_products",
            "get_low_stock",
            "get_stock_level",
            "get_purchases",
            "search_help",
        }
    ),
}

LEGACY_INTERNAL_FALLBACK_TOOLS = frozenset({"search_help"})

TOOL_MODULE_REQUIREMENTS: dict[str, frozenset[str]] = {
    "get_sales_summary": frozenset({"sales"}),
    "get_recent_sales": frozenset({"sales"}),
    "get_top_customers": frozenset({"customers", "sales"}),
    "search_customers": frozenset({"customers"}),
    "search_products": frozenset({"products"}),
    "search_services": frozenset({"services"}),
    "get_service": frozenset({"services"}),
    "get_low_stock": frozenset({"inventory", "products"}),
    "get_stock_level": frozenset({"inventory", "products"}),
    "get_cashflow_summary": frozenset({"cashflow"}),
    "get_account_balances": frozenset({"accounts"}),
    "get_debtors": frozenset({"accounts"}),
    "get_bookings": frozenset({"scheduling"}),
    "check_availability": frozenset({"scheduling"}),
    "get_quotes": frozenset({"quotes"}),
    "get_purchases": frozenset({"purchases"}),
    "get_recurring_expenses": frozenset({"recurring"}),
    "get_exchange_rates": frozenset({"currency"}),
    "create_quote": frozenset({"quotes"}),
    "request_quote": frozenset({"quotes"}),
    "create_sale": frozenset({"sales"}),
    "book_scheduling": frozenset({"scheduling"}),
    "create_cash_movement": frozenset({"cashflow"}),
    "generate_payment_link": frozenset({"sales", "quotes", "paymentgateway"}),
    "get_payment_status": frozenset({"sales", "quotes", "paymentgateway"}),
    "send_payment_info": frozenset({"sales", "whatsapp"}),
    "get_quote_payment_link": frozenset({"quotes", "paymentgateway"}),
    "get_payment_link": frozenset({"quotes", "paymentgateway"}),
    "search_suppliers": frozenset({"suppliers"}),
    "prepare_purchase_draft": frozenset({"purchases", "inventory", "products"}),
    "list_procurement_requests": frozenset({"purchases"}),
    "create_procurement_request": frozenset({"purchases"}),
    "get_procurement_request": frozenset({"purchases"}),
    "submit_procurement_request": frozenset({"purchases"}),
    "complete_onboarding_step": frozenset(),
    "skip_onboarding_step": frozenset(),
    "apply_business_profile": frozenset(),
    "update_business_info": frozenset(),
    "get_tenant_settings": frozenset(),
    "remember_fact": frozenset(),
    "search_help": frozenset(),
}


def normalize_role(role: str) -> str:
    return (role or "").strip().lower()


def filter_tools_by_modules(tools: Iterable[str], modules_active: Iterable[str]) -> frozenset[str]:
    active = {item.strip().lower() for item in modules_active if isinstance(item, str) and item.strip()}
    if not active:
        return frozenset(tools)
    allowed: set[str] = set()
    for tool_name in tools:
        required = TOOL_MODULE_REQUIREMENTS.get(tool_name)
        if required is None or not required or required.intersection(active):
            allowed.add(tool_name)
    return frozenset(allowed)


def resolve_commercial_internal_sales_tools(role: str, modules_active: Iterable[str]) -> frozenset[str]:
    base = COMMERCIAL_INTERNAL_SALES_ROLE_TOOLS.get(normalize_role(role), frozenset())
    return filter_tools_by_modules(base, modules_active)


def resolve_commercial_internal_procurement_tools(role: str, modules_active: Iterable[str]) -> frozenset[str]:
    base = COMMERCIAL_INTERNAL_PROCUREMENT_ROLE_TOOLS.get(normalize_role(role), frozenset())
    return filter_tools_by_modules(base, modules_active)


def is_legacy_internal_tool_allowed(role: str, modules_active: Iterable[str], tool_name: str) -> bool:
    allowed = LEGACY_INTERNAL_ROLE_TOOLS.get(normalize_role(role), LEGACY_INTERNAL_FALLBACK_TOOLS)
    if allowed == "*":
        return tool_name in filter_tools_by_modules({tool_name}, modules_active)
    filtered = filter_tools_by_modules(allowed, modules_active)
    return tool_name in filtered
