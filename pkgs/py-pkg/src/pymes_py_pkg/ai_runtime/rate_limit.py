from __future__ import annotations

import time
from collections import defaultdict, deque
from typing import Any

from pymes_py_pkg.errors import error_payload
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import JSONResponse


class RateLimitMiddleware(BaseHTTPMiddleware):
    def __init__(
        self,
        app,
        settings: Any,
        internal_prefixes: tuple[str, ...] = ("/v1/chat",),
        public_prefixes: tuple[str, ...] = ("/v1/public/",),
    ) -> None:  # type: ignore[no-untyped-def]
        super().__init__(app)
        self.settings = settings
        self.internal_prefixes = internal_prefixes
        self.public_prefixes = public_prefixes
        self._hits: dict[str, deque[float]] = defaultdict(deque)

    async def dispatch(self, request: Request, call_next):  # type: ignore[no-untyped-def]
        path = request.url.path
        is_public = any(path.startswith(prefix) for prefix in self.public_prefixes)
        is_internal = any(path.startswith(prefix) for prefix in self.internal_prefixes)
        if not (is_public or is_internal):
            return await call_next(request)
        request_id = getattr(request.state, "request_id", "")

        now = time.time()
        if is_public:
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
