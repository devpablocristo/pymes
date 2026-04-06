"""Sub-agente: Cobros — pagos, links de pago y cuentas corrientes."""

from __future__ import annotations

from typing import Any
from urllib.parse import quote

from runtime.domain.agent import SubAgent, SubAgentDescriptor
from runtime.types import ToolDeclaration

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.tools import accounts, payments
from src.agents.sub_agents.common import build_default_limits

DESCRIPTOR = SubAgentDescriptor(
    name="collections",
    description="Generar links de pago, consultar estado de cobros y saldos de cuentas corrientes",
)

SYSTEM_PROMPT = """\
Sos el agente de cobros de una plataforma de gestion para PyMEs.
Podes generar links de pago para ventas o presupuestos, consultar el estado de un cobro,
enviar info de pago por WhatsApp y consultar saldos de cuentas corrientes.
Si el usuario pregunta por cobros pendientes, deuda o saldos generales, usa get_account_balances.
Usa get_payment_status solo cuando el usuario se refiere a una venta o presupuesto especifico.
Responde siempre en espanol, claro y directo. No muestres JSON al usuario."""


def build(client: BackendClient, auth: AuthContext) -> SubAgent:
    async def generate_payment_link(*, org_id: str, reference_type: str, reference_id: str) -> dict[str, Any]:
        kind = reference_type.strip().lower()
        safe_reference_id = quote(reference_id.strip(), safe="")
        if kind == "sale":
            return await client.request("POST", f"/v1/sales/{safe_reference_id}/payment-link", auth=auth)
        if kind == "quote":
            return await client.request("POST", f"/v1/quotes/{safe_reference_id}/payment-link", auth=auth)
        return {"code": "invalid_reference_type", "message": "reference_type debe ser sale o quote"}

    async def get_payment_status(*, org_id: str, reference_type: str, reference_id: str) -> dict[str, Any]:
        kind = reference_type.strip().lower()
        safe_reference_id = quote(reference_id.strip(), safe="")
        if kind == "sale":
            return await payments.get_payment_status(client, auth, sale_id=safe_reference_id)
        if kind == "quote":
            return await client.request("GET", f"/v1/quotes/{safe_reference_id}/payment-link", auth=auth)
        return {"code": "invalid_reference_type", "message": "reference_type debe ser sale o quote"}

    async def send_payment_info(*, org_id: str, sale_id: str) -> dict[str, Any]:
        return await payments.send_payment_info(client, auth, sale_id=sale_id)

    async def get_account_balances(*, org_id: str) -> dict[str, Any]:
        return await accounts.get_account_balances(client, auth)

    tools = [
        ToolDeclaration(
            name="generate_payment_link",
            description="Generar link de pago para una venta o presupuesto",
            parameters={"type": "object", "properties": {"reference_type": {"type": "string", "description": "sale o quote"}, "reference_id": {"type": "string"}}, "required": ["reference_type", "reference_id"]},
        ),
        ToolDeclaration(
            name="get_payment_status",
            description="Consultar estado de cobro de una venta o presupuesto especifico",
            parameters={"type": "object", "properties": {"reference_type": {"type": "string", "description": "sale o quote"}, "reference_id": {"type": "string"}}, "required": ["reference_type", "reference_id"]},
        ),
        ToolDeclaration(
            name="send_payment_info",
            description="Obtener mensaje de WhatsApp con datos de pago para una venta",
            parameters={"type": "object", "properties": {"sale_id": {"type": "string"}}, "required": ["sale_id"]},
        ),
        ToolDeclaration(
            name="get_account_balances",
            description="Consultar saldos de cuentas corrientes, deuda vigente y cobros pendientes en general",
            parameters={"type": "object", "properties": {}},
        ),
    ]

    return SubAgent(
        descriptor=DESCRIPTOR,
        tools=tools,
        tool_handlers={
            "generate_payment_link": generate_payment_link,
            "get_payment_status": get_payment_status,
            "send_payment_info": send_payment_info,
            "get_account_balances": get_account_balances,
        },
        system_prompt=SYSTEM_PROMPT,
        limits=build_default_limits(),
    )
