# Pymes — Reglas del proyecto

## 1. Contexto

Plataforma SaaS multi-vertical para PyMEs latinoamericanas. Monorepo con:
- `pymes-core/` — base transversal (backend Go + shared)
- `professionals/` — vertical docentes/profesionales (backend Go)
- `workshops/` — vertical talleres mecánicos (backend Go)
- `beauty/` — vertical belleza / salón (equipo, menú de servicios; backend Go)
- `frontend/` — consola React unificada
- `ai/` — servicio FastAPI con Gemini

Código reutilizable: librería **`core`** (`github.com/devpablocristo/core/...`) para lo agnóstico; **`pymes-core/shared/`** para lo transversal del producto; lo atado al dominio de un servicio en el **`internal/`** de ese backend (no hay carpeta `pkgs/` en este repo).

Documentación canónica del monorepo: **`docs/README.md`** (índice), **`docs/AUTH.md`** (identidad y acceso), **`docs/PYMES_CORE.md`** (backend transversal), **`docs/CORE_INTEGRATION.md`** (librerías `core`).

---

## 2. Idioma

- **Código**: inglés
- **Comentarios**: español
- **TODOs**: inglés
- **Respuestas**: español siempre

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
4. **Verificación obligatoria antes de decir “listo” / “ya está”:** desde la raíz del monorepo ejecutar **`make build`** y **`make test`** en este mismo turno cuando el cambio afecta entrega o varios paquetes; si el alcance es mínimo, al menos el subset equivalente (p. ej. `go test` en el backend tocado + `npm run build` / `npm test` en frontend). Si tocás **Dockerfile**, **entrypoint** o **compose**, además: **`docker compose build`** del servicio afectado, **`docker compose up -d`**, y comprobar **HTTP** (p. ej. `curl` a `/healthz` en el puerto publicado). Ver `.cursor/rules/verify-before-claim.mdc`.
5. **Prohibido** afirmar “listo”, “ya está” o “funciona” sin evidencia de una ejecución exitosa en este turno (comandos + salida OK).

---

## 5. Arquitectura Go — Hexagonal (Gin + GORM + Lambda)

### 5.1 Estructura de proyecto

```
{vertical}/
├── backend/
│   ├── cmd/lambda/main.go
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
├── infra/
frontend/                           # consola React unificada
ai/                                 # servicio FastAPI
```

Librerías agnósticas: módulos `github.com/devpablocristo/core/...` en `go.mod` (checkout local típico `../core`), no carpeta `pkgs/` en este repo.

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
- La vertical elegida se guarda en `TenantProfile.vertical` (`'none' | 'professionals' | 'workshops' | 'beauty'`).
- El sidebar solo muestra la sección de la vertical elegida. Sin vertical = solo módulos comerciales/operaciones.
- Las rutas de verticales no elegidas siguen existiendo (no se bloquean) pero no aparecen en la navegación.

---

## 7. CRUD canónico (7 operaciones)

| Operación | Método | Path | Status |
|-----------|--------|------|--------|
| Create | `POST` | `/v1/{entities}` | 201 |
| Read | `GET` | `/v1/{entities}/{id}` | 200 |
| List | `GET` | `/v1/{entities}` | 200 |
| Update | `PATCH` | `/v1/{entities}/{id}` | 200 |
| Delete | `DELETE` | `/v1/{entities}/{id}` | 204 |
| Archive | `POST` | `/v1/{entities}/{id}/archive` | 204 |
| Restore | `POST` | `/v1/{entities}/{id}/restore` | 204 |

- DELETE = **hard delete** siempre. Archive = **soft delete**. Restore = limpia `archived_at`.
- Archive/Restore son idempotentes.
- List excluye archivados por default; `?archived=true` para incluirlos.

---

## 8. Seguridad

- Errores HTTP: `{code, message}`. NUNCA exponer `err.Error()` al cliente.
- Validar inputs: longitud, enums, formato.
- Sentinel errors en `repository.go`: `ErrNotFound`, `ErrAlreadyExists`, `ErrArchived`.
- API keys obligatorias. Fail si no están configuradas.
- Health endpoints (`/healthz`, `/readyz`) fuera de auth.

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
| Backend vertical | `prof-backend`, `work-backend`, `beauty-backend` | `pymes-prof-backend-1` |
| DB | `postgres` | `pymes-postgres-1` |
| Frontend | `frontend` | `pymes-frontend-1` |
| AI | `ai` | `pymes-ai-1` |

### Reglas Docker

- `postgres:16-alpine`, `restart: unless-stopped`, healthcheck
- Puertos configurables via env vars

### Desarrollo local (contenedores)

- **Flujo habitual del equipo:** levantar todo con **`make up`** (o `docker compose up -d --build`) desde la raíz del monorepo donde está `docker-compose.yml`; no se asume correr backends, frontend ni AI como procesos nativos en el host.
- Los **`cmd/local/main.go`** siguen existiendo (paridad con Gin, depuración, `go build` de verificación); ejecutarlos con `go run` en el host es **opcional** y está documentado en **`docs/AUTH.md`** como caso excepcional.
- Ver también **`README.md`** y **`Makefile`** (objetivos `up`, `down`, `build`, `test`, `logs`, `ps`).

### Nombres prohibidos

- NUNCA `web/`, `frontend/`, `ui/` → el frontend ya se llama `frontend/`
- NUNCA `api/`, `server/` → usar nombre del producto (`pymes-core/`, `professionals/`, `workshops/`)
- NUNCA `postgres:16` sin `-alpine`

---

## 11. Tests

- Go: table-driven, `t.Parallel()`, `httptest`, fakes inline en `_test.go`
- Python: pytest + httpx.AsyncClient, fakes inline
- Cubrir: happy path, not found, validation, conflict, archive/restore

---

## 12. WhatsApp Business — Módulo transversal

### 12.1 Arquitectura

WhatsApp es un módulo transversal en `pymes-core/backend/internal/whatsapp/`. No va en `core/saas/go` (es específico del producto pymes). Estructura hexagonal completa:

```
internal/whatsapp/
├── usecases.go                     # lógica + ports (RepositoryPort, TimelinePort, etc.)
├── usecases/domain/entities.go     # Connection, Message, Template, OptIn
├── handler.go                      # HTTP adapter (Gin)
├── handler/dto/dto.go              # DTOs request/response
├── repository.go                   # GORM adapter + sentinels + mappers
├── repository/models/models.go     # GORM models
├── clients.go                      # AIClient + MetaClient (Graph API v23.0)
├── inbound.go                      # Webhook handling (verify + HMAC + inbound messages)
├── *_test.go                       # 10 tests
```

### 12.2 Tablas

| Tabla | Propósito |
|-------|-----------|
| `whatsapp_connections` | 1 por org. Phone number ID, WABA ID, token encriptado, quality rating |
| `whatsapp_messages` | Historial enviados/recibidos. Status tracking (pending→sent→delivered→read) |
| `whatsapp_templates` | Templates de Meta. Draft→pending→approved/rejected. CRUD local |
| `whatsapp_opt_ins` | Consentimiento por contacto. Obligatorio antes de enviar |

### 12.3 API (endpoints)

**Links wa.me/ (legacy):**
- `GET /v1/whatsapp/quote/:id` — link de presupuesto
- `GET /v1/whatsapp/sale/:id/receipt` — link de comprobante
- `GET /v1/whatsapp/customer/:id/message` — mensaje libre

**Conexión:**
- `GET /v1/whatsapp/connection` — estado
- `POST /v1/whatsapp/connection` — conectar (phone_number_id, waba_id, access_token)
- `DELETE /v1/whatsapp/connection` — desconectar
- `GET /v1/whatsapp/connection/stats` — métricas

**Envío real (Graph API):**
- `POST /v1/whatsapp/send/text` — texto directo
- `POST /v1/whatsapp/send/template` — template aprobado
- `POST /v1/whatsapp/send/media` — imagen, documento, audio, video
- `POST /v1/whatsapp/send/interactive` — botones de respuesta rápida (max 3)

**Historial:**
- `GET /v1/whatsapp/messages` — listado con filtros (party_id, direction, status)

**Templates:**
- `GET /v1/whatsapp/templates` — listar
- `POST /v1/whatsapp/templates` — crear (draft)
- `GET /v1/whatsapp/templates/:id` — detalle
- `DELETE /v1/whatsapp/templates/:id` — eliminar

**Opt-in:**
- `GET /v1/whatsapp/opt-ins` — listar contactos con consentimiento
- `POST /v1/whatsapp/opt-ins` — registrar consentimiento
- `DELETE /v1/whatsapp/opt-ins/:party_id` — registrar opt-out
- `GET /v1/whatsapp/opt-ins/:party_id/status` — verificar estado

**Webhooks (públicos, sin auth):**
- `GET /v1/webhooks/whatsapp` — verificación Meta
- `POST /v1/webhooks/whatsapp` — inbound + status (rate limit 240/min, max 256KB)

### 12.4 Meta Graph API

- Versión: v23.0
- Client: `MetaClient` en `clients.go`
- Métodos: `SendTextMessage`, `SendTemplateMessage`, `SendMediaMessage`, `SendInteractiveButtons`, `MarkAsRead`
- Todos retornan `(waMessageID string, error)` para tracking
- Tokens almacenados encriptados via `paymentgateway.Crypto`

### 12.5 Multi-tenant

- Cada org tiene máximo 1 conexión (`whatsapp_connections.org_id` es PK)
- Cada conexión tiene su propio `phone_number_id` + `access_token`
- El flujo de conexión futuro será via Embedded Signup (popup Meta OAuth)
- Los mensajes se registran con `org_id` para aislamiento total

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
