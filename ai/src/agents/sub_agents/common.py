from __future__ import annotations

from runtime.services.orchestrator import OrchestratorLimits
from src.config import get_settings


def build_default_limits() -> OrchestratorLimits:
    """Límites del assistant definidos por configuración externa."""
    settings = get_settings()
    return OrchestratorLimits(
        max_tool_calls=max(1, int(settings.assistant_max_tool_calls)),
        tool_timeout_seconds=max(1.0, float(settings.assistant_tool_timeout_seconds)),
        total_timeout_seconds=max(1.0, float(settings.assistant_total_timeout_seconds)),
    )
