from __future__ import annotations

from typing import Any

from src.agents.commercial_runtime import CommercialRunState, _wrap_tool
from src.agents.policy import CommercialPolicy
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from runtime.types import ToolDeclaration
from src.tools import inventory, procurement_requests, products, purchases, suppliers


def _tool(name: str, description: str, parameters: dict[str, Any]) -> ToolDeclaration:
    return ToolDeclaration(name=name, description=description, parameters=parameters)

async def _build_procurement_tools(
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

    async def _search_suppliers(org_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        _ = org_id
        return await suppliers.search_suppliers(client, auth, query=query, limit=limit)

    async def _search_products(org_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        _ = org_id
        return await products.search_products(client, auth, query=query, limit=limit)

    async def _get_low_stock(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await inventory.get_low_stock(client, auth)

    async def _get_stock_level(org_id: str, product_id: str) -> dict[str, Any]:
        _ = org_id
        return await inventory.get_stock_level(client, auth, product_id=product_id)

    async def _get_purchases(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await purchases.get_purchases_summary(client, auth)

    async def _list_procurement_requests(org_id: str, limit: int = 20, archived: bool = False) -> dict[str, Any]:
        _ = org_id
        return await procurement_requests.list_procurement_requests(client, auth, limit=limit, archived=archived)

    async def _create_procurement_request(
        org_id: str,
        title: str,
        description: str = "",
        category: str = "",
        estimated_total: float = 0,
        currency: str = "ARS",
        lines: list[dict[str, Any]] | None = None,
    ) -> dict[str, Any]:
        _ = org_id
        return await procurement_requests.create_procurement_request(
            client,
            auth,
            title=title,
            description=description,
            category=category,
            estimated_total=estimated_total,
            currency=currency,
            lines=lines,
        )

    async def _get_procurement_request(org_id: str, request_id: str) -> dict[str, Any]:
        _ = org_id
        return await procurement_requests.get_procurement_request(client, auth, request_id=request_id)

    async def _submit_procurement_request(org_id: str, request_id: str) -> dict[str, Any]:
        _ = org_id
        return await procurement_requests.submit_procurement_request(client, auth, request_id=request_id)

    async def _prepare_purchase_draft(
        org_id: str,
        supplier_name: str | None = None,
        items: list[dict[str, Any]] | None = None,
    ) -> dict[str, Any]:
        _ = org_id
        draft_items: list[dict[str, Any]] = []
        if items:
            for item in items:
                quantity = float(item.get("quantity", 0) or 0)
                if quantity <= 0:
                    continue
                draft_items.append(
                    {
                        "product_id": str(item.get("product_id", "")).strip(),
                        "name": str(item.get("name", "")).strip(),
                        "recommended_quantity": quantity,
                        "reason": "requested_by_user",
                    }
                )
        if not draft_items:
            low_stock = await inventory.get_low_stock(client, auth)
            for row in list(low_stock.get("items", []))[:10]:
                current_qty = float(row.get("quantity", 0) or 0)
                min_qty = float(row.get("min_quantity", 0) or 0)
                suggested = max(min_qty * 2 - current_qty, min_qty - current_qty, 0)
                if suggested <= 0:
                    continue
                draft_items.append(
                    {
                        "product_id": str(row.get("product_id", "")).strip(),
                        "name": str(row.get("product_name", "")).strip(),
                        "recommended_quantity": round(suggested, 2),
                        "current_quantity": current_qty,
                        "min_quantity": min_qty,
                        "reason": "low_stock",
                    }
                )
        return {
            "status": "draft_ready",
            "supplier_name": (supplier_name or "").strip(),
            "items": draft_items,
            "final_purchase_created": False,
            "next_step": "Revisa el borrador y confirma con un comprador o admin antes de emitir la orden.",
        }

    specs = [
        (_tool("search_suppliers", "Buscar proveedores", {"type": "object", "properties": {"query": {"type": "string"}, "limit": {"type": "integer"}}, "required": ["query"]}), _search_suppliers),
        (_tool("search_products", "Buscar productos", {"type": "object", "properties": {"query": {"type": "string"}, "limit": {"type": "integer"}}, "required": ["query"]}), _search_products),
        (_tool("get_low_stock", "Consultar stock bajo", {"type": "object", "properties": {}}), _get_low_stock),
        (_tool("get_stock_level", "Consultar stock de un producto", {"type": "object", "properties": {"product_id": {"type": "string"}}, "required": ["product_id"]}), _get_stock_level),
        (_tool("get_purchases", "Consultar compras recientes", {"type": "object", "properties": {}}), _get_purchases),
        (_tool("prepare_purchase_draft", "Preparar borrador de compra sin emitir la orden final", {"type": "object", "properties": {"supplier_name": {"type": "string"}, "items": {"type": "array", "items": {"type": "object"}}}}), _prepare_purchase_draft),
        (
            _tool(
                "list_procurement_requests",
                "Listar solicitudes internas de compra (borradores y enviadas)",
                {"type": "object", "properties": {"limit": {"type": "integer"}, "archived": {"type": "boolean"}}},
            ),
            _list_procurement_requests,
        ),
        (
            _tool(
                "create_procurement_request",
                "Crear borrador de solicitud interna de compra o gasto antes de enviarla a aprobacion",
                {
                    "type": "object",
                    "properties": {
                        "title": {"type": "string"},
                        "description": {"type": "string"},
                        "category": {"type": "string"},
                        "estimated_total": {"type": "number"},
                        "currency": {"type": "string"},
                        "lines": {"type": "array", "items": {"type": "object"}},
                    },
                    "required": ["title"],
                },
            ),
            _create_procurement_request,
        ),
        (
            _tool(
                "get_procurement_request",
                "Obtener detalle de una solicitud interna por id",
                {"type": "object", "properties": {"request_id": {"type": "string"}}, "required": ["request_id"]},
            ),
            _get_procurement_request,
        ),
        (
            _tool(
                "submit_procurement_request",
                "Enviar solicitud interna: aplica politicas (governance) y puede crear orden de compra en borrador si corresponde",
                {"type": "object", "properties": {"request_id": {"type": "string"}}, "required": ["request_id"]},
            ),
            _submit_procurement_request,
        ),
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
