from __future__ import annotations

from contextlib import asynccontextmanager
from uuid import uuid4

from fastapi import FastAPI, HTTPException, Request
from fastapi.exceptions import RequestValidationError
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from pymes_py_pkg.errors import AppError, error_payload

from src.api.public_router import router as public_router
from src.api.router import router as chat_router
from src.backend_client import BackendClient
from src.config import get_settings
from src.llm.factory import create_provider
from src.middleware.auth import AuthMiddleware
from src.middleware.rate_limit import RateLimitMiddleware
from src.observability.logging import bind_request_context, clear_request_context, configure_logging, get_logger

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

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)
app.add_middleware(RateLimitMiddleware, settings=settings)
app.add_middleware(AuthMiddleware, settings=settings)


@app.middleware("http")
async def request_context_middleware(request: Request, call_next):
    request_id = request.headers.get("X-Request-ID", f"req_{uuid4().hex[:12]}")
    request.state.request_id = request_id
    bind_request_context(request_id)
    try:
        response = await call_next(request)
    finally:
        clear_request_context()
    response.headers["X-Request-ID"] = request_id
    return response


app.include_router(chat_router)
app.include_router(public_router)


@app.exception_handler(AppError)
async def handle_app_error(request: Request, exc: AppError) -> JSONResponse:
    request_id = getattr(request.state, "request_id", "")
    return JSONResponse(
        status_code=exc.status_code,
        content=error_payload(exc.code, exc.message, request_id, exc.details),
    )


@app.exception_handler(HTTPException)
async def handle_http_error(request: Request, exc: HTTPException) -> JSONResponse:
    request_id = getattr(request.state, "request_id", "")
    return JSONResponse(
        status_code=exc.status_code,
        content=error_payload("http_error", str(exc.detail), request_id),
    )


@app.exception_handler(RequestValidationError)
async def handle_validation_error(request: Request, exc: RequestValidationError) -> JSONResponse:
    request_id = getattr(request.state, "request_id", "")
    return JSONResponse(
        status_code=422,
        content=error_payload("validation_error", "request validation failed", request_id, {"errors": exc.errors()}),
    )


@app.exception_handler(Exception)
async def handle_unexpected_error(request: Request, exc: Exception) -> JSONResponse:
    request_id = getattr(request.state, "request_id", "")
    logger.exception("unhandled_exception", error=str(exc), path=request.url.path)
    return JSONResponse(
        status_code=500,
        content=error_payload("internal_error", "internal server error", request_id),
    )


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/readyz")
async def readyz() -> dict[str, str]:
    return {"status": "ready"}
