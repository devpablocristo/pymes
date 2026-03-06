from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_account_balances(client: BackendClient, auth: AuthContext) -> dict:
    return {"error": "accounts module not available yet"}


async def get_debtors(client: BackendClient, auth: AuthContext) -> dict:
    return {"error": "accounts module not available yet"}
