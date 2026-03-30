from __future__ import annotations

from typing import Final

try:
    from runtime import (
        ALL_ROUTING_SOURCES,
        ROUTING_SOURCE_COPILOT_AGENT,
        ROUTING_SOURCE_ORCHESTRATOR,
        ROUTING_SOURCE_READ_FALLBACK,
        is_known_routing_source,
        normalize_routing_source,
    )
except ImportError:
    ROUTING_SOURCE_COPILOT_AGENT: Final[str] = "copilot_agent"
    ROUTING_SOURCE_ORCHESTRATOR: Final[str] = "orchestrator"
    ROUTING_SOURCE_READ_FALLBACK: Final[str] = "read_fallback"
    ALL_ROUTING_SOURCES: Final[tuple[str, ...]] = (
        ROUTING_SOURCE_COPILOT_AGENT,
        ROUTING_SOURCE_ORCHESTRATOR,
        ROUTING_SOURCE_READ_FALLBACK,
    )

    def is_known_routing_source(name: str | None) -> bool:
        return bool(name and name in ALL_ROUTING_SOURCES)

    def normalize_routing_source(name: str | None) -> str:
        if is_known_routing_source(name):
            return str(name)
        return ROUTING_SOURCE_ORCHESTRATOR

PRODUCT_AGENT_NAME: Final[str] = "general"
COPILOT_AGENT_NAME: Final[str] = "copilot"

CLIENTES_DOMAIN_AGENT_NAME: Final[str] = "clientes"
PRODUCTOS_DOMAIN_AGENT_NAME: Final[str] = "productos"
VENTAS_DOMAIN_AGENT_NAME: Final[str] = "ventas"
COBROS_DOMAIN_AGENT_NAME: Final[str] = "cobros"
COMPRAS_DOMAIN_AGENT_NAME: Final[str] = "compras"

DOMAIN_AGENT_NAMES: Final[tuple[str, ...]] = (
    CLIENTES_DOMAIN_AGENT_NAME,
    PRODUCTOS_DOMAIN_AGENT_NAME,
    VENTAS_DOMAIN_AGENT_NAME,
    COBROS_DOMAIN_AGENT_NAME,
    COMPRAS_DOMAIN_AGENT_NAME,
)

ALL_ROUTED_AGENT_NAMES: Final[tuple[str, ...]] = (
    PRODUCT_AGENT_NAME,
    COPILOT_AGENT_NAME,
    *DOMAIN_AGENT_NAMES,
)


def is_known_routed_agent(name: str | None) -> bool:
    return bool(name and name in ALL_ROUTED_AGENT_NAMES)


def normalize_routed_agent(name: str | None) -> str:
    if is_known_routed_agent(name):
        return str(name)
    return PRODUCT_AGENT_NAME
