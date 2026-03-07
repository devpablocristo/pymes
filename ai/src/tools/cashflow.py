from __future__ import annotations

from datetime import UTC, datetime

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_cashflow_summary(client: BackendClient, auth: AuthContext, from_date: str, to_date: str) -> dict:
    return await client.request(
        "GET",
        "/v1/reports/cashflow-summary",
        auth=auth,
        params={"from": from_date, "to": to_date},
    )


async def create_cash_movement(
    client: BackendClient,
    auth: AuthContext,
    movement_type: str,
    amount: float,
    category: str = "other",
    description: str = "",
) -> dict:
    payload = {
        "type": movement_type,
        "amount": amount,
        "category": category,
        "description": description,
        "occurred_at": datetime.now(UTC).isoformat(),
    }
    return await client.request("POST", "/v1/cashflow", auth=auth, json=payload)
