from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_recurring_expenses(client: BackendClient, auth: AuthContext) -> dict:
    return await client.request(
        "GET",
        "/v1/recurring-expenses",
        auth=auth,
        params={"active": "true", "limit": 20},
    )
