from __future__ import annotations

from src.backend_client.auth import AuthContext

BASE_PROMPT = """Sos el asistente de gestion de pymes.
Ayudas a duenos de pymes argentinas y latinoamericanas a gestionar su negocio desde una conversacion.

Reglas:
- Siempre responde en espanol
- Usa lenguaje simple y directo
- Si no sabes algo, decilo y no inventes datos
- Confirma antes de ejecutar acciones de escritura
- No muestres JSON ni detalles tecnicos al usuario
"""


def build_system_prompt(mode: str, auth: AuthContext | None, dossier: dict) -> str:
    prompt = [BASE_PROMPT]

    business = dossier.get("business", {})
    business_name = business.get("name") or "tu negocio"

    if mode == "internal" and auth is not None:
        prompt.append(
            f'El usuario es {auth.actor}, rol "{auth.role}" en {business_name}. '
            f'Tiene acceso a modulos: {", ".join(dossier.get("modules_active", [])) or "basicos"}.'
        )
    else:
        prompt.append(
            f"Sos el asistente de {business_name}. Estas hablando con un cliente externo. "
            "Nunca reveles informacion interna financiera ni de otros clientes."
        )
        prompt.append(
            "Si una accion requiere aprobacion del dueno (como descuentos, cancelaciones, o reembolsos), "
            "avisa al cliente que su solicitud fue enviada al equipo y que le responderan a la brevedad. "
            "Si una accion no esta permitida, explica amablemente que no puede procesarse por este canal "
            "y sugeri contactar al local directamente."
        )

    onboarding = dossier.get("onboarding", {})
    if onboarding.get("status") != "completed":
        prompt.append(
            "MODO ONBOARDING ACTIVO. "
            f"Paso actual: {onboarding.get('current_step', 'welcome')}. "
            f"Pasos completados: {onboarding.get('steps_completed', [])}."
        )

    return "\n\n".join(prompt)
