# Re-export desde core_ai — la implementacion real vive en core/ai/python.
# Este modulo existe para mantener backward compatibility con imports existentes.
from core_ai.auth import AuthMiddleware
from core_ai.contexts import AuthContext
from core_ai.errors import AppError, error_payload
from core_ai.fastapi import apply_permissive_cors, install_request_context_middleware, register_common_exception_handlers
from core_ai.providers.gemini import GeminiProvider
from core_ai.logging import bind_request_context, clear_request_context, configure_logging, get_logger, get_request_id, update_request_context
from core_ai.orchestrator import OrchestratorLimits, orchestrate
from core_ai.provider_factory import create_provider
from core_ai.rate_limit import RateLimitMiddleware
from core_ai.resilience import CircuitBreaker, CircuitBreakerOpenError
from core_ai.types import ChatChunk, EchoProvider, LLMProvider, Message, ToolDeclaration

__all__ = [
    "AppError",
    "AuthContext",
    "AuthMiddleware",
    "ChatChunk",
    "CircuitBreaker",
    "CircuitBreakerOpenError",
    "EchoProvider",
    "GeminiProvider",
    "LLMProvider",
    "Message",
    "OrchestratorLimits",
    "RateLimitMiddleware",
    "ToolDeclaration",
    "apply_permissive_cors",
    "bind_request_context",
    "clear_request_context",
    "configure_logging",
    "create_provider",
    "error_payload",
    "get_logger",
    "get_request_id",
    "install_request_context_middleware",
    "orchestrate",
    "register_common_exception_handlers",
    "update_request_context",
]
