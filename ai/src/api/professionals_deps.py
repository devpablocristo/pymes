from __future__ import annotations

from fastapi import Request

from src.backend_client.professionals_client import ProfessionalsBackendClient
from src.api.state_deps import (
    get_auth_context,
    get_llm_provider,
    get_settings_dep,
    get_state_attr,
)


def get_professionals_backend_client(request: Request) -> ProfessionalsBackendClient:
    return get_state_attr(
        request,
        "professionals_backend_client",
        "professionals backend client not initialized",
    )
