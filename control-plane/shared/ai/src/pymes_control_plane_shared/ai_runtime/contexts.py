from __future__ import annotations

from dataclasses import dataclass


@dataclass
class AuthContext:
    org_id: str
    actor: str
    role: str
    scopes: list[str]
    mode: str
    authorization: str | None = None
    api_key: str | None = None
    api_actor: str | None = None
    api_role: str | None = None
    api_scopes: str | None = None
