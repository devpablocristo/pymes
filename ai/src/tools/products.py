from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def search_products(client: BackendClient, auth: AuthContext, query: str, limit: int = 10) -> dict:
    return await client.request(
        "GET",
        "/v1/products",
        auth=auth,
        params={"search": query, "limit": max(1, min(limit, 100))},
    )


async def get_product(client: BackendClient, auth: AuthContext, product_id: str) -> dict:
    return await client.request("GET", f"/v1/products/{product_id}", auth=auth)


async def get_public_services(client: BackendClient, org_id: str, limit: int = 20) -> dict:
    return await client.request(
        "GET",
        f"/v1/public/{org_id}/services",
        include_internal=True,
        params={"limit": max(1, min(limit, 100))},
    )
