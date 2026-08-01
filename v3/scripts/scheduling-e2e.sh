#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root_dir"

for command in docker go; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "$command is required" >&2
    exit 1
  }
done

docker compose up -d --wait postgres
for file in db/migrations/*.sql; do
  docker compose exec -T postgres \
    psql -U pymes -d pymes_v3 -v ON_ERROR_STOP=1 \
    -f "/migrations/$(basename "$file")" >/dev/null
done

cd backend
PYMES_DATABASE_TEST_URL=${PYMES_DATABASE_TEST_URL:-postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable}
export PYMES_DATABASE_TEST_URL
go test -race ./internal/scheduling \
  -run 'Test(PostgresSchedulingTenantIsolationConcurrencyAndRecovery|SchedulingHTTPEndToEnd|BookingStatusCustomizationPreservesTenantAndLifecycleInvariants)$' \
  -count=1
