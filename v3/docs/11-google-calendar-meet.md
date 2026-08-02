# Google Calendar y Meet

<!-- drift:bind v3/backend/internal/calendars/worker.go -->
<!-- drift:bind v3/backend/internal/calendars/repository.go -->
<!-- drift:bind v3/backend/internal/calendars/worker/helpers/events.go -->
<!-- drift:bind v3/backend/internal/scheduling/calendar_projection/helpers/events.go -->

## Alcance

Pymes proyecta turnos individuales confirmados y sus estados posteriores hacia
un calendario secundario llamado `Pymes`. Google no es fuente de verdad: una
caída, revocación o inconsistencia del proveedor no modifica ni bloquea la
reserva local. El MVP es unidireccional Pymes → Google; `watch`, `syncToken`,
Outlook y Teams quedan fuera.

La variable global `PYMES_GOOGLE_CALENDAR_ENABLED` debe configurar el workload
y la organización debe tener `google_calendar_enabled=true`. Rutas, proyección
y reconciliación fallan cerradas por tenant; desactivar el flag no borra la
conexión ni revoca tokens automáticamente.

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
path y la web nunca recibe ni reenvía el código. Web y BFF deben publicarse
bajo el mismo origen mediante routing por path para que Clerk envíe su cookie
`__session` durante la navegación de retorno; las llamadas API cross-origin
continúan usando `Authorization: Bearer`. El BFF valida criptográficamente
ambos transportes y nunca usa una cookie si existe un header Authorization
malformado. El `state` contiene una pista base64url sólo para seleccionar el
contexto RLS: el BFF autentica la misma sesión Clerk y valida después el hash
completo contra organización, actor, sesión, vencimiento y consumo único antes
de intercambiar el código.

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

- Agenda produce `CalendarSyncRequested` dentro de la transacción del turno.
  Calendars consume ese snapshot; no consulta tablas de Agenda para
  reconstruirlo.
- El snapshot canónico versión `1` contiene booking, operación y
  `source_version`; un `upsert` agrega summary, intervalo UTC RFC3339Nano, zona
  IANA y el opt-in de Meet. Un `delete` no acepta campos del evento.
- El productor calcula `snapshot_digest` como SHA-256 hexadecimal del snapshot.
  El consumidor rechaza campos desconocidos, digests no hexadecimales y
  cualquier digest que no coincida con el contenido reconstruido.
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

El estado resultante determina el comando: `confirmed`, `checked_in`,
`completed` y `no_show` hacen `upsert`; `cancelled` y `rescheduled` hacen
`delete`; `held` y `pending_confirmation` no se proyectan. Reprogramar elimina
el turno original con la versión incrementada y proyecta el reemplazo sólo si
su estado lo requiere. Los turnos de sesiones grupales quedan fuera: no existe
un evento Google por participante.

Una versión menor que la ya persistida es no-op y la misma versión con otro
digest es conflicto. La misma versión y digest sólo es no-op cuando el mapping
ya está sincronizado o eliminado; si quedó incierto o en reconciliación,
continúa el intento. Si no existe mapping, `delete` guarda un tombstone local
sin llamar a Google; un `404` del proveedor también converge a eliminado. El
payload del delete no conserva summary, descripción, ubicación, horarios, zona,
attendees ni el opt-in de Meet.

El relay toma exclusivamente `CalendarSyncRequested`; no puede arrendar eventos
de Commerce o Notifications. Los reintentos usan el outbox durable y conservan
la misma identidad lógica. Si la feature tenant está deshabilitada o todavía no
existe una conexión activa, el relay difiere el comando cinco minutos, libera
el lease y revierte el incremento de intentos; no lo publica, reintenta ni manda
a DLQ como si hubiera fallado Google.

La primera release de Pymes v3 es greenfield: la migración
`019_scheduling_calendar_projection.sql` debe ejecutarse antes de iniciar API o
worker, por lo que no existen turnos históricos que requieran backfill. Después
de esa migración, incluso los turnos creados con Google deshabilitado quedan
retenidos por el mecanismo anterior. Una futura importación de turnos desde v2
deberá incluir su propio backfill y continúa fuera del alcance de este plan.

## Política de Meet

`meet_requested` es opt-in: omitirlo al crear una reserva equivale a `false`.
Agenda lo congela en el turno; no forma parte del `PATCH` operativo, se conserva
al reprogramar y pasa de waitlist al turno aceptado. Para sesiones grupales se
rechaza `true` y no se proyecta un evento individual.

Calendars solicita la conferencia sólo cuando el snapshot inmutable vale
`true` y la conexión tiene Meet habilitado. El request ID determinístico permite
reintentar sin crear otra reunión. El estado pendiente se reconcilia por
polling; un fallo de conferencia no cambia el estado local del turno ni elimina
el evento Calendar.

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

## Validación real protegida

`.github/workflows/v3-google-live.yml` es manual, está fijado al environment
STG y sólo acepta `main`, CI verde para el SHA exacto, confirmación
`VALIDATE_GOOGLE_STG` y primer intento. Antes de leer credenciales ejecuta la
auditoría completa de protección GitHub.

El operador debe haber conectado una cuenta controlada y confirmado un turno.
El job consulta esa conexión y booking con una sesión Clerk corta, deriva el ID
base32hex y lee el evento mediante un access token de verificación separado.
Comprueba marker privado, digest, intervalo, ETag y, cuando se solicita,
`hangoutsMeet` exitoso. No crea, actualiza ni elimina recursos Google, no extrae
el grant cifrado de Pymes y no conserva payloads. Los tres valores sensibles
del piloto son secrets temporales de STG y se eliminan después del run.

`make protected-live-validation-test` prueba el job con transporte falso,
incluidos fallos de proveedor y no filtración de credenciales, sin llamar a
Google. Un run real verde continúa pendiente hasta ejecutar el piloto H8.
