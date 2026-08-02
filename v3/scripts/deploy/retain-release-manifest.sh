#!/usr/bin/env bash
set -euo pipefail
umask 077

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=release-evidence-lib.sh
source "$script_dir/release-evidence-lib.sh"

retain_release_manifest_validate_manifest() {
  local manifest="$1" environment="$2" source_sha="$3" accounting_sha="$4"
  local registry key value
  local -A expected=() seen=()
  registry="${PYMES_RELEASE_EVIDENCE_REGION}-docker.pkg.dev/${PYMES_RELEASE_EVIDENCE_PROJECT}/pymes"
  expected=(
    [PYMES_RELEASE_ENV]="$environment"
    [PYMES_SOURCE_SHA]="$source_sha"
    [PYMES_OPEN_ACCOUNTING_SOURCE_SHA]="$accounting_sha"
    [PYMES_API_IMAGE]=pymes-v3-api
    [PYMES_WEB_IMAGE]="pymes-v3-web-${environment}"
    [PYMES_WORKER_IMAGE]=pymes-v3-worker
    [PYMES_FISCAL_IMAGE]=pymes-v3-fiscal
    [PYMES_ACCOUNTING_IMAGE]=pymes-v3-accounting
    [PYMES_ACCOUNTING_ADMIN_IMAGE]=pymes-v3-accounting-admin
    [PYMES_PROVISION_IMAGE]=pymes-v3-provision
    [PYMES_MIGRATE_IMAGE]=pymes-v3-migrate
    [PYMES_FISCAL_MIGRATE_IMAGE]=pymes-v3-fiscal-migrate
    [PYMES_ACCOUNTING_MIGRATE_IMAGE]=pymes-v3-accounting-migrate
  )

  while IFS='=' read -r key value; do
    [[ -n "$key" && -n "$value" && -z "${seen[$key]:-}" ]] || {
      pymes_release_evidence_fail \
        "manifest contains an empty or duplicate entry"
      return 1
    }
    [[ -n "${expected[$key]+present}" ]] || {
      pymes_release_evidence_fail \
        "manifest contains non-allowlisted key $key"
      return 1
    }
    case "$key" in
      PYMES_RELEASE_ENV|PYMES_SOURCE_SHA|PYMES_OPEN_ACCOUNTING_SOURCE_SHA)
        [[ "$value" == "${expected[$key]}" ]] || {
          pymes_release_evidence_fail \
            "manifest $key differs from the reviewed release"
          return 1
        }
        ;;
      *)
        [[ "$value" =~ ^${registry}/${expected[$key]}@sha256:[0-9a-f]{64}$ ]] || {
          pymes_release_evidence_fail \
            "manifest $key is not a canonical immutable image"
          return 1
        }
        ;;
    esac
    seen["$key"]=1
  done <"$manifest"
  for key in "${!expected[@]}"; do
    [[ -n "${seen[$key]:-}" ]] || {
      pymes_release_evidence_fail "manifest is missing $key"
      return 1
    }
  done
  [[ "${#seen[@]}" -eq "${#expected[@]}" ]] || {
    pymes_release_evidence_fail "manifest has an unexpected shape"
    return 1
  }
}

retain_release_manifest_main() {
  local environment source_sha accounting_sha run_id run_attempt manifest receipt
  local bucket bucket_uri object_name object_uri bucket_json policy_json
  local checksum object_json generation object_size retained_until downloaded
  environment=${PYMES_RELEASE_EVIDENCE_ENV:-}
  source_sha=${PYMES_RELEASE_EVIDENCE_SOURCE_SHA:-}
  accounting_sha=${PYMES_RELEASE_EVIDENCE_ACCOUNTING_SHA:-}
  run_id=${PYMES_RELEASE_EVIDENCE_RUN_ID:-}
  run_attempt=${PYMES_RELEASE_EVIDENCE_RUN_ATTEMPT:-}
  manifest=${PYMES_RELEASE_EVIDENCE_MANIFEST:-}
  receipt=${PYMES_RELEASE_EVIDENCE_RECEIPT:-}

  pymes_release_evidence_validate_environment "$environment" || return
  [[ "$source_sha" =~ ^[0-9a-f]{40}$ ]] || {
    echo "PYMES_RELEASE_EVIDENCE_SOURCE_SHA must be a full lowercase Git SHA" >&2
    return 2
  }
  [[ "$accounting_sha" =~ ^[0-9a-f]{40}$ ]] || {
    echo "PYMES_RELEASE_EVIDENCE_ACCOUNTING_SHA must be a full lowercase Git SHA" >&2
    return 2
  }
  [[ "$run_id" =~ ^[1-9][0-9]*$ ]] || {
    echo "PYMES_RELEASE_EVIDENCE_RUN_ID must be a positive GitHub run ID" >&2
    return 2
  }
  [[ "$run_attempt" == 1 ]] || {
    echo "PYMES_RELEASE_EVIDENCE_RUN_ATTEMPT must be exactly 1" >&2
    return 2
  }
  for path in "$manifest" "$receipt"; do
    [[ "$path" == /* ]] || {
      echo "manifest and receipt paths must be absolute" >&2
      return 2
    }
  done
  [[ -f "$manifest" && ! -L "$manifest" ]] || {
    echo "release manifest must be one regular non-symlink file" >&2
    return 2
  }
  [[ ! -e "$receipt" ]] || {
    echo "refusing to overwrite release evidence receipt: $receipt" >&2
    return 2
  }
  for required in gcloud jq mktemp sha256sum stat; do
    command -v "$required" >/dev/null 2>&1 || {
      echo "$required is required" >&2
      return 1
    }
  done
  retain_release_manifest_validate_manifest \
    "$manifest" "$environment" "$source_sha" "$accounting_sha" || return
  checksum=$(sha256sum -- "$manifest" | awk '{print $1}')
  [[ "$checksum" =~ ^[0-9a-f]{64}$ ]]
  object_size=$(stat -c '%s' "$manifest")
  [[ "$object_size" =~ ^[1-9][0-9]*$ ]]
  [[ "$(gcloud config get-value account 2>/dev/null)" == \
      "$PYMES_RELEASE_EVIDENCE_BUILDER" ]] || {
    echo "release evidence must be published by the dedicated builder" >&2
    return 1
  }

  bucket=$(pymes_release_evidence_bucket_name "$environment") || return
  bucket_uri="gs://${bucket}"
  bucket_json=$(gcloud storage buckets describe "$bucket_uri" --format=json) ||
    return
  policy_json=$(gcloud storage buckets get-iam-policy "$bucket_uri" \
    --format=json) || return
  pymes_release_evidence_validate_bucket \
    "$bucket_json" "$environment" true || return
  pymes_release_evidence_validate_bucket_iam "$policy_json" || return

  object_name="releases/${environment}/${source_sha}/github-run-${run_id}/pymes-v3-images.env"
  object_uri="${bucket_uri}/${object_name}"
  gcloud storage cp "$manifest" "$object_uri" \
    --if-generation-match=0 \
    --content-type=text/plain \
    --custom-metadata="environment=${environment},source_sha=${source_sha},open_accounting_sha=${accounting_sha},manifest_sha256=${checksum},github_run_id=${run_id},github_run_attempt=${run_attempt}" \
    >/dev/null || return

  object_json=$(gcloud storage objects describe "$object_uri" --format=json) ||
    return
  jq -e \
    --arg bucket "$bucket" \
    --arg name "$object_name" \
    --arg environment "$environment" \
    --arg source_sha "$source_sha" \
    --arg accounting_sha "$accounting_sha" \
    --arg checksum "$checksum" \
    --arg run_id "$run_id" \
    --arg run_attempt "$run_attempt" \
    --arg object_size "$object_size" '
      .bucket == $bucket and
      .name == $name and
      (.generation | tostring | test("^[1-9][0-9]*$")) and
      (.size | tostring) == $object_size and
      .contentType == "text/plain" and
      .metadata.environment == $environment and
      .metadata.source_sha == $source_sha and
      .metadata.open_accounting_sha == $accounting_sha and
      .metadata.manifest_sha256 == $checksum and
      .metadata.github_run_id == $run_id and
      .metadata.github_run_attempt == $run_attempt and
      (.retentionExpirationTime | type == "string" and length > 0)
    ' <<<"$object_json" >/dev/null || {
      pymes_release_evidence_fail \
        "retained object metadata differs from the release"
      return 1
    }

  generation=$(jq -r '.generation' <<<"$object_json")
  retained_until=$(jq -r '.retentionExpirationTime' <<<"$object_json")
  downloaded=$(mktemp)
  trap 'rm -f -- "$downloaded"' RETURN
  gcloud storage cp "${object_uri}#${generation}" "$downloaded" >/dev/null ||
    return
  [[ "$(sha256sum -- "$downloaded" | awk '{print $1}')" == "$checksum" ]] || {
    pymes_release_evidence_fail \
      "downloaded immutable manifest checksum differs"
    return 1
  }

  jq -n \
    --arg schema pymes-v3-release-evidence-v1 \
    --arg environment "$environment" \
    --arg source_sha "$source_sha" \
    --arg accounting_sha "$accounting_sha" \
    --arg manifest_sha256 "$checksum" \
    --arg object_uri "$object_uri" \
    --arg generation "$generation" \
    --arg retained_until "$retained_until" \
    --arg github_run_id "$run_id" \
    '{
      schema: $schema,
      environment: $environment,
      source_sha: $source_sha,
      open_accounting_sha: $accounting_sha,
      manifest_sha256: $manifest_sha256,
      object_uri: $object_uri,
      generation: $generation,
      retained_until: $retained_until,
      github_run_id: $github_run_id
    }' >"$receipt"
  chmod 600 "$receipt"
  rm -f -- "$downloaded"
  trap - RETURN

  printf 'RELEASE EVIDENCE RETAINED environment=%s source_sha=%s uri=%s generation=%s sha256=%s retained_until=%s\n' \
    "$environment" "$source_sha" "$object_uri" "$generation" "$checksum" \
    "$retained_until"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  retain_release_manifest_main "$@"
fi
