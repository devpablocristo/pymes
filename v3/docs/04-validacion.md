# Validación y spikes descartables

Los spikes viven fuera de cualquier repositorio, por ejemplo en
`/tmp/pymes-v3-spikes`, y se eliminan después de documentar el resultado. No
consumen certificados reales ni llaman producción ARCA.

| Spike | Montaje | Aceptación |
|---|---|---|
| ARCA falso | HTTP fake que devuelve CAE, rechazo y corte tras procesar | la misma solicitud/número es consultada y nunca reemitida; se preserva snapshot. |
| Posteo con respuesta perdida | accounting fake persiste y corta conexión | reintento con igual clave devuelve mismo `journal_entry_id`; no duplica líneas. |
| Dos organizaciones | dos schemas y tokens con org distinta | ninguna consulta o comando puede observar/mutar la otra organización. |
| Identidad interna | JWKS local + KMS falso con CRC32C, versión explícita y overlap; tokens expirados, audiencia/rol incorrecto | producción rechaza semillas y aliases; se valida firma al arrancar, se rechazan claims ausentes o dispares y se propagan request/correlation/actor sin cruce tenant. |
| Caída/recuperación | apagar fiscal y contable durante publicaciones | outbox sobrevive, lease vence sin doble trabajo, backoff y reconciliación convergen. |

## Tabla de aceptación de contratos

| Caso | Fiscal | Contable |
|---|---|---|
| A/B/C | tipo, receptor, IVA y número explícito válidos | venta separa neto/IVA/total |
| NC/ND | asocia original y conserva referencia | reversa/ajuste enlaza asiento fuente |
| IVA y redondeo | importe fijo en snapshot, sin float | débitos = créditos por moneda; línea de redondeo explícita |
| Moneda extranjera | cotización y fecha inmovilizadas | importe transaccional y funcional; diferencia FX explícita |
| Período bloqueado | puede autorizarse pero no postear fuera de política | devuelve `PERIOD_LOCKED` sin mutación |
| Pago parcial | no aplica | reduce partida por importe, deja saldo, idempotente |

Antes del MVP se ejecutarán las suites del fork OA y del SDK en CI reproducible,
más las pruebas de Pymes v3 que implementen estos casos. Se prohíbe reutilizar
código GPL de LedgerSMB; sólo se transcriben casos expresados como requisitos.

## Resultado ejecutado

Los cinco spikes iniciales pasaron y sus invariantes se trasladaron a suites
durables. `make db-integration` valida RLS, numeración concurrente,
provisionamiento, Clerk, PostgreSQL Fiscal y el boundary contable por schema.
`make fiscal-e2e` y `make accounting-e2e` prueban los clientes reales contra
los servicios privados. `make backup-restore-smoke` construye tres bases fuente
y tres destinos descartables, restaura datos tenant reales —incluido el schema
headless de Accounting—, reaplica dos veces las migraciones y demuestra que el
worker one-shot recupera una respuesta contable perdida sin duplicar el asiento.

`make observability-e2e` valida endpoints y el heartbeat JSON sin PII;
`make monitoring-config-check` genera métricas, políticas y dashboard de STG y
PRD sin llamar a GCP; `make replay-smoke` aplica todas las migraciones en una
base descartable, mueve una DLQ, repite el comando como no-op y demuestra que
su auditoría no admite mutación.

`make workflow-policy-check` prueba que los workflows manuales de Google y
ARCA homologación acepten sólo STG, `main` con CI verde para el SHA exacto y un
`GITHUB_RUN_ATTEMPT` inicial, y que la auditoría completa de environments
preceda el uso de credenciales. `make protected-live-validation-test` prueba
sus validadores, sin red ni credenciales reales: mantiene tokens fuera de
`argv`, usa archivos `0600`, no imprime cuerpos de error y rechaza origen
`.invalid`, credenciales fiscales de producción o metadata Google que no
pertenezca a Pymes. El probe ARCA llama exclusivamente la validación WSAA/WSFE
del punto de venta, comprueba con `FECompUltimoAutorizado` que las nueve
secuencias A/B/C, NC y ND estén vacías y nunca ejecuta una autorización.

<!-- drift:bind v3/scripts/deploy/release-evidence-test.sh -->
<!-- drift:bind v3/scripts/deploy/bootstrap-release-evidence.sh -->
<!-- drift:bind v3/scripts/deploy/retain-release-manifest.sh -->

`make release-evidence-test` usa un adapter `gcloud` falso para probar el
bootstrap plan-only, el IAM exacto del builder y la separación entre `apply` y
el Bucket Lock irreversible. El lock falla sin la confirmación literal; la
publicación exige un bucket ya bloqueado, usa precondición de generación cero,
rechaza manifiestos alterados o con claves extra, vuelve a descargar la
generación publicada y emite un receipt sólo si checksum, metadata y retención
coinciden. El gate demuestra el mecanismo; no acredita que los buckets hayan
sido creados o bloqueados en GCP.

<!-- drift:bind v3/scripts/deploy/cloud-restore-drill.sh -->
<!-- drift:bind v3/scripts/deploy/cloud-restore-drill-test.sh -->

`make cloud-restore-drill-test` prueba con adapters que el orquestador cloud
valida primero el manifiesto de release y los tres backups, crea exactamente
tres destinos aislados —Pymes, Fiscal y Accounting— y nunca adopta una base
preexistente. También cubre estado con checksum, ownership marker, validator
revisado, dos reconciliaciones sin duplicados y cleanup con confirmación
separada. No conecta Cloud SQL ni sustituye el drill real.

<!-- drift:bind v3/scripts/deploy/collect-pilot-evidence.sh -->

`make pilot-evidence-test` reemplaza `gcloud` y `curl` por adapters
determinísticos y prueba Agenda, PerGo, Google/Meet y ARCA sin red. Demuestra
que confirmación, checksum de release, checkout, permisos de tokens y revisión
activa fallan antes de contactar un adapter; que un resultado no terminal no
publica evidencia; que los bundles se publican atómicamente sin respuestas
crudas ni PII; que el piloto PerGo exige una audience HTTPS real para la
identidad privada de Cloud Run y conserva sólo su referencia SHA-256; y que
cualquier alteración posterior rompe
`checksums.sha256`. Este gate valida el collector, no sustituye la ejecución de
los pilotos reales.

`make scheduling-e2e` cubre también la edición operativa de un turno:
`expected_version`, RLS y locks se resuelven dentro de PostgreSQL; un replay
exacto conserva el snapshot de respuesta del adapter, la misma clave con otro
hash devuelve `IDEMPOTENCY_KEY_REUSED`, una versión obsoleta no muta y los
cambios de participantes vuelven a validar recursos o cupo grupal. Servicio,
horario, snapshots comerciales, asignaciones y estado no son campos aceptados
por el `PATCH`; conservan sus comandos específicos. `make web-ci` prueba el
formulario separado, el cliente generado y que un rechazo mantiene la edición
abierta sin dejar una promesa no manejada.

<!-- drift:bind v3/backend/internal/scheduling/calendar_projection/helpers/events.go -->
<!-- drift:bind v3/backend/internal/calendars/worker/helpers/events.go -->

`make scheduling-e2e` y `make calendars-e2e` cubren la proyección Calendar por
estado, el par `delete` original/`upsert` reemplazo al reprogramar, la
expiración de holds, el opt-in inmutable de Meet y la aceptación concurrente de
waitlist exactamente una vez. El contrato cruzado demuestra que productor y
consumidor calculan el mismo snapshot; Calendars rechaza campos desconocidos,
digests no hexadecimales o alterados y deletes que lleven datos de un upsert.
Las reservas de sesiones no admiten una reunión individual ni emiten
proyección.

`make security` ejecuta `govulncheck` sobre Pymes y el runtime headless
contable, además de `npm audit` sobre Fiscal. El gate quedó en cero
vulnerabilidades alcanzables después de actualizar Go 1.26.5, `pgx`, `x/text`
y `go-jose`; la incorporación del cliente KMS actualizó además `grpc` a una
versión sin la vulnerabilidad alcanzable detectada por `govulncheck`.

## Evidencia H8 disponible

La política de release ya es verificable sin desplegar. `make
workflow-policy-check` comprueba que release y validaciones live sean manuales,
partan de `main`, exijan un CI verde para el SHA exacto y una confirmación
escrita. Los dos jobs live están fijados al environment `stg`, no solicitan WIF
ni aceptan tokens como inputs; los secretos se materializan únicamente en el
paso posterior a la auditoría de controles. Build y deploy usan identidades WIF
separadas por entorno. El primer paso de todos esos jobs rechaza cualquier
`GITHUB_RUN_ATTEMPT` distinto de `1`: Build lo hace antes de solicitar la
identidad del builder y Deploy antes de checkout, descargar artefactos o
autenticar. Un job o workflow reejecutado no puede reutilizar una validación ni
un manifiesto anterior y debe reemplazarse por un nuevo `workflow_dispatch`.
El builder rechaza
worktrees sucios, fija tanto Pymes como Open Accounting a commits completos,
publica SBOM y provenance y entrega únicamente referencias
`@sha256:<digest>` mediante un manifiesto con claves permitidas.
El mismo gate ejecuta los tests del seed inicial: rechaza otro head de GitHub,
CI fallido, imagen o service account distinta, escalado no nulo, tráfico, IAM,
variables, volúmenes, Cloud SQL, Direct VPC, Serverless VPC Connector y jobs con
ejecuciones. La auditoría admite únicamente las mutaciones Cloud Run sobre los
FQN exactos del proyecto, región y tipo esperados y los eventos
`iam.serviceAccounts.actAs` exitosos sobre las diez identidades runtime
allowlisted. Exige exactamente una mutación de creación por cada uno de los once
FQN y rechaza actualizaciones posteriores o `SetIamPolicy`. El seed no pasa
ninguna variante `--allow-unauthenticated`, porque ambas fuerzan una operación
IAM al reintentar; la política vacía se comprueba antes y después. Los fixtures
rechazan además otra región, tipo, cuenta, permiso o recurso de autorización.
La finalización acota inicio y fin con dos minutos de margen superior para
timestamps fraccionarios y reloj, espera al menos diez minutos de asentamiento
y exige dos lecturas idénticas de Admin Activity separadas por veinte segundos.
También prueba que la autoridad recurrente acepte la transición
seed → bootstrap → operational, pero rechace roles custom ampliados, Run Admin
de proyecto, condiciones IAM y cualquier administrador o invoker fuera de la
allowlist.

La autoridad no se confía sólo al bootstrap. Antes de autenticar al builder, el
job protegido autentica al deployer exacto del environment y vuelve a comprobar
pool/provider WIF, cuentas builder/deployer activas y sin claves, trust completo,
Artifact Registry, los catorce Secrets, diez identidades runtime keyless, KMS y
los once recursos Run. También exige que las políticas efectivas
`iam.disableCrossProjectServiceAccountUsage` e
`iam.disableServiceAccountKeyCreation` estén aplicadas y que
`orgpolicy.googleapis.com` esté disponible. Policy Analyzer debe estar
completamente explorado, sin grupos, caminos de impersonación ni pares efectivos
fuera de la allowlist de builder, deployer y cada identidad runtime. Una
consulta inversa adicional parte de los permisos sensibles —no del nombre del
rol— y compara cada triple recurso/permiso/identidad sobre proyecto, release y
runtime service accounts, Run, Secrets, Artifact Registry y KMS. Por eso un rol
custom o alternativo con el mismo poder, un grant heredado, una identidad
cross-environment o una cadena de impersonación también bloquean. El deploy
repite el límite KMS inmediatamente antes de usar las claves; cualquier lectura
incompleta, cuota agotada después del retry acotado o drift falla cerrado.

El bootstrap IAM incorpora un guarda de fuente y operador antes de cualquier
mutación: checkout limpio de `main`, HEAD y árbol idénticos al repositorio
GitHub esperado, cuenta activa directa `softponti@gmail.com` y ausencia de
impersonación, credential override, access-token file o login config en
`gcloud`. La cadena de ancestros debe ser exactamente proyecto
`pymes-dev-352318` → folder `673291958610` → organización `663017421195`.
Builder no recibe ningún rol de ancestro; el deployer recibe únicamente los
roles custom de lectura de IAM de organización y folder, sin condiciones. La
creación o actualización inicial de esos dos roles de organización y sus
bindings requiere un operador humano con administración de custom roles y
políticas IAM en esos scopes; el workflow no puede otorgarse esa facultad.

El gate busca cualquier `principal://` o `principalSet://` que mencione el pool
de release y exige el conjunto exacto correspondiente a la fase. Durante STG
sólo pueden existir Build y deployer STG, y la presencia prematura del subject
PRD falla; después del cierre STG, la preparación PRD exige exactamente esos
dos más el deployer PRD. Todos son bindings
`roles/iam.workloadIdentityUser` sobre su service account destino. Además relee
las políticas conocidas de proyecto, Artifact Registry, Secrets, KMS, runtime
y Cloud Run.
Builder y deployers deben aparecer sin claves y sin adjunción a workloads: se
combina Cloud Asset con lecturas directas de servicios, jobs y revisiones Cloud
Run, mientras la org policy prohíbe adjuntarlos desde otro proyecto. Las
políticas de cada runtime SA contienen sólo el `actAs` del deployer de su
entorno y su autoridad efectiva debe coincidir con la allowlist propia del
componente.

Cloud Asset Search y Policy Analyzer son fuentes de consistencia eventual:
`fullyExplored` prueba completitud del snapshot analizado, no frescura de un
cambio recién aplicado. Las lecturas directas son autoritativas para los
recursos enumerados; los barridos globales WIF/workload requieren una ventana
sin mutaciones, repetición y conservación de la evidencia antes de un cutover.
La espera automática de diez minutos y las dos lecturas separadas por veinte
segundos corresponden específicamente a Admin Activity del seed inicial; no
deben presentarse como garantía universal de frescura de Cloud Asset.

`make legacy-wif-test` prueba el corte sin tocar GCP: acepta sólo canaries
`stg operational` del SHA, árbol y workflow exactos; rechaza `bootstrap`, un
SHA ancestro, un árbol distinto, análisis IAM incompleto o con autoridad
residual, un deployer PRD provisionado prematuramente, grupos de ancestros cuya
membresía no pueda comprobarse, referencias Cloud Asset en otra región, errores
de Service Usage y marcadores cuyos eventos estén incompletos, desordenados o
pertenezcan a actores diferentes. También demuestra que el modo interno de test
no puede usarse al ejecutar directamente el script de retiro.

La release consulta GitHub antes de solicitar una credencial WIF. En cada
ejecución comprueba mediante la vista pública de la rama que `main` esté
protegida para todos y exija únicamente `Pymes V3 validate`; también relee las
reglas del environment seleccionado. La auditoría operativa, obligatoria antes
de crear WIF, comprueba además que `main` no exija reviewers, mantenga
resolución de conversaciones, historial lineal, ausencia de force-push/borrado
y aplicación a administradores. Los environments `stg` y `prd` sólo admiten
`main`; PRD conserva el conjunto exacto de reviewers de despliegue aprobado,
impide autoaprobación y no permite bypass administrativo. El bootstrap es
plan-only por defecto y falla cerrado si falta cualquiera de esas reglas.

`scripts/deploy/cloud-run-security-check.sh` ejecuta `cloud-run.sh` en dry-run
para STG y PRD sin permitir que se invoque `gcloud`. El gate positivo cubre:

- label `pymes-v3-release` igual al SHA fuente en los seis servicios y cinco
  jobs;
- marcador Web exacto
  `entorno:sha-fuente:sha256:digest-web`, proxy `/api/` al BFF y callbacks
  Clerk, PerGo y Google en el mismo origen público;
- imágenes exclusivamente por digest, secretos por versión numérica,
  invokers exactos y ausencia de `roles/run.invoker` a nivel proyecto;
- API, worker, Fiscal y provisioner con Direct VPC según su necesidad, además
  de Private Google Access y Public NAT comprobables. El bootstrap exige
  cobertura `ALL_IP_RANGES` de la única subred y converge de forma segura un
  recurso propio que hubiera quedado limitado al rango primario;
- Fiscal `mock` y `arca` usando siempre `fiscal-vault` en producción, con
  primary habilitada, rotación de 90 días e IAM directo; PerGo y Google sólo
  reciben configuración cuando su flag está habilitado.

Los casos negativos rechazan tags mutables, SHA inválido, origen público fuera
de Clerk, callback Google distinto, audience PerGo ausente, HTTP o con path,
CIDR fuera de `/20`–`/26`, modo Fiscal inválido, ARCA sin políticas de issuer y
creación de Cloud NAT sin aceptación explícita del costo. También prueban que
bootstrap no admite PRD, ARCA, PerGo, Google, origen real ni un secreto Clerk sin
`lifecycle=bootstrap-temporary`; operational rechaza ese label y la simulación
de metadata se acepta únicamente en dry-run. Un negativo adicional simula un
servicio STG activo y demuestra que bootstrap aborta antes de ejecutar
migraciones o cambiar ingress/IAM.

Cada servicio se crea primero como candidato con tráfico cero. En bootstrap,
API y Web además usan ingress interno y carecen de `allUsers`; el worker
conserva escalado manual `0`, mínimo de revisión `0` y no ejecuta deployment
health check. El script no promueve ni sondea esas URLs y, después de verificar
la configuración, elimina todos los tags de los seis servicios. El bootstrap
termina así con revisiones inertes, sin tráfico y sin una URL taggeada que
pueda confundirse con un release operacional.

En `operational`, las URLs taggeadas de API y Web quedan protegidas durante
pretraffic por una capability efímera de 32 bytes codificada como 64
hexadecimales. El workflow la genera por ejecución, la enmascara y la inyecta
directamente en ambas revisiones. Una petición al host candidato sin el header
exacto responde `404`; Web agrega la capability sólo al proxy interno hacia la
API candidata. El host estable de API no requiere ese header. El
verificador demuestra los rechazos sin capability y ejecuta los probes con la
capability correcta sin imprimirla.
El deploy y el verificador transportan ese valor mediante archivos efímeros con
permisos `0600`; la matriz stateful rechaza que aparezca en `argv` de `gcloud` o
`curl` y comprueba que los archivos se eliminen.

Antes de mutar tráfico, el release identifica un único baseline activo por
servicio y valida que el Web anterior apunte exactamente a la URL taggeada de
la revisión API anterior, con tag y capability coincidentes. No acepta
descripciones ambiguas, errores de lectura como si fueran ausencia ni reutiliza
el mismo SHA/tag ya activo. Todos los deploys de servicios fuerzan el chequeo
IAM de invocación y el readback verifica ingress, la ausencia o presencia
esperada de `allUsers` y los invokers exactos.

El worker permanece en escalado `0` durante pretraffic, se enruta último y sólo
entonces pasa a `1`. La promoción no se considera sana por el mero arranque del
contenedor: espera en Cloud Logging un evento
`worker_release_ready` de la revisión y SHA exactos, emitido únicamente después
de que el worker logró su primera lectura durable de métricas desde PostgreSQL.
Un fallo o cancelación antes de esa lectura no emite la señal ni inicia el
dispatch.

El verificador compara digests, SHA, forma del contenedor, identidades, ingress,
invokers, SQL, VPC, KMS/JWKS y configuración runtime antes de promover.
Después de mover exactamente el 100 % del tráfico, verifica el estado activo
dentro de la misma transacción de release, antes de desarmar el rollback. No
existe un segundo paso de verificación post-release capaz de fallar cuando ya
no puede revertir.

El asentamiento conserva exactamente un tag en API —el tag del release activo
que usa Web— y cero tags en Fiscal, Accounting, Accounting Admin, Worker y Web.
Cada retiro de tag se prueba tanto en control plane como contra su URL pública,
que debe responder `404`; luego se elimina la revisión candidata que ya no se
usa. También se comprueba que la URL del API anterior y la URL Web candidata
queden revocadas. Ante un fallo se restaura primero el tag API exacto del
baseline, se prueba su `/readyz` con su capability histórica y recién entonces
se devuelve Web. Para worker se baja primero a `0`; un primer despliegue fallido
se vuelve inerte, se elimina y se comprueba su ausencia.

Además de ese rollback automático, `rollback-cloud-run.sh` implementa la
recuperación durable por SHA de release: resuelve una única revisión API y Web
por label inmutable, pero el label no alcanza para autorizarla. Antes de leer o
mutar Cloud Run exige el manifiesto completo descargado de la release y el
SHA-256 que el job de build registró separadamente en su summary, valida su
allowlist y los repositorios canónicos, comprueba el pin de Open Accounting y
repite el mismo gate de provenance, materiales y SBOM de la release. Luego
exige que los digests API/Web de las revisiones coincidan exactamente con ese
manifiesto y valida readiness, identidad, ingress/IAM, tag, capability y pin
Web → API. Restaura primero y prueba el tag API, mueve Web antes que API y deja
otra vez la política exacta de tags. La capability se usa mediante un archivo
temporal modo `0600`, no aparece en argumentos ni logs. Sus tests locales
cubren éxito, orden, checksum o repositorio inválido, attestation rechazada,
digest de revisión ajeno al manifiesto, revisiones duplicadas o incompatibles,
timeout antes/después de aplicar tráfico y timeout de probe.
`cloud-run-transaction-test.sh`, ejecutado por `make quality`, reutiliza las
funciones reales del deploy contra un control/data plane stateful y agrega
fallos inyectados de `list`, `describe`, deploy con respuesta perdida, IAM,
promoción, asentamiento y ausencia de señal del worker. Cada escenario exige
baseline exacto o estado inerte fail-closed y prohíbe afirmar
`ROLLBACK COMPLETE` sin convergencia.

El smoke de backup/restore también prueba ahora los guardas destructivos:
permisos `0600`, manifiesto SHA-256 ligado a servicio/base/archivo, rechazo de
archivo alterado o de otro servicio, destino nuevo y vacío, confirmación exacta
obligatoria y restore en una única transacción. Los wrappers de libpq no
incluyen la URI ni su password en el vector de argumentos y Compose publica
todos sus puertos exclusivamente en `127.0.0.1`.

Esta evidencia demuestra implementación y gates locales, incluido el rollback
durable, no operación real.
Todavía no hay evidencia en el repositorio de imágenes v3 publicadas, recursos
WIF/red aplicados, controles GitHub reconciliados, servicios STG/PRD
desplegados, restores administrados ni pilotos de Agenda, PerGo, Google o
ARCA. `verify-cloud-run.sh` sólo constituirá evidencia de runtime después de
una release real exitosa.

El SHA fijado de Open Accounting
`ad1c182093986279aac7fb6582f7788202112a78` tiene CI remoto verde en el run
`30745457821`: pasaron backend, lint, integración PostgreSQL, E2E, frontend, el
runtime Pymes headless y el gate agregado. `make ci` de Pymes también pasa
contra el runtime incluido por ese checkout exacto y los controles H8 actuales.
Pymes PR #47 quedó fusionado en
`ccff2c106da92f3bfc74b2d12b5f4409aa743050`, y el workflow remoto de `main`
`30744384829` quedó verde para ese baseline.
