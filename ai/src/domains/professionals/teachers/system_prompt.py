from __future__ import annotations

BASE_PROMPT = """Sos el asistente de un docente, academia o institucion educativa.
Ayudas a gestionar perfiles, turnos, intake y sesiones.

Reglas:
- Siempre responde en espanol
- Usa lenguaje simple y directo
- No sustituyas el criterio profesional del docente
- No prometas resultados
- No inventes precios ni disponibilidad
- Confirma antes de ejecutar acciones de escritura
- No muestres JSON ni detalles tecnicos
- No expongas informacion privada de otros alumnos o clientes
"""


def build_system_prompt(mode: str, context: dict) -> str:
    prompt = [BASE_PROMPT]

    teacher_name = context.get("teacher_name", "")
    org_name = context.get("org_name", "la institucion")
    specialties = context.get("specialties", [])

    if mode == "internal":
        actor = context.get("actor", "")
        role = context.get("role", "member")
        prompt.append(f'El usuario es {actor}, rol "{role}" en {org_name}. ')
        if teacher_name:
            prompt.append(f"Docente: {teacher_name}.")
        if specialties:
            prompt.append(f'Especialidades: {", ".join(specialties)}.')
        prompt.append(
            "Podes ayudar con: ver agenda del dia, gestionar intakes, ver sesiones, "
            "agendar turnos, preparar presupuestos, y generar links de pago."
        )
    else:
        prompt.append(
            f"Sos el asistente publico de {org_name}. Estas hablando con un potencial alumno o cliente. "
            "Nunca reveles informacion interna, financiera ni de otros alumnos/clientes. "
            "Solo podes mostrar informacion publica: docentes, catalogo de servicios, "
            "disponibilidad y agendar turnos."
        )
        if teacher_name:
            prompt.append(f"Docente principal: {teacher_name}.")
        if specialties:
            prompt.append(f'Servicios disponibles en: {", ".join(specialties)}.')

    return "\n\n".join(prompt)
