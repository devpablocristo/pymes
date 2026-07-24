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
	assertBusinessOperations(t, spec)
	assertSecurityBoundaries(t, spec)
	assertMutationIdempotency(t, spec)
	assertNoClientSelectedTenant(t, spec)
	assertRoleAndLifecycleSchemas(t, spec)
}

func assertBusinessOperations(t *testing.T, spec *openapi3.T) {
	t.Helper()

	expected := map[string]string{
		http.MethodGet + " /api/v1/accounting/accounts":                            "ListAccountingAccounts",
		http.MethodPost + " /api/v1/accounting/accounts":                           "CreateAccountingAccount",
		http.MethodPost + " /api/v1/accounting/periods":                            "CreateAccountingPeriod",
		http.MethodPost + " /api/v1/accounting/statement-imports":                  "ImportAccountingStatement",
		http.MethodGet + " /api/v1/accounting/reports/{report}/export":             "ExportAccountingReport",
		http.MethodPost + " /api/v1/accounting/drafts/{draft_id}/post":             "PostJournalDraft",
		http.MethodPost + " /api/v1/accounting/journal-entries/{entry_id}/reverse": "ReverseJournalEntry",
		http.MethodGet + " /api/v1/accounting/reports/{report}":                    "GetAccountingReport",
		http.MethodGet + " /api/v1/team/members/{member_id}/business-permissions":  "GetTeamMemberBusinessPermissions",
		http.MethodPut + " /api/v1/team/members/{member_id}/business-permissions":  "UpdateTeamMemberBusinessPermissions",
		http.MethodPut + " /api/v1/fiscal/settings":                                "UpdateFiscalSettings",
		http.MethodGet + " /api/v1/fiscal/homologation/latest":                     "GetLatestFiscalHomologation",
		http.MethodPost + " /api/v1/fiscal/production/enable":                      "EnableFiscalProduction",
		http.MethodPost + " /api/v1/fiscal/purchase-vouchers":                      "CreateFiscalPurchaseVoucher",
		http.MethodPost + " /api/v1/fiscal/vouchers":                               "CreateFiscalVoucher",
		http.MethodGet + " /api/v1/fiscal/vouchers/{voucher_id}/pdf":               "GetFiscalVoucherPDF",
		http.MethodPost + " /api/v1/fiscal/credit-notes":                           "CreateFiscalCreditNote",
		http.MethodPost + " /api/v1/fiscal/debit-notes":                            "CreateFiscalDebitNote",
		http.MethodGet + " /api/v1/fiscal/iva-simple/{period}":                     "GetIVASimple",
	}
	for route, operationID := range expected {
		method, path, _ := strings.Cut(route, " ")
		operation := operationAt(t, spec, method, path)
		if operation.OperationID != operationID {
			t.Errorf("%s %s operationId = %q, want %q", method, path, operation.OperationID, operationID)
		}
	}
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
		if strings.HasPrefix(path, "/api/v1/admin/") {
			// Global-owner administration is the only boundary allowed to
			// address a tenant explicitly. Its handlers verify the owner
			// against the local authority before opening the transaction.
			continue
		}
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
	if len(permissions) != 12 || !slices.Contains(permissions, "sessions:manage:self") ||
		!slices.Contains(permissions, "accounting:manage") ||
		!slices.Contains(permissions, "fiscal:manage") ||
		slices.Contains(permissions, "team:ownership:transfer") {
		t.Errorf("Permission does not expose the fixed IAM matrix: %v", permissions)
	}
	delegated := enumStrings(t, spec, "DelegatedBusinessPermission")
	if !slices.Equal(delegated, []string{"accounting:manage", "fiscal:manage"}) {
		t.Errorf("DelegatedBusinessPermission = %v", delegated)
	}

	fiscalVoucherInput := schemaAt(t, spec, "FiscalVoucherInput")
	sourceType := fiscalVoucherInput.Properties["source_type"]
	if sourceType == nil || sourceType.Value == nil ||
		len(sourceType.Value.Enum) != 1 || sourceType.Value.Enum[0] != "sale" {
		t.Errorf("FiscalVoucherInput source_type must accept outbound sales only")
	}
	if !slices.Contains(fiscalVoucherInput.Required, "environment") {
		t.Error("FiscalVoucherInput must select its fiscal environment explicitly")
	}
	fiscalVoucher := schemaAt(t, spec, "FiscalVoucher")
	if !slices.Contains(fiscalVoucher.Required, "environment") {
		t.Error("FiscalVoucher must expose its isolated fiscal environment")
	}
	settingsInput := schemaAt(t, spec, "ArgentinaFiscalSettingsInput")
	if !slices.Contains(settingsInput.Required, "activity_start_date") {
		t.Error("Argentina fiscal settings must require the activity start date")
	}
	purchaseInput := schemaAt(t, spec, "FiscalPurchaseVoucherInput")
	if purchaseInput.Properties["associated_purchase_voucher_id"] == nil {
		t.Error("Purchase adjustments must be able to reference their original voucher")
	}
	listVouchers := operationAt(t, spec, http.MethodGet, "/api/v1/fiscal/vouchers")
	environment := findParameter(listVouchers.Parameters, "environment")
	if environment == nil || !environment.Required {
		t.Error("Fiscal voucher listings must require an explicit environment")
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
