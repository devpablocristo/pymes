from __future__ import annotations

import json
import time
from dataclasses import dataclass
from typing import Any

import httpx
from jose import jwt
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import JSONResponse

from src.backend_client.auth import AuthContext
from src.config import Settings


@dataclass
class JWKSCache:
    keys: dict[str, Any]
    expires_at: float


class AuthMiddleware(BaseHTTPMiddleware):
    def __init__(self, app, settings: Settings) -> None:  # type: ignore[no-untyped-def]
        super().__init__(app)
        self.settings = settings
        self._jwks_cache = JWKSCache(keys={}, expires_at=0)

    async def dispatch(self, request: Request, call_next):  # type: ignore[no-untyped-def]
        path = request.url.path
        if path.startswith("/healthz") or path.startswith("/v1/public/"):
            return await call_next(request)

        if not path.startswith("/v1/chat"):
            return await call_next(request)

        authz = request.headers.get("Authorization", "")
        api_key = request.headers.get("X-API-KEY", "")

        if authz.lower().startswith("bearer "):
            token = authz[7:].strip()
            payload = await self._decode_jwt(token)
            if payload is None:
                return JSONResponse(status_code=401, content={"error": "invalid jwt"})

            org_id = str(payload.get("org_id", "")).strip()
            actor = str(payload.get("sub", "")).strip()
            role = str(payload.get("org_role", "member")).strip() or "member"
            scopes = self._parse_scopes(payload)

            if not org_id or not actor:
                return JSONResponse(status_code=401, content={"error": "missing org_id/sub"})

            request.state.auth = AuthContext(
                org_id=org_id,
                actor=actor,
                role=role,
                scopes=scopes,
                mode="internal",
                authorization=authz,
            )
            return await call_next(request)

        if api_key and self.settings.auth_allow_api_key:
            org_id = request.headers.get("X-Org-ID", "").strip()
            actor = request.headers.get("X-Actor", "service").strip() or "service"
            role = request.headers.get("X-Role", "service").strip() or "service"
            scopes_raw = request.headers.get("X-Scopes", "")
            scopes = [s.strip() for s in scopes_raw.split(",") if s.strip()]
            if not org_id:
                return JSONResponse(status_code=401, content={"error": "X-Org-ID required for API key mode"})
            request.state.auth = AuthContext(
                org_id=org_id,
                actor=actor,
                role=role,
                scopes=scopes,
                mode="internal",
                api_key=api_key,
                api_actor=actor,
                api_role=role,
                api_scopes=scopes_raw,
            )
            return await call_next(request)

        return JSONResponse(status_code=401, content={"error": "unauthorized"})

    async def _decode_jwt(self, token: str) -> dict[str, Any] | None:
        if not token:
            return None
        if not self.settings.jwks_url:
            # Local dev fallback: no signature validation, decode claims only.
            try:
                return jwt.get_unverified_claims(token)
            except Exception:
                return None

        try:
            header = jwt.get_unverified_header(token)
            kid = header.get("kid")
            if not kid:
                return None
            keys = await self._get_jwks()
            jwk = keys.get(kid)
            if not jwk:
                return None
            return jwt.decode(
                token,
                jwk,
                algorithms=[header.get("alg", "RS256")],
                issuer=self.settings.jwt_issuer or None,
                options={"verify_aud": False},
            )
        except Exception:
            return None

    async def _get_jwks(self) -> dict[str, Any]:
        now = time.time()
        if self._jwks_cache.keys and now < self._jwks_cache.expires_at:
            return self._jwks_cache.keys

        async with httpx.AsyncClient(timeout=5.0) as client:
            response = await client.get(self.settings.jwks_url)
            response.raise_for_status()
            payload = response.json()

        keyed = {k.get("kid"): k for k in payload.get("keys", []) if k.get("kid")}
        self._jwks_cache = JWKSCache(keys=keyed, expires_at=now + 300)
        return keyed

    def _parse_scopes(self, payload: dict[str, Any]) -> list[str]:
        raw = payload.get("org_permissions") or payload.get("scopes") or payload.get("scope")
        if raw is None:
            return []
        if isinstance(raw, list):
            return [str(v).strip() for v in raw if str(v).strip()]
        return [s.strip() for s in str(raw).split(",") if s.strip()]
