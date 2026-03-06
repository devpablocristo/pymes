from __future__ import annotations

import time
from collections import defaultdict, deque

from pymes_py_pkg.errors import error_payload
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import JSONResponse

from src.config import Settings


class RateLimitMiddleware(BaseHTTPMiddleware):
    def __init__(self, app, settings: Settings) -> None:  # type: ignore[no-untyped-def]
        super().__init__(app)
        self.settings = settings
        self._hits: dict[str, deque[float]] = defaultdict(deque)

    async def dispatch(self, request: Request, call_next):  # type: ignore[no-untyped-def]
        path = request.url.path
        if not (path.startswith("/v1/chat") or path.startswith("/v1/public/")):
            return await call_next(request)
        request_id = getattr(request.state, "request_id", "")

        now = time.time()
        if path.startswith("/v1/public/"):
            key = f"public:{request.client.host if request.client else 'unknown'}"
            limit = self.settings.ai_external_rpm
        else:
            auth = getattr(request.state, "auth", None)
            org_id = getattr(auth, "org_id", "unknown")
            key = f"internal:{org_id}"
            limit = self.settings.ai_internal_rpm

        q = self._hits[key]
        while q and now - q[0] > 60:
            q.popleft()
        if len(q) >= max(limit, 1):
            return JSONResponse(
                status_code=429,
                content=error_payload("rate_limit_exceeded", "rate limit exceeded", request_id),
            )

        q.append(now)
        return await call_next(request)
