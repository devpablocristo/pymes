# Pymes — comandos frecuentes. Flujo local habitual: todo en contenedores (`make up`), no apps nativas en el host.
# docker-compose.yml en la raíz de este directorio.
.PHONY: up down build test logs ps

GO_PRIVATE = GOPRIVATE=github.com/devpablocristo/* GONOSUMDB=github.com/devpablocristo/* GONOPROXY=github.com/devpablocristo/* GOPROXY=direct

# Levanta stack local (Postgres, backends, frontend, AI, etc.)
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
	cd frontend && npm run build
	cd ai && (test -x .venv/bin/python && .venv/bin/python -m compileall -q src || python3 -m compileall -q src)

# Tests (Go en los tres backends + frontend + AI si hay pytest)
test:
	cd pymes-core/backend && $(GO_PRIVATE) go test ./...
	cd professionals/backend && $(GO_PRIVATE) go test ./...
	cd workshops/backend && $(GO_PRIVATE) go test ./...
	cd frontend && npm test
	cd ai && (test -x .venv/bin/pytest && .venv/bin/pytest -q || pytest -q)

# Seguimiento de logs de todos los servicios
logs:
	docker compose logs -f --tail=100

# Estado de contenedores
ps:
	docker compose ps
