from __future__ import annotations

from fastapi import FastAPI, Request
from fastapi.testclient import TestClient

from runtime.auth import AuthMiddleware, AuthSettings
from runtime.contexts import AuthContext


def create_app(*, api_key_verifier: object | None, protected: tuple[str, ...], public: tuple[str, ...]) -> FastAPI:
    app = FastAPI()
    app.add_middleware(
        AuthMiddleware,
        settings=AuthSettings(allow_api_key=True),
        api_key_verifier=api_key_verifier,
        protected_prefixes=protected,
        public_prefixes=public,
    )

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


class ProfessionalsVerifier:
    async def verify_api_key(self, key: str) -> AuthContext | None:
        del key
        return AuthContext(
            tenant_id="org-123",
            actor="api_key:key-123",
            role="service",
            scopes=["customers:write"],
            mode="internal",
        )


def test_professionals_chat_api_key_uses_resolved_identity() -> None:
    client = TestClient(
        create_app(
            api_key_verifier=ProfessionalsVerifier(),
            protected=("/v1/professionals/teachers/chat",),
            public=("/v1/workshops/auto-repair/public/",),
        )
    )

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


class RejectVerifier:
    async def verify_api_key(self, key: str) -> AuthContext | None:
        del key
        return None


def test_professionals_chat_rejects_unknown_api_key() -> None:
    client = TestClient(
        create_app(
            api_key_verifier=RejectVerifier(),
            protected=("/v1/professionals/teachers/chat",),
            public=("/v1/workshops/auto-repair/public/",),
        )
    )

    response = client.get("/v1/professionals/teachers/chat", headers={"X-API-KEY": "psk_invalid"})

    assert response.status_code == 401
    assert response.json()["error"]["code"] == "unauthorized"
    assert response.json()["error"]["message"] == "unauthorized"


class WorkshopsVerifier:
    async def verify_api_key(self, key: str) -> AuthContext | None:
        del key
        return AuthContext(
            tenant_id="org-456",
            actor="api_key:key-456",
            role="service",
            scopes=["work_orders:read"],
            mode="internal",
        )


def test_workshops_chat_api_key_uses_resolved_identity() -> None:
    client = TestClient(
        create_app(
            api_key_verifier=WorkshopsVerifier(),
            protected=("/v1/workshops/auto-repair/chat",),
            public=("/v1/workshops/auto-repair/public/",),
        )
    )

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
    client = TestClient(
        create_app(
            api_key_verifier=RejectVerifier(),
            protected=("/v1/workshops/auto-repair/chat",),
            public=("/v1/workshops/auto-repair/public/",),
        )
    )

    response = client.get("/v1/workshops/auto-repair/public/demo/chat")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
