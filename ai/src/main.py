from __future__ import annotations

from contextlib import asynccontextmanager
from uuid import uuid4

from fastapi import FastAPI, HTTPException, Request
from pymes_control_plane_shared.ai_runtime import (
    apply_permissive_cors,
    install_request_context_middleware,
    register_common_exception_handlers,
)

from src.api.public_router import router as public_router
from src.api.public_sales_router import router as public_sales_router
from src.api.internal_router import router as internal_router
from src.api.commercial_router import router as commercial_router
from src.api.router import router as chat_router
from src.backend_client.client import BackendClient
from src.config import get_settings
from src.db.engine import ping_database
from src.domains.professionals.teachers.backend_client import TeachersBackendClient
from src.domains.professionals.teachers.internal_router import router as teachers_chat_router
from src.domains.professionals.teachers.public_router import router as teachers_public_router
from pymes_control_plane_shared.ai_runtime import create_provider
from pymes_control_plane_shared.ai_runtime import AuthMiddleware
from pymes_control_plane_shared.ai_runtime import RateLimitMiddleware
from pymes_control_plane_shared.ai_runtime import bind_request_context, clear_request_context, configure_logging, get_logger
from src.observability.otel import configure_opentelemetry

settings = get_settings()
configure_logging(settings.ai_log_level, json_logs=settings.ai_log_json)
logger = get_logger(__name__)


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
    app.state.llm_provider = create_provider(settings)
    if not getattr(app.state, "otel_configured", False):
        configure_opentelemetry(app, settings, app.state.backend_client)
        app.state.otel_configured = True
    logger.info(
        "ai_service_started",
        environment=settings.ai_environment,
        backend_url=settings.backend_url,
        professionals_backend_url=settings.professionals_backend_url,
    )
    yield
    logger.info("ai_service_stopping")
    await app.state.backend_client.close()
    await app.state.teachers_backend_client.close()


app = FastAPI(title="pymes-ai", version="0.1.0", lifespan=lifespan)

apply_permissive_cors(app)
app.add_middleware(RateLimitMiddleware, settings=settings)
app.add_middleware(AuthMiddleware, settings=settings)
install_request_context_middleware(app, bind_request_context, clear_request_context)


app.include_router(chat_router)
app.include_router(commercial_router)
app.include_router(public_router)
app.include_router(public_sales_router)
app.include_router(internal_router)
app.include_router(teachers_chat_router)
app.include_router(teachers_public_router)
register_common_exception_handlers(app, logger)


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/readyz")
async def readyz() -> dict[str, str]:
    await ping_database()
    return {"status": "ready"}
