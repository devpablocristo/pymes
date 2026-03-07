from __future__ import annotations

import asyncio
import time
from typing import Any

import httpx

from src.backend_client.auth import AuthContext
from pymes_py_pkg.ai_runtime import get_logger, get_request_id

logger = get_logger(__name__)


class BackendClient:
    def __init__(self, base_url: str, internal_token: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.internal_token = internal_token
        self._client = httpx.AsyncClient(base_url=self.base_url, timeout=10.0)

    async def close(self) -> None:
        await self._client.aclose()

    def _headers(self, auth: AuthContext | None, include_internal: bool = False) -> dict[str, str]:
        headers: dict[str, str] = {}
        request_id = get_request_id()
        if request_id:
            headers["X-Request-ID"] = request_id
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
        headers = self._headers(auth, include_internal=include_internal)
        last_error: Exception | None = None
        for attempt in range(3):
            started_at = time.perf_counter()
            try:
                response = await self._client.request(method, path, headers=headers, **kwargs)
                if response.status_code >= 500 and attempt < 2:
                    logger.warning(
                        "backend_retryable_status",
                        method=method,
                        path=path,
                        status_code=response.status_code,
                        attempt=attempt + 1,
                    )
                    await asyncio.sleep(0.2 * (attempt + 1))
                    continue
                logger.info(
                    "backend_request",
                    method=method,
                    path=path,
                    status_code=response.status_code,
                    duration_ms=round((time.perf_counter() - started_at) * 1000, 2),
                )
                response.raise_for_status()
                if response.headers.get("content-type", "").startswith("application/json"):
                    return response.json()
                return {"raw": response.text}
            except httpx.HTTPStatusError:
                logger.warning(
                    "backend_http_error",
                    method=method,
                    path=path,
                    attempt=attempt + 1,
                    duration_ms=round((time.perf_counter() - started_at) * 1000, 2),
                )
                raise
            except (httpx.TimeoutException, httpx.NetworkError, httpx.RemoteProtocolError) as exc:
                last_error = exc
                logger.warning(
                    "backend_transport_error",
                    method=method,
                    path=path,
                    error=str(exc),
                    attempt=attempt + 1,
                    duration_ms=round((time.perf_counter() - started_at) * 1000, 2),
                )
                if attempt == 2:
                    raise
                await asyncio.sleep(0.2 * (attempt + 1))
        if last_error is not None:
            raise last_error
        raise RuntimeError("backend request failed without error")
