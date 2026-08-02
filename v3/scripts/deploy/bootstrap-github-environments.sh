#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repository=devpablocristo/pymes
repository_id=1173650578
repository_owner_id=81805584
mode=${PYMES_GITHUB_ENVIRONMENT_MODE:-plan}
reviewer_ids_csv=${PYMES_PRD_REVIEWER_IDS:-}

case "$mode" in
  plan|audit|apply) ;;
  *) echo "PYMES_GITHUB_ENVIRONMENT_MODE must be plan, audit or apply" >&2; exit 2 ;;
esac

if [[ "$mode" == "plan" ]]; then
  printf '%s\n' \
    "PLAN repository=${repository}" \
    "PLAN main required_check='Pymes V3 validate' strict=true reviews=1 last_push_approval=true admins=enforced" \
    "PLAN stg branch=main required_reviewers=none" \
    "PLAN prd branch=main required_reviewers=explicit prevent_self_review=true" \
    "PLAN prd admin_bypass=disabled (manual GitHub UI control; audited by script)" \
    "No GitHub settings changed. Use audit to inspect or apply with PYMES_PRD_REVIEWER_IDS."
  exit 0
fi

for command in gh jq; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "$command is required" >&2
    exit 1
  }
done

if [[ "$mode" == "audit" ]]; then
  exec "$script_dir/verify-github-environments.sh" all
fi

[[ "$reviewer_ids_csv" =~ ^[1-9][0-9]*(,[1-9][0-9]*){0,5}$ ]] || {
  echo "PYMES_PRD_REVIEWER_IDS must contain one to six comma-separated GitHub user IDs" >&2
  exit 2
}
reviewers_json=$(jq -cn --arg ids "$reviewer_ids_csv" '
  $ids
  | split(",")
  | map(tonumber)
  | if (unique | length) != length then error("duplicate reviewer ID") else . end
  | map({type:"User", id:.})
')

repository_json=$(gh api \
  -H 'X-GitHub-Api-Version: 2026-03-10' \
  "repos/${repository}")
jq -e \
  --arg repository "$repository" \
  --argjson repository_id "$repository_id" \
  --argjson owner_id "$repository_owner_id" '
    .full_name == $repository and
    .id == $repository_id and
    .owner.id == $owner_id and
    .default_branch == "main" and
    .archived == false and
    .disabled == false and
    .permissions.admin == true
  ' <<<"$repository_json" >/dev/null || {
    echo "GitHub token or repository identity cannot safely apply release controls" >&2
    exit 1
  }
main_branch_json=$(gh api \
  -H 'X-GitHub-Api-Version: 2026-03-10' \
  "repos/${repository}/branches/main")
jq -e '
  .name == "main" and
  (.commit.sha | type == "string" and test("^[0-9a-f]{40}$"))
' <<<"$main_branch_json" >/dev/null || {
  echo "GitHub main branch identity is invalid" >&2
  exit 1
}

collaborators_json=$(gh api \
  -H 'X-GitHub-Api-Version: 2026-03-10' \
  "repos/${repository}/collaborators?per_page=100")
jq -e --argjson reviewers "$reviewers_json" '
  ($reviewers | map(.id) | sort) as $expected
  | ([.[] | select(.permissions.pull == true) | .id] | unique | sort) as $readers
  | ($expected - $readers | length) == 0
' <<<"$collaborators_json" >/dev/null || {
  echo "every PRD reviewer must be a current repository collaborator with read access" >&2
  exit 1
}

tmp_dir=$(mktemp -d)
cleanup() {
  rm -r "$tmp_dir"
}
trap cleanup EXIT INT TERM

jq -n '{
  required_status_checks: {
    strict: true,
    checks: [
      {
        context: "Pymes V3 validate",
        app_id: 15368
      }
    ]
  },
  enforce_admins: true,
  required_pull_request_reviews: {
    dismiss_stale_reviews: true,
    require_code_owner_reviews: false,
    required_approving_review_count: 1,
    require_last_push_approval: true
  },
  restrictions: null,
  required_linear_history: true,
  allow_force_pushes: false,
  allow_deletions: false,
  block_creations: false,
  required_conversation_resolution: true,
  lock_branch: false,
  allow_fork_syncing: false
}' >"$tmp_dir/main-protection.json"
jq -n '{
  wait_timer: 0,
  prevent_self_review: false,
  reviewers: [],
  deployment_branch_policy: {
    protected_branches: false,
    custom_branch_policies: true
  }
}' >"$tmp_dir/stg.json"
jq -n --argjson reviewers "$reviewers_json" '{
  wait_timer: 0,
  prevent_self_review: true,
  reviewers: $reviewers,
  deployment_branch_policy: {
    protected_branches: false,
    custom_branch_policies: true
  }
}' >"$tmp_dir/prd.json"

preflight_main_only_policy() {
  local environment="$1" environment_json policies_json error_file
  error_file="$tmp_dir/${environment}-preflight.err"
  if ! environment_json=$(gh api \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "repos/${repository}/environments/${environment}" 2>"$error_file"); then
    if jq -e '
      .status == "404" and
      .message == "Not Found"
    ' <<<"$environment_json" >/dev/null 2>&1; then
      echo "GitHub environment preflight verified absent: $environment"
      return
    fi
    cat "$error_file" >&2
    echo "GitHub environment preflight could not prove absence or ownership: $environment" >&2
    exit 1
  fi
  jq -e \
    --arg environment "$environment" \
    --argjson expected_reviewers "$reviewers_json" '
    .name == $environment and
    (
      if $environment == "prd"
      then .can_admins_bypass == false
      else true
      end
    ) and
    (
      ([.protection_rules[]?.type] | sort) as $types
      | if $environment == "stg"
        then ($types == [] or $types == ["branch_policy"])
        else
          (
            $types == [] or
            $types == ["branch_policy"] or
            (
              $types == ["branch_policy", "required_reviewers"] and
              any(
                .protection_rules[]?;
                .type == "required_reviewers" and
                .prevent_self_review == true and
                (
                  [
                    .reviewers[]?
                    | (.reviewer.id // .id)
                  ]
                  | unique
                  | sort
                ) == ($expected_reviewers | map(.id) | unique | sort)
              )
            )
          )
        end
    )
  ' <<<"$environment_json" >/dev/null || {
    echo "GitHub environment preflight found unknown or conflicting protection rules: $environment" >&2
    exit 1
  }

  if ! policies_json=$(gh api \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "repos/${repository}/environments/${environment}/deployment-branch-policies" \
    2>"$error_file"); then
    if jq -e '
      .status == "404" and
      .message == "Not Found"
    ' <<<"$policies_json" >/dev/null 2>&1; then
      echo "GitHub environment preflight verified without branch policy: $environment"
      return
    fi
    cat "$error_file" >&2
    echo "GitHub environment branch-policy preflight failed: $environment" >&2
    exit 1
  fi
  jq -e '
    .total_count == 0 or
    (
      .total_count == 1 and
      (.branch_policies | length) == 1 and
      .branch_policies[0].name == "main" and
      ((.branch_policies[0].type // "branch") == "branch")
    )
  ' <<<"$policies_json" >/dev/null || {
    echo "refusing to mutate an environment with unexpected $environment deployment branch policies" >&2
    exit 1
  }
}

for environment in stg prd; do
  preflight_main_only_policy "$environment"
done
echo "GitHub mutation preflight verified: repository=${repository} environments=stg,prd"

gh api --method PUT \
  -H 'X-GitHub-Api-Version: 2026-03-10' \
  "repos/${repository}/branches/main/protection" \
  --input "$tmp_dir/main-protection.json" >/dev/null

ensure_main_only_policy() {
  local environment="$1" policies_json policy_count
  policies_json=$(gh api \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "repos/${repository}/environments/${environment}/deployment-branch-policies")
  policy_count=$(jq -r '.total_count' <<<"$policies_json")
  if [[ "$policy_count" == "0" ]]; then
    gh api --method POST \
      -H 'X-GitHub-Api-Version: 2026-03-10' \
      "repos/${repository}/environments/${environment}/deployment-branch-policies" \
      -f name=main -f type=branch >/dev/null
    return
  fi
  jq -e '
    .total_count == 1 and
    (.branch_policies | length) == 1 and
    .branch_policies[0].name == "main" and
    ((.branch_policies[0].type // "branch") == "branch")
  ' <<<"$policies_json" >/dev/null || {
    echo "refusing to delete or overwrite unexpected $environment deployment branch policies" >&2
    exit 1
  }
}

for environment in stg prd; do
  gh api --method PUT \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "repos/${repository}/environments/${environment}" \
    --input "$tmp_dir/${environment}.json" >/dev/null
  ensure_main_only_policy "$environment"
done

printf '%s\n' \
  "GitHub environment rules were configured." \
  "GitHub does not expose the admin-bypass switch in the documented REST API." \
  "Disable 'Allow administrators to bypass configured protection rules' for prd at:" \
  "https://github.com/${repository}/settings/environments"

"$script_dir/verify-github-environments.sh" all
