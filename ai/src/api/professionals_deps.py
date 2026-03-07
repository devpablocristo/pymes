from __future__ import annotations

from fastapi import HTTPException, Request, status

from src.backend_client.professionals_client import ProfessionalsBackendClient
from src.config import Settings, get_settings
from pymes_control_plane_shared.ai_runtime import LLMProvider
from pymes_control_plane_shared.ai_runtime import AuthContext


def get_settings_dep(request: Request) -> Settings:
    settings = getattr(request.app.state, "settings", None)
    if settings is None:
        settings = get_settings()
    return settings


def get_professionals_backend_client(request: Request) -> ProfessionalsBackendClient:
    client = getattr(request.app.state, "professionals_backend_client", None)
    if client is None:
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="professionals backend client not initialized")
    return client


def get_llm_provider(request: Request) -> LLMProvider:
    provider = getattr(request.app.state, "llm_provider", None)
    if provider is None:
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="llm provider not initialized")
    return provider


def get_auth_context(request: Request) -> AuthContext:
    auth = getattr(request.state, "auth", None)
    if auth is None:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="unauthorized")
    return auth
