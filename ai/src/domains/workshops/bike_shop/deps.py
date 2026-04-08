from __future__ import annotations

from fastapi import Request

from src.domains.workshops.bike_shop.backend_client import BikeShopBackendClient

# Re-exportados para Depends() en routers del dominio.
from src.api.state_deps import get_auth_context, get_llm_provider, get_state_attr  # noqa: F401


def get_bike_shop_backend_client(request: Request) -> BikeShopBackendClient:
    return get_state_attr(
        request,
        "bike_shop_backend_client",
        "bike shop backend client not initialized",
    )
