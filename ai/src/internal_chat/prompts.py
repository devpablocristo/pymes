from __future__ import annotations

import json
from typing import Any

from src.internal_chat.evidence import EvidencePacket
from src.internal_chat.routing import InternalRouteDecision

INTERNAL_CHAT_SYSTEM_PROMPT = """\
Sos el asistente interno de gestión de Pymes SaaS.
Tu única fuente de verdad es la evidencia JSON provista por el backend.
No inventes clientes, ventas, deudas, pagos, stock, agenda ni compras.
Si la evidencia no alcanza para responder una parte, decilo explícitamente.
No ejecutes acciones ni prometas que creaste o modificaste datos: esta versión es solo lectura.
Respondé en español rioplatense, claro, breve y accionable.
No muestres JSON ni nombres técnicos de endpoints.
"""


def build_internal_chat_user_prompt(
    *,
    message: str,
    decision: InternalRouteDecision,
    evidence: EvidencePacket,
    business_context: dict[str, Any] | None = None,
    deterministic_summary: str | None = None,
) -> str:
    payload = evidence.prompt_payload()
    business = business_context or {}
    business_name = ""
    if isinstance(business.get("business"), dict):
        business_name = str(business["business"].get("name") or "").strip()
    parts = [
        f"Negocio: {business_name or 'no informado'}",
        f"Pregunta del usuario: {message}",
        f"Alcance decidido: {decision.scope}",
        f"Razón de ruteo: {decision.reason}",
    ]
    if deterministic_summary:
        parts.extend(
            [
                "Resumen determinista ya calculado por backend/reportes:",
                deterministic_summary,
            ]
        )
    parts.extend(
        [
            "Evidencia real del backend:",
            json.dumps(payload, ensure_ascii=False, default=str),
            "",
            "Instrucciones de respuesta:",
            "- Contestá directamente la pregunta.",
            "- No recalcules los totales: usá el resumen determinista y la evidencia como fuente.",
            "- Si hay datos de ventas/cobros, priorizá riesgos y oportunidades concretas.",
            "- Nombrá clientes/productos/servicios solo si aparecen en la evidencia.",
            "- Si faltan datos, aclaralo y sugerí qué dato revisar después.",
            "- Cerrá con 1 a 3 próximos pasos concretos cuando tenga sentido.",
        ]
    )
    return "\n".join(parts)
