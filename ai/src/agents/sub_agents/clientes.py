"""Sub-agente: Clientes — búsqueda y gestión de clientes."""

from __future__ import annotations

from typing import Any

from runtime.domain.agent import SubAgent, SubAgentDescriptor
from runtime.types import ToolDeclaration

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.tools import customers
from src.agents.sub_agents.common import build_default_limits

DESCRIPTOR = SubAgentDescriptor(
    name="clientes",
    description="Buscar, consultar y crear clientes del negocio",
)

SYSTEM_PROMPT = """\
Sos el agente de clientes de una plataforma de gestion para PyMEs.
Podes buscar clientes por nombre/email, consultar datos de un cliente, o crear uno nuevo.
Si el usuario pide listar, contar, resumir o revisar clientes, usa la tool search_customers sin pedir aclaraciones innecesarias.
Si no te dan un nombre exacto, igual intenta una busqueda amplia y luego resume el resultado.
Responde siempre en espanol, claro y directo. No muestres JSON al usuario."""


def build(client: BackendClient, auth: AuthContext) -> SubAgent:
    """Construye el sub-agente de clientes."""

    async def search_customers(*, org_id: str, query: str = "", limit: int = 10) -> dict[str, Any]:
        return await customers.search_customers(client, auth, query=query, limit=limit)

    tools = [
        ToolDeclaration(
            name="search_customers",
            description="Buscar clientes por nombre, email o telefono",
            parameters={
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Texto de busqueda"},
                    "limit": {"type": "integer", "description": "Cantidad maxima de resultados"},
                },
            },
        ),
    ]

    return SubAgent(
        descriptor=DESCRIPTOR,
        tools=tools,
        tool_handlers={"search_customers": search_customers},
        system_prompt=SYSTEM_PROMPT,
        limits=build_default_limits(),
    )
