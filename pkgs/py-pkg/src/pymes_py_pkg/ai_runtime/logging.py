from __future__ import annotations

import logging
from contextvars import ContextVar
from typing import Any

import structlog

_request_id: ContextVar[str] = ContextVar("request_id", default="")
_org_id: ContextVar[str] = ContextVar("org_id", default="")
_user_id: ContextVar[str] = ContextVar("user_id", default="")


def configure_logging(level: str = "INFO", json_logs: bool = True) -> None:
    shared_processors: list[Any] = [
        structlog.contextvars.merge_contextvars,
        structlog.stdlib.add_log_level,
        structlog.stdlib.add_logger_name,
        structlog.processors.TimeStamper(fmt="iso", utc=True),
    ]

    logging.basicConfig(
        format="%(message)s",
        level=getattr(logging, level.upper(), logging.INFO),
    )

    renderer: Any
    if json_logs:
        renderer = structlog.processors.JSONRenderer()
    else:
        renderer = structlog.dev.ConsoleRenderer()

    structlog.configure(
        processors=[
            *shared_processors,
            structlog.processors.dict_tracebacks,
            structlog.processors.EventRenamer("message"),
            renderer,
        ],
        logger_factory=structlog.stdlib.LoggerFactory(),
        wrapper_class=structlog.stdlib.BoundLogger,
        cache_logger_on_first_use=True,
    )


def bind_request_context(request_id: str, org_id: str = "", user_id: str = "") -> None:
    _request_id.set(request_id)
    _org_id.set(org_id)
    _user_id.set(user_id)
    structlog.contextvars.clear_contextvars()
    structlog.contextvars.bind_contextvars(request_id=request_id, org_id=org_id, user_id=user_id)


def update_request_context(org_id: str = "", user_id: str = "") -> None:
    if org_id:
        _org_id.set(org_id)
    if user_id:
        _user_id.set(user_id)
    structlog.contextvars.bind_contextvars(
        request_id=_request_id.get(),
        org_id=_org_id.get(),
        user_id=_user_id.get(),
    )


def clear_request_context() -> None:
    _request_id.set("")
    _org_id.set("")
    _user_id.set("")
    structlog.contextvars.clear_contextvars()


def get_request_id() -> str:
    return _request_id.get()


def get_logger(name: str):
    return structlog.get_logger(name)
