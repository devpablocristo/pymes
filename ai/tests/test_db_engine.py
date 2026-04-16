from __future__ import annotations

from src.db.engine import _normalize_async_database_url


def test_normalize_async_database_url_removes_sslmode_for_asyncpg() -> None:
    raw = "postgres://user:pass@/pymes?host=/cloudsql/project:region:db&sslmode=disable"

    normalized = _normalize_async_database_url(raw)

    assert normalized.startswith("postgresql+asyncpg://user:pass@/pymes?")
    assert "host=%2Fcloudsql%2Fproject%3Aregion%3Adb" in normalized
    assert "sslmode" not in normalized
