package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v2"
)

type object map[string]any

var actionReferencePattern = regexp.MustCompile(`^([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)@([0-9a-f]{40})$`)

func main() {
	if len(os.Args) != 3 {
		fatalf("usage: workflowpolicy <v3-ci.yml> <v3-release.yml>")
	}
	ci := readWorkflow(os.Args[1])
	release := readWorkflow(os.Args[2])
	validatePinnedUses(ci, "ci")
	validatePinnedUses(release, "release")
	validateCI(ci)
	validateRelease(release)
	fmt.Println("Structured GitHub workflow policy verified")
}

func validatePinnedUses(value any, path string) {
	switch typed := value.(type) {
	case object:
		for key, child := range typed {
			childPath := path + "." + key
			if key == "uses" {
				reference, ok := child.(string)
				if !ok {
					fatalf("%s must be a string", childPath)
				}
				matches := actionReferencePattern.FindStringSubmatch(reference)
				if matches == nil {
					fatalf("%s contains mutable, local, Docker, or invalid action reference %q", childPath, reference)
				}
				switch matches[1] {
				case "actions", "docker", "google-github-actions":
				// Explicitly allowlisted publishers only.
				default:
					fatalf("%s uses non-allowlisted action owner %q", childPath, matches[1])
				}
			}
			validatePinnedUses(child, childPath)
		}
	case []any:
		for index, child := range typed {
			validatePinnedUses(child, fmt.Sprintf("%s[%d]", path, index))
		}
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "structured workflow policy violation: "+format+"\n", args...)
	os.Exit(1)
}

func readWorkflow(path string) object {
	data, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	var raw any
	if err := yaml.UnmarshalStrict(data, &raw); err != nil {
		fatalf("parse %s: %v", path, err)
	}
	normalized, err := normalize(raw, path)
	if err != nil {
		fatalf("%v", err)
	}
	return mustObject(normalized, path)
}

func normalize(value any, path string) (any, error) {
	switch typed := value.(type) {
	case map[any]any:
		result := make(object, len(typed))
		for rawKey, rawValue := range typed {
			var key string
			switch candidate := rawKey.(type) {
			case string:
				key = candidate
			case bool:
				// YAML 1.1 treats the GitHub-specific root key "on" as true.
				if candidate {
					key = "on"
				} else {
					return nil, fmt.Errorf("%s has unsupported boolean mapping key", path)
				}
			default:
				return nil, fmt.Errorf("%s has non-string mapping key %v", path, rawKey)
			}
			if _, exists := result[key]; exists {
				return nil, fmt.Errorf("%s has duplicate key %q", path, key)
			}
			child, err := normalize(rawValue, path+"."+key)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			normalized, err := normalize(child, fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	default:
		return value, nil
	}
}

func validateCI(workflow object) {
	requireString(workflow, "name", "Pymes V3 CI", "ci")
	triggers := requireObject(workflow, "on", "ci")
	requireExactKeys(triggers, "ci.on", "pull_request", "push")
	push := requireObject(triggers, "push", "ci.on")
	branches := requireStrings(push, "branches", "ci.on.push")
	requireExactStrings(branches, "ci.on.push.branches", "main")
	requirePermissions(workflow, "ci", map[string]string{"contents": "read"})

	jobs := requireObject(workflow, "jobs", "ci")
	requireExactKeys(jobs, "ci.jobs", "validate")
	validate := mustObject(jobs["validate"], "ci.jobs.validate")
	requireString(validate, "name", "Pymes V3 validate", "ci.jobs.validate")
}

func validateRelease(workflow object) {
	requireString(workflow, "name", "Pymes V3 Release", "release")
	requireString(
		workflow,
		"run-name",
		"Pymes V3 ${{ inputs.environment }} ${{ inputs.deploy_stage }} @ ${{ github.sha }}",
		"release",
	)
	triggers := requireObject(workflow, "on", "release")
	requireExactKeys(triggers, "release.on", "workflow_dispatch")
	dispatch := requireObject(triggers, "workflow_dispatch", "release.on")
	inputs := requireObject(dispatch, "inputs", "release.on.workflow_dispatch")
	deployStage := requireObject(inputs, "deploy_stage", "release.on.workflow_dispatch.inputs")
	requireString(
		deployStage,
		"type",
		"choice",
		"release.on.workflow_dispatch.inputs.deploy_stage",
	)
	requireString(
		deployStage,
		"default",
		"operational",
		"release.on.workflow_dispatch.inputs.deploy_stage",
	)
	requireBool(
		deployStage,
		"required",
		true,
		"release.on.workflow_dispatch.inputs.deploy_stage",
	)
	requireExactStrings(
		requireStrings(
			deployStage,
			"options",
			"release.on.workflow_dispatch.inputs.deploy_stage",
		),
		"release.on.workflow_dispatch.inputs.deploy_stage.options",
		"bootstrap",
		"operational",
	)
	requirePermissions(workflow, "release", map[string]string{})

	jobs := requireObject(workflow, "jobs", "release")
	requireExactKeys(jobs, "release.jobs", "build", "deploy", "validate")
	validate := mustObject(jobs["validate"], "release.jobs.validate")
	build := mustObject(jobs["build"], "release.jobs.build")
	deploy := mustObject(jobs["deploy"], "release.jobs.deploy")

	requireEnvironment(validate, "release.jobs.validate", "${{ inputs.environment }}")
	if _, exists := build["environment"]; exists {
		fatalf("release.jobs.build must not reference a GitHub environment")
	}
	requireEnvironment(deploy, "release.jobs.deploy", "${{ inputs.environment }}")
	requireNeeds(build, "release.jobs.build", "validate")
	requireNeeds(deploy, "release.jobs.deploy", "build", "validate")
	requirePermissions(validate, "release.jobs.validate", map[string]string{
		"actions":  "read",
		"contents": "read",
		"id-token": "write",
	})
	requirePermissions(build, "release.jobs.build", map[string]string{
		"contents": "read",
		"id-token": "write",
	})
	requirePermissions(deploy, "release.jobs.deploy", map[string]string{
		"actions":  "read",
		"contents": "read",
		"id-token": "write",
	})

	if err := validateRerunGuard(build, "Reject standalone build reruns", false); err != nil {
		fatalf("release image builder rerun guard: %v", err)
	}
	if err := validateRerunGuard(deploy, "Reject standalone deploy reruns", true); err != nil {
		fatalf("release deployer rerun guard: %v", err)
	}

	outputs := requireObject(validate, "outputs", "release.jobs.validate")
	requireString(
		outputs,
		"clerk_publishable_key",
		"${{ steps.release-config.outputs.clerk_publishable_key }}",
		"release.jobs.validate.outputs",
	)

	fullAudit := requireStep(validate, "Verify complete protected GitHub controls")
	auditEnv := requireObject(fullAudit, "env", "release.jobs.validate full GitHub audit")
	requireString(
		auditEnv,
		"GH_TOKEN",
		"${{ secrets.PYMES_GITHUB_RELEASE_AUDIT_TOKEN }}",
		"release.jobs.validate full GitHub audit env",
	)
	auditRun := requireScalarString(fullAudit, "run", "release.jobs.validate full GitHub audit")
	if !strings.Contains(auditRun, "verify-github-environments.sh all all-controls") ||
		strings.Contains(auditRun, "environment-only") {
		fatalf("release full GitHub audit must execute all-controls for both environments")
	}
	preBuildAudit := requireStep(
		validate,
		"Reverify protected GitHub controls before authority audit identity",
	)
	preBuildAuditEnv := requireObject(
		preBuildAudit,
		"env",
		"release.jobs.validate pre-build GitHub audit",
	)
	requireString(
		preBuildAuditEnv,
		"GH_TOKEN",
		"${{ secrets.PYMES_GITHUB_RELEASE_AUDIT_TOKEN }}",
		"release.jobs.validate pre-build GitHub audit env",
	)
	preBuildAuditRun := requireScalarString(
		preBuildAudit,
		"run",
		"release.jobs.validate pre-build GitHub audit",
	)
	if !strings.Contains(preBuildAuditRun, "verify-github-environments.sh all all-controls") ||
		strings.Contains(preBuildAuditRun, "environment-only") {
		fatalf("release pre-build GitHub audit must execute all-controls for both environments")
	}
	preBuildAuth := requireStep(validate, "Authenticate pre-build authority auditor")
	preBuildAuthWith := requireObject(
		preBuildAuth,
		"with",
		"release.jobs.validate pre-build authority auth",
	)
	requireString(
		preBuildAuthWith,
		"workload_identity_provider",
		"projects/884236221349/locations/global/workloadIdentityPools/pymes-v3-release-pool/providers/github",
		"release.jobs.validate pre-build authority auth",
	)
	requireString(
		preBuildAuthWith,
		"service_account",
		"pymes-v3-gh-deploy-${{ inputs.environment }}@pymes-dev-352318.iam.gserviceaccount.com",
		"release.jobs.validate pre-build authority auth",
	)
	preBuildAuthority := requireStep(
		validate,
		"Verify complete release authority before builder",
	)
	preBuildAuthorityEnv := requireObject(
		preBuildAuthority,
		"env",
		"release.jobs.validate pre-build authority verification",
	)
	requireString(
		preBuildAuthorityEnv,
		"PYMES_DEPLOY_ENV",
		"${{ inputs.environment }}",
		"release.jobs.validate pre-build authority verification env",
	)
	requireString(
		preBuildAuthorityEnv,
		"PYMES_DEPLOY_STAGE",
		"${{ inputs.deploy_stage }}",
		"release.jobs.validate pre-build authority verification env",
	)
	requireString(
		preBuildAuthorityEnv,
		"PYMES_GCP_PROJECT",
		"pymes-dev-352318",
		"release.jobs.validate pre-build authority verification env",
	)
	requireString(
		preBuildAuthorityEnv,
		"PYMES_GCP_REGION",
		"us-central1",
		"release.jobs.validate pre-build authority verification env",
	)
	preBuildAuthorityRun := requireScalarString(
		preBuildAuthority,
		"run",
		"release.jobs.validate pre-build authority verification",
	)
	if preBuildAuthorityRun != "./v3/scripts/deploy/verify-release-authority.sh" {
		fatalf("release pre-build authority verification must invoke the canonical verifier")
	}
	requireStepBefore(
		validate,
		"Reverify protected GitHub controls before authority audit identity",
		"Authenticate pre-build authority auditor",
	)
	requireStepBefore(
		validate,
		"Authenticate pre-build authority auditor",
		"Verify complete release authority before builder",
	)

	buildPush := requireStep(build, "Build and push release digests")
	buildEnv := requireObject(buildPush, "env", "release.jobs.build image build")
	requireFixedReleaseTarget(buildEnv, "release.jobs.build image build env")
	requireString(
		buildEnv,
		"VITE_CLERK_PUBLISHABLE_KEY",
		"${{ needs.validate.outputs.clerk_publishable_key }}",
		"release.jobs.build image build env",
	)

	buildAuth := requireStep(build, "Authenticate immutable image builder")
	buildAuthWith := requireObject(buildAuth, "with", "release.jobs.build auth")
	requireString(
		buildAuthWith,
		"workload_identity_provider",
		"projects/884236221349/locations/global/workloadIdentityPools/pymes-v3-release-pool/providers/github",
		"release.jobs.build auth",
	)
	requireStepBefore(
		build,
		"Reject standalone build reruns",
		"Authenticate immutable image builder",
	)
	buildIdentity := requireStep(build, "Validate isolated build identity")
	buildIdentityEnv := requireObject(
		buildIdentity,
		"env",
		"release.jobs.build identity validation",
	)
	requireFixedReleaseTarget(
		buildIdentityEnv,
		"release.jobs.build identity validation env",
	)
	requireString(
		buildIdentityEnv,
		"PYMES_GCP_PROJECT_NUMBER",
		"884236221349",
		"release.jobs.build identity validation env",
	)
	requireString(
		buildAuthWith,
		"service_account",
		"pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com",
		"release.jobs.build auth",
	)

	preDeployAudit := requireStep(deploy, "Reverify protected GitHub controls before deploy identity")
	preDeployAuditEnv := requireObject(
		preDeployAudit,
		"env",
		"release.jobs.deploy full GitHub audit",
	)
	requireString(
		preDeployAuditEnv,
		"GH_TOKEN",
		"${{ secrets.PYMES_GITHUB_RELEASE_AUDIT_TOKEN }}",
		"release.jobs.deploy full GitHub audit env",
	)
	preDeployAuditRun := requireScalarString(
		preDeployAudit,
		"run",
		"release.jobs.deploy full GitHub audit",
	)
	if !strings.Contains(preDeployAuditRun, "verify-github-environments.sh all all-controls") ||
		strings.Contains(preDeployAuditRun, "environment-only") {
		fatalf("release pre-deploy GitHub audit must execute all-controls for both environments")
	}

	deployAuth := requireStep(deploy, "Authenticate least-privilege deployer")
	deployAuthWith := requireObject(deployAuth, "with", "release.jobs.deploy auth")
	requireString(
		deployAuthWith,
		"workload_identity_provider",
		"projects/884236221349/locations/global/workloadIdentityPools/pymes-v3-release-pool/providers/github",
		"release.jobs.deploy auth",
	)
	requireString(
		deployAuthWith,
		"service_account",
		"pymes-v3-gh-deploy-${{ inputs.environment }}@pymes-dev-352318.iam.gserviceaccount.com",
		"release.jobs.deploy auth",
	)

	deployRelease := requireStep(deploy, "Deploy exact image digests")
	deployReleaseEnv := requireObject(
		deployRelease,
		"env",
		"release.jobs.deploy Cloud Run deployment",
	)
	requireString(
		deployReleaseEnv,
		"PYMES_DEPLOY_STAGE",
		"${{ inputs.deploy_stage }}",
		"release.jobs.deploy Cloud Run deployment env",
	)
	requireStepLast(deploy, "Deploy exact image digests")

	requireStepBefore(
		deploy,
		"Reverify protected GitHub controls before deploy identity",
		"Authenticate least-privilege deployer",
	)
	requireStepBefore(
		deploy,
		"Verify stage-scoped deployment authority",
		"Deploy exact image digests",
	)

	deployIdentity := requireStep(deploy, "Validate isolated deploy identity")
	deployIdentityEnv := requireObject(
		deployIdentity,
		"env",
		"release.jobs.deploy identity validation",
	)
	requireFixedReleaseTarget(
		deployIdentityEnv,
		"release.jobs.deploy identity validation env",
	)
	requireString(
		deployIdentityEnv,
		"PYMES_GCP_PROJECT_NUMBER",
		"884236221349",
		"release.jobs.deploy identity validation env",
	)
	deployAuthority := requireStep(deploy, "Verify stage-scoped deployment authority")
	deployAuthorityEnv := requireObject(
		deployAuthority,
		"env",
		"release.jobs.deploy authority verification",
	)
	requireString(
		deployAuthorityEnv,
		"PYMES_DEPLOY_ENV",
		"${{ inputs.environment }}",
		"release.jobs.deploy authority verification env",
	)
	requireString(
		deployAuthorityEnv,
		"PYMES_DEPLOY_STAGE",
		"${{ inputs.deploy_stage }}",
		"release.jobs.deploy authority verification env",
	)
	requireString(
		deployAuthorityEnv,
		"PYMES_GCP_PROJECT",
		"pymes-dev-352318",
		"release.jobs.deploy authority verification env",
	)
	requireString(
		deployAuthorityEnv,
		"PYMES_GCP_REGION",
		"us-central1",
		"release.jobs.deploy authority verification env",
	)
	for _, stepName := range []string{
		"Import allowlisted release manifest",
		"Verify provenance, materials, and SBOMs",
		"Deploy exact image digests",
	} {
		step := requireStep(deploy, stepName)
		stepEnv := requireObject(step, "env", "release.jobs.deploy "+stepName)
		requireString(
			stepEnv,
			"PYMES_GCP_PROJECT",
			"pymes-dev-352318",
			"release.jobs.deploy "+stepName+" env",
		)
		requireString(
			stepEnv,
			"PYMES_GCP_REGION",
			"us-central1",
			"release.jobs.deploy "+stepName+" env",
		)
	}
	for _, stepName := range []string{
		"Import allowlisted release manifest",
		"Verify provenance, materials, and SBOMs",
		"Deploy exact image digests",
	} {
		step := requireStep(deploy, stepName)
		stepEnv := requireObject(step, "env", "release.jobs.deploy "+stepName)
		requireString(
			stepEnv,
			"PYMES_ARTIFACT_REPOSITORY",
			"pymes",
			"release.jobs.deploy "+stepName+" env",
		)
	}

	upload := requireStep(build, "Upload digest manifest")
	uploadWith := requireObject(upload, "with", "release.jobs.build artifact upload")
	retention, ok := uploadWith["retention-days"].(int)
	if !ok || retention != 90 {
		fatalf("release artifact retention must be exactly 90 days")
	}
	manifestChecksum := requireStep(build, "Record immutable digest manifest checksum")
	manifestChecksumEnv := requireObject(
		manifestChecksum,
		"env",
		"release.jobs.build manifest checksum",
	)
	requireString(
		manifestChecksumEnv,
		"MANIFEST_FILE",
		"${{ runner.temp }}/pymes-v3-images.env",
		"release.jobs.build manifest checksum env",
	)
	manifestChecksumRun := requireScalarString(
		manifestChecksum,
		"run",
		"release.jobs.build manifest checksum",
	)
	if !strings.Contains(manifestChecksumRun, "sha256sum -- \"${MANIFEST_FILE}\"") ||
		!strings.Contains(manifestChecksumRun, "GITHUB_STEP_SUMMARY") {
		fatalf("release manifest checksum must be recorded independently in the build job summary")
	}
	requireStepBefore(
		build,
		"Build and push release digests",
		"Record immutable digest manifest checksum",
	)
	requireStepBefore(
		build,
		"Record immutable digest manifest checksum",
		"Upload digest manifest",
	)
}

func requireFixedReleaseTarget(environment object, path string) {
	requireString(
		environment,
		"PYMES_GCP_PROJECT",
		"pymes-dev-352318",
		path,
	)
	requireString(
		environment,
		"PYMES_GCP_REGION",
		"us-central1",
		path,
	)
	requireString(
		environment,
		"PYMES_ARTIFACT_REPOSITORY",
		"pymes",
		path,
	)
}

func requireStepBefore(job object, firstName, secondName string) {
	rawSteps, ok := job["steps"].([]any)
	if !ok {
		fatalf("job with ordered steps %q and %q has no steps sequence", firstName, secondName)
	}
	firstIndex := -1
	secondIndex := -1
	for index, rawStep := range rawSteps {
		step := mustObject(rawStep, fmt.Sprintf("steps[%d]", index))
		switch stepName, _ := step["name"].(string); stepName {
		case firstName:
			firstIndex = index
		case secondName:
			secondIndex = index
		}
	}
	if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
		fatalf("step %q must execute before %q", firstName, secondName)
	}
}

func requireStepLast(job object, name string) {
	rawSteps, ok := job["steps"].([]any)
	if !ok || len(rawSteps) == 0 {
		fatalf("job with final step %q has no steps sequence", name)
	}
	last := mustObject(rawSteps[len(rawSteps)-1], fmt.Sprintf(
		"steps[%d]",
		len(rawSteps)-1,
	))
	if lastName, _ := last["name"].(string); lastName != name {
		fatalf(
			"step %q must be the final job step so verification remains inside its rollback transaction",
			name,
		)
	}
}

func validateRerunGuard(job object, name string, mustBeFirst bool) error {
	rawSteps, ok := job["steps"].([]any)
	if !ok || len(rawSteps) == 0 {
		return fmt.Errorf("job has no steps sequence")
	}
	matchIndex := -1
	var match object
	for index, rawStep := range rawSteps {
		step, ok := rawStep.(object)
		if !ok {
			return fmt.Errorf("step %d must be a mapping", index)
		}
		if stepName, _ := step["name"].(string); stepName == name {
			if match != nil {
				return fmt.Errorf("expected exactly one step named %q", name)
			}
			match = step
			matchIndex = index
		}
	}
	if match == nil {
		return fmt.Errorf("missing step %q", name)
	}
	if mustBeFirst && matchIndex != 0 {
		return fmt.Errorf("step %q must be the first job step", name)
	}
	run, ok := match["run"].(string)
	if !ok {
		return fmt.Errorf("step %q must contain a scalar run script", name)
	}
	if !strings.Contains(run, `[[ "${GITHUB_RUN_ATTEMPT}" == "1" ]]`) {
		return fmt.Errorf("step %q does not reject GITHUB_RUN_ATTEMPT != 1", name)
	}
	if !strings.Contains(run, "dispatch a new release workflow") {
		return fmt.Errorf("step %q does not require a fresh release dispatch", name)
	}
	return nil
}

func requireStep(job object, name string) object {
	rawSteps, ok := job["steps"].([]any)
	if !ok {
		fatalf("job with step %q has no steps sequence", name)
	}
	var matches []object
	for index, rawStep := range rawSteps {
		step := mustObject(rawStep, fmt.Sprintf("steps[%d]", index))
		if stepName, _ := step["name"].(string); stepName == name {
			matches = append(matches, step)
		}
	}
	if len(matches) != 1 {
		fatalf("expected exactly one step named %q, got %d", name, len(matches))
	}
	return matches[0]
}

func requireEnvironment(job object, path, expected string) {
	raw, exists := job["environment"]
	if !exists {
		fatalf("%s must use protected environment %s", path, expected)
	}
	switch environment := raw.(type) {
	case string:
		if environment != expected {
			fatalf("%s environment is %q, expected %q", path, environment, expected)
		}
	case object:
		requireExactKeys(environment, path+".environment", "name")
		requireString(environment, "name", expected, path+".environment")
	default:
		fatalf("%s has invalid environment structure", path)
	}
}

func requireNeeds(job object, path string, expected ...string) {
	raw, exists := job["needs"]
	if !exists {
		fatalf("%s has no needs dependency", path)
	}
	var actual []string
	switch needs := raw.(type) {
	case string:
		actual = []string{needs}
	case []any:
		for _, item := range needs {
			value, ok := item.(string)
			if !ok {
				fatalf("%s needs contains a non-string value", path)
			}
			actual = append(actual, value)
		}
	default:
		fatalf("%s has invalid needs structure", path)
	}
	requireExactStrings(actual, path+".needs", expected...)
}

func requirePermissions(container object, path string, expected map[string]string) {
	permissions := requireObject(container, "permissions", path)
	expectedKeys := make([]string, 0, len(expected))
	for key := range expected {
		expectedKeys = append(expectedKeys, key)
	}
	requireExactKeys(permissions, path+".permissions", expectedKeys...)
	for key, value := range expected {
		requireString(permissions, key, value, path+".permissions")
	}
}

func requireObject(container object, key, path string) object {
	value, exists := container[key]
	if !exists {
		fatalf("%s is missing %q", path, key)
	}
	return mustObject(value, path+"."+key)
}

func mustObject(value any, path string) object {
	result, ok := value.(object)
	if !ok {
		fatalf("%s must be a mapping", path)
	}
	return result
}

func requireString(container object, key, expected, path string) {
	actual := requireScalarString(container, key, path)
	if actual != expected {
		fatalf("%s.%s is %q, expected %q", path, key, actual, expected)
	}
}

func requireBool(container object, key string, expected bool, path string) {
	value, exists := container[key]
	if !exists {
		fatalf("%s is missing %q", path, key)
	}
	actual, ok := value.(bool)
	if !ok {
		fatalf("%s.%s must be a boolean", path, key)
	}
	if actual != expected {
		fatalf("%s.%s is %t, expected %t", path, key, actual, expected)
	}
}

func requireScalarString(container object, key, path string) string {
	value, exists := container[key]
	if !exists {
		fatalf("%s is missing %q", path, key)
	}
	actual, ok := value.(string)
	if !ok {
		fatalf("%s.%s must be a string", path, key)
	}
	return actual
}

func requireStrings(container object, key, path string) []string {
	raw, exists := container[key]
	if !exists {
		fatalf("%s is missing %q", path, key)
	}
	values, ok := raw.([]any)
	if !ok {
		fatalf("%s.%s must be a sequence", path, key)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		item, ok := value.(string)
		if !ok {
			fatalf("%s.%s contains a non-string value", path, key)
		}
		result = append(result, item)
	}
	return result
}

func requireExactKeys(value object, path string, expected ...string) {
	actual := make([]string, 0, len(value))
	for key := range value {
		actual = append(actual, key)
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
		fatalf("%s keys are %v, expected %v", path, actual, expected)
	}
}

func requireExactStrings(actual []string, path string, expected ...string) {
	sort.Strings(actual)
	sort.Strings(expected)
	if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
		fatalf("%s values are %v, expected %v", path, actual, expected)
	}
}
