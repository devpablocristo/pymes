# Pymes — Reglas del proyecto

## 1. Contexto

Plataforma SaaS multi-vertical para PyMEs latinoamericanas. Monorepo con:
- `control-plane/` — base transversal (backend Go + shared)
- `professionals/` — vertical docentes/profesionales (backend Go)
- `workshops/` — vertical talleres mecánicos (backend Go)
- `frontend/` — consola React unificada
- `ai/` — servicio FastAPI con Gemini
- `pkgs/` — paquetes compartidos agnósticos (Go, Python, TypeScript)

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
4. Verificar antes de decir "listo": `go build` + `go vet` + `go test`
5. Nunca decir "listo" sin evidencia de ejecución exitosa

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
control-plane/
├── backend/                        # base transversal
├── shared/                         # runtime y utilidades compartidas entre verticales
│   ├── backend/                    # Go: auth, config, middleware
│   └── ai/                         # Python: AI runtime
├── infra/
frontend/                           # consola React unificada
ai/                                 # servicio FastAPI
pkgs/                               # código agnóstico portable
├── go-pkg/
└── ts-pkg/
```

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
| `control-plane/shared/` | Código transversal del producto | Específico de pymes, usado por varios verticales |
| `pkgs/` | Código agnóstico al proyecto | Se puede llevar a otro proyecto sin cambios |

`pkgs/` no importa código de ningún servicio. `control-plane/shared/` no sale del producto.

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

## 6. Verticales sobre control-plane

- `control-plane` es la base transversal obligatoria del producto.
- **Si algo aplica a más de un vertical, va en control-plane.** No duplicar.
- Las verticales solo contienen funcionalidad exclusiva de su dominio.
- Si una vertical consume capacidades de otra o de control-plane, la integración es por HTTP.
- Una vertical no importa handlers, usecases, repositories ni dominio interno de otra.
- No se permite duplicar en una vertical: auth, API keys, tenant/org, party model, customers, products, appointments, quotes, sales, payments, WhatsApp, billing, admin, ni la base común de AI.
- Todo prompt o diseño de vertical debe declarar: `reutiliza desde control-plane` y `crea nuevo en la vertical`.

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
| Backend vertical | `prof-backend`, `work-backend` | `pymes-prof-backend-1` |
| DB | `postgres` | `pymes-postgres-1` |
| Frontend | `frontend` | `pymes-frontend-1` |
| AI | `ai` | `pymes-ai-1` |

### Reglas Docker

- `postgres:16-alpine`, `restart: unless-stopped`, healthcheck
- Puertos configurables via env vars

### Nombres prohibidos

- NUNCA `web/`, `frontend/`, `ui/` → el frontend ya se llama `frontend/`
- NUNCA `api/`, `server/` → usar nombre del producto (`control-plane/`, `professionals/`, `workshops/`)
- NUNCA `postgres:16` sin `-alpine`

---

## 11. Tests

- Go: table-driven, `t.Parallel()`, `httptest`, fakes inline en `_test.go`
- Python: pytest + httpx.AsyncClient, fakes inline
- Cubrir: happy path, not found, validation, conflict, archive/restore

---

## 12. Reglas críticas

- NUNCA valores hardcodeados
- NUNCA exponer dominio por HTTP — siempre DTOs
- NUNCA `var body struct{...}` inline — siempre DTOs en `handler/dto/`
- NUNCA modificar migraciones existentes
- NUNCA `panic()`, NUNCA `_` para ignorar errores, NUNCA `fmt.Printf` para logging
- NUNCA `err.Error()` en respuestas HTTP al cliente
- NUNCA repositorios in-memory como artefacto de producción
- NUNCA sufijos en archivos si solo hay una implementación
- NUNCA decir "listo" sin haber buildado/testeado
- NUNCA duplicar funcionalidad de control-plane en una vertical
- NUNCA importar dominio interno entre verticales — solo HTTP
