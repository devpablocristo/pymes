from __future__ import annotations

from typing import TypeVar, cast

from fastapi import HTTPException, Request, status

from src.config import Settings, get_settings
from runtime.contexts import AuthContext
from runtime.types import LLMProvider

T = TypeVar("T")


def get_settings_dep(request: Request) -> Settings:
    settings = getattr(request.app.state, "settings", None)
    if settings is None:
        settings = get_settings()
    return settings


def get_state_attr(request: Request, attr: str, detail: str) -> T:
    value = getattr(request.app.state, attr, None)
    if value is None:
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=detail,
        )
    return cast(T, value)


def get_llm_provider(request: Request) -> LLMProvider:
    return get_state_attr(request, "llm_provider", "llm provider not initialized")


def get_auth_context(request: Request) -> AuthContext:
    auth = getattr(request.state, "auth", None)
    if auth is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="unauthorized",
        )
    return cast(AuthContext, auth)
