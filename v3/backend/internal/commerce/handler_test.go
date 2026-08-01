package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
	"github.com/google/uuid"
)

type commerceStub struct {
	Commerce
	readyErr error
}

func (s commerceStub) Ready(context.Context) error { return s.readyErr }

func TestReadinessReflectsCommerceDependency(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "ready", status: http.StatusOK},
		{name: "database unavailable", err: errors.New("database unavailable"), status: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			NewHTTPServer(commerceStub{readyErr: test.err}, nil).Handler().ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, recorder.Code)
			}
		})
	}
}

type organizationAuthStub struct {
	organizationID string
	principal      identitydomain.Principal
	err            error
}

func (s organizationAuthStub) Principal(*http.Request) (identitydomain.Principal, error) {
	if s.err != nil {
		return identitydomain.Principal{}, s.err
	}
	if s.principal.OrganizationID != "" {
		return s.principal, nil
	}
	return identitydomain.Principal{
		OrganizationID: s.organizationID, ActorID: "user_test", Role: identitydomain.RoleAdmin,
		OrganizationStatus: "ready", MembershipStatus: "active",
	}, nil
}

type mutationCommerceStub struct {
	commerceStub
	commands   []domain.IdempotencyCommand
	principals []identitydomain.Principal
	err        error
	now        time.Time
	failure    domain.AccountingFailure
	adjustment domain.AccountingAdjustment
	sale       domain.Sale
}

func (s *mutationCommerceStub) capture(ctx context.Context, command domain.IdempotencyCommand) {
	s.commands = append(s.commands, command)
	principal, _ := identityusecases.PrincipalFromContext(ctx)
	s.principals = append(s.principals, principal)
}

func (s *mutationCommerceStub) CreatePartyIdempotent(ctx context.Context, command domain.IdempotencyCommand, value domain.Party) (domain.Party, error) {
	s.capture(ctx, command)
	return value, s.err
}

func (s *mutationCommerceStub) CreatePurchaseAndQueueIdempotent(ctx context.Context, command domain.IdempotencyCommand, value domain.Purchase) (domain.Purchase, error) {
	s.capture(ctx, command)
	value.Status = "confirmed"
	return value, s.err
}

func (s *mutationCommerceStub) CreatePaymentAndApplicationsIdempotent(ctx context.Context, command domain.IdempotencyCommand, value domain.Payment, _ []domain.OpenItemApplication) (domain.Payment, error) {
	s.capture(ctx, command)
	value.Status = "confirmed"
	return value, s.err
}

func (s *mutationCommerceStub) CreateSaleAndQueueFiscalIdempotent(ctx context.Context, command domain.IdempotencyCommand, value domain.Sale, _ string) (domain.Sale, error) {
	s.capture(ctx, command)
	s.sale = value
	value.Voucher.VoucherNumber = 1
	value.SnapshotDigest = strings.Repeat("a", 64)
	return value, s.err
}

func TestCreateSaleFreezesCurrencyAndExactExchangeRate(t *testing.T) {
	t.Parallel()
	commerce := &mutationCommerceStub{}
	server := NewHTTPServer(commerce, organizationAuthStub{organizationID: "org_test"}).Handler()
	body := `{"id":"sale_usd","recipient_ref":"party_1","point_of_sale":1,"document_type":"FA","amount":"12.10","currency":"USD","exchange_rate":"1234.567890","credential_ref":"credential_1","fiscal":{"environment":"homologation","issue_date":"2026-07-31","totals":{"net":"10.00","vat":"2.10","exempt":"0","total":"12.10"},"recipient":{},"lines":[{}]}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org_test/sales", strings.NewReader(body))
	request.Header.Set("Idempotency-Key", "sale-usd")
	request.Header.Set("X-Source-Version", "1")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var snapshot map[string]any
	if err := json.Unmarshal(commerce.sale.FiscalSnapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["currency"] != "USD" || snapshot["exchange_rate"] != "1234.567890" {
		t.Fatalf("snapshot=%s", commerce.sale.FiscalSnapshot)
	}
}

func TestCreateSaleRejectsMissingOrInvalidExchangeRate(t *testing.T) {
	t.Parallel()
	tests := []string{
		`{"id":"sale_usd","recipient_ref":"party_1","point_of_sale":1,"document_type":"FA","amount":"12.10","currency":"USD","credential_ref":"credential_1","fiscal":{"environment":"homologation","issue_date":"2026-07-31","totals":{},"recipient":{},"lines":[{}]}}`,
		`{"id":"sale_usd","recipient_ref":"party_1","point_of_sale":1,"document_type":"FA","amount":"12.10","currency":"USD","exchange_rate":"0","credential_ref":"credential_1","fiscal":{"environment":"homologation","issue_date":"2026-07-31","totals":{},"recipient":{},"lines":[{}]}}`,
	}
	for _, body := range tests {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org_test/sales", strings.NewReader(body))
		request.Header.Set("Idempotency-Key", "sale-usd")
		request.Header.Set("X-Source-Version", "1")
		recorder := httptest.NewRecorder()
		server := NewHTTPServer(&mutationCommerceStub{}, organizationAuthStub{organizationID: "org_test"}).Handler()
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	}
}

func (s *mutationCommerceStub) CreateAccountingReversalIdempotent(ctx context.Context, command domain.IdempotencyCommand, value domain.AccountingReversal) (domain.AccountingReversal, error) {
	s.capture(ctx, command)
	value.OriginalJournalEntryID = "journal_1"
	return value, s.err
}

func (s *mutationCommerceStub) GetAccountingFailure(
	_ context.Context,
	organizationID, failureID string,
) (domain.AccountingFailure, error) {
	if s.err != nil {
		return domain.AccountingFailure{}, s.err
	}
	value := s.failure
	value.ID, value.OrganizationID = failureID, organizationID
	return value, nil
}

func (s *mutationCommerceStub) RequestAccountingAdjustmentIdempotent(
	ctx context.Context,
	command domain.IdempotencyCommand,
	_ string,
	value domain.AccountingAdjustment,
) (domain.AccountingAdjustment, error) {
	s.capture(ctx, command)
	value.Status = "pending"
	value.CreatedAt, value.UpdatedAt = s.Clock(), s.Clock()
	s.adjustment = value
	return value, s.err
}

func (s *mutationCommerceStub) GetParty(_ context.Context, organizationID, id string) (domain.Party, error) {
	return domain.Party{ID: id, OrganizationID: organizationID, Kind: "customer", DisplayName: "Alice"}, nil
}

func (s *mutationCommerceStub) GetPurchase(_ context.Context, organizationID, id string) (domain.Purchase, error) {
	return domain.Purchase{ID: id, OrganizationID: organizationID}, nil
}

func (s *mutationCommerceStub) GetPayment(_ context.Context, organizationID, id string) (domain.Payment, error) {
	return domain.Payment{ID: id, OrganizationID: organizationID}, nil
}

func (s *mutationCommerceStub) GetSale(_ context.Context, organizationID, id string) (domain.Sale, error) {
	return domain.Sale{ID: id, OrganizationID: organizationID}, nil
}

func (s *mutationCommerceStub) Clock() time.Time {
	if s.now.IsZero() {
		return time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	}
	return s.now
}

func publicMutationRequests() []struct {
	name string
	path string
	body string
} {
	return []struct {
		name string
		path string
		body string
	}{
		{name: "party", path: "/api/v1/organizations/org_test/parties", body: `{"id":"party_1","kind":"customer","display_name":"Alice"}`},
		{name: "sale", path: "/api/v1/organizations/org_test/sales", body: `{"id":"sale_1","recipient_ref":"party_1","point_of_sale":1,"document_type":"FA","amount":"121.00","currency":"ARS","credential_ref":"credential_1","fiscal":{"environment":"homologation","issue_date":"2026-07-31","totals":{},"recipient":{},"lines":[{}]}}`},
		{name: "purchase", path: "/api/v1/organizations/org_test/purchases", body: `{"id":"purchase_1","supplier_ref":"supplier_1","external_document_ref":"invoice_1","issue_date":"2026-07-31","amount":"100.00","currency":"ARS","net_amount":"0","exempt_amount":"100.00","vat_breakdown":[]}`},
		{name: "payment", path: "/api/v1/organizations/org_test/payments", body: `{"id":"payment_1","direction":"receipt","party_ref":"party_1","amount":"50.00","currency":"ARS","applications":[]}`},
		{name: "reversal", path: "/api/v1/organizations/org_test/reversals", body: `{"id":"reversal_1","document_kind":"purchase","document_id":"purchase_1","effective_at":"2026-07-31T12:00:00Z","reason":"supplier cancellation"}`},
	}
}

func TestPublicMutationsRequireOwnerOrAdminAndReadyOrganization(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		role             identitydomain.Role
		membershipStatus string
		organization     string
		wantStatus       int
		wantCode         string
	}{
		{name: "owner", role: identitydomain.RoleOwner, membershipStatus: "active", organization: "ready", wantStatus: http.StatusCreated},
		{name: "admin", role: identitydomain.RoleAdmin, membershipStatus: "active", organization: "ready", wantStatus: http.StatusCreated},
		{name: "member is read only", role: identitydomain.RoleMember, membershipStatus: "active", organization: "ready", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "viewer is read only", role: identitydomain.RoleViewer, membershipStatus: "active", organization: "ready", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "inactive owner", role: identitydomain.RoleOwner, membershipStatus: "inactive", organization: "ready", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "pending organization", role: identitydomain.RoleOwner, membershipStatus: "active", organization: "pending", wantStatus: http.StatusUnprocessableEntity, wantCode: "ORG_NOT_PROVISIONED"},
		{name: "failed organization", role: identitydomain.RoleAdmin, membershipStatus: "active", organization: "failed", wantStatus: http.StatusUnprocessableEntity, wantCode: "ORG_NOT_PROVISIONED"},
		{name: "suspended organization", role: identitydomain.RoleOwner, membershipStatus: "active", organization: "suspended", wantStatus: http.StatusUnprocessableEntity, wantCode: "ORG_NOT_PROVISIONED"},
	}
	for _, authCase := range cases {
		authCase := authCase
		t.Run(authCase.name, func(t *testing.T) {
			t.Parallel()
			for _, endpoint := range publicMutationRequests() {
				endpoint := endpoint
				t.Run(endpoint.name, func(t *testing.T) {
					t.Parallel()
					commerce := &mutationCommerceStub{}
					auth := organizationAuthStub{principal: identitydomain.Principal{
						OrganizationID: "org_test", ActorID: "user_test", Role: authCase.role,
						MembershipStatus: authCase.membershipStatus, OrganizationStatus: authCase.organization,
					}}
					request := httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(endpoint.body))
					request.Header.Set("Idempotency-Key", authCase.name+"-"+endpoint.name)
					request.Header.Set("X-Source-Version", "1")
					recorder := httptest.NewRecorder()
					NewHTTPServer(commerce, auth).Handler().ServeHTTP(recorder, request)
					if recorder.Code != authCase.wantStatus || (authCase.wantCode != "" && !strings.Contains(recorder.Body.String(), authCase.wantCode)) {
						t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
					}
					wantCommands := 0
					if authCase.wantStatus == http.StatusCreated {
						wantCommands = 1
					}
					if len(commerce.commands) != wantCommands {
						t.Fatalf("commands=%d want=%d", len(commerce.commands), wantCommands)
					}
					if wantCommands == 1 && (len(commerce.principals) != 1 || commerce.principals[0].ActorID != "user_test") {
						t.Fatalf("principal was not propagated: %+v", commerce.principals)
					}
				})
			}
		})
	}
}

func TestMemberAndViewerCanReadTheirOrganization(t *testing.T) {
	t.Parallel()
	paths := []string{
		"/api/v1/organizations/org_test/parties/party_1",
		"/api/v1/organizations/org_test/sales/sale_1",
		"/api/v1/organizations/org_test/purchases/purchase_1",
		"/api/v1/organizations/org_test/payments/payment_1",
	}
	for _, role := range []identitydomain.Role{identitydomain.RoleMember, identitydomain.RoleViewer} {
		role := role
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()
			for _, path := range paths {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				recorder := httptest.NewRecorder()
				auth := organizationAuthStub{principal: identitydomain.Principal{
					OrganizationID: "org_test", ActorID: "user_test", Role: role,
					MembershipStatus: "active", OrganizationStatus: "ready",
				}}
				NewHTTPServer(&mutationCommerceStub{}, auth).Handler().ServeHTTP(recorder, request)
				if recorder.Code != http.StatusOK {
					t.Fatalf("path=%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
				}
			}
		})
	}
}

func TestAccountingAdjustmentIsExplicitAuthorizedAndCarriesIdempotentOrigin(t *testing.T) {
	failureID := uuid.NewString()
	effectiveAt := "2026-08-03T00:00:00Z"
	commerce := &mutationCommerceStub{}
	auth := organizationAuthStub{principal: identitydomain.Principal{
		OrganizationID: "org_test", ActorID: "admin_test", Role: identitydomain.RoleAdmin,
		OrganizationStatus: "ready", MembershipStatus: "active",
	}}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/organizations/org_test/accounting-failures/"+failureID+"/adjustments",
		strings.NewReader(`{"id":"adjustment_1","effective_at":"`+effectiveAt+`","reason":"post in authorized open period"}`),
	)
	request.Header.Set("Idempotency-Key", "period-adjustment-1")
	request.Header.Set("X-Source-Version", "4")
	request.Header.Set("X-Request-ID", "request-adjustment-1")
	request.Header.Set("X-Correlation-ID", "correlation-adjustment-1")
	recorder := httptest.NewRecorder()
	NewHTTPServer(commerce, auth).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(commerce.commands) != 1 {
		t.Fatalf("commands=%d", len(commerce.commands))
	}
	command := commerce.commands[0]
	if command.Operation != domain.OperationCreateAccountingAdjustment ||
		command.SourceID != "adjustment_1" ||
		command.SourceVersion != 4 ||
		command.RequestID != "request-adjustment-1" ||
		command.CorrelationID != "correlation-adjustment-1" ||
		command.ActorRef != "admin_test" {
		t.Fatalf("command=%+v", command)
	}
	if commerce.adjustment.FailureID != failureID ||
		commerce.adjustment.OrganizationID != "org_test" ||
		commerce.adjustment.Reason != "post in authorized open period" {
		t.Fatalf("adjustment=%+v", commerce.adjustment)
	}

	memberRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/organizations/org_test/accounting-failures/"+failureID+"/adjustments",
		strings.NewReader(`{"id":"adjustment_2","effective_at":"`+effectiveAt+`","reason":"not authorized"}`),
	)
	memberRequest.Header.Set("Idempotency-Key", "period-adjustment-2")
	memberRequest.Header.Set("X-Source-Version", "1")
	memberRecorder := httptest.NewRecorder()
	NewHTTPServer(&mutationCommerceStub{}, organizationAuthStub{principal: identitydomain.Principal{
		OrganizationID: "org_test", ActorID: "member_test", Role: identitydomain.RoleMember,
		OrganizationStatus: "ready", MembershipStatus: "active",
	}}).Handler().ServeHTTP(memberRecorder, memberRequest)
	if memberRecorder.Code != http.StatusForbidden ||
		!strings.Contains(memberRecorder.Body.String(), "FORBIDDEN") {
		t.Fatalf(
			"member status=%d body=%s",
			memberRecorder.Code, memberRecorder.Body.String(),
		)
	}
}

func TestAccountingFailureIsVisibleWithoutPrivateCommandPayload(t *testing.T) {
	failureID := uuid.NewString()
	commerce := &mutationCommerceStub{failure: domain.AccountingFailure{
		SourceKind: "purchase", SourceID: "purchase_1",
		FailedEffectiveAt: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		Status:            "awaiting_adjustment", FailureCode: domain.ErrPeriodLocked.Error(),
		CorrelationID:  "correlation_1",
		CommandPayload: json.RawMessage(`{"private":"must-not-leak"}`),
		CreatedAt:      time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/organizations/org_test/accounting-failures/"+failureID,
		nil,
	)
	recorder := httptest.NewRecorder()
	NewHTTPServer(commerce, organizationAuthStub{organizationID: "org_test"}).
		Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"failure_code":"PERIOD_LOCKED"`) ||
		strings.Contains(recorder.Body.String(), "must-not-leak") ||
		strings.Contains(recorder.Body.String(), "command_payload") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicMutationsRequireIdempotencyIdentity(t *testing.T) {
	t.Parallel()
	server := NewHTTPServer(&mutationCommerceStub{}, organizationAuthStub{organizationID: "org_test"}).Handler()
	tests := []struct {
		name      string
		headers   map[string]string
		status    int
		errorCode string
	}{
		{name: "missing key", headers: map[string]string{"X-Source-Version": "1"}, status: http.StatusBadRequest, errorCode: "IDEMPOTENCY_KEY_REQUIRED"},
		{name: "missing source version", headers: map[string]string{"Idempotency-Key": "party-request"}, status: http.StatusBadRequest, errorCode: "SOURCE_VERSION_REQUIRED"},
		{name: "invalid source version", headers: map[string]string{"Idempotency-Key": "party-request", "X-Source-Version": "0"}, status: http.StatusBadRequest, errorCode: "SOURCE_VERSION_REQUIRED"},
		{name: "oversized key", headers: map[string]string{"Idempotency-Key": strings.Repeat("x", 256), "X-Source-Version": "1"}, status: http.StatusBadRequest, errorCode: "VALIDATION_ERROR"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org_test/parties", strings.NewReader(`{"id":"party_1","kind":"customer","display_name":"Alice"}`))
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.errorCode) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestPublicMutationsForwardCanonicalIdempotencyIdentity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		path      string
		body      string
		operation string
		sourceID  string
	}{
		{
			name: "party", path: "/api/v1/organizations/org_test/parties",
			body:      `{"id":"party_1","kind":"customer","display_name":"Alice"}`,
			operation: domain.OperationCreateParty, sourceID: "party_1",
		},
		{
			name: "sale", path: "/api/v1/organizations/org_test/sales",
			body:      `{"id":"sale_1","recipient_ref":"party_1","point_of_sale":1,"document_type":"FA","amount":"121.00","currency":"ARS","credential_ref":"credential_1","fiscal":{"environment":"homologation","issue_date":"2026-07-31","totals":{},"recipient":{},"lines":[{}]}}`,
			operation: domain.OperationCreateSale, sourceID: "sale_1",
		},
		{
			name: "purchase", path: "/api/v1/organizations/org_test/purchases",
			body:      `{"id":"purchase_1","supplier_ref":"supplier_1","external_document_ref":"invoice_1","issue_date":"2026-07-31","amount":"100.00","currency":"ARS","net_amount":"0","exempt_amount":"100.00","vat_breakdown":[]}`,
			operation: domain.OperationCreatePurchase, sourceID: "purchase_1",
		},
		{
			name: "payment", path: "/api/v1/organizations/org_test/payments",
			body:      `{"id":"payment_1","direction":"receipt","party_ref":"party_1","amount":"50.00","currency":"ARS","applications":[]}`,
			operation: domain.OperationCreatePayment, sourceID: "payment_1",
		},
		{
			name: "reversal", path: "/api/v1/organizations/org_test/reversals",
			body:      `{"id":"reversal_1","document_kind":"purchase","document_id":"purchase_1","effective_at":"2026-07-31T12:00:00Z","reason":"supplier cancellation"}`,
			operation: domain.OperationCreateAccountingReversal, sourceID: "reversal_1",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			commerce := &mutationCommerceStub{}
			server := NewHTTPServer(commerce, organizationAuthStub{organizationID: "org_test"}).Handler()
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Idempotency-Key", "public-request-key")
			request.Header.Set("X-Source-Version", "7")
			request.Header.Set("X-Request-ID", "request-"+test.name)
			request.Header.Set("X-Correlation-ID", "correlation-"+test.name)
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusCreated {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if len(commerce.commands) != 1 {
				t.Fatalf("captured commands=%d", len(commerce.commands))
			}
			command := commerce.commands[0]
			if command.Key != "public-request-key" || command.OrganizationID != "org_test" ||
				command.Operation != test.operation || command.SourceID != test.sourceID ||
				command.SourceVersion != 7 || len(command.PayloadHash) != 64 ||
				command.RequestID != "request-"+test.name ||
				command.CorrelationID != "correlation-"+test.name ||
				command.ActorRef != "user_test" {
				t.Fatalf("command=%+v", command)
			}
			if recorder.Header().Get("Idempotency-Key") != command.Key {
				t.Fatalf("response key=%q", recorder.Header().Get("Idempotency-Key"))
			}
		})
	}
}

func TestPublicMutationMapsChangedPayloadToStableConflict(t *testing.T) {
	t.Parallel()
	commerce := &mutationCommerceStub{err: domain.ErrIdempotencyKeyReused}
	server := NewHTTPServer(commerce, organizationAuthStub{organizationID: "org_test"}).Handler()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org_test/parties", strings.NewReader(`{"id":"party_1","kind":"customer","display_name":"Changed"}`))
	request.Header.Set("Idempotency-Key", "public-request-key")
	request.Header.Set("X-Source-Version", "1")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCanonicalPayloadHashIgnoresJSONFieldOrder(t *testing.T) {
	t.Parallel()
	commerce := &mutationCommerceStub{}
	server := NewHTTPServer(commerce, organizationAuthStub{organizationID: "org_test"}).Handler()
	bodies := []string{
		`{"id":"party_1","kind":"customer","display_name":"Alice","tax_identifier":"201"}`,
		`{"tax_identifier":"201","display_name":"Alice","kind":"customer","id":"party_1"}`,
	}
	for _, body := range bodies {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org_test/parties", strings.NewReader(body))
		request.Header.Set("Idempotency-Key", "public-request-key")
		request.Header.Set("X-Source-Version", "1")
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	}
	if len(commerce.commands) != 2 || commerce.commands[0].PayloadHash != commerce.commands[1].PayloadHash {
		t.Fatalf("commands=%+v", commerce.commands)
	}
}

func TestGeneratedServerRoutesEveryOpenAPIOperationAndBindsParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string
		mutation   bool
	}{
		{name: "health", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK, wantBody: `"status":"ok"`},
		{name: "readiness", method: http.MethodGet, path: "/readyz", wantStatus: http.StatusOK, wantBody: `"status":"ready"`},
		{name: "get party path parameter", method: http.MethodGet, path: "/api/v1/organizations/org_test/parties/party_bound", wantStatus: http.StatusOK, wantBody: `"id":"party_bound"`},
		{name: "get sale path parameter", method: http.MethodGet, path: "/api/v1/organizations/org_test/sales/sale_bound", wantStatus: http.StatusOK, wantBody: `"id":"sale_bound"`},
		{name: "get purchase path parameter", method: http.MethodGet, path: "/api/v1/organizations/org_test/purchases/purchase_bound", wantStatus: http.StatusOK, wantBody: `"id":"purchase_bound"`},
		{name: "get payment path parameter", method: http.MethodGet, path: "/api/v1/organizations/org_test/payments/payment_bound", wantStatus: http.StatusOK, wantBody: `"id":"payment_bound"`},
		{name: "create party", method: http.MethodPost, path: "/api/v1/organizations/org_test/parties", body: `{"id":"party_bound","kind":"customer","display_name":"Alice"}`, wantStatus: http.StatusCreated, wantBody: `"id":"party_bound"`, mutation: true},
		{name: "create sale", method: http.MethodPost, path: "/api/v1/organizations/org_test/sales", body: `{"id":"sale_bound","recipient_ref":"party_1","point_of_sale":1,"document_type":"FA","amount":"121.00","currency":"ARS","credential_ref":"credential_1","fiscal":{"environment":"homologation","issue_date":"2026-07-31","totals":{},"recipient":{},"lines":[{}]}}`, wantStatus: http.StatusCreated, wantBody: `"id":"sale_bound"`, mutation: true},
		{name: "create purchase", method: http.MethodPost, path: "/api/v1/organizations/org_test/purchases", body: `{"id":"purchase_bound","supplier_ref":"supplier_1","external_document_ref":"invoice_1","issue_date":"2026-07-31","amount":"100.00","currency":"ARS","net_amount":"0","exempt_amount":"100.00","vat_breakdown":[]}`, wantStatus: http.StatusCreated, wantBody: `"id":"purchase_bound"`, mutation: true},
		{name: "create payment", method: http.MethodPost, path: "/api/v1/organizations/org_test/payments", body: `{"id":"payment_bound","direction":"receipt","party_ref":"party_1","amount":"50.00","currency":"ARS","applications":[]}`, wantStatus: http.StatusCreated, wantBody: `"id":"payment_bound"`, mutation: true},
		{name: "create reversal", method: http.MethodPost, path: "/api/v1/organizations/org_test/reversals", body: `{"id":"reversal_bound","document_kind":"purchase","document_id":"purchase_1","effective_at":"2026-07-31T12:00:00Z","reason":"supplier cancellation"}`, wantStatus: http.StatusCreated, wantBody: `"id":"reversal_bound"`, mutation: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			commerce := &mutationCommerceStub{}
			server := NewHTTPServer(commerce, organizationAuthStub{organizationID: "org_test"}).Handler()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.mutation {
				request.Header.Set("Idempotency-Key", "generated-route-"+test.name)
				request.Header.Set("X-Source-Version", "9")
			}
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), test.wantBody) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if test.mutation {
				if len(commerce.commands) != 1 || commerce.commands[0].OrganizationID != "org_test" || commerce.commands[0].SourceVersion != 9 {
					t.Fatalf("generated parameters were not forwarded: %+v", commerce.commands)
				}
			}
		})
	}
}

func TestGeneratedBindingErrorsKeepStableJSONCodes(t *testing.T) {
	t.Parallel()
	server := NewHTTPServer(&mutationCommerceStub{}, organizationAuthStub{organizationID: "org_test"}).Handler()
	tests := []struct {
		name    string
		headers http.Header
		code    string
	}{
		{name: "missing idempotency key", headers: http.Header{"X-Source-Version": {"1"}}, code: "IDEMPOTENCY_KEY_REQUIRED"},
		{name: "missing source version", headers: http.Header{"Idempotency-Key": {"key"}}, code: "SOURCE_VERSION_REQUIRED"},
		{name: "malformed source version", headers: http.Header{"Idempotency-Key": {"key"}, "X-Source-Version": {"not-an-integer"}}, code: "SOURCE_VERSION_REQUIRED"},
		{name: "repeated generated parameter", headers: http.Header{"Idempotency-Key": {"key", "other"}, "X-Source-Version": {"1"}}, code: "VALIDATION_ERROR"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org_test/parties", strings.NewReader(`{"id":"party_1","kind":"customer","display_name":"Alice"}`))
			request.Header = test.headers.Clone()
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"`+test.code+`"`) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestPublicResponsesExposeOnlyOpenAPIFields(t *testing.T) {
	t.Parallel()
	server := NewHTTPServer(&mutationCommerceStub{}, organizationAuthStub{organizationID: "org_test"}).Handler()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org_test/sales", strings.NewReader(`{"id":"sale_1","recipient_ref":"party_1","point_of_sale":1,"document_type":"FA","amount":"121.00","currency":"ARS","credential_ref":"credential_secret","fiscal":{"environment":"homologation","issue_date":"2026-07-31","totals":{},"recipient":{"tax_identifier":"secret"},"lines":[{}]}}`))
	request.Header.Set("Idempotency-Key", "public-response")
	request.Header.Set("X-Source-Version", "1")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, forbidden := range []string{"credential_secret", "fiscal_snapshot", "correlation_id", "created_at", "updated_at", "tax_identifier"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("public response leaked %q: %s", forbidden, recorder.Body.String())
		}
	}
}

func TestGeneratedRouterRejectsUndeclaredCommercialRoutes(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	NewHTTPServer(&mutationCommerceStub{}, organizationAuthStub{organizationID: "org_test"}).Handler().
		ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org_test/manual-only", strings.NewReader(`{}`)))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestWriteCommandErrorPreservesFeatureDisabledCode(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	writeCommandError(response, domain.ErrFeatureDisabled)
	if response.Code != http.StatusForbidden ||
		!strings.Contains(response.Body.String(), `"FEATURE_DISABLED"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
}
