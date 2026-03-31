from __future__ import annotations

try:
    from runtime import (
        DEFAULT_LANGUAGE_CODE,
        OUTPUT_KIND_CHAT_REPLY,
        OUTPUT_KIND_INSIGHT_NOTIFICATION,
        SERVICE_KIND_INSIGHT,
        normalize_language_code,
    )
except ImportError:
    DEFAULT_LANGUAGE_CODE = "es"
    OUTPUT_KIND_CHAT_REPLY = "chat_reply"
    OUTPUT_KIND_INSIGHT_NOTIFICATION = "insight_notification"
    SERVICE_KIND_INSIGHT = "insight_service"

    def normalize_language_code(name: str | None) -> str:
        if str(name or "").strip().lower() in {"en", "es"}:
            return str(name or "").strip().lower()
        return DEFAULT_LANGUAGE_CODE
