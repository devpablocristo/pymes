from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    ai_port: int = 8000
    ai_log_level: str = "info"

    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5434/pymes"

    backend_url: str = "http://backend:8080"
    internal_service_token: str = "local-internal-token"

    llm_provider: str = "gemini"
    gemini_api_key: str = ""
    gemini_model: str = "gemini-2.0-flash"

    jwks_url: str = ""
    jwt_issuer: str = ""
    auth_allow_api_key: bool = True

    ai_internal_rpm: int = 120
    ai_external_rpm: int = 60

    model_config = SettingsConfigDict(env_file=(".env", "../../.env"), extra="ignore")


@lru_cache
def get_settings() -> Settings:
    return Settings()
