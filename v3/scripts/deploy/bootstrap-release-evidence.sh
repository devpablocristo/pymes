#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=release-evidence-lib.sh
source "$script_dir/release-evidence-lib.sh"

bootstrap_release_evidence_ensure_iam_read_role() {
  local role_id role_json
  role_id=${PYMES_RELEASE_EVIDENCE_IAM_READ_ROLE##*/}
  if ! gcloud iam roles describe "$role_id" \
    --project="$PYMES_RELEASE_EVIDENCE_PROJECT" >/dev/null 2>&1; then
    gcloud iam roles create "$role_id" \
      --project="$PYMES_RELEASE_EVIDENCE_PROJECT" \
      --title="Pymes v3 release evidence IAM reader" \
      --description="Read only the IAM policy of an explicitly bound release-evidence bucket" \
      --stage=GA \
      --permissions=storage.buckets.getIamPolicy >/dev/null || return
  fi
  role_json=$(gcloud iam roles describe "$role_id" \
    --project="$PYMES_RELEASE_EVIDENCE_PROJECT" --format=json) || return
  pymes_release_evidence_validate_iam_read_role "$role_json"
}

bootstrap_release_evidence_main() {
  local mode environment bucket bucket_uri project_json bucket_json policy_json
  local role_id role_json
  local lock_ack
  mode=${PYMES_RELEASE_EVIDENCE_MODE:-plan}
  environment=${PYMES_RELEASE_EVIDENCE_ENV:-}
  case "$mode" in plan|apply|lock|verify) ;; *)
    echo "PYMES_RELEASE_EVIDENCE_MODE must be plan, apply, lock, or verify" >&2
    return 2
  esac
  pymes_release_evidence_validate_environment "$environment" || return
  bucket=$(pymes_release_evidence_bucket_name "$environment") || return
  bucket_uri="gs://${bucket}"

  printf 'RELEASE EVIDENCE PLAN environment=%s project=%s region=%s bucket=%s retention_seconds=%s\n' \
    "$environment" \
    "$PYMES_RELEASE_EVIDENCE_PROJECT" \
    "$PYMES_RELEASE_EVIDENCE_REGION" \
    "$bucket" \
    "$PYMES_RELEASE_EVIDENCE_RETENTION_SECONDS"
  printf 'RELEASE EVIDENCE PLAN public=false uniform_access=true lifecycle_rules=0 writer=%s overwrite=false\n' \
    "$PYMES_RELEASE_EVIDENCE_BUILDER"

  if [[ "$mode" == plan ]]; then
    echo "PLAN ONLY resources_created=0 resources_changed=0"
    return 0
  fi

  for required in gcloud jq; do
    command -v "$required" >/dev/null 2>&1 || {
      echo "$required is required" >&2
      return 1
    }
  done
  pymes_release_evidence_validate_direct_auth || return
  project_json=$(gcloud projects describe "$PYMES_RELEASE_EVIDENCE_PROJECT" \
    --format=json) || return
  pymes_release_evidence_validate_project "$project_json" || return
  export CLOUDSDK_CORE_PROJECT="$PYMES_RELEASE_EVIDENCE_PROJECT"

  if [[ "$mode" == apply ]]; then
    if ! gcloud storage buckets describe "$bucket_uri" \
      --format=json >/dev/null 2>&1; then
      gcloud storage buckets create "$bucket_uri" \
        --project="$PYMES_RELEASE_EVIDENCE_PROJECT" \
        --location="$PYMES_RELEASE_EVIDENCE_REGION" \
        --default-storage-class=STANDARD \
        --uniform-bucket-level-access \
        --public-access-prevention \
        --retention-period=P1Y >/dev/null || return
      gcloud storage buckets update "$bucket_uri" \
        --update-labels="app=pymes-v3,component=release-evidence,environment=${environment},managed_by=pymes-v3" \
        >/dev/null || return
    fi

    bucket_json=$(gcloud storage buckets describe "$bucket_uri" --format=json) ||
      return
    pymes_release_evidence_validate_bucket \
      "$bucket_json" "$environment" false || return

    bootstrap_release_evidence_ensure_iam_read_role || return
    gcloud storage buckets add-iam-policy-binding "$bucket_uri" \
      --member="serviceAccount:${PYMES_RELEASE_EVIDENCE_BUILDER}" \
      --role=roles/storage.legacyBucketReader >/dev/null || return
    gcloud storage buckets add-iam-policy-binding "$bucket_uri" \
      --member="serviceAccount:${PYMES_RELEASE_EVIDENCE_BUILDER}" \
      --role=roles/storage.objectCreator >/dev/null || return
    gcloud storage buckets add-iam-policy-binding "$bucket_uri" \
      --member="serviceAccount:${PYMES_RELEASE_EVIDENCE_BUILDER}" \
      --role=roles/storage.objectViewer >/dev/null || return
    gcloud storage buckets add-iam-policy-binding "$bucket_uri" \
      --member="serviceAccount:${PYMES_RELEASE_EVIDENCE_BUILDER}" \
      --role="$PYMES_RELEASE_EVIDENCE_IAM_READ_ROLE" >/dev/null || return
  fi

  role_id=${PYMES_RELEASE_EVIDENCE_IAM_READ_ROLE##*/}
  role_json=$(gcloud iam roles describe "$role_id" \
    --project="$PYMES_RELEASE_EVIDENCE_PROJECT" --format=json) || return
  pymes_release_evidence_validate_iam_read_role "$role_json" || return
  bucket_json=$(gcloud storage buckets describe "$bucket_uri" --format=json) ||
    return
  policy_json=$(gcloud storage buckets get-iam-policy "$bucket_uri" \
    --format=json) || return
  pymes_release_evidence_validate_bucket \
    "$bucket_json" "$environment" \
    "$([[ "$mode" == verify ]] && echo true || echo false)" || return
  pymes_release_evidence_validate_bucket_iam "$policy_json" || return

  if [[ "$mode" == lock ]]; then
    if jq -e '.retentionPolicy.isLocked == true' \
      <<<"$bucket_json" >/dev/null; then
      echo "RELEASE EVIDENCE READY environment=$environment bucket=$bucket locked=true"
      return 0
    fi
    lock_ack="LOCK_RELEASE_EVIDENCE_${environment^^}_${bucket}"
    if [[ "${PYMES_RELEASE_EVIDENCE_LOCK_CONFIRMATION:-}" != "$lock_ack" ]]; then
      echo "locking the retention policy is irreversible and creates a project lien" >&2
      echo "set PYMES_RELEASE_EVIDENCE_LOCK_CONFIRMATION=$lock_ack" >&2
      return 2
    fi
    gcloud storage buckets update "$bucket_uri" \
      --lock-retention-period >/dev/null || return
    bucket_json=$(gcloud storage buckets describe "$bucket_uri" --format=json) ||
      return
    pymes_release_evidence_validate_bucket \
      "$bucket_json" "$environment" true || return
  fi

  locked=$(jq -r '.retentionPolicy.isLocked // false' <<<"$bucket_json")
  printf 'RELEASE EVIDENCE READY environment=%s bucket=%s locked=%s retention_seconds=%s\n' \
    "$environment" "$bucket" "$locked" \
    "$(jq -r '.retentionPolicy.retentionPeriod' <<<"$bucket_json")"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  bootstrap_release_evidence_main "$@"
fi
