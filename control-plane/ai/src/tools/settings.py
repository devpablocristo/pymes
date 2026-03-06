from __future__ import annotations

from typing import Any

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_tenant_settings(client: BackendClient, auth: AuthContext) -> dict[str, Any]:
    return await client.request("GET", "/v1/tenant-settings", auth=auth)


async def update_tenant_settings(client: BackendClient, auth: AuthContext, **fields: Any) -> dict[str, Any]:
    payload = {k: v for k, v in fields.items() if v is not None}
    if not payload:
        return {"error": "no fields to update"}
    return await client.request("PATCH", "/v1/tenant-settings", auth=auth, json=payload)


async def update_business_info(
    client: BackendClient,
    auth: AuthContext,
    business_name: str | None = None,
    business_tax_id: str | None = None,
    business_address: str | None = None,
    business_phone: str | None = None,
    default_currency: str | None = None,
    default_tax_rate: float | None = None,
    appointments_enabled: bool | None = None,
) -> dict[str, Any]:
    payload: dict[str, Any] = {}
    if business_name is not None:
        payload["business_name"] = business_name
    if business_tax_id is not None:
        payload["business_tax_id"] = business_tax_id
    if business_address is not None:
        payload["business_address"] = business_address
    if business_phone is not None:
        payload["business_phone"] = business_phone
    if default_currency is not None:
        payload["default_currency"] = default_currency
    if default_tax_rate is not None:
        payload["default_tax_rate"] = default_tax_rate
    if appointments_enabled is not None:
        payload["appointments_enabled"] = appointments_enabled
    if not payload:
        return {"error": "no fields to update"}
    return await client.request("PATCH", "/v1/tenant-settings", auth=auth, json=payload)
