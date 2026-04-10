# Runbook de incidentes — Servicio AI Pymes

## Owners

| Componente | Responsable |
|-----------|-------------|
| `ai/` (FastAPI) | Equipo AI / backend |
| `frontend/` (chat, notificaciones) | Equipo frontend |
| `pymes-core/backend/` (notificaciones, datos) | Equipo core |
| `core/ai/python/` (runtime, LLM client) | Equipo plataforma |

---

## Primeros pasos ante incidente

1. **Obtener `request_id`** del usuario o del frontend (visible en meta del mensaje, header `X-Request-ID`, body `request_id`).
2. **Buscar en logs** por `request_id`:
   ```
   request_id=req_<hex12>
   ```
3. **Verificar secuencia esperada:**
   - `chat_internal_started` → `internal_turn_routing_decision` → (handoff/insight/orchestrator) → `internal_turn_summary` → `chat_internal_completed`
4. Si falta `chat_internal_completed`, el turno falló antes de responder.

---

## Errores comunes

### 502 Bad Gateway

**Síntoma:** el frontend muestra error de red.

**Causas:**
- Backend core (`pymes-core`) caído → verificar `docker ps` / healthcheck de `cp-backend`.
- Timeout de LLM provider → buscar `timeout` en logs de `ai`.
- Proxy/nginx timeout → verificar config de compose.

**Mitigación:**
- `docker compose restart ai` si es el servicio AI.
- Verificar cuota del LLM provider.

### Handoff fallido (`handoff_failed` en logs)

**Síntoma:** usuario abre notificación en chat pero no ve bloques de insight.

**Causas:**
- `notification_not_found` → la notificación no se persistió en `in-app-notifications` del core.
- `scope_mismatch` → el `insight_scope` del handoff no coincide con el de la notificación persistida.
- `insight_resolution_failed` → `InsightsService` falló al computar el snapshot (backend core sin datos o error).

**Diagnóstico:**
```
buscar: handoff_failed request_id=<id>
ver campo: reason
```

### Timeout LLM

**Síntoma:** respuesta lenta o error genérico.

**Causa:** el provider (Gemini/OpenAI) no responde en el timeout configurado.

**Logs:** buscar `LLMError` o `timeout` en el turno.

**Mitigación:** el sistema tiene fallback a respuesta por defecto si el orchestrator falla. Si es recurrente, revisar límites en `_build_internal_general_limits()`.

### Cuota agotada

**Síntoma:** HTTP 429 o mensaje de cuota en el frontend.

**Causa:** el org superó el límite diario.

**Logs:** `check_quota` log con `quota_exceeded=True`.

**Mitigación:** ajustar plan del tenant o esperar reset diario.

### Evidencia no inyectada en follow-up

**Síntoma:** segundo turno en hilo de insight no referencia datos reales.

**Causas:**
- Evidencia expirada (>24h desde `computed_at`).
- Evidencia fuera de la ventana de 10 mensajes.
- Turn 1 no persistió `insight_evidence` (bug en Fase 3).

**Diagnóstico:**
```
buscar: insight_evidence_injected conversation_id=<id>
si no aparece: la evidencia no se encontró en el hilo
```

---

## Logs clave

| Evento | Cuándo aparece | Campos útiles |
|--------|---------------|---------------|
| `chat_internal_started` | Inicio del turno | `request_id`, `org_id`, `route_hint` |
| `internal_turn_routing_decision` | Post-routing | `handler_kind`, `routing_reason`, `routing_target` |
| `handoff_resolved` | Handoff exitoso | `handoff_scope`, `notification_id` |
| `handoff_failed` | Handoff fallido | `reason`, `handoff_scope` |
| `insight_evidence_injected` | Follow-up con evidencia | `scope`, `period` |
| `internal_turn_summary` | Fin del turno (resumen) | Todos los campos operativos |
| `chat_internal_completed` | Respuesta enviada | `routed_agent`, `tokens_*`, `tool_calls` |

---

## Cómo simular un turno

```bash
curl -X POST http://localhost:8082/v1/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <jwt>" \
  -d '{"message": "como vienen las ventas esta semana?"}'
```

Para probar handoff:
```bash
curl -X POST http://localhost:8082/v1/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <jwt>" \
  -d '{
    "message": "Quiero entender ventas de esta semana.",
    "handoff": {
      "source": "in_app_notification",
      "notification_id": "<id-real>",
      "insight_scope": "sales_collections",
      "period": "week",
      "compare": true,
      "top_limit": 5
    }
  }'
```
