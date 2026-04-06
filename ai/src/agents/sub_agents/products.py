"""Sub-agente: Productos — catálogo, stock y disponibilidad."""

from __future__ import annotations

from typing import Any

from runtime.domain.agent import SubAgent, SubAgentDescriptor
from runtime.types import ToolDeclaration

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.tools import inventory, products
from src.agents.sub_agents.common import build_default_limits

DESCRIPTOR = SubAgentDescriptor(
    name="products",
    description="Buscar productos, consultar stock actual y detectar stock bajo",
)

SYSTEM_PROMPT = """\
Sos el agente de productos e inventario de una plataforma de gestion para PyMEs.
Podes buscar productos, consultar el nivel de stock de un producto, y listar productos con stock bajo.
Si el usuario pide stock bajo, faltantes, inventario critico o productos para reponer, usa get_low_stock.
Si el usuario pregunta por el stock de un producto puntual, intenta identificarlo y usa get_stock_level.
Si el usuario pide buscar o listar productos, usa search_products antes de responder.
Responde siempre en espanol, claro y directo. No muestres JSON al usuario."""


def build(client: BackendClient, auth: AuthContext) -> SubAgent:
    async def search_products(*, org_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        return await products.search_products(client, auth, query=query, limit=limit)

    async def get_stock_level(*, org_id: str, product_id: str) -> dict[str, Any]:
        return await inventory.get_stock_level(client, auth, product_id=product_id)

    async def get_low_stock(*, org_id: str) -> dict[str, Any]:
        return await inventory.get_low_stock(client, auth)

    tools = [
        ToolDeclaration(
            name="search_products",
            description="Buscar productos por nombre, codigo o categoria",
            parameters={"type": "object", "properties": {"query": {"type": "string"}, "limit": {"type": "integer"}}, "required": ["query"]},
        ),
        ToolDeclaration(
            name="get_stock_level",
            description="Consultar stock actual de un producto especifico",
            parameters={"type": "object", "properties": {"product_id": {"type": "string"}}, "required": ["product_id"]},
        ),
        ToolDeclaration(
            name="get_low_stock",
            description="Listar productos con stock bajo el minimo configurado",
            parameters={"type": "object", "properties": {}},
        ),
    ]

    return SubAgent(
        descriptor=DESCRIPTOR,
        tools=tools,
        tool_handlers={
            "search_products": search_products,
            "get_stock_level": get_stock_level,
            "get_low_stock": get_low_stock,
        },
        system_prompt=SYSTEM_PROMPT,
        limits=build_default_limits(),
    )
