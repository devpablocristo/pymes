from __future__ import annotations

from typing import Any

from runtime.memory import (
    build_operational_memory_view,
    capture_operational_turn,
    consolidate_operational_memory,
    ensure_operational_memory,
    normalize_memory_text,
)


_VERTICAL_LABELS: dict[str, str] = {
    "workshops": "taller",
    "workshop": "taller",
    "auto_repair": "taller mecánico",
    "bike_shop": "bicicletería",
    "beauty": "belleza y bienestar",
    "restaurants": "gastronomía",
    "restaurant": "gastronomía",
    "gastronomia": "gastronomía",
    "professionals": "servicios profesionales",
    "professional_services": "servicios profesionales",
    "retail": "comercio minorista",
    "commerce": "comercio minorista",
    "comercio_minorista": "comercio minorista",
    "distribuidora": "distribuidora",
    "freelancer": "servicios independientes",
}

_PROFILE_VERTICAL_HINTS: dict[str, str] = {
    "comercio_minorista": "comercio minorista",
    "servicio_profesional": "servicios profesionales",
    "gastronomia": "gastronomía",
    "distribuidora": "distribuidora",
    "freelancer": "servicios independientes",
}

_VERTICAL_PLAYBOOKS: dict[str, list[str]] = {
    "taller": [
        "Prioriza turnos, órdenes, repuestos críticos, tiempos de reparación y entrega.",
        "Conecta agenda, stock y ventas de servicios antes de sugerir decisiones.",
    ],
    "taller mecánico": [
        "Prioriza turnos, órdenes, repuestos críticos, tiempos de reparación y entrega.",
        "Conecta agenda, stock y ventas de servicios antes de sugerir decisiones.",
    ],
    "bicicletería": [
        "Mirá ventas de productos, servicios de taller, stock crítico y estacionalidad deportiva.",
        "Diferenciá claramente catálogo de productos versus trabajo de taller.",
    ],
    "belleza y bienestar": [
        "Mirá ocupación de agenda, recurrencia de clientes, profesionales, ticket promedio y servicios estrella.",
        "Si hay agenda activa, prioriza llenar huecos y reactivar clientes recurrentes.",
    ],
    "gastronomía": [
        "Mirá ticket promedio, rotación, horas pico, mix de productos y reposición rápida.",
        "Evita responder como catálogo cuando el pedido es de operación o rentabilidad.",
    ],
    "servicios profesionales": [
        "Mirá agenda, utilización de profesionales, recurrencia, servicios vendidos y cobranza pendiente.",
        "Cuando falte contexto, prioriza lectura operativa antes que listar servicios.",
    ],
    "comercio minorista": [
        "Mirá rotación de stock, productos con precio, productos sin precio y oportunidades de reposición.",
        "Si el usuario pide vender más, prioriza ticket promedio, stock vendible y clientes recurrentes.",
    ],
    "distribuidora": [
        "Mirá abastecimiento, cuentas corrientes, stock, reposición y ventas por cliente.",
        "Priorizá márgenes, quiebres y cobranzas antes que catálogo.",
    ],
    "servicios independientes": [
        "Mirá pipeline comercial, presupuesto, agenda, recurrencia y cobranzas.",
        "Priorizá foco comercial y capacidad disponible.",
    ],
}

_SOFTWARE_PLAYBOOK = [
    "Pymes es una plataforma de gestión: módulos activos y datos cargados definen qué parte del negocio está visible.",
    "Ante pedidos ejecutivos como 'cómo viene el negocio', 'qué harías hoy' o 'cómo vender más', priorizá análisis, riesgos, prioridades y acciones antes que listar registros.",
    "Solo derivá a un especialista cuando la consulta baja claramente a un dominio operativo como ventas, cobros, compras, clientes, productos o servicios.",
    "No confundas una categoría elegida o un módulo activo con una orden de mostrar catálogo.",
]

_BUSINESS_MEMORY_CUES: tuple[str, ...] = (
    "recordá que",
    "recorda que",
    "tené en cuenta",
    "tene en cuenta",
    "mi negocio",
    "nuestro negocio",
    "somos",
    "trabajamos",
    "vendemos",
    "atendemos",
    "siempre",
    "nunca",
    "preferimos",
)

_USER_PREFERENCE_RULES: tuple[tuple[str, str], ...] = (
    ("breve", "El usuario prefiere respuestas breves."),
    ("corto", "El usuario prefiere respuestas cortas."),
    ("sin tabla", "El usuario prefiere respuestas sin tablas."),
    ("con tabla", "El usuario prefiere respuestas con tablas cuando agregan valor."),
    ("paso a paso", "El usuario prefiere explicaciones paso a paso."),
    ("priorizá", "El usuario suele pedir priorización explícita."),
    ("prioriza", "El usuario suele pedir priorización explícita."),
)


def _compact_text(value: Any) -> str:
    return str(value or "").strip()

def _extract_business_memory_candidate(user_message: str) -> str:
    text = normalize_memory_text(user_message)
    lowered = text.lower()
    if len(text) < 12:
        return ""
    if any(cue in lowered for cue in _BUSINESS_MEMORY_CUES):
        return text
    return ""


def _extract_user_preferences(user_message: str) -> list[str]:
    lowered = normalize_memory_text(user_message).lower()
    preferences: list[str] = []
    for needle, preference in _USER_PREFERENCE_RULES:
        if needle in lowered and preference not in preferences:
            preferences.append(preference)
    return preferences


def consolidate_memory(dossier: dict[str, Any]) -> dict[str, Any]:
    return consolidate_operational_memory(dossier)


def capture_turn_memory(
    dossier: dict[str, Any],
    *,
    user_id: str | None,
    user_message: str,
    assistant_reply: str,
    routed_agent: str,
    tool_calls: list[str] | None = None,
    pending_confirmations: list[str] | None = None,
    confirmed_actions: set[str] | None = None,
) -> dict[str, Any]:
    business_facts: list[str] = []
    normalized_user_message = normalize_memory_text(user_message)
    if business_fact := _extract_business_memory_candidate(normalized_user_message):
        business_facts.append(business_fact)
        add_learned_context(dossier, business_fact)

    capture_operational_turn(
        dossier,
        user_id=user_id,
        routed_agent=routed_agent,
        user_message=user_message,
        assistant_reply=assistant_reply,
        tool_calls=tool_calls,
        pending_confirmations=pending_confirmations,
        confirmed_actions=confirmed_actions,
        business_facts=business_facts,
        user_preferences=_extract_user_preferences(normalized_user_message),
    )
    return dossier


def infer_business_vertical(dossier: dict[str, Any]) -> str:
    business = dossier.get("business", {}) if isinstance(dossier, dict) else {}
    explicit_vertical = _compact_text(business.get("vertical")).lower()
    if explicit_vertical:
        return _VERTICAL_LABELS.get(explicit_vertical, explicit_vertical.replace("_", " "))

    profile = _compact_text(business.get("profile")).lower()
    if profile:
        return _PROFILE_VERTICAL_HINTS.get(profile, profile.replace("_", " "))

    modules = {
        _compact_text(module).lower()
        for module in dossier.get("modules_active", [])
        if _compact_text(module)
    }
    if "scheduling" in modules and "products" in modules:
        return "taller"
    if "scheduling" in modules and "services" in modules:
        return "servicios profesionales"
    if "inventory" in modules and "purchases" in modules:
        return "distribuidora"
    if "inventory" in modules and "sales" in modules:
        return "comercio minorista"
    return ""


def build_operating_context_for_prompt(dossier: dict[str, Any], user_id: str | None = None) -> str:
    business = dossier.get("business", {}) if isinstance(dossier, dict) else {}
    consolidate_memory(dossier)
    vertical = infer_business_vertical(dossier)
    ensure_operational_memory(dossier)
    memory_view = build_operational_memory_view(dossier, user_id)
    modules = [
        _compact_text(module)
        for module in dossier.get("modules_active", [])
        if _compact_text(module)
    ]
    software_lines = list(_SOFTWARE_PLAYBOOK)
    vertical_lines = _VERTICAL_PLAYBOOKS.get(vertical, [])

    context: list[str] = []
    if vertical:
        context.append(f"Vertical principal: {vertical}.")
    if business_name := _compact_text(business.get("name")):
        context.append(f"Negocio: {business_name}.")
    if modules:
        context.append(f"Módulos activos: {', '.join(modules)}.")
    if business_type := _compact_text(business.get("type")):
        context.append(f"Tipo de negocio declarado: {business_type}.")
    if profile := _compact_text(business.get("profile")):
        context.append(f"Perfil operativo: {profile}.")
    if description := _compact_text(business.get("description")):
        context.append(f"Descripción conocida: {description}.")

    if software_lines:
        context.append("Cómo funciona Pymes:")
        context.extend(f"- {line}" for line in software_lines)
    if vertical_lines:
        context.append("Cómo pensar esta vertical:")
        context.extend(f"- {line}" for line in vertical_lines)
    business_facts = memory_view["stable_business_facts"][-4:]
    business_facts = [item for item in business_facts if item]
    if business_facts:
        context.append("Memoria del negocio:")
        context.extend(f"- {item}" for item in business_facts)
    open_loops = memory_view["open_loops"][-3:]
    if open_loops:
        context.append("Temas abiertos recientes:")
        context.extend(f"- {detail}" for detail in open_loops if detail)
    decisions = memory_view["decisions"][-3:]
    if decisions:
        context.append("Decisiones recientes:")
        context.extend(f"- {detail}" for detail in decisions if detail)
    if user_id:
        preferences = memory_view["active_preferences"][-4:]
        preferences = [item for item in preferences if item]
        if preferences:
            context.append("Memoria del usuario interno:")
            context.extend(f"- {item}" for item in preferences)
        recent_topics = memory_view["recent_topics"][-3:]
        recent_topics = [item for item in recent_topics if item]
        if recent_topics:
            context.append("Temas recientes del usuario:")
            context.extend(f"- {item}" for item in recent_topics)

    return "\n".join(context).strip()


def summarize_dossier_for_context(dossier: dict[str, Any]) -> str:
    business = dossier.get("business", {})
    onboarding = dossier.get("onboarding", {})
    modules = dossier.get("modules_active", [])
    learned = dossier.get("learned_context", [])
    memory = ensure_operational_memory(dossier)
    consolidate_memory(dossier)
    vertical = infer_business_vertical(dossier)
    business_summary = {
        "name": business.get("name", ""),
        "type": business.get("type", ""),
        "profile": business.get("profile", ""),
        "vertical": vertical,
        "currency": business.get("currency", ""),
        "tax_rate": business.get("tax_rate"),
    }
    return (
        f"business={business_summary}; onboarding={onboarding}; modules_active={modules}; "
        f"team={dossier.get('team', [])}; learned_context={learned[-5:]}; "
        f"memory={{business_facts:{len(memory.get('business_facts', []))}, stable_business_facts:{len(memory.get('stable_business_facts', []))}, open_loops:{len(memory.get('open_loops', []))}, decisions:{len(memory.get('decisions', []))}, user_profiles:{len(memory.get('user_profiles', {}))}}}"
    )


def update_business_field(dossier: dict[str, Any], key: str, value: Any) -> dict[str, Any]:
    business = dossier.setdefault("business", {})
    business[key] = value
    return dossier


def add_learned_context(dossier: dict[str, Any], fact: str) -> dict[str, Any]:
    learned = dossier.setdefault("learned_context", [])
    if fact not in learned:
        learned.append(fact)
    if len(learned) > 100:
        dossier["learned_context"] = learned[-100:]
    return dossier


def set_modules(dossier: dict[str, Any], active: list[str]) -> dict[str, Any]:
    dossier["modules_active"] = list(active)
    return dossier


def set_preference(dossier: dict[str, Any], key: str, value: Any) -> dict[str, Any]:
    prefs = dossier.setdefault("preferences", {})
    prefs[key] = value
    return dossier


def sync_business_from_settings(dossier: dict[str, Any], settings: dict[str, Any]) -> dict[str, Any]:
    business = dossier.setdefault("business", {})
    mapping = {
        "business_name": "name",
        "business_tax_id": "tax_id",
        "business_address": "address",
        "business_phone": "phone",
        "default_currency": "currency",
        "default_tax_rate": "tax_rate",
        "business_vertical": "vertical",
        "vertical": "vertical",
        "business_type": "type",
        "business_description": "description",
    }
    for settings_key, dossier_key in mapping.items():
        val = settings.get(settings_key)
        if val is not None and val != "":
            business[dossier_key] = val
    return dossier
