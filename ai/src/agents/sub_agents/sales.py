"""Sub-agente: Ventas — presupuestos, ventas y operaciones comerciales."""

from __future__ import annotations

from typing import Any

from runtime.domain.agent import SubAgent, SubAgentDescriptor
from runtime.types import ToolDeclaration

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.tools import quotes, sales
from src.agents.sub_agents.common import build_default_limits

DESCRIPTOR = SubAgentDescriptor(
    name="sales",
    description="Crear presupuestos, registrar ventas, consultar ventas recientes y presupuestos",
)

SYSTEM_PROMPT = """\
Sos el agente de ventas de una plataforma de gestion para PyMEs.
Podes crear presupuestos, listar presupuestos existentes, registrar ventas y consultar ventas recientes.
Si el usuario pide un resumen, conteo o estado de ventas recientes, usa get_recent_sales sin pedir aclaraciones innecesarias.
Si el usuario pide presupuestos existentes, usa get_quotes primero y luego resume.
Responde siempre en espanol, claro y directo. No muestres JSON al usuario.
Si una accion requiere confirmacion, pedi confirmacion explicita."""


def build(client: BackendClient, auth: AuthContext) -> SubAgent:
    async def get_quotes(*, org_id: str, status_filter: str | None = None) -> dict[str, Any]:
        return await quotes.get_quotes(client, auth, status=status_filter)

    async def create_quote(*, org_id: str, customer_name: str, items: list[dict[str, Any]], notes: str = "") -> dict[str, Any]:
        return await quotes.create_quote(client, auth, customer_name=customer_name, items=items, notes=notes)

    async def create_sale(*, org_id: str, customer_name: str, items: list[dict[str, Any]], payment_method: str = "cash", notes: str = "") -> dict[str, Any]:
        return await sales.create_sale(client, auth, customer_name=customer_name, items=items, payment_method=payment_method, notes=notes)

    async def get_recent_sales(*, org_id: str, limit: int = 10) -> dict[str, Any]:
        return await sales.get_recent_sales(client, auth, limit=limit)

    tools = [
        ToolDeclaration(
            name="get_quotes",
            description="Listar presupuestos (opcionalmente filtrar por estado)",
            parameters={"type": "object", "properties": {"status_filter": {"type": "string", "description": "pending, accepted, rejected"}}},
        ),
        ToolDeclaration(
            name="create_quote",
            description="Crear un presupuesto comercial para un cliente",
            parameters={
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "notes": {"type": "string"},
                    "items": {"type": "array", "items": {"type": "object", "properties": {"product_id": {"type": "string"}, "name": {"type": "string"}, "quantity": {"type": "number"}}}},
                },
                "required": ["customer_name", "items"],
            },
        ),
        ToolDeclaration(
            name="create_sale",
            description="Registrar una venta",
            parameters={
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "payment_method": {"type": "string", "description": "cash, card, transfer"},
                    "notes": {"type": "string"},
                    "items": {"type": "array", "items": {"type": "object"}},
                },
                "required": ["customer_name", "items"],
            },
        ),
        ToolDeclaration(
            name="get_recent_sales",
            description="Consultar las ultimas ventas realizadas",
            parameters={"type": "object", "properties": {"limit": {"type": "integer"}}},
        ),
    ]

    return SubAgent(
        descriptor=DESCRIPTOR,
        tools=tools,
        tool_handlers={
            "get_quotes": get_quotes,
            "create_quote": create_quote,
            "create_sale": create_sale,
            "get_recent_sales": get_recent_sales,
        },
        system_prompt=SYSTEM_PROMPT,
        limits=build_default_limits(),
    )
