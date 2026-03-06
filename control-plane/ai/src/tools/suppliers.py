from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def search_suppliers(client: BackendClient, auth: AuthContext, query: str, limit: int = 10) -> dict:
    return await client.request(
        "GET",
        "/v1/suppliers",
        auth=auth,
        params={"search": query, "limit": max(1, min(limit, 100))},
    )
