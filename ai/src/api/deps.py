from __future__ import annotations

from fastapi import Depends, HTTPException, Request, status
from sqlalchemy.ext.asyncio import AsyncSession

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.db.engine import get_db_session
from src.db.repository import AIRepository
from src.api.state_deps import (
    get_auth_context,
    get_llm_provider,
    get_settings_dep,
    get_state_attr,
)


def get_backend_client(request: Request) -> BackendClient:
    return get_state_attr(request, "backend_client", "backend client not initialized")


async def get_repository(session: AsyncSession = Depends(get_db_session)) -> AIRepository:
    return AIRepository(session)
