#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077

# Performs a manual, application-only rollback of the public Web/API pair.
# Database migrations are deliberately outside this operation. Each release is
# recovered from immutable Cloud Run revision metadata and is revalidated
# before any traffic is changed.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repository_root=$(cd -- "$script_dir/../../.." && pwd)
attestation_verifier="$script_dir/build-push-images.sh"
# shellcheck source=release-candidate-tag.sh
source "$script_dir/release-candidate-tag.sh"

: "${PYMES_DEPLOY_ENV:?set PYMES_DEPLOY_ENV to stg or prd}"
case "$PYMES_DEPLOY_ENV" in
  stg|prd) ;;
  *)
    echo "PYMES_DEPLOY_ENV must be stg or prd" >&2
    exit 2
    ;;
esac

: "${PYMES_ROLLBACK_RELEASE_SHA:?set PYMES_ROLLBACK_RELEASE_SHA to the exact release commit}"
if [[ ! "$PYMES_ROLLBACK_RELEASE_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "PYMES_ROLLBACK_RELEASE_SHA must be exactly 40 lowercase hexadecimal characters" >&2
  exit 2
fi

: "${PYMES_ROLLBACK_IMAGE_MANIFEST:?set PYMES_ROLLBACK_IMAGE_MANIFEST to the downloaded immutable release manifest}"
: "${PYMES_ROLLBACK_IMAGE_MANIFEST_SHA256:?set PYMES_ROLLBACK_IMAGE_MANIFEST_SHA256 to the independently recorded manifest checksum}"
if [[ ! "$PYMES_ROLLBACK_IMAGE_MANIFEST_SHA256" =~ ^[0-9a-f]{64}$ ]]; then
  echo "PYMES_ROLLBACK_IMAGE_MANIFEST_SHA256 must be exactly 64 lowercase hexadecimal characters" >&2
  exit 2
fi
if [[ ! -f "$PYMES_ROLLBACK_IMAGE_MANIFEST" ||
      -L "$PYMES_ROLLBACK_IMAGE_MANIFEST" ]]; then
  echo "PYMES_ROLLBACK_IMAGE_MANIFEST must be one regular non-symlink file" >&2
  exit 2
fi

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
artifact_repository=${PYMES_ARTIFACT_REPOSITORY:-pymes}
if [[ "$project" != "pymes-dev-352318" ]]; then
  echo "PYMES_GCP_PROJECT must be the canonical pymes-dev-352318 project" >&2
  exit 2
fi
if [[ "$region" != "us-central1" ]]; then
  echo "PYMES_GCP_REGION must be the canonical us-central1 region" >&2
  exit 2
fi
if [[ "$artifact_repository" != "pymes" ]]; then
  echo "PYMES_ARTIFACT_REPOSITORY must be the canonical pymes repository" >&2
  exit 2
fi

for dependency in gcloud jq curl docker git sha256sum; do
  command -v "$dependency" >/dev/null 2>&1 || {
    echo "required command is unavailable: $dependency" >&2
    exit 2
  }
done

release_sha=$PYMES_ROLLBACK_RELEASE_SHA
candidate_tag=$(pymes_release_candidate_tag "$release_sha")
prefix="pymes-v3-${PYMES_DEPLOY_ENV}"
api_service="${prefix}-api"
web_service="${prefix}-web"
api_service_account="pymes-v3-api-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
web_service_account="pymes-v3-web-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
registry="${region}-docker.pkg.dev/${project}/${artifact_repository}"
revision_pattern='^[a-z][a-z0-9-]{0,62}$'

fail() {
  echo "Cloud Run rollback refused: $*" >&2
  exit 1
}

validate_release_manifest() {
  local actual_checksum key value pin_file pinned_accounting_sha
  local expected_prefix image_digest
  actual_checksum=$(sha256sum -- "$PYMES_ROLLBACK_IMAGE_MANIFEST" | awk '{print $1}')
  [[ "$actual_checksum" == "$PYMES_ROLLBACK_IMAGE_MANIFEST_SHA256" ]] ||
    fail "release manifest checksum does not match the independently recorded value"

  declare -gA release_manifest=()
  declare -A expected=(
    [PYMES_RELEASE_ENV]="$PYMES_DEPLOY_ENV"
    [PYMES_SOURCE_SHA]="$release_sha"
    [PYMES_OPEN_ACCOUNTING_SOURCE_SHA]="sha"
    [PYMES_API_IMAGE]="pymes-v3-api"
    [PYMES_WEB_IMAGE]="pymes-v3-web-${PYMES_DEPLOY_ENV}"
    [PYMES_WORKER_IMAGE]="pymes-v3-worker"
    [PYMES_FISCAL_IMAGE]="pymes-v3-fiscal"
    [PYMES_ACCOUNTING_IMAGE]="pymes-v3-accounting"
    [PYMES_ACCOUNTING_ADMIN_IMAGE]="pymes-v3-accounting-admin"
    [PYMES_PROVISION_IMAGE]="pymes-v3-provision"
    [PYMES_MIGRATE_IMAGE]="pymes-v3-migrate"
    [PYMES_FISCAL_MIGRATE_IMAGE]="pymes-v3-fiscal-migrate"
    [PYMES_ACCOUNTING_MIGRATE_IMAGE]="pymes-v3-accounting-migrate"
  )

  while IFS='=' read -r key value; do
    [[ -n "$key" && -n "$value" && -z "${release_manifest[$key]:-}" ]] ||
      fail "release manifest contains an empty or duplicate entry"
    [[ -n "${expected[$key]+present}" ]] ||
      fail "release manifest contains non-allowlisted key $key"
    case "$key" in
      PYMES_RELEASE_ENV|PYMES_SOURCE_SHA)
        [[ "$value" == "${expected[$key]}" ]] ||
          fail "release manifest $key does not match the requested rollback"
        ;;
      PYMES_OPEN_ACCOUNTING_SOURCE_SHA)
        [[ "$value" =~ ^[0-9a-f]{40}$ ]] ||
          fail "release manifest has an invalid Open Accounting source SHA"
        ;;
      *)
        expected_prefix="${registry}/${expected[$key]}@"
        image_digest=${value#"$expected_prefix"}
        [[ "$value" == "$expected_prefix$image_digest" &&
            "$image_digest" =~ ^sha256:[0-9a-f]{64}$ ]] ||
          fail "release manifest $key is not the exact canonical immutable image"
        ;;
    esac
    release_manifest["$key"]=$value
  done <"$PYMES_ROLLBACK_IMAGE_MANIFEST"

  for key in "${!expected[@]}"; do
    [[ -n "${release_manifest[$key]:-}" ]] ||
      fail "release manifest is missing $key"
  done
  [[ "${#release_manifest[@]}" -eq "${#expected[@]}" ]] ||
    fail "release manifest has an unexpected shape"

  pin_file="$repository_root/.github/dependencies/open-accounting.env"
  [[ -f "$pin_file" ]] ||
    fail "reviewed Open Accounting pin is unavailable"
  pinned_accounting_sha=$(sed -n \
    's/^OPEN_ACCOUNTING_REF=//p' "$pin_file")
  [[ "$(grep -c '^OPEN_ACCOUNTING_REF=' "$pin_file")" -eq 1 &&
      "$pinned_accounting_sha" == \
        "${release_manifest[PYMES_OPEN_ACCOUNTING_SOURCE_SHA]}" ]] ||
    fail "release manifest Open Accounting SHA differs from the reviewed source pin"
}

verify_release_supply_chain() {
  local accounting_context
  accounting_context=${OPEN_ACCOUNTING_CONTEXT:-"$repository_root/.deps/open-accounting"}
  (
    cd "$repository_root"
    export \
      PYMES_RELEASE_ENV="${release_manifest[PYMES_RELEASE_ENV]}" \
      PYMES_SOURCE_SHA="${release_manifest[PYMES_SOURCE_SHA]}" \
      PYMES_GCP_PROJECT="$project" \
      PYMES_GCP_REGION="$region" \
      PYMES_ARTIFACT_REPOSITORY="$artifact_repository" \
      OPEN_ACCOUNTING_CONTEXT="$accounting_context"
    local manifest_key
    for manifest_key in \
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
      export "$manifest_key=${release_manifest[$manifest_key]}"
    done
    "$attestation_verifier" verify-attestations
  ) >/dev/null ||
    fail "release manifest images failed canonical provenance, material, or SBOM verification"
}

service_json() {
  gcloud run services describe "$1" \
    --project="$project" --region="$region" --format=json
}

service_iam_policy_json() {
  gcloud run services get-iam-policy "$1" \
    --project="$project" --region="$region" --format=json
}

revision_json() {
  gcloud run revisions describe "$1" \
    --project="$project" --region="$region" --format=json
}

revision_env_value() {
  local name="$1"
  jq -er --arg name "$name" '
    (
      (
        .spec.containers[0].env //
        .spec.template.spec.containers[0].env //
        .template.containers[0].env //
        .containers[0].env //
        []
      )
      | [.[] | select(.name == $name)]
    ) as $matches
    | select(
        ($matches | length) == 1 and
        ($matches[0].value | type) == "string"
      )
    | $matches[0].value
  '
}

revision_image() {
  jq -er '
    .spec.containers[0].image //
    .spec.template.spec.containers[0].image //
    .template.containers[0].image //
    .containers[0].image
  '
}

resolve_release_revision() {
  local service="$1" revisions resolved
  revisions=$(gcloud run revisions list \
    --service="$service" \
    --project="$project" --region="$region" --format=json)
  resolved=$(jq -er --arg release "$release_sha" '
    [
      .[]
      | select(
          (
            .metadata.labels["pymes-v3-release"] //
            .labels["pymes-v3-release"] //
            ""
          ) == $release
        )
      | (.metadata.name // .name // "")
      | select(length > 0)
    ] as $matches
    | select(($matches | length) == 1)
    | $matches[0]
  ' <<<"$revisions") || {
    fail "service $service must have exactly one revision for release $release_sha"
  }
  if [[ ! "$resolved" =~ $revision_pattern ]]; then
    fail "service $service resolved an unsafe revision name"
  fi
  printf '%s' "$resolved"
}

validate_revision() {
  local role="$1" service="$2" revision="$3" json="$4"
  local expected_service_account="$5" image
  jq -e \
    --arg name "$revision" \
    --arg service "$service" \
    --arg service_account "$expected_service_account" \
    --arg environment "$PYMES_DEPLOY_ENV" \
    --arg release "$release_sha" '
      (.metadata.name // .name // "") == $name and
      (
        .metadata.labels["serving.knative.dev/service"] //
        .metadata.labels["run.googleapis.com/service"] //
        .labels["serving.knative.dev/service"] //
        ""
      ) == $service and
      (
        .metadata.labels.app //
        .labels.app //
        ""
      ) == "pymes-v3" and
      (
        .metadata.labels.env //
        .labels.env //
        ""
      ) == $environment and
      (
        .metadata.labels["pymes-v3-release"] //
        .labels["pymes-v3-release"] //
        ""
      ) == $release and
      (
        [
          (.status.conditions // .conditions // [])[]
          | select(
              (.type == "Ready" or .type == "READY") and
              (.status == "True" or .state == "CONDITION_SUCCEEDED")
            )
        ] | length
      ) == 1 and
      (
        (
          .spec.containers //
          .spec.template.spec.containers //
          .template.containers //
          .containers //
          []
        ) | length
      ) == 1 and
      (
        .spec.serviceAccountName //
        .spec.template.spec.serviceAccountName //
        .template.serviceAccount //
        .serviceAccount //
        ""
      ) == $service_account
    ' <<<"$json" >/dev/null || {
    fail "$role revision $revision has invalid ownership, identity, release labels, readiness, or container shape"
  }
  image=$(revision_image <<<"$json") ||
    fail "$role revision $revision has no unambiguous image"
  if [[ "$image" != "$registry/"* ||
        ! "${image##*@}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    fail "$role revision $revision is not pinned to a regional immutable image digest"
  fi
  printf '%s' "$image"
}

service_origin() {
  local service="$1" json="$2" origin
  origin=$(jq -er '.status.url | select(type == "string" and length > 0)' \
    <<<"$json") || fail "service $service has no HTTPS control-plane URL"
  if [[ ! "$origin" =~ ^https://${service}-[a-z0-9-]+([.][a-z0-9-]+)*[.]run[.]app$ ]]; then
    fail "service $service has an unexpected Cloud Run URL"
  fi
  printf '%s' "$origin"
}

validate_public_service_security() {
  local service="$1" json="$2" policy
  jq -e '
    (
      .metadata.annotations["run.googleapis.com/invoker-iam-disabled"] //
      .invokerIamDisabled //
      false
    ) as $disabled
    | (($disabled | tostring | ascii_downcase) == "false") and
      (
        (
          .metadata.annotations["run.googleapis.com/ingress"] //
          .ingress //
          ""
        ) == "all"
      )
  ' <<<"$json" >/dev/null ||
    fail "service $service must enforce invoker IAM and exact ingress=all before rollback"

  policy=$(service_iam_policy_json "$service")
  jq -e '
    [
      (.bindings // [])[]
      | select(.role == "roles/run.invoker")
    ] as $invoker
    | ($invoker | length) == 1 and
      (($invoker[0].condition // null) == null) and
      (($invoker[0].members // []) == ["allUsers"])
  ' <<<"$policy" >/dev/null ||
    fail "service $service must grant roles/run.invoker exactly and unconditionally to allUsers"
}

assert_tag_mapping() {
  local exact="$1" json
  json=$(service_json "$api_service")
  jq -e \
    --arg tag "$candidate_tag" \
    --arg revision "$api_revision" \
    --arg url "$api_tag_origin" \
    --argjson exact "$exact" '
      [
        (.status.traffic // .trafficStatuses // [])[]
        | select((.tag // "") | length > 0)
        | {
            tag: .tag,
            revision: (.revisionName // .revision // ""),
            url: (.url // "")
          }
      ] as $tags
      | (
          [
            $tags[]
            | select(
                .tag == $tag and
                .revision == $revision and
                .url == $url
              )
          ] | length
        ) == 1 and
        (
          ($exact | not) or
          (
            ($tags | length) == 1 and
            $tags[0].tag == $tag and
            $tags[0].revision == $revision and
            $tags[0].url == $url
          )
        )
    ' <<<"$json" >/dev/null
}

assert_active_revision() {
  local service="$1" revision="$2" json
  json=$(service_json "$service")
  jq -e --arg revision "$revision" '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select((.percent // 0) > 0)
    ] as $active
    | ($active | length) == 1 and
      ($active[0].percent // 0) == 100 and
      ($active[0].revisionName // $active[0].revision // "") == $revision
  ' <<<"$json" >/dev/null
}

assert_web_has_no_tags() {
  local json
  json=$(service_json "$web_service")
  jq -e '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select((.tag // "") | length > 0)
    ] | length == 0
  ' <<<"$json" >/dev/null
}

secret_curl_config=$(mktemp)
http_headers=$(mktemp)
http_body=$(mktemp)
cleanup() {
  rm -f -- "$secret_curl_config" "$http_headers" "$http_body"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 600 "$secret_curl_config"

http_exact() {
  local label="$1" url="$2" expected_status="$3" use_secret="$4"
  local result status effective redirect
  local -a arguments=(
    --disable
    --proto '=https'
    --tlsv1.2
    --silent
    --show-error
    --max-redirs 0
    --connect-timeout 10
    --max-time 30
    --header 'Accept: application/json'
    --dump-header "$http_headers"
    --output "$http_body"
    --write-out $'%{http_code}\n%{url_effective}\n%{redirect_url}'
  )
  if [[ "$use_secret" == "true" ]]; then
    arguments+=(--config "$secret_curl_config")
  fi
  result=$(curl "${arguments[@]}" "$url") ||
    fail "$label request failed"
  mapfile -t curl_result <<<"$result"
  status=${curl_result[0]:-}
  effective=${curl_result[1]:-}
  redirect=${curl_result[2]:-}
  if [[ "$status" != "$expected_status" ||
        "$effective" != "$url" ||
        -n "$redirect" ]] ||
    grep -Eiq '^location[[:space:]]*:' "$http_headers"; then
    fail "$label did not return exact HTTP $expected_status without redirects"
  fi
}

verify_tagged_api_data_plane() {
  http_exact "tagged API readiness" "${api_tag_origin}/readyz" 200 true
  http_exact "tagged API authentication boundary" \
    "${api_tag_origin}/api/v1/session" 403 true
  jq -e '
    type == "object" and
    .code == "FORBIDDEN" and
    (.message | type == "string" and length > 0)
  ' "$http_body" >/dev/null ||
    fail "tagged API did not return the canonical unauthenticated response"
}

verify_active_api_data_plane() {
  http_exact "active API readiness" "${api_origin}/readyz" 200 true
}

verify_active_web_data_plane() {
  local marker
  http_exact "active Web readiness" "${web_origin}/readyz" 200 false
  marker=$(awk '
    BEGIN { IGNORECASE=1 }
    /^x-pymes-release:/ {
      sub(/^[^:]*:[[:space:]]*/, "", $0)
      gsub("\r", "", $0)
      value=$0
    }
    END { print value }
  ' "$http_headers")
  [[ "$marker" == "$web_release_marker" ]] ||
    fail "active Web did not expose the rollback release marker"
  http_exact "active Web same-origin API proxy" \
    "${web_origin}/api/v1/session" 403 false
  jq -e '
    type == "object" and
    .code == "FORBIDDEN" and
    (.message | type == "string" and length > 0)
  ' "$http_body" >/dev/null ||
    fail "active Web did not proxy to the guarded rollback API"
}

mutate_api_tag() {
  local mode="$1" failed=false
  local flag
  case "$mode" in
    preserve) flag=--update-tags ;;
    exact) flag=--set-tags ;;
    *) fail "internal invalid tag mutation mode" ;;
  esac
  if ! gcloud run services update-traffic "$api_service" \
    --project="$project" --region="$region" \
    "$flag=${candidate_tag}=${api_revision}" --quiet >/dev/null; then
    failed=true
  fi
  if ! assert_tag_mapping "$([[ "$mode" == "exact" ]] && echo true || echo false)"; then
    if [[ "$mode" == "exact" ]]; then
      # A failed final cleanup must never strand the active Web without its
      # target. Repair the required mapping while preserving whatever tags the
      # control plane retained, then return failure for operator attention.
      gcloud run services update-traffic "$api_service" \
        --project="$project" --region="$region" \
        --update-tags="${candidate_tag}=${api_revision}" --quiet >/dev/null ||
        fail "final API tag policy failed and the required mapping could not be repaired"
      assert_tag_mapping false ||
        fail "final API tag policy failed and the required mapping is not viable"
    fi
    fail "API tag mutation did not converge to the required mapping"
  fi
  if [[ "$failed" == "true" ]]; then
    echo "Cloud Run API tag command timed out or failed, but readback proved the requested state" >&2
  fi
}

move_traffic() {
  local role="$1" service="$2" revision="$3" failed=false
  if ! gcloud run services update-traffic "$service" \
    --project="$project" --region="$region" \
    --to-revisions="${revision}=100" --quiet >/dev/null; then
    failed=true
  fi
  assert_active_revision "$service" "$revision" ||
    fail "$role traffic did not converge to revision $revision"
  if [[ "$failed" == "true" ]]; then
    echo "Cloud Run $role traffic command timed out or failed, but readback proved the requested state" >&2
  fi
}

clear_web_tags() {
  local failed=false
  if ! gcloud run services update-traffic "$web_service" \
    --project="$project" --region="$region" \
    --clear-tags --quiet >/dev/null; then
    failed=true
  fi
  assert_web_has_no_tags ||
    fail "Web tags were not completely removed"
  if [[ "$failed" == "true" ]]; then
    echo "Cloud Run Web tag cleanup timed out or failed, but readback proved the requested state" >&2
  fi
}

# Bind the request to the exact release artifact and re-run the same
# provenance/material/SBOM verification used by v3-release.yml. This happens
# before reading or mutating Cloud Run, so labels can never authorize rollback
# by themselves.
validate_release_manifest
verify_release_supply_chain

# Resolve and verify the immutable pair in full before the first mutation.
api_revision=$(resolve_release_revision "$api_service")
web_revision=$(resolve_release_revision "$web_service")
api_revision_document=$(revision_json "$api_revision")
web_revision_document=$(revision_json "$web_revision")
api_image=$(validate_revision API "$api_service" "$api_revision" \
  "$api_revision_document" "$api_service_account")
web_image=$(validate_revision Web "$web_service" "$web_revision" \
  "$web_revision_document" "$web_service_account")
if [[ "$api_image" != "${release_manifest[PYMES_API_IMAGE]}" ]]; then
  fail "API revision image does not equal the immutable release manifest"
fi
if [[ "$web_image" != "${release_manifest[PYMES_WEB_IMAGE]}" ]]; then
  fail "Web revision image does not equal the immutable release manifest"
fi

api_service_document=$(service_json "$api_service")
web_service_document=$(service_json "$web_service")
validate_public_service_security "$api_service" "$api_service_document"
validate_public_service_security "$web_service" "$web_service_document"
api_origin=$(service_origin "$api_service" "$api_service_document")
web_origin=$(service_origin "$web_service" "$web_service_document")
api_tag_origin="https://${candidate_tag}---${api_origin#https://}"
pymes_validate_cloud_run_tagged_url \
  "$api_tag_origin" "$candidate_tag" "$api_service" ||
  fail "derived API candidate URL violates the canonical Cloud Run DNS policy"

api_preflight_tag=$(revision_env_value PYMES_PREFLIGHT_TAG \
  <<<"$api_revision_document") ||
  fail "API revision does not preserve one direct PYMES_PREFLIGHT_TAG"
web_preflight_tag=$(revision_env_value PYMES_PREFLIGHT_TAG \
  <<<"$web_revision_document") ||
  fail "Web revision does not preserve one direct PYMES_PREFLIGHT_TAG"
api_preflight_token=$(revision_env_value PYMES_PREFLIGHT_TOKEN \
  <<<"$api_revision_document") ||
  fail "API revision does not preserve one direct PYMES_PREFLIGHT_TOKEN"
web_preflight_token=$(revision_env_value PYMES_PREFLIGHT_TOKEN \
  <<<"$web_revision_document") ||
  fail "Web revision does not preserve one direct PYMES_PREFLIGHT_TOKEN"
web_api_upstream=$(revision_env_value PYMES_API_UPSTREAM \
  <<<"$web_revision_document") ||
  fail "Web revision does not preserve one direct PYMES_API_UPSTREAM"
web_release_marker=$(revision_env_value PYMES_RELEASE_MARKER \
  <<<"$web_revision_document") ||
  fail "Web revision does not preserve one direct PYMES_RELEASE_MARKER"

if [[ "$api_preflight_tag" != "$candidate_tag" ||
      "$web_preflight_tag" != "$candidate_tag" ]]; then
  fail "Web/API preflight tags do not match the requested release"
fi
if [[ ! "$api_preflight_token" =~ ^[0-9a-f]{64}$ ||
      "$api_preflight_token" != "$web_preflight_token" ]]; then
  fail "Web/API preflight capabilities are invalid or do not match"
fi
if [[ "$web_api_upstream" != "$api_tag_origin" ]]; then
  fail "Web revision does not point to the exact tagged API revision origin"
fi
web_digest=${web_image##*@}
if [[ "$web_release_marker" != "${PYMES_DEPLOY_ENV}:${release_sha}:${web_digest}" ]]; then
  fail "Web release marker does not bind environment, source SHA, and image digest"
fi

# The capability is intentionally written only to a mode-0600 curl config and
# is never included in diagnostics or command-line arguments.
printf 'header = "X-Pymes-Preflight-Token: %s"\n' \
  "$api_preflight_token" >"$secret_curl_config"
unset api_preflight_token web_preflight_token

echo "ROLLBACK TARGET environment=$PYMES_DEPLOY_ENV release=$release_sha api_revision=$api_revision web_revision=$web_revision"

# Preserve existing tags until the old Web is no longer active. The Web is
# switched only after the rollback API tag is both visible in the control plane
# and reachable through the guarded data plane. This is intentionally monotonic:
# before the Web switch, a failure leaves only a capability-gated zero-traffic
# tag; after it, the selected Web keeps using that already-proven tagged API,
# even if moving the API's stable service URL must be retried by the operator.
mutate_api_tag preserve
verify_tagged_api_data_plane
echo "ROLLBACK API TAG VERIFIED tag=$candidate_tag revision=$api_revision"

move_traffic Web "$web_service" "$web_revision"
verify_active_web_data_plane
echo "ROLLBACK WEB VERIFIED revision=$web_revision traffic=100"

move_traffic API "$api_service" "$api_revision"
verify_active_api_data_plane
echo "ROLLBACK API VERIFIED revision=$api_revision traffic=100"

# Leave a single durable API tag for the active Web and no externally
# addressable Web revision tags.
mutate_api_tag exact
clear_web_tags
assert_active_revision "$web_service" "$web_revision" ||
  fail "Web traffic changed during final tag settlement"
assert_active_revision "$api_service" "$api_revision" ||
  fail "API traffic changed during final tag settlement"
assert_tag_mapping true ||
  fail "final API tag policy changed after settlement"
assert_web_has_no_tags ||
  fail "final Web tag policy changed after settlement"

echo "ROLLBACK COMPLETE environment=$PYMES_DEPLOY_ENV release=$release_sha database=unchanged api_tag=$candidate_tag web_tags=none"
