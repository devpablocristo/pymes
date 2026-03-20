.PHONY: help up down build test lint frontend-dev frontend-build frontend-test ai-dev ai-test ai-lint cp-build cp-test cp-vet cp-run prof-build prof-test prof-vet prof-run work-build work-test work-vet work-run

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Dev environment ──

up: ## Start local dev services (all Docker services)
	docker compose up -d --build

down: ## Stop local dev services
	docker compose down

# ── Control Plane ──

cp-build: ## Build pymes-core backend
	cd pymes-core/backend && go build ./...

cp-test: ## Run pymes-core backend tests
	cd pymes-core/backend && go test ./...

cp-vet: ## Run go vet on pymes-core backend
	cd pymes-core/backend && go vet ./...

cp-run: ## Run pymes-core backend locally
	cd pymes-core/backend && go run ./cmd/local

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

# ── Workshops Vertical ──

work-build: ## Build workshops backend
	cd workshops/backend && go build ./...

work-test: ## Run workshops backend tests
	cd workshops/backend && go test ./...

work-vet: ## Run go vet on workshops backend
	cd workshops/backend && go vet ./...

work-run: ## Run workshops backend locally
	cd workshops/backend && go run ./cmd/local

# ── All services ──

build: cp-build prof-build work-build frontend-build ai-lint ## Build all services

test: cp-test prof-test work-test frontend-test ai-test ## Test all services

lint: cp-vet prof-vet work-vet ai-lint ## Lint all services
