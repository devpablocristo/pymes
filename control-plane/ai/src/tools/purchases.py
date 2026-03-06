from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_purchases_summary(client: BackendClient, auth: AuthContext) -> dict:
    return await client.request("GET", "/v1/purchases", auth=auth, params={"limit": 20})
