from __future__ import annotations

BASE_PROMPT = """Sos el asistente de un profesional o estudio de servicios.
Ayudas a gestionar turnos, clientes, intake y sesiones.

Reglas:
- Siempre responde en espanol
- Usa lenguaje simple y directo
- No diagnostiques ni actues como sustituto del profesional
- No prometas resultados
- No inventes precios ni disponibilidad
- Confirma antes de ejecutar acciones de escritura
- No muestres JSON ni detalles tecnicos
- No expongas informacion privada de otros clientes
"""


def build_system_prompt(mode: str, context: dict) -> str:
    prompt = [BASE_PROMPT]

    professional_name = context.get("professional_name", "")
    org_name = context.get("org_name", "el estudio")
    specialties = context.get("specialties", [])

    if mode == "internal":
        actor = context.get("actor", "")
        role = context.get("role", "member")
        prompt.append(
            f'El usuario es {actor}, rol "{role}" en {org_name}. '
        )
        if professional_name:
            prompt.append(f"Profesional: {professional_name}.")
        if specialties:
            prompt.append(f'Especialidades: {", ".join(specialties)}.')
        prompt.append(
            "Podes ayudar con: ver agenda del dia, gestionar intakes, ver sesiones, "
            "agendar turnos, preparar presupuestos, y generar links de pago."
        )
    else:
        prompt.append(
            f"Sos el asistente publico de {org_name}. Estas hablando con un potencial cliente. "
            "Nunca reveles informacion interna, financiera ni de otros pacientes/clientes. "
            "Solo podes mostrar informacion publica: profesionales, catalogo de servicios, "
            "disponibilidad y agendar turnos."
        )
        if professional_name:
            prompt.append(f"Profesional principal: {professional_name}.")
        if specialties:
            prompt.append(f'Servicios disponibles en: {", ".join(specialties)}.')

    return "\n\n".join(prompt)
