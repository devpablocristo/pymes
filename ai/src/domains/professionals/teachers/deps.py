from __future__ import annotations

from fastapi import Request

from src.domains.professionals.teachers.backend_client import TeachersBackendClient

# Re-exportados para Depends() en routers del dominio (mismo criterio que src/api/deps.py).
from src.api.state_deps import get_auth_context, get_llm_provider, get_state_attr  # noqa: F401


def get_teachers_backend_client(request: Request) -> TeachersBackendClient:
    return get_state_attr(
        request,
        "teachers_backend_client",
        "teachers backend client not initialized",
    )
