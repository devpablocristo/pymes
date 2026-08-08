#!/usr/bin/env bash

set -euo pipefail

repo_root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
active_root="${repo_root}/v3"

if [[ ! -d "${active_root}" ]]; then
  echo "foundation migration boundary: active v3 tree not found under ${repo_root}" >&2
  exit 1
fi

declare -A allowed_legacy_files=()
while IFS= read -r path; do
  [[ -n "${path}" ]] && allowed_legacy_files["${path}"]=1
done <<'EOF'
v3/backend/go.mod
v3/backend/go.sum
v3/backend/internal/architecture/architecture_test.go
v3/backend/internal/calendars/google_calendar.go
v3/backend/internal/calendars/google_calendar/helpers/mapping.go
v3/backend/internal/identity/clerk.go
v3/backend/internal/identity/clerk_test.go
v3/backend/internal/identity/handler.go
v3/backend/internal/identity/handler_test.go
v3/backend/internal/identity/repository.go
v3/backend/internal/identity/repository_authorization_test.go
v3/backend/internal/observability/tracing.go
v3/backend/internal/postgres/platform_alignment_test.go
v3/backend/internal/postgres/postgres.go
v3/backend/internal/scheduling/architecture_test.go
v3/backend/internal/scheduling/platform_scheduling.go
v3/backend/internal/scheduling/platform_scheduling/helpers/mapping.go
v3/backend/internal/scheduling/platform_scheduling/models/types.go
v3/backend/wire/wire.go
v3/scripts/check-foundation-migration-boundary-test.sh
v3/scripts/check-foundation-migration-boundary.sh
v3/web/package-lock.json
v3/web/package.json
v3/web/scripts/check-dependency-policy.mjs
v3/web/src/components/CalendarBoard.test.tsx
v3/web/src/components/CalendarBoard.tsx
v3/web/src/pages/PublicBookingPage.tsx
EOF

legacy_pattern='github\.com/devpablocristo/platform/|@devpablocristo/platform-'
violations=0
references=0

while IFS= read -r file; do
  relative="${file#${repo_root}/}"
  while IFS= read -r match; do
    [[ -n "${match}" ]] || continue
    references=$((references + 1))
    if [[ -z "${allowed_legacy_files[${relative}]:-}" ]]; then
      echo "${relative}: new dependency on deprecated Platform is forbidden" >&2
      violations=$((violations + 1))
    fi
  done < <(grep -En "${legacy_pattern}" "${file}" 2>/dev/null || true)
done < <(
  find "${active_root}" -type f \
    -not -path '*/node_modules/*' \
    -not -path '*/dist/*' \
    -not -path '*/coverage/*' \
    -not -path '*/docs/*' \
    -print
)

local_dependency_pattern='(^|[[:space:]])replace[[:space:]].*=>[[:space:]]*(\.\.?/|/home/|/tmp/)|"(file:|link:|workspace:)|:[[:space:]]*"(file:|link:|workspace:)'
while IFS= read -r manifest; do
  relative="${manifest#${repo_root}/}"
  if grep -En "${local_dependency_pattern}" "${manifest}" >/dev/null 2>&1; then
    echo "${relative}: local dependency is forbidden; publish Foundation first" >&2
    violations=$((violations + 1))
  fi
done < <(
  find "${active_root}" -type f \
    \( -name 'go.mod' -o -name 'package.json' -o -name 'package-lock.json' -o -name 'pnpm-lock.yaml' \) \
    -not -path '*/node_modules/*' \
    -print
)

if (( violations > 0 )); then
  echo "foundation migration boundary: ${violations} violation(s)" >&2
  exit 1
fi

echo "foundation migration boundary: ${references} allowlisted legacy reference(s); active debt did not spread"
