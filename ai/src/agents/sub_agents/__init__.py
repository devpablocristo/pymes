"""Registro de sub-agentes de Pymes.

Para agregar un nuevo sub-agente:
1. Crear un archivo en este directorio con DESCRIPTOR y build()
2. Importarlo aquí y registrarlo en build_registry()
"""

from __future__ import annotations

from runtime.domain.agent import AgentRegistry

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient

from . import collections, customers, products, purchases, sales


def build_registry(client: BackendClient, auth: AuthContext) -> AgentRegistry:
    """Construye el registro con todos los sub-agentes de pymes."""
    registry = AgentRegistry()
    registry.register(customers.build(client, auth))
    registry.register(products.build(client, auth))
    registry.register(sales.build(client, auth))
    registry.register(collections.build(client, auth))
    registry.register(purchases.build(client, auth))
    return registry
