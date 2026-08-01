# Runbook operativo de Pymes v3

Este runbook cubre STG y PRD en el proyecto compartido
`pymes-dev-352318`. Fiscal sigue siendo un mock: las reglas de reconciliación
ya preservan numeración e idempotencia, pero no se cargan certificados ni se
habilita emisión ARCA hasta su etapa específica.

## Invariantes que nunca se rompen

- Una respuesta fiscal incierta se resuelve consultando el mismo punto de
  venta, tipo y número reservado; nunca se emite otro comprobante.
- Un asiento no se edita ni se duplica. Se reenvía el mismo comando e
  idempotency key o se registra una reversa/ajuste nuevo.
- Cada operación SQL de un tenant establece `app.org_id`; nunca se desactiva
  RLS ni se usa `BYPASSRLS` para investigar o recuperar.
- No se copian a tickets, comandos, logs ni chats payloads, XML, CUIT, nombres,
  tokens, certificados o URLs de base de datos.
- No se copian teléfonos, cuerpos, variables de template, API keys ni firmas
  PerGo. El diagnóstico usa notification ID, estado y código estable.
- Un restore se hace sobre una base nueva y se valida antes del corte. No se
  restaura encima de la única copia operativa.
- Las migraciones son forward-only. Un rollback de Cloud Run no revierte el
  esquema; toda revisión anterior debe ser compatible con las migraciones ya
  aplicadas.

## Señales y alertas

El worker escribe al iniciar y cada 60 segundos un registro JSON
`event=worker_metrics`. Sólo contiene contadores agregados y booleanos:
outbox pendiente/alquilada/reintentando/DLQ, edad del evento más antiguo,
incertidumbres fiscales, aplicaciones/reversas contables pendientes, circuitos
y readiness. No contiene identificadores de organización ni PII.

Cloud Run usa:

- startup y readiness: `GET /readyz`, que comprueba la base propia;
- liveness: `GET /healthz`, que comprueba el proceso sin reiniciarlo por una
  caída transitoria de PostgreSQL;
- worker interno: `GET /metrics`, útil en desarrollo y diagnóstico privado.

El aprovisionamiento idempotente crea nueve métricas basadas en logs, ocho
políticas y un dashboard por entorno:

```bash
PYMES_DEPLOY_ENV=stg \
PYMES_MONITORING_NOTIFICATION_CHANNELS='projects/pymes-dev-352318/notificationChannels/CHANNEL_ID' \
./scripts/deploy/provision-monitoring.sh
```

En PRD se exige al menos un canal, salvo una excepción consciente con
`PYMES_MONITORING_ALLOW_NO_CHANNELS=true`. Para validar toda la configuración
sin llamar a GCP:

```bash
PYMES_DEPLOY_ENV=prd PYMES_MONITORING_DRY_RUN=true \
./scripts/deploy/provision-monitoring.sh
```

| Alerta | Primera comprobación | Acción segura |
|---|---|---|
| Heartbeat ausente / not ready | Revisión activa, logs de arranque y Cloud SQL | Recuperar dependencia; no cambiar tráfico hasta que `/readyz` sea verde. |
| Outbox > 5 min | Edad, reintentos y circuitos | Recuperar Fiscal/Accounting; el worker reintenta con la misma clave. |
| DLQ no vacía | `failure_code`, tópico, intentos, sin leer payload | Corregir causa, registrar cambio y usar replay explícito. |
| Fiscal uncertain | Estado del Fiscal mock y consulta exacta | Dejar actuar al reconciliador; nunca volver a autorizar. |
| Circuito abierto | Servicio indicado y su base | Recuperar servicio; el circuito se cierra después del cooldown y una respuesta válida. |
| Notificación incierta / backlog | Probe privado de PerGo, leases y último código estable | Recuperar PerGo; reenviar únicamente el mismo outbox y trace ID. |

## Identidad interna KMS

Esta KMS no pertenece a ARCA: autentica llamadas privadas de worker y
provisioner hacia Fiscal/Accounting. Los certificados ARCA siguen fuera de
alcance hasta su etapa específica.

Producción no admite `PYMES_INTERNAL_SIGNING_SEED_B64`. Worker y provisioner exigen
`PYMES_INTERNAL_KMS_KEY_VERSION` con recurso numérico completo. Antes de abrir
el health server lee la clave pública, comprueba nombre, algoritmo
`EC_SIGN_ED25519` y CRC32C, firma un desafío con CRC32C y verifica localmente la
firma. Cualquier fallo deja caer la revisión; nunca se habilita una semilla como
fallback.

Para rotar sin cortar tokens válidos:

1. crear una nueva versión de `internal-jwt-signing`, sin deshabilitar la
   anterior;
2. fijar la nueva en `PYMES_INTERNAL_KMS_KEY_VERSION` y la anterior en
   `PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS`;
3. ejecutar `cloud-run.sh`: despliega primero Fiscal/Accounting con ambas claves
   públicas y después el worker firmando con la nueva;
4. esperar al menos cinco minutos desde que no quede ninguna revisión firmando
   con la anterior y comprobar que no hay `UNAUTHORIZED_SERVICE`;
5. desplegar otra vez sin la versión anterior en overlap;
6. sólo entonces deshabilitar la versión anterior. Destruirla requiere un cambio
   posterior independiente.

`cmd/internal-jwks` puede generar el documento manualmente con ADC:

```bash
export PYMES_INTERNAL_KMS_KEY_VERSION='projects/PROYECTO/locations/us-central1/keyRings/pymes-v3-ENV/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/N'
export PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS='projects/PROYECTO/locations/us-central1/keyRings/pymes-v3-ENV/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/M'
(cd backend && go run ./cmd/internal-jwks)
```

Si KMS deja de responder después del arranque, no salen llamadas privadas nuevas,
el outbox permanece durable y el worker reintenta. Recuperar permisos/API/KMS;
no cambiar el `org_id`, no recrear eventos y no inyectar una clave local.

## Reconciliación

`DurableWorker.DispatchOnce` ejecuta el relay y, en cada ciclo, consulta hasta
20 ventas `fiscal_uncertain`. La consulta usa el snapshot y número originales.
Al recuperar un resultado autorizado persiste el CAE mock y crea una única
solicitud contable. Si Accounting procesó y se perdió la respuesta, el reintento
conserva el command ID, hash e idempotency key y obtiene el mismo asiento.

Diagnóstico tenant-scoped, mostrando sólo metadatos operativos:

```sql
BEGIN;
SELECT set_config('app.org_id', 'ORG_OPACA', true);
SELECT id, topic, attempts, failure_code, failed_at
FROM app.outbox_dead_letters
ORDER BY failed_at;
SELECT id, status, point_of_sale, document_type, voucher_number
FROM app.sales
WHERE status = 'fiscal_uncertain'
ORDER BY created_at;
ROLLBACK;
```

No se actualizan estados a mano. Si la incertidumbre no converge:

1. confirmar `/readyz` de Fiscal y revisar su base;
2. comprobar que el worker sigue emitiendo heartbeat y que el circuito cerró;
3. verificar por número exacto en el ledger fiscal mock;
4. conservar el caso abierto si el ledger tampoco conoce el resultado;
5. escalar como defecto de software, sin crear un segundo comprobante.

## Replay administrado de DLQ

Antes del replay:

1. abrir un cambio/incidente y obtener una referencia opaca;
2. identificar organización, UUID y `failure_code` sin copiar el payload;
3. demostrar que la causa está corregida y que el servicio está ready;
4. para `PERIOD_LOCKED`, no modificar el snapshot: registrar el ajuste/reversa
   mediante el flujo de dominio en un período abierto y sólo replayar si el
   comando original volvió a ser válido;
5. tomar backup antes de una recuperación masiva.

Ejecutar un evento por vez:

```bash
PYMES_DATABASE_URL='obtenida de Secret Manager sin imprimirla' \
PYMES_REPLAY_ORGANIZATION_ID='org_opaca' \
PYMES_REPLAY_EVENT_ID='00000000-0000-4000-8000-000000000000' \
PYMES_REPLAY_FAILURE_CODE='DELIVERY_FAILED' \
PYMES_REPLAY_ACTOR_REF='ops:actor-opaco' \
PYMES_REPLAY_CHANGE_REF='incident:INC-1234' \
./scripts/replay-dead-letter.sh
```

El script valida forma de todas las referencias, establece RLS, bloquea el
registro, inserta `app.outbox_dead_letter_replays`, mueve el evento y reinicia
sus intentos en una sola transacción. Una repetición exacta devuelve éxito sin
duplicar: imprime `replay-created` para una transición nueva y `replay-noop`
cuando la auditoría demuestra que ya ocurrió. La auditoría conserva
organización, evento, fallo, timestamps,
actor opaco y cambio; triggers impiden `UPDATE`, `DELETE` y `TRUNCATE`.

Después:

1. observar que cae la DLQ y que el outbox converge;
2. verificar el único resultado fiscal/asiento por sus IDs, no por contenido;
3. adjuntar al incidente sólo IDs, código y timestamps;
4. no hacer replay en lote hasta que un evento haya convergido.

Validación local del mecanismo:

```bash
make replay-smoke
```

## Caída y recuperación

| Componente | Efecto esperado | Recuperación |
|---|---|---|
| API | No acepta comandos; worker continúa | Rollback/redeploy de API; confirmar readiness antes de abrir tráfico. |
| Worker | API sigue confirmando transacciones y outbox crece | Restaurar worker; leases vencen y otro ciclo las recupera. |
| Cloud KMS interno | La revisión nueva no arranca o fallan entregas privadas; outbox crece | Restaurar API/permisos de la versión fijada; nunca usar semilla ni cambiar de versión sin overlap. |
| Fiscal | Circuito abre; autorización queda pendiente o incierta | Recuperar servicio/base; reintento o consulta exacta, nunca nuevo número. |
| Accounting | CAE puede estar persistido y posting queda pendiente | Recuperar servicio/base; reenviar mismo comando idempotente. |
| PerGo | El turno queda confirmado y la intención pendiente o incierta | Recuperar PerGo; el lease vence y se reintenta con el mismo trace ID. Un webhook adelantado impide duplicar. |
| PostgreSQL Pymes | API y worker no ready; no hay escrituras parciales | Recuperar Cloud SQL o restaurar base nueva; luego reconciliar ambos downstreams. |
| PostgreSQL Fiscal | Fiscal no ready; worker reintenta | Restaurar sólo Fiscal; consultar solicitudes exactas desde Pymes. |
| PostgreSQL Accounting | Accounting no ready; postings quedan en outbox | Restaurar sólo Accounting; reintentar comandos desde Pymes. |

El ensayo local autoritativo es `make recovery-e2e`: detiene y recupera cada
servicio y cada PostgreSQL por separado, y reinicia el worker para verificar
leases y continuidad.

## Backup y restore

Backup lógico previo a cambios:

```bash
SERVICE=pymes PYMES_DATABASE_URL='...' \
./scripts/backup-postgres.sh /ruta/segura/pymes.dump
```

Repetir de forma independiente para `fiscal` y `accounting`. Guardar checksum,
timestamp, versión de migraciones e imagen desplegada. En PRD, combinarlo con
backups automáticos/PITR de Cloud SQL; el dump no reemplaza esa política.

Restore:

1. detener escrituras o poner API y worker sin tráfico;
2. crear una base destino nueva y un rol de restore de vida corta;
3. restaurar con la variable `*_RESTORE_DATABASE_URL`, nunca con la URL activa;
4. ejecutar las migraciones idempotentes hasta la revisión actual;
5. validar relaciones, conteos por tenant bajo RLS y probes;
6. ejecutar reconciliación fiscal exacta y postings idempotentes;
7. publicar una nueva versión del secreto de conexión y desplegar una revisión
   que la fije por número;
8. conservar la base anterior sin escrituras durante la ventana de rollback;
9. destruir credenciales temporales y documentar checksums/resultados.

Ejemplo sobre un destino aislado:

```bash
SERVICE=pymes PYMES_RESTORE_DATABASE_URL='postgres://DESTINO_NUEVO' \
./scripts/restore-postgres.sh /ruta/segura/pymes.dump
```

Para validar el paso 6 de forma determinista, ejecutar la misma imagen del
worker contra el destino restaurado con `PYMES_WORKER_RUN_ONCE=true`. El worker
hace una única pasada de lease/dispatch y reconciliación fiscal, conserva las
garantías idempotentes y termina; cualquier error de dispatch produce un cierre
con código estable. El valor por defecto sigue siendo el loop continuo de
producción. Si el backlog supera una tanda, repetir la pasada explícitamente y
comprobar las métricas entre ejecuciones.

`make backup-restore-smoke` no escribe en las bases activas ni elimina
volúmenes. Crea tres bases fuente y tres destinos con nombres validados,
ejecuta dos veces las migraciones de cada servicio y carga una organización
operativa común. Accounting se provisiona mediante su plano administrativo:
el ensayo comprueba el mapping global
`public.pymes_accounting_organizations`, el schema tenant, cuentas, período,
comando y asiento; Fiscal conserva una solicitud tenant y Pymes conserva
organización, compra y outbox.

El backup de Pymes se toma antes de entregar el posting y el de Accounting
después de postearlo. Ese desfase intencional reproduce el caso crítico
“Accounting confirmó pero Pymes perdió la respuesta”. Tras restaurar las tres
bases, el smoke vuelve a ejecutar todas las migraciones, levanta Fiscal y
Accounting sobre los destinos y corre la misma imagen del worker con
`PYMES_WORKER_RUN_ONCE=true`. La primera pasada debe recuperar el mismo
`journal_entry_id`, publicar el outbox y grabar un único inbox; la segunda debe
ser un no-op sin duplicar comando ni asiento. Las credenciales locales se leen
del entorno de los contenedores y los dumps sólo existen dentro de un
directorio temporal con permisos restringidos.

## Rollback de despliegue

1. congelar deploys y anotar revisión actual, revisión candidata, imágenes y
   migraciones ejecutadas;
2. elegir una revisión anterior compatible con el esquema actual;
3. mover primero una fracción de tráfico si el servicio lo admite y observar
   readiness, errores, heartbeat, DLQ e incertidumbres;
4. mover 100 % sólo cuando las invariantes converjan;
5. si ninguna revisión es compatible, corregir hacia adelante;
6. no revertir una versión KMS/JWT sin volver a publicar su clave en el JWKS
   antes del worker y sin conservar la ventana de cinco minutos; no volver a una
   contraseña de base ya revocada.

Comandos de referencia, ejecutados por servicio y región:

```bash
gcloud run revisions list --service=pymes-v3-ENV-api --region=us-central1
gcloud run services update-traffic pymes-v3-ENV-api \
  --region=us-central1 --to-revisions=REVISION_COMPATIBLE=100
```

Para worker (sin tráfico público), desplegar la imagen anterior compatible y
mantener una sola instancia activa. No ejecutar simultáneamente dos versiones
con semántica distinta. Tras cualquier rollback, verificar:

- `/healthz` y `/readyz`;
- heartbeat menor a 3 minutos;
- edad de outbox decreciente;
- cero nuevas DLQ e incertidumbres;
- una sola autorización y un solo asiento por fuente.

## Cierre de incidente

Un incidente sólo se cierra cuando las señales vuelven a verde, los eventos
pendientes convergen, no hay divergencias sin explicar, se registraron replay o
restore en auditoría y se añadió un caso de regresión si hubo defecto de código.
