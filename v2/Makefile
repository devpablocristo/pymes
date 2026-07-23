ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: up down ps logs smoke backend-run backend-test db-up db-down db-migrate db-test db-integration web-dev web-ci ci

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
	docker compose exec -T postgres sh -ec \
		'test "$$(psql -U pymes -d pymes_v2 -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM pg_namespace WHERE nspname = '\''app'\''")" = 1'

backend-run:
	cd backend && GOWORK=off go run ./cmd/api

backend-test:
	cd backend && GOWORK=off go test ./...

db-up:
	docker compose up -d --wait postgres

db-down:
	$(MAKE) down

db-migrate:
	cd db && GOWORK=off go run ./cmd/migrate

db-test:
	cd db && GOWORK=off go test ./...

db-integration: db-up
	cd db && PYMES_DATABASE_URL='postgres://pymes:pymes@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go run ./cmd/migrate
	cd db && PYMES_DATABASE_URL='postgres://pymes:pymes@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go run ./cmd/migrate
	docker compose restart postgres
	docker compose up -d --wait postgres
	cd db && PYMES_DATABASE_URL='postgres://pymes:pymes@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go run ./cmd/migrate
	cd db && PYMES_DATABASE_TEST_URL='postgres://pymes:pymes@127.0.0.1:55433/pymes_v2_test?sslmode=disable' GOWORK=off go test -count=1 ./...

web-dev:
	cd web && npm run dev

web-ci:
	cd web && npm ci && npm run typecheck && npm test && npm run build

ci: backend-test db-test web-ci
