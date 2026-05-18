# Pymes — Reglas del proyecto

## 1. Contexto

Plataforma SaaS multi-vertical para PyMEs latinoamericanas. Monorepo con:
- `pymes-core/` — base transversal (backend Go + shared)
- `professionals/` — vertical docentes/profesionales (backend Go)
- `workshops/` — vertical talleres mecánicos (backend Go)
- `beauty/` — vertical belleza / salón (equipo, menú de servicios; backend Go)
- `restaurants/` — vertical bares / restaurantes (zonas, mesas, sesiones de mesa; backend Go)
- `frontend/` — consola React unificada
- `ai/` — servicio FastAPI con Gemini
- `pymes-app/` — app móvil Expo (React Native, Expo Router v6, Clerk auth, Zustand)

Código reutilizable: librería **`core`** (`github.com/devpablocristo/core/...`) para lo agnóstico; **`pymes-core/shared/`** para lo transversal del producto; lo atado al dominio de un servicio en el **`internal/`** de ese backend (no hay carpeta `pkgs/` en este repo).

Documentación canónica del monorepo: **`docs/README.md`** (índice), **`docs/AUTH.md`** (identidad y acceso), **`docs/CLERK_LOCAL.md`** (Clerk en Docker, org y JWT), **`docs/PYMES_CORE.md`** (backend transversal), **`docs/CORE_INTEGRATION.md`** (librerías `core`), **`pymes-core/docs/FRAUD_PREVENTION.md`** (auditoría, cobros, RBAC / anti-fraude).

---

## 2. Idioma

### 2.1 Código — siempre inglés

Todo lo que es **código interno** debe estar en inglés sin excepciones:
- Variables, funciones, métodos, structs, types, interfaces, enums
- Nombres de tablas, columnas, índices, constraints en SQL
- Nombres de campos en JSON (API request/response), GORM tags, JSON tags
- Nombres de roles, permisos, recursos en RBAC
- Nombres de archivos y directorios
- Constantes, feature flags, config keys
- Seeds y fixtures (nombres de entidades de datos como roles, permisos)
- Endpoints y rutas HTTP
- Nombres de migraciones
- Test names y test data identifiers

### 2.2 Español — solo lo que ve el usuario

- **UI visible** (labels, placeholders, mensajes de error de UI, onboarding text): español (producto para LATAM)
- **Comentarios** en código: español (para aclarar lógica)
- **Documentación** (`.md`): español
- **Strings de i18n**: español (ES) e inglés (EN) según el locale
- **Descripciones de AI** (prompts, respuestas al usuario): español
- **TODOs**: inglés
- **Respuestas del asistente**: español siempre

---

## 3. Principios

- **DRY** — si se repite dos veces, abstraer
- **YAGNI** — no agregar lo que no se pidió
- **SOLID** — SRP, OCP, LSP, ISP, DIP
- **KISS** — tres líneas similares son mejores que una abstracción prematura
- **Fail fast** — validar inputs al inicio, retornar error inmediato
- **Cambios quirúrgicos** — solo modificar lo que se pide

---

## 4. Flujo de trabajo

1. TLDR primero
2. Ideal primero, luego práctico si difieren
3. Esperar aprobación antes de implementar algo no trivial
4. **Verificación obligatoria antes de decir “listo” / “ya está”:** para **todo cambio de código, configuración, CI, Docker o infraestructura**, ejecutar la validación relevante **antes** de reportar cierre o empujar el cambio. Desde la raíz del monorepo: **`make build`** y **`make test`** cuando el cambio afecta entrega o varios paquetes; si el alcance es mínimo, al menos el subset equivalente (p. ej. `go test` en el backend tocado + `npm run build` / `npm test` en frontend).
5. Si tocás **Dockerfile**, **entrypoint**, **compose** o wiring de runtime, además es obligatorio: **`docker compose build`** del servicio afectado, **`docker compose up -d`**, esperar readiness real y comprobar **HTTP** (p. ej. `curl` a `/healthz` en el puerto publicado). Si el smoke funcional depende de auth, seeds o secretos del entorno actual, hay que explicitarlo con precisión y dejar evidencia de la máxima validación posible en ese mismo turno. Ver `.cursor/rules/verify-before-claim.mdc`.
6. **Prohibido** afirmar “listo”, “ya está” o “funciona” sin evidencia de una ejecución exitosa en este turno (comandos + salida OK). También queda prohibido dejar el testing “para después”.
7. **Si el cambio apunta a corregir un bug visible por usuario, no alcanza con compilar ni con asumir la causa:** hay que **reproducirlo, iterar, volver a probar e insistir hasta verificar que el bug quedó resuelto** en el flujo afectado; recién ahí se puede devolver como cerrado.

---

## 5. Arquitectura Go — Hexagonal (Gin + GORM)

### 5.1 Estructura de proyecto

```
{vertical}/
├── backend/
│   ├── cmd/local/main.go
│   ├── internal/
│   │   ├── {modulo}/               # un dir por dominio de negocio
│   │   └── shared/                 # código transversal del servicio
│   ├── wire/bootstrap.go           # DI manual
│   ├── migrations/
│   │   ├── *.up.sql
│   │   └── runner.go
│   ├── Dockerfile
│   └── go.mod
pymes-core/
├── backend/                        # base transversal
├── shared/                         # runtime y utilidades compartidas entre verticales
│   ├── backend/                    # Go: auth, config, middleware
│   └── ai/                         # Python: AI runtime
├── infra/aws/                      # Terraform por cloud (hermanos: gcp/, azure/...)
frontend/                           # consola React unificada
ai/                                 # servicio FastAPI
professionals/                      # vertical (backend + infra/aws)
workshops/                          # vertical (backend + infra/aws)
beauty/                             # vertical (backend + infra/aws)
restaurants/                        # vertical (backend/; infra opcional por vertical)
```

Librerías agnósticas: módulos `github.com/devpablocristo/core/...` en `go.mod` (checkout local típico `../core`), no carpeta `pkgs/` en este repo. Puertos locales: ver **`docs/README.md`** (tabla) y **`docker-compose.yml`**.

### 5.2 Estructura de módulo

Cada adapter tiene su archivo principal en la raíz del módulo y un directorio con el mismo nombre para sus tipos auxiliares.

```
internal/{modulo}/
    usecases.go                      # lógica de negocio + ports (interfaces)
    usecases/
        domain/
            entities.go              # tipos de dominio (la verdad del negocio)

    handler.go                       # adapter HTTP (Gin)
    handler/
        dto/
            dto.go                   # tipos HTTP (request/response DTOs)

    repository.go                    # adapter DB (interface + sentinel errors + impl GORM)
    repository/
        models/
            models.go                # tipos DB (si difieren del dominio)

    {otro_adapter}.go                # ej: executor.go, gateway_adapter.go
    {otro_adapter}/
        ...                          # tipos/config del adapter

    *_test.go
```

### 5.3 Tipos y mappers por capa

Cada capa define sus propios tipos. Nunca expone los de otra capa.

| Capa | Tipos | Ubicación |
|------|-------|-----------|
| Dominio | Entidades de negocio | `usecases/domain/entities.go` |
| HTTP | DTOs request/response | `handler/dto/dto.go` |
| DB | Models (si difieren del dominio) | `repository/models/models.go` |
| Otros adapters | Tipos propios | `{adapter}/` |

Los **mappers** viven en el adapter que los necesita:
- `handler.go` convierte DTO → dominio (entrada) y dominio → DTO (salida)
- `repository.go` convierte dominio → model (escritura) y model → dominio (lectura)

**Los usecases solo conocen tipos de dominio.** Nunca importan DTOs ni models.

### 5.4 Código compartido

| Ubicación | Qué contiene | Criterio |
|-----------|-------------|----------|
| Librería **`core`** (`github.com/devpablocristo/core/...`) | Primitivas agnósticas (authn, saas, governance, helpers HTTP, etc.) | Portable entre productos; versionada fuera de este repo |
| `pymes-core/shared/` | Código transversal del producto | Específico de Pymes, usado por varios verticales o capas |
| `internal/{modulo}/` del servicio owner | Dominio y adapters del módulo | Acoplado al negocio de ese backend; no se fuerza a `shared` ni a `core` |

`pymes-core/shared/` no sustituye la librería `core`: cada uno tiene su criterio (ver reglas `library-placement`).

### 5.5 Persistencia

- PostgreSQL en desarrollo, staging y producción. **Sin excepciones.**
- **No existen repositorios in-memory.**
- Un solo archivo `repository.go` por módulo: interface + sentinel errors + implementación GORM. **Sin sufijos.**
- Para tests: fakes/stubs dentro del `_test.go`, nunca como archivo separado.
- **Identidad multi-tenant: tabla canónica `orgs`, columna FK `org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE`.** No usar `tenants` (no existe en el schema post-cutover) ni columnas `tenant_id`. Excepción: las tablas `tenant_settings` y `tenant_invitations` mantienen su nombre por convención `core/saas/go`, pero su FK también es `org_id`. Ver [`docs/DATABASE_INIT.md`](docs/DATABASE_INIT.md).
- **Soft delete: `archived_at timestamptz NULL`**. Excepciones documentadas: `users.deleted_at` (semántica GDPR) y `sales.voided_at` (regulación contable).
- **Migraciones nuevas**: numeración consecutiva en `pymes-core/backend/migrations/` (post-squash arrancamos desde 0018). Nunca modificar las 0001..0017 squashed. Toda migración nueva tiene su `.down.sql` reverso completo y `CREATE TABLE/INDEX IF NOT EXISTS`.

### 5.6 Naming por archivo

| Archivo | Contenido |
|---------|-----------|
| `usecases.go` | `Usecases` struct + `NewUsecases()` + lógica + ports |
| `usecases/domain/entities.go` | Entidades puras con json tags |
| `handler.go` | `Handler` struct + `NewHandler(uc interface)` + `RegisterRoutes()` |
| `handler/dto/dto.go` | **TODOS** los DTOs. NUNCA `var body struct{...}` inline |
| `repository.go` | `Repository` interface + sentinel errors + `Repository` + impl |
| `internal/shared/errors.go` | Error helpers compartidos, constantes |

### 5.7 Accept interfaces, return structs

- Constructores reciben **interfaces**, devuelven `*Struct`
- Interfaces se definen en el **consumidor**, no en el proveedor
- Cada adapter define su port con **solo los métodos que necesita** (ISP)

### 5.8 Convenciones Go (Uber Style Guide)

**Básicas:**
- `context.Context` siempre primer parámetro
- No `init()`, no `panic()`, no `_` para ignorar errores
- Slices como valores, punteros para structs de dominio
- Enums como typed string, IDs como `uuid.UUID`
- Structs literales nombrados, no posicionales
- Config desde env vars, nunca hardcodeado

**Errores:**
- Wrapping: `fmt.Errorf("create policy: %w", err)`
- Comparación: `errors.Is()`, nunca strings
- NUNCA exponer `err.Error()` al cliente HTTP — loguear y retornar mensaje genérico

**Control flow:**
- Early return, avoid unnecessary else
- Functional options para constructores con muchos params

**Performance:**
- `strconv` > `fmt` para conversiones
- `time.Duration` siempre, nunca `int` para duraciones
- Copy slices/maps at boundaries
- No fire-and-forget goroutines
- Propagar ctx, nunca `context.Background()` si ya hay ctx

**Naming:**
- Packages: lowercase, singular
- Receivers: 1-2 letras consistentes
- Unexported first

**Logging:** siempre `slog` o `zerolog`, nunca `fmt.Printf`

---

## 6. Verticales sobre pymes-core

- `pymes-core` es la base transversal obligatoria del producto.
- **Si algo aplica a más de un vertical, va en pymes-core.** No duplicar.
- Las verticales solo contienen funcionalidad exclusiva de su dominio.
- Si una vertical consume capacidades de otra o de pymes-core, la integración es por HTTP.
- Una vertical no importa handlers, usecases, repositories ni dominio interno de otra.
- No se permite duplicar en una vertical: auth, API keys, tenant/org, party model, customers, products, appointments, quotes, sales, payments, WhatsApp, billing, admin, ni la base común de AI.
- Todo prompt o diseño de vertical debe declarar: `reutiliza desde pymes-core` y `crea nuevo en la vertical`.

### 6.1 Selección de vertical

- Cada tenant elige **una sola vertical** (o ninguna) durante el onboarding.
- La vertical elegida se guarda en `TenantProfile.vertical` (`'none' | 'professionals' | 'workshops' | 'beauty' | 'restaurants'`).
- El sidebar solo muestra la sección de la vertical elegida. Sin vertical = solo módulos comerciales/operaciones.
- Las rutas de verticales no elegidas siguen existiendo (no se bloquean) pero no aparecen en la navegación.

---

## 7. CRUD canónico

Contrato HTTP compartido entre backend y frontend. Los segmentos de ruta vienen de la librería común **`modules/crud/paths`** (Go: `crudpaths.SegmentArchived`, `SegmentArchive`, `SegmentRestore`, `SegmentHard`; TS espejo: `modules/crud/ui/ts/src/restPaths.ts`) y son consumidos por el frontend vía `buildRestCrudDataSource` (`frontend/src/crud/restCrudDataSource.ts`). No redefinir estos literales en cada módulo.

| Operación | Método | Path | Status | Semántica |
|-----------|--------|------|--------|-----------|
| Create | `POST` | `/v1/{entities}` | 201 | — |
| Read | `GET` | `/v1/{entities}/{id}` | 200 | — |
| List | `GET` | `/v1/{entities}` | 200 | excluye archivados por default |
| List archivados | `GET` | `/v1/{entities}/archived` | 200 | equivalente a `List?archived=true` |
| Update | `PATCH` | `/v1/{entities}/{id}` | 200 | — |
| Delete (soft) | `DELETE` | `/v1/{entities}/{id}` | 204 | **soft delete** — marca archivado |
| Archive (alias soft) | `POST` | `/v1/{entities}/{id}/archive` | 204 | mismo efecto que `DELETE /:id`, idempotente |
| Restore | `POST` | `/v1/{entities}/{id}/restore` | 204 | limpia la marca de archivado, idempotente |
| Hard delete | `DELETE` | `/v1/{entities}/{id}/hard` | 204 | borrado físico irreversible |

- `DELETE /:id` = **soft delete**. El hard delete siempre es explícito en `/:id/hard`.
- `Archive` y `DELETE /:id` producen el mismo efecto; `Archive` existe como verbo explícito para el frontend.
- `Archive` / `Restore` / `Delete (soft)` son idempotentes.
- List admite `?archived=true` para incluir archivados; la sub-ruta `/archived` es un atajo canónico del frontend (misma semántica).
- La columna de archivado puede llamarse `deleted_at` o `archived_at` según el módulo; conceptualmente es la marca de soft delete.

---

## 8. Seguridad

- Errores HTTP: `{code, message}`. NUNCA exponer `err.Error()` al cliente.
- Validar inputs: longitud, enums, formato.
- Sentinel errors en `repository.go`: `ErrNotFound`, `ErrAlreadyExists`, `ErrArchived`.
- API keys obligatorias. Fail si no están configuradas.
- Health endpoints (`/healthz`, `/readyz`) fuera de auth.
- **Fraude / robos internos / trazabilidad de dinero:** documentación canónica en **[`pymes-core/docs/FRAUD_PREVENTION.md`](pymes-core/docs/FRAUD_PREVENTION.md)** (auditoría, evento `payment.created`, RBAC, export CSV, backlog). Cualquier cambio en cobros, `audit_log` o permisos de rutas sensibles debe mantener ese documento al día; está enlazado desde [`docs/README.md`](docs/README.md) y [`docs/PYMES_CORE.md`](docs/PYMES_CORE.md).

---

## 9. Python — FastAPI (servicio AI)

Arquitectura clean/layered. Pydantic para DTOs y config. Protocol para interfaces. Depends() para DI. Alembic para migraciones. Ruff + mypy. Mismas 7 operaciones CRUD.

- **Type hints siempre**
- **Pydantic para DTOs**, Pydantic Settings para config
- **async/await para I/O**
- **Protocol para interfaces**
- **No `print()`** — usar `logging`
- **`|` syntax para Optional** — `str | None`, no `Optional[str]`

---

## 10. Docker y naming

### Servicios en docker-compose

Los nombres de servicio NO llevan prefijo `pymes-`. El `COMPOSE_PROJECT_NAME` ya lo aporta.

| Tipo | Servicio compose | Container resultante |
|------|-----------------|---------------------|
| Backend Go | `cp-backend` | `pymes-cp-backend-1` |
| Backend vertical | `prof-backend`, `work-backend`, `beauty-backend`, `restaurants-backend` | `pymes-prof-backend-1` |
| DB | `postgres` | `pymes-postgres-1` |
| Frontend | `frontend` | `pymes-frontend-1` |
| AI / Chat | (sibling repo) `companion` | servido por Companion local en `:18085` |

### Reglas Docker

- `postgres:16-alpine`, `restart: unless-stopped`, healthcheck
- Puertos configurables via env vars

### Desarrollo local (contenedores)

- **Flujo habitual del equipo:** levantar todo con **`make up`** (o `docker compose up -d --build`) desde la raíz del monorepo donde está `docker-compose.yml`; no se asume correr backends, frontend ni AI como procesos nativos en el host.
- Los **`cmd/local/main.go`** siguen existiendo (paridad con Gin, depuración, `go build` de verificación); ejecutarlos con `go run` en el host es **opcional** y está documentado en **`docs/AUTH.md`** como caso excepcional.
- Ver también **`README.md`** y **`Makefile`** (objetivos `up`, `down`, `build`, `test`, `logs`, `ps`).

### Nombres prohibidos

- NUNCA `web/`, `frontend/`, `ui/` → el frontend ya se llama `frontend/`
- NUNCA `api/`, `server/` → usar nombre del producto (`pymes-core/`, `professionals/`, `workshops/`, `beauty/`, `restaurants/`)
- NUNCA `postgres:16` sin `-alpine`

---

## 11. Tests

- Go: table-driven, `t.Parallel()`, `httptest`, fakes inline en `_test.go`
- Python: pytest + httpx.AsyncClient, fakes inline
- Cubrir: happy path, not found, validation, conflict, archive/restore

---

## 12. Customer Messaging sobre WhatsApp

### 12.1 Arquitectura

La mensajería con clientes vive en `pymes-core/backend/internal/customer_messaging/`. El adapter proveedor de Meta vive en `pymes-core/backend/internal/customer_messaging/channels/whatsapp/`. No va en `core/saas/go` porque sigue siendo específico del producto pymes.

```
internal/customer_messaging/
├── usecases.go                               # lógica + ports
├── domain/entities.go                        # Connection, Message, Template, OptIn, Campaign
├── handler.go                                # HTTP adapter (Gin)
├── handler/dto/dto.go                        # DTOs request/response
├── repository.go                             # GORM adapter + sentinels + mappers
├── repository/models/models.go               # GORM models
├── inbound.go                                # Webhook handling (verify + HMAC + inbound messages/statuses)
├── channels/whatsapp/clients.go              # AIClient + MetaClient (Graph API)
├── *_test.go
```

### 12.2 Tablas

| Tabla | Propósito |
|-------|-----------|
| `whatsapp_connections` | 1 por org. Phone number ID, WABA ID, token encriptado, quality rating |
| `whatsapp_messages` | Historial enviados/recibidos. Status tracking (pending→sent→delivered→read) |
| `whatsapp_templates` | Templates de Meta. Draft→pending→approved/rejected. CRUD local |
| `whatsapp_opt_ins` | Consentimiento por contacto. Obligatorio antes de enviar |

### 12.3 API (endpoints)

**Links wa.me/:**
- `GET /v1/customer-messaging/share/quote/:id` — link de presupuesto
- `GET /v1/customer-messaging/share/sale/:id/receipt` — link de comprobante
- `GET /v1/customer-messaging/share/customer/:id/message` — mensaje libre

**Conexión:**
- `GET /v1/customer-messaging/connections/whatsapp` — estado
- `POST /v1/customer-messaging/connections/whatsapp` — conectar (phone_number_id, waba_id, access_token)
- `DELETE /v1/customer-messaging/connections/whatsapp` — desconectar
- `GET /v1/customer-messaging/connections/whatsapp/stats` — métricas

**Envío real (Graph API):**
- `POST /v1/customer-messaging/messages/text` — texto directo
- `POST /v1/customer-messaging/messages/template` — template aprobado
- `POST /v1/customer-messaging/messages/media` — imagen, documento, audio, video
- `POST /v1/customer-messaging/messages/interactive` — botones de respuesta rápida (max 3)

**Historial:**
- `GET /v1/customer-messaging/messages` — listado con filtros (party_id, direction, status)

**Templates:**
- `GET /v1/customer-messaging/templates` — listar
- `POST /v1/customer-messaging/templates` — crear (draft)
- `GET /v1/customer-messaging/templates/:id` — detalle
- `DELETE /v1/customer-messaging/templates/:id` — eliminar

**Opt-in:**
- `GET /v1/customer-messaging/consents` — listar contactos con consentimiento
- `POST /v1/customer-messaging/consents` — registrar consentimiento
- `DELETE /v1/customer-messaging/consents/:party_id` — registrar opt-out
- `GET /v1/customer-messaging/consents/:party_id/status` — verificar estado

**Webhooks (públicos, sin auth):**
- `GET /v1/webhooks/customer-messaging/whatsapp` — verificación Meta
- `POST /v1/webhooks/customer-messaging/whatsapp` — inbound + status (rate limit 240/min, max 256KB)

### 12.4 Meta Graph API

- Versión: v23.0
- Client: `MetaClient` en `channels/whatsapp/clients.go`
- Métodos: `SendTextMessage`, `SendTemplateMessage`, `SendMediaMessage`, `SendInteractiveButtons`, `MarkAsRead`
- Todos retornan `(waMessageID string, error)` para tracking
- Tokens almacenados encriptados via `paymentgateway.Crypto`

### 12.5 Multi-tenant

- Cada org tiene máximo 1 conexión (`whatsapp_connections.tenant_id` es PK)
- Cada conexión tiene su propio `phone_number_id` + `access_token`
- El flujo de conexión futuro será via Embedded Signup (popup Meta OAuth)
- Los mensajes se registran con `tenant_id` para aislamiento total

### 12.6 Compliance LATAM

- **Opt-in obligatorio**: tabla `whatsapp_opt_ins`, verificar antes de enviar
- **Templates en español**: idioma default `es`, categorías UTILITY/MARKETING/AUTHENTICATION
- **Status tracking**: sent→delivered→read via webhooks de Meta
- **Rate limits**: tier 1 (250 msgs/24h) → tier 5 (ilimitado), sube automáticamente

---

## 13. Reglas críticas

- NUNCA valores hardcodeados
- NUNCA exponer dominio por HTTP — siempre DTOs
- NUNCA `var body struct{...}` inline — siempre DTOs en `handler/dto/`
- NUNCA modificar migraciones existentes
- NUNCA `panic()`, NUNCA `_` para ignorar errores, NUNCA `fmt.Printf` para logging
- NUNCA `err.Error()` en respuestas HTTP al cliente
- NUNCA repositorios in-memory como artefacto de producción
- NUNCA sufijos en archivos si solo hay una implementación
- NUNCA decir "listo" sin haber buildado/testeado
- NUNCA duplicar funcionalidad de pymes-core en una vertical
- NUNCA importar dominio interno entre verticales — solo HTTP
- NUNCA crear tablas con prefijo `tenant_*` ni columnas `tenant_id`. Identidad canónica: `orgs` + columna `org_id`. Excepción única: las tablas saas `tenant_settings` y `tenant_invitations` mantienen su nombre histórico (su FK ya es `org_id`).
- NUNCA hacer queries SQL raw que referencien las tablas legacy `tenants`, `tenant_memberships`, `tenant_api_keys`, `tenant_usage_counters` — fueron renombradas en el cutover (PR #13) a `orgs`, `org_members`, `org_api_keys`, `org_usage_counters`. Si encontrás una referencia residual: bug, reportarlo.
- NUNCA llamar `saasmigrations.MigrateUp` en bootstrap. El schema saas está copiado y versionado en `pymes-core/backend/migrations/0001_saas_identity.up.sql`. Si `core/saas/go` evoluciona, hay que adoptar el cambio explícitamente en pymes-core con una migración nueva.
