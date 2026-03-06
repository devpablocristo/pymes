from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_recurring_expenses(client: BackendClient, auth: AuthContext) -> dict:
    return {"error": "recurring module not available yet"}
