"""Sub-agente: Servicios — catálogo de servicios ofrecidos por el negocio."""

from __future__ import annotations

from typing import Any

from runtime.domain.agent import SubAgent, SubAgentDescriptor
from runtime.types import ToolDeclaration

from src.agents.sub_agents.common import build_default_limits
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.tools import services

DESCRIPTOR = SubAgentDescriptor(
    name="services",
    description="Buscar y consultar servicios del catalogo del negocio",
)

SYSTEM_PROMPT = """\
Sos el agente de servicios de una plataforma de gestion para PyMEs.
Podes buscar servicios del catalogo, consultar precios, duracion estimada y categoria, y ver el detalle de un servicio puntual.
Si el usuario pide listar, contar, buscar o revisar servicios, usa search_services sin pedir aclaraciones innecesarias.
Si el usuario pregunta por un servicio especifico, intenta identificarlo y usa get_service.
Responde siempre en espanol, claro y directo. No muestres JSON al usuario."""


def build(client: BackendClient, auth: AuthContext) -> SubAgent:
    async def search_services_handler(*, org_id: str, query: str = "", limit: int = 20) -> dict[str, Any]:
        return await services.search_services(client, auth, query=query, limit=limit)

    async def get_service_handler(*, org_id: str, service_id: str) -> dict[str, Any]:
        return await services.get_service(client, auth, service_id=service_id)

    tools = [
        ToolDeclaration(
            name="search_services",
            description="Buscar servicios del catalogo por nombre, codigo o categoria",
            parameters={
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Texto a buscar (nombre, codigo o categoria)"},
                    "limit": {"type": "integer", "description": "Cantidad maxima de resultados"},
                },
            },
        ),
        ToolDeclaration(
            name="get_service",
            description="Ver el detalle de un servicio especifico del catalogo",
            parameters={
                "type": "object",
                "properties": {"service_id": {"type": "string", "description": "UUID del servicio"}},
                "required": ["service_id"],
            },
        ),
    ]

    return SubAgent(
        descriptor=DESCRIPTOR,
        tools=tools,
        tool_handlers={
            "search_services": search_services_handler,
            "get_service": get_service_handler,
        },
        system_prompt=SYSTEM_PROMPT,
        limits=build_default_limits(),
    )
