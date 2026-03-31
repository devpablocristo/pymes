from __future__ import annotations

from typing import Literal

from src.runtime_contracts import DEFAULT_LANGUAGE_CODE, normalize_language_code

LanguageCode = Literal["es", "en"]


def resolve_preferred_language(
    preferred_language: str | None,
    *,
    accept_language: str | None = None,
) -> str:
    normalized_preferred = normalize_language_code(preferred_language)
    if preferred_language is not None and preferred_language.strip():
        return normalized_preferred
    if not accept_language:
        return DEFAULT_LANGUAGE_CODE
    for raw_token in accept_language.split(","):
        candidate = raw_token.split(";", 1)[0].strip().lower()
        if not candidate:
            continue
        primary = candidate.split("-", 1)[0]
        normalized = normalize_language_code(primary)
        if normalized == primary:
            return normalized
    return DEFAULT_LANGUAGE_CODE
