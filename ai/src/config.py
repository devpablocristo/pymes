from functools import lru_cache

from pydantic import field_validator, model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

LOCAL_INTERNAL_SERVICE_TOKEN = "local-internal-token"
LOCAL_ENVIRONMENTS = {"", "development", "dev", "local", "test"}


class Settings(BaseSettings):
    ai_port: int = 8000
    ai_log_level: str = "info"
    ai_log_json: bool = True
    ai_environment: str = "development"

    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5434/pymes"

    backend_url: str = "http://cp-backend:8080"
    professionals_backend_url: str = "http://prof-backend:8081"
    workshops_backend_url: str = "http://work-backend:8082"
    internal_service_token: str = ""

    llm_provider: str = "gemini"
    gemini_api_key: str = ""
    gemini_model: str = "gemini-2.5-flash"
    gemini_vertex_project: str = ""
    gemini_vertex_location: str = "global"
    assistant_max_tool_calls: int = 5
    assistant_tool_timeout_seconds: float = 20.0
    assistant_total_timeout_seconds: float = 180.0
    llm_circuit_breaker_failures: int = 3
    llm_circuit_breaker_reset_seconds: int = 30

    jwks_url: str = ""
    jwt_issuer: str = ""
    auth_allow_api_key: bool = True

    ai_internal_rpm: int = 120
    ai_external_rpm: int = 60
    ai_enforce_plan_limits: bool = True
    otel_service_name: str = "pymes-ai"
    otel_exporter_otlp_endpoint: str = ""

    # Nexus Review — gobernanza de acciones
    review_url: str = ""
    review_api_key: str = ""
    review_callback_token: str = ""

    model_config = SettingsConfigDict(env_file=(".env", "../.env"), extra="ignore")

    @property
    def normalized_environment(self) -> str:
        return self.ai_environment.strip().lower() or "development"

    @property
    def is_local_environment(self) -> bool:
        return self.normalized_environment in LOCAL_ENVIRONMENTS

    @property
    def review_enabled(self) -> bool:
        return bool(self.review_url.strip())

    @field_validator("llm_provider")
    @classmethod
    def validate_llm_provider(cls, value: str) -> str:
        provider = value.strip().lower() or "gemini"
        if provider != "gemini":
            raise ValueError("LLM_PROVIDER must be gemini")
        return provider

    @model_validator(mode="after")
    def validate_runtime_settings(self) -> "Settings":
        token = self.internal_service_token.strip()
        if self.is_local_environment:
            self.ai_environment = self.normalized_environment
            self.internal_service_token = token or LOCAL_INTERNAL_SERVICE_TOKEN
        else:
            if not token or token == LOCAL_INTERNAL_SERVICE_TOKEN:
                raise ValueError("INTERNAL_SERVICE_TOKEN must be configured with a non-default value outside local environments")

            self.ai_environment = self.normalized_environment
            self.internal_service_token = token

        if not self.gemini_api_key.strip() and not self.gemini_vertex_project.strip():
            raise ValueError("GEMINI_API_KEY or GEMINI_VERTEX_PROJECT must be configured for Gemini")
        return self


@lru_cache
def get_settings() -> Settings:
    return Settings()
