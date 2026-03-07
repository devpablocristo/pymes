from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def generate_payment_link(client: BackendClient, auth: AuthContext, sale_id: str) -> dict:
    return await client.request("POST", f"/v1/sales/{sale_id}/payment-link", auth=auth)


async def get_payment_status(client: BackendClient, auth: AuthContext, sale_id: str) -> dict:
    return await client.request("GET", f"/v1/sales/{sale_id}/payment-link", auth=auth)


async def send_payment_info(client: BackendClient, auth: AuthContext, sale_id: str) -> dict:
    return await client.request("GET", f"/v1/whatsapp/sale/{sale_id}/payment-info", auth=auth)


async def get_public_quote_payment_link(client: BackendClient, org_id: str, quote_id: str) -> dict:
    return await client.request(
        "GET",
        f"/v1/public/{org_id}/quote/{quote_id}/payment-link",
        include_internal=True,
    )
