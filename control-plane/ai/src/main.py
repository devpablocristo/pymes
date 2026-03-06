from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from src.api.public_router import router as public_router
from src.api.router import router as chat_router
from src.backend_client.client import BackendClient
from src.config import get_settings
from src.llm.factory import create_provider
from src.middleware.auth import AuthMiddleware
from src.middleware.rate_limit import RateLimitMiddleware


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = get_settings()
    app.state.settings = settings
    app.state.backend_client = BackendClient(
        base_url=settings.backend_url,
        internal_token=settings.internal_service_token,
    )
    app.state.llm_provider = create_provider(settings)
    yield
    await app.state.backend_client.close()


settings = get_settings()
app = FastAPI(title="pymes-ai", version="0.1.0", lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)
app.add_middleware(RateLimitMiddleware, settings=settings)
app.add_middleware(AuthMiddleware, settings=settings)

app.include_router(chat_router)
app.include_router(public_router)


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}
