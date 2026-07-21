#!/usr/bin/env bash
set -euo pipefail

expected_tree="32981e1296c2f65019aead71b638fa81ab3a42ec"
current_tree="$(git rev-parse HEAD:v1)"

if [[ "$current_tree" != "$expected_tree" ]]; then
  echo "v1 archive changed: expected $expected_tree, got $current_tree" >&2
  echo "v1 is immutable; create new work under v2/ or update docs outside v1/." >&2
  exit 1
fi

echo "v1 archive tree is frozen at $expected_tree"
