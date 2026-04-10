from __future__ import annotations

from typing import Final

try:
    from runtime import (
        ALL_ROUTING_SOURCES,
        ROUTING_SOURCE_COPILOT_AGENT,
        ROUTING_SOURCE_ORCHESTRATOR,
        ROUTING_SOURCE_READ_FALLBACK,
        ROUTING_SOURCE_UI_HINT,
        is_known_routing_source,
        normalize_routing_source,
    )
except ImportError:
    ROUTING_SOURCE_COPILOT_AGENT: Final[str] = "copilot_agent"
    ROUTING_SOURCE_ORCHESTRATOR: Final[str] = "orchestrator"
    ROUTING_SOURCE_READ_FALLBACK: Final[str] = "read_fallback"
    ROUTING_SOURCE_UI_HINT: Final[str] = "ui_hint"
    ALL_ROUTING_SOURCES: Final[tuple[str, ...]] = (
        ROUTING_SOURCE_COPILOT_AGENT,
        ROUTING_SOURCE_ORCHESTRATOR,
        ROUTING_SOURCE_READ_FALLBACK,
        ROUTING_SOURCE_UI_HINT,
    )

    def is_known_routing_source(name: str | None) -> bool:
        return bool(name and name in ALL_ROUTING_SOURCES)

    def normalize_routing_source(name: str | None) -> str:
        if is_known_routing_source(name):
            return str(name)
        return ROUTING_SOURCE_ORCHESTRATOR

# Alias semántico: mismo valor que core exporta ("copilot_agent") hasta que core migre.
ROUTING_SOURCE_INSIGHT_CHAT_AGENT: Final[str] = ROUTING_SOURCE_COPILOT_AGENT

PRODUCT_AGENT_NAME: Final[str] = "general"
INSIGHT_CHAT_AGENT_NAME: Final[str] = "insight_chat"

CUSTOMERS_DOMAIN_AGENT_NAME: Final[str] = "customers"
PRODUCTS_DOMAIN_AGENT_NAME: Final[str] = "products"
SERVICES_DOMAIN_AGENT_NAME: Final[str] = "services"
SALES_DOMAIN_AGENT_NAME: Final[str] = "sales"
COLLECTIONS_DOMAIN_AGENT_NAME: Final[str] = "collections"
PURCHASES_DOMAIN_AGENT_NAME: Final[str] = "purchases"

DOMAIN_AGENT_NAMES: Final[tuple[str, ...]] = (
    CUSTOMERS_DOMAIN_AGENT_NAME,
    PRODUCTS_DOMAIN_AGENT_NAME,
    SERVICES_DOMAIN_AGENT_NAME,
    SALES_DOMAIN_AGENT_NAME,
    COLLECTIONS_DOMAIN_AGENT_NAME,
    PURCHASES_DOMAIN_AGENT_NAME,
)

ALL_ROUTED_AGENT_NAMES: Final[tuple[str, ...]] = (
    PRODUCT_AGENT_NAME,
    INSIGHT_CHAT_AGENT_NAME,
    *DOMAIN_AGENT_NAMES,
)

def is_known_routed_agent(name: str | None) -> bool:
    return bool(name and name in ALL_ROUTED_AGENT_NAMES)


def normalize_routed_agent(name: str | None) -> str:
    if is_known_routed_agent(name):
        return str(name)
    return PRODUCT_AGENT_NAME
