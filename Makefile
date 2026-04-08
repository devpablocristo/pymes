# Pymes — flujo local habitual: todo en contenedores (`make up`).
# Verificación preferida: targets `*-docker-*`; `build` y `test` quedan como respaldo nativo.
.PHONY: \
	up down ps logs \
	staticcheck ruff lint \
	seed seed-clear modules-check cleanup-pablo e2e-review-notifications \
	build-docker-frontend test-docker-frontend lint-docker-frontend test-docker-core test-docker-workshops \
	build test test-frontend-e2e

GO_PRIVATE = GOPRIVATE=github.com/devpablocristo/* GONOSUMDB=github.com/devpablocristo/* GONOPROXY=github.com/devpablocristo/* GOPROXY=direct
DC = docker compose
DC_MODULES = docker compose -f ../modules/docker-compose.yml

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
seed:
	bash scripts/seeds/load.sh

# Limpia seeds demo por el flujo único soportado (`scripts/seeds/clear.sh`).
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

# Módulo CRUD en ../modules: typecheck + tests TS y go test, todo en imágenes Docker (sin npm/go en el host).
modules-check:
	$(DC_MODULES) build crud-ts-check crud-go-check

# Stack local

# Levanta stack local (Postgres Pymes, Review, cp-backend, 4 verticales Go, frontend, AI)
up:
	$(DC) up -d --build

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
