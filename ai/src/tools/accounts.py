from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_account_balances(client: BackendClient, auth: AuthContext) -> dict:
    return await client.request(
        "GET",
        "/v1/accounts",
        auth=auth,
        params={"non_zero": "true", "limit": 20},
    )


async def get_debtors(client: BackendClient, auth: AuthContext) -> dict:
    return await client.request("GET", "/v1/accounts/debtors", auth=auth, params={"limit": 20})
