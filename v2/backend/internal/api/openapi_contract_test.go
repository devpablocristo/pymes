package api

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestIAMOpenAPIContract(t *testing.T) {
	t.Parallel()

	spec, err := GetSpec()
	if err != nil {
		t.Fatalf("load embedded OpenAPI spec: %v", err)
	}
	if err := spec.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI spec: %v", err)
	}

	assertIAMOperations(t, spec)
	assertSecurityBoundaries(t, spec)
	assertMutationIdempotency(t, spec)
	assertNoClientSelectedTenant(t, spec)
	assertRoleAndLifecycleSchemas(t, spec)
}

func assertIAMOperations(t *testing.T, spec *openapi3.T) {
	t.Helper()

	expected := map[string]string{
		http.MethodGet + " /api/v1/runtime-config":                           "GetRuntimeConfig",
		http.MethodGet + " /api/v1/session":                                  "GetCurrentSession",
		http.MethodGet + " /api/v1/organizations":                            "ListMyOrganizations",
		http.MethodPatch + " /api/v1/organization":                           "UpdateCurrentOrganization",
		http.MethodGet + " /api/v1/sessions":                                 "ListMySessions",
		http.MethodDelete + " /api/v1/sessions/{session_id}":                 "RevokeMySession",
		http.MethodGet + " /api/v1/team/members":                             "ListTeamMembers",
		http.MethodPatch + " /api/v1/team/members/{member_id}":               "UpdateTeamMember",
		http.MethodDelete + " /api/v1/team/members/{member_id}":              "RemoveTeamMember",
		http.MethodPost + " /api/v1/team/ownership-transfer":                 "TransferOwnership",
		http.MethodGet + " /api/v1/team/invitations":                         "ListTeamInvitations",
		http.MethodPost + " /api/v1/team/invitations":                        "CreateTeamInvitation",
		http.MethodPost + " /api/v1/team/invitations/{invitation_id}/resend": "ResendTeamInvitation",
		http.MethodPost + " /api/v1/team/invitations/{invitation_id}/revoke": "RevokeTeamInvitation",
		http.MethodPost + " /webhooks/clerk":                                 "ReceiveClerkWebhook",
	}

	for route, operationID := range expected {
		method, path, ok := strings.Cut(route, " ")
		if !ok {
			t.Fatalf("invalid test route %q", route)
		}
		pathItem := spec.Paths.Value(path)
		if pathItem == nil {
			t.Errorf("missing path %s", path)
			continue
		}
		operation := pathItem.GetOperation(method)
		if operation == nil {
			t.Errorf("missing operation %s %s", method, path)
			continue
		}
		if operation.OperationID != operationID {
			t.Errorf("%s %s operationId = %q, want %q", method, path, operation.OperationID, operationID)
		}
	}
}

func assertSecurityBoundaries(t *testing.T, spec *openapi3.T) {
	t.Helper()

	if len(spec.Security) != 1 {
		t.Fatalf("global security requirement count = %d, want 1", len(spec.Security))
	}
	if _, ok := spec.Security[0]["bearerAuth"]; !ok {
		t.Fatal("global security must require bearerAuth")
	}

	runtimeConfig := operationAt(t, spec, http.MethodGet, "/api/v1/runtime-config")
	if runtimeConfig.Security == nil || len(*runtimeConfig.Security) != 0 {
		t.Fatal("runtime config must explicitly opt out of authentication")
	}

	webhook := operationAt(t, spec, http.MethodPost, "/webhooks/clerk")
	if webhook.Security == nil || len(*webhook.Security) != 1 {
		t.Fatal("Clerk webhook must override bearerAuth with one signature requirement")
	}
	signatureRequirement := (*webhook.Security)[0]
	for _, name := range []string{"svixId", "svixTimestamp", "svixSignature"} {
		if _, ok := signatureRequirement[name]; !ok {
			t.Errorf("Clerk webhook security is missing %s", name)
		}
	}
	if _, ok := signatureRequirement["bearerAuth"]; ok {
		t.Error("Clerk webhook must not require bearerAuth")
	}
	webhookUnauthorized := webhook.Responses.Value("401")
	if webhookUnauthorized == nil || webhookUnauthorized.Value == nil ||
		webhookUnauthorized.Value.Description == nil ||
		!strings.Contains(*webhookUnauthorized.Value.Description, "WEBHOOK_INVALID_SIGNATURE") {
		t.Error("Clerk webhook 401 must use its signature-specific stable error")
	}

	for path, pathItem := range spec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation == runtimeConfig || operation == webhook {
				continue
			}
			if operation.Security != nil {
				t.Errorf("%s %s unexpectedly overrides global bearer security", method, path)
			}
		}
	}
}

func assertMutationIdempotency(t *testing.T, spec *openapi3.T) {
	t.Helper()

	for path, pathItem := range spec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation.OperationID == "ReceiveClerkWebhook" || !isMutation(method) {
				continue
			}
			parameter := findParameter(operation.Parameters, "Idempotency-Key")
			if parameter == nil {
				t.Errorf("%s %s is missing Idempotency-Key", method, path)
				continue
			}
			if !parameter.Required || parameter.In != openapi3.ParameterInHeader {
				t.Errorf("%s %s Idempotency-Key must be a required header", method, path)
			}
		}
	}
}

func assertNoClientSelectedTenant(t *testing.T, spec *openapi3.T) {
	t.Helper()

	for path, pathItem := range spec.Paths.Map() {
		assertParametersHaveNoTenantSelector(t, "path "+path, pathItem.Parameters)
		for method, operation := range pathItem.Operations() {
			assertParametersHaveNoTenantSelector(t, method+" "+path, operation.Parameters)
			if operation.RequestBody == nil || operation.RequestBody.Value == nil {
				continue
			}
			mediaType := operation.RequestBody.Value.Content.Get("application/json")
			if mediaType == nil {
				continue
			}
			assertSchemaHasNoTenantSelector(t, method+" "+path, mediaType.Schema, map[*openapi3.Schema]bool{})
		}
	}
}

func assertParametersHaveNoTenantSelector(t *testing.T, route string, parameters openapi3.Parameters) {
	t.Helper()
	for _, parameterRef := range parameters {
		if parameterRef == nil || parameterRef.Value == nil {
			continue
		}
		name := strings.ToLower(strings.ReplaceAll(parameterRef.Value.Name, "-", "_"))
		if slices.Contains([]string{
			"org_id",
			"organization_id",
			"tenant_id",
			"x_organization_id",
			"x_tenant_id",
		}, name) {
			t.Errorf("%s accepts forbidden tenant selector %q", route, parameterRef.Value.Name)
		}
	}
}

func assertSchemaHasNoTenantSelector(
	t *testing.T,
	route string,
	schemaRef *openapi3.SchemaRef,
	visited map[*openapi3.Schema]bool,
) {
	t.Helper()
	if schemaRef == nil || schemaRef.Value == nil || visited[schemaRef.Value] {
		return
	}
	schema := schemaRef.Value
	visited[schema] = true

	for name, property := range schema.Properties {
		normalized := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
		if slices.Contains([]string{"org_id", "organization_id", "tenant_id"}, normalized) {
			t.Errorf("%s request body accepts forbidden tenant selector %q", route, name)
		}
		assertSchemaHasNoTenantSelector(t, route, property, visited)
	}
	assertSchemaHasNoTenantSelector(t, route, schema.Items, visited)
	for _, nested := range append(append(schema.AllOf, schema.AnyOf...), schema.OneOf...) {
		assertSchemaHasNoTenantSelector(t, route, nested, visited)
	}
}

func assertRoleAndLifecycleSchemas(t *testing.T, spec *openapi3.T) {
	t.Helper()

	assignableRoles := enumStrings(t, spec, "AssignableRole")
	if !slices.Equal(assignableRoles, []string{"admin", "member"}) {
		t.Errorf("AssignableRole = %v, want [admin member]", assignableRoles)
	}

	membershipStatuses := enumStrings(t, spec, "MembershipStatus")
	if !slices.Contains(membershipStatuses, "pending") {
		t.Error("MembershipStatus must include pending for invitation provisioning")
	}

	invitationStatuses := enumStrings(t, spec, "InvitationStatus")
	for _, syncOnly := range []string{"queued", "failed", "revoke_pending"} {
		if slices.Contains(invitationStatuses, syncOnly) {
			t.Errorf("InvitationStatus must not mix synchronization state %q into its lifecycle", syncOnly)
		}
	}

	permissions := enumStrings(t, spec, "Permission")
	if len(permissions) != 9 || !slices.Contains(permissions, "sessions:manage:self") ||
		!slices.Contains(permissions, "team:ownership:transfer") {
		t.Errorf("Permission does not expose the fixed IAM matrix: %v", permissions)
	}

	currentSession := schemaAt(t, spec, "CurrentSession")
	if !slices.Contains(currentSession.Required, "role") {
		t.Error("CurrentSession must expose the effective role separately from the local membership")
	}
}

func operationAt(t *testing.T, spec *openapi3.T, method, path string) *openapi3.Operation {
	t.Helper()
	pathItem := spec.Paths.Value(path)
	if pathItem == nil {
		t.Fatalf("missing path %s", path)
	}
	operation := pathItem.GetOperation(method)
	if operation == nil {
		t.Fatalf("missing operation %s %s", method, path)
	}
	return operation
}

func findParameter(parameters openapi3.Parameters, name string) *openapi3.Parameter {
	for _, parameterRef := range parameters {
		if parameterRef != nil && parameterRef.Value != nil && parameterRef.Value.Name == name {
			return parameterRef.Value
		}
	}
	return nil
}

func enumStrings(t *testing.T, spec *openapi3.T, name string) []string {
	t.Helper()
	schema := schemaAt(t, spec, name)
	values := make([]string, 0, len(schema.Enum))
	for _, value := range schema.Enum {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("%s enum contains non-string value %T", name, value)
		}
		values = append(values, text)
	}
	return values
}

func schemaAt(t *testing.T, spec *openapi3.T, name string) *openapi3.Schema {
	t.Helper()
	schemaRef := spec.Components.Schemas[name]
	if schemaRef == nil || schemaRef.Value == nil {
		t.Fatalf("missing schema %s", name)
	}
	return schemaRef.Value
}

func isMutation(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
