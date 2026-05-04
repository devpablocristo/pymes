#!/usr/bin/env bash
# Construye y sirve el frontend para E2E. Sin Clerk, sin backend real.
set -euo pipefail
cd "$(dirname "$0")/.."

# Forzar modo sin Clerk para E2E
export VITE_CLERK_PUBLISHABLE_KEY=
export VITE_API_KEY=e2e-test-key
export VITE_API_ACTOR=e2e-admin
export VITE_API_ROLE=admin
export VITE_API_URL=http://127.0.0.1:9999
export E2E_OUT_DIR="${E2E_OUT_DIR:-/tmp/pymes-frontend-e2e-dist}"
E2E_PREVIEW_PORT="${E2E_PREVIEW_PORT:-4173}"

npx vite build --mode development --outDir "$E2E_OUT_DIR"
exec npx vite preview --host 127.0.0.1 --port "$E2E_PREVIEW_PORT" --outDir "$E2E_OUT_DIR"
