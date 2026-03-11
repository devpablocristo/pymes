from __future__ import annotations

from fastapi import Request

from src.api.state_deps import get_auth_context, get_llm_provider, get_state_attr
from src.domains.professionals.teachers.backend_client import TeachersBackendClient


def get_teachers_backend_client(request: Request) -> TeachersBackendClient:
    return get_state_attr(
        request,
        "teachers_backend_client",
        "teachers backend client not initialized",
    )
