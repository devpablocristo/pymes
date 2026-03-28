from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI
import httpx
from httpserver.fastapi_bootstrap import (
    apply_permissive_cors,
    install_request_context_middleware,
    register_common_exception_handlers,
)

from src.api.public_router import router as public_router
from src.api.public_sales_router import router as public_sales_router
from src.api.internal_router import router as internal_router
from src.api.commercial_router import router as commercial_router
from src.insights.router import router as insights_router
from src.api.pymes_assistant_router import router as pymes_assistant_router
from src.api.router import router as chat_router
from src.backend_client.client import BackendClient
from src.config import get_settings
from src.db.engine import ping_database
from src.domains.professionals.teachers.backend_client import TeachersBackendClient
from src.domains.professionals.teachers.internal_router import router as teachers_chat_router
from src.domains.professionals.teachers.public_router import router as teachers_public_router
from src.domains.workshops.auto_repair.backend_client import AutoRepairBackendClient
from src.domains.workshops.auto_repair.internal_router import router as auto_repair_chat_router
from src.domains.workshops.auto_repair.public_router import router as auto_repair_public_router
from runtime.provider_factory import create_provider
from runtime.auth import AuthMiddleware, AuthSettings, AuthContext
from runtime.rate_limit import RateLimitMiddleware, RateLimitSettings
from runtime.logging import bind_request_context, clear_request_context, configure_logging, get_logger
from src.api.review_callback import router as review_callback_router
from src.observability.otel import configure_opentelemetry
from src.review_client.client import ReviewClient

settings = get_settings()
configure_logging(settings.ai_log_level, json_logs=settings.ai_log_json)
logger = get_logger(__name__)


class LocalAPIKeyVerifier:
    """Valida API keys locales contra pymes-core y conserva la key para downstream auth."""

    def __init__(self, *, backend_url: str, internal_token: str) -> None:
        self._backend_url = backend_url.rstrip("/")
        self._internal_token = internal_token

    async def _resolve_api_key(self, key: str) -> dict[str, object] | None:
        async with httpx.AsyncClient(base_url=self._backend_url, timeout=10.0) as client:
            response = await client.post(
                "/v1/internal/v1/api-keys/resolve",
                headers={"X-Internal-Service-Token": self._internal_token},
                json={"api_key": key},
            )
        if response.status_code == 404:
            return None
        response.raise_for_status()
        payload = response.json()
        if not isinstance(payload, dict):
            return None
        return payload

    async def verify_api_key(self, key: str) -> AuthContext | None:
        if not key.startswith("psk_"):
            return None
        payload = await self._resolve_api_key(key)
        if payload is None:
            return None
        tenant = str(payload.get("org_id", "")).strip()
        key_id = str(payload.get("id", "")).strip()
        raw_scopes = payload.get("scopes", [])
        scopes = [str(item).strip() for item in raw_scopes if str(item).strip()] if isinstance(raw_scopes, list) else []
        scopes_csv = ",".join(scopes)
        if not tenant:
            return None
        return AuthContext(
            tenant_id=tenant,
            actor=f"api_key:{key_id or tenant}",
            role="service",
            scopes=scopes,
            mode="api_key",
            api_key=key,
            api_actor=f"api_key:{key_id or tenant}",
            api_role="service",
            api_scopes=scopes_csv or None,
        )


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = get_settings()
    app.state.settings = settings
    app.state.backend_client = BackendClient(
        base_url=settings.backend_url,
        internal_token=settings.internal_service_token,
    )
    app.state.teachers_backend_client = TeachersBackendClient(
        base_url=settings.professionals_backend_url,
        internal_token=settings.internal_service_token,
    )
    app.state.auto_repair_backend_client = AutoRepairBackendClient(
        base_url=settings.workshops_backend_url,
        internal_token=settings.internal_service_token,
    )
    app.state.llm_provider = create_provider(settings)
    # Nexus Review — gobernanza de acciones (opcional)
    if settings.review_enabled:
        app.state.review_client = ReviewClient(
            base_url=settings.review_url,
            api_key=settings.review_api_key,
        )
        logger.info("review_client_enabled", review_url=settings.review_url)
    else:
        app.state.review_client = None
    if not getattr(app.state, "otel_configured", False):
        configure_opentelemetry(app, settings, app.state.backend_client)
        app.state.otel_configured = True
    logger.info(
        "ai_service_started",
        environment=settings.ai_environment,
        backend_url=settings.backend_url,
        professionals_backend_url=settings.professionals_backend_url,
        workshops_backend_url=settings.workshops_backend_url,
    )
    yield
    logger.info("ai_service_stopping")
    await app.state.backend_client.close()
    await app.state.teachers_backend_client.close()
    await app.state.auto_repair_backend_client.close()
    if app.state.review_client is not None:
        await app.state.review_client.close()


app = FastAPI(title="pymes-ai", version="0.1.0", lifespan=lifespan)

install_request_context_middleware(app, bind_request_context, clear_request_context)
app.add_middleware(
    AuthMiddleware,
    settings=AuthSettings(allow_api_key=settings.auth_allow_api_key),
    protected_prefixes=("/v1/chat", "/v1/insights"),
    api_key_verifier=(
        LocalAPIKeyVerifier(
            backend_url=settings.backend_url,
            internal_token=settings.internal_service_token,
        )
        if settings.is_local_environment
        else None
    ),
)
app.add_middleware(
    RateLimitMiddleware,
    settings=RateLimitSettings(
        external_rpm=settings.ai_external_rpm,
        internal_rpm=settings.ai_internal_rpm,
    ),
)
apply_permissive_cors(app)


app.include_router(chat_router)
app.include_router(commercial_router)
app.include_router(insights_router)
app.include_router(pymes_assistant_router)
app.include_router(public_router)
app.include_router(public_sales_router)
app.include_router(internal_router)
app.include_router(teachers_chat_router)
app.include_router(teachers_public_router)
app.include_router(auto_repair_chat_router)
app.include_router(auto_repair_public_router)
app.include_router(review_callback_router)
register_common_exception_handlers(app, logger)


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/readyz")
async def readyz() -> dict[str, str]:
    await ping_database()
    return {"status": "ready"}
