# Seguridad y entornos GCP de Pymes v3

Fecha de provisión: 2026-07-31.

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

| Entorno | CMEK de secretos | Firma de identidad interna | Workload identities |
|---|---|---|---|
| STG | `pymes-v3-stg/secrets` | `pymes-v3-stg/internal-jwt-signing/cryptoKeyVersions/1` | `pymes-v3-{api,worker,provision,fiscal,accounting,accounting-admin}-stg` más identidades de migración |
| PRD | `pymes-v3-prd/secrets` | `pymes-v3-prd/internal-jwt-signing/cryptoKeyVersions/1` | `pymes-v3-{api,worker,provision,fiscal,accounting,accounting-admin}-prd` más identidades de migración |

Las claves `secrets` son simétricas de software, regionales y protegen las
réplicas de Secret Manager. Su único principal criptográfico directo es el
service agent de Secret Manager. La clave `internal-jwt-signing` es asimétrica
`EC_SIGN_ED25519`, distinta por entorno: sólo worker y provisioner reciben
`roles/cloudkms.signer` y `roles/cloudkms.publicKeyViewer` sobre esa clave. API,
Fiscal y Accounting no reciben permisos de firma ni acceso a material privado;
validan el JWKS publicado durante el despliegue. Las identidades conservan
`roles/cloudsql.client` para abrir el socket de Cloud SQL, lo que no concede
credenciales ni permisos SQL.

## Referencias de secretos

Cada nombre existe como secreto global, con una única réplica en `us-central1`
cifrada por la clave KMS regional del entorno. Cloud Run no admite secretos
regionales; esta réplica única conserva la localización y evita duplicar coste
de almacenamiento:

| Consumidor | Sufijo de secreto |
|---|---|
| API | `clerk-secret-key`, `clerk-webhook-secret`, `pergo-webhook-secrets`, `database-url` |
| Worker | `worker-database-url`, `pergo-api-key` |
| Fiscal | `fiscal-credential`, `fiscal-database-url` |
| Accounting | `accounting-database-url` |
| Accounting admin | `accounting-admin-database-url` |

El nombre final sigue el patrón `pymes-v3-{stg|prd}-{sufijo}`. Las semillas
JWT que pudieran existir de una provisión anterior ya no se montan ni se migran:
después de desplegar y verificar KMS deben retirarse sus bindings y deshabilitar
sus versiones mediante un cambio operativo separado y recuperable. Nunca se
guardan secretos en Git. La semilla fija de Compose es exclusivamente local.

## Firma interna con Cloud KMS

El código no crea recursos al arrancar. Un operador prepara STG o PRD con:

```bash
PYMES_KMS_BOOTSTRAP_ENV=stg \
./scripts/deploy/bootstrap-internal-identity.sh
```

El script idempotente crea, si falta, la clave asimétrica regional y concede al
worker sólo firma y lectura de clave pública. Imprime el nombre no secreto de la
versión primaria; el deploy debe copiar ese recurso completo a
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

Faltan antes de desplegar:

- URL pública de cada BFF y dominio real de PRD; el dominio Clerk de producción
  sigue siendo un placeholder y no se inventa uno;
- endpoint público para cada webhook y su secreto de firma, que sólo pueden
  cerrarse después de desplegar el BFF;
- URL HTTPS y workspace PerGo por entorno; cada organización configura sólo
  canal e identidad no secreta del remitente, mientras API y worker reciben
  secretos técnicos distintos y el callback único es
  `/api/v1/webhooks/pergo`;
- imágenes publicadas y servicios Cloud Run que usen las identidades indicadas;
- jobs Cloud Run de migración/provisionamiento y Monitoring;
- certificado fiscal sólo al reanudar la etapa ARCA.

El despliegue queda codificado en
[`scripts/deploy/cloud-run.sh`](../scripts/deploy/cloud-run.sh). Rechaza un
servicio si falta cualquier versión de secreto, usa Cloud SQL compartido y deja
Fiscal explícitamente en modo `mock`. Antes de los servicios, ejecuta jobs
idempotentes para las migraciones Pymes, Fiscal y Accounting. Despliega primero
Fiscal/Accounting con JWKS activo+solapado y después el worker con la nueva
versión firmante; nunca inyecta `PYMES_INTERNAL_SIGNING_SEED_B64` en producción.

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

A fecha de esta revisión no existe ningún servicio ni job Cloud Run `pymes-v3`;
los recursos creados son contenedores/versions de secretos, roles/bases
lógicas, service accounts y claves KMS. Por tanto todavía no hay coste de
workloads v3 siempre encendidos.
