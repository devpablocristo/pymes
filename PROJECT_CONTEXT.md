# Project Context

Guia operativa para agentes y colaboradores en el monorepo Pymes. Resume el estado verificable del repo y apunta a las fuentes canonicas antes de tocar codigo o documentacion.

## Fuentes primarias

- `CLAUDE.md`: reglas generales del proyecto, idioma, arquitectura, verificacion y prohibiciones.
- `docs/README.md`: indice de documentacion del monorepo.
- `docs/AGENTS.md`: contrato de capabilities y operacion de agentes.
- `docs/API_CONTRACTS.md`, `docs/GOVERNANCE.md`, `docs/HUMAN_AGENT_UX.md`: agentic APIs, Nexus Governance y UX humano-agente.
- `docs/DATABASE_INIT.md`: bootstrap de base, identidad `org_id`, migraciones post-squash y debug.
- `docs/CRUD_STANDARD.md`, `docs/TECH_DEBT_LEDGER.md`, `docs/UI_SYSTEM.md`: CRUD canonico, deuda tecnica y sistema UI.
- `Makefile`, `docker-compose.yml`, `.env.example`: comandos, servicios locales y configuracion esperada.

Antes de documentar o cambiar comportamiento, verificar contra codigo real con `rg`, manifests, handlers, migraciones y compose. No inventar endpoints, modulos, variables ni reglas.

## Mapa del repo

- `core/`: backend transversal Go y `core/shared/` para utilidades especificas de Pymes compartidas entre verticales.
- `professionals/`, `workshops/`, `beauty/`, `restaurants/`, `medical/`: backends Go verticales.
- `ui/`: consola React/Vite unificada.
- `mobile/`: app Expo / React Native con Expo Router, Clerk y Zustand.
- `ai/`: servicio FastAPI historico/de soporte; el chat local actual apunta a Companion como repo hermano via `VITE_COMPANION_BASE_URL`.
- `docs/`: documentacion de arquitectura, API, gobernanza, UI y migraciones.
- `scripts/`: auditorias, seeds, migraciones, infra y utilidades.
- `infra/`: definicion declarativa de verticales y recursos asociados.

Codigo reutilizable:

- Librerias `github.com/devpablocristo/platform/...` y otros modulos externos en `go.mod` para primitivas agnosticas.
- `core/shared/` para codigo transversal del producto.
- `internal/` del backend owner para logica acoplada a un dominio.
- No usar carpeta `pkgs/`; no importar dominio interno entre verticales.

## Stack y servicios locales

- Go `1.26.1` segun `go.mod`.
- UI: Node `>=20.9.0`, React 18, Vite, TypeScript, Vitest.
- Mobile: Expo SDK 54, React Native 0.81, React 19, Expo Router v6.
- DB local: `postgres:16-alpine`, base `pymes`, puerto host `5434`.
- Nexus Governance corre fuera de este compose, usualmente en `../nexus`, y Pymes lo consume por `GOVERNANCE_URL`.
- Companion corre como repo hermano para chat/capabilities, usualmente en `:18085`.

Puertos locales publicados por `docker-compose.yml`:

- `cp-backend`: `8100 -> 8080`
- `ui`: `5180 -> 5173`
- `prof-backend`: `8181 -> 8081`
- `work-backend`: `8282 -> 8082`
- `beauty-backend`: `8383 -> 8083`
- `restaurants-backend`: `8484 -> 8084`
- `medical-backend`: `8585 -> 8085`
- `mailhog`: `1025` SMTP y `8025` UI

## Flujo de trabajo y comandos

Flujo local habitual: Docker primero desde la raiz del monorepo.

- `make up`: build y levanta stack local.
- `make down`: baja el stack.
- `make ps`: estado de contenedores.
- `make logs`: logs con tail.
- `make audit`: auditorias arquitecturales.
- `make audit-baseline`: baseline antes de refactors estructurales.
- `make seed`, `make seed-clear`, `make seed-verify`, `make seed-reset`: datos demo.
- `make test-docker-core`, `make test-docker-workshops`, `make build-docker-ui`, `make test-docker-ui`, `make lint-docker-ui`: verificaciones dentro de contenedores.
- `make build` y `make test`: respaldo nativo cuando Docker no esta disponible o el alcance lo justifica.

Regla de cierre: no afirmar que un cambio de codigo, config, CI, Docker o infraestructura esta terminado sin haber ejecutado en el turno la verificacion relevante y haber visto salida OK. Si se toca Dockerfile, entrypoint, compose o wiring de runtime, tambien corresponde rebuild, `up -d`, readiness y smoke HTTP.

Para cambios solo documentales, no hace falta `make build` ni `make test`; si corresponde, validar con `git diff --check` y busquedas puntuales.

## Reglas criticas

- Codigo interno siempre en ingles: nombres Go/TS/Python, SQL, JSON, endpoints, permisos, migraciones, tests.
- UI visible, documentacion y respuestas al usuario en espanol.
- Cambios quirurgicos: tocar solo lo pedido.
- PostgreSQL en desarrollo, staging y produccion. No repositorios in-memory como artefacto productivo.
- Identidad multi-tenant canonica: tabla `orgs`, FK `org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE`.
- Excepciones de nombre historico: `tenant_settings` y `tenant_invitations` conservan nombre, pero usan FK `org_id`.
- Soft delete canonico: `archived_at timestamptz NULL`; excepciones documentadas: `users.deleted_at` y `sales.voided_at`.
- No modificar migraciones squashed existentes. Las nuevas van numeradas despues de las existentes y con `.down.sql` completo.
- Errores HTTP: `{code, message}`; no exponer `err.Error()` al cliente.
- Handlers HTTP usan DTOs, nunca `var body struct{...}` inline.
- Logging con `slog` o `zerolog`, no `fmt.Printf`.
- No duplicar capacidades de `core` en verticales; integraciones entre verticales por HTTP.

## Agentes y gobernanza

La superficie agentic canonica es `/v1/agent/*`, documentada en `docs/AGENTS.md`.

- Los agentes descubren capabilities, no llaman endpoints de negocio sueltos.
- Reads publican `risk_level=read` y no requieren confirmacion ni Review.
- Writes actuales requieren confirmacion, Nexus Review e idempotencia.
- Canales permitidos: `human_ui`, `internal_agent`, `external_agent`, `mcp`.
- API keys externas para acciones no-read requieren firma externa.
- Nexus es el motor de policies, risk y approvals; Pymes falla cerrado si Nexus no responde para acciones gobernadas.
- Executors de dominio agentic estan en `contract_only` hasta conectarlos uno por uno.

## UI, mobile y AI

UI:

- Usar tokens de `ui/src/styles/tokens.css` y reglas de `docs/UI_SYSTEM.md`.
- No introducir Tailwind ni CSS Modules.
- Iconos nuevos con `@tabler/icons-react`; sidebar conserva `ShellIcons.tsx` por decision previa.
- Scripts principales en `ui/package.json`: `build`, `typecheck`, `lint`, `test`, `test:e2e`.

Mobile:

- Convenciones detalladas en `mobile/CLAUDE.md`.
- User-facing strings desde `constants/translations.ts`.
- Estado global con Zustand, no React Context para estado global.
- Estilos con `StyleSheet.create`; no hardcodear colores, tamanos ni strings.

AI / Companion:

- `ai/` conserva runtime FastAPI y checks `ruff` / `pytest`.
- Chat en UI usa Companion por `VITE_COMPANION_BASE_URL`; no asumir que el compose de Pymes levanta un servicio AI local.

## Posiblemente obsoleto / pendiente de confirmar

- `CLAUDE.md` referencia `docs/CLERK_LOCAL.md`, `docs/PYMES_CORE.md`, `docs/CORE_INTEGRATION.md` y `core/docs/FRAUD_PREVENTION.md`, pero esos paths no existen hoy en el repo. No eliminar la referencia sin decidir si hay que restaurar esos docs o actualizar el indice.
- Algunas reglas `.cursor` usan nombres historicos como `pymes-core/` o `frontend/`. En este repo actual, los paths verificables son `core/` y `ui/`.
- Cualquier referencia antigua a `tenant_id` debe contrastarse con `docs/DATABASE_INIT.md`. El estado post-squash usa `org_id`, salvo excepciones documentadas.
- `ai/` existe, pero el chat/capabilities moderno se coordina con Companion como repo hermano. Revisar `docs/AI_COMPANION_MIGRATION.md` antes de asumir ownership.

## Cambios de esta edicion

- Se creo este archivo porque `PROJECT_CONTEXT.md` no existia en la raiz.
- Se resumio el contexto operativo a partir de `CLAUDE.md`, `docs/`, `Makefile`, `docker-compose.yml`, manifests y reglas existentes.
- Se conservaron reglas vigentes y se marcaron dudas en vez de borrar conocimiento historico.
- No se agregaron reglas nuevas sin respaldo en archivos del repo.
