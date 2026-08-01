#!/usr/bin/env bash

# Shared Cloud Run release-tag policy. Keep the tag short because Cloud Run
# embeds it in the first DNS label of the tagged revision URL together with the
# service name and an implementation-defined suffix.

pymes_release_candidate_tag() {
  local release_sha="${1:-}"

  if [[ ! "$release_sha" =~ ^[0-9a-f]{40}$ ]]; then
    echo "release SHA must be exactly 40 lowercase hexadecimal characters" >&2
    return 2
  fi

  printf 'c-%s' "${release_sha:0:16}"
}

pymes_validate_release_candidate_tag() {
  local release_sha="${1:-}" candidate_tag="${2:-}" expected

  expected=$(pymes_release_candidate_tag "$release_sha") || return
  if [[ "$candidate_tag" != "$expected" ||
        ! "$candidate_tag" =~ ^c-[0-9a-f]{16}$ ]]; then
    echo "candidate tag must equal $expected" >&2
    return 2
  fi
}

pymes_validate_cloud_run_tagged_url() {
  local tagged_url="${1:-}" candidate_tag="${2:-}" service="${3:-}"
  local hostname label
  local -a labels

  if [[ ! "$service" =~ ^[a-z]([a-z0-9-]*[a-z0-9])?$ ]]; then
    echo "Cloud Run service name is not a lowercase DNS label: $service" >&2
    return 2
  fi
  if [[ ! "$candidate_tag" =~ ^c-[0-9a-f]{16}$ ]]; then
    echo "Cloud Run candidate tag is not in canonical c-<16 hex> form" >&2
    return 2
  fi
  if [[ ! "$tagged_url" =~ ^https://[^/@:\?\#]+$ ]]; then
    echo "Cloud Run tagged URL must be one HTTPS origin without credentials, port, path, query or fragment" >&2
    return 2
  fi

  hostname=${tagged_url#https://}
  if [[ "$hostname" != "$candidate_tag---$service".* &&
        "$hostname" != "$candidate_tag---$service"-* ]]; then
    echo "Cloud Run tagged URL does not identify tag $candidate_tag and service $service" >&2
    return 2
  fi
  if (( ${#hostname} > 253 )); then
    echo "Cloud Run tagged hostname exceeds the 253-character DNS limit" >&2
    return 2
  fi

  IFS='.' read -r -a labels <<<"$hostname"
  for label in "${labels[@]}"; do
    if (( ${#label} == 0 || ${#label} > 63 )) ||
       [[ ! "$label" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]]; then
      echo "Cloud Run tagged hostname contains an invalid DNS label: $label" >&2
      return 2
    fi
  done
}
