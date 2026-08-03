package commerce

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	accountingapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type tokenSource struct{}

func (tokenSource) Token(_ context.Context, audience, organizationID string) (string, error) {
	return audience + ":" + organizationID, nil
}

type metadataTokenSource struct {
	metadata identityusecases.RequestMetadata
}

func (source *metadataTokenSource) Token(
	ctx context.Context,
	audience,
	organizationID string,
) (string, error) {
	source.metadata, _ = identityusecases.RequestMetadataFromContext(ctx)
	return audience + ":" + organizationID, nil
}

type platformTokenSource struct{}

func (platformTokenSource) PlatformToken(_ context.Context, audience string) (string, error) {
	return "platform:" + audience, nil
}

func writeContractJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func accountingEventFixture(commandID, organizationID, idempotencyKey, correlationID, snapshotDigest, journalEntryID string) domain.AccountingEvent {
	return domain.AccountingEvent{
		EventID:        uuid.NewString(),
		CommandID:      commandID,
		OrganizationID: organizationID,
		IdempotencyKey: idempotencyKey,
		SourceVersion:  1,
		SnapshotDigest: snapshotDigest,
		Status:         "posted",
		JournalEntryID: journalEntryID,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		CorrelationID:  correlationID,
	}
}

func TestAccountingClientAgainstHeadlessService(t *testing.T) {
	baseURL := os.Getenv("PYMES_ACCOUNTING_TEST_URL")
	provisioningBaseURL := os.Getenv("PYMES_ACCOUNTING_PROVISIONING_TEST_URL")
	if baseURL == "" || provisioningBaseURL == "" {
		t.Skip("PYMES_ACCOUNTING_TEST_URL and PYMES_ACCOUNTING_PROVISIONING_TEST_URL are required")
	}
	tokens, err := identityaccess.TokenSourceFromRuntime("worker:accounting-contract-test")
	if err != nil {
		t.Fatal(err)
	}
	defer tokens.Close()
	client := HTTPAccountingClient{BaseURL: baseURL, Tokens: tokens}
	provisioningClient := HTTPAccountingProvisioningClient{BaseURL: provisioningBaseURL, Tokens: tokens}
	organizationID := "org_contract_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := provisioningClient.ProvisionOrganization(context.Background(), organizationdomain.Organization{ID: organizationID, Name: "Contract Test"}); err != nil {
		t.Fatal(err)
	}
	command := domain.PostingCommand{
		CommandID: uuid.NewString(), OrganizationID: organizationID, SourceType: "sales_invoice",
		IdempotencyKey: "accounting-contract-" + uuid.NewString(),
		SourceID:       "sale-contract", SourceVersion: 1,
		SnapshotDigest: strings.Repeat("a", 64), CorrelationID: "accounting-contract",
		EffectiveAt: time.Now().UTC(), Description: "Contract sale",
		Lines: []domain.PostingLine{
			{AccountCode: "1200", Debit: domain.Money{Amount: "121", Currency: "ARS"}, Credit: domain.Money{Amount: "0", Currency: "ARS"}, OpenItem: true, PartyRef: "party-contract"},
			{AccountCode: "4100", Debit: domain.Money{Amount: "0", Currency: "ARS"}, Credit: domain.Money{Amount: "121", Currency: "ARS"}},
		},
	}
	ctx := identityaccess.WithRequestMetadata(context.Background(), identityaccess.RequestMetadata{
		RequestID:     "accounting-contract-request",
		CorrelationID: command.CorrelationID,
	})
	posted, err := client.Post(ctx, command)
	if err != nil || posted.Status != "posted" || posted.JournalEntryID == "" || len(posted.OpenItemIDs) != 1 ||
		posted.OrganizationID != command.OrganizationID || posted.CommandID != command.CommandID ||
		posted.IdempotencyKey != command.IdempotencyKey || posted.SourceVersion != command.SourceVersion ||
		posted.SnapshotDigest != command.SnapshotDigest || posted.CorrelationID != command.CorrelationID {
		t.Fatalf("posted=%+v err=%v", posted, err)
	}
	duplicate, err := client.Post(ctx, command)
	if err != nil || duplicate.JournalEntryID != posted.JournalEntryID || len(duplicate.OpenItemIDs) != 1 {
		t.Fatalf("duplicate=%+v err=%v", duplicate, err)
	}
	secondCommand := command
	secondCommand.CommandID = uuid.NewString()
	secondCommand.IdempotencyKey = "accounting-contract-" + uuid.NewString()
	secondCommand.SourceID = "sale-contract-2"
	secondCommand.SnapshotDigest = strings.Repeat("c", 64)
	secondPosted, err := client.Post(ctx, secondCommand)
	if err != nil || secondPosted.JournalEntryID == "" ||
		secondPosted.JournalEntryID == posted.JournalEntryID {
		t.Fatalf("second posted=%+v err=%v", secondPosted, err)
	}

	reportClient, err := client.generatedClient(
		organizationID,
		"accounting-report-"+uuid.NewString(),
		command.CorrelationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	limit := 1
	asOf := openapi_types.Date{Time: time.Now().UTC().AddDate(0, 0, 1)}
	firstPage, err := reportClient.GetReportWithResponse(
		ctx,
		organizationID,
		accountingapi.GeneralLedger,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			Limit:          &limit,
			XCorrelationID: command.CorrelationID,
		},
	)
	if err != nil {
		t.Fatalf("first general ledger page: %v", err)
	}
	if firstPage.StatusCode() != http.StatusOK || firstPage.JSON200 == nil {
		t.Fatalf(
			"first general ledger response=%d %s",
			firstPage.StatusCode(),
			firstPage.Body,
		)
	}
	if firstPage.JSON200.Entries == nil ||
		len(*firstPage.JSON200.Entries) != 1 ||
		firstPage.JSON200.HasMore == nil ||
		!*firstPage.JSON200.HasMore ||
		firstPage.JSON200.NextCursor == nil ||
		*firstPage.JSON200.NextCursor == "" ||
		firstPage.JSON200.OrganizationId == nil ||
		*firstPage.JSON200.OrganizationId != organizationID ||
		firstPage.JSON200.Report == nil ||
		*firstPage.JSON200.Report != accountingapi.AccountingReportReportGeneralLedger ||
		firstPage.JSON200.AsOf == nil ||
		firstPage.JSON200.AsOf.Time.Format("2006-01-02") !=
			asOf.Time.Format("2006-01-02") {
		t.Fatalf("first general ledger page=%#v", firstPage.JSON200)
	}
	nextCursor := *firstPage.JSON200.NextCursor

	secondPage, err := reportClient.GetReportWithResponse(
		ctx,
		organizationID,
		accountingapi.GeneralLedger,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			Limit:          &limit,
			Cursor:         &nextCursor,
			XCorrelationID: command.CorrelationID,
		},
	)
	if err != nil {
		t.Fatalf("second general ledger page: %v", err)
	}
	if secondPage.StatusCode() != http.StatusOK || secondPage.JSON200 == nil {
		t.Fatalf(
			"second general ledger response=%d %s",
			secondPage.StatusCode(),
			secondPage.Body,
		)
	}
	if secondPage.JSON200.Entries == nil ||
		len(*secondPage.JSON200.Entries) != 1 ||
		secondPage.JSON200.HasMore == nil ||
		*secondPage.JSON200.HasMore ||
		secondPage.JSON200.NextCursor != nil {
		t.Fatalf("second general ledger page=%#v", secondPage.JSON200)
	}
	firstEntry := (*firstPage.JSON200.Entries)[0]
	secondEntry := (*secondPage.JSON200.Entries)[0]
	if firstEntry.Id == secondEntry.Id {
		t.Fatalf("general ledger pages overlap: first=%#v second=%#v", firstEntry, secondEntry)
	}

	changed := command
	changed.SnapshotDigest = strings.Repeat("b", 64)
	if _, err := client.Post(ctx, changed); err == nil || !strings.Contains(err.Error(), "IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("changed payload err=%v", err)
	}
}

func TestAccountingProvisioningClientSendsCanonicalPrivilegedRequest(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	issuer, err := identityaccess.NewServiceIssuerFromSeed(
		"https://pymes.internal",
		"pymes-key-1",
		make([]byte, ed25519.SeedSize),
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	tokens := &identityaccess.IssuerTokenSource{Issuer: issuer, Subject: "provision-org"}
	const (
		organizationID = "org_acme"
		requestID      = "request-provision-1"
		correlationID  = "correlation-provision-1"
		canonicalBody  = `{"organization_id":"org_acme","display_name":"Acme"}`
	)
	bodySum := sha256.Sum256([]byte(canonicalBody))
	organizationSum := sha256.Sum256([]byte(organizationID))
	wantIdempotencyKey := "provision-org-v1:" + hex.EncodeToString(organizationSum[:])

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut ||
			r.URL.EscapedPath() != "/internal/v1/organizations/org_acme" {
			t.Fatalf("method=%s path=%s", r.Method, r.URL.EscapedPath())
		}
		encoded, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(encoded) != canonicalBody {
			t.Fatalf("body=%q want=%q", encoded, canonicalBody)
		}
		if r.Header.Get("Content-Type") != "application/json" ||
			r.Header.Get("Idempotency-Key") != wantIdempotencyKey ||
			r.Header.Get("X-Request-ID") != requestID ||
			r.Header.Get("X-Correlation-ID") != correlationID ||
			r.Header.Get("X-Payload-Digest") != hex.EncodeToString(bodySum[:]) {
			t.Fatalf("headers=%v", r.Header)
		}
		if key := r.Header.Get("Idempotency-Key"); len(key) < 16 || len(key) > 128 {
			t.Fatalf("idempotency key length=%d", len(key))
		}
		if r.Header.Get("X-Serverless-Authorization") != "Bearer platform:"+server.URL {
			t.Fatalf("platform authorization=%q", r.Header.Get("X-Serverless-Authorization"))
		}
		authorization := r.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") {
			t.Fatalf("authorization=%q", authorization)
		}
		claims, verifyErr := identityaccess.VerifyInternalCredential(
			strings.TrimPrefix(authorization, "Bearer "),
			issuer.PublicKey(),
			now,
			"https://pymes.internal",
			"accounting-provisioning",
			organizationID,
		)
		if verifyErr != nil {
			t.Fatal(verifyErr)
		}
		if claims.RequestID != requestID ||
			claims.CorrelationID != correlationID ||
			claims.Subject != "provision-org" ||
			len(claims.Roles) != 1 || claims.Roles[0] != "service" {
			t.Fatalf("claims=%+v", claims)
		}
		writeContractJSON(w, http.StatusCreated, map[string]string{"status": "ready"})
	}))
	defer server.Close()

	ctx := identityusecases.WithRequestMetadata(context.Background(), identityusecases.RequestMetadata{
		RequestID:     requestID,
		CorrelationID: correlationID,
	})
	client := HTTPAccountingProvisioningClient{
		BaseURL:        server.URL + "/",
		Tokens:         tokens,
		PlatformTokens: platformTokenSource{},
	}
	if err := client.ProvisionOrganization(ctx, organizationdomain.Organization{
		ID:   organizationID,
		Name: "Acme",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAccountingProvisioningClientCreatesSignedRequestMetadataWhenAbsent(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	issuer, err := identityaccess.NewServiceIssuerFromSeed(
		"https://pymes.internal",
		"pymes-key-1",
		make([]byte, ed25519.SeedSize),
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims, verifyErr := identityaccess.VerifyInternalCredential(
			authorization,
			issuer.PublicKey(),
			now,
			"https://pymes.internal",
			"accounting-provisioning",
			"org_background",
		)
		if verifyErr != nil {
			t.Fatal(verifyErr)
		}
		if r.Header.Get("X-Request-ID") == "" ||
			r.Header.Get("X-Request-ID") != claims.RequestID ||
			r.Header.Get("X-Correlation-ID") != claims.CorrelationID {
			t.Fatalf("headers=%v claims=%+v", r.Header, claims)
		}
		writeContractJSON(w, http.StatusCreated, map[string]string{"status": "ready"})
	}))
	defer server.Close()

	client := HTTPAccountingProvisioningClient{
		BaseURL: server.URL,
		Tokens: &identityaccess.IssuerTokenSource{
			Issuer:  issuer,
			Subject: "provision-org",
		},
	}
	if err := client.ProvisionOrganization(context.Background(), organizationdomain.Organization{
		ID:   "org_background",
		Name: "Background",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFiscalClientAgainstMockAdapter(t *testing.T) {
	baseURL := os.Getenv("PYMES_FISCAL_TEST_URL")
	if baseURL == "" {
		t.Skip("PYMES_FISCAL_TEST_URL is required")
	}
	snapshot := json.RawMessage(`{"environment":"homologation","concept":"products","issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"21","exempt":"0","total":"121"},"recipient":{"document_type":"CUIT","document_number":"20123456789","vat_condition":"registered"},"lines":[{"description":"Servicio","quantity":"1","unit_price":"100","vat_rate":"21","net":"100"}]}`)
	request := domain.FiscalRequest{RequestID: "fiscal:contract:1", OrganizationID: "org_contract", IdempotencyKey: "fiscal-contract-request-1", SourceVersion: 1, CredentialRef: "mock://credential/contract", Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 1}, Total: domain.Money{Amount: "121", Currency: "ARS"}, SnapshotDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", CorrelationID: "contract", FiscalSnapshot: snapshot}
	tokens, err := identityaccess.TokenSourceFromRuntime("worker:fiscal-contract-test")
	if err != nil {
		t.Fatal(err)
	}
	defer tokens.Close()
	ctx := identityaccess.WithRequestMetadata(context.Background(), identityaccess.RequestMetadata{
		RequestID:     "fiscal-contract-request",
		CorrelationID: request.CorrelationID,
	})
	client := HTTPFiscalClient{BaseURL: baseURL, Tokens: tokens}
	authorized, err := client.Authorize(ctx, request)
	if err != nil || authorized.Status != "authorized" || authorized.CAE == "" ||
		authorized.OrganizationID != request.OrganizationID || authorized.RequestID != request.RequestID ||
		authorized.IdempotencyKey != request.IdempotencyKey || authorized.SourceVersion != request.SourceVersion ||
		authorized.SnapshotDigest != request.SnapshotDigest || authorized.CorrelationID != request.CorrelationID {
		t.Fatalf("authorize=%+v err=%v", authorized, err)
	}
	consulted, err := client.Consult(ctx, request)
	if err != nil || consulted.Status != "authorized" || consulted.CAE != authorized.CAE {
		t.Fatalf("consult=%+v err=%v", consulted, err)
	}
}

func TestAccountingClientForwardsOrganizationAndIdempotency(t *testing.T) {
	commandID := uuid.NewString()
	journalEntryID := uuid.NewString()
	idempotencyKey := "posting-command-" + uuid.NewString()
	snapshotDigest := strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer accounting:org_a" || r.Header.Get("Idempotency-Key") != idempotencyKey || r.Header.Get("X-Correlation-ID") != "sale_1" {
			t.Fatalf("missing internal headers")
		}
		if r.URL.Path != "/internal/v1/organizations/org_a/posting-commands" {
			t.Fatalf("path %s", r.URL.Path)
		}
		var payload struct {
			CommandID      string `json:"command_id"`
			OrganizationID string `json:"organization_id"`
			IdempotencyKey string `json:"idempotency_key"`
			CorrelationID  string `json:"correlation_id"`
			SourceVersion  int    `json:"source_version"`
			SnapshotDigest string `json:"snapshot_digest"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.CommandID != commandID || payload.OrganizationID != "org_a" ||
			payload.IdempotencyKey != idempotencyKey || payload.CorrelationID != "sale_1" ||
			payload.SourceVersion != 1 || payload.SnapshotDigest != snapshotDigest {
			t.Fatalf("universal metadata=%+v", payload)
		}
		writeContractJSON(w, http.StatusCreated, accountingEventFixture(commandID, "org_a", idempotencyKey, "sale_1", snapshotDigest, journalEntryID))
	}))
	defer server.Close()
	result, err := (HTTPAccountingClient{BaseURL: server.URL, Tokens: tokenSource{}}).Post(context.Background(), domain.PostingCommand{
		CommandID:      commandID,
		OrganizationID: "org_a",
		IdempotencyKey: idempotencyKey,
		CorrelationID:  "sale_1",
		SourceType:     "sales_invoice",
		SourceID:       "sale_1",
		SourceVersion:  1,
		SnapshotDigest: snapshotDigest,
		EffectiveAt:    time.Now().UTC(),
		Description:    "Sale",
		Lines: []domain.PostingLine{
			{AccountCode: "1200", Debit: domain.Money{Amount: "1", Currency: "ARS"}, Credit: domain.Money{Amount: "0", Currency: "ARS"}},
			{AccountCode: "4100", Debit: domain.Money{Amount: "0", Currency: "ARS"}, Credit: domain.Money{Amount: "1", Currency: "ARS"}},
		},
	})
	if err != nil || result.JournalEntryID != journalEntryID || result.IdempotencyKey != idempotencyKey {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAccountingGeneratedClientsCarryUniversalMetadataForAllCommands(t *testing.T) {
	type invoke func(HTTPAccountingClient, string, string, string) (domain.AccountingEvent, error)
	tests := []struct {
		name       string
		path       string
		wantStatus string
		invoke     invoke
	}{
		{
			name:       "journal reversal",
			path:       "/internal/v1/organizations/org_a/reversals",
			wantStatus: "reversed",
			invoke: func(client HTTPAccountingClient, commandID, key, digest string) (domain.AccountingEvent, error) {
				return client.Reverse(context.Background(), domain.ReversalCommand{
					CommandID: commandID, OrganizationID: "org_a", IdempotencyKey: key,
					SourceVersion: 7, SnapshotDigest: digest, OriginalJournalEntryID: uuid.NewString(),
					EffectiveAt: time.Now().UTC(), Reason: "cancellation", CorrelationID: "correlation-1",
				})
			},
		},
		{
			name:       "open item application",
			path:       "/internal/v1/organizations/org_a/open-item-applications",
			wantStatus: "applied",
			invoke: func(client HTTPAccountingClient, commandID, key, digest string) (domain.AccountingEvent, error) {
				return client.ApplyOpenItem(context.Background(), domain.AccountingApplicationCommand{
					CommandID: commandID, OrganizationID: "org_a", IdempotencyKey: key,
					SourceVersion: 7, SnapshotDigest: digest, DebitOpenItemID: uuid.NewString(),
					CreditOpenItemID: uuid.NewString(), Amount: domain.Money{Amount: "10", Currency: "ARS"},
					AppliedAt: time.Now().UTC(), CorrelationID: "correlation-1",
				})
			},
		},
		{
			name:       "open item application reversal",
			path:       "/internal/v1/organizations/org_a/open-item-application-reversals",
			wantStatus: "reversed",
			invoke: func(client HTTPAccountingClient, commandID, key, digest string) (domain.AccountingEvent, error) {
				return client.ReverseOpenItemApplication(context.Background(), domain.AccountingApplicationReversalCommand{
					CommandID: commandID, OrganizationID: "org_a", IdempotencyKey: key,
					SourceVersion: 7, SnapshotDigest: digest, ApplicationID: uuid.NewString(),
					ReversedAt: time.Now().UTC(), Reason: "payment reversal", CorrelationID: "correlation-1",
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commandID := uuid.NewString()
			idempotencyKey := "accounting-operation-" + uuid.NewString()
			digest := strings.Repeat("b", 64)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.path || r.Header.Get("Idempotency-Key") != idempotencyKey ||
					r.Header.Get("X-Correlation-ID") != "correlation-1" {
					t.Fatalf("path=%s headers=%v", r.URL.Path, r.Header)
				}
				var payload struct {
					CommandID      string `json:"command_id"`
					OrganizationID string `json:"organization_id"`
					IdempotencyKey string `json:"idempotency_key"`
					CorrelationID  string `json:"correlation_id"`
					SourceVersion  int    `json:"source_version"`
					SnapshotDigest string `json:"snapshot_digest"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.CommandID != commandID || payload.OrganizationID != "org_a" ||
					payload.IdempotencyKey != idempotencyKey || payload.CorrelationID != "correlation-1" ||
					payload.SourceVersion != 7 || payload.SnapshotDigest != digest {
					t.Fatalf("universal metadata=%+v", payload)
				}
				event := accountingEventFixture(commandID, "org_a", idempotencyKey, "correlation-1", digest, uuid.NewString())
				event.SourceVersion = 7
				event.Status = test.wantStatus
				writeContractJSON(w, http.StatusCreated, event)
			}))
			defer server.Close()

			result, err := test.invoke(HTTPAccountingClient{BaseURL: server.URL}, commandID, idempotencyKey, digest)
			if err != nil || result.Status != test.wantStatus || result.IdempotencyKey != idempotencyKey ||
				result.SourceVersion != 7 || result.SnapshotDigest != digest {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestAccountingClientCarriesCurrencyAndOriginalDocumentSemantics(t *testing.T) {
	commandID := uuid.NewString()
	journalEntryID := uuid.NewString()
	idempotencyKey := "credit-note-" + uuid.NewString()
	snapshotDigest := strings.Repeat("d", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ExchangeRate           string                        `json:"exchange_rate"`
			RelatedSource          domain.PostingSourceReference `json:"related_source"`
			OriginalJournalEntryID string                        `json:"original_journal_entry_id"`
			Lines                  []struct {
				FunctionalAmount string `json:"functional_amount"`
			} `json:"lines"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.ExchangeRate != "1250.123457" || payload.RelatedSource.ID != "sale_original" ||
			payload.OriginalJournalEntryID != "journal_original" || len(payload.Lines) != 2 ||
			payload.Lines[0].FunctionalAmount != "151277.44" {
			t.Fatalf("payload=%+v", payload)
		}
		writeContractJSON(w, http.StatusCreated, accountingEventFixture(commandID, "org_a", idempotencyKey, commandID, snapshotDigest, journalEntryID))
	}))
	defer server.Close()
	command := domain.PostingCommand{
		CommandID:      commandID,
		OrganizationID: "org_a",
		IdempotencyKey: idempotencyKey,
		SourceType:     "sales_credit_note",
		SourceID:       "sale_note",
		SourceVersion:  1,
		SnapshotDigest: snapshotDigest,
		CorrelationID:  commandID,
		EffectiveAt:    time.Now().UTC(),
		Description:    "Credit note",
		ExchangeRate:   "1250.123457",
		RelatedSource: &domain.PostingSourceReference{
			Type: "sales_invoice", ID: "sale_original", Version: 1,
		},
		OriginalJournalEntryID: "journal_original",
		Lines: []domain.PostingLine{
			{
				AccountCode:      "1200",
				Debit:            domain.Money{Amount: "0.00", Currency: "USD"},
				Credit:           domain.Money{Amount: "121.01", Currency: "USD"},
				FunctionalAmount: "151277.44",
			},
			{
				AccountCode:      "4100",
				Debit:            domain.Money{Amount: "121.01", Currency: "USD"},
				Credit:           domain.Money{Amount: "0.00", Currency: "USD"},
				FunctionalAmount: "151277.44",
			},
		},
	}
	if _, err := (HTTPAccountingClient{BaseURL: server.URL}).Post(context.Background(), command); err != nil {
		t.Fatal(err)
	}
}

func TestAccountingClientClassifiesPeriodLocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "PERIOD_LOCKED", "title": "period locked", "correlation_id": "locked-period"})
	}))
	defer server.Close()
	_, err := (HTTPAccountingClient{BaseURL: server.URL}).Post(context.Background(), domain.PostingCommand{
		CommandID:      uuid.NewString(),
		OrganizationID: "org_a",
		IdempotencyKey: "locked-period-" + uuid.NewString(),
		SourceType:     "purchase_invoice",
		SourceID:       "purchase_locked",
		SourceVersion:  1,
		SnapshotDigest: strings.Repeat("e", 64),
		CorrelationID:  "locked-period",
		EffectiveAt:    time.Now().UTC(),
		Description:    "Locked purchase",
		Lines: []domain.PostingLine{
			{AccountCode: "5100", Debit: domain.Money{Amount: "1", Currency: "ARS"}, Credit: domain.Money{Amount: "0", Currency: "ARS"}},
			{AccountCode: "2100", Debit: domain.Money{Amount: "0", Currency: "ARS"}, Credit: domain.Money{Amount: "1", Currency: "ARS"}},
		},
	})
	if !errors.Is(err, domain.ErrPeriodLocked) {
		t.Fatalf("err=%v", err)
	}
}

func TestAccountingClientKeepsInternalJWTWhileAddingCloudRunIdentity(t *testing.T) {
	commandID := uuid.NewString()
	idempotencyKey := "cloud-run-" + uuid.NewString()
	snapshotDigest := strings.Repeat("f", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer accounting:org_a" {
			t.Fatalf("internal authorization=%q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Serverless-Authorization") != "Bearer platform:"+"http://"+r.Host {
			t.Fatalf("platform authorization=%q", r.Header.Get("X-Serverless-Authorization"))
		}
		writeContractJSON(w, http.StatusCreated, accountingEventFixture(commandID, "org_a", idempotencyKey, commandID, snapshotDigest, uuid.NewString()))
	}))
	defer server.Close()
	client := HTTPAccountingClient{BaseURL: server.URL, Tokens: tokenSource{}, PlatformTokens: platformTokenSource{}}
	if _, err := client.Post(context.Background(), domain.PostingCommand{
		CommandID:      commandID,
		OrganizationID: "org_a",
		IdempotencyKey: idempotencyKey,
		SourceType:     "sales_invoice",
		SourceID:       "sale_cloud",
		SourceVersion:  1,
		SnapshotDigest: snapshotDigest,
		CorrelationID:  commandID,
		EffectiveAt:    time.Now().UTC(),
		Description:    "Cloud sale",
		Lines: []domain.PostingLine{
			{AccountCode: "1200", Debit: domain.Money{Amount: "1", Currency: "ARS"}, Credit: domain.Money{Amount: "0", Currency: "ARS"}},
			{AccountCode: "4100", Debit: domain.Money{Amount: "0", Currency: "ARS"}, Credit: domain.Money{Amount: "1", Currency: "ARS"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFiscalGeneratedClientForwardsUniversalMetadata(t *testing.T) {
	idempotencyKey := "fiscal-request-0001"
	snapshotDigest := strings.Repeat("c", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Correlation-ID") != "sale/correlation" || r.Header.Get("Authorization") != "Bearer fiscal:org_a" ||
			r.Header.Get("Idempotency-Key") != idempotencyKey {
			t.Fatalf("headers=%v", r.Header)
		}
		if r.URL.Path != "/internal/v1/organizations/org_a/authorizations" {
			t.Fatalf("path %s", r.URL.Path)
		}
		var payload struct {
			RequestID      string `json:"request_id"`
			OrganizationID string `json:"organization_id"`
			IdempotencyKey string `json:"idempotency_key"`
			CorrelationID  string `json:"correlation_id"`
			SourceVersion  int    `json:"source_version"`
			SnapshotDigest string `json:"snapshot_digest"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.RequestID != "fiscal/request:1" || payload.OrganizationID != "org_a" ||
			payload.IdempotencyKey != idempotencyKey || payload.CorrelationID != "sale/correlation" ||
			payload.SourceVersion != 3 || payload.SnapshotDigest != snapshotDigest {
			t.Fatalf("universal metadata=%+v", payload)
		}
		writeContractJSON(w, http.StatusCreated, domain.FiscalResult{
			RequestID:      payload.RequestID,
			OrganizationID: payload.OrganizationID,
			IdempotencyKey: payload.IdempotencyKey,
			SourceVersion:  payload.SourceVersion,
			Status:         "authorized",
			CAE:            "12345678901234",
			SnapshotDigest: payload.SnapshotDigest,
			ObservedAt:     time.Now().UTC().Format(time.RFC3339Nano),
			CorrelationID:  payload.CorrelationID,
		})
	}))
	defer server.Close()
	snapshot := json.RawMessage(`{"environment":"homologation","issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"21","exempt":"0","total":"121"},"recipient":{"document_type":"CUIT","document_number":"20123456789","vat_condition":"registered"},"lines":[{"description":"Servicio","quantity":"1","unit_price":"100","vat_rate":"21","net":"100"}]}`)
	request := domain.FiscalRequest{RequestID: "fiscal/request:1", OrganizationID: "org_a", IdempotencyKey: idempotencyKey, SourceVersion: 3, CredentialRef: "mock://credential", Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 1}, SnapshotDigest: snapshotDigest, CorrelationID: "sale/correlation", FiscalSnapshot: snapshot}
	tokens := &metadataTokenSource{}
	requestContext := identityusecases.WithRequestMetadata(
		context.Background(),
		identityusecases.RequestMetadata{},
	)
	result, err := (HTTPFiscalClient{BaseURL: server.URL, Tokens: tokens}).Authorize(requestContext, request)
	if err != nil || result.CAE == "" || result.IdempotencyKey != request.IdempotencyKey ||
		result.SourceVersion != request.SourceVersion || result.SnapshotDigest != request.SnapshotDigest {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if tokens.metadata.CorrelationID != request.CorrelationID ||
		tokens.metadata.RequestID != request.CorrelationID {
		t.Fatalf("token metadata does not match outbound headers: %+v", tokens.metadata)
	}
}
