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
    # Usa el catálogo rico de pymes-core y adapta el shape a lo que esperan
    # los agentes (id, name, price, currency, unit).
    raw = await client.request(
        "GET",
        f"/v1/public/{org_id}/catalog/services",
        include_internal=True,
        params={"limit": max(1, min(limit, 100))},
    )
    items: list[dict] = []
    for row in (raw or {}).get("items", []) or []:
        items.append(
            {
                "id": row.get("id", ""),
                "name": row.get("name", ""),
                "description": row.get("description", ""),
                "unit": "unit",
                "price": float(row.get("sale_price") or 0),
                "currency": row.get("currency", "") or "ARS",
            }
        )
    return {"items": items}
