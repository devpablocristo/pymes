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

npx vite build --mode development
exec npx vite preview --host 127.0.0.1 --port 4173
