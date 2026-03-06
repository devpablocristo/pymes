from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_appointments(client: BackendClient, auth: AuthContext, from_date: str | None = None, to_date: str | None = None) -> dict:
    params = {}
    if from_date:
        params["from"] = from_date
    if to_date:
        params["to"] = to_date
    return await client.request("GET", "/v1/appointments", auth=auth, params=params)


async def check_availability(client: BackendClient, org_id: str, date: str, duration: int = 60) -> dict:
    return await client.request(
        "GET",
        f"/v1/public/{org_id}/availability",
        include_internal=True,
        params={"date": date, "duration": duration},
    )


async def book_appointment(
    client: BackendClient,
    org_id: str,
    customer_name: str,
    customer_phone: str,
    title: str,
    start_at: str,
    duration: int = 60,
) -> dict:
    payload = {
        "customer_name": customer_name,
        "customer_phone": customer_phone,
        "title": title,
        "start_at": start_at,
        "duration": duration,
    }
    return await client.request("POST", f"/v1/public/{org_id}/book", include_internal=True, json=payload)


async def get_my_appointments(client: BackendClient, org_id: str, phone: str) -> dict:
    return await client.request(
        "GET",
        f"/v1/public/{org_id}/my-appointments",
        include_internal=True,
        params={"phone": phone},
    )
