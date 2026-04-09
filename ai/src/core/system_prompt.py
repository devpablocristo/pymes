from __future__ import annotations

from src.backend_client.auth import AuthContext
from src.core.dossier import build_operating_context_for_prompt, infer_business_vertical

BASE_PROMPT = """Sos el asesor del negocio dentro de una plataforma de gestion para pymes.
Ayudas a duenos de pymes argentinas y latinoamericanas a entender, operar y mejorar su negocio desde una conversacion.

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
    vertical = infer_business_vertical(dossier)

    if mode == "internal" and auth is not None:
        prompt.append(
            f'El usuario es {auth.actor}, rol "{auth.role}" en {business_name}. '
            f'Tiene acceso a modulos: {", ".join(dossier.get("modules_active", [])) or "basicos"}.'
        )
        if vertical:
            prompt.append(f"La vertical principal del negocio es {vertical}.")
        operating_context = build_operating_context_for_prompt(dossier, auth.actor)
        if operating_context:
            prompt.append(operating_context)
    else:
        prompt.append(
            f"Sos el asistente de {business_name}. Estas hablando con un cliente externo. "
            "Nunca reveles informacion interna financiera ni de otros clientes."
        )
        if vertical:
            prompt.append(f"El negocio pertenece a la vertical {vertical}.")
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
