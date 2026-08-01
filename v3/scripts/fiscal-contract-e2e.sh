#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
port=${PYMES_FISCAL_E2E_PORT:-$(node -e 'const net=require("node:net");const server=net.createServer();server.listen(0,"127.0.0.1",()=>{process.stdout.write(String(server.address().port));server.close();});')}
log_file=/tmp/pymes-v3-fiscal-e2e.log

cd "$root_dir/fiscal-adapter"
npm run build >/dev/null
cd "$root_dir"
docker compose up -d --wait fiscal-postgres
docker compose run --rm fiscal-migrate >/dev/null

cd "$root_dir/fiscal-adapter"
FISCAL_ADAPTER_MODE=mock \
FISCAL_MOCK_SCENARIO=authorized \
FISCAL_DATABASE_URL='postgres://fiscal:fiscal@127.0.0.1:55435/pymes_fiscal?sslmode=disable' \
PYMES_ENVIRONMENT=development \
PYMES_INTERNAL_ISSUER=pymes-v3 \
PYMES_INTERNAL_JWKS_JSON='{"keys":[{"kty":"OKP","crv":"Ed25519","alg":"EdDSA","use":"sig","key_ops":["verify"],"kid":"local-dev-1","x":"ebVWLo_mVPlAeLES6KmLp5AfhTrmlb7X4OORC60ElmQ"}]}' \
PORT="$port" node dist/src/cmd/api.js >"$log_file" 2>&1 &
server_pid=$!
cleanup() {
  kill "$server_pid" 2>/dev/null || true
  wait "$server_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

attempt=0
until curl -fsS "http://127.0.0.1:$port/readyz" >/dev/null 2>&1; do
  if ! kill -0 "$server_pid" 2>/dev/null; then
    sed -n '1,120p' "$log_file"
    exit 1
  fi
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 50 ]; then
    sed -n '1,120p' "$log_file"
    exit 1
  fi
  sleep 0.1
done

cd "$root_dir/backend"
PYMES_FISCAL_TEST_URL="http://127.0.0.1:$port" \
PYMES_INTERNAL_SIGNING_SEED_B64='AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA=' \
PYMES_INTERNAL_ISSUER=pymes-v3 \
PYMES_INTERNAL_KEY_ID=local-dev-1 \
go test ./internal/commerce -run TestFiscalClientAgainstMockAdapter -count=1
