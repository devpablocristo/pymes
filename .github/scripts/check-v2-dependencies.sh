#!/usr/bin/env bash
set -euo pipefail

workspace_file="$(find v2 -type d -name node_modules -prune -o -type f \( -name 'go.work' -o -name 'go.work.sum' \) -print -quit)"
if [[ -n "${workspace_file}" ]]; then
  echo "local Go workspace files are forbidden in v2: ${workspace_file}" >&2
  exit 1
fi

mapfile -d '' manifests < <(
  find v2 -type d -name node_modules -prune -o -type f \( \
    -name 'go.mod' -o \
    -name 'go.sum' -o \
    -name 'package.json' -o \
    -name 'package-lock.json' \
  \) -print0
)

if [[ "${#manifests[@]}" -eq 0 ]]; then
  echo "no v2 dependency manifests found" >&2
  exit 1
fi

go_mods=()
for manifest in "${manifests[@]}"; do
  if [[ "${manifest}" == *.mod ]]; then
    go_mods+=("${manifest}")
  fi
done

if [[ "${#go_mods[@]}" -gt 0 ]] && grep -nE '^[[:space:]]*replace([[:space:](]|$)' "${go_mods[@]}"; then
  echo "replace directives are forbidden in v2" >&2
  exit 1
fi

if grep -nE '(file:|link:|workspace:|/home/|[A-Za-z]:\\|:[[:space:]]*"(\.{1,2}/|/))' "${manifests[@]}"; then
  echo "local dependency references are forbidden in v2" >&2
  exit 1
fi

if grep -nE 'github\.com/devpablocristo/platform/[^[:space:]]+[[:space:]]+v0\.0\.0-' "${manifests[@]}"; then
  echo "platform pseudo-versions are forbidden in v2" >&2
  exit 1
fi

echo "verified ${#manifests[@]} v2 dependency manifests"
