.PHONY: help up down build test lint frontend-dev frontend-build frontend-test ai-dev ai-test ai-lint cp-build cp-test cp-vet cp-run prof-build prof-test prof-vet prof-run

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Dev environment ──

up: ## Start local dev services (all Docker services)
	docker compose up -d --build

down: ## Stop local dev services
	docker compose down

# ── Control Plane ──

cp-build: ## Build control-plane backend
	cd control-plane/backend && go build ./...

cp-test: ## Run control-plane backend tests
	cd control-plane/backend && go test ./...

cp-vet: ## Run go vet on control-plane backend
	cd control-plane/backend && go vet ./...

cp-run: ## Run control-plane backend locally
	cd control-plane/backend && go run ./cmd/local

# ── Frontend (unified) ──

frontend-dev: ## Run frontend dev server
	cd frontend && npm run dev

frontend-build: ## Build frontend
	cd frontend && npm run build

frontend-test: ## Run frontend tests
	cd frontend && npm test

# ── AI service (unified) ──

ai-dev: ## Run AI service locally (uvicorn reload)
	cd ai && (test -x .venv/bin/uvicorn && .venv/bin/uvicorn src.main:app --host 0.0.0.0 --port 8000 --reload || uvicorn src.main:app --host 0.0.0.0 --port 8000 --reload)

ai-test: ## Run AI service tests
	cd ai && (test -x .venv/bin/pytest && .venv/bin/pytest -q || pytest -q)

ai-lint: ## Basic AI static check (compile)
	cd ai && env PYTHONPYCACHEPREFIX=/tmp/ai-lint sh -c 'test -x .venv/bin/python && .venv/bin/python -m compileall -q src || python -m compileall -q src'

# ── Professionals Vertical ──

prof-build: ## Build professionals backend
	cd professionals/backend && go build ./...

prof-test: ## Run professionals backend tests
	cd professionals/backend && go test ./...

prof-vet: ## Run go vet on professionals backend
	cd professionals/backend && go vet ./...

prof-run: ## Run professionals backend locally
	cd professionals/backend && go run ./cmd/local

# ── All services ──

build: cp-build prof-build frontend-build ai-lint ## Build all services

test: cp-test prof-test frontend-test ai-test ## Test all services

lint: cp-vet prof-vet ai-lint ## Lint all services
