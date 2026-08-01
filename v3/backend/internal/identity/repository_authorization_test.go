package identity_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	commerce "github.com/devpablocristo/pymes/v3/backend/internal/commerce"
	commercedomain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	identity "github.com/devpablocristo/pymes/v3/backend/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixedSessionVerifier struct{ claims clerk.SessionClaims }

func (v fixedSessionVerifier) VerifySession(context.Context, string) (clerk.SessionClaims, error) {
	return v.claims, nil
}

type authorizationCommerceStub struct {
	commerce.Commerce
	calls int
}

func (s *authorizationCommerceStub) capture(ctx context.Context) {
	principal, ok := identity.PrincipalFromContext(ctx)
	if !ok || principal.ActorID == "" || principal.OrganizationID == "" {
		panic("authorized local principal was not propagated")
	}
	s.calls++
}

func (s *authorizationCommerceStub) CreatePartyIdempotent(ctx context.Context, _ commercedomain.IdempotencyCommand, value commercedomain.Party) (commercedomain.Party, error) {
	s.capture(ctx)
	return value, nil
}

func (s *authorizationCommerceStub) CreatePurchaseAndQueueIdempotent(ctx context.Context, _ commercedomain.IdempotencyCommand, value commercedomain.Purchase) (commercedomain.Purchase, error) {
	s.capture(ctx)
	value.Status = "confirmed"
	return value, nil
}

func (s *authorizationCommerceStub) CreatePaymentAndApplicationsIdempotent(ctx context.Context, _ commercedomain.IdempotencyCommand, value commercedomain.Payment, _ []commercedomain.OpenItemApplication) (commercedomain.Payment, error) {
	s.capture(ctx)
	value.Status = "confirmed"
	return value, nil
}

func (s *authorizationCommerceStub) CreateSaleAndQueueFiscalIdempotent(ctx context.Context, _ commercedomain.IdempotencyCommand, value commercedomain.Sale, _ string) (commercedomain.Sale, error) {
	s.capture(ctx)
	value.Voucher.VoucherNumber = 1
	value.SnapshotDigest = strings.Repeat("a", 64)
	return value, nil
}

func (s *authorizationCommerceStub) CreateAccountingReversalIdempotent(ctx context.Context, _ commercedomain.IdempotencyCommand, value commercedomain.AccountingReversal) (commercedomain.AccountingReversal, error) {
	s.capture(ctx)
	value.OriginalJournalEntryID = "journal_1"
	return value, nil
}

func (s *authorizationCommerceStub) Clock() time.Time {
	return time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
}

type mutationEndpoint struct {
	name string
	path string
	body string
}

func mutationEndpoints(organizationID string) []mutationEndpoint {
	prefix := "/api/v1/organizations/" + organizationID
	return []mutationEndpoint{
		{name: "party", path: prefix + "/parties", body: `{"id":"party_1","kind":"customer","display_name":"Alice"}`},
		{name: "sale", path: prefix + "/sales", body: `{"id":"sale_1","recipient_ref":"party_1","point_of_sale":1,"document_type":"FA","amount":"121.00","currency":"ARS","credential_ref":"credential_1","fiscal":{"environment":"homologation","issue_date":"2026-07-31","totals":{},"recipient":{},"lines":[{}]}}`},
		{name: "purchase", path: prefix + "/purchases", body: `{"id":"purchase_1","supplier_ref":"supplier_1","external_document_ref":"invoice_1","issue_date":"2026-07-31","amount":"100.00","net_amount":"100.00","exempt_amount":"0.00","vat_breakdown":[{"rate":"0","base_amount":"100.00","tax_amount":"0.00"}],"currency":"ARS"}`},
		{name: "payment", path: prefix + "/payments", body: `{"id":"payment_1","direction":"receipt","party_ref":"party_1","amount":"50.00","currency":"ARS","applications":[]}`},
		{name: "reversal", path: prefix + "/reversals", body: `{"id":"reversal_1","document_kind":"purchase","document_id":"purchase_1","effective_at":"2026-07-31T12:00:00Z","reason":"supplier cancellation"}`},
	}
}

func TestPostgresPrincipalDrivesBFFMutationAuthorization(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `
		TRUNCATE app.clerk_webhook_inbox,app.memberships,app.organization_identities,
		         app.idempotency_records,app.outbox,app.accounting_reversals,
		         app.accounting_application_commands,app.open_item_applications,
		         app.payments,app.purchases,app.sales,app.parties,app.organizations
		CASCADE`); err != nil {
		t.Fatal(err)
	}

	type principalFixture struct {
		providerOrg, localOrg, user, role, membershipStatus, organizationStatus string
	}
	fixtures := []principalFixture{
		{providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_owner", role: "owner", membershipStatus: "active", organizationStatus: "ready"},
		{providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_admin", role: "admin", membershipStatus: "active", organizationStatus: "ready"},
		{providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_member", role: "member", membershipStatus: "active", organizationStatus: "ready"},
		{providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_viewer", role: "viewer", membershipStatus: "active", organizationStatus: "ready"},
		{providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_inactive", role: "owner", membershipStatus: "inactive", organizationStatus: "ready"},
		{providerOrg: "clerk_authz_pending", localOrg: "org_authz_pending", user: "user_pending", role: "owner", membershipStatus: "active", organizationStatus: "pending"},
		{providerOrg: "clerk_authz_failed", localOrg: "org_authz_failed", user: "user_failed", role: "admin", membershipStatus: "active", organizationStatus: "failed"},
		{providerOrg: "clerk_authz_suspended", localOrg: "org_authz_suspended", user: "user_suspended", role: "owner", membershipStatus: "active", organizationStatus: "suspended"},
	}
	insertedOrganizations := map[string]bool{}
	for _, fixture := range fixtures {
		if !insertedOrganizations[fixture.localOrg] {
			if _, err = pool.Exec(ctx, `
				INSERT INTO app.organizations(id,name,slug,status)
				VALUES($1,$2,$3,$4)`, fixture.localOrg, fixture.localOrg, fixture.localOrg, fixture.organizationStatus); err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `
				INSERT INTO app.organization_identities(provider,provider_organization_id,org_id)
				VALUES('clerk',$1,$2)`, fixture.providerOrg, fixture.localOrg); err != nil {
				t.Fatal(err)
			}
			insertedOrganizations[fixture.localOrg] = true
		}
		tx, txErr := pool.BeginTx(ctx, pgx.TxOptions{})
		if txErr != nil {
			t.Fatal(txErr)
		}
		if _, txErr = tx.Exec(ctx, `SELECT set_config('app.org_id',$1,true)`, fixture.localOrg); txErr == nil {
			_, txErr = tx.Exec(ctx, `
				INSERT INTO app.memberships(org_id,provider,provider_user_id,role,permissions,status)
				VALUES($1,'clerk',$2,$3,'["commerce:read"]'::jsonb,$4)`,
				fixture.localOrg, fixture.user, fixture.role, fixture.membershipStatus)
		}
		if txErr == nil {
			txErr = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if txErr != nil {
			t.Fatal(txErr)
		}
	}

	tests := []struct {
		name, providerOrg, localOrg, user string
		wantStatus                        int
		wantCode                          string
	}{
		{name: "owner", providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_owner", wantStatus: http.StatusCreated},
		{name: "admin", providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_admin", wantStatus: http.StatusCreated},
		{name: "member", providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_member", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "viewer", providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_viewer", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "inactive", providerOrg: "clerk_authz_ready", localOrg: "org_authz_ready", user: "user_inactive", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "pending", providerOrg: "clerk_authz_pending", localOrg: "org_authz_pending", user: "user_pending", wantStatus: http.StatusUnprocessableEntity, wantCode: "ORG_NOT_PROVISIONED"},
		{name: "failed", providerOrg: "clerk_authz_failed", localOrg: "org_authz_failed", user: "user_failed", wantStatus: http.StatusUnprocessableEntity, wantCode: "ORG_NOT_PROVISIONED"},
		{name: "suspended", providerOrg: "clerk_authz_suspended", localOrg: "org_authz_suspended", user: "user_suspended", wantStatus: http.StatusUnprocessableEntity, wantCode: "ORG_NOT_PROVISIONED"},
	}
	memberships := identity.New(pool)
	for _, test := range tests {
		for _, endpoint := range mutationEndpoints(test.localOrg) {
			commerceStub := &authorizationCommerceStub{}
			auth := identity.ClerkAuthenticator{
				Memberships: memberships,
				Verifier: fixedSessionVerifier{claims: clerk.SessionClaims{
					OrganizationID: test.providerOrg, Subject: test.user,
				}},
			}
			request := httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(endpoint.body))
			request.Header.Set("Authorization", "Bearer verified-session")
			request.Header.Set("Idempotency-Key", test.name+"-"+endpoint.name)
			request.Header.Set("X-Source-Version", "1")
			recorder := httptest.NewRecorder()
			commerce.NewHTTPServer(commerceStub, auth).Handler().ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus || (test.wantCode != "" && !strings.Contains(recorder.Body.String(), test.wantCode)) {
				t.Fatalf("case=%s endpoint=%s status=%d body=%s", test.name, endpoint.name, recorder.Code, recorder.Body.String())
			}
			wantCalls := 0
			if test.wantStatus == http.StatusCreated {
				wantCalls = 1
			}
			if commerceStub.calls != wantCalls {
				t.Fatalf("case=%s endpoint=%s calls=%d want=%d", test.name, endpoint.name, commerceStub.calls, wantCalls)
			}
		}
	}
}

func TestPostgresOwnerAndAdminPersistEveryBFFMutation(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `
		TRUNCATE app.clerk_webhook_inbox,app.memberships,app.organization_identities,
		         app.idempotency_records,app.outbox,app.accounting_reversals,
		         app.accounting_application_commands,app.open_item_applications,
		         app.payments,app.purchases,app.sales,app.parties,app.organizations
		CASCADE`); err != nil {
		t.Fatal(err)
	}

	for _, role := range []string{"owner", "admin"} {
		localOrg := "org_bff_" + role
		providerOrg := "clerk_bff_" + role
		user := "user_bff_" + role
		if _, err = pool.Exec(ctx, `
			INSERT INTO app.organizations(id,name,slug,status) VALUES($1,$2,$3,'ready')`,
			localOrg, localOrg, localOrg); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `
			INSERT INTO app.organization_identities(provider,provider_organization_id,org_id)
			VALUES('clerk',$1,$2)`, providerOrg, localOrg); err != nil {
			t.Fatal(err)
		}
		tx, txErr := pool.BeginTx(ctx, pgx.TxOptions{})
		if txErr != nil {
			t.Fatal(txErr)
		}
		if _, txErr = tx.Exec(ctx, `SELECT set_config('app.org_id',$1,true)`, localOrg); txErr == nil {
			_, txErr = tx.Exec(ctx, `
				INSERT INTO app.memberships(org_id,provider,provider_user_id,role,permissions,status)
				VALUES($1,'clerk',$2,$3,'["commerce:read","commerce:write"]'::jsonb,'active')`,
				localOrg, user, role)
		}
		if txErr == nil {
			txErr = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if txErr != nil {
			t.Fatal(txErr)
		}

		store := commerce.New(pool)
		store.Now = func() time.Time { return time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC) }
		reversalDocumentID := "posted_purchase_" + role
		if err = store.CreatePurchaseAndQueue(ctx, commercedomain.Purchase{
			ID: reversalDocumentID, OrganizationID: localOrg, SupplierRef: "supplier_" + role,
			ExternalDocumentRef: "preseed_" + role,
			IssueDate:           "2026-07-31",
			Total:               commercedomain.Money{Amount: "100.00", Currency: "ARS"},
			NetAmount:           "0",
			ExemptAmount:        "100.00",
			SnapshotDigest:      strings.Repeat("b", 64), CorrelationID: "preseed:" + role,
		}); err != nil {
			t.Fatal(err)
		}
		tx, txErr = pool.BeginTx(ctx, pgx.TxOptions{})
		if txErr != nil {
			t.Fatal(txErr)
		}
		if _, txErr = tx.Exec(ctx, `SELECT set_config('app.org_id',$1,true)`, localOrg); txErr == nil {
			_, txErr = tx.Exec(ctx, `
				UPDATE app.purchases SET status='posted',journal_entry_id=$2
				WHERE org_id=$1 AND id=$3`, localOrg, "journal_"+role, reversalDocumentID)
		}
		if txErr == nil {
			txErr = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if txErr != nil {
			t.Fatal(txErr)
		}

		commands := commerce.Commands{Store: store, Now: store.Clock}
		auth := identity.ClerkAuthenticator{
			Memberships: identity.New(pool),
			Verifier: fixedSessionVerifier{claims: clerk.SessionClaims{
				OrganizationID: providerOrg, Subject: user,
			}},
		}
		prefix := "/api/v1/organizations/" + localOrg
		endpoints := []mutationEndpoint{
			{name: "party", path: prefix + "/parties", body: `{"id":"party_` + role + `","kind":"customer","display_name":"Alice"}`},
			{name: "sale", path: prefix + "/sales", body: `{"id":"sale_` + role + `","recipient_ref":"party_` + role + `","point_of_sale":1,"document_type":"FA","amount":"121.00","currency":"ARS","credential_ref":"credential_1","fiscal":{"environment":"homologation","issue_date":"2026-07-31","totals":{},"recipient":{},"lines":[{}]}}`},
			{name: "purchase", path: prefix + "/purchases", body: `{"id":"purchase_` + role + `","supplier_ref":"supplier_` + role + `","external_document_ref":"invoice_` + role + `","issue_date":"2026-07-31","amount":"100.00","net_amount":"100.00","exempt_amount":"0.00","vat_breakdown":[{"rate":"0","base_amount":"100.00","tax_amount":"0.00"}],"currency":"ARS"}`},
			{name: "payment", path: prefix + "/payments", body: `{"id":"payment_` + role + `","direction":"receipt","party_ref":"party_` + role + `","amount":"50.00","currency":"ARS","applications":[]}`},
			{name: "reversal", path: prefix + "/reversals", body: `{"id":"reversal_` + role + `","document_kind":"purchase","document_id":"` + reversalDocumentID + `","effective_at":"2026-07-31T12:00:00Z","reason":"supplier cancellation"}`},
		}
		for _, endpoint := range endpoints {
			key := "persist-" + role + "-" + endpoint.name
			request := httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(endpoint.body))
			request.Header.Set("Authorization", "Bearer verified-session")
			request.Header.Set("Idempotency-Key", key)
			request.Header.Set("X-Source-Version", "1")
			recorder := httptest.NewRecorder()
			commerce.NewHTTPServer(commands, auth).Handler().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusCreated {
				t.Fatalf("role=%s endpoint=%s status=%d body=%s", role, endpoint.name, recorder.Code, recorder.Body.String())
			}
			var records int
			if err = pool.QueryRow(ctx, `
				SELECT count(*) FROM app.idempotency_records
				WHERE org_id=$1 AND idempotency_key=$2 AND completed_at IS NOT NULL`, localOrg, key).Scan(&records); err != nil {
				t.Fatal(err)
			}
			if records != 1 {
				t.Fatalf("role=%s endpoint=%s completed idempotency records=%d", role, endpoint.name, records)
			}
		}
	}
}
