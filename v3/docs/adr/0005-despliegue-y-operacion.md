# ADR 0005: bases separadas, secreto centralizado y recuperación verificable

**Estado:** aceptada.  
**Decisión:** Pymes, Fiscal y Accounting tienen despliegues y bases separados.
Accounting utiliza schemas por organización provisionados. Fiscal conserva el
vault ARCA en su propia base y lo cifra por envelope encryption con una clave
KMS distinta por entorno; Calendar usa otra clave. Secret Manager guarda
configuración técnica global, no certificados o claves privadas de clientes.
Backups, migraciones y restores se ejecutan por servicio.

La clave KMS de identidad interna es independiente de la clave fiscal:
API, worker y provisioner reciben `roles/cloudkms.signer` y
`roles/cloudkms.publicKeyViewer` únicamente sobre
`pymes-v3-{env}/internal-jwt-signing`. Fiscal y Accounting no pueden firmar;
validan el JWKS activo y solapado. Producción fija una
`CryptoKeyVersion` numérica y nunca monta una semilla JWT desde Secret Manager.

Cada entorno se construye desde el mismo SHA, pin de Open Accounting, receta y
materiales exactos, publica sus imágenes en Artifact Registry y despliega por
digest. Los digests de STG y PRD no tienen por qué coincidir: la Web incorpora
metadata y la publishable key pública de Clerk propias del entorno durante el
build. El manifiesto por entorno vincula Pymes, Open Accounting y las diez
imágenes; debe conservarse como evidencia durable y la retención actual del
artifact durante 90 días en GitHub Actions debe complementarse con
almacenamiento inmutable si la política de auditoría exige una ventana mayor.
El builder valida en vivo el IAM de cada bucket de evidencia con el custom role
de proyecto `pymesV3ReleaseEvidenceIamRead`, compuesto únicamente por
`storage.buckets.getIamPolicy` y ligado al bucket Pymes objetivo, nunca al
proyecto.
GitHub Actions usará identidades WIF separadas
para build, STG y PRD; no utilizará claves JSON persistentes. PRD requiere una
aprobación explícita de otro reviewer, impide autoaprobación y bypass
administrativo, y reconstruye desde la misma fuente y materiales validados,
no desde fuentes distintas.
La frontera reproduce el patrón project-scoped observado en Pymes v2, pero con
pool, cuentas y recursos exclusivos de v3 y sin dependencia de código, runtime
o configuración de v2.
La rama `main` exige el check único `Pymes V3 validate`, no exige reviewers y
mantiene historial lineal, resolución de conversaciones y enforcement para
administradores. Cada release vuelve a comprobar el check y el enforcement
visibles en el resumen público de la rama y las reglas del environment
seleccionado. La auditoría completa de reviews de despliegue, bypass y
allowlists se ejecuta con credencial de operador y bloquea la creación de WIF.
La automatización WIF está preparada, pero la identidad nueva todavía no está
aprovisionada. El rollout dedicado v3 es STG-first: sólo existen builder y
deployer STG hasta que un canary v3 permita ejecutar `close`; después se
provisiona el deployer PRD y las verificaciones STG aceptan y exigen exactamente
Build+STG+PRD. `close` no retira ni modifica identidades históricas que pueda
seguir usando v2.

La autoridad nueva se vuelve a auditar en cada ejecución. El bootstrap exige
un checkout limpio cuyo HEAD y árbol sean el `main` remoto exacto, la identidad
GitHub esperada y el operador directo `softponti@gmail.com`; rechaza
impersonación, credential overrides y archivos de token/configuración
alternativos de `gcloud`. La frontera normal se fija a proyecto
`pymes-dev-352318`, número
`884236221349`, región `us-central1` y recursos Pymes explícitos. Builder,
deployers, workloads, `prepare`, `finalize` y el workflow no requieren roles,
bindings ni lecturas en folder, organización u otros proyectos. La única
excepción transitoria es la auditoría humana read-only de ancestros que ejecuta
el retiro one-time del WIF legado; no crea grants ni realiza mutaciones
externas. Ese retiro y `migrate-project-secret-access.sh` no pertenecen al
release v3 y están prohibidos mientras v2 siga activo.
`gcp-target-policy.sh` es el fusible común de los scripts mutantes normales:
antes de escribir fija proyecto `pymes-dev-352318`, región `us-central1`,
Artifact Registry `pymes`, Cloud SQL `pymes-dev-db` y red `default` con
subred/router/NAT `pymes-v3-serverless`. La Org Policy API debe estar
habilitada y las constraints efectivas
`iam.disableCrossProjectServiceAccountUsage` e
`iam.disableServiceAccountKeyCreation` deben estar forzadas antes de crear o
usar las identidades de release.

Antes del builder y antes del deploy se verifican WIF, cuentas keyless, policies
directas sobre Artifact Registry, Secrets, runtime identities, KMS y Run. Un
scan del path completo del pool admite sólo el conjunto exacto de la fase:
Build+STG y cero PRD hasta cerrar STG; Build+STG+PRD después. Cada subject debe
tener `roles/iam.workloadIdentityUser` sólo sobre su service account;
cualquier `principal://` o `principalSet://` directo en otro recurso falla.
Builder y deployers tampoco pueden estar adjuntos a workloads: Cloud Asset se
combina con lecturas de servicios, jobs y revisiones Run y la constraint
cross-project cierra la ruta externa al proyecto.

Policy Analyzer debe quedar completamente explorado. La auditoría inversa
consulta permisos sensibles y compara triples recurso/permiso/identidad, por lo
que también rechaza grupos, roles custom o alternativos, cross-environment e
impersonación fuera de la allowlist project-scoped. La policy entrante de
cada runtime SA contiene únicamente el `actAs` del deployer de su entorno y su
autoridad saliente se limita por componente. El análisis es project-scoped:
evalúa IAM definido en el proyecto Pymes y en sus recursos, sin enumerar ni
afirmar cobertura de políticas externas. Build y Deploy rechazan todo
`GITHUB_RUN_ATTEMPT != 1` como primer paso, antes de checkout, artifacts o
autenticación: un reintento es siempre un nuevo dispatch con validación nueva.

Cloud Asset Search y Policy Analyzer son eventualmente consistentes:
`fullyExplored` no equivale a frescura. La decisión exige lecturas directas para
recursos conocidos y una ventana sin mutaciones con scans repetidos y evidencia
estable antes de seed o cierre; el retiro legado independiente aplicará la misma
regla sólo después de retirar v2. La espera de diez minutos y la doble lectura
de Admin Activity son una protección específica del seed, no una garantía
general sobre los índices de Cloud Asset.

El primer alta de recursos no usa un grant temporal de proyecto. Como Cloud Run
no admite una condición segura por nombre para `roles/run.admin`, el Owner
directo preexistente crea mediante un script cerrado sólo los seis servicios y
cinco jobs inertes. El seed exige `main` protegido, CI verde, manifest SHA-256,
digests atestiguados y consentimiento ligado al SHA; no ejecuta jobs ni adjunta
secretos, SQL o red. Cloud Audit Logs y el readback exacto deben pasar antes de
que `finalize` conceda Run Admin al deployer en cada recurso individual. La
auditoría liga las mutaciones al FQN exacto de proyecto, región y tipo; fuera de
Cloud Run sólo admite el `iam.serviceAccounts.actAs` exitoso sobre una identidad
runtime allowlisted que genera el propio alta. Exige exactamente una mutación
inicial por recurso y rechaza toda actualización o `SetIamPolicy` posterior;
el seed evita ambos flags de autenticación que harían que `gcloud` escriba IAM
y comprueba policy vacía antes y después;
la ventana inicio/fin debe haber asentado diez minutos y producir dos lecturas
idénticas separadas por veinte segundos, con dos minutos de margen superior
para timestamps fraccionarios y reloj.

**Consecuencia:** cada rollout crea primero una revisión candidata sin tráfico.
El alta inicial usa un stage `bootstrap` exclusivo de STG: Fiscal mock,
integraciones apagadas, secreto Clerk etiquetado
`lifecycle=bootstrap-temporary`, origen `.invalid`, API/Web con ingress interno
y ningún invocador público; además deja el worker con escala mínima cero.
Verifica configuración e IAM pero no sondea ni promueve la URL taggeada; antes
de terminar elimina todos los tags de los seis servicios. El stage rechaza
antes de mutar cualquier servicio que ya tenga tráfico activo, por lo que nunca
puede usarse para cortar un STG operacional.

`operational`, que es el valor por defecto, rechaza el label temporal, exige el
origen Clerk real, abre API/Web y promueve el 100 % sólo si el chequeo
pretraffic pasa. API y Web comparten una capability efímera de release: sus
hosts candidatos responden `404` sin el header exacto y Web lo agrega sólo al
proxy hacia su API candidata. Deploy y verificación transportan la capability
mediante archivos efímeros `0600`, sin exponerla en `argv`. Antes de mutar, la
transacción resuelve un único
baseline por servicio y exige el pin durable Web → tag/revisión API, con la
misma capability. Los errores de lectura no se interpretan como ausencia.
Todos los deploys de servicios habilitan el chequeo IAM de invocación y el
readback valida ingress e invokers exactos.

Luego fija una política exacta de tags: API conserva sólo el tag del release
activo requerido por el proxy de Web y los otros cinco servicios no conservan
ninguno. La limpieza se demuestra también en data plane: las URLs retiradas
deben responder `404`. La revisión activa se comprueba dentro de la misma
transacción, antes de desarmar el rollback. Ante un fallo se restaura y prueba
primero el tag API requerido por el Web anterior, se revierte el tráfico o se
falla cerrado. `rollback-cloud-run.sh` permite repetir esa recuperación de
Web/API por SHA a partir de metadata durable, sin depender del workspace de la
ejecución fallida. Startup/readiness usan `/readyz` y liveness usa `/healthz`.
Worker publica `/metrics` en red privada y emite cada minuto un heartbeat JSON
agregado, sin organización ni PII. Un script
idempotente crea métricas basadas en logs, alertas y dashboard por entorno.
Los replay DLQ son explícitos, tenant-scoped e insertan una auditoría inmutable
antes de mover el evento. La expiración de certificados se agrega con ARCA
real. Logs y trazas no pueden incluir PII, XML ni secretos. Los dumps se crean
con permisos restrictivos, checksum y destino absoluto; URI y password no
entran en `argv`. El restore sólo acepta una base nueva y vacía, sin otras
conexiones, con confirmación exacta y una única transacción. El procedimiento de
reconciliación, restore, caída y rollback vive en
[`10-runbook-operacion.md`](../10-runbook-operacion.md).

El worker es una excepción deliberada al health check de despliegue: antes de
crear su candidato, el servicio queda en escalado manual `0`; la revisión usa
`min-instances=0` y `--no-deploy-health-check`. Se la enruta al 100 % como
último servicio mientras sigue deshabilitada y sólo entonces se cambia a
escalado manual `1`. La release espera un `worker_release_ready` ligado al SHA
y revisión, emitido sólo después de la primera lectura durable del worker
contra PostgreSQL. Un error pretraffic elimina el candidato sin ejecutarlo;
un rollback vuelve primero a `0`, restaura la revisión anterior, elimina la
candidata y recién después reactiva una instancia. Sin baseline elimina el
servicio exacto después de fijarlo en `0`.
