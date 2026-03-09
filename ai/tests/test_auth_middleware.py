from __future__ import annotations

from fastapi import FastAPI, Request
from fastapi.testclient import TestClient

from pymes_control_plane_shared.ai_runtime import AuthMiddleware


class Settings:
    backend_url = "http://cp-backend:8080"
    internal_service_token = "local-internal-token"
    auth_allow_api_key = True
    jwks_url = ""
    jwt_issuer = ""


def create_app() -> FastAPI:
    app = FastAPI()
    app.add_middleware(AuthMiddleware, settings=Settings())

    @app.get("/v1/professionals/chat")
    async def professionals_chat(request: Request) -> dict[str, object]:
        auth = request.state.auth
        return {
            "org_id": auth.org_id,
            "actor": auth.actor,
            "role": auth.role,
            "scopes": auth.scopes,
        }

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
        "/v1/professionals/chat",
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

    response = client.get("/v1/professionals/chat", headers={"X-API-KEY": "psk_invalid"})

    assert response.status_code == 401
    assert response.json()["error"]["code"] == "unauthorized"
    assert response.json()["error"]["message"] == "invalid api key"
