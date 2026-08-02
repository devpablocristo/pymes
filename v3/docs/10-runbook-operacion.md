# Runbook operativo de Pymes v3

Este runbook cubre STG y PRD en el proyecto compartido
`pymes-dev-352318`. El repositorio contiene runtime y automatización para
Fiscal `mock` o ARCA real por organización, pero el estado operativo sigue
abierto: todavía no existe evidencia de una release v3 desplegada en STG/PRD ni
de pilotos de Agenda, PerGo, Google o ARCA. El primer rollout debe usar
`PYMES_FISCAL_MODE=mock`; ARCA sólo se habilita por tenant después de completar
su onboarding y validación en homologación.

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

## Preparación y release inmutable

Los bootstrap son explícitos y se ejecutan antes de la primera release:

1. `bootstrap-github-environments.sh` protege `main` y crea/audita los
   environments `stg`/`prd`;
2. `bootstrap-workload-identities.sh` crea identidades runtime separadas por
   entorno;
3. `bootstrap-internal-identity.sh` prepara Ed25519 para API, worker y
   provisioner;
4. `bootstrap-data-encryption.sh` prepara `secrets`, `calendar-tokens` y
   `fiscal-vault`, cada una con primary habilitada, rotación de 90 días y
   principals directos exactos;
5. `migrate-regional-secrets.sh` crea contenedores globales con réplica
   `us-central1`, genera el secreto HMAC de acciones de Agenda si falta y nunca
   recrea el secreto fiscal global obsoleto;
6. `bootstrap-network-egress.sh` prepara la subred Direct VPC con Private
   Google Access y un Public NAT compartido.

A fecha de esta revisión, las tres claves simétricas ya rotan cada 90 días en
STG y PRD, y ambos secretos `scheduling-action-token-secret` existen con versión
1 e IAM mínimo. Todavía deben cargarse y validarse valores reales para Clerk
webhook, PerGo y Google. La única excepción acotada es el valor aleatorio de
bootstrap del webhook Clerk en STG: debe llevar el label
`lifecycle=bootstrap-temporary`, queda detrás de ingress interno sin invocadores
y debe rotarse antes del stage operacional.

El sexto paso genera costo recurrente. Por defecto sólo imprime el plan y no
consulta ni modifica GCP. Para aplicarlo se requieren simultáneamente
`PYMES_NETWORK_BOOTSTRAP_APPLY=true` y
`PYMES_NETWORK_COST_ACK=I_ACCEPT_RECURRING_CLOUD_NAT_COST`, después de aprobar
ese costo. El CIDR debe estar entre `/20` y `/26`; no se reutiliza una subred
incompatible.

### Primer alta STG y cierre de Clerk

`PYMES_DEPLOY_STAGE` acepta `bootstrap` u `operational` y vale
`operational` si se omite. Bootstrap existe sólo para descubrir la URL estable
de Cloud Run sin fingir que ya existe un endpoint Clerk:

1. comprobar que `pymes-v3-stg-clerk-webhook-secret` tiene una versión numérica
   habilitada, aleatoria y no reutilizada, réplica/CMEK correctos y el label
   exacto `lifecycle=bootstrap-temporary`. El valor no se imprime ni se copia a
   GitHub;
2. ejecutar manualmente `Pymes V3 Release` con `environment=stg` y
   `deploy_stage=bootstrap`. `PYMES_PUBLIC_BASE_URL` y
   `PYMES_CLERK_AUTHORIZED_PARTIES` deben estar ausentes o vacíos en esa primera
   ejecución: el script fija ambos al origen reservado no resoluble
   `https://pymes-v3-stg-bootstrap.invalid`;
3. confirmar en el log
   `BOOTSTRAP COMPLETE ... traffic=0 ingress=internal unauthenticated=denied
   worker_min=0 tag_public_access=denied promotion=skipped`. Bootstrap rechaza
   PRD, ARCA, PerGo o Google. API y Web se crean con `ingress=internal`, sin
   `allUsers`, y el worker queda en escala cero; el tag sólo identifica la
   revisión y no habilita un piloto. También rechaza el intento completo antes
   de las migraciones si cualquiera de los seis servicios ya recibe tráfico;
   sólo admite recursos ausentes o una repetición del alta que siga al 0 %;
4. obtener la URL estable, sin usar la URL taggeada del candidato:

   ```bash
   gcloud run services describe pymes-v3-stg-web \
     --project=pymes-dev-352318 \
     --region=us-central1 \
     --format='value(status.url)'
   ```

5. agregar esa URL exacta a las authorized parties de la instancia Clerk STG.
   Crear deshabilitado el endpoint
   `URL/api/v1/webhooks/clerk` y suscribir
   `organization.created`, `organization.updated`, `organization.deleted`,
   `organizationMembership.created`, `organizationMembership.updated` y
   `organizationMembership.deleted`;
6. cargar el signing secret Svix del endpoint como una versión nueva del mismo
   secreto mediante el canal seguro del operador. Verificar que `latest`
   resuelve a esa versión numérica y recién entonces quitar el label
   `lifecycle=bootstrap-temporary`;
7. configurar en el environment GitHub STG la URL real como
   `PYMES_PUBLIC_BASE_URL` e incluirla exactamente en
   `PYMES_CLERK_AUTHORIZED_PARTIES`. Ejecutar el workflow con
   `deploy_stage=operational`;
8. comprobar el stage `active`, habilitar el endpoint Clerk, enviar un evento
   de prueba y verificar una sola fila en `app.clerk_webhook_inbox`. Repetir el
   mismo evento debe ser idempotente. Reproducir desde Clerk cualquier entrega
   fallida durante la transición;
9. deshabilitar la versión temporal sólo cuando ninguna revisión activa ni
   taggeada la referencie. Eliminar el label no basta para esa comprobación.

El deploy real y el verificador leen el label desde Secret Manager. La variable
`PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN` sólo simula metadata dentro de
`cloud-run-security-check.sh`; ambos scripts la rechazan fuera de dry-run. Un
bootstrap exitoso no habilita tráfico, no habilita pilotos y no cuenta como
release STG operacional.

El bootstrap GitHub también es plan-only. Para aplicarlo exige uno a seis IDs
de usuarios colaboradores como reviewers de PRD:

```bash
PYMES_GITHUB_ENVIRONMENT_MODE=apply \
PYMES_PRD_REVIEWER_IDS=123456,789012 \
./scripts/deploy/bootstrap-github-environments.sh
```

El workflow publica el check estable `Pymes V3 validate` en cada PR y push a
`main`; el bootstrap configura la protección para exigirlo junto con review,
aprobación del último push, enforcement para administradores y políticas de
deployment limitadas a `main`. Hoy `main` sólo exige `v2-ci` para no
administradores, `stg` no tiene reglas y permite bypass, y `prd` no existe.
GitHub no expone en su REST documentado el switch de bypass administrativo: un
administrador debe desmarcarlo para PRD en Settings → Environments y luego
ejecutar:

```bash
PYMES_GITHUB_ENVIRONMENT_MODE=audit \
PYMES_PRD_REVIEWER_IDS=123456,789012 \
./scripts/deploy/bootstrap-github-environments.sh
```

La auditoría completa, ejecutada con credencial de operador, es obligatoria y
bloquea la creación o modificación del WIF si encuentra diferencias. Cada
environment debe contener el secreto
`PYMES_GITHUB_RELEASE_AUDIT_TOKEN` con `Administration:read` y la variable
`PYMES_PRD_REVIEWER_IDS` con el mismo conjunto exacto aprobado. El workflow
audita ambos environments en `validate` y repite esa auditoría inmediatamente
antes de pedir la identidad de deploy; el token nativo no puede releer todos
los detalles administrativos de la rama. No se usa **Re-run jobs** ni
**Re-run failed jobs** para una release: Build y Deploy rechazan
`GITHUB_RUN_ATTEMPT != 1`, y Deploy lo hace antes de checkout, descarga de
artefactos o autenticación. Todo reintento operativo comienza con un
`workflow_dispatch` nuevo.

El target no se configura mediante variables GitHub: proyecto
`pymes-dev-352318`, número `884236221349`, región `us-central1`, repositorio
Artifact Registry `pymes`, provider WIF y emails de builder/deployers son
constantes versionadas y verificadas por el workflow. Los mismos IDs exactos de
reviewers deben acompañar la preparación y finalización de las identidades.

Antes de ejecutar siquiera el plan de `bootstrap-release-identities.sh`:

1. usar un checkout limpio de la rama `main`, cuyo HEAD y árbol coincidan con
   el `main` remoto del repositorio `devpablocristo/pymes`;
2. autenticar `gcloud` directamente como `softponti@gmail.com` y declarar
   `PYMES_RELEASE_IDENTITY_OPERATOR_EMAIL=softponti@gmail.com`;
3. dejar sin valor tanto las variables `CLOUDSDK_AUTH_*` de credenciales
   alternativas como las properties `auth/impersonate_service_account`,
   `auth/credential_file_override`, `auth/access_token_file` y
   `auth/login_config_file`;
4. habilitar `orgpolicy.googleapis.com` y comprobar que las policies efectivas
   `iam.disableCrossProjectServiceAccountUsage` e
   `iam.disableServiceAccountKeyCreation` estén forzadas en el proyecto;
5. confirmar que el operador humano dispone temporalmente de administración de
   custom roles en la organización y de lectura/escritura IAM en organización
   `663017421195` y folder `673291958610`. Esta autoridad no se entrega al WIF:
   se necesita para crear o converger los dos roles lectores y sus bindings.

El script falla antes de mutar si no puede probar cualquiera de estas
condiciones. El bloque siguiente se ejecuta desde la raíz del checkout Pymes:

```bash
PYMES_PRD_REVIEWER_IDS=123456,789012 \
PYMES_RELEASE_IDENTITY_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_RELEASE_IDENTITY_ENV=stg \
PYMES_RELEASE_IDENTITY_APPLY=true \
PYMES_RELEASE_IDENTITY_PHASE=prepare \
./v3/scripts/deploy/bootstrap-release-identities.sh

# Desde un checkout limpio de main, construir el manifiesto fuera del repo.
# La publishable key de Clerk es pública, pero debe ser la exacta de STG.
release_sha="$(git rev-parse HEAD)"
manifest="/ruta/segura/pymes-v3-stg-${release_sha}.env"
PYMES_RELEASE_ENV=stg \
PYMES_SOURCE_SHA="$release_sha" \
PYMES_IMAGE_MANIFEST="$manifest" \
OPEN_ACCOUNTING_CONTEXT=/ruta/al/checkout/oa-fijado \
VITE_CLERK_PUBLISHABLE_KEY="$PYMES_CLERK_PUBLISHABLE_KEY_STG" \
./v3/scripts/deploy/build-push-images.sh

# El Owner preexistente revisado crea sólo 6 servicios inertes y 5 jobs no
# ejecutados. No se crea ni se revoca ningún grant temporal.
PYMES_CLOUD_RUN_SEED_APPLY=true \
PYMES_CLOUD_RUN_SEED_ENV=stg \
PYMES_CLOUD_RUN_SEED_MANIFEST="$manifest" \
PYMES_CLOUD_RUN_SEED_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_CLOUD_RUN_SEED_ACK="SEED_STG_${release_sha}" \
OPEN_ACCOUNTING_CONTEXT=/ruta/al/checkout/oa-fijado \
./v3/scripts/deploy/seed-cloud-run-resources.sh

# Copiar literalmente las seis variables PYMES_INITIAL_SEED_* impresas por el
# seed. Finalize verifica manifiesto, SHA-256, estado inerte y Audit Logs.
read -r -p 'PYMES_INITIAL_SEED_STARTED_AT: ' seed_started_at_impreso
read -r -p 'PYMES_INITIAL_SEED_COMPLETED_AT: ' seed_completed_at_impreso
read -r -p 'PYMES_INITIAL_SEED_MANIFEST_SHA256: ' manifest_sha256_impreso
PYMES_PRD_REVIEWER_IDS=123456,789012 \
PYMES_RELEASE_IDENTITY_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_RELEASE_IDENTITY_ENV=stg \
PYMES_RELEASE_IDENTITY_APPLY=true \
PYMES_RELEASE_IDENTITY_PHASE=finalize \
PYMES_INITIAL_SEED_RELEASE_SHA="$release_sha" \
PYMES_INITIAL_SEED_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_INITIAL_SEED_STARTED_AT="$seed_started_at_impreso" \
PYMES_INITIAL_SEED_COMPLETED_AT="$seed_completed_at_impreso" \
PYMES_INITIAL_SEED_MANIFEST="$manifest" \
PYMES_INITIAL_SEED_MANIFEST_SHA256="$manifest_sha256_impreso" \
./v3/scripts/deploy/bootstrap-release-identities.sh

# Primer canary STG exitoso mediante el WIF dedicado:
PYMES_STG_CANARY_RUN_ID=30700000001 \
PYMES_LEGACY_WIF_MODE=apply \
./v3/scripts/deploy/retire-legacy-pymes-wif.sh

# Segundo canary, ejecutado después del retiro:
PYMES_STG_CANARY_RUN_ID=30700000001 \
PYMES_STG_POST_RETIRE_CANARY_RUN_ID=30700000002 \
PYMES_LEGACY_WIF_MODE=audit \
./v3/scripts/deploy/retire-legacy-pymes-wif.sh

PYMES_PRD_REVIEWER_IDS=123456,789012 \
PYMES_RELEASE_IDENTITY_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_STG_CANARY_RUN_ID=30700000001 \
PYMES_STG_POST_RETIRE_CANARY_RUN_ID=30700000002 \
PYMES_RELEASE_IDENTITY_APPLY=true \
PYMES_RELEASE_IDENTITY_PHASE=close \
./v3/scripts/deploy/bootstrap-release-identities.sh

# PRD se prepara sólo después de cerrar STG:
PYMES_PRD_REVIEWER_IDS=123456,789012 \
PYMES_RELEASE_IDENTITY_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_RELEASE_IDENTITY_ENV=prd \
PYMES_RELEASE_IDENTITY_APPLY=true \
PYMES_RELEASE_IDENTITY_PHASE=prepare \
./v3/scripts/deploy/bootstrap-release-identities.sh

# Repetir build + seed para PRD desde el mismo SHA/materiales, usando la
# publishable key PRD y un manifiesto PRD propio. Luego finalizar con las cinco
# evidencias exactas impresas por ese seed:
prd_manifest="/ruta/segura/pymes-v3-prd-${release_sha}.env"
PYMES_RELEASE_ENV=prd \
PYMES_SOURCE_SHA="$release_sha" \
PYMES_IMAGE_MANIFEST="$prd_manifest" \
OPEN_ACCOUNTING_CONTEXT=/ruta/al/checkout/oa-fijado \
VITE_CLERK_PUBLISHABLE_KEY="$PYMES_CLERK_PUBLISHABLE_KEY_PRD" \
./v3/scripts/deploy/build-push-images.sh
PYMES_CLOUD_RUN_SEED_APPLY=true \
PYMES_CLOUD_RUN_SEED_ENV=prd \
PYMES_CLOUD_RUN_SEED_MANIFEST="$prd_manifest" \
PYMES_CLOUD_RUN_SEED_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_CLOUD_RUN_SEED_ACK="SEED_PRD_${release_sha}" \
OPEN_ACCOUNTING_CONTEXT=/ruta/al/checkout/oa-fijado \
./v3/scripts/deploy/seed-cloud-run-resources.sh
read -r -p 'PYMES_INITIAL_SEED_STARTED_AT PRD: ' prd_seed_started_at_impreso
read -r -p 'PYMES_INITIAL_SEED_COMPLETED_AT PRD: ' prd_seed_completed_at_impreso
read -r -p 'PYMES_INITIAL_SEED_MANIFEST_SHA256 PRD: ' prd_manifest_sha256_impreso
PYMES_PRD_REVIEWER_IDS=123456,789012 \
PYMES_RELEASE_IDENTITY_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_RELEASE_IDENTITY_ENV=prd \
PYMES_RELEASE_IDENTITY_APPLY=true \
PYMES_RELEASE_IDENTITY_PHASE=finalize \
PYMES_INITIAL_SEED_RELEASE_SHA="$release_sha" \
PYMES_INITIAL_SEED_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_INITIAL_SEED_STARTED_AT="$prd_seed_started_at_impreso" \
PYMES_INITIAL_SEED_COMPLETED_AT="$prd_seed_completed_at_impreso" \
PYMES_INITIAL_SEED_MANIFEST="$prd_manifest" \
PYMES_INITIAL_SEED_MANIFEST_SHA256="$prd_manifest_sha256_impreso" \
./v3/scripts/deploy/bootstrap-release-identities.sh
```

`bootstrap-release-identities.sh` también es plan-only por defecto. Al
aplicarlo crea un pool WIF dedicado a releases, limitado al repositorio Pymes,
`refs/heads/main` y los environments protegidos `stg`/`prd`. La topología final
usa tres service accounts diferentes: un builder y un deployer para cada
entorno. Sin embargo, el postcondition es phase-aware: antes de cerrar STG
exige exactamente Build+STG y cero trust PRD; después del cierre exige
Build+STG+PRD. El builder sólo publica en el Artifact Registry seleccionado; el
deployer puede leer los digests/metadata necesarios, impersonar las identidades
runtime exactas y administrar Cloud Run, sin reutilizar la identidad CI
compartida.
Tanto `prepare` como `finalize` exigen exactamente un
`PYMES_RELEASE_IDENTITY_ENV`; no inspeccionan ni conceden permisos al otro
entorno. Antes del primer grant, el bootstrap exige que las service accounts
de release estén habilitadas, sin claves administradas por usuarios, con policy
vacía o el único binding WIF esperado y sin roles preexistentes fuera de la
allowlist.

La cadena de ancestros debe ser exactamente proyecto `pymes-dev-352318`,
folder `673291958610` y organización `663017421195`. Builder no recibe rol de
ancestro. El deployer sólo recibe, sin condición, los roles
`pymesV3ReleaseFolderIamRead` y
`pymesV3ReleaseOrganizationIamRead` sobre sus scopes exactos. Los roles se
comparan permiso por permiso; un rol ampliado, otro binding de una identidad
Pymes, un principal federado, grupo, dominio o autoridad no demostrablemente de
sólo lectura bloquea. Como el workflow no administra custom roles de
organización, su alta o corrección debe efectuarla el operador humano de
bootstrap con permisos administrativos explícitos y luego retirarlos según la
política de acceso privilegiado.

El gate requiere `orgpolicy.googleapis.com` y relee las policies efectivas
`iam.disableCrossProjectServiceAccountUsage` e
`iam.disableServiceAccountKeyCreation`; ambas deben contener una única regla
`enforce: true`. Además ejecuta IAM Policy Analyzer en el proyecto con
expansión de grupos, roles e impersonación: la respuesta debe estar
completamente explorada. La validación inversa consulta permisos sensibles en
chunks y compara triples recurso/permiso/identidad exactos para Project,
release/runtime service accounts, Run, Secrets, Artifact Registry y las cuatro
CryptoKeys. Rechaza membresía indirecta, roles custom o alternativos, autoridad
heredada, grants cross-environment y caminos de impersonación aunque el nombre
del rol aparente ser inocuo. Un error, una respuesta parcial o cuota persistente
después del retry acotado bloquean.

Un barrido por el path completo de `pymes-v3-release-pool` rechaza cualquier
binding directo `principal://` o `principalSet://`; las únicas excepciones son
los subjects exactos de la fase con `roles/iam.workloadIdentityUser` en su
service account destino: Build+STG y cero PRD antes de `close`,
Build+STG+PRD después. También se releen directamente proyecto, Artifact
Registry, Secrets, service accounts runtime, KMS y, en `finalize`, cada
service/job de Cloud Run. Builder y deployers deben estar keyless y sin
adjunción a workloads:
se combina Cloud Asset con listados directos de servicios, jobs y revisiones
Cloud Run, mientras la org policy impide adjuntarlos desde otro proyecto.

Esas condiciones se revalidan en cada release. Antes de autenticar al builder,
el job `validate` usa el deployer protegido del entorno para comprobar el
pool/provider, builder y deployer activos, keyless y con trust completo exacto;
roles de proyecto/Artifact Registry; catorce Secrets; diez cuentas runtime
habilitadas y keyless; KMS; y los once recursos Run. Policy Analyzer debe estar
completamente explorado, sin group edges, impersonación residual ni pares
efectivos heredados fuera de la allowlist. Para cada runtime SA, la policy
entrante debe contener sólo el `roles/iam.serviceAccountUser` del deployer del
entorno y su autoridad efectiva debe coincidir con la allowlist del componente:
SQL, Secrets, KMS e invocaciones privadas estrictamente necesarias. Después del
build, `deploy` repite el mismo verificador antes de attestations y cualquier
mutación. Un permiso de lectura denegado es un fallo, no un warning.

Los primeros pasos de `build` y `deploy` rechazan
`GITHUB_RUN_ATTEMPT != 1`; Build lo hace antes de autenticar al builder y
Deploy antes de checkout, artifacts o autenticación. No usar “Re-run failed
jobs” ni “Re-run all jobs” para una release: se debe iniciar un nuevo
`workflow_dispatch`, que vuelve a ejecutar validación, aprobación y auditoría
de autoridad desde cero.

Cloud Asset Search y Policy Analyzer son eventualmente consistentes.
`fullyExplored` describe el snapshot consultado, no prueba que un binding o
workload creado instantes antes ya esté indexado. Antes de `finalize`, del
retiro WIF y de `close`, congelar cambios IAM/workloads, esperar el período
operativo acordado y repetir el scan de principals WIF y el inventario de
adjunciones hasta obtener dos evidencias estables. No avanzar ante diferencias
ni tratar una API deshabilitada como ausencia de recursos. Los diez minutos y
las lecturas separadas por veinte segundos que siguen cubren específicamente
Admin Activity del seed inicial; no sustituyen esta precaución para Cloud Asset.

Después de `prepare` para STG, el Owner directo preexistente y revisado
`softponti@gmail.com` crea únicamente los once recursos iniciales mediante
`seed-cloud-run-resources.sh`. El proceso no agrega ni revoca autoridad humana:
usa el acceso que ya existe, exige el head protegido de `main`, CI V3 verde,
imágenes con attestation, consentimiento ligado al SHA y registra el instante de
inicio. Los seis servicios quedan internos, sin IAM, tráfico, instancias,
secretos, SQL ni red; los cinco jobs quedan sin IAM, secretos, SQL, red ni
ejecuciones. `finalize` exige las seis evidencias impresas, compara el SHA-256
del manifiesto, vuelve a verificar cada imagen e identidad runtime y revisa
Admin Activity desde ese instante. Rechaza más de mil eventos, cualquier
mutación ajena a Cloud Run salvo el evento
`iam.serviceAccounts.actAs` exitoso que el propio alta produce sobre una de las
diez service accounts runtime exactas, cualquier FQN fuera del proyecto,
`us-central1`, tipo y allowlist y toda ejecución o borrado. El evento `actAs`
debe conservar permiso, `granted`, respuesta y recurso de autorización
coincidentes; otra escritura IAM falla cerrado. Nunca puede persistir un
`roles/run.admin` de Pymes a nivel proyecto: se concede sólo sobre los seis
servicios y cinco jobs exactos. La ventana debe contener exactamente una
creación/replace inicial por cada FQN: una segunda mutación sobre el mismo
recurso o `SetIamPolicy` invalida el seed aunque el readback final sea inerte.
Por eso el seed no usa `--allow-unauthenticated` ni su variante negativa:
mantiene el default privado y verifica IAM vacío antes y después, incluso al
retomar un intento parcial.
La ventana toma `PYMES_INITIAL_SEED_COMPLETED_AT` más dos minutos como límite
superior exclusivo para cubrir timestamps fraccionarios y reloj; hay que esperar
diez minutos antes de finalizar y el gate exige que dos lecturas separadas por
veinte segundos sean idénticas.
Recién entonces se ejecuta el primer canary.

El retiro legado exige un run `operational` exitoso del SHA actual exacto de
`main`: comprueba commit, árbol completo y blob del workflow contra el checkout
local limpio. Es un corte STG-first: exige builder y deployer STG exactos y
finalizados, y falla si la service account deployer PRD ya fue provisionada.
El subject PRD puede existir en la condición del provider, pero no tiene cuenta
destino hasta después de `close`. Antes de mutar, verifica las autorizaciones
directas exactas de las identidades nuevas y una consulta IAM Policy Analyzer
completamente explorada. También exige que la cuenta compartida no conserve
autoridad efectiva —directa, por recurso, grupo o
impersonación— sobre Pymes. Como Policy Analyzer sólo es utilizable en el
scope del proyecto con el operador actual, el corte falla cerrado ante cualquier
binding `group:` en políticas de ancestros: no presume que la cuenta compartida
esté fuera de un grupo cuya membresía no puede probar. Cloud Asset más
inventarios directos por producto descartan claves o workloads del proyecto en
cualquier región; una API deshabilitada hace fallar la prueba en lugar de
interpretarse como inventario vacío. La policy efectiva
`iam.disableCrossProjectServiceAccountUsage` debe impedir adjunciones desde
otros proyectos. Debido a la consistencia eventual de Cloud Asset, el cambio se
ejecuta sólo después de una ventana sin mutaciones y de repetir esos inventarios
hasta conservar evidencia estable.

`apply` reescribe las dos políticas IAM completas preservando sus `etag`,
elimina únicamente el principal legado exacto y luego deshabilita la cuenta
exclusiva. El marcador durable se deriva de los dos eventos auditados
`SetIAMPolicy` exitosos cuyos requests ya no contienen ese principal y de un
evento `DisableServiceAccount` posterior, todos ejecutados por el mismo actor.
`audit` exige un segundo canary `operational`, distinto pero del mismo SHA y
árbol, iniciado después del marcador; el primero debe haber terminado antes.
`close` repite la auditoría antes de declarar terminada la transición. Los
roles de proyecto de la cuenta dedicada quedan inertes mientras está
deshabilitada y sólo se limpian en un cambio posterior reversible; nunca se
alteran los roles de la cuenta compartida. PRD se crea/finaliza después de este
cierre: `prepare prd`, bootstrap inicial revisado y `finalize prd`, todos
mediante `PYMES_RELEASE_IDENTITY_ENV=prd`.

La release autoritativa es el workflow manual `Pymes V3 Release`:
su nombre de run incluye environment, `deploy_stage` y SHA. Sólo
`Pymes V3 stg operational @ <sha>` puede servir como canary; una ejecución
`bootstrap` crea candidatos sin tráfico y nunca prueba el camino operacional.

1. rechaza un ref distinto de `main`, exige un run exitoso de `Pymes V3 CI`
   para el SHA exacto, valida `deploy_stage`, verifica la política completa de
   `main`, STG y PRD y requiere escribir `DEPLOY_PRD` para producción;
2. hace checkout del SHA de Pymes y del pin completo de Open Accounting;
3. construye desde worktrees limpios, publica SBOM y provenance y genera un
   manifiesto allowlisted con imágenes sólo por `@sha256`;
4. repite la auditoría completa de GitHub inmediatamente antes de autenticar el
   deployer;
5. ejecuta migraciones y crea servicios candidatos con tráfico cero y el label
   `pymes-v3-release=<sha-fuente>`. Cada deploy fuerza el invoker IAM check de
   Cloud Run y el readback posterior valida ingress e invokers. Antes de crear
   candidatos, resuelve de forma exacta los baselines activos: un error de
   lectura no cuenta como ausencia y Web debe apuntar al tag y capability de la
   API anterior exacta. Antes de crear el candidato del worker, pone ese
   servicio en escalado manual `0`; su revisión declara `min-instances=0` y
   omite el deployment health check, por lo que el `Runner` no puede consumir
   leases durante pretraffic;
6. genera una capability efímera aleatoria de 32 bytes, la enmascara en GitHub
   Actions y la inyecta en las revisiones API y Web junto al tag candidato
   mediante un archivo de variables efímero con modo `0600`, nunca mediante
   `argv`. El verificador aplica el mismo criterio a `curl`.
   Ejecuta `verify-cloud-run.sh` en fase pretraffic. En `bootstrap` exige API y
   Web internas, sin invocadores públicos, y termina dejando las seis candidatas
   al 0 % y sin ningún tag;
7. sólo en `operational` abre API/Web y mueve cada servicio a exactamente
   100 % de la revisión candidata. Los hosts taggeados de API/Web devuelven
   `404` sin `X-Pymes-Preflight-Token`; Nginx agrega la capability sólo al
   upstream API candidato y el verificador demuestra ambos caminos. El worker
   se promueve último: primero se enruta su revisión mientras el servicio
   continúa en escalado `0`, se activa con escalado manual `1` y se espera el
   log `worker_release_ready` ligado a revisión y SHA, que sólo aparece después
   de la primera lectura durable del worker. Luego verifica otra vez la
   revisión activa dentro del mismo paso y antes de desarmar el rollback. Si
   una promoción falla, detiene el worker en `0`, restaura la revisión anterior,
   elimina el candidato y sólo entonces reactiva una instancia. Sin revisión
   anterior elimina el servicio worker del primer despliegue después de fijarlo
   en `0`; los demás servicios restauran tráfico en orden inverso o fallan
   cerrado cambiando ingress a interno, retirando invokers y comprobando el
   estado antes de eliminar el primer despliegue. Después de promover, conserva
   exactamente un tag en API —el del release activo que consume Web— y elimina
   todos los tags de Fiscal, Accounting, Accounting Admin, Worker y Web. Si hay
   rollback, restaura y prueba primero el tag API de la revisión anterior para
   que el Web anterior no pierda su upstream y elimina cada tag candidato. La
   limpieza no se limita al control plane: cada URL retirada debe responder
   `404`, incluida la API anterior y la Web candidata.

No se despliega manualmente un tag, `latest` ni un manifiesto editado. Hasta
que ese workflow termine y el verificador pase, H8 está implementado pero no
operado.

No se agrega un paso independiente de “verificación posterior” después de
`cloud-run.sh`: la verificación activa forma parte de la transacción que aún
puede ejecutar rollback. `Deploy exact image digests` es el último paso del job
de deploy. Tampoco se reejecutan Build o Deploy de forma aislada; ambos tienen
su guarda de `GITHUB_RUN_ATTEMPT` como primer paso y cualquier reintento exige
un nuevo `workflow_dispatch`.

El manifiesto debe conservarse como evidencia durable de release. El workflow
retiene actualmente su artifact durante 90 días; si la política de auditoría
exige una ventana mayor, debe copiarse además a almacenamiento inmutable antes
de que venza. STG y PRD usan el mismo SHA, pin OA y receta, pero el build actual
incluye metadata del ambiente y una publishable key Clerk específica en Web:
se comparan materiales exactos, no se afirma igualdad de digest entre entornos.

## Web pública en el mismo origen

En stage operacional, la URL canónica es `PYMES_PUBLIC_BASE_URL`, que debe
aparecer exactamente en las authorized parties de Clerk. El navegador llama
`/api/` sobre ese mismo origen; Nginx recibe `PYMES_API_UPSTREAM` al desplegar
y actúa como proxy al BFF. El bundle no contiene una URL de API ni secretos.
El origen `.invalid` de bootstrap nunca se publica ni se conserva después de la
rotación.

Cada imagen Web recibe
`PYMES_RELEASE_MARKER=entorno:sha-fuente:sha256:digest-web`. Nginx publica ese
valor sin transformación en `X-Pymes-Release`, incluso en `/readyz`. El
verificador exige que el env runtime y el header público coincidan exactamente;
esto distingue una revisión correcta de una imagen o ruta de dominio
desactualizada. El callback Google, cuando se habilita, debe ser
`PYMES_PUBLIC_BASE_URL/api/v1/calendars/google/oauth/callback`; el callback
PerGo también se publica bajo el mismo origen.

Cada revisión API/Web conserva además `PYMES_PREFLIGHT_TAG` y una
`PYMES_PREFLIGHT_TOKEN` de 64 caracteres hexadecimales, iguales entre el par.
La capability no es una credencial de usuario ni se expone al navegador: sólo
autoriza probes del release y el salto Web candidato → API candidata. El
middleware API y Nginx aplican el gate únicamente cuando el host comienza por
el tag de esa revisión; el origen estable sigue usando Clerk/JWT normalmente.
Nunca copiar la capability a incidentes, argumentos, logs o artefactos.

## Señales y alertas

El worker escribe al iniciar y cada 60 segundos un registro JSON
`event=worker_metrics`. Sólo contiene contadores agregados y booleanos:
outbox pendiente/alquilada/reintentando/DLQ, edad del evento más antiguo,
incertidumbres fiscales, aplicaciones/reversas contables pendientes, circuitos
y readiness. No contiene identificadores de organización ni PII.

En una promoción escribe una única señal JSON
`event=worker_release_ready`, `ready=true`, `release_sha=<sha>` y
`revision=<K_REVISION>` después de que la primera recolección durable de
métricas desde PostgreSQL haya sido exitosa. El release busca el evento para el
servicio, revisión y SHA exactos. Ausencia, timeout o una primera lectura
fallida impiden cerrar la transacción; una línea de “proceso iniciado” no
reemplaza esta evidencia.

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
| Fiscal uncertain | Estado y modo del servicio Fiscal, más consulta exacta | Dejar actuar al reconciliador; nunca volver a autorizar. |
| Circuito abierto | Servicio indicado y su base | Recuperar servicio; el circuito se cierra después del cooldown y una respuesta válida. |
| Notificación incierta / backlog | Probe privado de PerGo, leases y último código estable | Recuperar PerGo; reenviar únicamente el mismo outbox, Idempotency-Key, payload y trace ID. |

## Identidad interna KMS

Esta KMS no pertenece a ARCA: autentica llamadas privadas de worker y
provisioner, y también de la API hacia Fiscal para el onboarding de
credenciales. Los certificados y claves ARCA son siempre tenant-scoped en
Fiscal; no existe una credencial fiscal global de Pymes.

Producción no admite `PYMES_INTERNAL_SIGNING_SEED_B64`. API, worker y
provisioner exigen `PYMES_INTERNAL_KMS_KEY_VERSION` con recurso numérico
completo. Antes de abrir el health server cada firmante lee la clave pública,
comprueba nombre, algoritmo
`EC_SIGN_ED25519` y CRC32C, firma un desafío con CRC32C y verifica localmente la
firma. Cualquier fallo deja caer la revisión; nunca se habilita una semilla como
fallback.

Para rotar sin cortar tokens válidos:

1. crear una nueva versión de `internal-jwt-signing`, sin deshabilitar la
   anterior;
2. fijar la nueva en `PYMES_INTERNAL_KMS_KEY_VERSION` y la anterior en
   `PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS`;
3. ejecutar `cloud-run.sh`: despliega primero Fiscal/Accounting con ambas claves
   públicas y después API, worker y provisioner firmando con la nueva;
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
el outbox permanece durable y el worker reintenta. Recuperar permisos o la API
de KMS;
no cambiar el `org_id`, no recrear eventos y no inyectar una clave local.

## Vault Fiscal y readiness

`fiscal-vault` es una CryptoKey simétrica diferente por entorno. Sólo la
service account Fiscal recibe
`roles/cloudkms.cryptoKeyEncrypterDecrypter`; el deploy falla si ese rol está
heredado desde proyecto/keyring, si aparecen principals directos adicionales,
si la primary no está habilitada o si falta la rotación de 90 días.
`calendar-tokens` queda limitado a API/worker y `secrets` al service agent de
Secret Manager bajo la misma regla.

En producción tanto `mock` como `arca` exigen `FISCAL_KMS_KEY_NAME`; la clave
local se admite únicamente en desarrollo/test y es mutuamente excluyente.
Fiscal no se declara ready sólo porque PostgreSQL responda: durante el arranque
genera una data key, cifra y descifra un test con AAD mediante Cloud KMS,
compara el plaintext en tiempo constante y borra buffers sensibles. Si la
clave, permisos, API o AAD fallan, la revisión no queda ready. ARCA agrega
patrones de issuer separados para homologación y producción; el mock no puede
recibirlos accidentalmente.

## Reconciliación

`DurableWorker.DispatchOnce` ejecuta el relay y, en cada ciclo, consulta hasta
20 ventas `fiscal_uncertain`. La consulta usa el snapshot y número originales.
Al recuperar un resultado autorizado persiste el resultado/CAE y crea una única
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
3. verificar por número exacto mediante el mismo adapter Fiscal; en ARCA nunca
   usar un número distinto;
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
| Cloud KMS interno | La revisión nueva no arranca o fallan entregas privadas; outbox crece | Restaurar API de KMS y permisos de API, worker y provisioner para la versión fijada; nunca usar semilla ni cambiar de versión sin overlap. |
| Cloud KMS Fiscal | Fiscal no supera readiness y Pymes conserva las solicitudes pendientes | Restaurar la API de KMS o el acceso directo de la identidad Fiscal a `fiscal-vault`; nunca habilitar el cifrador local en Cloud Run. |
| Fiscal | Circuito abre; autorización queda pendiente o incierta | Recuperar servicio/base; reintento o consulta exacta, nunca nuevo número. |
| Accounting | CAE puede estar persistido y posting queda pendiente | Recuperar servicio/base; reenviar mismo comando idempotente. |
| PerGo | El turno queda confirmado y la intención pendiente o incierta | Recuperar PerGo; el lease vence y se reintenta con el mismo Idempotency-Key, payload y trace ID. El ledger de ingreso devuelve el receipt original y un webhook adelantado evita regresiones de estado. |
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

Repetir de forma independiente para `fiscal` y `accounting`. El script exige
una ruta absoluta con nombre seguro, aplica `umask 077`, no sobrescribe ni el
dump ni su manifiesto y publica ambos atómicamente desde archivos temporales
con permisos `0600`. El manifiesto registra formato, servicio, base fuente,
archivo y SHA-256; conservar además timestamp, versión de migraciones e imagen
desplegada. La URI y su password se convierten a parámetros libpq en un wrapper
y nunca se pasan por `argv`. En PRD, combinarlo con backups automáticos/PITR de
Cloud SQL; el dump no reemplaza esa política.

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
SERVICE=pymes \
PYMES_RESTORE_DATABASE_URL='postgres://DESTINO_NUEVO' \
RESTORE_CONFIRMATION='RESTORE:pymes:NOMBRE_DESTINO_NUEVO' \
./scripts/restore-postgres.sh /ruta/segura/pymes.dump
```

El restore rechaza archivos o manifiestos que no sean regulares, enlaces
simbólicos, checksums inválidos, servicios cruzados, nombres inseguros y dumps
sin el marcador de schema esperado para el servicio. Siempre exige una base
nueva y vacía, sin otras conexiones, y la confirmación exacta
`RESTORE:<service>:<database>`; el nombre o un prefijo de restore nunca omiten
esa confirmación. `pg_restore` no limpia objetos existentes: se ejecuta sin
ownership, con salida ante el primer error y en una sola transacción.

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
directorio temporal con permisos restringidos. El smoke también demuestra que
fallan de forma segura los restores sin manifiesto, con dump alterado,
servicio cruzado, sin confirmación o contra un destino que ya contiene
relaciones.

## Rollback de despliegue

1. congelar deploys y anotar revisión actual, revisión candidata, imágenes y
   migraciones ejecutadas;
2. elegir el SHA de una release anterior compatible con el esquema actual;
3. descargar el artifact
   `pymes-v3-images-<entorno>-<SHA>` de la ejecución original y copiar el
   `Manifest SHA-256` desde el summary del job
   `Build and attest immutable images`. No calcular ese valor desde el archivo
   recién descargado: la comparación independiente es la que detecta una
   sustitución. Usar un checkout limpio del mismo SHA de Pymes y un checkout
   limpio de Open Accounting en el pin declarado por el manifiesto; autenticar
   Docker Buildx para leer attestations del Artifact Registry;
4. ejecutar la recuperación durable de Web/API:

   ```bash
   PYMES_DEPLOY_ENV=stg \
   PYMES_ROLLBACK_RELEASE_SHA=0123456789abcdef0123456789abcdef01234567 \
   PYMES_ROLLBACK_IMAGE_MANIFEST=/ruta/segura/pymes-v3-images.env \
   PYMES_ROLLBACK_IMAGE_MANIFEST_SHA256=<sha256-del-summary-del-build> \
   OPEN_ACCOUNTING_CONTEXT=/ruta/al/checkout/open-accounting \
   ./scripts/deploy/rollback-cloud-run.sh
   ```

   Antes de cualquier llamada a Cloud Run, el script valida checksum, forma
   completa y repositorios exactos del manifiesto, pin de Open Accounting y
   vuelve a verificar provenance, materiales y SBOM de todos sus digests con
   el mismo verificador de `v3-release.yml`. Después exige exactamente una
   revisión API y una Web con el label del SHA y que ambas imágenes sean las
   referencias exactas del manifiesto; recién entonces valida readiness,
   contenedor, identidad, ingress, IAM, marker, tag/capability y el pin exacto
   Web → API. Crea primero el tag API y lo prueba con la capability histórica
   desde un archivo temporal `0600`, mueve Web, verifica su mismo origen, mueve
   API y deja un único tag API y cero tags Web. Un timeout sólo se acepta si un
   readback prueba que la mutación solicitada sí quedó aplicada;
5. verificar los servicios privados y worker por separado; si el incidente
   requiere revertirlos, mantener worker en escala `0` hasta haber restaurado
   sus dependencias y comprobar la señal durable;
6. si ninguna revisión es compatible, corregir hacia adelante;
7. no revertir una versión KMS/JWT sin volver a publicar su clave en el JWKS
   antes del worker y sin conservar la ventana de cinco minutos; no volver a una
   contraseña de base ya revocada.

Los comandos manuales siguientes son sólo de diagnóstico; no sustituyen
`rollback-cloud-run.sh` para el par público:

```bash
gcloud run revisions list --service=pymes-v3-ENV-api --region=us-central1
gcloud run services update-traffic pymes-v3-ENV-api \
  --region=us-central1 --to-revisions=REVISION_COMPATIBLE=100
```

Para worker (sin tráfico público), detener primero el servicio con
`gcloud run services update ... --scaling=0`, enrutar la revisión anterior y
recién entonces reactivarla con `--scaling=1`. La automatización además elimina
la revisión candidata; si no existe revisión anterior elimina el servicio
después de fijarlo en `scaling=0`.
No ejecutar simultáneamente dos versiones con semántica distinta. Tras
cualquier rollback, verificar:

- `/healthz` y `/readyz`;
- que API conserve únicamente el tag del release restaurado y que Fiscal,
  Accounting, Accounting Admin, Worker y Web no conserven tags;
- que las URLs de tags retirados respondan `404`;
- si también se revirtió Worker, `worker_release_ready` de esa revisión/SHA;
  si el rollback fue sólo Web/API, release conocida y heartbeat menor a
  3 minutos del Worker que permaneció activo;
- edad de outbox decreciente;
- cero nuevas DLQ e incertidumbres;
- una sola autorización y un solo asiento por fuente.

## Cierre de incidente

Un incidente sólo se cierra cuando las señales vuelven a verde, los eventos
pendientes convergen, no hay divergencias sin explicar, se registraron replay o
restore en auditoría y se añadió un caso de regresión si hubo defecto de código.
