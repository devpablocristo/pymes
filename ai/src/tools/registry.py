from __future__ import annotations

from typing import Any

from src.agents.tool_access import is_internal_tool_allowed
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.tools.external_registry import build_external_tools
from src.tools.internal_profile_registry import register_profile_tools
from src.tools.registry_common import ToolHandler, tool
from runtime.types import ToolDeclaration
from src.tools import (
    accounts,
    cashflow,
    currency,
    customers,
    help,
    inventory,
    payments,
    products,
    purchases,
    quotes,
    recurring,
    sales,
    scheduling,
    services,
)

__all__ = ["build_external_tools", "build_internal_tools"]


def _maybe_add(
    declarations: list[ToolDeclaration],
    handlers: dict[str, ToolHandler],
    role: str,
    modules_active: list[str],
    declaration: ToolDeclaration,
    handler: ToolHandler,
) -> None:
    if not is_internal_tool_allowed(role, modules_active, declaration.name):
        return
    declarations.append(declaration)
    handlers[declaration.name] = handler


def build_internal_tools(
    client: BackendClient,
    auth: AuthContext,
    dossier: dict[str, Any],
) -> tuple[list[ToolDeclaration], dict[str, ToolHandler]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, ToolHandler] = {}
    role = auth.role
    modules_active = dossier.get("modules_active", []) if isinstance(dossier, dict) else []

    async def _get_sales_summary(tenant_id: str, period: str = "today") -> dict[str, Any]:
        _ = tenant_id
        return await sales.get_sales_summary(client, auth, period=period)

    async def _get_recent_sales(tenant_id: str, limit: int = 10) -> dict[str, Any]:
        _ = tenant_id
        return await sales.get_recent_sales(client, auth, limit=limit)

    async def _get_top_customers(tenant_id: str, from_date: str, to_date: str) -> dict[str, Any]:
        _ = tenant_id
        return await customers.get_top_customers(client, auth, from_date=from_date, to_date=to_date)

    async def _search_customers(tenant_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        _ = tenant_id
        return await customers.search_customers(client, auth, query=query, limit=limit)

    async def _search_products(tenant_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        _ = tenant_id
        return await products.search_products(client, auth, query=query, limit=limit)

    async def _search_services(tenant_id: str, query: str = "", limit: int = 20) -> dict[str, Any]:
        _ = tenant_id
        return await services.search_services(client, auth, query=query, limit=limit)

    async def _get_service(tenant_id: str, service_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await services.get_service(client, auth, service_id=service_id)

    async def _get_low_stock(tenant_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await inventory.get_low_stock(client, auth)

    async def _get_stock_level(tenant_id: str, product_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await inventory.get_stock_level(client, auth, product_id=product_id)

    async def _get_cashflow_summary(tenant_id: str, from_date: str, to_date: str) -> dict[str, Any]:
        _ = tenant_id
        return await cashflow.get_cashflow_summary(client, auth, from_date=from_date, to_date=to_date)

    async def _get_account_balances(tenant_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await accounts.get_account_balances(client, auth)

    async def _get_debtors(tenant_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await accounts.get_debtors(client, auth)

    async def _get_bookings(tenant_id: str, from_date: str | None = None, to_date: str | None = None) -> dict[str, Any]:
        _ = tenant_id
        return await scheduling.get_bookings(client, auth, from_date=from_date, to_date=to_date)

    async def _check_availability(tenant_id: str, date: str, duration: int = 60) -> dict[str, Any]:
        return await scheduling.check_availability(client, tenant_id=tenant_id, date=date, duration=duration)

    async def _get_quotes(tenant_id: str, status: str | None = None) -> dict[str, Any]:
        _ = tenant_id
        return await quotes.get_quotes(client, auth, status=status)

    async def _get_purchases(tenant_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await purchases.get_purchases_summary(client, auth)

    async def _get_recurring_expenses(tenant_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await recurring.get_recurring_expenses(client, auth)

    async def _get_exchange_rates(tenant_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await currency.get_exchange_rates(client, auth)

    async def _create_quote(tenant_id: str, customer_name: str, items: list[dict[str, Any]], notes: str = "") -> dict[str, Any]:
        _ = tenant_id
        return await quotes.create_quote(client, auth, customer_name=customer_name, items=items, notes=notes)

    async def _create_sale(
        tenant_id: str,
        customer_name: str,
        items: list[dict[str, Any]],
        payment_method: str = "cash",
        notes: str = "",
    ) -> dict[str, Any]:
        _ = tenant_id
        return await sales.create_sale(
            client,
            auth,
            customer_name=customer_name,
            items=items,
            payment_method=payment_method,
            notes=notes,
        )

    async def _generate_payment_link(tenant_id: str, sale_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await payments.generate_payment_link(client, auth, sale_id=sale_id)

    async def _get_payment_status(tenant_id: str, sale_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await payments.get_payment_status(client, auth, sale_id=sale_id)

    async def _send_payment_info(tenant_id: str, sale_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await payments.send_payment_info(client, auth, sale_id=sale_id)

    async def _book_scheduling(
        tenant_id: str,
        customer_name: str,
        customer_phone: str,
        title: str,
        start_at: str,
        duration: int = 60,
    ) -> dict[str, Any]:
        return await scheduling.book_scheduling(
            client,
            tenant_id=tenant_id,
            customer_name=customer_name,
            customer_phone=customer_phone,
            title=title,
            start_at=start_at,
            duration=duration,
        )

    async def _create_cash_movement(
        tenant_id: str,
        movement_type: str,
        amount: float,
        category: str = "other",
        description: str = "",
    ) -> dict[str, Any]:
        _ = tenant_id
        return await cashflow.create_cash_movement(
            client,
            auth,
            movement_type=movement_type,
            amount=amount,
            category=category,
            description=description,
        )

    async def _search_help(tenant_id: str, query: str) -> dict[str, Any]:
        _ = tenant_id
        return await help.search_help_docs(query)

    register_profile_tools(
        declarations=declarations,
        handlers=handlers,
        role=role,
        modules_active=modules_active,
        client=client,
        auth=auth,
        dossier=dossier,
        add_tool=_maybe_add,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "get_sales_summary",
            "Ventas por periodo",
            {
                "type": "object",
                "properties": {"period": {"type": "string", "description": "today, week, month"}},
            },
        ),
        _get_sales_summary,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "get_recent_sales",
            "Ultimas ventas",
            {
                "type": "object",
                "properties": {"limit": {"type": "integer", "description": "max 50"}},
            },
        ),
        _get_recent_sales,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "get_top_customers",
            "Top clientes por facturacion",
            {
                "type": "object",
                "properties": {
                    "from_date": {"type": "string", "description": "YYYY-MM-DD"},
                    "to_date": {"type": "string", "description": "YYYY-MM-DD"},
                },
                "required": ["from_date", "to_date"],
            },
        ),
        _get_top_customers,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "search_customers",
            "Buscar clientes",
            {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "texto a buscar"},
                    "limit": {"type": "integer", "description": "max 100"},
                },
                "required": ["query"],
            },
        ),
        _search_customers,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "search_products",
            "Buscar productos",
            {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "texto a buscar"},
                    "limit": {"type": "integer", "description": "max 100"},
                },
                "required": ["query"],
            },
        ),
        _search_products,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "search_services",
            "Buscar servicios del catalogo",
            {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "texto a buscar (nombre, codigo o categoria)"},
                    "limit": {"type": "integer", "description": "max 100"},
                },
            },
        ),
        _search_services,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "get_service",
            "Detalle de un servicio del catalogo",
            {
                "type": "object",
                "properties": {"service_id": {"type": "string", "description": "UUID del servicio"}},
                "required": ["service_id"],
            },
        ),
        _get_service,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool("get_low_stock", "Productos con stock bajo", {"type": "object", "properties": {}}),
        _get_low_stock,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "get_stock_level",
            "Stock de un producto",
            {
                "type": "object",
                "properties": {"product_id": {"type": "string", "description": "UUID del producto"}},
                "required": ["product_id"],
            },
        ),
        _get_stock_level,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "get_cashflow_summary",
            "Resumen de caja por rango",
            {
                "type": "object",
                "properties": {
                    "from_date": {"type": "string", "description": "YYYY-MM-DD"},
                    "to_date": {"type": "string", "description": "YYYY-MM-DD"},
                },
                "required": ["from_date", "to_date"],
            },
        ),
        _get_cashflow_summary,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool("get_account_balances", "Resumen de cuentas corrientes", {"type": "object", "properties": {}}),
        _get_account_balances,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool("get_debtors", "Clientes deudores", {"type": "object", "properties": {}}),
        _get_debtors,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "get_bookings",
            "Listar turnos",
            {
                "type": "object",
                "properties": {
                    "from_date": {"type": "string", "description": "YYYY-MM-DD"},
                    "to_date": {"type": "string", "description": "YYYY-MM-DD"},
                },
            },
        ),
        _get_bookings,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "check_availability",
            "Consultar disponibilidad de turnos",
            {
                "type": "object",
                "properties": {
                    "date": {"type": "string", "description": "YYYY-MM-DD"},
                    "duration": {"type": "integer", "description": "duracion en minutos"},
                },
                "required": ["date"],
            },
        ),
        _check_availability,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "get_quotes",
            "Listar presupuestos",
            {
                "type": "object",
                "properties": {
                    "status": {
                        "type": "string",
                        "description": "draft, sent, accepted, rejected, expired",
                    }
                },
            },
        ),
        _get_quotes,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool("get_purchases", "Resumen de compras", {"type": "object", "properties": {}}),
        _get_purchases,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool("get_recurring_expenses", "Gastos recurrentes", {"type": "object", "properties": {}}),
        _get_recurring_expenses,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool("get_exchange_rates", "Cotizaciones del dia", {"type": "object", "properties": {}}),
        _get_exchange_rates,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "create_quote",
            "Crear presupuesto",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "items": {"type": "array", "items": {"type": "object"}},
                    "notes": {"type": "string"},
                },
                "required": ["customer_name", "items"],
            },
        ),
        _create_quote,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "create_sale",
            "Crear venta",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "items": {"type": "array", "items": {"type": "object"}},
                    "payment_method": {"type": "string"},
                    "notes": {"type": "string"},
                },
                "required": ["customer_name", "items"],
            },
        ),
        _create_sale,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "generate_payment_link",
            "Generar link de pago para una venta",
            {
                "type": "object",
                "properties": {"sale_id": {"type": "string", "description": "UUID de la venta"}},
                "required": ["sale_id"],
            },
        ),
        _generate_payment_link,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "get_payment_status",
            "Consultar estado de link de pago de una venta",
            {
                "type": "object",
                "properties": {"sale_id": {"type": "string", "description": "UUID de la venta"}},
                "required": ["sale_id"],
            },
        ),
        _get_payment_status,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "send_payment_info",
            "Obtener link de WhatsApp con datos de transferencia de una venta",
            {
                "type": "object",
                "properties": {"sale_id": {"type": "string", "description": "UUID de la venta"}},
                "required": ["sale_id"],
            },
        ),
        _send_payment_info,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "book_scheduling",
            "Reservar turno",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "customer_phone": {"type": "string"},
                    "title": {"type": "string"},
                    "start_at": {"type": "string", "description": "RFC3339"},
                    "duration": {"type": "integer"},
                },
                "required": ["customer_name", "customer_phone", "title", "start_at"],
            },
        ),
        _book_scheduling,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "create_cash_movement",
            "Crear movimiento de caja",
            {
                "type": "object",
                "properties": {
                    "movement_type": {"type": "string", "description": "income o expense"},
                    "amount": {"type": "number"},
                    "category": {"type": "string"},
                    "description": {"type": "string"},
                },
                "required": ["movement_type", "amount"],
            },
        ),
        _create_cash_movement,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "search_help",
            "Buscar ayuda funcional",
            {
                "type": "object",
                "properties": {"query": {"type": "string"}},
                "required": ["query"],
            },
        ),
        _search_help,
    )

    return declarations, handlers
