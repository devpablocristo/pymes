.DEFAULT_GOAL := help
.PHONY: help test fiscal-test accounting-test quality security fiscal-e2e accounting-e2e api-check api-generate db-integration backup-restore-smoke e2e ci up down backup restore

ACCOUNTING_BUILD_CONTEXT ?= ../../open-accounting

help:
	@echo "Pymes v3"
	@echo "  make test            Ejecuta pruebas Go y del fiscal adapter mock"
	@echo "  make fiscal-test     Ejecuta typecheck y pruebas TypeScript fiscales"
	@echo "  make accounting-test Ejecuta las suites del runtime contable headless"
	@echo "  make quality         Ejecuta vet y valida Compose"
	@echo "  make security        Verifica vulnerabilidades alcanzables en los tres runtimes"
	@echo "  make fiscal-e2e      Valida el cliente Go contra el backend fiscal mock"
	@echo "  make accounting-e2e  Valida el cliente Go contra Open Accounting headless"
	@echo "  make api-check       Valida los contratos OpenAPI YAML"
	@echo "  make api-generate    Regenera servidor/tipos/clientes desde OpenAPI"
	@echo "  make db-integration  Aplica migraciones dos veces sobre PostgreSQL local"
	@echo "  make backup-restore-smoke  Prueba backup y restore de las tres bases"
	@echo "  make e2e             Ejecuta las invariantes de la vertical comercial"
	@echo "  make ci              Ejecuta todos los checks locales"
	@echo "  make backup SERVICE=pymes|fiscal|accounting FILE=…"
	@echo "  make restore SERVICE=pymes|fiscal|accounting FILE=…"

test:
	cd backend && go test ./...
	$(MAKE) fiscal-test
	$(MAKE) accounting-test

fiscal-test:
	cd fiscal-adapter && npm ci --ignore-scripts && npm run ci

accounting-test:
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go test ./internal/accounting ./internal/tenant ./internal/pymesaccounting ./internal/database/pymesaccounting ./cmd/pymes-accounting
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go test ./docs -run 'TestRuntimePersistenceStaysBehindRepositories|TestRepositoryRawSQLExceptionsAreExplicit'

quality:
	cd backend && go vet ./...
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go vet ./internal/accounting ./internal/tenant ./internal/pymesaccounting ./internal/database/pymesaccounting ./cmd/pymes-accounting
	docker compose config --quiet

security:
	cd backend && go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./internal/accounting ./internal/tenant ./internal/pymesaccounting ./internal/database/pymesaccounting ./cmd/pymes-accounting
	cd fiscal-adapter && npm audit --omit=dev

api-check:
	python3 -c 'import yaml; [yaml.safe_load(open(p)) for p in ["api/openapi.yaml", "contracts/accounting.openapi.yaml", "contracts/fiscal.openapi.yaml"]]'
	./scripts/contracts-check.sh

api-generate:
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package publicapi --generate types,chi-server -o internal/contracts/publicapi/public.gen.go ../api/openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package accountingapi --generate types,client -o internal/contracts/accountingapi/accounting.gen.go ../contracts/accounting.openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package fiscalapi --generate types,client -o internal/contracts/fiscalapi/fiscal.gen.go ../contracts/fiscal.openapi.yaml

up:
	docker compose up --build --detach --wait

down:
	docker compose down

db-integration:
	./scripts/db-integration.sh

e2e:
	$(MAKE) db-integration
	$(MAKE) backup-restore-smoke
	cd backend && PYMES_DATABASE_TEST_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable' go test ./internal/commerce/repository -run 'TestDurableWorkerRecoversLostFiscalAndAccountingResponses' -count=1
	$(MAKE) fiscal-e2e
	$(MAKE) accounting-e2e

fiscal-e2e:
	./scripts/fiscal-contract-e2e.sh

accounting-e2e:
	./scripts/accounting-contract-e2e.sh

backup-restore-smoke:
	./scripts/backup-restore-smoke.sh

ci:
	$(MAKE) api-check
	$(MAKE) quality
	$(MAKE) security
	$(MAKE) test
	$(MAKE) e2e

backup:
	SERVICE="$(or $(SERVICE),pymes)" ./scripts/backup-postgres.sh "$(FILE)"

restore:
	SERVICE="$(or $(SERVICE),pymes)" ./scripts/restore-postgres.sh "$(FILE)"
