from __future__ import annotations

from types import SimpleNamespace

from src import runtime_bootstrap


def test_build_llm_provider_uses_vertex_when_project_is_configured(monkeypatch) -> None:
    calls: dict[str, str] = {}

    class StubGeminiProvider:
        def __init__(self, *, api_key: str, model: str) -> None:
            calls["api_key"] = api_key
            calls["model"] = model
            self.client = None

    class StubVertexClient:
        def __init__(self, *, vertexai: bool, project: str, location: str) -> None:
            calls["vertexai"] = "true" if vertexai else "false"
            calls["vertex_project"] = project
            calls["vertex_location"] = location

    monkeypatch.setattr(runtime_bootstrap, "GeminiProvider", StubGeminiProvider)
    monkeypatch.setattr(runtime_bootstrap.genai, "Client", StubVertexClient)

    provider = runtime_bootstrap.build_llm_provider(
        SimpleNamespace(
            llm_provider="gemini",
            gemini_model="gemini-2.5-flash",
            gemini_vertex_project="pymes-dev-352318",
            gemini_vertex_location="global",
            gemini_api_key="",
        )
    )

    assert isinstance(provider, StubGeminiProvider)
    assert calls == {
        "api_key": "vertex-ai",
        "model": "gemini-2.5-flash",
        "vertexai": "true",
        "vertex_project": "pymes-dev-352318",
        "vertex_location": "global",
    }


def test_claim_helpers_support_nested_clerk_claims() -> None:
    claims = {
        "sub": "user_123",
        "o": {
            "id": "org_123",
            "rol": "org:admin",
            "per": ["admin:console:read", "admin:console:write"],
        },
    }

    assert runtime_bootstrap._first_string_claim(claims, "sub") == "user_123"
    assert runtime_bootstrap._first_string_claim(claims, "tenant_id", "org_id", "o.id") == "org_123"
    assert runtime_bootstrap._first_string_claim(claims, "role", "org_role", "o.rol") == "org:admin"
    assert runtime_bootstrap._first_scopes_claim(claims, "scopes", "org_permissions", "o.per") == [
        "admin:console:read",
        "admin:console:write",
    ]
    assert runtime_bootstrap._clerk_compact_org_id_from_claims(claims) == "org_123"


def test_split_scopes_accepts_csv_and_spaces() -> None:
    assert runtime_bootstrap._parse_scopes("admin:console:read, admin:console:write sales:read") == [
        "admin:console:read",
        "admin:console:write",
        "sales:read",
    ]
