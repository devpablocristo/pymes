from __future__ import annotations

from datetime import UTC, date, datetime, timedelta

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


def _range(period: str) -> tuple[str, str]:
    today = datetime.now(UTC).date()
    p = (period or "today").lower()
    if p == "week":
        start = today - timedelta(days=today.weekday())
    elif p == "month":
        start = date(today.year, today.month, 1)
    else:
        start = today
    return start.isoformat(), today.isoformat()


async def get_sales_summary(client: BackendClient, auth: AuthContext, period: str = "today") -> dict:
    from_date, to_date = _range(period)
    return await client.request(
        "GET",
        "/v1/reports/sales-summary",
        auth=auth,
        params={"from": from_date, "to": to_date},
    )


async def get_recent_sales(client: BackendClient, auth: AuthContext, limit: int = 10) -> dict:
    return await client.request("GET", "/v1/sales", auth=auth, params={"limit": max(1, min(limit, 50))})


async def create_sale(
    client: BackendClient,
    auth: AuthContext,
    customer_name: str,
    items: list[dict],
    payment_method: str = "cash",
    notes: str = "",
) -> dict:
    payload = {
        "customer_name": customer_name,
        "payment_method": payment_method,
        "items": items,
        "notes": notes,
    }
    return await client.request("POST", "/v1/sales", auth=auth, json=payload)
