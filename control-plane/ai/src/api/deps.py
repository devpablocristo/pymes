from __future__ import annotations

from fastapi import Depends, HTTPException, Request, status
from sqlalchemy.ext.asyncio import AsyncSession

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.config import Settings
from src.db.engine import get_db_session
from src.db.repository import AIRepository
from src.llm.base import LLMProvider


def get_settings_dep(request: Request) -> Settings:
    settings = getattr(request.app.state, "settings", None)
    if settings is None:
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="settings not initialized")
    return settings


def get_backend_client(request: Request) -> BackendClient:
    client = getattr(request.app.state, "backend_client", None)
    if client is None:
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="backend client not initialized")
    return client


def get_llm_provider(request: Request) -> LLMProvider:
    provider = getattr(request.app.state, "llm_provider", None)
    if provider is None:
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="llm provider not initialized")
    return provider


async def get_repository(session: AsyncSession = Depends(get_db_session)) -> AIRepository:
    return AIRepository(session)


def get_auth_context(request: Request) -> AuthContext:
    auth = getattr(request.state, "auth", None)
    if auth is None:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="unauthorized")
    return auth
