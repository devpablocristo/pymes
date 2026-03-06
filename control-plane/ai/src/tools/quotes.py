from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_quotes(client: BackendClient, auth: AuthContext, status: str | None = None) -> dict:
    params = {"status": status} if status else {}
    return await client.request("GET", "/v1/quotes", auth=auth, params=params)


async def create_quote(
    client: BackendClient,
    auth: AuthContext,
    customer_name: str,
    items: list[dict],
    notes: str = "",
) -> dict:
    return await client.request(
        "POST",
        "/v1/quotes",
        auth=auth,
        json={"customer_name": customer_name, "items": items, "notes": notes},
    )
