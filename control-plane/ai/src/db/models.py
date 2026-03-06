from __future__ import annotations

from datetime import date, datetime
from typing import Any

from sqlalchemy import JSON, Date, DateTime, ForeignKey, Integer, String, Text
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


class AIDossier(Base):
    __tablename__ = "ai_dossiers"

    org_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("orgs.id", ondelete="CASCADE"), primary_key=True)
    dossier: Mapped[dict[str, Any]] = mapped_column(JSON, default=dict)
    version: Mapped[int] = mapped_column(Integer, default=1, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)


class AIConversation(Base):
    __tablename__ = "ai_conversations"

    id: Mapped[str] = mapped_column(UUID(as_uuid=False), primary_key=True)
    org_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("orgs.id", ondelete="CASCADE"), nullable=False, index=True)
    user_id: Mapped[str | None] = mapped_column(UUID(as_uuid=False), ForeignKey("users.id"), nullable=True, index=True)
    agent_party_id: Mapped[str | None] = mapped_column(UUID(as_uuid=False), ForeignKey("parties.id"), nullable=True, index=True)
    mode: Mapped[str] = mapped_column(String(20), default="internal", nullable=False)
    external_contact: Mapped[str] = mapped_column(Text, default="", nullable=False)
    title: Mapped[str] = mapped_column(Text, default="", nullable=False)
    messages: Mapped[list[dict[str, Any]]] = mapped_column(JSON, default=list, nullable=False)
    tool_calls_count: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    tokens_input: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    tokens_output: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)


class AIUsageDaily(Base):
    __tablename__ = "ai_usage_daily"

    org_id: Mapped[str] = mapped_column(UUID(as_uuid=False), ForeignKey("orgs.id", ondelete="CASCADE"), primary_key=True)
    usage_date: Mapped[date] = mapped_column(Date, primary_key=True)
    queries: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    tokens_input: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    tokens_output: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
