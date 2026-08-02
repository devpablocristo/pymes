# Seguridad y entornos GCP de Pymes v3

Fecha de provisión: 2026-08-02.

Pymes v3 comparte el proyecto `pymes-dev-352318` con otros productos para
reducir costes. El aislamiento de v3 se hace por entorno, cuentas de servicio,
permisos por secreto y claves KMS; no se reutilizan identidades ni claves entre
STG y PRD.

## Recursos compartidos reutilizados

| Recurso | Uso |
|---|---|
| Región `us-central1` | Cloud Run, KMS y réplica única de secretos. |
| Artifact Registry `pymes` | Imágenes de Pymes v3 cuando se publique el runtime. |
| Cloud SQL `pymes-dev-db` | Bases lógicas separadas de Pymes, Fiscal y Accounting para STG/PRD; no se creó una instancia adicional. |

## Cifrado e identidades

| Entorno | Claves simétricas | Firma de identidad interna | Workload identities |
|---|---|---|---|
| STG | `pymes-v3-stg/{secrets,calendar-tokens,fiscal-vault}` | `pymes-v3-stg/internal-jwt-signing/cryptoKeyVersions/1` | `pymes-v3-{api,worker,web,provision,fiscal,accounting,accounting-admin}-stg` más identidades de migración |
| PRD | `pymes-v3-prd/{secrets,calendar-tokens,fiscal-vault}` | `pymes-v3-prd/internal-jwt-signing/cryptoKeyVersions/1` | `pymes-v3-{api,worker,web,provision,fiscal,accounting,accounting-admin}-prd` más identidades de migración |

Las claves simétricas deben ser regionales y rotar como máximo cada 90 días.
STG y PRD ya cumplen esa política para `secrets`, `calendar-tokens` y
`fiscal-vault`.
`secrets` protege las réplicas de Secret Manager y su único principal
criptográfico directo es el service agent de Secret Manager.
`calendar-tokens` sólo cifra conexiones OAuth desde API/worker;
`fiscal-vault` sólo cifra material ARCA desde Fiscal. La clave
`internal-jwt-signing` es asimétrica
`EC_SIGN_ED25519`, distinta por entorno: API, worker y provisioner reciben
`roles/cloudkms.signer` y `roles/cloudkms.publicKeyViewer` sobre esa clave. API,
worker y provisioner firman al invocar sus servicios privados; Fiscal y
Accounting no reciben permisos de firma ni acceso a material privado y validan
el JWKS publicado durante el despliegue. Las identidades conservan
`roles/cloudsql.client` para abrir el socket de Cloud SQL, lo que no concede
credenciales ni permisos SQL.

## Referencias de secretos

Al cerrar el bootstrap, cada nombre de esta tabla debe existir como secreto
global, con una única réplica en `us-central1` cifrada por la clave KMS regional
del entorno. Cloud Run no admite secretos regionales; esta réplica única
conserva la localización y evita duplicar coste de almacenamiento:

| Consumidor | Sufijo de secreto |
|---|---|
| API | `clerk-secret-key`, `clerk-webhook-secret`, `scheduling-action-token-secret`, `pergo-webhook-secrets`, `database-url` |
| Worker | `scheduling-action-token-secret`, `worker-database-url`, `pergo-api-key` |
| API y worker al habilitar Google | `google-client-secret` |
| Fiscal | `fiscal-database-url` |
| Accounting | `accounting-database-url` |
| Accounting admin | `accounting-admin-database-url` |
| Migraciones | `migrate-database-url`, `fiscal-migrate-database-url`, `accounting-migrate-database-url` |

`pymes-v3-{stg,prd}-scheduling-action-token-secret` ya existen con versión 1 e
IAM mínimo aplicado. Los demás contenedores enumerados existen, pero existencia
no implica que su valor sea operativo ni que versión, réplica, CMEK o IAM hayan
superado la auditoría final. En particular todavía faltan valores reales
verificados para Clerk webhook, PerGo y Google. En STG,
`pymes-v3-stg-clerk-webhook-secret` posee una versión de bootstrap y el label
`lifecycle=bootstrap-temporary`: ese valor aleatorio sólo permite crear
revisiones cerradas a tráfico; no representa un endpoint Svix configurado ni
es válido para una release operacional.

El nombre final sigue el patrón `pymes-v3-{stg|prd}-{sufijo}`. El secreto
global `fiscal-credential` es obsoleto: las credenciales de clientes viven
cifradas en la base Fiscal y no se migran a Secret Manager. Las semillas JWT
que pudieran existir de una provisión anterior tampoco se montan ni se migran:
después de desplegar y verificar KMS deben retirarse sus bindings y deshabilitar
sus versiones mediante un cambio operativo separado y recuperable. Nunca se
guardan secretos en Git. La semilla fija de Compose es exclusivamente local.

Google Calendar queda deshabilitado hasta que cada entorno tenga su propio
cliente OAuth, secreto y CryptoKey `calendar-tokens`. El único redirect
permitido es
`https://API_ORIGIN/api/v1/calendars/google/oauth/callback`: no lleva
organización en el path y nunca apunta a la Web estática. El deployment gate
rechaza callbacks por tenant, claves de otro entorno y cualquier intento de
inyectar OAuth/KMS en Web.

## Release sin claves persistentes

La topología final usa un pool WIF dedicado y tres identidades:
`pymes-v3-gh-build`, `pymes-v3-gh-deploy-stg` y
`pymes-v3-gh-deploy-prd`. La condición del provider limita repositorio,
owner, rama `main`, workflow y evento `workflow_dispatch`; además permite
únicamente el subject de rama para Build y los subjects `environment:stg` y
`environment:prd` para sus deployers respectivos. Build sólo escribe imágenes;
cada deploy sólo puede leer imágenes y administrar los recursos de su entorno,
suplantando las service accounts runtime exactas.

La creación y el trust son estrictamente STG-first: mientras se prepara,
finaliza y cierra STG sólo existen los bindings exactos de Build y deployer
STG; cualquier trust o service account deployer PRD prematuro bloquea. Se
ejecutan el canary anterior al retiro, el retiro legado, el canary posterior y
`close`;
recién entonces puede crearse `pymes-v3-gh-deploy-prd`. `prepare` para PRD
repite la auditoría completa del cierre antes de realizar su primera mutación.

Cloud Run no permite conceder `roles/run.admin` sobre un servicio o job que aún
no existe, y ese rol no puede acotarse por nombre mediante IAM Conditions. Por
eso nunca se entrega Run Admin a nivel proyecto al workflow. Después de
`prepare`, el Owner directo preexistente `softponti@gmail.com` ejecuta
`seed-cloud-run-resources.sh` desde el head protegido de `main` con CI verde y
un manifiesto de digests atestiguados. El script no crea autoridad nueva: deja
seis servicios internos con escalado manual cero, tráfico e IAM vacíos, y cinco
jobs sin ejecuciones; ninguno recibe secretos, SQL ni red. `finalize` valida el
manifiesto y su SHA-256, el estado exacto de los once recursos y Admin Activity
antes de conceder Run Admin únicamente sobre esos nombres. La ventana auditada
sólo acepta escrituras Cloud Run cuyos FQN ligan proyecto, `us-central1`, tipo y
nombre exactos, más el `iam.serviceAccounts.actAs` que Cloud Run registra para
una de las diez identidades runtime allowlisted. Debe existir exactamente una
mutación inicial por cada FQN; una actualización posterior, `SetIamPolicy`,
otra escritura IAM, región, cuenta o recurso falla cerrado. `finalize` sólo
acepta una ventana explícita inicio/fin, agrega dos minutos al límite superior
para cubrir precisión fraccionaria y reloj, exige que el seed haya terminado al
menos diez minutos antes y compara dos lecturas idénticas separadas por veinte
segundos para no confiar en una ingestión parcial de Admin Activity.
El seed no usa flags de autenticación de `gcloud`: ambos caminos
`--[no-]allow-unauthenticated` pueden escribir IAM en un retry. En su lugar,
Cloud Run conserva el default privado y el readback exige policy vacía.

Proyecto `pymes-dev-352318`, número `884236221349`, región `us-central1`,
Artifact Registry `pymes`, provider y service accounts de release son
constantes del workflow, no variables GitHub mutables. `prepare` y `finalize`
reciben un único `PYMES_RELEASE_IDENTITY_ENV`. Aun en modo plan, el bootstrap
exige checkout limpio de `main`, HEAD y árbol iguales al `main` remoto del
repositorio GitHub de ID `1173650578`, cuenta activa directa
`softponti@gmail.com` y ninguna configuración de impersonación o credencial
externa en `gcloud`. Antes de conceder permisos rechaza cuentas deshabilitadas,
claves administradas por usuarios, callers distintos del subject exacto o roles
preexistentes fuera de la allowlist.

La cadena de recursos se fija a proyecto `pymes-dev-352318`, folder
`673291958610` y organización `663017421195`. Builder no recibe permisos en
folder u organización. Cada deployer recibe exactamente, sin condición, los
roles custom de lectura
`pymesV3ReleaseFolderIamRead` y
`pymesV3ReleaseOrganizationIamRead` en el ancestro correspondiente. Los demás
bindings de esas policies deben ser del Owner directo revisado o roles
estrictamente de lectura; grupos, dominios, principals federados, otras
identidades Pymes o roles con autoridad fallan cerrado.

Crear o actualizar esos roles custom y sus bindings es una acción humana de
bootstrap: el operador necesita administración de custom roles en la
organización y capacidad de leer/escribir IAM en la organización y el folder.
No se concede esa autoridad al WIF ni puede autoaprovisionársela el workflow.
`orgpolicy.googleapis.com` forma parte de las APIs requeridas; `prepare` puede
habilitarla, pero `finalize` y la release sólo leen y fallan si está ausente.
En cada ejecución deben resultar efectivamente forzadas sobre el proyecto:

- `constraints/iam.disableCrossProjectServiceAccountUsage`;
- `constraints/iam.disableServiceAccountKeyCreation`.

La prueba efectiva usa IAM Policy Analyzer a nivel proyecto con grupos, roles
e impersonación expandidos y exige exploración completa. Además de validar la
autoridad saliente de cada cuenta conocida, ejecuta consultas inversas por
permisos sensibles y sólo acepta triples recurso/permiso/identidad exactos para
Project, service accounts, Cloud Run, Secrets, Artifact Registry y KMS. Esto
rechaza autoridad equivalente obtenida mediante roles custom, herencia,
service-account impersonation o grants cross-environment. Además lee la cadena
exacta de ancestros. Un scan del path completo del pool rechaza cualquier
binding directo `principal://` o `principalSet://` fuera del conjunto exacto
de la fase: Build+STG y cero PRD antes de `close`; Build+STG+PRD después.
Las políticas conocidas de proyecto, Artifact Registry, Secrets, KMS, runtime
y Cloud Run se releen directamente. Builder y deployers deben estar keyless y
no adjuntos a ningún workload; el inventario combina Cloud Asset con servicios,
jobs y revisiones Cloud Run y la org policy impide una adjunción
cross-project. También prevalida y revalida las políticas IAM completas de las
identidades runtime y su autoridad efectiva por componente, incluidos caminos
de impersonación.

Ese pool y sus service accounts de release todavía no existen en GCP. En GitHub,
`main` exige exactamente `Pymes V3 validate` para todos, no exige reviewers de
PR y conserva enforcement administrativo, historial lineal y resolución de
conversaciones. Los environments `stg` y `prd` existen y sólo aceptan `main`;
PRD exige reviewers de despliegue e impide autoaprobación, pero todavía permite
bypass administrativo. Falta además
`PYMES_GITHUB_RELEASE_AUDIT_TOKEN` en ambos environments. Por eso el bootstrap
WIF debe permanecer bloqueado hasta desactivar ese bypass, cargar la credencial
de auditoría y volver a verificar los controles V3.

Antes de autenticar contra WIF, cada release comprueba que `main` esté
protegida para todos por el único check `Pymes V3 validate` y que ambos
environments sólo acepten `main`; PRD exige deployment reviewers, prohíbe
autoaprobación y bypass administrativo. Como el token nativo de Actions no dispone de
`Administration:read`, cada environment guarda una credencial de auditoría
separada del token nativo. El workflow ejecuta la auditoría completa al validar
la release y otra vez inmediatamente antes de autenticar el deployer. La
configuración se prepara con `bootstrap-github-environments.sh`; ese bootstrap
rechaza reglas desconocidas antes de hacer PUT y el bootstrap de identidades
falla antes de mutar GCP si el environment objetivo no coincide con la
política.

La validación es continua. El job protegido `validate`, inmediatamente antes de
permitir el builder, autentica al deployer exacto del entorno y verifica tanto
builder como deployer: pool/provider WIF, cuenta habilitada, cero claves
administradas, policy completa con un único subject, roles directos de proyecto
y Artifact Registry, Secrets, runtime identities keyless, KMS y los once
recursos Run. IAM Policy Analyzer debe quedar completamente explorado, sin
group edges, impersonación residual ni autoridad efectiva fuera de los recursos
allowlisted. El job `deploy` repite el gate después del build y antes de leer
attestations o mutar. El rol custom sólo agrega las lecturas exactas necesarias;
no contiene secretos, uso criptográfico ni mutaciones. El job de build empieza
rechazando todo `GITHUB_RUN_ATTEMPT != 1`, antes de autenticar al builder; el
job de deploy aplica el mismo rechazo como primer paso, antes de checkout,
descarga del manifiesto o autenticación. Para reintentar se dispara una release
nueva; no se reejecuta un job ni un workflow que pudiera conservar validación o
artefactos de un intento anterior.

Cloud Asset Search y Policy Analyzer tienen consistencia eventual. Un resultado
`fullyExplored` no demuestra que un grant creado segundos antes ya sea visible.
Por eso las políticas de los recursos conocidos se releen directamente y,
antes del seed, retiro legado o cierre, debe existir una ventana operativa sin
mutaciones seguida de una segunda auditoría estable del scan WIF y del
inventario de workloads. La espera de diez minutos más la doble lectura de
veinte segundos implementada por `finalize` cubre Admin Activity del seed; no
convierte los demás índices de Cloud Asset en lecturas instantáneas.

Las identidades históricas no pueden quedar como ruta alternativa permanente.
Después de finalizar STG, un canary `operational` debe ejecutar el SHA actual
exacto de `main`, su árbol completo y el mismo workflow revisado. Antes del
corte se comprueban las autorizaciones exactas de las identidades nuevas, IAM
Policy Analyzer completamente explorado, ausencia de bindings `group:` en
ancestros cuya membresía no pueda demostrarse, cero autoridad efectiva de la
cuenta compartida sobre Pymes e inventario global sin workloads de la cuenta
exclusiva. Sólo entonces se elimina el principal legado exacto de las dos
políticas revisadas y se deshabilita la cuenta exclusiva. Dos
`SetIAMPolicy` exitosos sin el principal y el `DisableServiceAccount`
posterior, realizados por el mismo actor, forman el marcador durable. Un
segundo canary `operational` del mismo SHA debe comenzar después de ese
marcador; la fase `close` repite Cloud Asset y toda la auditoría. Este cutover
es STG-first y bloquea si la service account deployer PRD ya existe; `prepare`
y `finalize` de PRD se ejecutan sólo después de `close`. Este cutover todavía
no se ejecutó.

El workflow valida primero que el SHA tenga CI V3 verde, construye imágenes
linux/amd64 con SBOM/provenance, las resuelve a digest y genera un manifiesto.
No ejecuta ese archivo: sólo parsea una allowlist de claves y valida cada
referencia contra Artifact Registry. PRD exige la confirmación exacta
`DEPLOY_PRD`.

STG y PRD parten del mismo SHA Pymes, pin OA, Dockerfiles, lockfiles y receta.
No se afirma que sus digests sean iguales: la metadata de ambiente y la
publishable key Clerk de la Web forman parte del build actual y producen
artefactos específicos de cada entorno.

## Firma interna con Cloud KMS

El código no crea recursos al arrancar. Un operador prepara STG o PRD con:

```bash
PYMES_KMS_BOOTSTRAP_ENV=stg \
./scripts/deploy/bootstrap-internal-identity.sh
```

El script idempotente crea, si falta, la clave asimétrica regional y concede
firma y lectura de clave pública exactamente a API, worker y provisioner.
Imprime el nombre no secreto de la versión primaria; el deploy debe copiar ese
recurso completo a
`PYMES_INTERNAL_KMS_KEY_VERSION`. No se acepta `primary`, `latest` ni el nombre
de una `CryptoKey` sin versión.

`cmd/internal-jwks` usa el cliente Go oficial para leer las claves públicas,
verificar nombre, algoritmo y CRC32C, derivar un `kid` estable y emitir JWKS. El
deploy lo ejecuta siempre y rechaza un `PYMES_INTERNAL_JWKS_JSON` precalculado
si no coincide byte a byte. La identidad que ejecuta el deploy necesita ADC y
permiso `cloudkms.cryptoKeyVersions.viewPublicKey`; no necesita firmar.

## Clerk

STG usa la instancia de desarrollo existente de la aplicación Pymes. Su claim
`aud` conserva `pymes-v2-api` y agrega `pymes-v3`, por lo que v2 no se corta
durante el piloto. PRD usa la instancia `production` de la misma aplicación:
organizaciones y slugs están habilitados, los roles iniciales son
`org:admin`/`org:member`, se exige organización activa y `aud=pymes-v3`.
Las claves de backend de ambos entornos tienen una versión habilitada en Secret
Manager; no se materializan en archivos del repositorio.

El primer alta de Cloud Run resuelve el ciclo entre URL y webhook mediante dos
stages explícitos:

- `PYMES_DEPLOY_STAGE=bootstrap` sólo acepta STG, Fiscal `mock`, PerGo y Google
  deshabilitados, el origen reservado
  `https://pymes-v3-stg-bootstrap.invalid` y el label real
  `lifecycle=bootstrap-temporary` en el secreto de webhook. Ejecuta migraciones,
  crea las seis revisiones candidatas con tag y tráfico cero, mantiene API y
  Web con ingress interno y sin invocadores públicos, mantiene el worker con
  escala mínima cero, verifica `pretraffic` y termina sin ejecutar
  `update-traffic`. Antes de terminar retira y comprueba la ausencia de todos
  los tags: deja revisiones inertes sin tráfico, no una URL candidata durable.
  Para no convertir bootstrap en un mecanismo de corte, falla antes de mutar
  si cualquiera de los seis servicios ya tiene tráfico activo; sólo permite
  servicios ausentes o todavía al 0 %.
- `PYMES_DEPLOY_STAGE=operational` es el valor por defecto. Exige el origen
  público real dentro de las authorized parties, rechaza el label temporal y,
  después del chequeo `pretraffic`, recién puede promover la release. API y Web
  reciben la misma capability efímera por ejecución y el tag candidato:
  cualquier request directo a esos hosts sin
  `X-Pymes-Preflight-Token` exacto responde `404`; Nginx agrega el valor sólo a
  su llamada interna a la API candidata. Al finalizar conserva sólo el tag del
  release activo de API requerido por Web; Fiscal, Accounting, Accounting
  Admin, Worker y Web quedan sin tags. La verificación activa rechaza tags
  residuales y prueba que las URLs retiradas ya respondan `404`.

En ambos stages el candidato del worker usa mínimo de revisión `0`,
`--no-deploy-health-check` y escalado manual `0`, de modo que crear o verificar
la revisión no inicia el `Runner`. Operational la promueve última: enruta
primero el 100 % todavía en `0`, cambia a `1` y espera un log
`worker_release_ready` de esa revisión y SHA. El worker sólo emite esa señal
después de completar la primera lectura durable de su `MetricsReader`; no es un
simple mensaje de proceso iniciado. Cualquier salida fallida vuelve a `0` antes
de restaurar tráfico y elimina la candidata; si no había una revisión activa
anterior, elimina el servicio exacto después de deshabilitarlo.

Antes de crear candidatos, el script resuelve exactamente una revisión activa
por servicio. En particular exige que `PYMES_API_UPSTREAM` del Web anterior
apunte a la URL de un único tag de la API anterior y que ambos conserven la
misma capability histórica. Un error de `list` o `describe` nunca se interpreta
como recurso ausente. En rollback se restaura primero ese tag API, se prueba
por TLS con su capability y recién después se devuelve Web. Todo deploy de
servicio usa el chequeo IAM de invocación de Cloud Run y los readbacks exigen
ingress e invokers exactos. La verificación activa se ejecuta dentro de la
transacción, antes de desarmar el trap de rollback.

`PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN` existe sólo para los tests
locales de política. Un deploy real y `verify-cloud-run.sh` siempre leen el
label del secreto directamente desde Secret Manager y rechazan esa variable.

Faltan antes de una release operacional:

- URL pública de cada BFF y dominio real de PRD; el dominio Clerk de producción
  sigue siendo un placeholder y no se inventa uno;
- endpoint Clerk inicialmente deshabilitado en
  `PUBLIC_BASE/api/v1/webhooks/clerk`, suscripciones de organización y
  membresía, y su secreto Svix real cargado como una nueva versión;
- URL HTTPS y workspace PerGo por entorno; cada organización configura sólo
  canal e identidad no secreta del remitente, mientras API y worker reciben
  secretos técnicos distintos y el callback único es
  `/api/v1/webhooks/pergo`;
- imágenes publicadas y servicios Cloud Run que usen las identidades indicadas;
- jobs Cloud Run de migración/provisionamiento y Monitoring;
- credenciales de organizaciones piloto para PerGo, Google y ARCA.

El despliegue queda codificado en
[`scripts/deploy/cloud-run.sh`](../scripts/deploy/cloud-run.sh). Rechaza un
servicio si falta cualquier versión de secreto, usa Cloud SQL compartido y
permite Fiscal `mock` o `arca` de forma explícita. Antes de los servicios, ejecuta jobs
idempotentes para las migraciones Pymes, Fiscal y Accounting. Despliega primero
Fiscal/Accounting con JWKS activo+solapado y después el worker con la nueva
versión firmante; nunca inyecta `PYMES_INTERNAL_SIGNING_SEED_B64` en producción.

API, worker y Fiscal requieren Direct VPC Egress; el job de provisionamiento
también lo usa. La red compartida ya está provisionada en la VPC `default`:
subred `pymes-v3-serverless` `10.120.0.0/24` en `us-central1`, Private Google
Access, router y Public NAT `pymes-v3-serverless`. El NAT automático cubre
`ALL_IP_RANGES` únicamente de esa subred. El bootstrap es fail-closed,
plan-only por defecto y converge de forma segura un NAT propio que hubiese
quedado limitado al rango primario; rechaza cualquier NAT compartido con otra
subred. Crear y mantener Cloud NAT tiene costo recurrente y el apply exige la
aceptación explícita versionada en el runbook.

El bootstrap de identidades se ejecuta primero con
`bootstrap-workload-identities.sh`; las claves internas se preparan con
`bootstrap-internal-identity.sh`. Para cambios PostgreSQL, ejecutar
[`scripts/deploy/bootstrap-cloudsql.sh`](../scripts/deploy/bootstrap-cloudsql.sh)
con credenciales administrativas sólo en el entorno del operador. El script
exige `PYMES_BOOTSTRAP_COMPONENTS`; tocar credenciales runtime/migración exige
además `PYMES_ROTATE_DATABASE_CREDENTIALS=true`. El alta selectiva
`accounting-admin` crea su rol/URL y revoca/verifica DDL del runtime sin rotar
las demás contraseñas. Las URLs se escriben directamente en Secret Manager; no
se imprimen ni guardan contraseñas.

A fecha de esta revisión no existe evidencia versionada de ningún servicio ni
job Cloud Run `pymes-v3` desplegado por este release.
Las service accounts runtime y las claves de identidad interna están
provisionadas. STG y PRD tienen rotación simétrica completa y cada uno posee su
secreto HMAC de Agenda en versión 1 con acceso mínimo. Faltan valores reales de
Clerk webhook, PerGo y Google; la versión temporal STG no cuenta como valor
real. También faltan el pool y las tres identidades WIF de release, el cierre
de la reconciliación de GitHub mediante la desactivación del bypass
administrativo de PRD y el retiro recuperable de los contenedores obsoletos
`fiscal-credential` e `internal-jwt-seed`. No se afirma ningún despliegue ni
piloto hasta que el stage operacional compare digest, revisión, IAM, red,
secretos, pin Web → API, señal durable del worker y release marker, y termine
con tráfico activo verificado.
