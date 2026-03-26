#!/usr/bin/env bash
# Build FE sin Clerk (ProtectedRoute deja pasar) y levanta preview para E2E Wowdash.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export VITE_CLERK_PUBLISHABLE_KEY=""
npm run build
exec npx vite preview --host 127.0.0.1 --port 4173 --strictPort
