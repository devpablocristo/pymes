.PHONY: help dev-up dev-down build test lint

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Dev environment ──

dev-up: ## Start local dev services (postgres, mailhog)
	docker compose up -d

dev-down: ## Stop local dev services
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

# ── All services ──

build: cp-build ## Build all services

test: cp-test ## Test all services

lint: cp-vet ## Lint all services
