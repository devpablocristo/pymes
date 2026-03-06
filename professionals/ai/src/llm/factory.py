from src.config import Settings
from src.llm.base import EchoProvider, LLMProvider
from src.llm.gemini import GeminiProvider
from pymes_py_pkg.resilience import CircuitBreaker


def create_provider(config: Settings) -> LLMProvider:
    provider = config.llm_provider.strip().lower()
    if provider == "gemini":
        if not config.gemini_api_key:
            return EchoProvider()
        breaker = CircuitBreaker(
            failure_threshold=max(config.llm_circuit_breaker_failures, 1),
            recovery_timeout_seconds=max(config.llm_circuit_breaker_reset_seconds, 1),
        )
        return GeminiProvider(api_key=config.gemini_api_key, model=config.gemini_model, circuit_breaker=breaker)
    raise ValueError(f"LLM provider desconocido: {config.llm_provider}")
