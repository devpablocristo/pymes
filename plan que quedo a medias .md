# Plan arquitectónico — Alinear `pymes` con `core`/`modules` (Status/FSM y CRUD)

## 1. Diagnóstico ejecutivo

**Hallazgo principal**: el plan táctico anterior ("adoptar FSM + helper `RegisterStatusEndpoint`") **resuelve el síntoma puntual de UpdateStatus** pero deja pasar tres cosas estructurales:

1. **El frontend tiene una reinvención casera de FSM** (`buildFullyConnectedStatusStateMachine` en `frontend/src/modules/billing/billingHelpers.ts`) usada por 4 dominios (sales, quotes, purchases, invoices), conectada por su propio "fully-connected" (todo pasa con todo). Esto es la causa raíz del "drag arbitrario funciona" — el FE deja mover a cualquier columna y el BE simplemente no validaba transiciones. Solucionar el BE sin tocar el FE deja el bug visible: el usuario arrastra y recibe 409 en vez de un drag bloqueado pre-server. **El frontend ya importa `@devpablocristo/core-fsm`** (en `frontend/src/modules/crud/kanbanTransitionModel.ts`) pero esa primitiva está infrautilizada — solo el módulo genérico de kanban la usa, no los dominios.
2. **`MapFSMError` y `RegisterStatusEndpoint` ambos en `internal/shared/handlers/` invierten la arquitectura**: si `sales/usecases.go` importa `internal/shared/handlers` para llamar `MapFSMError`, la capa de aplicación pasa a depender de la capa HTTP/handler. **Corrección obligatoria**: separar `MapFSMError` (agnóstico de HTTP, depende solo de `core/concurrency/go/fsm` + `core/errors/go/domainerr`) en `pymes-core/backend/internal/shared/status/fsm_errors.go`. `RegisterStatusEndpoint` queda en `handlers/` porque sí depende de Gin/RBAC.
3. **Audit/timeline/webhook post-transición** están bien extraídos como ports unificados (24 dominios usan la misma interface), pero la **decisión "emit timeline solo si hay party_id"** está dispersa — sales emite si hay `customer_id`, purchases si hay `supplier_id`, quotes no emite. Si se quiere normalizar, va como helper local en pymes-core (no es agnóstico — depende de la noción de "party" del producto).

4. **Tests E2E + Playwright NO son opcionales**: el bug original es UX (drag arbitrario que devuelve 409). El smoke manual no garantiza que el FE bloquee pre-server cuando el grafo cambia. Hay PR explícito de E2E backend (Go API tests con server real) + Playwright para drag-and-drop, NO solo "smoke manual".

5. **Detectar `status` en `PATCH /invoices/:id` requiere parseo explícito**: Gin ignora silenciosamente campos JSON desconocidos. Sacar `Status *string` del DTO NO devuelve 400 automáticamente — el body con `{"status":"paid"}` simplemente se descarta. Hay que parsear con `map[string]json.RawMessage` y rechazar explícitamente.

6. **Decisión purchases (`voided` terminal o no) DEBE cerrarse antes de implementar**: el plan no puede dejar una bifurcación A/B con verificación SQL como excusa para improvisar. Se cierra ahora a **opción A (no terminal)**, conservando comportamiento actual. Cualquier cambio a B es un PR breaking separado con producto involucrado.

7. **`IsTerminal` upstream a `core` es opcional, no bloqueante**: para `Validate(current, next)` no se necesita. Si en algún test/runtime hace falta detectar terminal, usar `errors.Is(Validate(s, "_probe_unknown"), fsm.ErrTerminal)` está prohibido por feo. Mejor: NO usarlo en este refactor. Hacer PR upstream solo por paridad TS, en paralelo, sin bloquear los demás PRs.

**Por qué el plan anterior era parcial**:
- ✅ Acertó: usar `core/concurrency/go/fsm`, no inventar.
- ✅ Acertó: NO subir a `core`/`modules` el helper HTTP (depende de RBAC pymes).
- ✅ Acertó: dejar audit/timeline/webhook en cada usecase.
- ❌ Faltó: **frontend** (100% backend-céntrico cuando el bug es FE+BE).
- ❌ Faltó: separar `MapFSMError` de `handlers` (invertía dirección de dependencias usecase→handler).
- ❌ Faltó: detección explícita de `status` en PATCH genérico de invoices (Gin ignora campos desconocidos).
- ❌ Faltó: cerrar decisión `voided` terminal en purchases (dejaba A/B abierto).
- ❌ Faltó: tests E2E + Playwright como fase obligatoria (solo "smoke manual").
- ❌ Faltó: política explícita para `same-status` (idempotente vs conflict).
- ❌ Faltó: ver que `paid → voided` en sales rompe lógica contable (`Void()` tiene side effects que `UpdateStatus` no replica).

**Deuda arquitectónica detectada (más allá de status)**:
| Patrón | Estado |
|---|---|
| Soft-delete | Convive `archived_at` (~9 dominios) y `deleted_at` (~6 dominios). Documentado en CLAUDE.md como "ambos OK", pero rompe abstracción. |
| Pagination | ✅ Extraída en `core/http/go/pagination` |
| Audit ports | ✅ Interface unificada usada en 24 dominios |
| Timeline ports | ✅ Unificada |
| Webhook | ✅ Unificada |
| RegisterRoutes wiring | Repetido en cada dominio (~7 líneas), trivial. **No abstraer**. |
| Tags/Notes/Favorite | 5 dominios idénticos. **No abstraer** (DTO mapping ya es declarativo). |
| Status enum/FSM frontend | Duplicado: 4 configs × `buildFullyConnectedStatusStateMachine` con sus propias listas de status hardcodeadas. **Causa raíz del bug**. |
| Status labels (i18n) | Solo invoices usa constante (`INVOICE_STATUS_LABELS`); sales/quotes/purchases inline. |
| Endpoint paths | Hardcodeados en cada `*Config.ts`. No usan `modules/crud/paths`. |

**Objetivos del refactor**:
1. Eliminar la divergencia FE/BE en transiciones de status.
2. Adoptar `core/concurrency/go/fsm` en backend para los 4 dominios con `UpdateStatus`.
3. Adoptar `@devpablocristo/core-fsm` en frontend para los 4 `*Config.ts` (reemplazar `buildFullyConnectedStatusStateMachine` por Builder).
4. Espejar grafos FE↔BE para que `canMoveToColumn` bloquee pre-server lo que el BE rechazaría.
5. Pequeño aporte upstream: `IsTerminal` en `StringMachine` Go (paridad TS).
6. Documentar las reglas para que futuras IAs/desarrolladores no recaigan.
7. **NO** sobre-extraer: tags/notes/favorite/audit/RegisterRoutes ya están bien.

---

## 2. Mapa actual del repo

### 2.1 Estructura física

```
/home/pablocristo/Proyectos/pablo/
├── core/                    # primitivas agnósticas
├── modules/                 # capacidades reusables
├── pymes/                   # producto
│   ├── pymes-core/backend/  # Go (4 verticales agregan más)
│   ├── frontend/            # consola React
│   ├── ai/                  # FastAPI
│   ├── professionals/, workshops/, beauty/, restaurants/  # verticales
└── otros productos: nexus, ponti, toollab, medmory, companion
```

### 2.2 Mapa de paquetes relevantes

| Área | Paquete | Responsabilidad | Problemas detectados | Evidencia |
|---|---|---|---|---|
| **core** | `core/concurrency/go/fsm` | Builder + StringMachine | Falta `IsTerminal(s)` (TS lo tiene como `isTerminal`) | `core/concurrency/go/fsm/builder.go:1-100` |
| **core** | `core/errors/go/domainerr` | Errores tipados | OK. `Newf(kind, fmt, args)` para mensajes formateados | `domainerr.go:39-58` |
| **core** | `core/http/gin/go` | `Respond(c, err)` mapea domainerr→HTTP | OK. Ya usado vía `pymes-core/shared/backend/httperrors` | `respond.go` |
| **core** | `core/http/go/pagination` | `Config`, `NormalizeLimit` | OK. Usado por `internal/shared/handlers/pagination.go` | `pagination.go` |
| **modules** | `modules/crud/paths/go` | `SegmentArchived`, `SegmentArchive`, `SegmentRestore`, `SegmentHard` | Solo lo usa `quotes`. Sales/purchases/invoices usan paths raw. | `pymes-core/backend/internal/quotes/handler.go:55` |
| **modules** | `modules/crud/archive/go` | `IfArchived(*time.Time, "resource")`, `IsArchived`, `ErrArchived` | Usado en 5+ dominios. **Acepta cualquier campo `*time.Time`** — convive `ArchivedAt` y `DeletedAt`. | `archive/go/archive.go` |
| **modules** | `modules/crud/ui/ts` | `CrudPage`, `CrudShellHeaderActionsColumn`, ~22 imports en frontend | OK. Frontend lo usa. | `frontend/src/modules/crud/CrudResourceShellHeader.tsx` |
| **modules** | `modules/work-orders/ts` | FSM + Kanban mapping para WO | Usa `@devpablocristo/core-fsm` correctamente | `modules/work-orders/ts/src/stateMachine.ts:14`, `kanbanConfig.ts` |
| **modules** | `modules/kanban/board/ts` | `StatusKanbanBoard.tsx` | OK. 2 imports en pymes frontend. | |
| **pymes-core** | `internal/sales/`, `quotes/`, `purchases/` | UpdateStatus heterogéneo | sales/quotes no validan transiciones; purchases usa switch hardcoded | `sales/usecases.go:362-391`, `quotes/usecases.go:137-176`, `purchases/usecases.go:181-217` |
| **pymes-core** | `internal/invoices/` | Sin endpoint /status; status entra por PATCH genérico | Sin validación, sin audit `invoice.status_updated` | `invoices/handler.go:43`, `usecases.go:184` |
| **pymes-core** | `internal/shared/handlers/` | RBAC, pagination wrappers, auth ctx | Lugar correcto para `RegisterStatusEndpoint` | `pagination.go`, `rbac_middleware.go` |
| **frontend** | `src/modules/billing/billingHelpers.ts` | `buildFullyConnectedStatusStateMachine` (caseroo) | **Reinventa core-fsm**. Genera grafo full-connected → bug "drag arbitrario" | usado en `billingSalesConfig.ts`, `billingQuotesConfig.ts`, `billingPurchasesConfig.ts`, `billingInvoicesConfig.ts` |
| **frontend** | `src/modules/crud/kanbanTransitionModel.ts` | `canMoveToColumn(from, target)` — pre-server | Único consumidor de `core-fsm` en frontend | `kanbanTransitionModel.ts:1` |

### 2.3 Convenciones del monorepo (verificadas)

- **Naming**: `{capacidad}/{runtime}/` (ej. `core/concurrency/go/`, `modules/crud/ui/ts/`).
- **Versioning**: archivo `VERSION` por runtime; tags git `{capacidad}/{runtime}/v{X.Y.Z}`.
- **npm**: `@devpablocristo/{core,modules}-{capacidad}` (ej. `@devpablocristo/core-fsm@^0.2.0`, `@devpablocristo/modules-crud-ui@^0.9.1`).
- **Go**: import `github.com/devpablocristo/{core,modules}/{capacidad}/{runtime}`.
- **Dependencias**: modules ⇒ core OK; core ⇒ modules NO; nadie ⇒ pymes.
- **Verificado**: `rg "github.com/devpablocristo/pymes" /home/pablocristo/Proyectos/pablo/core/` y en `/modules/` → 0 matches. ✅ Sin violaciones actuales.

---

## 3. Reglas de arquitectura propuestas

Estas reglas se documentarán en `pymes/ARCHITECTURE.md` y `pymes/AI_GUIDELINES.md` (Sección 13).

### 3.1 Regla de decisión (en orden estricto)

Antes de crear cualquier helper, archivo, paquete o abstracción, responder:

1. **¿Existe ya en `core` o `modules`?** Si sí → usar. Punto. No reinventar.
2. **¿Es una primitiva pura (sin HTTP, sin DB, sin RBAC, sin nombres de tablas, sin entidades de negocio)?** Sí → debería vivir en `core`. Si no existe el paquete, considerar PR upstream.
3. **¿Es un workflow/capacidad reusable que depende solo de primitivas de core?** Sí → debería vivir en `modules`. Justificar con ≥2 productos como consumidores potenciales.
4. **¿Depende de RBAC interno, Gin handler, audit names específicos, DTOs concretos, o cualquier decisión de negocio del producto?** Sí → vive en `pymes-core` (o un vertical).
5. **¿Es reusable pero hoy mezcla parte agnóstica + parte específica?** Separar: lo agnóstico va a `core`/`modules`, lo específico queda como adapter en `pymes-core`.
6. **¿Se repite en ≥2 dominios pero con diferencias menores?** Diseñar abstracción por capacidad (Archivable, StatusTransitionable, etc.) — pero solo si la abstracción reduce código netamente, no por mero deduplicar.
7. **¿Solo se usa una vez?** No abstraer.

### 3.2 Reglas de imports (enforcement con grep en CI)

| Capa | Puede importar | NO puede importar |
|---|---|---|
| `core/*` | otros paquetes de core (con cuidado) | `modules/*`, `pymes/*` |
| `modules/*` | `core/*` | `pymes/*` |
| `pymes-core/backend/internal/*` | `core/*`, `modules/*`, `pymes-core/shared/*`, otros internal hermanos vía interfaces | NUNCA estructuras GORM/handlers de otro internal |
| `pymes-core/shared/backend/*` | `core/*`, `modules/*` | `pymes-core/backend/internal/*` (back-dep) |
| `verticales (workshops/, professionals/, ...)` | core, modules, pymes-core/backend HTTP API | dominio interno de otros verticales |
| `frontend/src/*` | `@devpablocristo/core-*`, `@devpablocristo/modules-*` | librerías privadas de otros productos |

Comando de auditoría (entra a CI):
```bash
rg "github.com/devpablocristo/pymes" /home/pablocristo/Proyectos/pablo/core/ /home/pablocristo/Proyectos/pablo/modules/  # debe estar VACÍO
rg "pymes-core/backend/internal" /home/pablocristo/Proyectos/pablo/pymes/pymes-core/shared/  # debe estar VACÍO (back-dep)
```

### 3.3 Regla DURA: solo versiones publicadas de `core`/`modules`

**Prohibido absolutamente** consumir `core` y `modules` desde checkouts locales. Esto vale para Go y para el frontend.

| Tecnología | Permitido | PROHIBIDO |
|---|---|---|
| Go (`pymes/go.mod`) | `require github.com/devpablocristo/core/concurrency/go v0.1.1` (versión semántica publicada vía git tag) | `replace github.com/devpablocristo/core/concurrency/go => ../../core/concurrency/go` |
| Go (`go.work`) | NO usar para core/modules | `use ../core` |
| Frontend (`package.json`) | `"@devpablocristo/core-fsm": "^0.2.0"` (npm publicado) | `"@devpablocristo/core-fsm": "file:../../core/concurrency/fsm/ts"`, `"link:..."`, `npm link` |
| Verticales (workshops/, etc.) | mismos imports versionados | mismos prohibidos |

**Por qué**: los `replace` locales hacen que el código dependa del estado del checkout del dev. Cualquiera que clona un solo repo (sin tener `core` o `modules` al lado) se rompe; los CI builds que sí los tienen pueden divergir; los tags de versión pierden significado; los rollbacks se vuelven imposibles.

**Aplicación a este refactor**:
- PR 1 (`IsTerminal` upstream a `core`): debe **mergearse y publicarse con tag** (`concurrency/go/v0.1.2` o lo que toque) ANTES de que pymes pueda usarlo. Si demora, los demás PRs NO usan `IsTerminal` — siguen con `Validate` solamente.
- PR 7 (FE migra `buildFullyConnectedStatusStateMachine` → `core-fsm` Builder): solo si `@devpablocristo/core-fsm` está disponible en npm con la versión necesaria. Hoy ya está (`^0.2.0` en `package.json` línea 26). Si el PR de codegen futuro requiere versión nueva, se publica primero.
- Cualquier helper agnóstico que se quiera "pre-mover" a `core`/`modules` durante el refactor pero todavía sin publicar: **NO se mueve**. Queda local en pymes hasta que esté publicado.

**Verificación obligatoria en CI** (entra a Sección 10):
```bash
# Go: ningún replace de core/modules
grep -E "^replace.*devpablocristo/(core|modules)" /home/pablocristo/Proyectos/pablo/pymes/go.mod
# debe estar VACÍO

# Frontend: ningún file:/link: para devpablocristo
grep -E "\"@devpablocristo/.*\":\s*\"(file:|link:)" /home/pablocristo/Proyectos/pablo/pymes/frontend/package.json
# debe estar VACÍO
```

### 3.3 Reglas para Status/FSM

- **Primitivas de FSM** (Builder, Machine, errors): `core/concurrency/go/fsm` (Go) y `@devpablocristo/core-fsm` (TS). **Único origen**.
- **Grafos concretos** de cada dominio (sales, quotes, etc.): viven en el dominio (`pymes-core/backend/internal/<domain>/fsm.go`). Nunca subir grafos pymes a core/modules.
- **Mapeo FSM → HTTP error**: helper local en pymes-core (`RegisterStatusEndpoint` y `MapFSMError`). Justificación: el mensaje de error tiene que poder ser internacionalizado/customizado por producto.
- **Frontend**: cada `*Config.ts` debe declarar el MISMO grafo que el backend usando `@devpablocristo/core-fsm` Builder. Sin "fully-connected".
- **Ideal futuro (out of scope)**: generar el grafo desde una spec OpenAPI/JSON shared para que FE y BE no diverjan jamás. Hoy se sincronizan manualmente; la sincronización está documentada.

### 3.4 Reglas para CRUD

- **Soft-delete**: usar `archive.IfArchived(field, "resource")` de `modules/crud/archive/go`. El nombre del campo (`archived_at` o `deleted_at`) es decisión de schema de cada tabla; el helper acepta ambos. No hacer migración masiva.
- **Pagination**: usar `core/http/go/pagination` vía wrappers de `internal/shared/handlers/pagination.go`. No reinventar.
- **Path segments**: usar `modules/crud/paths/go` (`crudpaths.SegmentArchived`, etc.) en `RegisterRoutes` cuando aplique. Hoy solo `quotes` lo hace; el resto debería migrar (out of scope inmediato).
- **Audit/Timeline/Webhook ports**: ya unificados; mantener interface `AuditPort.Log(ctx, orgID, actor, action, resource, id, payload)`.
- **NO abstraer**: RegisterRoutes wiring, Tags/Notes/Favorite DTO fields, error mapping en handler — son código declarativo trivial.

---

## 4. Inventario de duplicaciones y divergencias

| Patrón | Dominios afectados | Diferencias actuales | Riesgo | Ubicación recomendada | Acción propuesta |
|---|---|---|---|---|---|
| Validación de status (BE) | sales, quotes, purchases | sales/quotes hardcoded set; purchases con switch + transiciones | Bug latente: drag a estado inválido lo guarda | `pymes-core/backend/internal/<dom>/fsm.go` con `core/concurrency/go/fsm` | **Migrar a FSM** (Sección 6) |
| Falta UpdateStatus (BE) | invoices | Status entra por PATCH /:id genérico, sin audit `invoice.status_updated` | No auditable, divergencia con sales/quotes/purchases | `pymes-core/backend/internal/invoices/` | **Agregar endpoint** + FSM |
| Helper register de endpoint /status | sales, quotes, purchases | ~40 líneas idénticas (parsing ID + body + mapeo error) | Drift accidental al copiar | `pymes-core/backend/internal/shared/handlers/status_endpoint.go` | **Crear `RegisterStatusEndpoint[T]`** (Sección 5.3) |
| FSM "fully-connected" en FE | sales, quotes, purchases, invoices | `buildFullyConnectedStatusStateMachine` permite todo→todo | Drag arbitrario (causa raíz UX bug) | `frontend/src/modules/billing/billingHelpers.ts` | **Reemplazar con `core-fsm` Builder** que espeje el BE |
| Endpoint paths hardcodeados (FE) | 4 configs | `/v1/sales/${id}/status`, etc. raw strings | Drift al refactorizar | Constantes locales (no merece módulo nuevo) | **Centralizar en `frontend/src/lib/endpoints.ts`** (PR auxiliar) |
| Status labels (i18n) | invoices vs resto | Solo invoices usa `INVOICE_STATUS_LABELS`; resto inline | Inconsistencia, no i18n | `frontend/src/modules/billing/<dom>StatusLabels.ts` | **Estandarizar constantes** (PR auxiliar, opcional) |
| Soft-delete column name | `archived_at` (9) vs `deleted_at` (6) | Inconsistencia de schema | Cognitivo + grep ruido | DB | **NO migrar columnas** (riesgo > beneficio). Documentar como excepción aceptada. |
| `IsTerminal(state)` Go | n/a | TS tiene `isTerminal`; Go no | Asimetría primitiva, fuerza workaround | `core/concurrency/go/fsm/builder.go` | **Agregar método** (PR upstream a core, opcional pero recomendado) |
| Decisión "emitir timeline si hay party_id" | sales (customer), purchases (supplier), quotes (no emite) | Variación de criterio | Inconsistencia menor | Cada usecase | **No mover**, documentar como decisión por dominio |
| Audit / Timeline / Webhook ports | 24 dominios | Interfaces idénticas | Ninguno | Como están | **NO tocar** |
| Pagination | 60+ usos | `ParseLimitQuery` wrapper común | Ninguno | `internal/shared/handlers/pagination.go` | **NO tocar** |

---

## 5. Diseño propuesto de librerías reutilizables

### 5.1 Cambios en `core`

#### 5.1.1 `core/concurrency/go/fsm` — agregar `IsTerminal` (opcional, alto valor)

- **Paquete**: `github.com/devpablocristo/core/concurrency/go/fsm`
- **Responsabilidad**: paridad con TS (`@devpablocristo/core-fsm` ya tiene `isTerminal`).
- **API propuesta** (en `builder.go`):
  ```go
  // IsTerminal informa si un estado fue marcado como terminal con Builder.Terminal(...).
  func (m *StringMachine) IsTerminal(state string) bool {
      _, ok := m.terminals[state]
      return ok
  }
  ```
- **Por qué es agnóstico**: reflexión pura sobre el grafo construido. Sin HTTP, DB, ni dominio.
- **Qué NO debe incluir**: nada de eventos, audit, ni mapping HTTP.
- **Tests**: agregar a `fsm_test.go` casos `terminal: true`, `non-terminal: false`, `unknown: false`.
- **Coordinación**: requiere PR upstream al monorepo `core`. Si esto es out-of-scope, **mitigar localmente** con un helper en pymes:
  ```go
  // pymes-core/backend/internal/shared/fsmext/is_terminal.go
  func IsTerminal(m *fsm.StringMachine, state string) bool {
      // Truco: una transición a estado distinto desde un terminal devuelve ErrTerminal,
      // de un no-terminal sin regla devuelve ErrInvalidTransition.
      if state == "" { return false }
      err := m.Validate(state, state+"_probe")  // probe state que nunca existe
      return errors.Is(err, fsm.ErrTerminal)
  }
  ```
  El helper local sería temporal y se reemplazaría tras el PR upstream.

**Decisión**: incluir el PR upstream en el plan (Sección 8 PR 1) **pero hacerlo independiente** — si demora la review, los demás PRs no quedan bloqueados.

#### 5.1.2 NO se sube nada más a core

Considerados y descartados:
- ❌ Helper `MapFSMError(current, next, err) error` que devuelva `domainerr.Conflict(...)` — el mensaje formateado y la decisión de qué Kind usar (Conflict vs BusinessRule vs Validation) puede variar por producto. Va en pymes.
- ❌ `core/http/gin/go.RegisterStatusEndpoint` — depende de RBAC, auth ctx, response mapper que son de pymes.

### 5.2 Cambios en `modules`

**No se crean módulos nuevos** en este plan. Justificación:

- `modules/workflow/status/go` (idea inicial): tendría que parametrizar audit, timeline, webhook, response mapping — termina siendo un mini-framework rígido. La capacidad ya está cubierta entre `core/concurrency/go/fsm` (primitiva) y un helper local en pymes (adapter HTTP).
- `modules/crud/handler-helpers/go`: el wiring HTTP+RBAC+ctx es 99% específico de pymes (pyme tiene su `RBACMiddleware`, su `GetAuthContext`, su pattern de `httperrors.Respond`). Otro producto tendría su propio wiring.

**Lo que sí mejora en modules** (sin crear paquete nuevo):
- `modules/crud/paths/go` ya existe pero solo `quotes` lo usa en pymes. **Acción** (PR auxiliar, fuera del scope de status): migrar `RegisterRoutes` de los demás dominios para usar `crudpaths.SegmentArchived` y similares. Reduce drift.

### 5.3 Cambios en `pymes-core`

#### 5.3.1 `MapFSMError` (capa aplicación, agnóstica de HTTP)

- **Path**: `pymes-core/backend/internal/shared/status/fsm_errors.go`
- **Por qué este path y no `handlers/`**: el usecase llama a esta función. Si vive en `handlers/`, el usecase termina dependiendo de la capa HTTP (back-dep prohibido por arquitectura hexagonal).
- **Dependencias permitidas**: `core/concurrency/go/fsm` + `core/errors/go/domainerr` + `fmt`/`errors` (stdlib). NADA de Gin, RBAC, http.
- **Firma**:
  ```go
  package status

  import (
      "errors"
      "fmt"

      "github.com/devpablocristo/core/concurrency/go/fsm"
      "github.com/devpablocristo/core/errors/go/domainerr"
  )

  // MapFSMError envuelve sentinels de fsm en domainerr.Conflict para que
  // httperrors.Respond los mapee a 409 con mensajes legibles.
  // Los usecases lo invocan tras `<dom>StateMachine.Validate(current, next)`.
  func MapFSMError(current, next string, err error) error {
      if err == nil {
          return nil
      }
      if errors.Is(err, fsm.ErrTerminal) {
          return domainerr.Newf(domainerr.KindConflict, "status %q is terminal", current)
      }
      if errors.Is(err, fsm.ErrInvalidTransition) {
          return domainerr.Newf(domainerr.KindConflict, "status transition not allowed: %s -> %s", current, next)
      }
      return err
  }
  ```
- **Tests**: `fsm_errors_test.go` con tabla `[ErrInvalidTransition, ErrTerminal, nil, errFoo]`.

#### 5.3.2 `RegisterStatusEndpoint[T]` (capa HTTP/Gin)

- **Path**: `pymes-core/backend/internal/shared/handlers/status_endpoint.go`
- **Justificación de ubicación**: depende de `RBACMiddleware`, `GetAuthContext`, `WriteValidation` que viven en el mismo paquete. Mover a `pymes-core/shared/backend/` introduciría back-dep.
- **NO importa `internal/shared/status`**: el usecase ya devuelve un `domainerr.Conflict` (vía `MapFSMError`); el handler solo invoca `httperrors.Respond(c, err)` que lo mapea a 409. Las dos capas se conectan vía error tipado, no vía import.
- **Firma**:
  ```go
  package handlers

  type StatusUpdater[T any] func(ctx context.Context, orgID, id uuid.UUID, nextStatus, actor string) (T, error)
  type StatusResponseMapper[T any] func(T) any

  // RegisterStatusEndpoint registra PATCH <basePath>/:id/status con parsing uniforme.
  func RegisterStatusEndpoint[T any](
      auth *gin.RouterGroup,
      rbac *RBACMiddleware,
      resource, permission, basePath string,
      update StatusUpdater[T],
      mapper StatusResponseMapper[T],
  ) {
      auth.PATCH(basePath+"/:id/status",
          rbac.RequirePermission(resource, permission),
          func(c *gin.Context) {
              a := GetAuthContext(c)
              orgID, err := uuid.Parse(a.OrgID)
              if err != nil { WriteValidation(c, "invalid tenant"); return }
              id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
              if err != nil { WriteValidation(c, "invalid id"); return }
              var req struct { Status string `json:"status" binding:"required"` }
              if err := c.ShouldBindJSON(&req); err != nil { WriteValidation(c, "invalid request body"); return }
              next := strings.TrimSpace(strings.ToLower(req.Status))
              if next == "" {
                  httperrors.Respond(c, domainerr.Validation("status is required")); return
              }
              out, err := update(c.Request.Context(), orgID, id, next, a.Actor)
              if err != nil { httperrors.Respond(c, err); return }
              c.JSON(http.StatusOK, mapper(out))
          })
  }
  ```
- **Consumidores**: sales, quotes, purchases, invoices. Cada uno declara su FSM en `<dom>/fsm.go` y registra el endpoint en `RegisterRoutes`.

**Diagrama de dependencias resultante** (sin back-deps):

```
  handler.go                         usecases.go
      │                                  │
      ├─ imports ────────────────────────│
      │   shared/handlers (RegisterStatusEndpoint)   shared/status (MapFSMError)
      │                                  │
      │                                  ├─ imports
      │                                  │   core/concurrency/go/fsm
      │                                  │   core/errors/go/domainerr
      │                                  │   <dom>/fsm.go (saleStateMachine)
      │
      └─ httperrors.Respond(c, err) ──→ core/http/gin/go.Respond
                                        (mapea domainerr.Conflict → 409 JSON)
```

Las flechas son unidireccionales. El usecase NO importa `handlers`. El handler NO importa `status` (recibe el `domainerr` del usecase).

#### 5.3.3 FSM por dominio (4 archivos nuevos)

- `pymes-core/backend/internal/sales/fsm.go`
- `pymes-core/backend/internal/quotes/fsm.go`
- `pymes-core/backend/internal/purchases/fsm.go`
- `pymes-core/backend/internal/invoices/fsm.go`

Grafos concretos en Sección 6. Cada uno expone `var <domain>StateMachine = fsm.NewBuilder()....Build()`.

#### 5.3.4 Refactor de `UpdateStatus` por dominio

Cada dominio:
1. Lee `current` con `repo.GetByID` (sales y quotes hoy NO lo hacen — agregar).
2. Aplica `archive.IfArchived(current.<DeletedOrArchivedAt>, "<resource>")`.
3. Llama `<domain>StateMachine.Validate(current.Status, next)` → error envuelto con `status.MapFSMError(current, next, err)` (import de `internal/shared/status`).
4. Persiste con `repo.UpdateStatus` (o `SetStatus`).
5. Emite audit/timeline/webhook (sin cambios de side effects, salvo que quotes ahora también emite timeline si hay `customer_id` para uniformidad — **decisión opcional, marcar como breaking**).

**Política `same-status` (idempotente)**: el FSM de `core` retorna `nil` para `from == to` (verificado en `core/concurrency/go/fsm/builder.go:CanTransition`). Los usecases NO deben rechazar `same-status` — devuelven 200 con la entidad sin tocar. Si el dominio quiere rechazarlo (ej. terminal idempotente vs conflict), debe manejarlo explícitamente. Tests cubren ambos casos.

#### 5.3.5 invoices — endpoint nuevo + ajuste de Update + detección explícita de `status`

- **Agregar** `UpdateStatusInput`, método `UpdateStatus` en usecases con FSM + audit `invoice.status_updated`.
- `repository.go`: agregar `UpdateStatus(ctx, orgID, id, status)` (UPDATE simple).
- DTO: `UpdateInvoiceStatusRequest{Status string}` en `handler/dto/dto.go`.
- **Quitar `Status *string` de `UpdateInvoiceRequest`**. **PERO** Gin ignora silenciosamente campos JSON desconocidos — sacar el campo del DTO NO devuelve 400 automáticamente.
- **Detección explícita en handler `Update`** (líneas a editar en `invoices/handler.go`):
  ```go
  func (h *Handler) Update(c *gin.Context) {
      // Leer body como mapa raw para detectar status sin tocar el DTO tipado.
      bodyBytes, err := io.ReadAll(c.Request.Body)
      if err != nil {
          handlers.WriteValidation(c, "invalid request body")
          return
      }
      var raw map[string]json.RawMessage
      if err := json.Unmarshal(bodyBytes, &raw); err != nil {
          handlers.WriteValidation(c, "invalid request body")
          return
      }
      if _, hasStatus := raw["status"]; hasStatus {
          httperrors.Respond(c, domainerr.Validation("use PATCH /invoices/:id/status to change status"))
          return
      }
      // Re-bind al DTO tipado tras la validación.
      var req dto.UpdateInvoiceRequest
      if err := json.Unmarshal(bodyBytes, &req); err != nil {
          handlers.WriteValidation(c, "invalid request body")
          return
      }
      // ... resto del handler
  }
  ```
  Esto es localizado al handler de invoices.Update; no afecta otros dominios.
- `handler.go`: agregar ruta vía `RegisterStatusEndpoint`. Mantener `PATCH /:id` para favorite/tags/notes con la detección anterior.

Side effects que **conserva**: PATCH genérico para favorite/tags/notes/dates. Audit log de update genérico. **No emite timeline ni webhook por status hoy**; agregarlo es opcional (decisión por dominio).

### 5.4 Cambios en frontend

#### 5.4.1 Reemplazar `buildFullyConnectedStatusStateMachine` por `core-fsm` Builder

- **Archivo afectado**: `frontend/src/modules/billing/billingHelpers.ts` (función actual `buildFullyConnectedStatusStateMachine`).
- **Acción**: deprecar la función. En cada `billing<Sales|Quotes|Purchases|Invoices>Config.ts`, reemplazar su uso con un Builder de `core-fsm` que **espeje** el grafo del backend.
- **Nueva fuente de verdad** (por dominio): `frontend/src/modules/billing/<dom>StateMachine.ts`:
  ```ts
  import { Builder, type StringMachine } from '@devpablocristo/core-fsm';

  export const salesStateMachine: StringMachine = new Builder()
      .terminal('voided')
      .freeTransitionsAmong('draft', 'pending')
      .allow('draft', 'completed')
      .allow('pending', 'completed')
      .allow('completed', 'paid')
      .allowAnyTo('voided')
      .build();
  ```
- **Riesgo**: si el frontend y el backend declaran grafos distintos, vuelve la divergencia. **Mitigación**: tests que importen ambas declaraciones (la de Go y la de TS) NO es viable directamente — se requiere un test de "shape" (lista de transiciones esperadas hardcoded) en ambos lados. PR 9 incluye este test.

#### 5.4.2 Centralizar endpoints

- **Archivo nuevo**: `frontend/src/lib/endpoints.ts`:
  ```ts
  export const billingEndpoints = {
      sales:     { base: '/v1/sales',     status: (id: string) => `/v1/sales/${id}/status` },
      quotes:    { base: '/v1/quotes',    status: (id: string) => `/v1/quotes/${id}/status` },
      purchases: { base: '/v1/purchases', status: (id: string) => `/v1/purchases/${id}/status` },
      invoices:  { base: '/v1/invoices',  status: (id: string) => `/v1/invoices/${id}/status` },
  } as const;
  ```
- **Archivos a modificar**: `billingSalesConfig.ts`, `billingQuotesConfig.ts`, `billingPurchasesConfig.ts`, `lib/invoicesApi.ts`. Reemplazar strings raw.

#### 5.4.3 Migrar `updateInvoiceStatus`

- **Archivo**: `frontend/src/lib/invoicesApi.ts:190-192`
  ```ts
  // Antes:
  await apiRequest(`/v1/invoices/${id}`, { method: 'PATCH', body: { status } });
  // Después:
  await apiRequest(billingEndpoints.invoices.status(id), { method: 'PATCH', body: { status } });
  ```
- También en `frontend/src/modules/billing/billingInvoicesConfig.ts`: quitar el `if (values.status !== undefined) body.status = ...` del builder de `Update` genérico. Status ya no entra por ahí.

---

## 6. Rediseño específico Status/FSM

### 6.1 Crítica de las FSM propuestas en el plan anterior

#### Sales

**Plan anterior**: `paid → voided` permitido vía AllowAnyTo.

**Problema real**: el método `Void()` (línea 262 de `sales/handler.go`) tiene side effects contables que `UpdateStatus` no replica (reverse stock + reverse cashflow). Si dejamos `paid → voided` en el FSM de UpdateStatus, el usuario puede saltarse la lógica contable arrastrando la card al kanban.

**Decisión revisada**: `paid` es **terminal** en UpdateStatus. La anulación de una venta cobrada va por `POST /sales/:id/void` (handler dedicado). El FSM declara:

```go
var saleStateMachine = fsm.NewBuilder().
    Terminal("voided", "paid").  // paid también terminal en UpdateStatus
    FreeTransitionsAmong("draft", "pending").
    Allow("draft", "completed").
    Allow("pending", "completed").
    Allow("completed", "paid").
    AllowAnyTo("voided").  // global desde no-terminales
    Build()
```

Esto significa que el kanban no permite arrastrar cards `paid → voided`; el usuario debe usar el botón "Anular venta" que invoca `Void()`. **Marcar como breaking** en CHANGELOG.

#### Quotes

**Plan anterior**: `accepted` terminal.

**Problema real**: el método `ToSale` (línea 452 de `quotes/usecases.go`) llama a `repo.SetStatus(quoteID, "accepted")` después de convertir a venta — pero solo si el quote no está ya en `accepted`. Si `accepted` es terminal, `SetStatus(accepted → accepted)` funciona (from==to siempre permitido por `CanTransition`).

**Validación adicional**: los métodos `Send`, `Accept`, `Reject` no invocan FSM hoy. Si los hago delegar a `UpdateStatus`, pasan por el FSM.

**Decisión**: el FSM propuesto está bien. Send/Accept/Reject delegan a UpdateStatus. ToSale conserva su lógica.

```go
var quoteStateMachine = fsm.NewBuilder().
    Terminal("accepted", "rejected", "expired").
    Allow("draft", "sent").
    AllowFromStatesTo("rejected", "draft", "sent").
    AllowFromStatesTo("expired", "draft", "sent").
    Allow("sent", "accepted").
    Build()
```

#### Purchases — DECISIÓN CERRADA: opción A (`voided` NO terminal)

**Justificación**: el grafo actual (`canTransitionPurchaseStatus` líneas 314-335 de `purchases/usecases.go`) permite `voided → draft|partial|received|voided`. **Cambiar a terminal sería breaking** y el plan no puede dejar A/B abierto con verificación SQL como excusa para improvisar durante la implementación.

**Decisión firme**: opción A. Conservamos comportamiento actual.

```go
var purchaseStateMachine = fsm.NewBuilder().
    FreeTransitionsAmong("draft", "partial", "received", "voided").
    Build()
```

**Si el producto en el futuro pide hacer `voided` terminal**: PR separado, breaking change explícito, con:
1. Aprobación de producto/contabilidad.
2. Verificación SQL (`SELECT COUNT(*) FROM purchases WHERE status='voided' AND updated_at > created_at + interval '1 minute'`) como precondición de merge.
3. Migración de datos si aplica.

Ese PR está fuera del scope de este refactor.

#### Invoices

**Plan anterior**: `paid` terminal, `pending → overdue → paid`.

**Validación**: `invoicesDemo.ts` muestra solo 3 estados (`paid`, `pending`, `overdue`). Sin `voided`. Sin `cancelled`. El CHECK constraint de DB acepta solo esos.

**Decisión**: el grafo está bien.

```go
var invoiceStateMachine = fsm.NewBuilder().
    Terminal("paid").
    Allow("pending", "overdue").
    Allow("pending", "paid").
    Allow("overdue", "paid").
    Build()
```

⚠️ **Nota**: si en el futuro se agrega "voided" para anular facturas, requiere migración de schema + extensión del grafo. No incluido en este plan.

### 6.2 Flujo de validación de transición (canónico)

```
1. Frontend: drag-and-drop
   └─ kanbanTransitionModel.canMoveToColumn(from, to)
      └─ <dom>StateMachine.canTransition(from, to)  // core-fsm en TS
         └─ si false: bloquear visualmente (no permitir drop)

2. Si pasa, FE invoca persistMove → apiRequest(PATCH /v1/<dom>/:id/status, {status})

3. Backend: handlers.RegisterStatusEndpoint
   └─ parsea ID + body
   └─ llama uc.UpdateStatus(ctx, orgID, id, next, actor)
      └─ repo.GetByID → current
      └─ archive.IfArchived(current.ArchivedAt, "<dom>") → si archived, 409
      └─ <dom>StateMachine.Validate(current.Status, next)
         ├─ ErrInvalidTransition → handlers.MapFSMError → domainerr.Conflict → 409
         ├─ ErrTerminal → handlers.MapFSMError → domainerr.Conflict → 409
         └─ nil → continúa
      └─ repo.UpdateStatus(...)
      └─ audit.Log(ctx, ..., "<dom>.status_updated", ...)
      └─ timeline.RecordEvent(...) [si dominio aplica]
      └─ webhook.Enqueue(...) [si dominio aplica]
   └─ mapper(out) → JSON
```

### 6.3 Side effects post-transición

| Dominio | Audit | Timeline | Webhook |
|---|---|---|---|
| sales | `sale.status_updated` | si `customer_id != nil` | sí (canónico) |
| quotes | `quote.status_updated` | **agregar** si `customer_id != nil` (uniformidad) | **agregar** si configurado |
| purchases | `purchase.status_updated` | si `supplier_id != nil` | sí |
| invoices | `invoice.status_updated` (NUEVO) | opcional, decidir por dominio | opcional |

**Garantía**: `Validate` se llama **antes** de `repo.UpdateStatus` y antes de los emit. Si `Validate` falla → return temprano → sin escritura ni eventos. Tests cubren esto.

### 6.4 Sincronización FE↔BE — fases

**Hoy**: divergencia silenciosa — FE permite todo (full-connected), BE no validaba (sales/quotes) o sí validaba pero diferente (purchases). El usuario arrastra y recibe 409.

**Fase inicial (este plan, PR 7 + PR 9)**: tabla constante en ambos lados con comentario `// MUST MATCH backend/.../<dom>/fsm.go` (y viceversa).
- Mitigación parcial: code review obligatorio cruzado FE↔BE cuando se modifique uno.
- Limitación honesta: NO previene drift automáticamente; depende de la disciplina humana.
- Test "shape match" (Sección 9.6) — Go test con tabla constante, TS test con la misma tabla. Si alguien edita un lado sin el otro, el test del lado no editado sigue pasando, pero un PR que toque la tabla obliga a tocar las dos versiones (visible en el diff).

**Fase futura obligatoria (PR fuera de scope, decidir antes de cerrar este refactor)**: una de tres opciones.

| Opción | Fuente de verdad | Trade-off |
|---|---|---|
| **A — Spec JSON/YAML compartida** | `pymes/config/status-workflows/<dom>.json` con `{"states":[...], "transitions":[{"from":"draft","to":"sent"}]}`. Ambos lados leen en build-time vía codegen. | Requiere build pipeline en ambos lados; cambio toca 1 archivo; FE↔BE garantizado. Necesita codegen Go + TS. |
| **B — Endpoint backend de metadata** | `GET /v1/meta/status-workflows/<resource>` devuelve la lista canónica. FE hace fetch en build-time (genera consts) o runtime (consume y memoriza). | Mantiene SoT en backend; cambio en FSM del backend propaga sin tocar FE. Nightly test cross-check. Costo: endpoint nuevo, build pipeline FE. |
| **C — Codegen desde FSM Go** | Script `tools/fsm-export/main.go` que importa los FSM de cada dominio y emite `frontend/src/modules/billing/<dom>StateMachine.gen.ts`. CI corre el script y falla si el archivo está stale. | SoT en backend; FE consume tipos+constantes; sin endpoint runtime; requiere script + CI step. |

**Recomendación de evaluación**: opción C (codegen) — mínima fricción operacional, máxima garantía. Pero requiere un PR dedicado fuera de scope. Listar como issue `pymes-arch-followup-fsm-codegen`.

**Mientras tanto**: PR 9 incluye los tests de shape match con comentarios `MUST MATCH` y revisión cruzada en code review.

### 6.5 Riesgos breaking

| Cambio | Breaking | Mitigación |
|---|---|---|
| `paid → voided` ya no permitido vía UpdateStatus en sales | Sí | Documentar "usar Void()". Verificar con `git log -p` si alguien usa `UpdateStatus(saleID, "voided")` en el código. |
| `voided → *` ya no permitido en purchases (opción B) | Sí | Verificar conteo en DB. Si problemático, fallback a opción A. |
| `PATCH /invoices/:id` con `status` en body devuelve 400 | Sí | Frontend migrado en mismo PR. |
| `Send/Accept/Reject` en quotes ahora pasan por FSM | Sí menor | El FSM permite los mismos pares que la lógica anterior. Tests existentes adaptados. |

---

## 7. Rediseño específico CRUD

### 7.1 Capacidades equivalentes (extracción evaluada)

| Capacidad | Repetición | Diferencias | Decisión |
|---|---|---|---|
| `Archive`/`Restore`/`HardDelete` repository | ~12 dominios con misma estructura | Algunos `archived_at`, otros `deleted_at` | **NO extraer a interface común** ahora. `archive.IfArchived` ya cubre lo crítico. Documentar. |
| `RegisterRoutes` patterns | 36 dominios | Idéntico salvo paths | NO extraer (trivial) |
| Audit/Timeline/Webhook ports | 24 dominios | Identical interface | YA extraído |
| Pagination | 60+ usos | Idéntico | YA extraído (`core/http/go/pagination`) |
| Tags/Notes/Favorite DTO fields | 5 dominios | Idéntico | NO extraer (declarativo) |
| `path segments` (`/archived`, `/restore`, `/archive`, `/hard`) | quotes lo usa con `crudpaths`; otros raw | Drift potencial | **Migrar** sales/purchases/invoices a `crudpaths` en PR auxiliar |
| Status enum + FSM | 4 dominios | Hardcoded vs switch vs nada | Sección 6 |
| List + filtros (branch_id, customer_id, status, dates) | sales, quotes, purchases, invoices, returns | Cada uno parsea manual | **NO extraer aún**. Patrón es declarativo y sus diferencias son reales (filtros distintos). |

### 7.2 Lo que NO se hace (y por qué)

- **NO se crea un mega "CRUDFramework"**: cada `Create/Update/Delete` tiene validaciones de dominio específicas (reglas de negocio). Un framework genérico forzaría todos los dominios a un molde y reduciría legibilidad.
- **NO se migra `deleted_at` → `archived_at`**: requiere migración SQL + cambios en GORM models + bug-prone. El helper `archive.IfArchived` ya abstrae el nombre.
- **NO se extrae "audit + status_updated event" a un wrapper**: el evento se nombra distinto por dominio (`sale.status_updated`, etc.) y el payload puede variar. Local en cada usecase.
- **NO se aborda `customer_messaging`, `dashboard`, `reports`, `pdfgen`**: están fuera del patrón CRUD canónico.

### 7.3 Camino incremental para CRUD

Después de los PRs de status (Sección 8 PR 1-7):

- **PR aux 1**: migrar sales/purchases/invoices a usar `crudpaths.SegmentArchived` etc. (out of scope inmediato pero recomendado).
- **PR aux 2**: estandarizar status labels en frontend (constantes `<dom>_STATUS_LABELS` en cada módulo). Out of scope inmediato.
- **PR futuro**: explorar generación TS desde OpenAPI del backend (incluido status enum). Requiere ampliar la spec OpenAPI más allá de AI chat.

---

## 8. Plan de PRs incremental

### PR 0 — Documentación arquitectónica

**Objetivo**: consolidar `ARCHITECTURE.md` y `AI_GUIDELINES.md` ANTES de los cambios técnicos.

- **Archivos a crear**:
  - `pymes/ARCHITECTURE.md` (Sección 13)
  - `pymes/AI_GUIDELINES.md` (Sección 13)
  - actualizar `pymes/CLAUDE.md` con referencia a ambos.
- **Tests**: ninguno (solo docs).
- **Verificación**: review humano.
- **Riesgo**: bajo.
- **Aceptación**: docs aprobados.

### PR 1 — `core/concurrency/go/fsm.IsTerminal` (OPCIONAL, paralelo, NO bloqueante)

**Objetivo**: paridad con TS (`@devpablocristo/core-fsm` ya tiene `isTerminal`).

**Importante**: este refactor NO lo necesita. Para validar transición usamos `Validate(from, to)` que ya devuelve `ErrTerminal` si corresponde. `IsTerminal` solo aporta legibilidad si en algún momento un test/UI quiere preguntar directamente "¿este estado es terminal?". **Sin workarounds tipo `Validate(s, s+"_probe")` — eso queda explícitamente prohibido**.

- **Pasos del PR upstream a `core`**:
  1. Modificar `/home/pablocristo/Proyectos/pablo/core/concurrency/go/fsm/builder.go` (agregar método).
  2. API nueva: `func (m *StringMachine) IsTerminal(state string) bool { _, ok := m.terminals[state]; return ok }`.
  3. Agregar tests a `fsm_test.go` table-driven (terminal/no-terminal/unknown).
  4. **Bump `VERSION` de `core/concurrency/go`** (ej. `0.1.1` → `0.1.2`).
  5. Tag git: `concurrency/go/v0.1.2` y push.
  6. **Recién después** se actualiza `pymes/go.mod` con `require github.com/devpablocristo/core/concurrency/go v0.1.2`. **NO usar `replace` local intermedio**.
- **Verificación**:
  ```bash
  cd /home/pablocristo/Proyectos/pablo/core/concurrency/go
  go test ./fsm/...
  # tras tag y bump:
  cd /home/pablocristo/Proyectos/pablo/pymes
  go get github.com/devpablocristo/core/concurrency/go@v0.1.2
  go mod tidy
  ```
- **Riesgo**: bajo, sin breaking.
- **Independencia**: PR paralelo a los de pymes. Si demora la review/publish en `core`, los PRs 2-9 de pymes mergeable sin esperar (`IsTerminal` no es requerido).
- **Aceptación**: tests verdes en `core`, tag `concurrency/go/v0.1.2` publicado, `pymes/go.mod` apunta al tag.

### PR 2 — Helper `MapFSMError` (status) + `RegisterStatusEndpoint` (handlers)

**Objetivo**: tener ambos helpers listos antes de migrar dominios. Dos archivos en dos paquetes distintos para preservar la dirección de dependencias.

- **Archivos a crear**:
  - `pymes-core/backend/internal/shared/status/fsm_errors.go` (`MapFSMError`)
  - `pymes-core/backend/internal/shared/status/fsm_errors_test.go`
  - `pymes-core/backend/internal/shared/handlers/status_endpoint.go` (`RegisterStatusEndpoint[T]`)
  - `pymes-core/backend/internal/shared/handlers/status_endpoint_test.go`
- **APIs nuevas**: ver Sección 5.3.1 + 5.3.2.
- **Tests**:
  - `status/fsm_errors_test.go`: tabla [`ErrInvalidTransition` → Conflict; `ErrTerminal` → Conflict; `nil` → nil; error genérico → passthrough].
  - `handlers/status_endpoint_test.go` (httptest mockeando `StatusUpdater`): 200 ok, 400 body inválido, 400 status vacío, 400 uuid inválido, 409 si updater devuelve `domainerr.Conflict`, 404 si `domainerr.NotFoundf`, 500 si error genérico.
- **Verificación**:
  ```bash
  cd /home/pablocristo/Proyectos/pablo/pymes
  go test ./pymes-core/backend/internal/shared/status/... -race -count=1
  go test ./pymes-core/backend/internal/shared/handlers/... -race -count=1
  go vet ./pymes-core/backend/internal/shared/...
  ```
- **Verificación de no back-dep**:
  ```bash
  rg "internal/shared/handlers" /home/pablocristo/Proyectos/pablo/pymes/pymes-core/backend/internal/shared/status/
  # debe estar VACÍO
  ```
- **Riesgo**: bajo, sin consumidores.
- **Aceptación**: tests verdes, no back-dep.

### PR 3 — Sales: FSM + refactor

**Objetivo**: migrar sales a FSM compartido.

- **Archivos a crear**: `pymes-core/backend/internal/sales/fsm.go`.
- **Archivos a modificar**:
  - `pymes-core/backend/internal/sales/usecases.go` (líneas 363-391, 446-453): refactor `UpdateStatus`. Eliminar `isValidSaleStatus`. Agregar `repo.GetByID`. Llamar `saleStateMachine.Validate`. Wrap con `handlers.MapFSMError`.
  - `pymes-core/backend/internal/sales/handler.go` (líneas 39, 264-291): eliminar método handler `UpdateStatus`. Reemplazar con `handlers.RegisterStatusEndpoint(...)`.
  - `pymes-core/backend/internal/sales/usecases_test.go`: agregar `TestSale_UpdateStatus_FSM` table-driven.
- **APIs nuevas**: `var saleStateMachine *fsm.StringMachine`.
- **Código eliminado**: `isValidSaleStatus`, `validSaleStatuses`, método `UpdateStatus` del Handler (queda solo el de `Usecases`).
- **Tests**: 9 casos:
  1. transición válida (ej. `draft → completed`) → 200
  2. transición inválida (ej. `draft → paid`) → 409 `ErrInvalidTransition`
  3. desde terminal (`voided → draft`) → 409 `ErrTerminal`
  4. desde terminal (`paid → voided`) → 409 `ErrTerminal` (paid es terminal en UpdateStatus)
  5. status vacío → 400 validation
  6. status desconocido (no en grafo) → 409 `ErrInvalidTransition`
  7. **same-status (`draft → draft`) → 200 idempotente**
  8. **same-status desde terminal (`paid → paid`) → 200 idempotente**
  9. archivado → 409
  + invariante: side effects (audit/timeline/webhook) NO se disparan en casos 2-6.
- **Verificación**:
  ```bash
  cd /home/pablocristo/Proyectos/pablo/pymes
  go test ./pymes-core/backend/internal/sales/... -race -count=1
  ```
- **Riesgo**: medio. Solo `paid → voided` cambia (ahora rechazado en `UpdateStatus`; sigue funcionando vía `POST /sales/:id/void`). Las transiciones permitidas se documentan literalmente en el comentario de `fsm.go` para evitar interpretaciones (no usar frases tipo "*→ pending").
- **Aceptación**: tests verdes, smoke manual del kanban (drag entre columnas, drag inválido devuelve 409).

### PR 4 — Quotes: FSM + refactor + Send/Accept/Reject delegation

- **Archivos a crear**: `pymes-core/backend/internal/quotes/fsm.go`.
- **Archivos a modificar**:
  - `quotes/usecases.go` (líneas 137-176, 143-155, 398-450): refactor `UpdateStatus` (agregar `GetByID`); reescribir `Send/Accept/Reject` para delegar en `UpdateStatus`. Eliminar `validQuoteStatuses`, `isValidQuoteStatus`.
  - `quotes/handler.go` (líneas 55, 315-342): adoptar `RegisterStatusEndpoint`.
  - `quotes/usecases_test.go`: actualizar tests existentes de `Send/Accept/Reject` (la nueva validación es FSM, no `current.Status != "draft"`). Agregar `TestQuote_UpdateStatus_FSM`.
- **APIs nuevas**: `var quoteStateMachine *fsm.StringMachine`.
- **Código eliminado**: `validQuoteStatuses`, `isValidQuoteStatus`, validación inline en Send/Accept/Reject.
- **Tests**: igual a PR 3 + tests específicos para Send/Accept/Reject (que ahora pasan por FSM).
- **Verificación**:
  ```bash
  go test ./pymes-core/backend/internal/quotes/... -race -count=1
  ```
- **Riesgo**: medio. Send/Accept/Reject pueden fallar si el grafo difiere de la lógica anterior. **Mitigación**: en el código actual, Send solo permite si `current=draft`, Accept solo si `current=sent`, Reject si `current in {draft, sent}`. El FSM propuesto permite los mismos pares.
- **Aceptación**: tests verdes, ToSale sigue funcionando.

### PR 5 — Purchases: FSM + refactor

- **Archivos a crear**: `pymes-core/backend/internal/purchases/fsm.go`.
- **Archivos a modificar**:
  - `purchases/usecases.go` (181-217, 301-335): reemplazar `canTransitionPurchaseStatus` por `purchaseStateMachine.Validate`. Conservar `markReceivedAt`.
  - `purchases/handler.go` (línea 41, 156-177): adoptar `RegisterStatusEndpoint`.
  - `purchases/usecases_test.go`: ya existe (único dominio con `TestUpdateStatus*` previo). Adaptar.
- **APIs nuevas**: `var purchaseStateMachine *fsm.StringMachine` (con `voided` NO terminal — ver decisión cerrada en Sección 6.1).
- **Código eliminado**: `canTransitionPurchaseStatus`. `normalizePurchaseStatus` se conserva pero solo para `prepareCreate` (default + lower/trim).
- **Tests**: existentes adaptados + casos `same-status` idempotente + transiciones libres entre los 4 estados.
- **Verificación**:
  ```bash
  cd /home/pablocristo/Proyectos/pablo/pymes
  go test ./pymes-core/backend/internal/purchases/... -race -count=1
  ```
- **Riesgo**: bajo (preserva comportamiento). NO breaking porque `voided` sigue NO terminal.
- **Aceptación**: tests verdes, smoke manual.

### PR 6 — Invoices: endpoint nuevo + detección explícita de `status` + frontend

- **Archivos a crear**:
  - `pymes-core/backend/internal/invoices/fsm.go`
  - `pymes-core/backend/internal/invoices/usecases_test.go` (no existe hoy)
- **Archivos a modificar**:
  - `invoices/usecases.go`: agregar `UpdateStatusInput`, método `UpdateStatus`. **Eliminar el bloque `if in.Status != nil`** del `Update` genérico (ya no aceptamos status por ahí).
  - `invoices/handler.go`: **agregar detección explícita** de `status` en el handler `Update` (snippet en Sección 5.3.5). NO depender de "Gin descarta el campo": Gin lo ignora silenciosamente, así que sin la detección un cliente que mande `{"status":"paid"}` no se entera de que el campo se descartó.
  - `invoices/repository.go`: agregar `UpdateStatus(ctx, orgID, id, status)`.
  - `invoices/handler/dto/dto.go`: agregar `UpdateInvoiceStatusRequest{Status string}`. Quitar `Status *string` de `UpdateInvoiceRequest`.
  - **Frontend**:
    - `frontend/src/lib/invoicesApi.ts:190-192`: `updateInvoiceStatus` apunta a `/v1/invoices/${id}/status` (PATCH).
    - `frontend/src/modules/billing/billingInvoicesConfig.ts` (~165): quitar el bloque que mete `status` en el body de Update genérico.
- **APIs nuevas**: `var invoiceStateMachine`, endpoint `PATCH /v1/invoices/:id/status`.
- **Código eliminado**: el bloque de status del Update genérico (BE) y del builder de body (FE).
- **Tests**:
  - Backend `usecases_test.go`: igual al patrón de los anteriores (válido / inválido / terminal / empty / archived / same-status idempotente).
  - Backend `handler_test.go`: caso explícito **"PATCH /invoices/:id con `{status:'paid'}` → 400 con mensaje `use PATCH /invoices/:id/status to change status`"**.
  - Frontend: actualizar `billingHelpers.test.tsx` si rompe.
- **Verificación**:
  ```bash
  cd /home/pablocristo/Proyectos/pablo/pymes
  go test ./pymes-core/backend/internal/invoices/... -race -count=1
  cd frontend && npx tsc --noEmit && npx vitest run
  ```
- **Riesgo**: medio. **Breaking**: `PATCH /invoices/:id` con `status` ahora retorna 400 explícito (antes lo aceptaba). Solo el frontend lo usaba (verificado), migrado en mismo PR.
- **Aceptación**: backend tests verdes, frontend tests verdes, curl manual:
  ```bash
  # debe retornar 400:
  curl -X PATCH localhost:8100/v1/invoices/<id> -H "Content-Type: application/json" -d '{"status":"paid"}' -H "Authorization: ..."
  # debe retornar 200:
  curl -X PATCH localhost:8100/v1/invoices/<id>/status -H "Content-Type: application/json" -d '{"status":"paid"}' -H "Authorization: ..."
  ```

### PR 7 — Frontend: reemplazar `buildFullyConnectedStatusStateMachine` por `core-fsm`

**Objetivo**: alinear FE con BE para que el drag bloquee pre-server.

- **Archivos a crear**:
  - `frontend/src/modules/billing/salesStateMachine.ts`
  - `frontend/src/modules/billing/quotesStateMachine.ts`
  - `frontend/src/modules/billing/purchasesStateMachine.ts`
  - `frontend/src/modules/billing/invoicesStateMachine.ts`
- **Archivos a modificar**:
  - `frontend/src/modules/billing/billingHelpers.ts`: deprecar `buildFullyConnectedStatusStateMachine` (mantener exportada si tests fallan; eliminar en cleanup).
  - `frontend/src/modules/billing/billing<Sales|Quotes|Purchases|Invoices>Config.ts`: reemplazar el llamado a `buildFullyConnectedStatusStateMachine` con import del Builder espejo.
- **APIs nuevas**: `salesStateMachine`, `quotesStateMachine`, etc. (TS, type `StringMachine`).
- **Tests**: agregar `<dom>StateMachine.test.ts` table-driven con los pares válidos/inválidos. Estos tests son **espejo** de los Go (PR 3-6).
- **Verificación**:
  ```bash
  cd frontend
  npm run typecheck
  npx vitest run
  ```
- **Riesgo**: medio. Si el FE declara grafo distinto al BE, hay regresión silenciosa. Mitigación: tabla constante en cada test, code review cruzado FE↔BE.
- **Aceptación**: tests verdes, smoke manual: drag a columna inválida ahora se BLOQUEA visualmente (no dispara request → no espera 409).

### PR 8 — Frontend: centralizar endpoints + status labels

**Objetivo**: cleanup, baja prioridad.

- **Archivos a crear**: `frontend/src/lib/endpoints.ts`.
- **Archivos a modificar**: `*Config.ts` y `*Api.ts` para usar las constantes.
- **Tests**: actualizaciones menores.
- **Verificación**: `npm run typecheck && npx vitest run`.
- **Riesgo**: bajo.
- **Aceptación**: refactor compila + tests verdes.

### PR 9 — E2E API (Go) + Playwright (TS) — OBLIGATORIO

**Objetivo**: el smoke manual NO reemplaza E2E. Esta fase verifica el flow real FE↔BE con DB de prueba.

#### 9.1 E2E API backend (Go contra cp-backend real con DB efímera)

- **Archivos a crear**:
  - `pymes-core/backend/internal/e2e/status_workflows_test.go` — usa `httptest` o cliente HTTP real contra cp-backend con build tag `e2e`.
  - `pymes-core/backend/internal/e2e/helpers/db.go` — seed mínimo (org, user, party, sale/quote/purchase/invoice).
  - `pymes-core/backend/internal/e2e/helpers/auth.go` — token de prueba.
- **Casos cubiertos** (table-driven):
  ```go
  // Sales
  {endpoint: "PATCH /v1/sales/:id/status", body: `{"status":"completed"}`, from: "draft", want: 200},
  {endpoint: "PATCH /v1/sales/:id/status", body: `{"status":"paid"}`, from: "draft", want: 409},  // inválido
  {endpoint: "PATCH /v1/sales/:id/status", body: `{"status":"draft"}`, from: "voided", want: 409}, // terminal
  // Invoices: PATCH genérico ya no acepta status
  {endpoint: "PATCH /v1/invoices/:id", body: `{"status":"paid","notes":"x"}`, want: 400},
  {endpoint: "PATCH /v1/invoices/:id", body: `{"notes":"x"}`, want: 200}, // sin status OK
  {endpoint: "PATCH /v1/invoices/:id/status", body: `{"status":"paid"}`, from: "pending", want: 200},
  // Audit no se escribe en transición inválida
  {action: "drag a invalid", verify: "audit_log row count NOT incremented"},
  // Audit sí se escribe en transición válida
  {action: "drag a valid", verify: "audit_log row exists with action=sale.status_updated"},
  ```
- **Verificación**:
  ```bash
  cd /home/pablocristo/Proyectos/pablo/pymes
  docker compose up -d postgres cp-backend
  go test -tags e2e ./pymes-core/backend/internal/e2e/... -count=1 -v
  ```

#### 9.2 Playwright E2E (frontend)

- **Archivos a crear**:
  - `frontend/e2e/billing-status-workflows.spec.ts`
  - `frontend/e2e/helpers/login.ts` (reusa el login existente para `medlablocal`)
  - `frontend/e2e/helpers/seedBillingData.ts` (vía API, crea sale/quote/purchase/invoice en estados conocidos)
- **Specs cubiertos**:
  ```ts
  test.describe('sales kanban', () => {
    test('drag valid: draft → pending persiste tras refresh', async ({page}) => {...});
    test('drag invalid: drop bloqueado, NO HTTP request al backend', async ({page}) => {
      const requests: string[] = [];
      page.on('request', r => requests.push(r.url()));
      // intentar drag draft → paid
      // expect(requests).not.toContain('/v1/sales/.../status');
    });
    test('drag desde voided: bloqueado', async ({page}) => {...});
  });

  test.describe('quotes kanban', () => {
    test('accepted/rejected/expired no se pueden mover', async ({page}) => {...});
  });

  test.describe('purchases kanban', () => {
    test('transiciones libres entre draft/partial/received/voided permitidas', async ({page}) => {...});
  });

  test.describe('invoices', () => {
    test('drag pending → paid persiste, audit visible', async ({page}) => {...});
    test('edit form genérico no manda status en body PATCH', async ({page}) => {
      // interceptar PATCH /v1/invoices/<id> y verificar body sin "status"
    });
    test('intentar PATCH /v1/invoices/<id> con status devuelve 400', async ({page}) => {
      // request directo desde page.request
    });
  });
  ```
- **Verificación**:
  ```bash
  cd /home/pablocristo/Proyectos/pablo/pymes
  make up
  cd frontend && npx playwright test e2e/billing-status-workflows.spec.ts
  ```
- **Aceptación**: ambos suites verdes en CI.

#### 9.3 Test de "shape match" FE↔BE

- **Archivos a crear**:
  - `pymes-core/backend/internal/sales/fsm_match_test.go` (tabla canónica de transiciones permitidas)
  - `pymes-core/backend/internal/quotes/fsm_match_test.go`
  - `pymes-core/backend/internal/purchases/fsm_match_test.go`
  - `pymes-core/backend/internal/invoices/fsm_match_test.go`
  - `frontend/src/modules/billing/__tests__/<dom>StateMachine.shape.test.ts` para los 4
- Cada par (Go/TS) declara la MISMA tabla constante. Si el grafo cambia, ambos lados deben actualizar; el comentario `// MUST MATCH frontend/src/modules/billing/<dom>StateMachine.ts` (y viceversa) facilita el code review.
- **Limitación**: esto NO previene drift automáticamente; depende del code review. Ver Sección 6.4 para fase futura con spec compartida.

### PR 10 — Cleanup final

- **Archivos a modificar/eliminar**:
  - Eliminar `buildFullyConnectedStatusStateMachine` si tras PR 7 ya no tiene consumidores.
  - Eliminar TODOs sobre `IsTerminal` si PR 1 mergeó upstream.
- **Verificación**:
  ```bash
  cd /home/pablocristo/Proyectos/pablo/pymes
  rg "buildFullyConnectedStatusStateMachine" frontend/src/  # debe ser 0
  rg "isValid.*Status|valid.*Statuses" pymes-core/backend/internal/{sales,quotes,purchases,invoices}/  # debe ser 0
  go test ./... -race -count=1
  cd frontend && npx tsc --noEmit && npx vitest run
  ```
- **Riesgo**: bajo.
- **Aceptación**: 0 referencias a funciones deprecadas; CI completo verde.

### Dependencias de PRs

```
PR 0 (docs) ─────────────────────────┐
                                     │
PR 1 (IsTerminal core, OPCIONAL) ────┤  paralelo, no bloqueante
                                     │
PR 2 (helpers MapFSMError + RegisterStatusEndpoint) ──> PR 3 ─┐
                                                        PR 4 ─┤
                                                        PR 5 ─┤── independientes entre sí
                                                        PR 6 ─┘
                                                              │
PR 7 (frontend FSM, paralelo a PR 3-6) ──> PR 8 ──────────────┤
                                                              │
                                                              ↓
                                              PR 9 (E2E + Playwright OBLIGATORIO)
                                                              │
                                                              ↓
                                              PR 10 (cleanup final)
```

- PR 3-6 pueden mergearse en cualquier orden tras PR 2.
- PR 7 (frontend FSM) puede arrancar en paralelo al backend; no depende.
- **PR 9 es bloqueante para PR 10**: el cleanup solo se hace tras verificar que E2E pasan.
- PR 1 corre en paralelo todo el tiempo; si demora, PR 10 lo limpia.

---

## 9. Tests obligatorios

### 9.1 Tests `core` (PR 1)

- `core/concurrency/go/fsm/builder_test.go`:
  - `TestStringMachine_IsTerminal_True` (estado terminal)
  - `TestStringMachine_IsTerminal_False` (no terminal)
  - `TestStringMachine_IsTerminal_Unknown` (estado no declarado → false)

### 9.2 Tests `pymes-core` por dominio (PR 3-6)

Por cada dominio (sales, quotes, purchases, invoices), `usecases_test.go`:

| Caso | Esperado | Política |
|---|---|---|
| transición válida (FSM permite) | 200, status persistido, audit emitido, repo.UpdateStatus llamado | normal |
| transición inválida (`ErrInvalidTransition`) | 409 `domainerr.IsConflict`, repo NO llamado, audit NO emitido | rechazo |
| origen terminal a destino distinto (`ErrTerminal`) | 409 `domainerr.IsConflict`, repo NO llamado | rechazo |
| status vacío | 400 `domainerr.IsValidation` | rechazo |
| status desconocido (no en grafo) | 409 `domainerr.IsConflict` (FSM lo trata como `ErrInvalidTransition`) | rechazo |
| **same-status no terminal** (ej. `draft → draft`) | **200, idempotente** (FSM `CanTransition` retorna true para `from == to`) | idempotente |
| **same-status terminal** (ej. `paid → paid`, `voided → voided`) | **200, idempotente** (misma regla) | idempotente |
| entidad archivada | 409 `domainerr.IsConflict` (vía `archive.IfArchived`, antes de `Validate`) | rechazo |
| sin permiso RBAC | 403 (a nivel handler — test de integración) | rechazo |
| audit/timeline/webhook NO se llaman si el `Validate` rechaza | invariante verificable con mocks | invariante |
| audit/timeline/webhook NO se llaman si el `archive.IfArchived` rechaza | invariante | invariante |

**Política `same-status` definida**: idempotente siempre (incluso desde terminal). Esto refleja el comportamiento de `core/concurrency/go/fsm.CanTransition`. Razón: el cliente puede hacer un retry y no debe recibir 409 espurio. Si en algún dominio se quiere comportamiento distinto (ej. `paid → paid` rechazado), el usecase debe agregar el check ANTES de llamar `Validate` (no se hereda del FSM).

### 9.3 Tests handler (PR 2)

`pymes-core/backend/internal/shared/handlers/status_endpoint_test.go` con `httptest`:

| Caso | Esperado |
|---|---|
| body válido + updater retorna OK | 200 + JSON mapper |
| body sin status | 400 `validation` |
| body con status vacío | 400 `validation` |
| body inválido (no JSON) | 400 |
| `:id` no UUID | 400 |
| updater retorna `domainerr.Conflict(...)` | 409 |
| updater retorna `domainerr.NotFoundf(...)` | 404 |
| updater retorna error genérico | 500 |
| `MapFSMError(_, _, ErrInvalidTransition)` | `domainerr.IsConflict` true |
| `MapFSMError(_, _, ErrTerminal)` | `domainerr.IsConflict` true |
| `MapFSMError(_, _, nil)` | `nil` |

### 9.4 Tests frontend (PR 7)

Por dominio, `<dom>StateMachine.test.ts`:

```ts
import { salesStateMachine } from './salesStateMachine';

describe('salesStateMachine', () => {
    it.each([
        ['draft', 'pending', true],
        ['draft', 'completed', true],
        ['draft', 'paid', false],          // inválido
        ['completed', 'paid', true],
        ['paid', 'voided', false],         // terminal según diseño
        ['voided', 'draft', false],        // terminal
        ['draft', 'unknown', false],       // estado fuera de grafo
    ])('canTransition %s → %s = %s', (from, to, expected) => {
        expect(salesStateMachine.canTransition(from, to)).toBe(expected);
    });
});
```

### 9.5 Test de regresión (PR 9)

`pymes-core/backend/internal/sales/fsm_match_test.go`:

```go
// Tabla canónica de pares válidos. Sincronizar con frontend/src/modules/billing/salesStateMachine.test.ts
var saleAllowedTransitions = [][2]string{
    {"draft", "pending"},
    {"pending", "draft"},
    {"draft", "completed"},
    // ... lista completa
}

func TestSalesFSM_MatchesFrontend(t *testing.T) {
    for _, pair := range saleAllowedTransitions {
        if !saleStateMachine.CanTransition(pair[0], pair[1]) {
            t.Errorf("FE espera %s→%s permitido, BE lo rechaza", pair[0], pair[1])
        }
    }
}
```

Misma tabla del lado FE. Code review cruzado obligatorio cuando se modifique uno.

### 9.6 Tests CRUD (no nuevos, sino documentación de cobertura)

- Pagination: ya cubierto por `internal/shared/handlers/pagination_test.go`.
- Archive: cubrir en `usecases_test.go` por dominio. Solo `purchases` lo tiene hoy.
- Audit/Timeline/Webhook ports: tests por dominio que verifican que se invocan tras mutación exitosa.

---

## 10. Comandos de verificación

### 10.1 Build & test

```bash
# Backend Go (todo)
cd /home/pablocristo/Proyectos/pablo/pymes
go build ./...
go vet ./...
go test ./... -race -count=1

# Tests específicos
go test ./pymes-core/backend/internal/sales/...     -race -count=1
go test ./pymes-core/backend/internal/quotes/...    -race -count=1
go test ./pymes-core/backend/internal/purchases/... -race -count=1
go test ./pymes-core/backend/internal/invoices/...  -race -count=1
go test ./pymes-core/backend/internal/shared/handlers/... -race -count=1

# core (cuando se haga PR 1)
cd /home/pablocristo/Proyectos/pablo/core/concurrency/go
go test ./fsm/...

# Frontend
cd /home/pablocristo/Proyectos/pablo/pymes/frontend
npx tsc --noEmit
npx vitest run
npx playwright test e2e-real    # smoke E2E
```

### 10.2 Anti-duplicación / regresión

```bash
# Ningún validador hardcoded de status fuera del FSM
rg "isValid.*Status|valid.*Statuses" /home/pablocristo/Proyectos/pablo/pymes/pymes-core/backend/internal/{sales,quotes,purchases,invoices}/
# Después del refactor: 0 matches (excepto el helper de purchases para el create default)

# Ningún canTransition hardcoded en switch
rg "canTransition.*Status" /home/pablocristo/Proyectos/pablo/pymes/pymes-core/backend/internal/
# Después del refactor: 0 matches (purchases lo elimina)

# Endpoints /status que no usen el helper
rg "PATCH.*\".*status\"|/status\"" /home/pablocristo/Proyectos/pablo/pymes/pymes-core/backend/internal/{sales,quotes,purchases,invoices}/handler.go
# Después del refactor: 0 ocurrencias raw — todas vía RegisterStatusEndpoint

# Frontend: ningún uso de buildFullyConnectedStatusStateMachine
rg "buildFullyConnectedStatusStateMachine" /home/pablocristo/Proyectos/pablo/pymes/frontend/src/
# Después del cleanup PR 9: 0 matches
```

### 10.3 Verificación de imports prohibidos

```bash
# core no debe importar pymes
rg "github.com/devpablocristo/pymes" /home/pablocristo/Proyectos/pablo/core/
# debe estar vacío

# modules no debe importar pymes
rg "github.com/devpablocristo/pymes" /home/pablocristo/Proyectos/pablo/modules/
# debe estar vacío

# pymes-core/shared no debe importar pymes-core/internal (back-dep)
rg "pymes-core/backend/internal" /home/pablocristo/Proyectos/pablo/pymes/pymes-core/shared/
# debe estar vacío

# Usecases NO importan internal/shared/handlers (Sección 5.3.1 — separación capas)
rg "internal/shared/handlers" /home/pablocristo/Proyectos/pablo/pymes/pymes-core/backend/internal/{sales,quotes,purchases,invoices}/usecases.go
# debe estar vacío

# MapFSMError vive en status, NO en handlers
rg "func MapFSMError" /home/pablocristo/Proyectos/pablo/pymes/pymes-core/backend/internal/shared/handlers/
# debe estar vacío
rg "func MapFSMError" /home/pablocristo/Proyectos/pablo/pymes/pymes-core/backend/internal/shared/status/
# debe estar (1 match)
```

### 10.4 Verificación de versiones publicadas (Regla 3.3)

```bash
# Go: ningún replace local de core/modules
grep -E "^replace.*devpablocristo/(core|modules)" /home/pablocristo/Proyectos/pablo/pymes/go.mod
# debe estar VACÍO

# Verticales también
for d in workshops professionals beauty restaurants; do
  if [ -f "/home/pablocristo/Proyectos/pablo/pymes/$d/backend/go.mod" ]; then
    grep -E "^replace.*devpablocristo/(core|modules)" "/home/pablocristo/Proyectos/pablo/pymes/$d/backend/go.mod"
  fi
done
# debe estar VACÍO

# Frontend: ningún file:/link: a devpablocristo
grep -E "\"@devpablocristo/[^\"]+\":\s*\"(file:|link:|\\.\\./)" /home/pablocristo/Proyectos/pablo/pymes/frontend/package.json
# debe estar VACÍO

# Verificación positiva: cada @devpablocristo/* en package.json apunta a versión semver
grep -E "\"@devpablocristo/" /home/pablocristo/Proyectos/pablo/pymes/frontend/package.json
# debe mostrar solo lineas con "^X.Y.Z" o "X.Y.Z" o "~X.Y.Z"
```

### 10.4 Smoke manual (UI)

```
1. make up                              # rebuild cp-backend
2. http://localhost:5180/medlablocal/sales/board      # drag entre columnas
3. http://localhost:5180/medlablocal/quotes/board
4. http://localhost:5180/medlablocal/purchases/board
5. http://localhost:5180/medlablocal/invoices/board

Por cada uno:
- Drag a columna válida → status persiste tras refresh, sin error en consola.
- Drag a columna inválida → bloqueado visualmente por kanbanTransitionModel (no hay request HTTP en network tab).
- Si forzo un PATCH con curl a transición inválida → 409 + mensaje legible.

# Auditar evento en DB
PGPASSWORD=postgres psql -h localhost -p 5434 -U postgres -d pymes -c \
  "SELECT action, payload FROM audit_log WHERE action LIKE '%.status_updated' ORDER BY created_at DESC LIMIT 10;"
```

---

## 11. Riesgos y mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación | PR donde se controla |
|---|---|---|---|---|
| Datos en DB con status fuera del nuevo grafo | Baja | Alto (500 al validar) | Pre-merge: `SELECT DISTINCT status FROM <table>`. CHECK constraint ya restringe en sales. | PR 3-6 |
| Breaking change `paid → voided` en sales | Media | Medio | Documentar en CHANGELOG. Buscar usos en código (`grep UpdateStatus.*voided`). | PR 3 |
| Breaking change `voided → *` en purchases | Media | Medio | Verificar uso en producción con SQL. Si aparece, fallback a opción A (no terminal). | PR 5 |
| Breaking en `PATCH /invoices/:id` con `status` | Alta | Bajo | Frontend migrado mismo PR. Backend devuelve 400 claro. | PR 6 |
| Divergencia FE↔BE FSM | Media | Alto (UX silenciosa) | Tabla canónica en tests de ambos lados. Code review cruzado. PR 9 introduce test. | PR 7, PR 9 |
| Send/Accept/Reject en quotes rompen tests existentes | Media | Bajo | Tests adaptados en mismo PR. | PR 4 |
| Side effects (audit/timeline/webhook) corren aunque FSM rechace | Baja | Alto (datos corruptos) | Diseño: `Validate` antes de `repo.UpdateStatus` y antes de emit. Test cubre. | PR 3-6 |
| Helper `RegisterStatusEndpoint` introduce bug de wiring | Baja | Medio | Tests del helper en PR 2 antes de consumidores. | PR 2 |
| `IsTerminal` upstream bloqueado por review en core | Media | Bajo (workaround temporal) | Helper local `fsmext.IsTerminal` en pymes; eliminar tras merge upstream. | PR 1 / PR 9 |
| Sobre-abstracción accidental en CRUD | Baja | Alto (deuda técnica) | Plan explícitamente NO abstrae tags/notes/favorite/RegisterRoutes. Documentado. | docs |
| `archive.IfArchived` se usa con campo wrong (DeletedAt vs ArchivedAt) | Baja | Medio (false negatives) | Code review. Documentar la regla en CLAUDE.md sec. 5.5. | docs |
| Frontend test de "shape match" no se mantiene | Media | Alto (drift) | PR 9 incluye tabla canónica + comentario `// MUST MATCH backend/...` en ambos lados. Mitigación parcial; fase futura: codegen (Sección 6.4). | PR 9 |
| **PATCH `/invoices/:id` con `status` en body se ignora silenciosamente** | Alta sin fix | Alto (cliente cree que actualizó) | **Detección explícita** con `map[string]json.RawMessage` en el handler de Update (Sección 5.3.5). | PR 6 |
| **Política same-status no documentada en el código** | Media | Medio (drift accidental) | Comentario en `<dom>StateMachine` y test explícito. | PR 3-6 |
| **E2E faltantes dejan regresiones invisibles** | Alta sin PR 9 | Alto | PR 9 obligatorio: API E2E Go + Playwright. NO se hace cleanup hasta que pasen. | PR 9 |
| Decisión purchases A/B abierta durante implementación | (cerrada) | (cerrada) | **Decidida en plan: opción A (`voided` no terminal)**. Cualquier cambio a B es PR separado con producto. | Sección 6.1 |
| `IsTerminal` upstream bloquea pymes | Baja | Bajo | PR 1 paralelo, no bloqueante. PR 10 limpia si llega. | PR 1 |
| Migración de PRs grandes deja el sistema inconsistente | Baja | Alto | Cada PR es independiente, mergeable, revertible. PR 2 sin consumidores; PR 3-6 cada uno completo. | todos |

---

## 12. Criterios de aceptación final

Checklist objetiva al final del refactor:

### Backend
- [ ] `rg "isValid.*Status|valid.*Statuses" pymes-core/backend/internal/{sales,quotes,purchases,invoices}/` devuelve 0 (excepto el normalizador de defaults en purchases.prepareCreate).
- [ ] `rg "canTransitionPurchaseStatus|canTransition.*Status" pymes-core/backend/internal/` devuelve 0.
- [ ] Cada dominio tiene `<dom>/fsm.go` con `<dom>StateMachine *fsm.StringMachine`.
- [ ] Cada dominio registra su endpoint vía `handlers.RegisterStatusEndpoint`.
- [ ] `MapFSMError` vive en `internal/shared/status/`, NO en `handlers/`. Verificar: `rg "MapFSMError" pymes-core/backend/internal/shared/handlers/` debe devolver 0.
- [ ] Usecases NO importan `internal/shared/handlers`: `rg "shared/handlers" pymes-core/backend/internal/{sales,quotes,purchases,invoices}/usecases.go` debe devolver 0.
- [ ] `invoices` tiene endpoint `PATCH /v1/invoices/:id/status` y emite `invoice.status_updated` en audit.
- [ ] `PATCH /v1/invoices/:id` con `status` en body → 400 con mensaje exacto "use PATCH /invoices/:id/status to change status" (verificado con curl + handler test).
- [ ] `go test ./pymes-core/backend/internal/{sales,quotes,purchases,invoices}/... -race -count=1` pasa.
- [ ] `go test ./pymes-core/backend/internal/shared/{status,handlers}/... -race -count=1` pasa.
- [ ] `core` y `modules` no importan `pymes` (verificado con grep).
- [ ] **Política same-status idempotente** verificada con tests explícitos en cada dominio (`draft → draft → 200`, `paid → paid → 200`).

### Frontend
- [ ] `rg "buildFullyConnectedStatusStateMachine" frontend/src/` devuelve 0.
- [ ] Cada `*Config.ts` (sales/quotes/purchases/invoices) importa su FSM de un archivo `<dom>StateMachine.ts`.
- [ ] Cada FSM frontend usa `@devpablocristo/core-fsm` Builder.
- [ ] `frontend/src/lib/invoicesApi.ts:updateInvoiceStatus` apunta a `/v1/invoices/${id}/status`.
- [ ] `frontend/src/modules/billing/billingInvoicesConfig.ts` ya no mete `status` en body de Update genérico.
- [ ] Drag en kanban a columna inválida queda BLOQUEADO visualmente (no dispara HTTP request).
- [ ] `npm run typecheck` y `npx vitest run` pasan.

### FE↔BE consistencia
- [ ] Test `<dom>FSM_MatchesFrontend` (Go) y `<dom>StateMachine.shape.test.ts` (TS) declaran la misma tabla de transiciones.
- [ ] Cada FSM tiene un comentario `// MUST MATCH frontend/src/modules/billing/<dom>StateMachine.ts` (y viceversa).
- [ ] Issue creado para fase futura de codegen / spec compartida (ver Sección 6.4).

### E2E (PR 9, OBLIGATORIO antes de cleanup)
- [ ] `go test -tags e2e ./pymes-core/backend/internal/e2e/... -count=1` pasa con DB efímera.
- [ ] `npx playwright test e2e/billing-status-workflows.spec.ts` pasa (4 dominios cubiertos: drag válido, drag inválido bloqueado pre-server, edit form de invoices no manda status).
- [ ] Audit log verificado: aparece `<dom>.status_updated` solo en transiciones válidas.

### Versiones publicadas (Regla 3.3)
- [ ] `grep -E "^replace.*devpablocristo/(core|modules)" pymes/go.mod` devuelve 0.
- [ ] `grep -E "\"@devpablocristo/[^\"]+\":\s*\"(file:|link:|\\.\\./)" frontend/package.json` devuelve 0.
- [ ] Si PR 1 mergeó: `pymes/go.mod` apunta a `core/concurrency/go v0.1.2` (o tag bumped) — versión semver, no commit hash.
- [ ] Si PR 1 NO mergeó: `pymes/go.mod` sigue en `core/concurrency/go v0.1.1` y el código de pymes NO usa `IsTerminal`.

### Endpoints status
- [ ] Todos los `/status` devuelven 200 en transiciones válidas.
- [ ] Devuelven 409 con mensaje `"status transition not allowed: <from> -> <to>"` o `"status %q is terminal"` en inválidas.
- [ ] Devuelven 400 con mensaje "status is required" si el body viene vacío.

### Side effects
- [ ] Si `Validate` rechaza, `repo.UpdateStatus` NO se llama.
- [ ] Si `Validate` rechaza, `audit.Log` NO se llama.
- [ ] Tests cubren ambos.

### Documentación
- [ ] `pymes/ARCHITECTURE.md` existe y describe core/modules/pymes-core/frontend.
- [ ] `pymes/AI_GUIDELINES.md` existe con regla de decisión y ejemplos.
- [ ] `pymes/CLAUDE.md` referencia ambos.
- [ ] README de cada paquete nuevo (FSM por dominio) explica el grafo y el comentario MUST MATCH.

### Limpieza
- [ ] No quedan TODOs sobre `IsTerminal` ni helpers temporales tras PR 1 + PR 9.
- [ ] CHANGELOG documenta los breaking changes (`paid → voided` en sales, `voided → *` en purchases, `PATCH /invoices/:id` con status).

---

## 13. Documentación para evitar recaídas

### 13.1 `pymes/ARCHITECTURE.md` (nuevo)

Contenido propuesto (esqueleto):

```markdown
# Arquitectura del producto Pymes

## 1. Capas y origen del código

| Origen | Qué contiene | Reglas |
|---|---|---|
| `core/*` | Primitivas puras agnósticas (FSM, errors, http, pagination, fsm, ...) | NO depende de pymes ni de modules |
| `modules/*` | Capacidades reusables (CRUD paths, archive, kanban-board UI, work-orders FSM, scheduling) | Puede depender de core; NO de pymes |
| `pymes-core/backend/internal/*` | Dominio del producto. Hexagonal: usecases + handler + repository | Importa core, modules; NO importa internal de otro vertical |
| `pymes-core/shared/backend/*` | Helpers transversales del producto (httperrors, ports, etc.) | NO depende de internal/* (back-dep prohibido) |
| `verticales/*` | Workshops, professionals, beauty, restaurants | Importa core, modules, HTTP API de pymes-core; NO domain interno de otro vertical |
| `frontend/*` | Consola React | Importa @devpablocristo/{core,modules}-* desde npm |

## 2. Regla de decisión para nuevos helpers

[copiar Sección 3.1 de este plan]

## 3. Patrón canónico Status/FSM

[copiar Sección 6 de este plan, sin grafos concretos pero sí la arquitectura]

## 4. Patrón canónico CRUD

- Soft delete: usar `archive.IfArchived` de `modules/crud/archive/go`. Aceptamos `archived_at` y `deleted_at` como nombres de columna.
- Pagination: usar `core/http/go/pagination` vía wrappers de `internal/shared/handlers/pagination.go`.
- Path segments: usar `modules/crud/paths/go` (`crudpaths.SegmentArchived`, etc.).
- Audit/Timeline/Webhook: ports unificados (interfaces idénticas en todos los dominios).
- Tags/Notes/Favorite: campos declarativos en DTO. NO abstraer.

## 5. Cómo agregar un nuevo dominio CRUD

1. Crear `pymes-core/backend/internal/<dom>/{usecases,handler,repository}.go`.
2. Si tiene status: crear `<dom>/fsm.go` con `var <dom>StateMachine = fsm.NewBuilder()...Build()`.
3. En `RegisterRoutes`: usar `crudpaths` para segmentos canónicos. Para `/status`, usar `handlers.RegisterStatusEndpoint`.
4. Agregar `archive.IfArchived` en mutaciones que dependen de no estar archivado.
5. Frontend: declarar `<dom>StateMachine.ts` con `@devpablocristo/core-fsm` Builder. Comentario `// MUST MATCH backend/.../<dom>/fsm.go`.
6. Tests: por dominio, table-driven (válido / inválido / terminal / empty / unknown / archived).

## 6. Anti-patrones (con ejemplos reales)

❌ **No hagas**: Validar status con `switch` o slice hardcoded.
✅ **Hacé**: Declarar FSM en `<dom>/fsm.go` con Builder de `core/concurrency/go/fsm`.

❌ **No hagas**: Reinventar `buildFullyConnectedStatusStateMachine` en frontend.
✅ **Hacé**: Usar `Builder` de `@devpablocristo/core-fsm`, espejar el grafo del backend.

❌ **No hagas**: Llamar `audit.Log` antes de validar la transición.
✅ **Hacé**: `Validate → repo.UpdateStatus → audit.Log → timeline → webhook`. Si `Validate` falla, return temprano.

❌ **No hagas**: Importar `pymes` desde `core` o `modules`.
✅ **Hacé**: Si necesitás algo agnóstico de pymes en core/modules, identificá la primitiva y proponé PR upstream.

❌ **No hagas**: Crear helper en `internal/shared/handlers/` cuando es agnóstico.
✅ **Hacé**: Identificá la dependencia. Si solo depende de http+errors → core. Si depende de RBAC pymes → local.
```

### 13.2 `pymes/AI_GUIDELINES.md` (nuevo)

```markdown
# Guías para asistentes de IA / nuevos contribuyentes

## Antes de escribir código nuevo

Respondé en orden:

1. ¿Existe ya en core o modules? `rg "<concepto>" /home/pablocristo/Proyectos/pablo/{core,modules}/`
2. ¿Es primitiva pura? → core
3. ¿Es capacidad reusable? → modules
4. ¿Depende de pymes? → pymes-core
5. ¿Se usa una vez? → no abstraigas

## Antes de copiar lógica de otro dominio

Si vas a copiar algo de otro dominio:
- ¿La parte que copiás es primitiva? → quizás debería estar en core/modules.
- ¿Es lógica de negocio? → revisar si la diferencia es real o accidental.

## Patrones canónicos

- **Status update**: `<dom>StateMachine` + `RegisterStatusEndpoint` + `MapFSMError`. Ver `pymes-core/backend/internal/sales/fsm.go` como referencia.
- **Soft delete**: `archive.IfArchived(record.ArchivedAt | DeletedAt, "<resource>")`.
- **Pagination**: `handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{...})`.
- **Audit**: `u.audit.Log(ctx, orgID, actor, "<resource>.<verb>", "<resource>", id, payload)`.
- **HTTP error**: `httperrors.Respond(c, err)` mapea `domainerr` a HTTP. NUNCA `c.JSON(500, gin.H{"error": err.Error()})`.

## Regla DURA: solo versiones publicadas de core/modules

**Prohibido absolutamente** consumir `core` y `modules` desde checkouts locales:

❌ NUNCA:
```
// pymes/go.mod
replace github.com/devpablocristo/core/concurrency/go => ../../core/concurrency/go
```
```json
// frontend/package.json
"@devpablocristo/core-fsm": "file:../../core/concurrency/fsm/ts"
"@devpablocristo/core-fsm": "link:../../../core/..."
```

✅ Siempre:
```
// pymes/go.mod
require github.com/devpablocristo/core/concurrency/go v0.1.2
```
```json
"@devpablocristo/core-fsm": "^0.2.0"
```

**Por qué**: los `replace` locales rompen builds para cualquiera que clona un solo repo, hacen los tags de versión irrelevantes, e impiden rollbacks limpios.

**Si necesitás algo nuevo en core/modules**:
1. Hacé el cambio en core/modules.
2. Bump VERSION + tag git (`<capacidad>/<runtime>/v<X.Y.Z>`).
3. Publicá (npm para TS, push tag para Go).
4. Recién entonces actualizá `pymes` para usar el nuevo tag.

NO hagas `replace` "temporal" mientras esperás que mergeen el upstream — si bloquea, **mantenete con la versión vieja en pymes** o evitá el feature hasta que esté publicado.

## Comandos útiles

[lista de comandos de Sección 10]
```

### 13.3 README en cada FSM nuevo

`pymes-core/backend/internal/sales/fsm.go`:

```go
// Package sales — fsm.go
//
// FSM canónico de transiciones de status para Sale.
// Grafo:
//
//   draft ↔ pending → completed → paid (terminal)
//                \         \
//                 \---------\---→ voided (terminal, global desde no-terminales)
//
// Reglas:
// - draft↔pending son free (puede ir y volver).
// - completed es one-way de draft/pending.
// - paid es terminal en UpdateStatus. Para anular un paid → usar Void().
// - voided es alcanzable desde cualquier no-terminal vía AllowAnyTo.
//
// MUST MATCH frontend/src/modules/billing/salesStateMachine.ts.
// Cualquier cambio acá requiere actualizar el frontend en el mismo PR
// y la tabla del test fsm_match_test.go.
package sales
```

Mismo patrón para los otros 3 dominios.

---

## Anexo — Verificaciones realizadas durante la auditoría

| Verificación | Resultado | Comando |
|---|---|---|
| Convivencia `archived_at` y `deleted_at` | Confirmada | `rg "archived_at\|deleted_at" pymes-core/backend/migrations/*.sql` |
| `IsTerminal` ausente en Go FSM | Confirmado | `grep "IsTerminal" core/concurrency/go/fsm/*.go` (sin matches) |
| `IsTerminal` presente en TS FSM | Confirmado por uso | `modules/work-orders/ts/src/kanbanConfig.ts` línea ~91 |
| `domainerr.Newf(kind, fmt, args)` existe | Confirmado | `core/errors/go/domainerr/domainerr.go:39` |
| `core/http/go/pagination` ya extraído | Confirmado | `pymes-core/backend/internal/shared/handlers/pagination.go:7` |
| Pyme no importa pymes desde core ni modules | Confirmado | `rg "github.com/devpablocristo/pymes" core/ modules/` → 0 |
| 36 dominios CRUD identificados | Confirmado | `ls pymes-core/backend/internal` |
| `buildFullyConnectedStatusStateMachine` usado en 4 configs | Confirmado | `rg "buildFullyConnectedStatusStateMachine" frontend/src/` |
| `core-fsm` solo usado en 1 archivo en frontend | Confirmado | `rg "@devpablocristo/core-fsm" frontend/src/` |
| 24 dominios usan AuditPort identical | Confirmado | grep en `pymes-core/backend/internal/*/usecases.go` |

**Pendiente de verificación pre-implementation**:
- `SELECT COUNT(*) FROM purchases WHERE status = 'voided' AND ...`: cuenta de purchases anuladas con re-vivenciamientos. Decide opción A vs B en PR 5.
- `SELECT DISTINCT status FROM sales/quotes/purchases/invoices`: garantiza que no hay status fuera del FSM en datos existentes.
