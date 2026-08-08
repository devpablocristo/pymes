#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
guard="${script_dir}/check-foundation-migration-boundary.sh"
fixture="$(mktemp -d)"
trap 'rm -rf "${fixture}"' EXIT

mkdir -p \
  "${fixture}/v1" \
  "${fixture}/v3/backend/internal/identity" \
  "${fixture}/v3/backend/internal/commerce" \
  "${fixture}/v3/web/src"

printf '%s\n' 'package identity' 'import _ "github.com/devpablocristo/platform/sdks/clerk/go"' \
  >"${fixture}/v3/backend/internal/identity/clerk.go"
printf '%s\n' 'package frozen' 'import _ "github.com/devpablocristo/platform/errors/go"' \
  >"${fixture}/v1/frozen.go"
printf '%s\n' 'package commerce' 'import _ "github.com/devpablocristo/foundation/platform/errors/go"' \
  >"${fixture}/v3/backend/internal/commerce/foundation.go"
printf '%s\n' '{"name":"fixture","private":true}' >"${fixture}/v3/web/package.json"

bash "${guard}" "${fixture}" >/dev/null

printf '%s\n' 'package commerce' 'import _ "github.com/devpablocristo/platform/errors/go"' \
  >"${fixture}/v3/backend/internal/commerce/leak.go"
if bash "${guard}" "${fixture}" >"${fixture}/stdout" 2>"${fixture}/stderr"; then
  echo "fixture with a new backend Platform import passed" >&2
  exit 1
fi
grep -Fq 'new dependency on deprecated Platform is forbidden' "${fixture}/stderr"
rm "${fixture}/v3/backend/internal/commerce/leak.go"

printf '%s\n' 'import "@devpablocristo/platform-browser";' \
  >"${fixture}/v3/web/src/leak.ts"
if bash "${guard}" "${fixture}" >"${fixture}/stdout" 2>"${fixture}/stderr"; then
  echo "fixture with a new frontend Platform import passed" >&2
  exit 1
fi
grep -Fq 'new dependency on deprecated Platform is forbidden' "${fixture}/stderr"
rm "${fixture}/v3/web/src/leak.ts"

printf '%s\n' 'module example.test/app' 'go 1.26.5' \
  'replace example.test/shared => ../../foundation' >"${fixture}/v3/backend/go.mod"
if bash "${guard}" "${fixture}" >"${fixture}/stdout" 2>"${fixture}/stderr"; then
  echo "fixture with a local dependency passed" >&2
  exit 1
fi
grep -Fq 'local dependency is forbidden' "${fixture}/stderr"

echo "foundation migration boundary fixtures: ok"
