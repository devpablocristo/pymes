from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def search_services(client: BackendClient, auth: AuthContext, query: str = "", limit: int = 20) -> dict:
    params: dict[str, object] = {"limit": max(1, min(limit, 100))}
    if query:
        params["search"] = query
    return await client.request("GET", "/v1/services", auth=auth, params=params)


async def get_service(client: BackendClient, auth: AuthContext, service_id: str) -> dict:
    return await client.request("GET", f"/v1/services/{service_id}", auth=auth)
