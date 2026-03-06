.PHONY: help up down build test lint ai-dev ai-test ai-lint

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Dev environment ──

up: ## Start local dev services (all Docker services)
	docker compose up -d

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

cp-frontend-dev: ## Run control-plane frontend dev server
	cd control-plane/frontend && npm run dev

cp-frontend-build: ## Build control-plane frontend
	cd control-plane/frontend && npm run build

# ── AI service ──

ai-dev: ## Run AI service locally (uvicorn reload)
	cd control-plane/ai && (test -x .venv/bin/uvicorn && .venv/bin/uvicorn src.main:app --host 0.0.0.0 --port 8000 --reload || uvicorn src.main:app --host 0.0.0.0 --port 8000 --reload)

ai-test: ## Run AI service tests
	cd control-plane/ai && (test -x .venv/bin/pytest && .venv/bin/pytest -q || pytest -q)

ai-lint: ## Basic AI static check (compile)
	cd control-plane/ai && (test -x .venv/bin/python && .venv/bin/python -m compileall -q src || python -m compileall -q src)

# ── Professionals Vertical ──

prof-build: ## Build professionals backend
	cd professionals/backend && go build ./...

prof-test: ## Run professionals backend tests
	cd professionals/backend && go test ./...

prof-vet: ## Run go vet on professionals backend
	cd professionals/backend && go vet ./...

prof-run: ## Run professionals backend locally
	cd professionals/backend && go run ./cmd/local

prof-frontend-dev: ## Run professionals frontend dev server
	cd professionals/frontend && npm run dev

prof-frontend-build: ## Build professionals frontend
	cd professionals/frontend && npm run build

prof-ai-dev: ## Run professionals AI service locally
	cd professionals/ai && (test -x .venv/bin/uvicorn && .venv/bin/uvicorn src.main:app --host 0.0.0.0 --port 8001 --reload || uvicorn src.main:app --host 0.0.0.0 --port 8001 --reload)

prof-ai-test: ## Run professionals AI tests
	cd professionals/ai && (test -x .venv/bin/pytest && .venv/bin/pytest -q || pytest -q)

prof-ai-lint: ## Basic professionals AI static check
	cd professionals/ai && (test -x .venv/bin/python && .venv/bin/python -m compileall -q src || python -m compileall -q src)

# ── All services ──

build: cp-build prof-build ai-lint prof-ai-lint ## Build all services

test: cp-test prof-test ai-test prof-ai-test ## Test all services

lint: cp-vet prof-vet ai-lint prof-ai-lint ## Lint all services
