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
STG y PRD, ambos secretos `scheduling-action-token-secret` existen con versión
1 e IAM mínimo, las APIs Cloud Asset/Org Policy/Policy Analyzer están
habilitadas y ambas policies IAM exigidas están forzadas. La subred
`pymes-v3-serverless` (`10.120.0.0/24`) tiene Private Google Access y el Public
NAT homónimo cubre `ALL_IP_RANGES` sólo de esa subred. Monitoring ya contiene
9 métricas y 8 policies con canal por entorno, además de dashboards separados;
los uptime checks se crean cuando existan las URLs reales. Todavía deben
cargarse y validarse valores reales para Clerk webhook, PerGo y Google. La única excepción
acotada es el valor aleatorio de bootstrap del webhook Clerk en STG: debe llevar el label
`lifecycle=bootstrap-temporary`, queda detrás de ingress interno sin invocadores
y debe rotarse antes del stage operacional.

El sexto paso genera costo recurrente. Por defecto sólo imprime el plan y no
consulta ni modifica GCP. Para aplicarlo se requieren simultáneamente
`PYMES_NETWORK_BOOTSTRAP_APPLY=true` y
`PYMES_NETWORK_COST_ACK=I_ACCEPT_RECURRING_CLOUD_NAT_COST`, después de aprobar
ese costo. El CIDR debe estar entre `/20` y `/26`; no se reutiliza una subred
incompatible ni un NAT que atienda otra subred. Un rerun converge el recurso
propio desde `PRIMARY_IP_RANGE` a `ALL_IP_RANGES` y vuelve a leer la
configuración antes de declarar `NETWORK READY`.

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
`main`; el bootstrap configura la protección para exigirlo sin reviewers de PR,
con enforcement para administradores, historial lineal, resolución de
conversaciones y políticas de deployment limitadas a `main`. Actualmente esa
protección de `main` ya está aplicada; `stg` y `prd` existen y sólo aceptan
`main`, y PRD exige reviewers de despliegue e impide autoaprobación. Sigue
pendiente desactivar el bypass administrativo de PRD y cargar
`PYMES_GITHUB_RELEASE_AUDIT_TOKEN` en ambos environments. GitHub no expone en su
REST documentado el switch de bypass administrativo: un administrador debe
desmarcarlo para PRD en Settings → Environments y luego ejecutar:

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

El gate requiere `orgpolicy.googleapis.com`; `prepare` habilita únicamente las
APIs ausentes y nunca vuelve a habilitar las ya activas. Después de una
habilitación relee, con retry acotado, las policies efectivas
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
Build+STG+PRD después. Antes de crear identidades, `prepare` sólo admite un
subconjunto válido del estado previo —incluido el estado vacío inicial—; el
postcondition exige el conjunto exacto de la fase. También se releen
directamente proyecto, Artifact
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

<!-- drift:bind v3/scripts/deploy/bootstrap-release-evidence.sh -->
<!-- drift:bind v3/scripts/deploy/retain-release-manifest.sh -->
<!-- drift:bind .github/workflows/v3-release.yml -->

El manifiesto se conserva como evidencia durable antes de publicar el artifact
de GitHub de 90 días. El build llama a
`scripts/deploy/retain-release-manifest.sh`; si esa publicación falla, no
continúa. Cada ambiente usa un bucket determinístico separado con uniform
access, prevención de acceso público, sin versioning ni reglas lifecycle,
retención mínima de un año y Bucket Lock. El builder sólo puede listar el
bucket, crear objetos y leerlos; no recibe permiso para sobrescribir o borrar.

Los buckets todavía son una precondición operativa, no evidencia ya
provisionada. Prepararlos una sola vez por ambiente:

```bash
PYMES_RELEASE_EVIDENCE_ENV=stg \
PYMES_RELEASE_EVIDENCE_MODE=plan \
./v3/scripts/deploy/bootstrap-release-evidence.sh

PYMES_RELEASE_EVIDENCE_ENV=stg \
PYMES_RELEASE_EVIDENCE_MODE=apply \
./v3/scripts/deploy/bootstrap-release-evidence.sh

PYMES_RELEASE_EVIDENCE_ENV=stg \
PYMES_RELEASE_EVIDENCE_MODE=lock \
PYMES_RELEASE_EVIDENCE_LOCK_CONFIRMATION=\
LOCK_RELEASE_EVIDENCE_STG_pymes-v3-release-evidence-stg-884236221349 \
./v3/scripts/deploy/bootstrap-release-evidence.sh

PYMES_RELEASE_EVIDENCE_ENV=stg \
PYMES_RELEASE_EVIDENCE_MODE=verify \
./v3/scripts/deploy/bootstrap-release-evidence.sh
```

Repetir explícitamente para PRD con `PRD` y
`pymes-v3-release-evidence-prd-884236221349`. `apply` crea el bucket si falta,
valida su configuración si ya existe y agrega los bindings requeridos del
builder, pero deliberadamente no activa el lock. Bloquear la política de
retención es irreversible y crea un project lien; revisar proyecto, ambiente,
nombre, plazo e IAM antes de escribir la confirmación. `verify` exige el lock
ya activo.

La publicación usa un nombre ligado a ambiente, SHA fuente y GitHub run ID y
envía `--if-generation-match=0`: un segundo objeto con el mismo nombre falla en
vez de reemplazar evidencia. Después valida tamaño, metadata, fecha de
retención y generación, descarga esa generación exacta y compara el SHA-256.
El receipt `0600` registra URI, generación y vencimiento mínimo y acompaña al
artifact corto. STG y PRD usan el mismo SHA, pin OA y receta, pero el build
incluye metadata del ambiente y una publishable key Clerk específica en Web:
se comparan materiales exactos, no se afirma igualdad de digest entre entornos.

## Migración de acceso amplio a Secret Manager

<!-- drift:bind v3/scripts/deploy/migrate-project-secret-access.sh -->

[`migrate-project-secret-access.sh`](../scripts/deploy/migrate-project-secret-access.sh)
reemplaza los grants históricos a nivel proyecto por accessors directos sobre
los secretos exactos. No lee payloads ni elimina secretos, versiones,
workloads o service accounts. El modo por defecto es `plan` y no consulta GCP:

```bash
PYMES_PROJECT_SECRET_ACCESS_SCOPE=runtime \
./v3/scripts/deploy/migrate-project-secret-access.sh
```

La secuencia segura es `runtime` antes del primer despliegue STG y `github`
sólo después de completar el retiro WIF legado y comprobar que la cuenta
dedicada está deshabilitada. `apply` inventaría servicios, jobs y todas las
revisiones retenidas, concede primero cada accessor directo, relee las
policies y el inventario global y recién entonces retira
`roles/secretmanager.secretAccessor` a nivel proyecto. Para la cuenta GitHub
dedicada también retira el `roles/secretmanager.admin` redundante.

```bash
PYMES_PROJECT_SECRET_ACCESS_MODE=apply \
PYMES_PROJECT_SECRET_ACCESS_SCOPE=runtime \
PYMES_PROJECT_SECRET_ACCESS_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_PROJECT_SECRET_ACCESS_CONFIRM=MIGRATE_PROJECT_SECRET_ACCESS_RUNTIME \
./v3/scripts/deploy/migrate-project-secret-access.sh

PYMES_PROJECT_SECRET_ACCESS_MODE=audit \
PYMES_PROJECT_SECRET_ACCESS_SCOPE=runtime \
PYMES_PROJECT_SECRET_ACCESS_OPERATOR_EMAIL=softponti@gmail.com \
./v3/scripts/deploy/migrate-project-secret-access.sh
```

Después de los dos canaries y el retiro WIF, repetir con `scope=github` y
confirmación `MIGRATE_PROJECT_SECRET_ACCESS_GITHUB`. Cada `AUDIT READY` prueba
los secrets directos exactos del principal y `project_secret_access=none`.
`scope=all` existe para auditoría final; no sustituye el orden runtime primero.

## Retiro recuperable de secretos obsoletos

<!-- drift:bind v3/scripts/deploy/retire-obsolete-secrets.sh -->

Los contenedores `pymes-v3-{stg,prd}-fiscal-credential` e
`internal-jwt-seed` pertenecen a diseños reemplazados. El primero no puede
tener una versión habilitada y, antes del retiro, sólo puede conservar el
accessor incondicional de `pymes-v3-fiscal-{env}` de su mismo entorno; Fiscal
conserva las credenciales de cada tenant cifradas en su propia base. El segundo
sólo puede tener, antes del retiro, el binding incondicional
`roles/secretmanager.secretAccessor` para las cuentas API, worker y provisioner
de su mismo entorno. Cualquier otro miembro, rol, condición o JSON inesperado
bloquea el procedimiento.

Como gate independiente, el script inspecciona `includedPermissions` de cada
rol IAM de proyecto y bloquea todo principal no humano que pueda ejecutar
`secretmanager.versions.access`. La única excepción humana admitida es el
binding incondicional exacto `roles/owner` cuyo único miembro es
`user:softponti@gmail.com`.

[`retire-obsolete-secrets.sh`](../scripts/deploy/retire-obsolete-secrets.sh) es
`plan` por defecto y no consulta GCP en ese modo:

```bash
PYMES_OBSOLETE_SECRETS_ENV=stg \
./v3/scripts/deploy/retire-obsolete-secrets.sh
```

El retiro sólo se ejecuta después de conservar evidencia de una release que use
exclusivamente `internal-jwt-signing` en KMS; el script no infiere esa
precondición, pero sí demuestra que ningún workload conserva referencias a los
secretos obsoletos. Antes de cualquier mutación relee todos los servicios, jobs
y revisiones Cloud Run del proyecto y falla si cualquiera conserva una
referencia a alguno de los dos nombres. El inventario Cloud Run y el gate IAM
de proyecto se revalidan antes de procesar cada secreto y otra vez al final.
También exige el proyecto exacto `pymes-dev-352318`, región `us-central1`,
cuenta gcloud directa `softponti@gmail.com`, configuración activa sobre ese
proyecto y ausencia de impersonación, credential files, access tokens o login
configs.

Operar STG y PRD por separado permite observar cada postcondición:

```bash
PYMES_OBSOLETE_SECRETS_MODE=apply \
PYMES_OBSOLETE_SECRETS_ENV=stg \
PYMES_OBSOLETE_SECRETS_OPERATOR_EMAIL=softponti@gmail.com \
PYMES_OBSOLETE_SECRETS_CONFIRM=RETIRE_OBSOLETE_PYMES_V3_STG \
./v3/scripts/deploy/retire-obsolete-secrets.sh

PYMES_OBSOLETE_SECRETS_MODE=audit \
PYMES_OBSOLETE_SECRETS_ENV=stg \
PYMES_OBSOLETE_SECRETS_OPERATOR_EMAIL=softponti@gmail.com \
./v3/scripts/deploy/retire-obsolete-secrets.sh
```

Repetir luego con `prd` y
`RETIRE_OBSOLETE_PYMES_V3_PRD`. Existe una selección explícita `all`, cuya
confirmación es `RETIRE_OBSOLETE_PYMES_V3_ALL`, pero no sustituye la secuencia
STG primero. `apply` preflights todos los entornos elegidos antes del primer
cambio, quita cada accessor conocido de forma individual y deshabilita sólo
las versiones habilitadas de `internal-jwt-seed`. Nunca usa `set-iam-policy`,
no elimina contenedores y no destruye versiones.

Cada escritura se relee por su postcondición. Si gcloud pierde la respuesta
después de aplicar el cambio, el script acepta únicamente el estado exacto ya
alcanzado y deja una marca `RECOVERED`; un rerun no vuelve a mutar. La
postcondición auditable es: ambos contenedores preservados, cero bindings IAM,
cero versiones habilitadas y cero referencias desde servicios, jobs o
revisiones, además de cero principals no humanos con acceso heredado. La
incorporación de esta herramienta no prueba que el retiro haya sido aplicado:
la salida `AUDIT READY ... inherited_nonhuman=none cloud_run_refs=none` de cada
entorno debe guardarse como evidencia operativa H8.

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

## Validaciones live protegidas

Google real y ARCA homologación tienen workflows separados del CI
determinístico:

- `v3-google-live.yml` sólo lee una conexión, un turno y la proyección
  determinística ya existente en Google. No crea, cambia ni borra eventos;
- `v3-arca-homologation.yml` ejecuta el probe WSAA/WSFE del punto de venta,
  lo habilita después de una respuesta válida y no autoriza ningún comprobante.

Ambos jobs están fijados al environment GitHub `stg`. Antes de usar un secreto
rechazan cualquier fuente distinta de `refs/heads/main`, un SHA sin
`Pymes V3 validate` verde, un rerun o la confirmación escrita incorrecta; luego
auditan la protección completa de `main`, `stg` y `prd`. No solicitan WIF ni
reciben credenciales mediante inputs.

Preparación Google:

1. completar OAuth desde Pymes con una cuenta controlada, conexión `active` y
   feature flag tenant habilitado;
2. confirmar desde el producto un turno controlado y esperar la convergencia
   del worker;
3. obtener el ID del calendario secundario desde la cuenta Google controlada;
4. generar para el verificador un access token corto y separado, con lectura
   Calendar sobre esa cuenta. Nunca extraer ni reutilizar el grant cifrado que
   Pymes almacena;
5. cargar temporalmente en el environment STG
   `PYMES_LIVE_PILOT_API_TOKEN`,
   `PYMES_GOOGLE_PILOT_ACCESS_TOKEN` y
   `PYMES_GOOGLE_PILOT_CALENDAR_ID`. El primero es una sesión Clerk corta con
   acceso al tenant; los tres valores se cargan por canal seguro y se eliminan
   al terminar;
6. disparar desde `main`:

   ```bash
   gh workflow run v3-google-live.yml \
     --ref main \
     -f organization_id=ORG_OPACA \
     -f connection_id=UUID_CONEXION \
     -f booking_id=UUID_TURNO \
     -f expected_meet=true \
     -f confirmation=VALIDATE_GOOGLE_STG
   ```

El job exige conexión `active`, turno confirmado/atendido/completado, evento
base32hex exacto, marker privado `pymes_managed`, digest, intervalo y ETag. Si
se espera Meet, exige `hangoutsMeet`, estado `success` y una URI de video
válida. No conserva el JSON de Pymes o Google como artifact ni lo imprime.

Preparación ARCA:

1. desplegar Fiscal STG en modo `arca`, completar CSR/certificado de
   homologación para el tenant y mantener el punto de venta reservado para el
   piloto;
2. cargar temporalmente `PYMES_LIVE_PILOT_API_TOKEN` en STG con una sesión
   Clerk corta de owner/admin del tenant;
3. disparar:

   ```bash
   gh workflow run v3-arca-homologation.yml \
     --ref main \
     -f organization_id=ORG_OPACA \
     -f credential_id=fcred_CREDENCIAL_OPACA \
     -f point_of_sale=NUMERO \
     -f confirmation=VALIDATE_ARCA_HOMOLOGATION_STG
   ```

El job rechaza una credencial de producción, vencida o no lista. La única
mutación es registrar `validated_at` y `enabled=true` para ese punto de venta
de homologación después del probe; el summary registra explícitamente
`Voucher emitted: false`. No se cargan certificado, clave, CUIT, XML ni
respuestas ARCA en GitHub. Si vence cualquiera de los tokens no se usa
**Re-run jobs**: se rota el secret temporal y se crea un dispatch nuevo.

Validación local, enteramente falsa y sin proveedor:

```bash
make protected-live-validation-test
```

Un run protegido verde prueba la integración seleccionada, pero no reemplaza
el resto del piloto: aislamiento tenant, observabilidad, recuperación y
evidencia comercial se cierran por separado.

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

### Drill cloud de las tres bases

<!-- drift:bind v3/scripts/deploy/cloud-restore-drill.sh -->
<!-- drift:bind v3/scripts/deploy/cloud-restore-drill-test.sh -->

`scripts/deploy/cloud-restore-drill.sh` orquesta `plan`, `restore`, `verify` y
`cleanup` para Pymes, Fiscal y Accounting como una sola evidencia. Está
restringido al proyecto/región versionados y a la instancia
`pymes-dev-db`; exige autenticación directa del operador revisado, sin
impersonación, y una conexión administrativa por el socket exacto del Cloud SQL
Proxy. No cambia tráfico ni secretos y nunca restaura sobre
`pymes_v3_{stg,prd}`, `pymes_v3_fiscal_{stg,prd}` o
`pymes_v3_accounting_{stg,prd}`.

Preparar un ID único de 8–16 caracteres alfanuméricos minúsculos que empiece con
letra, el manifiesto durable de la release y los tres dumps con sus manifiestos:

```bash
export PYMES_RESTORE_DRILL_ENV=stg
export PYMES_RESTORE_DRILL_ID=restorea1
export PYMES_RESTORE_DRILL_SOURCE_SHA='<SHA Pymes completo>'
export PYMES_RESTORE_DRILL_ACCOUNTING_SHA='<SHA OA completo>'
export PYMES_RESTORE_DRILL_RELEASE_MANIFEST='/ruta/segura/pymes-v3-images.env'
export PYMES_RESTORE_DRILL_RELEASE_MANIFEST_SHA256='<SHA-256 independiente>'
export PYMES_RESTORE_DRILL_PYMES_BACKUP='/ruta/segura/pymes.dump'
export PYMES_RESTORE_DRILL_FISCAL_BACKUP='/ruta/segura/fiscal.dump'
export PYMES_RESTORE_DRILL_ACCOUNTING_BACKUP='/ruta/segura/accounting.dump'
export PYMES_RESTORE_DRILL_STATE='/ruta/segura/restorea1.state.json'

PYMES_RESTORE_DRILL_MODE=plan \
./v3/scripts/deploy/cloud-restore-drill.sh
```

El preflight valida las 13 claves allowlisted del manifiesto, los SHA de Pymes y
Open Accounting, y que cada dump corresponda a la base fuente y schema
esperados. `plan` no consulta GCP ni PostgreSQL y rechaza un state preexistente.
Para `restore`, configurar el proxy y las credenciales administrativas por un
canal seguro, además de tres URLs que apunten exactamente a los nombres destino
calculados:

```bash
export PGHOST=/cloudsql/pymes-dev-352318:us-central1:pymes-dev-db
export PGPORT=5432
export PGDATABASE=postgres
export PGUSER='<rol temporal con CREATEDB>'
export PGPASSWORD='<cargar por canal seguro>'
export PYMES_RESTORE_DATABASE_URL='postgres://.../pymes_v3_restore_stg_restorea1'
export FISCAL_RESTORE_DATABASE_URL='postgres://.../pymes_v3_fiscal_restore_stg_restorea1'
export ACCOUNTING_RESTORE_DATABASE_URL='postgres://.../pymes_v3_accounting_restore_stg_restorea1'

PYMES_RESTORE_DRILL_MODE=restore \
PYMES_RESTORE_DRILL_CONFIRMATION="RESTORE_CLOUD_STG_restorea1_${PYMES_RESTORE_DRILL_SOURCE_SHA}" \
./v3/scripts/deploy/cloud-restore-drill.sh
```

Antes de crear la primera base, `restore` comprueba que ninguno de los tres
destinos exista. Luego crea los tres con un ownership marker que liga ambiente,
drill y SHA, invoca `restore-postgres.sh` con la confirmación exacta de cada
servicio y deja un state privado con checksum. Si una ejecución queda en fase
`prepared`, no adoptar ni borrar bases manualmente: usar `cleanup`, que sólo
acepta targets con el marker exacto.

`verify` no incluye un validator de producción implícito. El operador debe
revisar un ejecutable regular, no symlink, de su propiedad y no escribible por
grupo/otros; registrar su checksum y pasarlo explícitamente:

```bash
export PYMES_RESTORE_DRILL_VALIDATOR='/ruta/revisada/validate-restore.sh'
export PYMES_RESTORE_DRILL_VALIDATOR_SHA256='<SHA-256 del validator revisado>'

PYMES_RESTORE_DRILL_MODE=verify \
./v3/scripts/deploy/cloud-restore-drill.sh
```

El witness sólo se acepta si liga los tres destinos y acredita migraciones,
aislamiento tenant, probes, dos reconciliaciones, cero duplicados de solicitudes
fiscales, comandos contables o asientos, y cero outbox recuperable sin
publicar. Se publica una sola vez junto con su checksum y el state pasa a
`verified`. Esto constituye evidencia del drill, no un cutover.

Finalizada la revisión, eliminar únicamente los tres destinos propios:

```bash
PYMES_RESTORE_DRILL_MODE=cleanup \
PYMES_RESTORE_DRILL_CLEANUP_CONFIRMATION="DELETE_RESTORE_DRILL_STG_restorea1_${PYMES_RESTORE_DRILL_SOURCE_SHA}" \
./v3/scripts/deploy/cloud-restore-drill.sh
```

`cleanup` vuelve a validar state, checksum, nombres y ownership marker antes de
cada `DROP DATABASE`, y conserva el state en fase `cleaned` y el witness para
auditoría. Un drill cloud real continúa pendiente hasta ejecutar este flujo
contra los tres destinos aislados y revisar su evidencia.

## Evidencia controlada de pilotos

<!-- drift:bind v3/scripts/deploy/collect-pilot-evidence.sh -->

Un piloto sólo cuenta después de ejecutar
`scripts/deploy/collect-pilot-evidence.sh`. El collector es estrictamente de
lectura: no crea turnos, mensajes, eventos ni comprobantes y no cambia Cloud
Run, Google, PerGo o ARCA. Primero se completa el flujo con una cuenta
controlada; después el collector prueba el estado terminal contra fuentes
observables y publica un bundle redactado.

Antes de cualquier llamada, el script exige simultáneamente:

- checkout en el SHA desplegado;
- manifiesto de imágenes original y su SHA-256 copiado de la evidencia
  independiente del build;
- origen público HTTPS exacto;
- directorio destino absoluto, nuevo y con basename seguro;
- tokens únicamente mediante archivos regulares `0400`/`0600`, propiedad del
  operador;
- confirmación exacta
  `COLLECT_<ENV>_<PILOTO>_<SHA>`.

Si falta una condición, no consulta GCP ni HTTP. El bundle se construye en un
directorio privado hermano y se publica mediante un único `rename` sólo después
de verificar todo. Un error elimina respuestas crudas y configuraciones
temporales; nunca deja un bundle parcial.

Las variables comunes son:

```bash
export PYMES_PILOT_ENV=stg
export PYMES_PILOT_SOURCE_SHA='<SHA completo desplegado>'
export PYMES_PILOT_PUBLIC_BASE_URL='https://ORIGEN-STG'
export PYMES_PILOT_RELEASE_MANIFEST='/ruta/privada/pymes-v3-images.env'
export PYMES_PILOT_RELEASE_MANIFEST_SHA256='<SHA-256 del summary de Build>'
export PYMES_PILOT_EVIDENCE_DIR='/ruta/privada/evidence/agenda-SHA'
```

No escribir el JWT Clerk ni el access token Google en el historial del shell.
El operador los carga por su canal seguro en archivos temporales `0600` y los
destruye al terminar. El collector no los copia al bundle.

### Agenda

La evidencia requiere dos organizaciones distintas y un turno real confirmado,
atendido, completado o no-show en cada una. Consulta ambos turnos con
credenciales tenant-scoped y repite las lecturas cruzadas bajo la otra
organización; ambas deben devolver `404`.

```bash
PYMES_PILOT_KIND=agenda \
PYMES_PILOT_CONFIRMATION="COLLECT_STG_AGENDA_${PYMES_PILOT_SOURCE_SHA}" \
PYMES_PILOT_ORGANIZATION_A='<org A>' \
PYMES_PILOT_ORGANIZATION_B='<org B>' \
PYMES_PILOT_BOOKING_A='<UUID turno A>' \
PYMES_PILOT_BOOKING_B='<UUID turno B>' \
PYMES_PILOT_BEARER_TOKEN_A_FILE='/ruta/privada/clerk-a.token' \
PYMES_PILOT_BEARER_TOKEN_B_FILE='/ruta/privada/clerk-b.token' \
./scripts/deploy/collect-pilot-evidence.sh
```

El resultado conserva horarios, timezone, duración, estado y cardinalidad/modo
de asignaciones, pero no party, cliente, contacto, notas, servicio, recurso ni
IDs en claro.

### PerGo

El runtime debe demostrar PerGo habilitado en API y worker, endpoint HTTPS real,
audience HTTPS exacta y sin path para la identidad de Cloud Run, canal
`whatsapp`/`whatsapp_cloud` y ausencia de fallback global. La audience puede ser
el origen administrado de Cloud Run aunque `PERGO_URL` use un dominio propio: no
se fuerza igualdad entre ambas. La feature del tenant debe estar activa y la
proyección pública debe haber convergido a `delivered` o `read` con un external
message ID. El collector no llama PerGo ni reenvía el mensaje; conserva endpoint
y audience únicamente como referencias SHA-256.

```bash
PYMES_PILOT_KIND=pergo \
PYMES_PILOT_CONFIRMATION="COLLECT_STG_PERGO_${PYMES_PILOT_SOURCE_SHA}" \
PYMES_PILOT_ORGANIZATION_ID='<org piloto>' \
PYMES_PILOT_NOTIFICATION_ID='<notification ID>' \
PYMES_PILOT_BEARER_TOKEN_FILE='/ruta/privada/clerk.token' \
./scripts/deploy/collect-pilot-evidence.sh
```

Teléfono, body, variables, workspace, sender y provider message ID quedan fuera;
los identificadores necesarios se conservan únicamente como referencias
SHA-256.

### Google Calendar y Meet

Además de la conexión activa y la feature tenant, este camino hace una única
lectura `GET` contra Google Calendar. Deriva el event ID determinístico desde
organización, conexión y turno, consulta ese evento exacto y exige estado
`confirmed`, solución `hangoutsMeet` y un único entry point de video bajo
`meet.google.com`. Esa llamada adicional requiere la segunda confirmación
`READ_GOOGLE_<ENV>_<SHA>`.

```bash
PYMES_PILOT_KIND=google \
PYMES_PILOT_CONFIRMATION="COLLECT_STG_GOOGLE_${PYMES_PILOT_SOURCE_SHA}" \
PYMES_PILOT_PROVIDER_CONFIRMATION="READ_GOOGLE_STG_${PYMES_PILOT_SOURCE_SHA}" \
PYMES_PILOT_ORGANIZATION_ID='<org piloto>' \
PYMES_PILOT_GOOGLE_CONNECTION_ID='<UUID conexión>' \
PYMES_PILOT_BOOKING_ID='<UUID turno>' \
PYMES_PILOT_GOOGLE_CALENDAR_ID='<calendario controlado>' \
PYMES_PILOT_BEARER_TOKEN_FILE='/ruta/privada/clerk.token' \
PYMES_PILOT_GOOGLE_TOKEN_FILE='/ruta/privada/google.token' \
./scripts/deploy/collect-pilot-evidence.sh
```

Summary, description, attendees, calendar ID, Meet URI, ETag y tokens no se
retienen; se registran sólo hechos verificables y referencias unidireccionales.

### ARCA

El collector exige Fiscal en modo `arca`, vault KMS del entorno, política de
issuer de homologación, feature tenant activa, credencial `ready` de
homologación y una venta con CAE de 14 dígitos que haya convergido a un único
asiento.

```bash
PYMES_PILOT_KIND=arca \
PYMES_PILOT_CONFIRMATION="COLLECT_STG_ARCA_${PYMES_PILOT_SOURCE_SHA}" \
PYMES_PILOT_ORGANIZATION_ID='<org piloto>' \
PYMES_PILOT_FISCAL_CREDENTIAL_ID='<fcred_ credencial opaca>' \
PYMES_PILOT_SALE_ID='<sale ID>' \
PYMES_PILOT_BEARER_TOKEN_FILE='/ruta/privada/clerk.token' \
./scripts/deploy/collect-pilot-evidence.sh
```

No retiene CUIT, razón social, certificado, serial, CAE ni IDs contables en
claro. La API pública no expone el historial de una consulta fiscal exacta; por
eso el bundle afirma autorización y contabilización observadas, pero declara
explícitamente que no prueba una consulta ARCA independiente. No se debe
convertir ese límite en una afirmación manual.

### Bundle y verificación

Cada directorio final contiene exactamente:

- `manifest.json`: timestamp UTC, SHA fuente, checksum del manifiesto, checksum
  del collector, origen y las seis revisiones/digests/identidades activas;
- `pilot.json`: aserciones redactadas propias del piloto;
- `README.txt`: frontera de lo demostrado;
- `checksums.sha256`: integridad de los tres archivos anteriores.

Verificación posterior:

```bash
(cd /ruta/privada/evidence/agenda-SHA &&
  sha256sum --check checksums.sha256)
```

Guardar el bundle junto al manifiesto original y al summary independiente del
workflow. No modificarlo, recomputar checksums ni completar campos a mano.
`make pilot-evidence-test` ejecuta los cuatro caminos y sus negativos contra
fakes determinísticos; no usa red ni cloud.

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
