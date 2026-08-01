# Google Calendar y Meet

## Alcance

Pymes proyecta turnos confirmados hacia un calendario secundario llamado
`Pymes`. Google no es fuente de verdad: una caída, revocación o inconsistencia
del proveedor no modifica ni bloquea la reserva local. El MVP es unidireccional
Pymes → Google; `watch`, `syncToken`, Outlook y Teams quedan fuera.

El contexto `internal/calendars` es dueño de OAuth, conexiones tenant, mappings,
outbox, idempotencia y reconciliación. El SDK publicado
`github.com/devpablocristo/platform/sdks/google-calendar/go` es un cliente
técnico de bajo nivel y queda detrás de `google_calendar.go`; ningún tipo del
SDK ingresa al dominio, handler o repositorio.

## Modelo y aislamiento

- `calendar_connections`: configuración y grant cifrado por organización.
- `calendar_oauth_states`: hash SHA-256 del estado OAuth, actor, sesión,
  organización, propósito y vencimiento de diez minutos.
- `external_calendar_events`: proyección por conexión y turno, ETag, estado de
  Meet, versión de fuente y digest del snapshot.
- `calendar_sync_attempts`: auditoría de cada intento sin tokens ni payload
  privado.

Todas las claves primarias y foráneas tenant incluyen `org_id`; las cuatro
tablas tienen RLS forzado. El repositorio abre una transacción, fija
`app.org_id` localmente y nunca acepta el tenant desde un payload externo.

## OAuth

1. Un owner/admin autenticado solicita la conexión.
2. Pymes genera 32 bytes aleatorios y persiste únicamente su hash.
3. El estado queda ligado a organización, actor y `sid` verificado por Clerk.
4. La web redirige al URL devuelto por el BFF.
5. La web entrega `code` y `state` al BFF con la misma sesión Clerk.
6. Pymes consume el estado una sola vez, intercambia el código y cifra el
   grant antes de crear el calendario secundario.
7. Si se pierde la respuesta de creación, Pymes busca el marker único
   `pymes-connection:<connection_id>` en CalendarList antes de cualquier nuevo
   intento.

Scopes mínimos:

- `calendar.app.created`, para los calendarios y eventos creados por Pymes;
- `calendar.calendarlist.readonly`, para reconciliar el calendario secundario;
- `calendar.freebusy`, sólo cuando la organización habilita FreeBusy.

El callback termina directamente en el BFF mediante
`GET /api/v1/calendars/google/oauth/callback`; no incluye organización en el
path y la web nunca recibe ni reenvía el código. El `state` contiene una pista
base64url sólo para seleccionar el contexto RLS: el BFF autentica la sesión
Clerk y valida después el hash completo contra organización, actor, sesión,
vencimiento y consumo único antes de intercambiar el código.

## Cifrado

Cada grant se serializa y cifra con envelope encryption:

1. se genera un DEK AES-256 aleatorio;
2. el grant se cifra con AES-GCM;
3. el DEK se envuelve mediante Cloud KMS;
4. ambos pasos usan como AAD
   `pymes-v3/calendars/<org_id>/connections/<connection_id>`;
5. se verifican los CRC32C requeridos por Cloud KMS;
6. sólo el envelope se persiste.

STG y PRD usan claves KMS distintas. La clave local de 32 bytes existe
exclusivamente para desarrollo y tests; producción la rechaza al arrancar.

## Proyección idempotente

- El ID de evento es base32hex determinístico a partir de organización,
  conexión y turno.
- El `requestId` de Meet se deriva por separado.
- El digest del snapshot se guarda en una propiedad privada Google.
- Un create con `409` o timeout se resuelve con `events.get` exacto.
- Updates y deletes envían `If-Match`; un `412` refresca el ETag y reintenta.
- Un ETag existente nunca se borra al persistir un fallo transitorio.
- Una versión de fuente antigua no reemplaza una proyección nueva.
- Meet pendiente se reconcilia por polling; un fallo de Meet se registra sin
  invalidar el turno ni el evento.
- Un refresh token revocado cambia la conexión a `reauth_required`.

El relay toma exclusivamente `CalendarSyncRequested`; no puede arrendar eventos
de Commerce o Notifications. Los reintentos usan el outbox durable y conservan
la misma identidad lógica.

## Configuración

| Variable | Uso |
|---|---|
| `PYMES_GOOGLE_CALENDAR_ENABLED` | Habilita rutas y worker Calendar |
| `PYMES_GOOGLE_CLIENT_ID` | OAuth client del entorno |
| `PYMES_GOOGLE_CLIENT_SECRET` | Secret Manager → variable de workload |
| `PYMES_GOOGLE_REDIRECT_URL` | `${API_ORIGIN}/api/v1/calendars/google/oauth/callback` |
| `PYMES_CALENDAR_KMS_KEY` | CryptoKey de STG o PRD |
| `PYMES_CALENDAR_LOCAL_KEY` | Base64 de 32 bytes, sólo local/test |
| `PYMES_GOOGLE_AUTH_URL` | Override contractual, no producción |
| `PYMES_GOOGLE_TOKEN_URL` | Override contractual, no producción |
| `PYMES_GOOGLE_REVOKE_URL` | Override contractual, no producción |
| `PYMES_GOOGLE_CALENDAR_URL` | Override contractual, no producción |

Al habilitar Google, desarrollo exige exactamente KMS o clave local; producción
exige KMS, HTTPS y endpoints oficiales. La configuración parcial aborta el
arranque.

## Operación y recuperación

- `pending`: el grant existe, pero falta reconciliar el calendario secundario.
- `active`: puede recibir proyecciones.
- `reauth_required`: el usuario debe repetir OAuth.
- `revoked`: conexión desconectada localmente.

Para una incidencia:

1. verificar backlog `CalendarSyncRequested` y estado de conexión;
2. no crear eventos manualmente ni cambiar IDs;
3. restaurar el proveedor o completar reautorización;
4. ejecutar el reconciliador;
5. comprobar que cada turno tenga como máximo una proyección por conexión;
6. revisar `calendar_sync_attempts` por código estable, nunca por contenido
   sensible.

Los jobs contra Google real son protegidos y separados del CI determinístico.
