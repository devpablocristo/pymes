#!/usr/bin/env bash
set -euo pipefail

base_sha="${1:-}"
head_sha="${2:-HEAD}"

if [[ -z "${base_sha}" ]]; then
  echo "missing base revision for legacy immutability check" >&2
  exit 2
fi
if ! git rev-parse --verify --quiet "${head_sha}^{commit}" >/dev/null; then
  echo "invalid head revision: ${head_sha}" >&2
  exit 2
fi

# GitHub sends an all-zero before SHA for a newly created branch. Main is not
# expected to be created by this workflow, so fail closed if no real baseline
# can be established instead of silently accepting the complete legacy tree.
if [[ "${base_sha}" =~ ^0+$ ]] ||
  ! git rev-parse --verify --quiet "${base_sha}^{commit}" >/dev/null; then
  echo "cannot establish immutable legacy baseline: ${base_sha}" >&2
  exit 2
fi

mapfile -d '' changed_legacy_paths < <(
  git diff --no-renames --name-only -z \
    --diff-filter=ACDMRTUXB \
    "${base_sha}" "${head_sha}" -- v1 v2
)

if ((${#changed_legacy_paths[@]} > 0)); then
  echo "v1/ and v2/ are immutable historical references; detected changes:" >&2
  printf '  - %s\n' "${changed_legacy_paths[@]}" >&2
  echo "reconstruct required behavior under v3/ without changing legacy trees." >&2
  exit 1
fi

echo "v1/ and v2/ are unchanged between ${base_sha} and ${head_sha}"
