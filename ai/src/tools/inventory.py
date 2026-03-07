from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_low_stock(client: BackendClient, auth: AuthContext) -> dict:
    return await client.request("GET", "/v1/reports/low-stock", auth=auth)


async def get_stock_level(client: BackendClient, auth: AuthContext, product_id: str) -> dict:
    return await client.request("GET", f"/v1/inventory/{product_id}", auth=auth)
