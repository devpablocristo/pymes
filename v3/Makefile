.DEFAULT_GOAL := help
.PHONY: help build migration-image-check test fiscal-test fiscal-real-contract accounting-test web-ci quality workflow-policy-check gcp-scope-check protected-live-validation-test release-evidence-test cloud-restore-drill-test legacy-wif-test project-secret-access-test obsolete-secrets-test compose-security-check libpq-exec-test pilot-evidence-test security fiscal-e2e accounting-e2e commercial-e2e scheduling-e2e notifications-e2e calendars-e2e observability-e2e recovery-e2e replay-smoke monitoring-config-check api-check api-generate architecture-check db-integration backup-restore-smoke e2e ci up down backup restore

ACCOUNTING_BUILD_CONTEXT ?= ../../open-accounting
PUBLIC_GO_OPERATION_IDS := getHealthz,getReadyz,createSale,createParty,getParty,createPurchase,getPurchase,createPayment,getPayment,createAccountingReversal,getAccountingFailure,createAccountingAdjustment,getSale,requestFiscalCredentialCSR,getFiscalCredential,uploadFiscalCertificate,configureFiscalPointOfSale,validateFiscalPointOfSale

help:
	@echo "Pymes v3"
	@echo "  make build           Construye API, worker, migraciones y adaptadores desplegables"
	@echo "  make migration-image-check  Construye las imágenes de migración Cloud Run"
	@echo "  make test            Ejecuta pruebas Go y del fiscal adapter mock"
	@echo "  make fiscal-test     Ejecuta typecheck y pruebas TypeScript fiscales"
	@echo "  make fiscal-real-contract  Prueba onboarding, KMS, WSAA y WSFE real con fakes"
	@echo "  make accounting-test Ejecuta las suites del runtime contable headless"
	@echo "  make web-ci          Valida cliente generado, tipos, tests, build, auditoría y E2E browser"
	@echo "  make quality         Ejecuta vet y valida Compose"
	@echo "  make workflow-policy-check  Valida Actions, release inmutable e IAM declarativo"
	@echo "  make gcp-scope-check Verifica el destino canónico de toda mutación GCP"
	@echo "  make protected-live-validation-test  Valida jobs Google/ARCA protegidos sin llamar proveedores"
	@echo "  make release-evidence-test  Prueba retención create-only y Bucket Lock sin llamar a GCP"
	@echo "  make cloud-restore-drill-test  Prueba el orquestador cloud de restore con adapters falsos"
	@echo "  make legacy-wif-test Prueba el corte WIF legado con adapters falsos"
	@echo "  make project-secret-access-test  Prueba reemplazo de grants Secret Manager a nivel proyecto"
	@echo "  make obsolete-secrets-test  Prueba el retiro recuperable de secretos obsoletos con gcloud falso"
	@echo "  make compose-security-check Verifica que Compose publique sólo en loopback"
	@echo "  make libpq-exec-test Verifica que URLs y passwords no lleguen a argv"
	@echo "  make pilot-evidence-test  Valida bundles redactados de pilotos sin contactar cloud"
	@echo "  make security        Verifica vulnerabilidades alcanzables en los tres runtimes"
	@echo "  make fiscal-e2e      Valida BFF y cliente Go contra Fiscal HTTP/PostgreSQL real con ARCA fake"
	@echo "  make accounting-e2e  Valida el cliente Go contra Open Accounting headless"
	@echo "  make commercial-e2e  Recorre la vertical pública completa contra servicios reales/mocks"
	@echo "  make scheduling-e2e  Valida Agenda HTTP/PostgreSQL, aislamiento y concurrencia"
	@echo "  make notifications-e2e  Valida PerGo, reintentos y webhooks firmados"
	@echo "  make calendars-e2e   Valida OAuth, cifrado, Google/Meet fake, RLS y recuperación"
	@echo "  make observability-e2e  Comprueba salud, readiness y métricas del stack"
	@echo "  make recovery-e2e   Comprueba caída y recuperación independiente de servicios y bases"
	@echo "  make replay-smoke   Comprueba replay DLQ idempotente y auditoría inmutable"
	@echo "  make monitoring-config-check  Valida Monitoring STG/PRD sin llamar a GCP"
	@echo "  make api-check       Valida los contratos OpenAPI YAML"
	@echo "  make api-generate    Regenera servidor/tipos/clientes desde OpenAPI"
	@echo "  make architecture-check  Valida estructura vertical, imports, composición y lifecycle Go"
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
	./fiscal-adapter/scripts/migrate-test.sh

fiscal-real-contract:
	cd fiscal-adapter && npm ci --ignore-scripts && npm run test:real

accounting-test:
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go test ./internal/accounting ./internal/tenant ./internal/pymesaccounting ./internal/pymesaccountingdbguard ./internal/database/pymesaccounting ./internal/database/pymesaccountingadmin ./internal/pymesaccountingadminhttp ./cmd/migrate ./cmd/pymes-accounting ./cmd/pymes-accounting-admin
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go test ./docs -run 'TestRuntimePersistenceStaysBehindRepositories|TestRepositoryRawSQLExceptionsAreExplicit'

web-ci:
	cd web && npm ci --ignore-scripts && npm run ci

quality:
	cd backend && go vet ./...
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go vet ./internal/accounting ./internal/tenant ./internal/pymesaccounting ./internal/pymesaccountingdbguard ./internal/database/pymesaccounting ./internal/database/pymesaccountingadmin ./internal/pymesaccountingadminhttp ./cmd/migrate ./cmd/pymes-accounting ./cmd/pymes-accounting-admin
	for script in scripts/*.sh; do \
		case "$$(head -n 1 "$$script")" in \
			*"bash") bash -n "$$script" ;; \
			*) sh -n "$$script" ;; \
		esac || exit 1; \
	done
	for script in scripts/deploy/*.sh; do bash -n "$$script" || exit 1; done
	$(MAKE) legacy-wif-test
	$(MAKE) project-secret-access-test
	$(MAKE) obsolete-secrets-test
	$(MAKE) compose-security-check
	$(MAKE) libpq-exec-test
	$(MAKE) pilot-evidence-test
	./scripts/deploy/cloud-run-security-check.sh
	./scripts/deploy/cloud-run-transaction-test.sh
	ACCOUNTING_BUILD_CONTEXT="$(ACCOUNTING_BUILD_CONTEXT)" ./scripts/deploy/rollback-cloud-run-test.sh
	$(MAKE) workflow-policy-check
	$(MAKE) gcp-scope-check
	$(MAKE) protected-live-validation-test
	$(MAKE) release-evidence-test
	$(MAKE) cloud-restore-drill-test
	$(MAKE) monitoring-config-check
	docker compose config --quiet

workflow-policy-check:
	./scripts/deploy/workflow-policy-check.sh

gcp-scope-check:
	./scripts/deploy/gcp-target-policy-test.sh

protected-live-validation-test:
	./scripts/deploy/protected-live-validation-test.sh

release-evidence-test:
	bash scripts/deploy/release-evidence-test.sh

cloud-restore-drill-test:
	bash scripts/deploy/cloud-restore-drill-test.sh

legacy-wif-test:
	bash scripts/deploy/retire-legacy-pymes-wif-test.sh

project-secret-access-test:
	bash scripts/deploy/migrate-project-secret-access-test.sh

obsolete-secrets-test:
	bash scripts/deploy/retire-obsolete-secrets-test.sh

compose-security-check:
	./scripts/compose-security-check.sh

libpq-exec-test:
	./scripts/libpq-exec-test.sh

pilot-evidence-test:
	./scripts/deploy/collect-pilot-evidence-test.sh

security:
	cd backend && go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
	cd "$(ACCOUNTING_BUILD_CONTEXT)" && go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./internal/accounting ./internal/tenant ./internal/pymesaccounting ./internal/pymesaccountingdbguard ./internal/database/pymesaccounting ./internal/database/pymesaccountingadmin ./internal/pymesaccountingadminhttp ./cmd/migrate ./cmd/pymes-accounting ./cmd/pymes-accounting-admin
	cd fiscal-adapter && npm audit --omit=dev

api-check:
	python3 -c 'import yaml; [yaml.safe_load(open(p)) for p in ["api/openapi.yaml", "api/scheduling.openapi.yaml", "contracts/accounting.openapi.yaml", "contracts/fiscal.openapi.yaml", "contracts/pergo.openapi.yaml", "monitoring/alerts.yml"]]'
	./scripts/contracts-check.sh

api-generate:
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --include-operation-ids "$(PUBLIC_GO_OPERATION_IDS)" --package publicapi --generate types,chi-server -o internal/commerce/handler/dto/public.gen.go ../api/openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package accountingapi --generate types,client -o internal/commerce/accounting/models/accounting.gen.go ../contracts/accounting.openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package accountingapi --generate chi-server -o internal/commerce/accounting/models/accounting.server.gen.go ../contracts/accounting.openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package fiscalapi --generate types,client -o internal/commerce/fiscal/models/fiscal.gen.go ../contracts/fiscal.openapi.yaml
	cd backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --package fiscalapi --generate chi-server -o internal/commerce/fiscal/models/fiscal.server.gen.go ../contracts/fiscal.openapi.yaml

architecture-check:
	bash scripts/check-foundation-migration-boundary-test.sh
	bash scripts/check-foundation-migration-boundary.sh ..
	cd backend && go test -count=1 ./internal/architecture

up:
	docker compose up --build --detach --wait

down:
	docker compose down

db-integration:
	./scripts/db-integration.sh

e2e:
	$(MAKE) db-integration
	$(MAKE) scheduling-e2e
	$(MAKE) commercial-e2e
	$(MAKE) notifications-e2e
	$(MAKE) calendars-e2e
	$(MAKE) backup-restore-smoke
	$(MAKE) observability-e2e
	$(MAKE) recovery-e2e
	$(MAKE) replay-smoke
	docker compose stop worker
	cd backend && PYMES_DATABASE_TEST_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable' go test ./internal/commerce -run 'TestDurableWorkerRecoversLostFiscalAndAccountingResponses' -count=1
	docker compose up -d --wait worker
	$(MAKE) fiscal-e2e
	$(MAKE) accounting-e2e

fiscal-e2e:
	./scripts/fiscal-contract-e2e.sh

accounting-e2e:
	./scripts/accounting-contract-e2e.sh

commercial-e2e:
	./scripts/commercial-vertical-e2e.sh

scheduling-e2e:
	./scripts/scheduling-e2e.sh

notifications-e2e:
	./scripts/notifications-e2e.sh

calendars-e2e:
	./scripts/calendars-e2e.sh

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
	$(MAKE) architecture-check
	$(MAKE) quality
	$(MAKE) security
	$(MAKE) test
	$(MAKE) fiscal-real-contract
	$(MAKE) web-ci
	$(MAKE) build
	$(MAKE) migration-image-check
	$(MAKE) e2e

backup:
	SERVICE="$(or $(SERVICE),pymes)" ./scripts/backup-postgres.sh "$(FILE)"

restore:
	SERVICE="$(or $(SERVICE),pymes)" ./scripts/restore-postgres.sh "$(FILE)"
