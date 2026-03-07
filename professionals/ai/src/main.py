from __future__ import annotations

from contextlib import asynccontextmanager
from uuid import uuid4

from fastapi import FastAPI, HTTPException, Request
from pymes_py_pkg.ai_runtime import (
    apply_permissive_cors,
    install_request_context_middleware,
    register_common_exception_handlers,
)

from src.api.public_router import router as public_router
from src.api.router import router as chat_router
from src.backend_client import BackendClient
from src.config import get_settings
from pymes_py_pkg.ai_runtime import create_provider
from pymes_py_pkg.ai_runtime import AuthMiddleware
from pymes_py_pkg.ai_runtime import RateLimitMiddleware
from pymes_py_pkg.ai_runtime import bind_request_context, clear_request_context, configure_logging, get_logger

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
    app.state.llm_provider = create_provider(settings)
    logger.info(
        "professionals_ai_started",
        environment=settings.ai_environment,
        backend_url=settings.backend_url,
    )
    yield
    logger.info("professionals_ai_stopping")
    await app.state.backend_client.close()


app = FastAPI(title="pymes-professionals-ai", version="0.1.0", lifespan=lifespan)

apply_permissive_cors(app)
app.add_middleware(RateLimitMiddleware, settings=settings)
app.add_middleware(AuthMiddleware, settings=settings)
install_request_context_middleware(app, bind_request_context, clear_request_context)


app.include_router(chat_router)
app.include_router(public_router)
register_common_exception_handlers(app, logger)


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/readyz")
async def readyz() -> dict[str, str]:
    return {"status": "ready"}
