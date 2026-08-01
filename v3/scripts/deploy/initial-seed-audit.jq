def successful:
  ((.protoPayload.status.code // 0) == 0);

def actor_matches:
  .protoPayload.authenticationInfo.principalEmail == $operator or
  any(
    .protoPayload.authenticationInfo.serviceAccountDelegationInfo[]?;
    .firstPartyPrincipal.principalEmail == $operator
  );

def normalized_run_resource($resource):
  $resource
  | sub("^//run\\.googleapis\\.com/"; "")
  | sub(
      "^projects/" + $project_number + "/";
      "projects/" + $project + "/"
    );

def normalized_iam_resource($resource):
  $resource | sub("^//iam\\.googleapis\\.com/"; "");

def allowed_run_resource($resource):
  normalized_run_resource($resource) as $normalized
  | any($allowed_resources[]; . == $normalized);

def run_resource_matches_labels($resource):
  normalized_run_resource($resource) as $normalized
  | (
      (.resource.labels.project_id? // $project) as $label_project
      | $label_project == $project or $label_project == $project_number
    ) and
    ((.resource.labels.location? // $region) == $region) and
    (
      (.resource.labels.service_name? // "") as $service_name
      | $service_name == "" or
        $normalized == (
          "projects/" + $project + "/locations/" + $region +
          "/services/" + $service_name
        )
    ) and
    (
      (.resource.labels.job_name? // "") as $job_name
      | $job_name == "" or
        $normalized == (
          "projects/" + $project + "/locations/" + $region +
          "/jobs/" + $job_name
        )
    ) and
    (
      (
        (.resource.labels.service_name? // "") == "" or
        (.resource.labels.job_name? // "") == ""
      )
    );

def run_resource_basename($resource):
  normalized_run_resource($resource) | split("/")[-1];

def request_targets:
  [
    .protoPayload.request.name?,
    .protoPayload.request.resource?,
    .protoPayload.request.resource.name?,
    .protoPayload.request.service.name?,
    .protoPayload.request.service.metadata.name?,
    .protoPayload.request.job.name?,
    .protoPayload.request.job.metadata.name?
  ]
  | map(select(type == "string" and length > 0));

def request_ids:
  [
    .protoPayload.request.serviceId?,
    .protoPayload.request.service_id?,
    .protoPayload.request.jobId?,
    .protoPayload.request.job_id?
  ]
  | map(select(. != null and (tostring | length) > 0))
  | map(tostring);

def optional_request_target_matches($resource):
  all(
    request_targets[];
    . as $request_target |
    normalized_run_resource($request_target) ==
      normalized_run_resource($resource) or
    $request_target == run_resource_basename($resource)
  ) and
  all(
    request_ids[];
    . == run_resource_basename($resource)
  ) and
  (
    (.protoPayload.request.parent? // "") as $parent
    | $parent == "" or
      normalized_run_resource($parent) ==
        (
          "projects/" + $project + "/locations/" + $region
        )
  );

def service_method:
  test("(^|\\.)(CreateService|UpdateService|ReplaceService)$");

def job_method:
  test("(^|\\.)(CreateJob|UpdateJob|ReplaceJob)$");

def valid_run_mutation:
  successful and
  .protoPayload.serviceName == "run.googleapis.com" and
  (
    (.protoPayload.resourceName // "") as $resource
    | $resource != "" and
      allowed_run_resource($resource) and
      run_resource_matches_labels($resource) and
      optional_request_target_matches($resource) and
      (
        (.protoPayload.methodName // "") as $method
        | (
            ($method | service_method) and
            (
              normalized_run_resource($resource)
              | contains("/services/")
            )
          ) or
          (
            ($method | job_method) and
            (
              normalized_run_resource($resource)
              | contains("/jobs/")
            )
          )
      )
  );

def allowed_service_account_resource($resource):
  normalized_iam_resource($resource) as $normalized
  | any(
      $allowed_sas[];
      . as $email |
      $normalized == ("projects/-/serviceAccounts/" + $email) or
      $normalized == ("projects/" + $project + "/serviceAccounts/" + $email) or
      $normalized == ("projects/" + $project_number + "/serviceAccounts/" + $email)
    );

def valid_act_as:
  successful and
  .protoPayload.serviceName == "iam.googleapis.com" and
  .protoPayload.methodName == "iam.serviceAccounts.actAs" and
  .protoPayload.response.success == true and
  (
    (.protoPayload.resourceName // "") as $resource
    | (
        normalized_iam_resource($resource) | split("/")[-1]
      ) as $email
    | allowed_service_account_resource($resource) and
      .protoPayload.request.name == $email and
      (
        (.protoPayload.request.project_number? // $project_number)
        | tostring
      ) == $project_number and
      (
        [.protoPayload.authorizationInfo[]?] as $authorization
        | ($authorization | length) == 1 and
          all(
            $authorization[];
            .permission == "iam.serviceAccounts.actAs" and
            .granted == true and
            (.permissionType // "ADMIN_WRITE") == "ADMIN_WRITE" and
            allowed_service_account_resource(.resource // "") and
            normalized_iam_resource(.resource // "") ==
              normalized_iam_resource($resource)
          )
      )
  );

. as $logs
| ($logs | length) < $limit and
  all(
    $logs[];
    (successful | not) or
    (actor_matches and (valid_run_mutation or valid_act_as))
  ) and
  (
    [$logs[] | select(valid_run_mutation)] as $run_mutations
    | ($run_mutations | length) == ($allowed_resources | length) and
      (
        $allowed_resources
        | all(
            .[];
            . as $resource |
            (
              [
                $run_mutations[]
                | select(
                    normalized_run_resource(.protoPayload.resourceName) ==
                      $resource
                  )
              ]
              | length
            ) == 1
          )
      )
  ) and
  (
    $allowed_resources
    | all(
        .[];
        . as $resource |
        any(
          $logs[];
          valid_run_mutation and
          normalized_run_resource(.protoPayload.resourceName) == $resource
        )
      )
  )
