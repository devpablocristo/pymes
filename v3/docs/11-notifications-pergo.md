# Notificaciones WhatsApp mediante PerGo

Pymes conserva la intención comercial de cada notificación y PerGo se encarga
del transporte. La integración no forma parte de la transacción que crea o
modifica el turno: esa transacción persiste la intención y
`NotificationRequested` en el outbox; el worker entrega después.

## Ownership

| Dato o decisión | Responsable |
|---|---|
| organización, destinatario, tipo de mensaje y momento | Pymes |
| template, versión, variables y estado comercial | Pymes |
| API key, proveedor WhatsApp y credenciales del canal | PerGo |
| cola del proveedor, external message ID y entrega | PerGo |
| estado proyectado `queued/sent/delivered/read/failed` | Pymes |

El navegador sólo puede consultar la proyección redactada mediante
`GET /api/v1/organizations/{organizationId}/notifications/{notificationId}`.
No recibe teléfono, cuerpo ni variables y nunca llama directamente a PerGo.

## Flujo durable

1. El caso de uso de Scheduling crea una intención con identidad e idempotency
   key estables.
2. `notifications.Postgres.Create` comprueba el feature flag tenant,
   persiste `app.notifications` y agrega `NotificationRequested` al outbox en
   una sola transacción.
3. El dispatcher de Notifications toma un lease exclusivamente sobre
   `NotificationRequested`. Commerce, Scheduling y Calendars tienen
   dispatchers y allowlists independientes, por lo que ningún contexto puede
   robar o publicar el evento de otro.
4. El adapter llama `POST /api/v1/messages` con la API key de workspace.
5. `X-Trace-ID` contiene, codificados en base64url, organización e ID de
   notificación: `pymes.v1.<organization>.<notification>`.
6. PerGo devuelve `202 queued` o un resultado incierto/reintentable/terminal.
7. PerGo publica el estado al callback global
`POST /api/v1/webhooks/pergo`.
8. El BFF verifica firma y timestamp antes de extraer el tenant del trace ID;
   comprueba además el workspace esperado y aplica el evento mediante un inbox
   inmutable.

El trace ID permite usar un único webhook de workspace sin aceptar el tenant
desde una URL pública. La firma cubre el cuerpo crudo y la ventana máxima es de
cinco minutos. `PERGO_WEBHOOK_SECRETS` acepta la clave activa y claves anteriores
separadas por coma para rotación sin corte.

Para `whatsapp_cloud`, Pymes envía los parámetros textuales del body ordenados
por la clave estable de la variable. El catálogo de templates debe fijar ese
orden —se recomiendan prefijos `01_`, `02_`, etc.— y una nueva disposición exige
incrementar la versión. Los modelos `components` siguen siendo privados del
adapter; no ingresan al dominio.

## Idempotencia y pérdida de respuesta

PerGo usa `X-Trace-ID` como identidad de despacho. Pymes también envía
`Idempotency-Key`, correlation ID, versión de template, tipo y organización
dentro de metadata. Una repetición idéntica obtiene el mismo mensaje; una
repetición con payload distinto es `PERGO_IDEMPOTENCY_CONFLICT`.

Un timeout puede haber ocurrido antes o después de que PerGo aceptara el
mensaje. Pymes marca la intención `uncertain`, conserva el mismo evento del
outbox y reintenta con las mismas identidades. Si el webhook llegó primero y
marcó `sent`, una respuesta tardía o perdida no degrada el estado y el siguiente
reintento no vuelve a enviar.

Los cambios de estado son monotónicos:

```text
pending/uncertain → queued → sent → delivered → read
        └──────────────────────────────────────→ failed
```

Un webhook repetido conserva una sola fila por
`(org_id, payload_hash)`. Eventos atrasados quedan auditados, pero no degradan
un estado más avanzado.

## Persistencia y aislamiento

La migración `015_notifications_pergo.sql` crea:

- `notification_settings`;
- `notifications`;
- `notification_webhook_inbox`.

Las tres tablas tienen `org_id`, RLS habilitada y forzada. Las identidades
durables son compuestas por tenant y la configuración `whatsapp_enabled`
falla cerrada. El inbox no admite `UPDATE`, `DELETE` ni `TRUNCATE`.

Teléfono, cuerpo y variables son datos operativos privados: no se incluyen en
logs, métricas, errores ni respuestas públicas. Backups y acceso SQL deben
seguir las mismas reglas de PII que `parties`.

## Configuración

API:

```text
PYMES_PERGO_ENABLED=true
PERGO_WORKSPACE_ID=<workspace esperado>
PERGO_WEBHOOK_SECRETS=<secreto-activo[,secreto-anterior]>
```

Worker:

```text
PYMES_PERGO_ENABLED=true
PERGO_URL=<URL privada de PerGo>
PERGO_API_KEY=<Secret Manager>
PERGO_WORKSPACE_ID=<workspace esperado>
PERGO_CHANNEL=whatsapp
PERGO_TIMEOUT=5s
```

`PERGO_API_KEY` y los secretos de webhook son credenciales distintas. En STG y
PRD deben existir como versiones separadas de Secret Manager y estar accesibles
únicamente por las service accounts de worker y API, respectivamente.

## Validación y recuperación

`make notifications-e2e` usa el fake contractual de Compose y cubre:

- envío exitoso;
- timeout después de procesar sin segundo mensaje;
- indisponibilidad y recuperación;
- timeout antes de procesar;
- webhook duplicado;
- firma inválida.

`make db-integration` agrega aislamiento entre organizaciones, clave
idempotente reutilizada con otro payload, concurrencia e inbox. Los tests no
requieren PerGo real ni exponen credenciales.

Ante una caída:

1. confirmar API, worker, PostgreSQL y PerGo mediante sus probes;
2. observar backlog/reintentos agregados, sin inspeccionar cuerpo o teléfono;
3. recuperar PerGo o conectividad;
4. dejar que venza el lease y que el dispatcher de Notifications use la misma
   identidad;
5. verificar convergencia por notification ID y external message ID;
6. no crear otra intención ni editar manualmente el estado.
