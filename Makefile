# Pymes — comandos frecuentes. Flujo local habitual: todo en contenedores (`make up`), no apps nativas en el host.
# docker-compose.yml en la raíz de este directorio.
.PHONY: up down build test logs ps staticcheck ruff lint seed-core-demo seed-workshops-demo

GO_PRIVATE = GOPRIVATE=github.com/devpablocristo/* GONOSUMDB=github.com/devpablocristo/* GONOPROXY=github.com/devpablocristo/* GOPROXY=direct

# Análisis estático Go (código muerto U1000, imports duplicados, etc.); versión alineada con go.mod
staticcheck:
	$(GO_PRIVATE) go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...

# Lint Python del servicio AI (ruff en ai/src); requiere `pip install -r ai/requirements.txt` o ruff en PATH
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

# Levanta stack local (Postgres, cp-backend, 4 verticales Go, frontend, AI)
up:
	docker compose up -d --build

# Baja y elimina contenedores de la red del proyecto
down:
	docker compose down

# Compila backends Go + build del frontend + chequeo básico del servicio AI
build:
	cd pymes-core/backend && $(GO_PRIVATE) go build ./...
	cd professionals/backend && $(GO_PRIVATE) go build ./...
	cd workshops/backend && $(GO_PRIVATE) go build ./...
	cd beauty/backend && $(GO_PRIVATE) go build ./...
	cd restaurants/backend && $(GO_PRIVATE) go build ./...
	cd frontend && npm run build
	cd ai && (test -x .venv/bin/python && .venv/bin/python -m compileall -q src || python3 -m compileall -q src)

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

# Seguimiento de logs de todos los servicios
logs:
	docker compose logs -f --tail=100

# Estado de contenedores
ps:
	docker compose ps
