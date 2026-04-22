from __future__ import annotations

import importlib
import os
import sys
import types as _t


def _ensure_runtime_package_stub() -> None:
    src = os.path.join(os.path.dirname(__file__), "..", "..", "..", "core", "ai", "python", "src")
    runtime_src = os.path.join(src, "runtime")
    has_local_runtime = os.path.exists(os.path.join(runtime_src, "contexts.py"))

    if has_local_runtime and src not in sys.path:
        sys.path.insert(0, src)

    if has_local_runtime and "runtime" not in sys.modules:
        runtime_pkg = _t.ModuleType("runtime")
        runtime_pkg.__path__ = [runtime_src]
        runtime_pkg.__package__ = "runtime"
        sys.modules["runtime"] = runtime_pkg
        contracts_mod = importlib.import_module("runtime.domain.contracts")
        for attr in (
            "AIRequestContext",
            "ALL_ROUTING_SOURCES",
            "OUTPUT_KIND_CHAT_REPLY",
            "OUTPUT_KIND_INSIGHT_NOTIFICATION",
            "ROUTING_SOURCE_COPILOT_AGENT",
            "ROUTING_SOURCE_ORCHESTRATOR",
            "ROUTING_SOURCE_READ_FALLBACK",
            "SERVICE_KIND_INSIGHT",
            "is_known_routing_source",
            "normalize_routing_source",
        ):
            setattr(runtime_pkg, attr, getattr(contracts_mod, attr))
        completions_mod = importlib.import_module("runtime.completions")
        for attr in ("LLMError", "build_llm_client", "validate_json_completion"):
            setattr(runtime_pkg, attr, getattr(completions_mod, attr))

    if "runtime.logging" not in sys.modules:
        logging_mod = _t.ModuleType("runtime.logging")

        class _Logger:
            def info(self, *args, **kwargs):
                return None

            def warning(self, *args, **kwargs):
                return None

            def error(self, *args, **kwargs):
                return None

            def exception(self, *args, **kwargs):
                return None

        def _get_logger(name: str = ""):
            _ = name
            return _Logger()

        def _noop(*args, **kwargs):
            return None

        def _format_log_event(*args, **kwargs):
            _ = (args, kwargs)
            return {}

        def _get_request_id() -> str | None:
            return None

        logging_mod.get_logger = _get_logger  # type: ignore[attr-defined]
        logging_mod.update_request_context = _noop  # type: ignore[attr-defined]
        logging_mod.bind_request_context = _noop  # type: ignore[attr-defined]
        logging_mod.clear_request_context = _noop  # type: ignore[attr-defined]
        logging_mod.configure_logging = _noop  # type: ignore[attr-defined]
        logging_mod.format_log_event = _format_log_event  # type: ignore[attr-defined]
        logging_mod.get_request_id = _get_request_id  # type: ignore[attr-defined]
        sys.modules["runtime.logging"] = logging_mod

    if "httpserver" not in sys.modules:
        httpserver_mod = _t.ModuleType("httpserver")
        httpserver_mod.__path__ = []  # type: ignore[attr-defined]
        sys.modules["httpserver"] = httpserver_mod

    if "httpserver.errors" not in sys.modules:
        errors_mod = _t.ModuleType("httpserver.errors")

        from dataclasses import dataclass, field
        from typing import Any

        @dataclass(slots=True)
        class AppError(Exception):
            code: str
            message: str
            status_code: int = 400
            details: dict[str, Any] = field(default_factory=dict)

            def __str__(self) -> str:
                return self.message

        def error_payload(
            code: str = "",
            message: str = "",
            request_id: str = "",
            details: dict | None = None,
        ):
            return {
                "error": {
                    "code": code,
                    "message": message,
                    "details": details or {},
                    "request_id": request_id,
                }
            }

        errors_mod.AppError = AppError  # type: ignore[attr-defined]
        errors_mod.error_payload = error_payload  # type: ignore[attr-defined]
        sys.modules["httpserver.errors"] = errors_mod


_ensure_runtime_package_stub()
