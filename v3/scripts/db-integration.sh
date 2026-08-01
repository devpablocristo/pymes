#!/usr/bin/env sh
set -eu
root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root_dir"
docker compose stop worker api fiscal-fake accounting accounting-admin
docker compose up -d --wait postgres fiscal-postgres accounting-postgres

# Immutable inbox/audit tables intentionally cannot be truncated. Recreate
# only the three disposable Docker databases so every integration run starts
# from authoritative empty state, then apply each migrator twice below to
# prove migration idempotence.
docker compose run --rm --no-deps -e PGPASSWORD=pymes postgres \
  psql -h postgres -U pymes -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS pymes_v3 WITH (FORCE)"
docker compose run --rm --no-deps -e PGPASSWORD=pymes postgres \
  psql -h postgres -U pymes -d postgres -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE pymes_v3 OWNER pymes"
docker compose run --rm --no-deps -e PGPASSWORD=fiscal fiscal-postgres \
  psql -h fiscal-postgres -U fiscal -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS pymes_fiscal WITH (FORCE)"
docker compose run --rm --no-deps -e PGPASSWORD=fiscal fiscal-postgres \
  psql -h fiscal-postgres -U fiscal -d postgres -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE pymes_fiscal OWNER fiscal"
docker compose run --rm --no-deps -e PGPASSWORD=accounting accounting-postgres \
  psql -h accounting-postgres -U accounting -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS pymes_accounting WITH (FORCE)"
docker compose run --rm --no-deps -e PGPASSWORD=accounting accounting-postgres \
  psql -h accounting-postgres -U accounting -d postgres -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE pymes_accounting OWNER accounting"

for file in db/migrations/*.sql; do
  docker compose exec -T postgres psql -U pymes -d pymes_v3 -v ON_ERROR_STOP=1 -f "/migrations/$(basename "$file")"
  docker compose exec -T postgres psql -U pymes -d pymes_v3 -v ON_ERROR_STOP=1 -f "/migrations/$(basename "$file")"
done
docker compose run --rm fiscal-migrate
docker compose run --rm fiscal-migrate
docker compose build accounting-migrate
docker compose run --rm accounting-roles
docker compose run --rm --no-deps accounting-migrate
docker compose run --rm --no-deps accounting-migrate
docker compose run --rm --no-deps accounting-grants
docker compose exec -T postgres psql -U pymes -d pymes_v3 -v ON_ERROR_STOP=1 -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_v3_rls_test') THEN CREATE ROLE pymes_v3_rls_test; END IF; END \$\$; GRANT USAGE ON SCHEMA app TO pymes_v3_rls_test; GRANT SELECT, INSERT ON app.sales, app.organizations, app.accounting_reversals TO pymes_v3_rls_test; INSERT INTO app.organizations(id,name,slug,status) VALUES ('org_a','A','a','ready'),('org_b','B','b','ready') ON CONFLICT (id) DO NOTHING; INSERT INTO app.sales(id,org_id,recipient_ref,point_of_sale,document_type,voucher_number,amount,currency,status,snapshot_digest,correlation_id,request_id,actor_ref,source_version) VALUES ('shared_sale','org_a','party',1,'FA',1,1,'ARS','posted',repeat('a',64),'test','request:rls:a','system:test',1),('shared_sale','org_b','party',1,'FA',1,1,'ARS','posted',repeat('b',64),'test','request:rls:b','system:test',1) ON CONFLICT (org_id,id) DO NOTHING; INSERT INTO app.accounting_reversals(id,org_id,document_kind,document_id,original_journal_entry_id,effective_at,reason,status,snapshot_digest,correlation_id,request_id,actor_ref,source_version) VALUES ('shared_reversal','org_a','purchase','purchase_a','journal_a',now(),'test','requested',repeat('a',64),'test','request:reversal:a','system:test',1),('shared_reversal','org_b','purchase','purchase_b','journal_b',now(),'test','requested',repeat('b',64),'test','request:reversal:b','system:test',1) ON CONFLICT (org_id,id) DO NOTHING; SET ROLE pymes_v3_rls_test; SET app.org_id = 'org_a'; DO \$\$ DECLARE sale_count bigint; reversal_count bigint; BEGIN SELECT count(*) INTO sale_count FROM app.sales; SELECT count(*) INTO reversal_count FROM app.accounting_reversals; IF sale_count <> 1 OR reversal_count <> 1 THEN RAISE EXCEPTION 'RLS isolation failed: sales=%, reversals=%', sale_count, reversal_count; END IF; END \$\$; RESET ROLE;"
cd backend
PYMES_DATABASE_TEST_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable' go test ./internal/commerce
PYMES_DATABASE_TEST_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable' go test ./internal/identity
PYMES_DATABASE_TEST_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable' go test ./internal/postgres
PYMES_DATABASE_TEST_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable' go test ./internal/organization
cd ../fiscal-adapter
FISCAL_DATABASE_TEST_URL='postgres://fiscal:fiscal@127.0.0.1:55435/pymes_fiscal?sslmode=disable' npm run test:postgres
accounting_dir=${ACCOUNTING_BUILD_CONTEXT:-../../open-accounting}
case "$accounting_dir" in
  /*) cd "$accounting_dir" ;;
  *) cd "$root_dir/$accounting_dir" ;;
esac
ACCOUNTING_DATABASE_TEST_URL='postgres://accounting:accounting@127.0.0.1:55436/pymes_accounting?sslmode=disable' \
  go test -race -tags=integration ./internal/pymesaccounting -count=1
