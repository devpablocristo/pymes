# Pymes — flujo local habitual: todo en contenedores (`make up`).
# Verificación preferida: targets `*-docker-*`; `build` y `test` quedan como respaldo nativo.
.PHONY: \
	up down ps logs \
	go-compile staticcheck ruff lint companion-openapi-check \
	audit audit-baseline audit-crud audit-crud-json audit-crud-strict audit-debt audit-governance ui-typecheck ai-test \
	seed seed-clear seed-clear-verify seed-verify seed-reset modules-check cleanup-pablo e2e-governance-notifications \
	build-docker-ui test-docker-ui lint-docker-ui test-docker-core test-docker-workshops \
	build test test-ui-e2e

GO_PRIVATE = GOPRIVATE=github.com/devpablocristo/* GONOSUMDB=github.com/devpablocristo/* GONOPROXY=github.com/devpablocristo/* GOPROXY=https://proxy.golang.org,direct
GO_PACKAGES = ./core/backend/... ./core/shared/... ./workshops/backend/... ./professionals/backend/... ./restaurants/backend/... ./beauty/backend/... ./medical/backend/...
# Repo hermano `local-infra`. Override por CLI: `make up LOCAL_INFRA_DIR=/ruta/al/local-infra`.
#
# GNU Make importa variables del shell; `?=´ respeta el entorno y un export viejo (p. ej. ruta de otra
# máquina como /home/pablo/...) rompe `make up`. Si LOCAL_INFRA_DIR viene del entorno pero esa ruta
# no existe, lo ignoramos y usamos ../local-infra portable.
LOCAL_INFRA_DEFAULT := $(abspath $(CURDIR)/../local-infra)
ifeq ($(origin LOCAL_INFRA_DIR),environment)
  ifneq ($(strip $(LOCAL_INFRA_DIR)),)
    ifeq ($(wildcard $(LOCAL_INFRA_DIR)/.),)
      override LOCAL_INFRA_DIR := $(LOCAL_INFRA_DEFAULT)
    endif
  else
    override LOCAL_INFRA_DIR := $(LOCAL_INFRA_DEFAULT)
  endif
endif
ifndef LOCAL_INFRA_DIR
  LOCAL_INFRA_DIR := $(LOCAL_INFRA_DEFAULT)
endif

# Compose padre: `local-infra` del ecosistema si existe; si no, overlay mínimo del repo (paridad con CI / sin checkout extra).
LOCAL_INFRA_COMPOSE := $(LOCAL_INFRA_DIR)/docker-compose.yml
ifeq ($(wildcard $(LOCAL_INFRA_COMPOSE)),)
BASE_COMPOSE := $(abspath $(CURDIR)/.github/ci-infra/docker-compose.yml)
else
BASE_COMPOSE := $(abspath $(LOCAL_INFRA_COMPOSE))
endif
DC = docker compose --project-directory $(CURDIR) -f $(BASE_COMPOSE) -f $(CURDIR)/docker-compose.yml

# Calidad

# Compilacion rapida de todos los backends Go del monorepo, sin caer en ui/node_modules.
go-compile:
	$(GO_PRIVATE) go test $(GO_PACKAGES) -run '^$$'

# Análisis estático Go (código muerto U1000, imports duplicados, etc.); versión alineada con go.mod
staticcheck:
	$(GO_PRIVATE) go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 $(GO_PACKAGES)

# Alias historico: el servicio `pymes/ai` fue retirado; el runtime IA vive en Axis Companion.
ruff:
	@echo "pymes/ai retirado; ejecutar checks de IA en ../axis/companion"

# Go staticcheck del monorepo Pymes.
lint: staticcheck

# Auditorias de saneamiento arquitectural: no cambian comportamiento productivo.
audit: audit-crud audit-debt audit-governance

# Baseline reproducible antes de refactors estructurales.
audit-baseline: go-compile audit ui-typecheck companion-openapi-check

audit-crud:
	@python3 scripts/audit/crud_contract.py

audit-crud-json:
	@python3 scripts/audit/crud_contract.py --format json

audit-crud-strict:
	@python3 scripts/audit/crud_contract.py --strict

audit-debt:
	@python3 scripts/audit/debt_scan.py

audit-governance:
	@bash scripts/audit/governance_boundary.sh

ui-typecheck:
	cd ui && npm run typecheck

companion-openapi-check:
	cd ui && npm run generate:ai-types

ai-test:
	@echo "pymes/ai retirado; ejecutar tests de IA en ../axis/companion"

# Seeds y utilidades

# Carga seeds demo por el flujo único soportado (`scripts/seeds/load.sh`).
# Debe dejar datos útiles en los CRUDs principales del producto.
seed:
	bash scripts/seeds/load.sh

# Limpia datos CRUD/demo preservando bootstrap del tenant (org, users, members, settings, API keys).
seed-clear:
	bash scripts/seeds/clear.sh

# Verifica que seed-clear haya dejado vacias las pantallas operativas sin borrar bootstrap.
seed-clear-verify:
	bash scripts/seeds/verify.sh --cleared

# Verifica que los seeds de pantallas operativas tengan al menos 10 registros visibles.
seed-verify:
	bash scripts/seeds/verify.sh

# Flujo completo y repetible: limpiar, cargar y verificar.
seed-reset:
	bash scripts/seeds/clear.sh
	bash scripts/seeds/verify.sh --cleared
	bash scripts/seeds/load.sh
	bash scripts/seeds/verify.sh

# E2E del notification center gobernado por Review: request -> inbox -> approve/reject -> cleanup.
# Uso: `make e2e-governance-notifications` o `make e2e-governance-notifications DECISION=reject`
e2e-governance-notifications:
	bash scripts/e2e-governance-notifications.sh "$(DECISION)"

# Limpieza del árbol padre (p.ej. ~/Projects/Pablo): caches Python, vacíos, binarios Go sueltos bajo backend/cmd, dirs vacíos.
# Simular: make cleanup-pablo DRY_RUN=1
cleanup-pablo:
	DRY_RUN=$(DRY_RUN) bash scripts/cleanup-pablo-tree.sh "$(CURDIR)/.."

# Verificación opcional del repo `modules` desde el workspace local del ecosistema.
modules-check:
	bash -c 'M="${MODULES_REPO_PATH:-../modules}" && \
		docker compose -f "$$M/docker-compose.yml" build crud-ts-check crud-go-check'

# Stack local

# Stack local (compose Pymes). Nexus governance corre en el compose del repo ../nexus.
# Levantá Nexus antes con `make up` (o `docker compose up`) en ese repo.
up:
	$(DC) build cp-backend prof-backend work-backend beauty-backend restaurants-backend medical-backend ui
	$(DC) up -d --no-build

# Baja y elimina contenedores de la red del proyecto
down:
	$(DC) down

# Observabilidad

# Estado de contenedores
ps:
	$(DC) ps

# Seguimiento de logs de todos los servicios
logs:
	$(DC) logs -f --tail=100

# --- Docker-first: requiere contenedores en marcha (`make up`) ---

build-docker-ui:
	$(DC) exec -T ui npm run build

test-docker-ui:
	$(DC) exec -T ui npm test

lint-docker-ui:
	$(DC) exec -T ui npm run lint

test-docker-core:
	$(DC) exec -T cp-backend go test ./...

test-docker-workshops:
	$(DC) exec -T work-backend go test ./...

# Respaldo nativo

# Compila backends Go + build del frontend. Companion se valida en el repo Axis.
build:
	cd core/backend && $(GO_PRIVATE) go build ./...
	cd professionals/backend && $(GO_PRIVATE) go build ./...
	cd workshops/backend && $(GO_PRIVATE) go build ./...
	cd beauty/backend && $(GO_PRIVATE) go build ./...
	cd restaurants/backend && $(GO_PRIVATE) go build ./...
	cd ui && npm run build

# Tests (Go en core + professionals + workshops + beauty + restaurants + ui)
test:
	cd core/backend && $(GO_PRIVATE) go test ./...
	cd professionals/backend && $(GO_PRIVATE) go test ./...
	cd workshops/backend && $(GO_PRIVATE) go test ./...
	cd beauty/backend && $(GO_PRIVATE) go test ./...
	cd restaurants/backend && $(GO_PRIVATE) go test ./...
	cd ui && npm test

# E2E UI (Playwright / Chromium)
test-ui-e2e:
	cd ui && npm run test:e2e
