from __future__ import annotations

from dataclasses import dataclass
from typing import Literal

from src.backend_client.auth import AuthContext
from src.agents.tool_access import (
    COMMERCIAL_EXTERNAL_SALES_TOOLS,
    resolve_commercial_internal_procurement_tools,
    resolve_commercial_internal_sales_tools,
)

AgentMode = Literal["external_sales", "internal_sales", "internal_procurement"]
Channel = Literal["web_public", "whatsapp", "api", "embedded", "internal_ui"]


@dataclass(frozen=True)
class CommercialPolicy:
    agent_mode: AgentMode
    channel: Channel
    allowed_tools: frozenset[str]
    confirm_required_tools: frozenset[str]
    max_tool_calls: int
    tool_timeout_seconds: int
    total_timeout_seconds: int

    def allows(self, tool_name: str) -> bool:
        return tool_name in self.allowed_tools

    def requires_confirmation(self, tool_name: str) -> bool:
        return tool_name in self.confirm_required_tools


EXTERNAL_CONFIRM_REQUIRED = frozenset({"book_scheduling"})
INTERNAL_SALES_CONFIRM_REQUIRED = frozenset({"create_quote", "create_sale", "generate_payment_link"})
INTERNAL_PROCUREMENT_CONFIRM_REQUIRED = frozenset()


def build_external_sales_policy(channel: Channel = "web_public") -> CommercialPolicy:
    return CommercialPolicy(
        agent_mode="external_sales",
        channel=channel,
        allowed_tools=COMMERCIAL_EXTERNAL_SALES_TOOLS,
        confirm_required_tools=EXTERNAL_CONFIRM_REQUIRED,
        max_tool_calls=5,
        tool_timeout_seconds=8,
        total_timeout_seconds=30,
    )


def build_internal_sales_policy(auth: AuthContext, modules_active: list[str], channel: Channel = "internal_ui") -> CommercialPolicy:
    allowed_tools = resolve_commercial_internal_sales_tools(auth.role, modules_active)
    return CommercialPolicy(
        agent_mode="internal_sales",
        channel=channel,
        allowed_tools=allowed_tools,
        confirm_required_tools=INTERNAL_SALES_CONFIRM_REQUIRED.intersection(allowed_tools),
        max_tool_calls=6,
        tool_timeout_seconds=10,
        total_timeout_seconds=45,
    )


def build_internal_procurement_policy(auth: AuthContext, modules_active: list[str], channel: Channel = "internal_ui") -> CommercialPolicy:
    allowed_tools = resolve_commercial_internal_procurement_tools(auth.role, modules_active)
    return CommercialPolicy(
        agent_mode="internal_procurement",
        channel=channel,
        allowed_tools=allowed_tools,
        confirm_required_tools=INTERNAL_PROCUREMENT_CONFIRM_REQUIRED,
        max_tool_calls=8,
        tool_timeout_seconds=10,
        total_timeout_seconds=45,
    )
