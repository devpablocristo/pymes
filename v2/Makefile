ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE_PROFILES ?= $(if $(strip $(PYMES_CLERK_SECRET_KEY)),iam,)
export COMPOSE_PROFILES

.DEFAULT_GOAL := help

.PHONY: help up down ps logs smoke clerk-check provision-org api-generate api-check backend-run backend-test db-up db-bootstrap db-down db-migrate db-test db-integration web-dev web-ci ci

help:
	@echo "Pymes v2"
	@echo ""
	@echo "  make up              Construye y levanta postgres, migrate, backend y web"
	@echo "                       (también iam-worker cuando Clerk está configurado)"
	@echo "  make smoke           Verifica base, API y web"
	@echo "  make clerk-check     Verifica la configuración remota de Clerk sin modificarla"
	@echo "  make provision-org ORG_NAME='Acme' ORG_SLUG=acme OWNER_EMAIL=owner@example.com"
	@echo "  make api-generate     Regenera contratos Go y TypeScript desde OpenAPI"
	@echo "  make api-check        Verifica que el código generado no tenga drift"
	@echo "  make ps              Muestra el estado del stack"
	@echo "  make logs            Sigue los logs"
	@echo "  make down            Detiene el stack sin borrar el volumen"
	@echo "  make ci              Ejecuta los checks nativos de v2"
	@echo "  make db-integration  Prueba migraciones y reinicio de PostgreSQL"

up:
	docker compose stop backend web >/dev/null 2>&1 || true
	docker compose rm -f migrate >/dev/null 2>&1 || true
	docker compose up --build --detach --wait --wait-timeout 180

down:
	docker compose down

ps:
	docker compose ps

logs:
	docker compose logs --follow --tail=200

smoke:
	docker compose exec -T backend wget -q -O - http://127.0.0.1:8080/healthz >/dev/null
	docker compose exec -T backend wget -q -O - http://127.0.0.1:8080/readyz >/dev/null
	docker compose exec -T web wget -q -O - http://127.0.0.1:8080/healthz >/dev/null
	test "$$(curl --fail --silent http://127.0.0.1:$${PYMES_WEB_PORT:-15173}/api/v1/runtime-config | tr -d '[:space:]')" = '{"auth":{"configured":false,"provider":"clerk"}}' || test -n "$${PYMES_CLERK_PUBLISHABLE_KEY:-}"
	docker compose exec -T postgres sh -ec \
		'test "$$(psql -U pymes -d pymes_v2 -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM pg_namespace WHERE nspname = '\''app'\''")" = 1'
	docker compose exec -T postgres sh -ec \
		'test "$$(psql -U pymes -d pymes_v2 -v ON_ERROR_STOP=1 -tAc "SELECT count(*) FROM schema_migrations WHERE scope = '\''iam/core'\''")" = 1'
	docker compose exec -T postgres sh -ec \
		'test "$$(psql -U pymes -d pymes_v2 -v ON_ERROR_STOP=1 -tAc "SELECT count(*) FROM schema_migrations WHERE scope = '\''outbox/core'\''")" = 1'
	docker compose exec -T postgres sh -ec \
		'test "$$(psql -U pymes -d pymes_v2 -v ON_ERROR_STOP=1 -tAc "SELECT count(*) FROM schema_migrations WHERE scope = '\''idempotency/core'\''")" = 1'
	docker compose exec -T postgres sh -ec \
		'test "$$(psql -U pymes -d pymes_v2 -v ON_ERROR_STOP=1 -tAc "SELECT count(*) FROM schema_migrations WHERE scope = '\''pymes-v2'\''")" = 7'

clerk-check:
	./scripts/check-clerk-config.sh

provision-org:
	@test -n "$(strip $(ORG_NAME))" || (echo "ORG_NAME es obligatorio" >&2; exit 2)
	@test -n "$(strip $(ORG_SLUG))" || (echo "ORG_SLUG es obligatorio" >&2; exit 2)
	@test -n "$(strip $(OWNER_EMAIL))" || (echo "OWNER_EMAIL es obligatorio" >&2; exit 2)
	docker compose up -d --wait postgres
	docker compose build migrate provision-org
	docker compose run --rm --no-deps db-bootstrap
	docker compose run --rm --no-deps migrate
	docker compose run --rm --no-deps provision-org \
		--name "$(ORG_NAME)" \
		--slug "$(ORG_SLUG)" \
		--owner-email "$(OWNER_EMAIL)"

api-generate:
	cd backend && GOWORK=off go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --config oapi-codegen.yaml ../api/openapi.yaml
	cd web && npm run api:generate

api-check: api-generate
	git -C .. diff --exit-code -- v2/backend/internal/api/openapi.gen.go v2/web/src/api/schema.generated.ts

backend-run:
	cd backend && GOWORK=off go run ./cmd/api

backend-test:
	cd backend && GOWORK=off go test ./...

db-up:
	docker compose up -d --wait postgres
	docker compose run --rm --no-deps db-bootstrap

db-bootstrap:
	docker compose up -d --wait postgres
	docker compose run --rm --no-deps db-bootstrap

db-down:
	$(MAKE) down

db-migrate:
	cd db && PYMES_DATABASE_URL='$(PYMES_MIGRATOR_DATABASE_URL)' GOWORK=off go run ./cmd/migrate

db-test:
	cd db && GOWORK=off go test ./...

db-integration: db-up
	cd db && PYMES_DATABASE_URL='postgres://pymes:pymes@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go run ./cmd/migrate
	cd db && PYMES_DATABASE_URL='postgres://pymes:pymes@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go run ./cmd/migrate
	docker compose restart postgres
	docker compose up -d --wait postgres
	cd db && PYMES_DATABASE_URL='postgres://pymes:pymes@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go run ./cmd/migrate
	cd db && PYMES_DATABASE_TEST_URL='postgres://pymes:pymes@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go test -count=1 ./...
	cd db && PYMES_DATABASE_URL='postgres://pymes_migrator:pymes_migrator@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go run ./cmd/migrate
	cd backend && PYMES_DATABASE_TEST_URL='postgres://pymes_migrator:pymes_migrator@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go test -count=1 ./internal/provisioning ./internal/iamoutbox

web-dev:
	cd web && npm run dev

web-ci:
	cd web && npm ci && npm run api:generate && npm run typecheck && npm test && npm run build

ci: api-check backend-test db-test web-ci
