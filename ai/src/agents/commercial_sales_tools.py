from __future__ import annotations

from typing import Any

from src.agents.commercial_runtime import CommercialRunState, _wrap_tool
from src.agents.policy import CommercialPolicy
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from runtime.types import ToolDeclaration
from src.tools import accounts, customers, inventory, payments, products, quotes, sales


def _tool(name: str, description: str, parameters: dict[str, Any]) -> ToolDeclaration:
    return ToolDeclaration(name=name, description=description, parameters=parameters)

async def _build_internal_sales_tools(
    *,
    client: BackendClient,
    auth: AuthContext,
    repo: AIRepository,
    org_id: str,
    conversation_id: str | None,
    policy: CommercialPolicy,
    state: CommercialRunState,
    confirmed_actions: set[str],
) -> tuple[list[ToolDeclaration], dict[str, Any]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, Any] = {}

    async def _search_customers(org_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        _ = org_id
        return await customers.search_customers(client, auth, query=query, limit=limit)

    async def _search_products(org_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        _ = org_id
        return await products.search_products(client, auth, query=query, limit=limit)

    async def _get_low_stock(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await inventory.get_low_stock(client, auth)

    async def _get_stock_level(org_id: str, product_id: str) -> dict[str, Any]:
        _ = org_id
        return await inventory.get_stock_level(client, auth, product_id=product_id)

    async def _get_quotes(org_id: str, status_filter: str | None = None) -> dict[str, Any]:
        _ = org_id
        return await quotes.get_quotes(client, auth, status=status_filter)

    async def _create_quote(org_id: str, customer_name: str, items: list[dict[str, Any]], notes: str = "") -> dict[str, Any]:
        _ = org_id
        return await quotes.create_quote(client, auth, customer_name=customer_name, items=items, notes=notes)

    async def _create_sale(
        org_id: str,
        customer_name: str,
        items: list[dict[str, Any]],
        payment_method: str = "cash",
        notes: str = "",
    ) -> dict[str, Any]:
        _ = org_id
        return await sales.create_sale(client, auth, customer_name=customer_name, items=items, payment_method=payment_method, notes=notes)

    async def _generate_payment_link(org_id: str, reference_type: str, reference_id: str) -> dict[str, Any]:
        _ = org_id
        kind = reference_type.strip().lower()
        if kind == "sale":
            return await client.request("POST", f"/v1/sales/{reference_id}/payment-link", auth=auth)
        if kind == "quote":
            return await client.request("POST", f"/v1/quotes/{reference_id}/payment-link", auth=auth)
        return {"code": "invalid_reference_type", "message": "reference_type debe ser sale o quote"}

    async def _get_payment_status(org_id: str, reference_type: str, reference_id: str) -> dict[str, Any]:
        _ = org_id
        kind = reference_type.strip().lower()
        if kind == "sale":
            return await payments.get_payment_status(client, auth, sale_id=reference_id)
        if kind == "quote":
            return await client.request("GET", f"/v1/quotes/{reference_id}/payment-link", auth=auth)
        return {"code": "invalid_reference_type", "message": "reference_type debe ser sale o quote"}

    async def _send_payment_info(org_id: str, sale_id: str) -> dict[str, Any]:
        _ = org_id
        return await payments.send_payment_info(client, auth, sale_id=sale_id)

    async def _get_account_balances(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await accounts.get_account_balances(client, auth)

    async def _get_recent_sales(org_id: str, limit: int = 10) -> dict[str, Any]:
        _ = org_id
        return await sales.get_recent_sales(client, auth, limit=limit)

    specs = [
        (_tool("search_customers", "Buscar clientes", {"type": "object", "properties": {"query": {"type": "string"}, "limit": {"type": "integer"}}, "required": ["query"]}), _search_customers),
        (_tool("search_products", "Buscar productos", {"type": "object", "properties": {"query": {"type": "string"}, "limit": {"type": "integer"}}, "required": ["query"]}), _search_products),
        (_tool("get_low_stock", "Consultar stock bajo", {"type": "object", "properties": {}}), _get_low_stock),
        (_tool("get_stock_level", "Consultar stock de un producto", {"type": "object", "properties": {"product_id": {"type": "string"}}, "required": ["product_id"]}), _get_stock_level),
        (_tool("get_quotes", "Listar presupuestos", {"type": "object", "properties": {"status_filter": {"type": "string"}}}), _get_quotes),
        (_tool("create_quote", "Crear presupuesto comercial", {"type": "object", "properties": {"customer_name": {"type": "string"}, "notes": {"type": "string"}, "items": {"type": "array", "items": {"type": "object"}}}, "required": ["customer_name", "items"]}), _create_quote),
        (_tool("create_sale", "Crear venta", {"type": "object", "properties": {"customer_name": {"type": "string"}, "payment_method": {"type": "string"}, "notes": {"type": "string"}, "items": {"type": "array", "items": {"type": "object"}}}, "required": ["customer_name", "items"]}), _create_sale),
        (_tool("generate_payment_link", "Generar link de pago para venta o presupuesto", {"type": "object", "properties": {"reference_type": {"type": "string", "description": "sale o quote"}, "reference_id": {"type": "string"}}, "required": ["reference_type", "reference_id"]}), _generate_payment_link),
        (_tool("get_payment_status", "Consultar estado de cobro o link", {"type": "object", "properties": {"reference_type": {"type": "string", "description": "sale o quote"}, "reference_id": {"type": "string"}}, "required": ["reference_type", "reference_id"]}), _get_payment_status),
        (_tool("send_payment_info", "Obtener mensaje de WhatsApp para cobro", {"type": "object", "properties": {"sale_id": {"type": "string"}}, "required": ["sale_id"]}), _send_payment_info),
        (_tool("get_account_balances", "Consultar cuentas corrientes", {"type": "object", "properties": {}}), _get_account_balances),
        (_tool("get_recent_sales", "Consultar ventas recientes", {"type": "object", "properties": {"limit": {"type": "integer"}}}), _get_recent_sales),
    ]

    for declaration, raw_handler in specs:
        if not policy.allows(declaration.name):
            continue
        declarations.append(declaration)
        handlers[declaration.name] = await _wrap_tool(
            name=declaration.name,
            handler=raw_handler,
            repo=repo,
            org_id=org_id,
            conversation_id=conversation_id,
            policy=policy,
            state=state,
            actor_id=auth.actor,
            actor_type="internal_user",
            channel=policy.channel,
            confirmed_actions=confirmed_actions,
        )
    return declarations, handlers
