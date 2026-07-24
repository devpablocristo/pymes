# Pymes v2

Runtime técnico de la nueva generación de Pymes. Parte de una base vacía y no
lee, ejecuta ni importa código de `v1/`.

## Componentes

- [`backend/`](backend/) es la API Go y su composition root.
- [`db/`](db/) contiene el migrador y las migraciones PostgreSQL nuevas.
- [`web/`](web/) es la aplicación React responsive.
- [`compose.yaml`](compose.yaml) levanta PostgreSQL 16, el migrador one-shot,
  la API y la web para desarrollo local. La API y los workers Go usan Air; la
  consola usa el HMR de Vite. Con Clerk configurado también inicia el worker
  IAM que consume el outbox.

Cada componente mantiene dependencias y checks propios. Los módulos de
`platform` se consumen exclusivamente mediante versiones publicadas.

El almacenamiento de certificados y comprobantes fiscales tiene adaptadores
separados para desarrollo y producción. La configuración KMS/S3, el modelo de
envelope encryption y los permisos mínimos están documentados en
[`docs/v2/fiscal-secure-storage.md`](../docs/v2/fiscal-secure-storage.md).
La arquitectura, las garantías y el flujo operativo del MVP están en
[`docs/v2/accounting-fiscal-mvp.md`](../docs/v2/accounting-fiscal-mvp.md).

## Desarrollo local con Docker

Requiere Docker con Compose. No requiere Go ni Node instalados en el host:

```bash
make up
make smoke
```

La web queda disponible en `http://localhost:15173`, la API en
`http://localhost:18080` y PostgreSQL en `127.0.0.1:55433`. El job `migrate`
termina antes de que se inicie el backend. Los cambios en `backend/**/*.go`
recompilan el proceso correspondiente con Air y los cambios en `web/src`
se reflejan mediante Vite sin reconstruir imágenes. Las configuraciones
`.air.api.toml`, `.air.iam-worker.toml` y `.air.iam-sync-worker.toml` mantienen
el mismo flujo para cada binario de larga duración.

Sin claves Clerk el stack permanece saludable, pero toda la superficie IAM
responde en modo cerrado con `AUTH_NOT_CONFIGURED`; no existe un bypass local.
Para habilitar Clerk, copiar `.env.example` a `.env` y completar únicamente las
variables `PYMES_CLERK_*` de la aplicación exclusiva de v2. Nginx reenvía
`/api/*` y `/webhooks/clerk` al backend sin agregar identidad ni organización.
Cuando existe `PYMES_CLERK_SECRET_KEY`, `make up` habilita automáticamente el
perfil Compose `iam`; sin esa clave el worker no arranca y ningún efecto
externo se simula.

Pymes selecciona organizaciones exclusivamente desde su directorio local. La
instancia Clerk debe emitir el session token estándar con
`aud=pymes-v2-api`, permitir que Pymes active la organización admitida y tener
slugs habilitados. No se usan JWT templates porque no conservan el vínculo de
sesión requerido por la API. Con la CLI autenticada y el repositorio vinculado,
la configuración remota puede verificarse sin modificarla:

```bash
make clerk-check
```

El check exige `force_organization_selection=false`, `slug_disabled=false`,
el claim de audiencia y ausencia de JWT templates. También admite un destino
explícito con
`./scripts/check-clerk-config.sh --app app_... --instance dev`.

Los puertos publicados pueden cambiarse, por ejemplo con
`PYMES_API_PORT=28080 PYMES_WEB_PORT=25173 make up`. Dentro de Docker el
backend y el servidor web conservan el puerto `8080`.

`make down` detiene los servicios sin borrar el volumen. `make ps` muestra el
estado y `make logs` sigue los logs del stack.

## Aprovisionamiento privado

Las organizaciones no se crean desde la web. Un operador registra una
organización nueva con Docker, sin necesitar Go en el host:

```bash
make provision-org \
  ORG_NAME="Acme Argentina" \
  ORG_SLUG="acme-argentina" \
  OWNER_EMAIL="owner@example.com"
```

El comando crea la organización local en estado `provisioning` y anexa una
operación al outbox dentro de la misma transacción. No llama a Clerk ni requiere
sus claves. Repetir exactamente el mismo slug, nombre y email devuelve los
mismos identificadores; reutilizar el slug con otro payload falla con
`IAM_PROVISION_PAYLOAD_CONFLICT`. Cuando Clerk está configurado, el worker
consume la operación, reconcilia la organización por slug y envía una única
invitación inicial al owner.

## Desarrollo nativo

Para ejecutar backend y web fuera de contenedores se requieren Go 1.26.5,
Node 20.19 o posterior y npm 10:

```bash
cp .env.example .env
make db-up
make db-migrate
make backend-run
make web-dev
```

En ejecución nativa, la API escucha en la dirección definida por
`PYMES_HTTP_ADDR`. El ejemplo usa `:8080` y expone:

- `GET /healthz`: liveness del proceso;
- `GET /readyz`: disponibilidad de PostgreSQL.

Para ejecutar los mismos checks de cada componente:

```bash
make ci
make db-integration
```

`make db-down` detiene el stack sin borrar su volumen.

Principios iniciales:

- núcleo horizontal y web responsive;
- monolito modular con API Go y PostgreSQL nuevos;
- OpenAPI como contrato público;
- aislamiento tenant fail-closed;
- Money decimal exacto;
- consumo directo de releases publicadas de `platform`;
- cero lectura, escritura o fallback hacia v1.
