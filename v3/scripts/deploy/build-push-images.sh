#!/usr/bin/env bash
set -euo pipefail

# Builds every deployable Pymes v3 image from reviewed, clean source trees and
# writes a strict KEY=VALUE manifest containing digest-only image references.

operation=${1:-build}
case "$operation" in
  build|verify-source-pins|verify-attestations) ;;
  *) echo "usage: $0 [build|verify-source-pins|verify-attestations]" >&2; exit 2 ;;
esac

: "${PYMES_RELEASE_ENV:?set PYMES_RELEASE_ENV to stg or prd}"
case "$PYMES_RELEASE_ENV" in
  stg|prd) ;;
  *) echo "PYMES_RELEASE_ENV must be stg or prd" >&2; exit 2 ;;
esac

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
repository=${PYMES_ARTIFACT_REPOSITORY:-pymes}
source_sha=${PYMES_SOURCE_SHA:-${GITHUB_SHA:-}}
accounting_context=${OPEN_ACCOUNTING_CONTEXT:-.deps/open-accounting}
pymes_dockerfile=v3/Dockerfile
accounting_dockerfile="${accounting_context}/Dockerfile"
manifest_file=${PYMES_IMAGE_MANIFEST:-${RUNNER_TEMP:-/tmp}/pymes-v3-images.env}
if [[ "$operation" == "build" ]]; then
  : "${VITE_CLERK_PUBLISHABLE_KEY:?set the environment-specific Clerk publishable key}"
  case "$PYMES_RELEASE_ENV" in
    stg) clerk_key_pattern='^pk_test_[A-Za-z0-9_-]+$' ;;
    prd) clerk_key_pattern='^pk_live_[A-Za-z0-9_-]+$' ;;
  esac
  if [[ ! "$VITE_CLERK_PUBLISHABLE_KEY" =~ $clerk_key_pattern ]]; then
    echo "Clerk publishable key does not match PYMES_RELEASE_ENV" >&2
    exit 2
  fi
fi

sha_pattern='^[0-9a-f]{40}$'
name_pattern='^[a-z][a-z0-9-]{0,62}$'
if [[ ! "$project" =~ $name_pattern ]]; then
  echo "PYMES_GCP_PROJECT is not a valid project ID" >&2
  exit 2
fi
if [[ ! "$region" =~ ^[a-z]+-[a-z]+[0-9]+$ ]]; then
  echo "PYMES_GCP_REGION is invalid" >&2
  exit 2
fi
if [[ ! "$repository" =~ $name_pattern ]]; then
  echo "PYMES_ARTIFACT_REPOSITORY is invalid" >&2
  exit 2
fi
if [[ ! "$source_sha" =~ $sha_pattern ]]; then
  echo "PYMES_SOURCE_SHA must be one full lowercase commit SHA" >&2
  exit 2
fi
if [[ "$source_sha" != "$(git rev-parse HEAD)" ]]; then
  echo "PYMES_SOURCE_SHA does not match the checked-out Pymes commit" >&2
  exit 1
fi
if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
  echo "Pymes worktree must be clean before building release images" >&2
  git status --short >&2
  exit 1
fi
pymes_repository=devpablocristo/pymes
pymes_origin=$(git remote get-url origin)
case "$pymes_origin" in
  "https://github.com/${pymes_repository}"|"https://github.com/${pymes_repository}.git"|"git@github.com:${pymes_repository}.git") ;;
  *)
    echo "Pymes checkout origin does not match the release repository" >&2
    exit 1
    ;;
esac

pin_file=.github/dependencies/open-accounting.env
if [[ ! -f "$pin_file" ]]; then
  echo "missing Open Accounting pin: $pin_file" >&2
  exit 1
fi
accounting_repository=$(sed -n 's/^OPEN_ACCOUNTING_REPOSITORY=//p' "$pin_file")
accounting_repository_id=$(sed -n 's/^OPEN_ACCOUNTING_REPOSITORY_ID=//p' "$pin_file")
accounting_sha=$(sed -n 's/^OPEN_ACCOUNTING_REF=//p' "$pin_file")
if [[ $(grep -c '^OPEN_ACCOUNTING_REPOSITORY=' "$pin_file") -ne 1 ||
      $(grep -c '^OPEN_ACCOUNTING_REPOSITORY_ID=' "$pin_file") -ne 1 ||
      $(grep -c '^OPEN_ACCOUNTING_REF=' "$pin_file") -ne 1 ||
      ! "$accounting_sha" =~ $sha_pattern ]]; then
  echo "Open Accounting pin must contain one repository identity and one full commit SHA" >&2
  exit 1
fi
if [[ "$accounting_repository" != "devpablocristo/open-accounting" ||
      "$accounting_repository_id" != "1317775856" ]]; then
  echo "Open Accounting repository identity differs from the reviewed fork" >&2
  exit 1
fi
if [[ ! -d "$accounting_context/.git" ]]; then
  echo "Open Accounting must be checked out at $accounting_context" >&2
  exit 1
fi
if [[ "$(git -C "$accounting_context" rev-parse HEAD)" != "$accounting_sha" ]]; then
  echo "Open Accounting checkout does not match OPEN_ACCOUNTING_REF" >&2
  exit 1
fi
if [[ -n "$(git -C "$accounting_context" status --porcelain=v1 --untracked-files=all)" ]]; then
  echo "Open Accounting worktree must be clean before building release images" >&2
  git -C "$accounting_context" status --short >&2
  exit 1
fi
accounting_origin=$(git -C "$accounting_context" remote get-url origin)
case "$accounting_origin" in
  "https://github.com/${accounting_repository}"|"https://github.com/${accounting_repository}.git"|"git@github.com:${accounting_repository}.git") ;;
  *)
    echo "Open Accounting checkout origin does not match the pinned repository" >&2
    exit 1
    ;;
esac

if [[ "$operation" == "build" ]]; then
  for value in "$VITE_CLERK_PUBLISHABLE_KEY"; do
    if [[ "$value" == *$'\n'* || "$value" == *$'\r'* ]]; then
      echo "Web build arguments must be single-line values" >&2
      exit 2
    fi
  done
fi

verify_dockerfile_base_pins() {
  local dockerfile="$1" line keyword reference alias_keyword alias
  declare -A stages=()
  [[ -f "$dockerfile" ]] || {
    echo "missing release Dockerfile: $dockerfile" >&2
    return 1
  }
  while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*[Ff][Rr][Oo][Mm][[:space:]]+ ]] || continue
    read -r keyword reference alias_keyword alias <<<"$line"
    [[ "$reference" != --platform=* ]] || {
      echo "release Dockerfiles must not hide platform selection in FROM: $dockerfile" >&2
      return 1
    }
    if [[ -z "${stages[$reference]:-}" &&
          ! "$reference" =~ @sha256:[0-9a-f]{64}$ ]]; then
      echo "external Docker base is not pinned by digest in $dockerfile: $reference" >&2
      return 1
    fi
    if [[ "${alias_keyword,,}" == "as" ]]; then
      [[ "$alias" =~ ^[A-Za-z0-9_.-]+$ ]] || {
        echo "invalid Docker build stage alias in $dockerfile" >&2
        return 1
      }
      stages["$alias"]=1
    fi
  done <"$dockerfile"
}

verify_dockerfile_base_pins "$pymes_dockerfile"
verify_dockerfile_base_pins "$accounting_dockerfile"

if [[ "$operation" == "verify-source-pins" ]]; then
  echo "Pymes and Open Accounting sources and Docker base digests are immutable"
  exit 0
fi

pinned_digest_for_base() {
  local dockerfile="$1" needle="$2" line keyword reference remainder digest
  declare -A matches=()
  while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*[Ff][Rr][Oo][Mm][[:space:]]+ ]] || continue
    read -r keyword reference remainder <<<"$line"
    [[ "$reference" == *"$needle"* ]] || continue
    digest=${reference##*@}
    [[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]] || {
      echo "cannot resolve immutable digest for $needle in $dockerfile" >&2
      return 1
    }
    matches["$digest"]=1
  done <"$dockerfile"
  [[ "${#matches[@]}" -eq 1 ]] || {
    echo "expected one immutable digest for $needle in $dockerfile" >&2
    return 1
  }
  printf '%s\n' "${!matches[@]}"
}

registry="${region}-docker.pkg.dev/${project}/${repository}"
pymes_source="https://github.com/${pymes_repository}"
accounting_source="https://github.com/${accounting_repository}"
temporary_directory=$(mktemp -d)
cleanup() {
  rm -rf -- "$temporary_directory"
}
trap cleanup EXIT

declare -A release_images=()

material_expectations_for_image() {
  case "$1" in
    PYMES_API_IMAGE|PYMES_WORKER_IMAGE|PYMES_PROVISION_IMAGE|PYMES_MIGRATE_IMAGE)
      printf '%s|%s\n' \
        'pkg:docker/golang@' "$(pinned_digest_for_base "$pymes_dockerfile" 'golang:')" \
        'distroless/static-debian12@' "$(pinned_digest_for_base "$pymes_dockerfile" 'distroless/static-debian12:')"
      ;;
    PYMES_WEB_IMAGE)
      printf '%s|%s\n' \
        'pkg:docker/node@' "$(pinned_digest_for_base "$pymes_dockerfile" 'node:')" \
        'nginxinc/nginx-unprivileged@' "$(pinned_digest_for_base "$pymes_dockerfile" 'nginxinc/nginx-unprivileged:')"
      ;;
    PYMES_FISCAL_IMAGE)
      printf '%s|%s\n' \
        'pkg:docker/node@' "$(pinned_digest_for_base "$pymes_dockerfile" 'node:')"
      ;;
    PYMES_FISCAL_MIGRATE_IMAGE)
      printf '%s|%s\n' \
        'pkg:docker/postgres@' "$(pinned_digest_for_base "$pymes_dockerfile" 'postgres:')"
      ;;
    PYMES_ACCOUNTING_IMAGE|PYMES_ACCOUNTING_ADMIN_IMAGE|PYMES_ACCOUNTING_MIGRATE_IMAGE)
      printf '%s|%s\n' \
        'pkg:docker/golang@' "$(pinned_digest_for_base "$accounting_dockerfile" 'golang:')" \
        'pkg:docker/alpine@' "$(pinned_digest_for_base "$accounting_dockerfile" 'alpine:')"
      ;;
    *)
      echo "unknown release image variable: $1" >&2
      return 2
      ;;
  esac
}

verify_image_attestations() {
  local variable_name="$1" image_ref="$2" revision="$3" source="$4"
  local expected_digest image_metadata provenance sbom pattern material_digest
  expected_digest=${image_ref##*@}
  image_metadata="${temporary_directory}/${variable_name}.image.json"
  provenance="${temporary_directory}/${variable_name}.provenance.json"
  sbom="${temporary_directory}/${variable_name}.sbom.json"

  [[ "$image_ref" =~ @sha256:[0-9a-f]{64}$ ]] || {
    echo "$variable_name is not an immutable image reference" >&2
    return 1
  }
  docker buildx imagetools inspect "$image_ref" --format '{{json .}}' >"$image_metadata"
  docker buildx imagetools inspect "$image_ref" --format '{{json .Provenance}}' >"$provenance"
  docker buildx imagetools inspect "$image_ref" --format '{{json .SBOM}}' >"$sbom"

  jq -e \
    --arg digest "$expected_digest" \
    --arg revision "$revision" \
    --arg source "$source" \
    --arg environment "$PYMES_RELEASE_ENV" \
    --arg pymes_sha "$source_sha" \
      --arg accounting_sha "$accounting_sha" \
      --arg pymes_repository_id "1173650578" \
      --arg accounting_repository_id "$accounting_repository_id" '
      (
        .image.config.Labels //
        .image.config.labels //
        .image["linux/amd64"].config.Labels //
        .image["linux/amd64"].config.labels
      ) as $labels |
      (
        .manifest.digest == $digest and
        $labels["org.opencontainers.image.revision"] == $revision and
        $labels["org.opencontainers.image.source"] == $source and
        $labels["org.opencontainers.image.version"] == $revision and
        $labels["io.pymes.release.environment"] == $environment and
        $labels["io.pymes.release.pymes-revision"] == $pymes_sha and
        $labels["io.pymes.release.open-accounting-revision"] == $accounting_sha and
        $labels["io.pymes.release.pymes-repository-id"] == $pymes_repository_id and
        $labels["io.pymes.release.open-accounting-repository-id"] == $accounting_repository_id
      )
    ' "$image_metadata" >/dev/null || {
      echo "$variable_name metadata is not bound to the reviewed release inputs" >&2
      return 1
    }

  jq -e \
    --arg pymes_sha "$source_sha" \
    --arg accounting_sha "$accounting_sha" \
    --arg pymes_repository_id "1173650578" \
    --arg accounting_repository_id "$accounting_repository_id" '
      (.SLSA // .["linux/amd64"].SLSA) as $slsa |
      $slsa.buildType == "https://mobyproject.org/buildkit@v1" and
      ($slsa.materials | type == "array" and length > 0) and
      all($slsa.materials[];
        (.uri | type == "string" and length > 0) and
        (.digest.sha256 | type == "string" and test("^[0-9a-f]{64}$"))
      ) and
      ([.. | strings] | index($pymes_sha) != null) and
      ([.. | strings] | index($accounting_sha) != null) and
      ([.. | strings] | index($pymes_repository_id) != null) and
      ([.. | strings] | index($accounting_repository_id) != null)
    ' "$provenance" >/dev/null || {
      echo "$variable_name lacks complete BuildKit provenance for the reviewed release inputs" >&2
      return 1
    }

  while IFS='|' read -r pattern material_digest; do
    jq -e --arg pattern "$pattern" --arg material_digest "$material_digest" '
      (.SLSA // .["linux/amd64"].SLSA) as $slsa |
      any($slsa.materials[];
        (.uri | contains($pattern)) and
        (.uri | contains("digest=" + $material_digest))
      )
    ' "$provenance" >/dev/null || {
      echo "$variable_name provenance omits pinned base material $pattern$material_digest" >&2
      return 1
    }
  done < <(material_expectations_for_image "$variable_name")

  jq -e '
    (.SPDX // .["linux/amd64"].SPDX) as $spdx |
    $spdx.SPDXID == "SPDXRef-DOCUMENT" and
    ($spdx.spdxVersion | type == "string" and startswith("SPDX-")) and
    ($spdx.documentNamespace | type == "string" and length > 0) and
    ($spdx.packages | type == "array")
  ' "$sbom" >/dev/null || {
    echo "$variable_name lacks a valid SPDX SBOM attestation" >&2
    return 1
  }
}

build_image() {
  local variable_name="$1" image_name="$2" target="$3" context="$4" revision="$5" source="$6"
  shift 6
  local tag="${registry}/${image_name}:${revision}" metadata_file digest
  metadata_file="${temporary_directory}/${image_name}.json"

  docker buildx build \
    --platform=linux/amd64 \
    --target="$target" \
    --tag="$tag" \
    --label="org.opencontainers.image.revision=${revision}" \
    --label="org.opencontainers.image.source=${source}" \
    --label="org.opencontainers.image.version=${revision}" \
    --label="io.pymes.release.environment=${PYMES_RELEASE_ENV}" \
    --label="io.pymes.release.pymes-revision=${source_sha}" \
    --label="io.pymes.release.open-accounting-revision=${accounting_sha}" \
    --label="io.pymes.release.pymes-repository-id=1173650578" \
    --label="io.pymes.release.open-accounting-repository-id=${accounting_repository_id}" \
    --provenance=mode=max \
    --sbom=true \
    --push \
    --metadata-file="$metadata_file" \
    "$@" \
    "$context"

  digest=$(jq -er '."containerimage.digest"' "$metadata_file")
  if [[ ! "$digest" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    echo "buildx did not return an immutable digest for $image_name" >&2
    exit 1
  fi
  release_images["$variable_name"]="${registry}/${image_name}@${digest}"
  verify_image_attestations \
    "$variable_name" \
    "${release_images[$variable_name]}" \
    "$revision" \
    "$source"
}

if [[ "$operation" == "verify-attestations" ]]; then
  while IFS='|' read -r variable_name image_name revision source; do
    image_ref=${!variable_name:-}
    expected_prefix="${registry}/${image_name}@"
    image_digest=${image_ref#"$expected_prefix"}
    [[ "$image_ref" == "$expected_prefix$image_digest" &&
       "$image_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || {
      echo "$variable_name does not match the expected immutable Artifact Registry reference" >&2
      exit 1
    }
    verify_image_attestations "$variable_name" "$image_ref" "$revision" "$source"
  done <<EOF
PYMES_API_IMAGE|pymes-v3-api|${source_sha}|${pymes_source}
PYMES_WEB_IMAGE|pymes-v3-web-${PYMES_RELEASE_ENV}|${source_sha}|${pymes_source}
PYMES_WORKER_IMAGE|pymes-v3-worker|${source_sha}|${pymes_source}
PYMES_FISCAL_IMAGE|pymes-v3-fiscal|${source_sha}|${pymes_source}
PYMES_PROVISION_IMAGE|pymes-v3-provision|${source_sha}|${pymes_source}
PYMES_MIGRATE_IMAGE|pymes-v3-migrate|${source_sha}|${pymes_source}
PYMES_FISCAL_MIGRATE_IMAGE|pymes-v3-fiscal-migrate|${source_sha}|${pymes_source}
PYMES_ACCOUNTING_IMAGE|pymes-v3-accounting|${accounting_sha}|${accounting_source}
PYMES_ACCOUNTING_ADMIN_IMAGE|pymes-v3-accounting-admin|${accounting_sha}|${accounting_source}
PYMES_ACCOUNTING_MIGRATE_IMAGE|pymes-v3-accounting-migrate|${accounting_sha}|${accounting_source}
EOF
  echo "All release image digests have verified provenance, pinned materials, and SPDX SBOMs"
  exit 0
fi

build_image PYMES_API_IMAGE pymes-v3-api api v3 "$source_sha" "$pymes_source"
build_image PYMES_WEB_IMAGE "pymes-v3-web-${PYMES_RELEASE_ENV}" web v3 "$source_sha" "$pymes_source" \
  --build-arg="VITE_CLERK_PUBLISHABLE_KEY=${VITE_CLERK_PUBLISHABLE_KEY}"
build_image PYMES_WORKER_IMAGE pymes-v3-worker worker v3 "$source_sha" "$pymes_source"
build_image PYMES_FISCAL_IMAGE pymes-v3-fiscal fiscal-adapter v3 "$source_sha" "$pymes_source"
build_image PYMES_PROVISION_IMAGE pymes-v3-provision provision-org v3 "$source_sha" "$pymes_source"
build_image PYMES_MIGRATE_IMAGE pymes-v3-migrate migrate v3 "$source_sha" "$pymes_source"
build_image PYMES_FISCAL_MIGRATE_IMAGE pymes-v3-fiscal-migrate fiscal-migrate v3 "$source_sha" "$pymes_source"

build_image PYMES_ACCOUNTING_IMAGE pymes-v3-accounting pymes-accounting "$accounting_context" \
  "$accounting_sha" "$accounting_source"
build_image PYMES_ACCOUNTING_ADMIN_IMAGE pymes-v3-accounting-admin pymes-accounting-admin "$accounting_context" \
  "$accounting_sha" "$accounting_source"
build_image PYMES_ACCOUNTING_MIGRATE_IMAGE pymes-v3-accounting-migrate pymes-accounting-migrate "$accounting_context" \
  "$accounting_sha" "$accounting_source"

manifest_parent=$(dirname -- "$manifest_file")
mkdir -p -- "$manifest_parent"
if [[ -e "$manifest_file" ]]; then
  echo "refusing to overwrite release manifest: $manifest_file" >&2
  exit 1
fi
manifest_tmp=$(mktemp "${manifest_file}.tmp.XXXXXX")
chmod 600 "$manifest_tmp"
{
  printf 'PYMES_RELEASE_ENV=%s\n' "$PYMES_RELEASE_ENV"
  printf 'PYMES_SOURCE_SHA=%s\n' "$source_sha"
  printf 'PYMES_OPEN_ACCOUNTING_SOURCE_SHA=%s\n' "$accounting_sha"
  for key in \
    PYMES_API_IMAGE \
    PYMES_WEB_IMAGE \
    PYMES_WORKER_IMAGE \
    PYMES_FISCAL_IMAGE \
    PYMES_ACCOUNTING_IMAGE \
    PYMES_ACCOUNTING_ADMIN_IMAGE \
    PYMES_PROVISION_IMAGE \
    PYMES_MIGRATE_IMAGE \
    PYMES_FISCAL_MIGRATE_IMAGE \
    PYMES_ACCOUNTING_MIGRATE_IMAGE; do
    printf '%s=%s\n' "$key" "${release_images[$key]}"
  done
} >"$manifest_tmp"
mv -- "$manifest_tmp" "$manifest_file"
chmod 600 "$manifest_file"

printf 'release manifest written: %s\n' "$manifest_file"
