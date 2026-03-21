"""Herramientas para solicitudes internas de compra (pymes-core procurement_requests)."""

from __future__ import annotations

from typing import Any

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def list_procurement_requests(
    client: BackendClient,
    auth: AuthContext,
    *,
    limit: int = 20,
    archived: bool = False,
) -> dict[str, Any]:
    params: dict[str, Any] = {"limit": limit}
    if archived:
        params["archived"] = "true"
    return await client.request("GET", "/v1/procurement-requests", auth=auth, params=params)


async def create_procurement_request(
    client: BackendClient,
    auth: AuthContext,
    *,
    title: str,
    description: str = "",
    category: str = "",
    estimated_total: float = 0,
    currency: str = "ARS",
    lines: list[dict[str, Any]] | None = None,
) -> dict[str, Any]:
    body: dict[str, Any] = {
        "title": title,
        "description": description,
        "category": category,
        "estimated_total": estimated_total,
        "currency": currency,
        "lines": lines or [],
    }
    return await client.request("POST", "/v1/procurement-requests", auth=auth, json=body)


async def get_procurement_request(client: BackendClient, auth: AuthContext, *, request_id: str) -> dict[str, Any]:
    rid = str(request_id).strip()
    return await client.request("GET", f"/v1/procurement-requests/{rid}", auth=auth)


async def submit_procurement_request(client: BackendClient, auth: AuthContext, *, request_id: str) -> dict[str, Any]:
    rid = str(request_id).strip()
    return await client.request("POST", f"/v1/procurement-requests/{rid}/submit", auth=auth, json={})
