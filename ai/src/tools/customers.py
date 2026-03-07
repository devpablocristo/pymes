from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def search_customers(client: BackendClient, auth: AuthContext, query: str, limit: int = 10) -> dict:
    return await client.request(
        "GET",
        "/v1/customers",
        auth=auth,
        params={"search": query, "limit": max(1, min(limit, 100))},
    )


async def get_top_customers(client: BackendClient, auth: AuthContext, from_date: str, to_date: str) -> dict:
    return await client.request(
        "GET",
        "/v1/reports/sales-by-customer",
        auth=auth,
        params={"from": from_date, "to": to_date},
    )
