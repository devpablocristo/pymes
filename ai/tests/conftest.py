from __future__ import annotations

import os
import sys
import types as _t


def _ensure_runtime_package_stub() -> None:
    if "runtime" not in sys.modules:
        src = os.path.join(os.path.dirname(__file__), "..", "..", "..", "core", "ai", "python", "src")
        runtime_pkg = _t.ModuleType("runtime")
        runtime_pkg.__path__ = [os.path.join(src, "runtime")]
        runtime_pkg.__package__ = "runtime"
        sys.modules["runtime"] = runtime_pkg

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


_ensure_runtime_package_stub()
