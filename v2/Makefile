ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: backend-run backend-test db-up db-down db-migrate db-test db-integration web-dev web-ci ci

backend-run:
	cd backend && GOWORK=off go run ./cmd/api

backend-test:
	cd backend && GOWORK=off go test ./...

db-up:
	docker compose up -d --wait postgres

db-down:
	docker compose down

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
