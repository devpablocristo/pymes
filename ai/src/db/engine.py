from __future__ import annotations

from collections.abc import AsyncIterator
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from src.config import get_settings

settings = get_settings()


def _normalize_async_database_url(raw: str) -> str:
    url = (raw or "").strip()
    if url.startswith("postgres://"):
        url = "postgresql+asyncpg://" + url[len("postgres://"):]
    elif url.startswith("postgresql://"):
        url = "postgresql+asyncpg://" + url[len("postgresql://"):]

    split = urlsplit(url)
    if not split.query:
        return url

    filtered = [(key, value) for key, value in parse_qsl(split.query, keep_blank_values=True) if key != "sslmode"]
    return urlunsplit((split.scheme, split.netloc, split.path, urlencode(filtered), split.fragment))


engine = create_async_engine(_normalize_async_database_url(settings.database_url), future=True)
SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)


async def get_db_session() -> AsyncIterator[AsyncSession]:
    async with SessionLocal() as session:
        yield session


from contextlib import asynccontextmanager  # noqa: E402

@asynccontextmanager
async def get_session() -> AsyncIterator[AsyncSession]:
    """Context manager para obtener una sesión de DB fuera de FastAPI Depends."""
    async with SessionLocal() as session:
        yield session


async def ping_database() -> None:
    async with engine.connect() as conn:
        await conn.execute(text("SELECT 1"))
