# Pymes — comandos frecuentes. Flujo local habitual: todo en contenedores (`make up`), no apps nativas en el host.
# Verificación preferida: targets test-docker-* / build-docker-* (requieren `make up`).
# build/test nativos abajo = respaldo/CI rápido en host si no hay Docker.
# docker-compose.yml en la raíz de este directorio.
.PHONY: up down build test build-docker-frontend test-docker-frontend lint-docker-frontend test-docker-core test-docker-workshops logs ps staticcheck ruff lint seed-core-demo seed-workshops-demo seed-docker-core seed-docker-workshops seed-docker-all modules-check cleanup-pablo e2e-review-notifications

GO_PRIVATE = GOPRIVATE=github.com/devpablocristo/* GONOSUMDB=github.com/devpablocristo/* GONOPROXY=github.com/devpablocristo/* GOPROXY=direct

# Análisis estático Go (código muerto U1000, imports duplicados, etc.); versión alineada con go.mod
staticcheck:
	$(GO_PRIVATE) go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...

# Lint Python del servicio AI (ruff en ai/src); requiere `pip install -r ai/requirements-dev.txt` o ruff en PATH
ruff:
	cd ai && (test -x .venv/bin/ruff && .venv/bin/ruff check src || ruff check src || python3 -m ruff check src)

# Go staticcheck + ruff AI
lint: staticcheck ruff

# Demo SQL del control plane (orden: org → negocio → RBAC → transversal). No son migraciones.
# En Docker: PYMES_SEED_DEMO=true en cp-backend aplica lo mismo al arrancar.
seed-core-demo:
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Definí DATABASE_URL (ej. postgres://postgres:postgres@localhost:5434/pymes?sslmode=disable)" >&2; \
		exit 1; \
	fi
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f pymes-core/backend/seeds/01_local_org.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f pymes-core/backend/seeds/02_core_business.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f pymes-core/backend/seeds/03_rbac.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f pymes-core/backend/seeds/04_transversal_modules_demo.sql

# Demo workshops auto_repair (misma DB que el core; requiere seed-core-demo antes).
# En Docker: PYMES_SEED_DEMO=true en work-backend.
seed-workshops-demo:
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Definí DATABASE_URL" >&2; \
		exit 1; \
	fi
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f workshops/backend/seeds/auto_repair_demo.sql

# Seeds 01–04 del core contra Postgres del `docker compose` (idempotente).
# Útil si cp-backend no aplicó demo o reiniciaste volumen sin resembrar.
seed-docker-core:
	docker compose exec -T postgres psql -U postgres -d pymes -v ON_ERROR_STOP=1 < pymes-core/backend/seeds/01_local_org.sql
	docker compose exec -T postgres psql -U postgres -d pymes -v ON_ERROR_STOP=1 < pymes-core/backend/seeds/02_core_business.sql
	docker compose exec -T postgres psql -U postgres -d pymes -v ON_ERROR_STOP=1 < pymes-core/backend/seeds/03_rbac.sql
	docker compose exec -T postgres psql -U postgres -d pymes -v ON_ERROR_STOP=1 < pymes-core/backend/seeds/04_transversal_modules_demo.sql

# Misma semilla auto_repair contra Postgres del `docker compose` (org demo 000...001).
# Útil si work-backend arrancó sin PYMES_SEED_DEMO o falló el seed al inicio.
seed-docker-workshops:
	docker compose exec -T postgres psql -U postgres -d pymes -v ON_ERROR_STOP=1 < workshops/backend/seeds/auto_repair_demo.sql

# Demo completo en Docker: core + talleres (un solo comando tras `docker compose up`).
seed-docker-all: seed-docker-core seed-docker-workshops

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
	docker compose -f ../modules/docker-compose.yml build crud-ts-check crud-go-check

# Levanta stack local (Postgres Pymes, Review, cp-backend, 4 verticales Go, frontend, AI)
up:
	docker compose up -d --build

# Baja y elimina contenedores de la red del proyecto
down:
	docker compose down

# --- Docker-first: requiere contenedores en marcha (`make up`) ---
build-docker-frontend:
	docker compose exec -T frontend npm run build

test-docker-frontend:
	docker compose exec -T frontend npm test

lint-docker-frontend:
	docker compose exec -T frontend npm run lint

test-docker-core:
	docker compose exec -T cp-backend go test ./...

test-docker-workshops:
	docker compose exec -T work-backend go test ./...

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

# E2E frontend: recorre todas las rutas Wowdash (Chromium; build sin Clerk; ~3–4 min; requiere `npm run test:e2e:wowdash:install` una vez)
test-frontend-e2e-wowdash:
	cd frontend && npm run test:e2e:wowdash

# Seguimiento de logs de todos los servicios
logs:
	docker compose logs -f --tail=100

# Estado de contenedores
ps:
	docker compose ps
