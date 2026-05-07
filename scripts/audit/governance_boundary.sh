#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

fail=false

check_zero() {
  local label="$1"
  local pattern="$2"
  shift 2
  local tmp
  tmp="$(mktemp)"
  if rg -n "$pattern" "$@" >"$tmp"; then
    echo "FAIL $label" >&2
    cat "$tmp" >&2
    fail=true
  else
    echo "OK $label"
  fi
  rm -f "$tmp"
}

check_zero \
  "no embedded governance packages" \
  'core/governance/go/(decision|policy|risk|approval|kernel)' \
  pymes-core workshops professionals restaurants beauty medical ai frontend scripts .github Makefile \
  --glob '!**/node_modules/**' --glob '!**/__pycache__/**'

check_zero \
  "no local governance engine/evaluator" \
  'decision\.Engine|policy\.Evaluator|risk\.Evaluate|approval\.RequirementFor' \
  pymes-core workshops professionals restaurants beauty medical \
  --glob '*.go'

check_zero \
  "no old Pymes policy list naming" \
  'ListPoliciesForOrg' \
  pymes-core/backend/internal/procurement \
  --glob '*.go'

tmp="$(mktemp)"
if rg -n 'WithOrgID|X-Org-ID|org_id == tenant_id' pymes-core scripts .github Makefile \
  --glob '!**/__pycache__/**' \
  --glob '!pymes-core/backend/internal/governanceproxy/client.go' \
  --glob '!pymes-core/backend/internal/governanceproxy/client_test.go' \
  --glob '!scripts/migrate_procurement_policies_to_nexus.sh' \
  --glob '!scripts/audit/governance_boundary.sh' >"$tmp"; then
  echo "FAIL Nexus tenant-scope wire naming leaked outside explicit adapters" >&2
  cat "$tmp" >&2
  fail=true
else
  echo "OK Nexus tenant-scope wire naming isolated"
fi
rm -f "$tmp"

if [[ "$fail" == true ]]; then
  exit 1
fi
