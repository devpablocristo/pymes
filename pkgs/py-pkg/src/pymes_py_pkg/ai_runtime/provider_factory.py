from __future__ import annotations

from typing import Any

from pymes_py_pkg.ai_runtime.gemini import GeminiProvider
from pymes_py_pkg.ai_runtime.types import EchoProvider, LLMProvider
from pymes_py_pkg.resilience import CircuitBreaker


def create_provider(config: Any) -> LLMProvider:
    provider = str(getattr(config, "llm_provider", "")).strip().lower()
    if provider == "gemini":
        if not getattr(config, "gemini_api_key", ""):
            return EchoProvider()
        breaker = CircuitBreaker(
            failure_threshold=max(int(getattr(config, "llm_circuit_breaker_failures", 3)), 1),
            recovery_timeout_seconds=max(int(getattr(config, "llm_circuit_breaker_reset_seconds", 30)), 1),
        )
        return GeminiProvider(
            api_key=str(getattr(config, "gemini_api_key", "")),
            model=str(getattr(config, "gemini_model", "gemini-2.0-flash")),
            circuit_breaker=breaker,
        )
    raise ValueError(f"LLM provider desconocido: {getattr(config, 'llm_provider', '')}")
