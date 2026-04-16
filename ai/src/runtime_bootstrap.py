from __future__ import annotations

import asyncio
import time
from typing import Any
from uuid import UUID

import httpx
from google import genai
from jose import jwt

from runtime.contexts import AuthContext
from runtime.providers.gemini import GeminiProvider
from runtime.provider_factory import create_provider as create_runtime_provider

from src.config import Settings


def build_llm_provider(settings: Settings) -> Any:
    provider = settings.llm_provider.strip().lower() or "echo"
    if provider == "gemini" and settings.gemini_vertex_project.strip():
        gemini = GeminiProvider(
            api_key="vertex-ai",
            model=settings.gemini_model.strip() or "gemini-2.0-flash",
        )
        gemini.client = genai.Client(
            vertexai=True,
            project=settings.gemini_vertex_project.strip(),
            location=settings.gemini_vertex_location.strip() or "us-central1",
        )
        return gemini
    return create_runtime_provider(settings)


class ClerkBearerVerifier:
    def __init__(
        self,
        *,
        jwks_url: str,
        jwt_issuer: str,
        backend_url: str,
        internal_token: str,
        jwks_cache_ttl_seconds: float = 300.0,
    ) -> None:
        self._jwks_url = jwks_url.strip()
        self._jwt_issuer = _normalize_issuer(jwt_issuer)
        self._backend_url = backend_url.rstrip("/")
        self._internal_token = internal_token.strip()
        self._jwks_cache_ttl_seconds = max(0.0, jwks_cache_ttl_seconds)
        self._jwks_by_kid: dict[str, dict[str, Any]] = {}
        self._jwks_loaded_at = 0.0
        self._lock = asyncio.Lock()

    async def verify_bearer(self, token: str) -> AuthContext | None:
        raw_token = token.strip()
        if not raw_token or not self._jwks_url:
            return None

        try:
            header = jwt.get_unverified_header(raw_token)
            kid = str(header.get("kid", "")).strip()
            if not kid:
                return None

            jwk = await self._load_jwk(kid)
            if jwk is None:
                return None

            claims = jwt.decode(
                raw_token,
                jwk,
                algorithms=["RS256", "RS384", "RS512"],
                issuer=self._jwt_issuer or None,
                options={
                    "verify_aud": False,
                    "verify_iss": bool(self._jwt_issuer),
                },
            )
        except Exception:
            return None

        actor = _first_string_claim(claims, "sub")
        if not actor:
            return None

        raw_org = _first_string_claim(claims, "tenant_id", "org_id", "o.id")
        if not raw_org:
            raw_org = _clerk_compact_org_id_from_claims(claims)
        org_id = await self._resolve_org_id(raw_org)
        if not org_id:
            return None

        role = _normalize_role(_first_string_claim(claims, "role", "org_role", "o.rol")) or "member"
        scopes = _first_scopes_claim(claims, "scopes", "org_permissions", "o.per")

        return AuthContext(
            tenant_id=org_id,
            actor=actor,
            role=role,
            scopes=scopes,
            mode="bearer",
            authorization=f"Bearer {raw_token}",
        )

    async def _load_jwk(self, kid: str) -> dict[str, Any] | None:
        async with self._lock:
            if self._should_refresh_jwks():
                await self._refresh_jwks()
            key = self._jwks_by_kid.get(kid)
            if key is not None:
                return key
            await self._refresh_jwks()
            return self._jwks_by_kid.get(kid)

    def _should_refresh_jwks(self) -> bool:
        if not self._jwks_by_kid:
            return True
        if self._jwks_cache_ttl_seconds <= 0:
            return True
        return (time.monotonic() - self._jwks_loaded_at) >= self._jwks_cache_ttl_seconds

    async def _refresh_jwks(self) -> None:
        async with httpx.AsyncClient(timeout=10.0) as client:
            response = await client.get(self._jwks_url)
            response.raise_for_status()
            payload = response.json()

        keys = payload.get("keys", [])
        if not isinstance(keys, list):
            self._jwks_by_kid = {}
            self._jwks_loaded_at = time.monotonic()
            return

        parsed: dict[str, dict[str, Any]] = {}
        for item in keys:
            if not isinstance(item, dict):
                continue
            kid = str(item.get("kid", "")).strip()
            if kid:
                parsed[kid] = item

        self._jwks_by_kid = parsed
        self._jwks_loaded_at = time.monotonic()

    async def _resolve_org_id(self, ref: str) -> str:
        normalized = ref.strip()
        if not normalized:
            return ""
        try:
            UUID(normalized)
            return normalized
        except ValueError:
            pass

        if not self._internal_token:
            return ""

        async with httpx.AsyncClient(base_url=self._backend_url, timeout=10.0) as client:
            response = await client.get(
                "/v1/internal/v1/orgs/resolve-ref",
                headers={"X-Internal-Service-Token": self._internal_token},
                params={"ref": normalized},
            )
        if response.status_code != 200:
            return ""
        payload = response.json()
        if not isinstance(payload, dict):
            return ""
        org_id = str(payload.get("org_id", "")).strip()
        try:
            UUID(org_id)
        except ValueError:
            return ""
        return org_id


def _normalize_issuer(raw: str) -> str:
    return raw.strip().rstrip("/")


def _normalize_role(role: str) -> str:
    normalized = role.strip()
    if normalized.startswith("org:"):
        normalized = normalized[4:]
    return normalized.strip()


def _first_string_claim(claims: dict[str, Any], *names: str) -> str:
    for name in names:
        value = _claim_value(claims, name)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return ""


def _first_scopes_claim(claims: dict[str, Any], *names: str) -> list[str]:
    for name in names:
        value = _claim_value(claims, name)
        scopes = _parse_scopes(value)
        if scopes:
            return scopes
    return []


def _claim_value(claims: dict[str, Any], path: str) -> Any:
    normalized = path.strip()
    if not normalized:
        return None
    if normalized in claims:
        return claims[normalized]
    current: Any = claims
    for part in normalized.split("."):
        if not isinstance(current, dict):
            return None
        current = current.get(part.strip())
        if current is None:
            return None
    return current


def _parse_scopes(value: Any) -> list[str]:
    if isinstance(value, str):
        return _split_scopes(value)
    if isinstance(value, list):
        out: list[str] = []
        for item in value:
            if isinstance(item, str):
                out.extend(_split_scopes(item))
        return out
    return []


def _split_scopes(raw: str) -> list[str]:
    normalized = raw.replace(",", " ").strip()
    return [part for part in normalized.split() if part]


def _clerk_compact_org_id_from_claims(claims: dict[str, Any]) -> str:
    raw_org = claims.get("o")
    if not isinstance(raw_org, dict):
        return ""
    value = raw_org.get("id")
    if not isinstance(value, str):
        return ""
    return value.strip()
