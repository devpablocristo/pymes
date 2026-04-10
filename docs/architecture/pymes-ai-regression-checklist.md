# Checklist de regresión — Asistente Pymes AI

Escenarios obligatorios antes de cada release que toque `ai/`, `frontend/` (chat/notificaciones), o contratos HTTP del asistente.

---

## Escenarios automatizados (pytest)

Correr desde `ai/`:

```bash
python -m pytest tests/ -v --tb=short
```

Cubren: routing pipeline, handoff parsing, evidencia, contratos SSE, insights router, auth middleware.

---

## Escenarios manuales (smoke con stack levantado)

Levantar con `make up` desde la raíz del monorepo.

### 1. Chat libre sin hint

- Abrir `/chat` en la consola.
- Escribir: "hola, como estas?"
- **Esperado:** respuesta del asesor general (`routed_agent: general`).
- **Log:** `internal_turn_routing_decision` con `routing_reason: no_deterministic_match`.

### 2. Hint de dominio (sales)

- Escribir: "resumime las ventas del mes"
- **Esperado:** respuesta con datos de ventas (`routed_agent: sales`).
- **Log:** `routing_reason: explicit_route_hint` o post-routing hint applied.

### 3. Notificación insight → handoff → respuesta con bloques

- Ir a `/notifications`.
- Generar insights (botón "Generar insights").
- Click en "Explicar en chat" en una notificación de ventas.
- **Esperado:** el chat abre con mensaje del usuario + respuesta con bloques (insight_card, kpi_group, table).
- **Log:** `handoff_resolved` con `handoff_scope: sales_collections`.
- **Response:** `routed_agent: insight_chat`, `routing_source: ui_hint`.

### 4. Segundo turno en hilo insight (follow-up anclado)

- En el mismo hilo del escenario 3, escribir: "qué implica eso para el negocio?"
- **Esperado:** respuesta que referencia números del insight previo (no genéricos).
- **Log:** `insight_evidence_injected` con `scope: sales_collections`.

### 5. Handoff inválido → fallback

- (curl) Enviar POST a `/v1/chat` con `handoff.notification_id` inexistente.
- **Esperado:** fallback a legacy insight_chat o orchestrator. No 500.
- **Log:** `handoff_failed` con `reason: notification_not_found` o similar.

### 6. Conversación sin evidencia → sin inyección extra

- Chat nuevo, escribir algo operativo: "creá un cliente con nombre Test"
- **Esperado:** routing a orchestrator → tools de creación. Sin `insight_evidence_injected` en logs.

---

## Verificación post-release

1. Buscar en logs: `internal_turn_summary` con filtro `org_id` del tenant de prueba.
2. Verificar que `routing_reason`, `handler_kind`, `has_handoff`, `evidence_injected` estén presentes.
3. Confirmar que no hay errores 500 en los 7 escenarios.
