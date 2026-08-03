#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root_dir"
docker compose up -d --build --wait accounting accounting-admin

cd "$root_dir/backend"
PYMES_ACCOUNTING_TEST_URL='http://127.0.0.1:18082' \
PYMES_ACCOUNTING_PROVISIONING_TEST_URL='http://127.0.0.1:18087' \
PYMES_INTERNAL_SIGNING_SEED_B64='AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA=' \
PYMES_INTERNAL_ISSUER=pymes-v3 \
PYMES_INTERNAL_KEY_ID=local-dev-1 \
go test ./internal/commerce -run TestAccountingClientAgainstHeadlessService -count=1
