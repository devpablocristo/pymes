# Notificaciones WhatsApp mediante PerGo

> Este documento describe el baseline PerGo vigente durante la transición. ADR
> 0014 lo reemplaza por una instancia Foundation Communications sin cambiar el
> ownership comercial de Pymes Notifications.

Pymes conserva la intención comercial de cada notificación y PerGo se encarga
del transporte. La integración no forma parte de la transacción que crea o
modifica el turno: esa transacción persiste la intención y
`NotificationRequested` en el outbox; el worker entrega después.

## Ownership

| Dato o decisión | Responsable |
|---|---|
| organización, destinatario, tipo de mensaje y momento | Pymes |
| template, versión, variables y estado comercial | Pymes |
| canal lógico e identidad no secreta del remitente por organización | Pymes |
| API key, proveedor WhatsApp y credenciales del canal | PerGo |
| cola del proveedor, external message ID y entrega | PerGo |
| estado proyectado `queued/sent/delivered/read/failed` | Pymes |

El navegador sólo puede consultar la proyección redactada mediante
`GET /api/v1/organizations/{organizationId}/notifications/{notificationId}`.
No recibe teléfono, cuerpo ni variables y nunca llama directamente a PerGo.

## Flujo durable

1. Scheduling agrega `NotificationRequested` a su outbox dentro de la
   transacción del turno.
2. El consumidor de Notifications proyecta el snapshot recibido, resuelve la
   ruta no secreta de la organización y comprueba
   `organization_feature_flags.whatsapp_enabled` antes de persistir
   idempotentemente una intención propia. No accede al repository de Scheduling
   ni crea un segundo outbox. Los casos de uso que originan directamente una
   notificación conservan la variante transaccional que persiste intención y
   evento juntos.
3. El dispatcher de Notifications toma un lease exclusivamente sobre
   `NotificationRequested`. Commerce, Scheduling y Calendars tienen
   dispatchers y allowlists independientes, por lo que ningún contexto puede
   robar o publicar el evento de otro.
4. El adapter llama `POST /api/v1/messages` con dos credenciales distintas:
   `Authorization` lleva la API key del workspace y
   `X-Serverless-Authorization` lleva un ID token OIDC emitido para el origen
   exacto configurado en `PYMES_PERGO_AUDIENCE`.
5. `X-Trace-ID` contiene, codificados en base64url, organización e ID de
   notificación: `pymes.v1.<organization>.<notification>`.
6. El ledger de ingreso de PerGo devuelve `202 queued`; la entrega al proveedor
   ocurre después mediante su worker.
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

PerGo usa un ledger PostgreSQL durable por
`(workspace_id, Idempotency-Key)`. Pymes reenvía la misma clave y el mismo
payload; una repetición aceptada obtiene exactamente el mismo `message_id`,
`queued_at` y trace ID sin volver a publicar en la cola. Reutilizar la clave con
otro payload devuelve el conflicto estable `idempotency_key_reused`.
`X-Trace-ID` queda como correlación tenant-aware y no sustituye al ledger.
Como las claves del dominio Pymes son tenant-locales y varias organizaciones
comparten el workspace, el adapter deriva el header mediante SHA-256 de
`organization_id + idempotency_key`; así dos tenants no colisionan y ningún
identificador sensible aparece en la clave externa.

Un timeout puede haber ocurrido antes o después de que PerGo aceptara el
mensaje. Pymes marca la intención `uncertain`, conserva el mismo evento del
outbox y reintenta con las mismas identidades. El ledger cubre respuesta perdida,
reinicio y concurrencia en el ingreso a PerGo. Si el webhook llegó primero y
marcó `sent`, una respuesta tardía o perdida tampoco degrada el estado.

La entrega externa también conserva un claim PostgreSQL con lease, generación y
fencing. PerGo confirma `sending` en la base antes de llamar al proveedor y no
mantiene una transacción abierta durante esa llamada. Si se pierde una respuesta
de transporte o vence un claim que ya había llegado a `sending`, PerGo marca el
dispatch interno `uncertain`, lo ACKea sin fallback ni reenvío automático y
publica el evento estable `failed` con `error=DELIVERY_UNCERTAIN`. Pymes persiste
exclusivamente `PERGO_DELIVERY_UNCERTAIN`; cualquier texto libre del proveedor
se reduce a `PERGO_DELIVERY_FAILED`, evitando PII o secretos.

Esta política prioriza no enviar dos WhatsApp cuando el proveedor no ofrece una
clave idempotente verificable. El resultado ambiguo requiere reconciliación u
operación explícita; nunca se presenta como certeza de no entrega ni se
redispara automáticamente. No se documenta como exactly-once de punta a punta.

Los cambios de estado son monotónicos:

```text
pending/uncertain → queued → sent → delivered → read
        └──────────────────────────────────────→ failed
```

Un webhook repetido conserva una sola fila por
`(org_id, payload_hash)`. Eventos atrasados quedan auditados, pero no degradan
un estado más avanzado.

El vocabulario contractual de callbacks es exactamente
`queued/sent/delivered/read/failed`. `sending` y `uncertain` son estados internos
de PerGo y no atraviesan el webhook.

## Persistencia y aislamiento

La migración `016_notifications_pergo.sql` crea:

- `notification_settings`;
- `notifications`;
- `notification_webhook_inbox`.

`organization_feature_flags`, creada por
`018_organization_feature_flags.sql`, es la única fuente de verdad para
habilitar WhatsApp. `notification_settings` conserva exclusivamente la ruta no
secreta hacia PerGo (`pergo_channel` y `pergo_sender_identity`); su antiguo
booleano ya no gobierna el rollout. Las tablas tenant tienen `org_id`, RLS
habilitada y forzada. Las identidades durables son compuestas por tenant y
`whatsapp_enabled` falla cerrado. El inbox no admite `UPDATE`, `DELETE` ni
`TRUNCATE`.

`017_notifications_pergo_routes.sql` agrega:

- `pergo_channel` y `pergo_sender_identity` a la configuración tenant;
- el snapshot inmutable `delivery_channel` y `sender_identity` a cada intención.

## Routing por organización

`pergo_sender_identity` identifica una conexión o remitente que ya existe en
PerGo; no contiene token, API key ni secreto del proveedor. La resolución ocurre
antes de calcular el digest de la intención, por lo que un cambio posterior de
configuración sólo afecta mensajes nuevos. El adapter envía ese valor en
`from`.

Pymes no almacena credenciales de canales PerGo en su base. La única API key que
usa el worker es una credencial técnica del workload, inyectada desde Secret
Manager; las conexiones, tokens y secretos WhatsApp permanecen exclusivamente
en PerGo.

La ausencia de ruta falla cerrada con `PERGO_ROUTE_NOT_CONFIGURED`. Existe un
fallback global exclusivamente para el fake de Compose o un piloto controlado,
y requiere habilitarlo de forma explícita. No debe utilizarse como modelo SaaS.

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
PYMES_PERGO_AUDIENCE=<origen HTTPS exacto del servicio privado>
PERGO_API_KEY=<Secret Manager>
PERGO_WORKSPACE_ID=<workspace esperado>
PERGO_CHANNEL=whatsapp_cloud
PERGO_ALLOW_GLOBAL_ROUTE_FALLBACK=false
PERGO_TIMEOUT=5s
```

`PERGO_API_KEY` y los secretos de webhook son credenciales distintas. En STG y
PRD deben existir como versiones separadas de Secret Manager y estar accesibles
únicamente por las service accounts de worker y API, respectivamente.
`PYMES_PERGO_AUDIENCE` es la audiencia del ID token de plataforma y puede ser
distinta de `PERGO_URL`; en producción debe ser un origen HTTPS sin path, query,
fragmento, credenciales ni barra final. El servicio privado de PerGo concede
`roles/run.invoker` únicamente al worker Pymes del mismo ambiente.
El workflow de release la recibe exclusivamente desde la variable protegida
`PYMES_PERGO_AUDIENCE` del environment GitHub seleccionado; el gate
estructurado del workflow falla si esa propagación se elimina.
`PERGO_CHANNEL` sólo se consulta cuando el fallback explícito está habilitado;
la ruta normal proviene del snapshot tenant.

## Validación y recuperación

`make notifications-e2e` usa el fake contractual de Compose y cubre:

- envío exitoso;
- proyección consumer-owned de Scheduling y routing tenant hasta `from`;
- replay del fake con receipt estable y conflicto ante payload distinto;
- timeout después de procesar sin segundo mensaje;
- indisponibilidad y recuperación;
- timeout antes de procesar;
- webhook duplicado;
- firma inválida.

La suite del fork PerGo agrega concurrencia entre workers, fencing de claims,
caída después de declarar `sending` y respuesta de transporte ambigua. En esos
casos demuestra que no existe una segunda llamada al proveedor ni un fallback y
que el único callback público es `failed/DELIVERY_UNCERTAIN`.

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
