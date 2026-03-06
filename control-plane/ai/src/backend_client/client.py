from __future__ import annotations

from typing import Any

import httpx

from src.backend_client.auth import AuthContext


class BackendClient:
    def __init__(self, base_url: str, internal_token: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.internal_token = internal_token
        self._client = httpx.AsyncClient(base_url=self.base_url, timeout=10.0)

    async def close(self) -> None:
        await self._client.aclose()

    def _headers(self, auth: AuthContext | None, include_internal: bool = False) -> dict[str, str]:
        headers: dict[str, str] = {}
        if include_internal and self.internal_token:
            headers["X-Internal-Service-Token"] = self.internal_token

        if auth is None:
            return headers

        if auth.authorization:
            headers["Authorization"] = auth.authorization
        if auth.api_key:
            headers["X-API-KEY"] = auth.api_key
            headers["X-Actor"] = auth.api_actor or auth.actor
            headers["X-Role"] = auth.api_role or auth.role
            headers["X-Scopes"] = auth.api_scopes or ",".join(auth.scopes)
        return headers

    async def request(
        self,
        method: str,
        path: str,
        auth: AuthContext | None = None,
        include_internal: bool = False,
        **kwargs: Any,
    ) -> dict[str, Any]:
        response = await self._client.request(
            method,
            path,
            headers=self._headers(auth, include_internal=include_internal),
            **kwargs,
        )
        response.raise_for_status()
        if response.headers.get("content-type", "").startswith("application/json"):
            return response.json()
        return {"raw": response.text}
