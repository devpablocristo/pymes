from __future__ import annotations

from fastapi import FastAPI, Request
from fastapi.testclient import TestClient

from core_ai.auth import AuthMiddleware


class Settings:
    backend_url = "http://cp-backend:8080"
    internal_service_token = "local-internal-token"
    auth_allow_api_key = True
    jwks_url = ""
    jwt_issuer = ""


def create_app() -> FastAPI:
    app = FastAPI()
    app.add_middleware(AuthMiddleware, settings=Settings())

    @app.get("/v1/professionals/teachers/chat")
    async def teachers_chat(request: Request) -> dict[str, object]:
        auth = request.state.auth
        return {
            "org_id": auth.org_id,
            "actor": auth.actor,
            "role": auth.role,
            "scopes": auth.scopes,
        }

    @app.get("/v1/workshops/auto-repair/chat")
    async def auto_repair_chat(request: Request) -> dict[str, object]:
        auth = request.state.auth
        return {
            "org_id": auth.org_id,
            "actor": auth.actor,
            "role": auth.role,
            "scopes": auth.scopes,
        }

    @app.get("/v1/workshops/auto-repair/public/demo/chat")
    async def auto_repair_public_chat() -> dict[str, str]:
        return {"status": "ok"}

    return app


def test_professionals_chat_api_key_uses_resolved_identity(monkeypatch) -> None:
    async def fake_resolve(_self: AuthMiddleware, _api_key: str, _request_id: str) -> dict[str, object]:
        return {
            "id": "key-123",
            "org_id": "org-123",
            "scopes": ["customers:read", "customers:write"],
        }

    monkeypatch.setattr(AuthMiddleware, "_resolve_api_key", fake_resolve)
    client = TestClient(create_app())

    response = client.get(
        "/v1/professionals/teachers/chat",
        headers={
            "X-API-KEY": "psk_test",
            "X-Actor": "spoofed-user",
            "X-Role": "admin",
            "X-Org-ID": "spoofed-org",
            "X-Scopes": "customers:write,unknown:scope",
        },
    )

    assert response.status_code == 200
    assert response.json() == {
        "org_id": "org-123",
        "actor": "api_key:key-123",
        "role": "service",
        "scopes": ["customers:write"],
    }


def test_professionals_chat_rejects_unknown_api_key(monkeypatch) -> None:
    async def fake_resolve(_self: AuthMiddleware, _api_key: str, _request_id: str) -> None:
        return None

    monkeypatch.setattr(AuthMiddleware, "_resolve_api_key", fake_resolve)
    client = TestClient(create_app())

    response = client.get("/v1/professionals/teachers/chat", headers={"X-API-KEY": "psk_invalid"})

    assert response.status_code == 401
    assert response.json()["error"]["code"] == "unauthorized"
    assert response.json()["error"]["message"] == "invalid api key"


def test_workshops_chat_api_key_uses_resolved_identity(monkeypatch) -> None:
    async def fake_resolve(_self: AuthMiddleware, _api_key: str, _request_id: str) -> dict[str, object]:
        return {
            "id": "key-456",
            "org_id": "org-456",
            "scopes": ["work_orders:read", "work_orders:write"],
        }

    monkeypatch.setattr(AuthMiddleware, "_resolve_api_key", fake_resolve)
    client = TestClient(create_app())

    response = client.get(
        "/v1/workshops/auto-repair/chat",
        headers={
            "X-API-KEY": "psk_test",
            "X-Org-ID": "spoofed-org",
            "X-Scopes": "work_orders:read,unknown:scope",
        },
    )

    assert response.status_code == 200
    assert response.json() == {
        "org_id": "org-456",
        "actor": "api_key:key-456",
        "role": "service",
        "scopes": ["work_orders:read"],
    }


def test_workshops_public_chat_is_not_authenticated() -> None:
    client = TestClient(create_app())

    response = client.get("/v1/workshops/auto-repair/public/demo/chat")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
