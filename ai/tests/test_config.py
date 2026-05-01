from __future__ import annotations

import pytest
from pydantic import ValidationError

from src.config import LOCAL_INTERNAL_SERVICE_TOKEN, Settings


def test_settings_local_environment_defaults_internal_token() -> None:
    settings = Settings(
        _env_file=None,
        ai_environment="development",
        internal_service_token="",
        gemini_api_key="test-key",
    )

    assert settings.internal_service_token == LOCAL_INTERNAL_SERVICE_TOKEN


def test_settings_reject_non_gemini_provider() -> None:
    with pytest.raises(ValidationError):
        Settings(
            _env_file=None,
            ai_environment="development",
            internal_service_token="",
            llm_provider="ollama",
            gemini_api_key="test-key",
        )


def test_settings_requires_gemini_credentials() -> None:
    with pytest.raises(ValidationError):
        Settings(
            _env_file=None,
            ai_environment="development",
            internal_service_token="",
            gemini_api_key="",
            gemini_vertex_project="",
        )


def test_settings_reject_missing_internal_token_outside_local() -> None:
    with pytest.raises(ValidationError):
        Settings(
            _env_file=None,
            ai_environment="production",
            internal_service_token="",
            gemini_api_key="test-key",
        )


def test_settings_reject_default_internal_token_outside_local() -> None:
    with pytest.raises(ValidationError):
        Settings(
            _env_file=None,
            ai_environment="production",
            internal_service_token=LOCAL_INTERNAL_SERVICE_TOKEN,
            gemini_api_key="test-key",
        )
