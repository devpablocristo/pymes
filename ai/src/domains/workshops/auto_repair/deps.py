from __future__ import annotations

from fastapi import Request

from src.api.state_deps import get_auth_context, get_llm_provider, get_state_attr
from src.domains.workshops.auto_repair.backend_client import AutoRepairBackendClient


def get_auto_repair_backend_client(request: Request) -> AutoRepairBackendClient:
    return get_state_attr(
        request,
        "auto_repair_backend_client",
        "auto repair backend client not initialized",
    )
