from __future__ import annotations

from datetime import date, datetime
from typing import Any

from sqlalchemy import JSON, Boolean, Date, DateTime, ForeignKey, Integer, String, Text
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


class AIDossier(Base):
    __tablename__ = "ai_dossiers"

    org_id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True)
    dossier: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict)
    version: Mapped[int] = mapped_column(Integer, default=1, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)


class AIConversation(Base):
    __tablename__ = "ai_conversations"

    id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True)
    org_id: Mapped[str] = mapped_column(UUID(as_uuid=False), nullable=False, index=True)
    user_id: Mapped[str | None] = mapped_column(UUID(as_uuid=False), nullable=True, index=True)
    agent_party_id: Mapped[str | None] = mapped_column(UUID(as_uuid=False), nullable=True, index=True)
    mode: Mapped[str] = mapped_column(String(20), default="internal", nullable=False)
    external_contact: Mapped[str] = mapped_column(Text, default="", nullable=False)
    title: Mapped[str] = mapped_column(Text, default="", nullable=False)
    messages: Mapped[list[dict[str, Any]]] = mapped_column(JSON, default=list, nullable=False)
    tool_calls_count: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    tokens_input: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    tokens_output: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    # Campos para atención al cliente gobernada (WhatsApp + Review)
    channel: Mapped[str | None] = mapped_column(String(32), nullable=True)
    contact_phone: Mapped[str | None] = mapped_column(String(32), nullable=True)
    contact_name: Mapped[str | None] = mapped_column(String(255), nullable=True)
    party_id: Mapped[str | None] = mapped_column(UUID(as_uuid=False), nullable=True)
    pending_action: Mapped[dict[str, Any] | None] = mapped_column(JSON, nullable=True)
    review_request_id: Mapped[str | None] = mapped_column(UUID(as_uuid=False), nullable=True)
    review_status: Mapped[str | None] = mapped_column(String(32), nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)


class AIUsageDaily(Base):
    __tablename__ = "ai_usage_daily"

    org_id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True)
    usage_date: Mapped[date] = mapped_column(Date, primary_key=True)
    queries: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    tokens_input: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    tokens_output: Mapped[int] = mapped_column(Integer, default=0, nullable=False)


class AIAgentEvent(Base):
    __tablename__ = "ai_agent_events"

    id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True)
    org_id: Mapped[str] = mapped_column(UUID(as_uuid=False), nullable=False, index=True)
    conversation_id: Mapped[str | None] = mapped_column(
        UUID(as_uuid=False),
        ForeignKey("ai_conversations.id", ondelete="SET NULL"),
        nullable=True,
        index=True,
    )
    external_request_id: Mapped[str | None] = mapped_column(Text, nullable=True, index=True)
    agent_mode: Mapped[str] = mapped_column(String(40), nullable=False)
    channel: Mapped[str] = mapped_column(String(40), nullable=False)
    actor_id: Mapped[str] = mapped_column(Text, nullable=False)
    actor_type: Mapped[str] = mapped_column(String(40), nullable=False)
    action: Mapped[str] = mapped_column(Text, nullable=False)
    tool_name: Mapped[str] = mapped_column(Text, default="", nullable=False)
    entity_type: Mapped[str] = mapped_column(Text, default="", nullable=False)
    entity_id: Mapped[str] = mapped_column(Text, default="", nullable=False)
    result: Mapped[str] = mapped_column(String(40), nullable=False)
    confirmed: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)
    event_metadata: Mapped[dict[str, Any]] = mapped_column("metadata", JSON, default=dict, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
