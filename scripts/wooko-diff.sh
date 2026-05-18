#!/usr/bin/env bash
# wooko-diff.sh — comparar archivo por archivo la ui actual con
# el fork visual de Wooko (wooko.com/pymes/ui/) durante la migración UI.
#
# Uso:
#   scripts/wooko-diff.sh <ruta-relativa-a-ui/>
#
# Ejemplos:
#   scripts/wooko-diff.sh src/styles/tokens.css
#   scripts/wooko-diff.sh src/pages/OnboardingPage.tsx
#   scripts/wooko-diff.sh src/components/   # diff de directorio completo
#
# Variables:
#   DIFF_BIN   override del comando diff (default: diff -u o git --no-pager diff --no-index si está disponible)

set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "uso: scripts/wooko-diff.sh <ruta-relativa-a-ui/>" >&2
  echo "ej:  scripts/wooko-diff.sh src/styles/tokens.css" >&2
  exit 2
fi

# Resolvé root del repo (asume el script ubicado en scripts/).
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ACTUAL="$ROOT/ui/$1"
WOOKO="$ROOT/wooko.com/pymes/ui/$1"

if [ ! -e "$ACTUAL" ] && [ ! -e "$WOOKO" ]; then
  echo "ninguna versión existe en:" >&2
  echo "  $ACTUAL" >&2
  echo "  $WOOKO" >&2
  exit 1
fi

if [ ! -e "$ACTUAL" ]; then
  echo "[wooko-only] $1 — solo existe en wooko, no en actual" >&2
  echo "$WOOKO"
  exit 0
fi
if [ ! -e "$WOOKO" ]; then
  echo "[actual-only] $1 — solo existe en actual, no en wooko" >&2
  echo "$ACTUAL"
  exit 0
fi

# Preferí git diff si está disponible: mejor coloreado y context-aware.
if command -v git >/dev/null 2>&1; then
  exec git --no-pager diff --no-index --color=always -- "$ACTUAL" "$WOOKO" || true
fi

# Fallback a diff plano.
exec "${DIFF_BIN:-diff -u}" "$ACTUAL" "$WOOKO"
