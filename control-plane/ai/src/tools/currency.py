from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_exchange_rates(client: BackendClient, auth: AuthContext) -> dict:
    return {"error": "exchange rates endpoint not available yet"}
