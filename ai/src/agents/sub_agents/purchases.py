"""Sub-agente: Compras — proveedores, solicitudes de compra y abastecimiento."""

from __future__ import annotations

from typing import Any

from runtime.domain.agent import SubAgent, SubAgentDescriptor
from runtime.types import ToolDeclaration

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.tools import inventory, procurement_requests, purchases, suppliers
from src.agents.sub_agents.common import build_default_limits

DESCRIPTOR = SubAgentDescriptor(
    name="purchases",
    description="Buscar proveedores, crear solicitudes de compra, consultar compras y preparar borradores de orden",
)

SYSTEM_PROMPT = """\
Sos el agente de compras y abastecimiento de una plataforma de gestion para PyMEs.
Podes buscar proveedores, crear solicitudes internas de compra, listar compras, y preparar borradores de orden.
No emitas compras finales automaticamente. Limita la respuesta a analisis, sugerencias y borradores.
Si el usuario pide sugerencias de reposicion, faltantes o que prepares una compra, usa prepare_purchase_draft.
Si faltan datos menores, intenta igual con stock bajo antes de pedir aclaraciones.
Responde siempre en espanol, claro y directo. No muestres JSON al usuario."""


def build(client: BackendClient, auth: AuthContext) -> SubAgent:
    async def search_suppliers(*, org_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        return await suppliers.search_suppliers(client, auth, query=query, limit=limit)

    async def get_purchases(*, org_id: str) -> dict[str, Any]:
        return await purchases.get_purchases_summary(client, auth)

    async def list_procurement_requests(*, org_id: str, limit: int = 20, archived: bool = False) -> dict[str, Any]:
        return await procurement_requests.list_procurement_requests(client, auth, limit=limit, archived=archived)

    async def create_procurement_request(
        *, org_id: str, title: str, description: str = "", category: str = "",
        estimated_total: float = 0, currency: str = "ARS", lines: list[dict[str, Any]] | None = None,
    ) -> dict[str, Any]:
        return await procurement_requests.create_procurement_request(
            client, auth, title=title, description=description, category=category,
            estimated_total=estimated_total, currency=currency, lines=lines,
        )

    async def submit_procurement_request(*, org_id: str, request_id: str) -> dict[str, Any]:
        return await procurement_requests.submit_procurement_request(client, auth, request_id=request_id)

    async def prepare_purchase_draft(*, org_id: str, supplier_name: str | None = None, items: list[dict[str, Any]] | None = None) -> dict[str, Any]:
        draft_items: list[dict[str, Any]] = []
        if items:
            for item in items:
                quantity = float(item.get("quantity", 0) or 0)
                if quantity <= 0:
                    continue
                draft_items.append({
                    "product_id": str(item.get("product_id", "")).strip(),
                    "name": str(item.get("name", "")).strip(),
                    "recommended_quantity": quantity,
                    "reason": "requested_by_user",
                })
        if not draft_items:
            low_stock = await inventory.get_low_stock(client, auth)
            for row in list(low_stock.get("items", []))[:10]:
                current_qty = float(row.get("quantity", 0) or 0)
                min_qty = float(row.get("min_quantity", 0) or 0)
                suggested = max(min_qty * 2 - current_qty, min_qty - current_qty, 0)
                if suggested <= 0:
                    continue
                draft_items.append({
                    "product_id": str(row.get("product_id", "")).strip(),
                    "name": str(row.get("product_name", "")).strip(),
                    "recommended_quantity": round(suggested, 2),
                    "current_quantity": current_qty,
                    "min_quantity": min_qty,
                    "reason": "low_stock",
                })
        return {
            "status": "draft_ready",
            "supplier_name": (supplier_name or "").strip(),
            "items": draft_items,
            "final_purchase_created": False,
            "next_step": "Revisa el borrador y confirma con un comprador o admin antes de emitir la orden.",
        }

    tools = [
        ToolDeclaration(name="search_suppliers", description="Buscar proveedores por nombre o rubro", parameters={"type": "object", "properties": {"query": {"type": "string"}, "limit": {"type": "integer"}}, "required": ["query"]}),
        ToolDeclaration(name="get_purchases", description="Consultar compras recientes", parameters={"type": "object", "properties": {}}),
        ToolDeclaration(name="list_procurement_requests", description="Listar solicitudes internas de compra", parameters={"type": "object", "properties": {"limit": {"type": "integer"}, "archived": {"type": "boolean"}}}),
        ToolDeclaration(
            name="create_procurement_request",
            description="Crear borrador de solicitud interna de compra",
            parameters={"type": "object", "properties": {"title": {"type": "string"}, "description": {"type": "string"}, "category": {"type": "string"}, "estimated_total": {"type": "number"}, "currency": {"type": "string"}, "lines": {"type": "array", "items": {"type": "object"}}}, "required": ["title"]},
        ),
        ToolDeclaration(name="submit_procurement_request", description="Enviar solicitud de compra a aprobacion", parameters={"type": "object", "properties": {"request_id": {"type": "string"}}, "required": ["request_id"]}),
        ToolDeclaration(name="prepare_purchase_draft", description="Preparar borrador de compra basado en stock bajo o items especificos", parameters={"type": "object", "properties": {"supplier_name": {"type": "string"}, "items": {"type": "array", "items": {"type": "object"}}}}),
    ]

    return SubAgent(
        descriptor=DESCRIPTOR,
        tools=tools,
        tool_handlers={
            "search_suppliers": search_suppliers,
            "get_purchases": get_purchases,
            "list_procurement_requests": list_procurement_requests,
            "create_procurement_request": create_procurement_request,
            "submit_procurement_request": submit_procurement_request,
            "prepare_purchase_draft": prepare_purchase_draft,
        },
        system_prompt=SYSTEM_PROMPT,
        limits=build_default_limits(),
    )
