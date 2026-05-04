# Pymes — flujo local habitual: todo en contenedores (`make up`).
# Verificación preferida: targets `*-docker-*`; `build` y `test` quedan como respaldo nativo.
.PHONY: \
	up down ps logs \
	staticcheck ruff lint \
	seed seed-clear modules-check cleanup-pablo e2e-review-notifications \
	build-docker-frontend test-docker-frontend lint-docker-frontend test-docker-core test-docker-workshops \
	build test test-frontend-e2e

GO_PRIVATE = GOPRIVATE=github.com/devpablocristo/* GONOSUMDB=github.com/devpablocristo/* GONOPROXY=github.com/devpablocristo/* GOPROXY=https://proxy.golang.org,direct
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

# Nexus Governance (servicio Docker `review`): mismo módulo que en CI (`../nexus/governance`) o checkout renombrado `../nexus-governance/governance`.
ifneq ($(wildcard $(abspath $(CURDIR)/../nexus/governance/go.mod)),)
  NEXUS_GOVERNANCE_CONTEXT := $(abspath $(CURDIR)/../nexus/governance)
else ifneq ($(wildcard $(abspath $(CURDIR)/../nexus-governance/governance/go.mod)),)
  NEXUS_GOVERNANCE_CONTEXT := $(abspath $(CURDIR)/../nexus-governance/governance)
else
  NEXUS_GOVERNANCE_CONTEXT := $(abspath $(CURDIR)/../nexus/governance)
endif
export NEXUS_GOVERNANCE_CONTEXT

# Calidad

# Análisis estático Go (código muerto U1000, imports duplicados, etc.); versión alineada con go.mod
staticcheck:
	$(GO_PRIVATE) go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...
# Lint Python del servicio AI (ruff en ai/src); requiere `pip install -r ai/requirements-dev.txt` o ruff en PATH
ruff:
	cd ai && (test -x .venv/bin/ruff && .venv/bin/ruff check src || ruff check src || python3 -m ruff check src)

# Go staticcheck + ruff AI
lint: staticcheck ruff

# Seeds y utilidades

# Carga seeds demo por el flujo único soportado (`scripts/seeds/load.sh`).
# Debe dejar datos útiles en los CRUDs principales del producto.
seed:
	bash scripts/seeds/load.sh

# Limpia datos CRUD/demo preservando bootstrap del tenant (org, users, members, settings, API keys).
seed-clear:
	bash scripts/seeds/clear.sh

# E2E del notification center gobernado por Review: request -> inbox -> approve/reject -> cleanup.
# Uso: `make e2e-review-notifications` o `make e2e-review-notifications DECISION=reject`
e2e-review-notifications:
	bash scripts/e2e-review-notifications.sh "$(DECISION)"

# Limpieza del árbol padre (p.ej. ~/Projects/Pablo): caches Python, vacíos, binarios Go sueltos bajo backend/cmd, dirs vacíos.
# Simular: make cleanup-pablo DRY_RUN=1
cleanup-pablo:
	DRY_RUN=$(DRY_RUN) bash scripts/cleanup-pablo-tree.sh "$(CURDIR)/.."

# Verificación opcional del repo `modules` desde el workspace local del ecosistema.
modules-check:
	bash -c 'M="${MODULES_REPO_PATH:-../modules}" && \
		docker compose -f "$$M/docker-compose.yml" build crud-ts-check crud-go-check'

# Stack local

# Levanta Ollama compartido del ecosistema local (opcional si existe el compose en LOCAL_INFRA_DIR)
llm-up:
	@if [ -f "$(LOCAL_INFRA_DIR)/docker-compose.ollama.yml" ]; then \
		docker compose --project-directory "$(LOCAL_INFRA_DIR)" -f "$(LOCAL_INFRA_DIR)/docker-compose.ollama.yml" up -d; \
	else \
		echo "Skipping llm-up: $(LOCAL_INFRA_DIR)/docker-compose.ollama.yml not found (clone local-infra alongside pymes or set LOCAL_INFRA_DIR)."; \
	fi

# Asegura el modelo LLM local por defecto en el Ollama compartido
llm-pull:
	@if [ -f "$(LOCAL_INFRA_DIR)/docker-compose.ollama.yml" ] && [ -f "$(LOCAL_INFRA_DIR)/scripts/pull-ollama-model.sh" ]; then \
		bash "$(LOCAL_INFRA_DIR)/scripts/pull-ollama-model.sh" gemma4:e4b; \
	else \
		echo "Skipping llm-pull: no Ollama stack under LOCAL_INFRA_DIR=$(LOCAL_INFRA_DIR)."; \
	fi

# Levanta stack local (infra compartida + Review + cp-backend + 4 verticales Go + frontend + AI)
up:
	@if [ ! -f "$(NEXUS_GOVERNANCE_CONTEXT)/go.mod" ]; then \
		echo "Falta Nexus governance en $(NEXUS_GOVERNANCE_CONTEXT)/go.mod — cloná github.com/devpablocristo/nexus junto a pymes como ../nexus o ../nexus-governance." >&2; \
		exit 1; \
	fi
	@$(MAKE) llm-up
	@$(MAKE) llm-pull
	$(DC) build review cp-backend prof-backend work-backend beauty-backend restaurants-backend medical-backend frontend ai
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

build-docker-frontend:
	$(DC) exec -T frontend npm run build

test-docker-frontend:
	$(DC) exec -T frontend npm test

lint-docker-frontend:
	$(DC) exec -T frontend npm run lint

test-docker-core:
	$(DC) exec -T cp-backend go test ./...

test-docker-workshops:
	$(DC) exec -T work-backend go test ./...

# Respaldo nativo

# Compila backends Go + build del frontend + chequeo básico del servicio AI (nativo en host)
build:
	cd pymes-core/backend && $(GO_PRIVATE) go build ./...
	cd professionals/backend && $(GO_PRIVATE) go build ./...
	cd workshops/backend && $(GO_PRIVATE) go build ./...
	cd beauty/backend && $(GO_PRIVATE) go build ./...
	cd restaurants/backend && $(GO_PRIVATE) go build ./...
	cd frontend && npm run build
	cd ai && _pc=$$(mktemp -d) && export PYTHONPYCACHEPREFIX=$$_pc && (test -x .venv/bin/python && .venv/bin/python -m compileall -q src || python3 -m compileall -q src); _e=$$?; rm -rf $$_pc; exit $$_e

# Tests (Go en pymes-core + professionals + workshops + beauty + restaurants + frontend + AI)
test:
	cd pymes-core/backend && $(GO_PRIVATE) go test ./...
	cd professionals/backend && $(GO_PRIVATE) go test ./...
	cd workshops/backend && $(GO_PRIVATE) go test ./...
	cd beauty/backend && $(GO_PRIVATE) go test ./...
	cd restaurants/backend && $(GO_PRIVATE) go test ./...
	cd frontend && npm test
	@$(MAKE) ruff
	cd ai && (test -x .venv/bin/pytest && .venv/bin/pytest -q || pytest -q)

# E2E frontend (Playwright / Chromium)
test-frontend-e2e:
	cd frontend && npm run test:e2e
