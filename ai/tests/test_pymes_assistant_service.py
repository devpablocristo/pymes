from __future__ import annotations

from types import SimpleNamespace

import pytest

from src.agents.service import run_internal_orchestrated_chat
from src.backend_client.auth import AuthContext


class StubRepo:
    def __init__(self) -> None:
        self.append_calls: list[dict[str, object]] = []
        self.track_calls: list[dict[str, int | str]] = []
        self.agent_events: list[dict[str, object]] = []

    async def append_messages(self, **kwargs):
        self.append_calls.append(kwargs)
        return SimpleNamespace(id=kwargs["conversation_id"])

    async def track_usage(self, org_id: str, tokens_in: int, tokens_out: int) -> None:
        self.track_calls.append({"org_id": org_id, "tokens_in": tokens_in, "tokens_out": tokens_out})

    async def record_agent_event(self, **kwargs) -> None:
        self.agent_events.append(kwargs)


class StubRegistry:
    def names(self) -> list[str]:
        return []

    def get(self, _name: str):
        return None


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_persists_routed_agent(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="clientes", tool_call=None)
        yield SimpleNamespace(type="tool_call", text=None, tool_call=SimpleNamespace(name="search_customers"))
        yield SimpleNamespace(type="text", text="Encontré 3 clientes.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="listame los clientes",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "clientes"
    assert result.tool_calls == ["search_customers"]
    assert repo.append_calls
    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert assistant_message["routed_agent"] == "clientes"
    assert assistant_message["routed_mode"] == "clientes"
    assert repo.track_calls == [{"org_id": "org-123", "tokens_in": result.tokens_input, "tokens_out": result.tokens_output}]
    assert repo.agent_events[-1]["action"] == "chat.completed"
    assert repo.agent_events[-1]["agent_mode"] == "clientes"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_uses_general_fallback(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="hola",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "general"
    assert "clientes, productos, ventas, cobros y compras" in result.reply


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_requires_confirmation_for_sensitive_tools(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="ventas", tool_call=None)
        yield SimpleNamespace(type="tool_call", text=None, tool_call=SimpleNamespace(name="create_sale"))
        yield SimpleNamespace(
            type="tool_result",
            text=None,
            tool_call=SimpleNamespace(
                name="create_sale",
                arguments={
                    "pending_confirmation": True,
                    "required_action": "create_sale",
                },
            ),
        )

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="registrá una venta",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.pending_confirmations == ["create_sale"]
    assert "confirmed_actions" in result.reply
    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert assistant_message["pending_confirmations"] == ["create_sale"]
    assert repo.agent_events[-1]["result"] == "confirmation_required"
