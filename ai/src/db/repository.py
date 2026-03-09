from __future__ import annotations

import copy
from datetime import UTC, date, datetime
from typing import Any
from uuid import uuid4

from sqlalchemy import and_, delete, select, text
from sqlalchemy.ext.asyncio import AsyncSession

from src.db.models import AIAgentEvent, AIConversation, AIDossier, AIUsageDaily


DEFAULT_DOSSIER: dict[str, Any] = {
    "business": {
        "name": "",
        "type": "",
        "profile": "",
        "description": "",
        "currency": "ARS",
        "secondary_currency": None,
        "tax_rate": 21.0,
    },
    "onboarding": {
        "status": "pending",
        "current_step": "welcome",
        "steps_completed": [],
        "steps_skipped": [],
    },
    "modules_active": [],
    "modules_inactive": [],
    "preferences": {},
    "team": [],
    "learned_context": [],
    "kpis_baseline": {},
}


def _deep_merge(target: dict[str, Any], patch: dict[str, Any]) -> dict[str, Any]:
    merged = copy.deepcopy(target)
    for key, value in patch.items():
        if key in merged and isinstance(merged[key], dict) and isinstance(value, dict):
            merged[key] = _deep_merge(merged[key], value)
        else:
            merged[key] = value
    return merged


class AIRepository:
    def __init__(self, db: AsyncSession) -> None:
        self.db = db

    async def get_or_create_dossier(self, org_id: str) -> dict[str, Any]:
        row = await self.db.get(AIDossier, org_id)
        if row:
            return row.dossier
        now = datetime.now(UTC)
        row = AIDossier(org_id=org_id, dossier=copy.deepcopy(DEFAULT_DOSSIER), version=1, created_at=now, updated_at=now)
        self.db.add(row)
        await self.db.commit()
        return row.dossier

    async def update_dossier(self, org_id: str, patch: dict[str, Any]) -> dict[str, Any]:
        row = await self.db.get(AIDossier, org_id)
        now = datetime.now(UTC)
        if row is None:
            row = AIDossier(org_id=org_id, dossier=copy.deepcopy(DEFAULT_DOSSIER), version=1, created_at=now, updated_at=now)
            self.db.add(row)
        row.dossier = _deep_merge(row.dossier, patch)
        row.version = int(row.version) + 1
        row.updated_at = now
        await self.db.commit()
        return row.dossier

    async def create_conversation(
        self,
        org_id: str,
        mode: str,
        user_id: str | None = None,
        external_contact: str = "",
        title: str = "",
    ) -> AIConversation:
        now = datetime.now(UTC)
        agent_party_id = await self.get_agent_party_id(org_id)
        row = AIConversation(
            id=str(uuid4()),
            org_id=org_id,
            user_id=user_id,
            agent_party_id=agent_party_id,
            mode=mode,
            external_contact=external_contact,
            title=title,
            messages=[],
            tool_calls_count=0,
            tokens_input=0,
            tokens_output=0,
            created_at=now,
            updated_at=now,
        )
        self.db.add(row)
        await self.db.commit()
        await self.db.refresh(row)
        return row

    async def get_agent_party_id(self, org_id: str) -> str | None:
        query = text(
            """
            SELECT p.id
            FROM parties p
            JOIN party_agents pa ON pa.party_id = p.id
            WHERE p.org_id = :org_id
              AND p.party_type = 'automated_agent'
              AND pa.agent_kind = 'ai'
            ORDER BY p.created_at ASC
            LIMIT 1
            """
        )
        row = await self.db.execute(query, {"org_id": org_id})
        party_id = row.scalar_one_or_none()
        if party_id is None:
            return None
        return str(party_id)

    async def get_conversation(self, org_id: str, conversation_id: str) -> AIConversation | None:
        query = select(AIConversation).where(
            and_(AIConversation.org_id == org_id, AIConversation.id == conversation_id)
        )
        result = await self.db.execute(query)
        return result.scalar_one_or_none()

    async def get_latest_external_conversation(self, org_id: str, external_contact: str) -> AIConversation | None:
        query = (
            select(AIConversation)
            .where(
                AIConversation.org_id == org_id,
                AIConversation.mode == "external",
                AIConversation.external_contact == external_contact,
            )
            .order_by(AIConversation.updated_at.desc())
            .limit(1)
        )
        result = await self.db.execute(query)
        return result.scalar_one_or_none()

    async def list_conversations(self, org_id: str, mode: str, user_id: str | None, limit: int = 50) -> list[AIConversation]:
        query = select(AIConversation).where(AIConversation.org_id == org_id, AIConversation.mode == mode)
        if mode == "internal":
            if user_id is None:
                query = query.where(AIConversation.user_id.is_(None))
            else:
                query = query.where(AIConversation.user_id == user_id)
        query = query.order_by(AIConversation.updated_at.desc()).limit(limit)
        result = await self.db.execute(query)
        return list(result.scalars().all())

    async def append_messages(
        self,
        org_id: str,
        conversation_id: str,
        new_messages: list[dict[str, Any]],
        tool_calls_count: int,
        tokens_input: int,
        tokens_output: int,
    ) -> AIConversation | None:
        row = await self.get_conversation(org_id, conversation_id)
        if row is None:
            return None
        row.messages = [*row.messages, *new_messages]
        row.tool_calls_count += tool_calls_count
        row.tokens_input += tokens_input
        row.tokens_output += tokens_output
        row.updated_at = datetime.now(UTC)
        if not row.title:
            first_user = next((m.get("content", "") for m in row.messages if m.get("role") == "user"), "")
            row.title = first_user[:60]
        await self.db.commit()
        await self.db.refresh(row)
        return row

    async def delete_conversation(self, org_id: str, conversation_id: str) -> bool:
        result = await self.db.execute(
            delete(AIConversation).where(AIConversation.org_id == org_id, AIConversation.id == conversation_id)
        )
        await self.db.commit()
        return bool(result.rowcount)

    async def track_usage(self, org_id: str, tokens_in: int, tokens_out: int) -> None:
        today = date.today()
        existing = await self.db.execute(
            select(AIUsageDaily).where(AIUsageDaily.org_id == org_id, AIUsageDaily.usage_date == today)
        )
        row = existing.scalar_one_or_none()
        if row is None:
            row = AIUsageDaily(
                org_id=org_id,
                usage_date=today,
                queries=1,
                tokens_input=tokens_in,
                tokens_output=tokens_out,
            )
            self.db.add(row)
        else:
            row.queries += 1
            row.tokens_input += tokens_in
            row.tokens_output += tokens_out
        await self.db.commit()

    async def get_month_usage(self, org_id: str, year: int, month: int) -> dict[str, int]:
        start = date(year, month, 1)
        if month == 12:
            end = date(year + 1, 1, 1)
        else:
            end = date(year, month + 1, 1)
        query = select(AIUsageDaily).where(
            AIUsageDaily.org_id == org_id,
            AIUsageDaily.usage_date >= start,
            AIUsageDaily.usage_date < end,
        )
        rows = (await self.db.execute(query)).scalars().all()
        return {
            "queries": sum(r.queries for r in rows),
            "tokens_input": sum(r.tokens_input for r in rows),
            "tokens_output": sum(r.tokens_output for r in rows),
        }

    async def count_external_conversations_in_month(self, org_id: str, year: int, month: int) -> int:
        start = datetime(year, month, 1, tzinfo=UTC)
        if month == 12:
            end = datetime(year + 1, 1, 1, tzinfo=UTC)
        else:
            end = datetime(year, month + 1, 1, tzinfo=UTC)
        query = text(
            """
            SELECT COUNT(*)
            FROM ai_conversations
            WHERE org_id = :org_id
              AND mode = 'external'
              AND created_at >= :start
              AND created_at < :end
            """
        )
        row = await self.db.execute(query, {"org_id": org_id, "start": start, "end": end})
        return int(row.scalar_one() or 0)

    async def get_plan_code(self, org_id: str) -> str:
        row = await self.db.execute(
            text("SELECT plan_code FROM tenant_settings WHERE org_id = :org_id LIMIT 1"),
            {"org_id": org_id},
        )
        plan = row.scalar_one_or_none()
        if not plan:
            return "starter"
        return str(plan).strip().lower() or "starter"

    async def has_agent_request(self, org_id: str, request_id: str) -> bool:
        normalized = str(request_id).strip()
        if not normalized:
            return False
        query = select(AIAgentEvent.id).where(
            AIAgentEvent.org_id == org_id,
            AIAgentEvent.external_request_id == normalized,
        ).limit(1)
        row = await self.db.execute(query)
        return row.scalar_one_or_none() is not None

    async def record_agent_event(
        self,
        *,
        org_id: str,
        conversation_id: str | None,
        agent_mode: str,
        channel: str,
        actor_id: str,
        actor_type: str,
        action: str,
        result: str,
        confirmed: bool,
        tool_name: str = "",
        entity_type: str = "",
        entity_id: str = "",
        external_request_id: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> None:
        now = datetime.now(UTC)
        row = AIAgentEvent(
            id=str(uuid4()),
            org_id=org_id,
            conversation_id=conversation_id,
            external_request_id=(external_request_id or "").strip() or None,
            agent_mode=agent_mode,
            channel=channel,
            actor_id=actor_id,
            actor_type=actor_type,
            action=action,
            tool_name=tool_name,
            entity_type=entity_type,
            entity_id=entity_id,
            result=result,
            confirmed=confirmed,
            event_metadata=metadata or {},
            created_at=now,
        )
        self.db.add(row)
        await self.db.commit()
