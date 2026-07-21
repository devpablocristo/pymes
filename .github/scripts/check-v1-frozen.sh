#!/usr/bin/env bash
set -euo pipefail

expected_tree="5182664257d6b8311b743a17c13b9105df7f0ee6"
current_tree="$(git rev-parse HEAD:v1)"

if [[ "$current_tree" != "$expected_tree" ]]; then
  echo "v1 archive changed: expected $expected_tree, got $current_tree" >&2
  echo "v1 is immutable; create new work under v2/ or update docs outside v1/." >&2
  exit 1
fi

echo "v1 archive tree is frozen at $expected_tree"
