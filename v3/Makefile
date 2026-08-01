.DEFAULT_GOAL := help
.PHONY: help build migration-image-check test fiscal-test accounting-test quality security fiscal-e2e accounting-e2e commercial-e2e observability-e2e recovery-e2e replay-smoke monitoring-config-check api-check api-generate db-integration backup-restore-smoke e2e ci up down backup restore

ACCOUNTING_BUILD_CONTEXT ?= ../../open-accounting

help:
	@echo "Pymes v3"
	@echo "  make build           Construye API, worker, migraciones y adaptadores desplegables"
	@echo "  make migration-image-check  Construye las imágenes de migración Cloud Run"
	@echo "  make test            Ejecuta pruebas Go y del fiscal adapter mock"
	@echo "  make fiscal-test     Ejecuta typecheck y pruebas TypeScript fiscales"
	@echo "  make accounting-test Ejecuta las suites del runtime contable headless"
	@echo "  make quality         Ejecuta vet y valida Compose"
	@echo "  make security        Verifica vulnerabilidades alcanzables en los tres runtimes"
	@echo "  make fiscal-e2e      Valida el cliente Go contra el backend fiscal mock"
	@echo "  make accounting-e2e  Valida el cliente Go contra Open Accounting headless"
	@echo "  make commercial-e2e  Recorre la vertical pública completa contra servicios reales/mocks"
	@echo "  make observability-e2e  Comprueba salud, readiness y métricas del stack"
	@echo "  make recovery-e2e   Comprueba caída y recuperación independiente de servicios y bases"
	@echo "  make replay-smoke   Comprueba replay DLQ idempotente y auditoría inmutable"
	@echo "  make monitoring-config-check  Valida Monitoring STG/PRD sin llamar a GCP"
	@echo "  make api-check       Valida los contratos OpenAPI YAML"
	@echo "  make api-generate    Regenera servidor/tipos/clientes desde OpenAPI"
	@echo "  make db-integration  Aplica migraciones dos veces sobre PostgreSQL local"
	@echo "  make backup-restore-smoke  Prueba backup y restore de las tres bases"
	@echo "  make e2e             Ejecuta las invariantes de la vertical comercial"
	@echo "  make ci              Ejecuta todos los checks locales"
	@echo "  make backup SERVICE=pymes|fiscal|accounting FILE=…"
	@echo "  make restore SERVICE=pymes|fiscal|accounting FILE=…"

build:
	docker compose build

migration-image-check:
	docker build --target migrate -q . >/dev/null
	docker build --target fiscal-migrate -q . >/dev/null
	docker build --target pymes-accounting-migrate -q "$(ACCOUNTING_BUILD_CONTEXT)" >/dev/null
	docker build --target pymes-accounting-admin -q "$(ACCOUNTING_BUILD_CONTEXT)" >/dev/null

test:
	cd backend && go test ./...
	$(MAKE) fiscal-test
	$(MAKE) accounting-test

fiscal-test:
	cd fiscal-adapter && npm ci --ignore-scripts && npm run ci

accounting-test:
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go test ./internal/accounting ./internal/tenant ./internal/pymesaccounting ./internal/database/pymesaccounting ./internal/database/pymesaccountingadmin ./internal/pymesaccountingadminhttp ./cmd/pymes-accounting ./cmd/pymes-accounting-admin
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go test ./docs -run 'TestRuntimePersistenceStaysBehindRepositories|TestRepositoryRawSQLExceptionsAreExplicit'

quality:
	cd backend && go vet ./...
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go vet ./internal/accounting ./internal/tenant ./internal/pymesaccounting ./internal/database/pymesaccounting ./internal/database/pymesaccountingadmin ./internal/pymesaccountingadminhttp ./cmd/pymes-accounting ./cmd/pymes-accounting-admin
	for script in scripts/*.sh; do sh -n "$$script"; done
	bash -n scripts/deploy/cloud-run.sh
	bash -n scripts/deploy/bootstrap-internal-identity.sh
	bash -n scripts/deploy/bootstrap-workload-identities.sh
	bash -n scripts/deploy/bootstrap-cloudsql.sh
	bash -n scripts/deploy/migrate-regional-secrets.sh
	bash -n scripts/deploy/provision-monitoring.sh
	$(MAKE) monitoring-config-check
	docker compose config --quiet

security:
	cd backend && go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./internal/accounting ./internal/tenant ./internal/pymesaccounting ./internal/database/pymesaccounting ./internal/database/pymesaccountingadmin ./internal/pymesaccountingadminhttp ./cmd/pymes-accounting ./cmd/pymes-accounting-admin
	cd fiscal-adapter && npm audit --omit=dev

api-check:
	python3 -c 'import yaml; [yaml.safe_load(open(p)) for p in ["api/openapi.yaml", "contracts/accounting.openapi.yaml", "contracts/fiscal.openapi.yaml", "monitoring/alerts.yml"]]'
	./scripts/contracts-check.sh

api-generate:
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --exclude-operation-ids clerkWebhook --package publicapi --generate types,chi-server -o internal/contracts/publicapi/public.gen.go ../api/openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package accountingapi --generate types,client -o internal/contracts/accountingapi/accounting.gen.go ../contracts/accounting.openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package accountingapi --generate chi-server -o internal/contracts/accountingapi/accounting.server.gen.go ../contracts/accounting.openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package fiscalapi --generate types,client -o internal/contracts/fiscalapi/fiscal.gen.go ../contracts/fiscal.openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package fiscalapi --generate chi-server -o internal/contracts/fiscalapi/fiscal.server.gen.go ../contracts/fiscal.openapi.yaml

up:
	docker compose up --build --detach --wait

down:
	docker compose down

db-integration:
	./scripts/db-integration.sh

e2e:
	$(MAKE) db-integration
	$(MAKE) commercial-e2e
	$(MAKE) backup-restore-smoke
	$(MAKE) observability-e2e
	$(MAKE) recovery-e2e
	$(MAKE) replay-smoke
	cd backend && PYMES_DATABASE_TEST_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable' go test ./internal/commerce/repository -run 'TestDurableWorkerRecoversLostFiscalAndAccountingResponses' -count=1
	$(MAKE) fiscal-e2e
	$(MAKE) accounting-e2e

fiscal-e2e:
	./scripts/fiscal-contract-e2e.sh

accounting-e2e:
	./scripts/accounting-contract-e2e.sh

commercial-e2e:
	./scripts/commercial-vertical-e2e.sh

observability-e2e:
	./scripts/observability-e2e.sh

recovery-e2e:
	./scripts/recovery-e2e.sh

replay-smoke:
	./scripts/replay-dead-letter-smoke.sh

monitoring-config-check:
	PYMES_DEPLOY_ENV=stg PYMES_MONITORING_DRY_RUN=true ./scripts/deploy/provision-monitoring.sh >/dev/null
	PYMES_DEPLOY_ENV=prd PYMES_MONITORING_DRY_RUN=true ./scripts/deploy/provision-monitoring.sh >/dev/null

backup-restore-smoke:
	./scripts/backup-restore-smoke.sh

ci:
	$(MAKE) api-check
	$(MAKE) quality
	$(MAKE) security
	$(MAKE) test
	$(MAKE) build
	$(MAKE) migration-image-check
	$(MAKE) e2e

backup:
	SERVICE="$(or $(SERVICE),pymes)" ./scripts/backup-postgres.sh "$(FILE)"

restore:
	SERVICE="$(or $(SERVICE),pymes)" ./scripts/restore-postgres.sh "$(FILE)"
