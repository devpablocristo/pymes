from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient


async def get_report(client: BackendClient, auth: AuthContext, report_path: str, params: dict | None = None) -> dict:
    return await client.request("GET", f"/v1/reports/{report_path}", auth=auth, params=params or {})
