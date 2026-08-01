#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
database_url=${PYMES_DATABASE_TEST_URL:-postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable}

cd "$root_dir"
docker compose up -d --wait postgres
docker compose run --rm migrate

cd backend
PYMES_DATABASE_TEST_URL="$database_url" \
  go test -race ./internal/calendars ./internal/calendars/worker/helpers \
  -count=1

