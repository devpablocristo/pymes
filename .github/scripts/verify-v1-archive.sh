#!/usr/bin/env bash
set -euo pipefail

snapshot_ref="${1:-v1-final}"
archive_ref="${2:-HEAD}"

if ! git rev-parse --verify --quiet "${snapshot_ref}^{commit}" >/dev/null; then
  echo "missing snapshot ref: ${snapshot_ref}" >&2
  exit 1
fi

expected_count=0
while IFS=$'\t' read -r -d '' metadata archived_path; do
  expected_oid="${metadata##* }"
  if ! actual_oid="$(git rev-parse "${archive_ref}:v1/${archived_path}" 2>/dev/null)"; then
    echo "missing archived path: v1/${archived_path}" >&2
    exit 1
  fi
  if [[ "${actual_oid}" != "${expected_oid}" ]]; then
    echo "content drift: v1/${archived_path}" >&2
    exit 1
  fi
  expected_count=$((expected_count + 1))
done < <(git ls-tree -rz --full-tree "${snapshot_ref}")

if [[ "${expected_count}" -ne 1587 ]]; then
  echo "unexpected snapshot file count: ${expected_count}" >&2
  exit 1
fi

if git ls-files -z | grep -zEq '(^|/)(\.env|node_modules|LedgerSMB-master|arca-facturacion|pyafipws)(/|$)'; then
  echo "forbidden local artifact is tracked" >&2
  exit 1
fi

echo "verified ${expected_count} immutable v1 blobs against ${snapshot_ref}"
