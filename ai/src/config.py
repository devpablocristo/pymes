from functools import lru_cache

from pydantic import model_validator
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
    gemini_model: str = "gemini-2.0-flash"
    llm_circuit_breaker_failures: int = 3
    llm_circuit_breaker_reset_seconds: int = 30

    jwks_url: str = ""
    jwt_issuer: str = ""
    auth_allow_api_key: bool = True

    ai_internal_rpm: int = 120
    ai_external_rpm: int = 60
    otel_service_name: str = "pymes-ai"
    otel_exporter_otlp_endpoint: str = ""

    model_config = SettingsConfigDict(env_file=(".env", "../.env"), extra="ignore")

    @property
    def normalized_environment(self) -> str:
        return self.ai_environment.strip().lower() or "development"

    @property
    def is_local_environment(self) -> bool:
        return self.normalized_environment in LOCAL_ENVIRONMENTS

    @model_validator(mode="after")
    def validate_internal_service_token(self) -> "Settings":
        token = self.internal_service_token.strip()
        if self.is_local_environment:
            self.ai_environment = self.normalized_environment
            self.internal_service_token = token or LOCAL_INTERNAL_SERVICE_TOKEN
            return self

        if not token or token == LOCAL_INTERNAL_SERVICE_TOKEN:
            raise ValueError("INTERNAL_SERVICE_TOKEN must be configured with a non-default value outside local environments")

        self.ai_environment = self.normalized_environment
        self.internal_service_token = token
        return self


@lru_cache
def get_settings() -> Settings:
    return Settings()
