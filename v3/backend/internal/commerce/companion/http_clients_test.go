package companion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity/access"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/domain"
	"github.com/google/uuid"
)

type tokenSource struct{}

func (tokenSource) Token(_ context.Context, audience, organizationID string) (string, error) {
	return audience + ":" + organizationID, nil
}

func TestAccountingClientAgainstHeadlessService(t *testing.T) {
	baseURL := os.Getenv("PYMES_ACCOUNTING_TEST_URL")
	if baseURL == "" {
		t.Skip("PYMES_ACCOUNTING_TEST_URL is required")
	}
	tokens, err := identityaccess.TokenSourceFromRuntime("worker:accounting-contract-test")
	if err != nil {
		t.Fatal(err)
	}
	client := HTTPAccountingClient{BaseURL: baseURL, Tokens: tokens}
	organizationID := "org_contract_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := client.ProvisionOrganization(context.Background(), organizationdomain.Organization{ID: organizationID, Name: "Contract Test"}); err != nil {
		t.Fatal(err)
	}
	command := domain.PostingCommand{
		CommandID: uuid.NewString(), OrganizationID: organizationID, SourceType: "sales_invoice",
		SourceID: "sale-contract", SourceVersion: 1,
		SnapshotDigest: strings.Repeat("a", 64), CorrelationID: "accounting-contract",
		EffectiveAt: time.Now().UTC(), Description: "Contract sale",
		Lines: []domain.PostingLine{
			{AccountCode: "1200", Debit: domain.Money{Amount: "121", Currency: "ARS"}, Credit: domain.Money{Amount: "0", Currency: "ARS"}, OpenItem: true, PartyRef: "party-contract"},
			{AccountCode: "4100", Debit: domain.Money{Amount: "0", Currency: "ARS"}, Credit: domain.Money{Amount: "121", Currency: "ARS"}},
		},
	}
	posted, err := client.Post(context.Background(), command)
	if err != nil || posted.Status != "posted" || posted.JournalEntryID == "" || len(posted.OpenItemIDs) != 1 ||
		posted.CorrelationID != command.CorrelationID {
		t.Fatalf("posted=%+v err=%v", posted, err)
	}
	duplicate, err := client.Post(context.Background(), command)
	if err != nil || duplicate.JournalEntryID != posted.JournalEntryID || len(duplicate.OpenItemIDs) != 1 {
		t.Fatalf("duplicate=%+v err=%v", duplicate, err)
	}
	changed := command
	changed.SnapshotDigest = strings.Repeat("b", 64)
	if _, err := client.Post(context.Background(), changed); err == nil || !strings.Contains(err.Error(), "IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("changed payload err=%v", err)
	}
}

func TestFiscalClientAgainstMockAdapter(t *testing.T) {
	baseURL := os.Getenv("PYMES_FISCAL_TEST_URL")
	if baseURL == "" {
		t.Skip("PYMES_FISCAL_TEST_URL is required")
	}
	snapshot := json.RawMessage(`{"environment":"homologation","issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"21","exempt":"0","total":"121"},"recipient":{"document_type":"CUIT","document_number":"20123456789","vat_condition":"registered"},"lines":[{"description":"Servicio","quantity":"1","unit_price":"100","vat_rate":"21","net":"100"}]}`)
	request := domain.FiscalRequest{RequestID: "fiscal:contract:1", OrganizationID: "org_contract", CredentialRef: "mock://credential/contract", Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 1}, Total: domain.Money{Amount: "121", Currency: "ARS"}, SnapshotDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", CorrelationID: "contract", FiscalSnapshot: snapshot}
	tokens, err := identityaccess.TokenSourceFromRuntime("worker:fiscal-contract-test")
	if err != nil {
		t.Fatal(err)
	}
	client := HTTPFiscalClient{BaseURL: baseURL, Tokens: tokens}
	authorized, err := client.Authorize(context.Background(), request)
	if err != nil || authorized.Status != "authorized" || authorized.CAE == "" || authorized.CorrelationID != request.CorrelationID {
		t.Fatalf("authorize=%+v err=%v", authorized, err)
	}
	consulted, err := client.Consult(context.Background(), request)
	if err != nil || consulted.Status != "authorized" || consulted.CAE != authorized.CAE {
		t.Fatalf("consult=%+v err=%v", consulted, err)
	}
}

func TestAccountingClientForwardsOrganizationAndIdempotency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer accounting:org_a" || r.Header.Get("Idempotency-Key") != "cmd_1" || r.Header.Get("X-Correlation-ID") != "sale_1" {
			t.Fatalf("missing internal headers")
		}
		if r.URL.Path != "/internal/v1/organizations/org_a/posting-commands" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(domain.AccountingEvent{CommandID: "cmd_1", OrganizationID: "org_a", Status: "posted", JournalEntryID: "je_1"})
	}))
	defer server.Close()
	result, err := (HTTPAccountingClient{BaseURL: server.URL, Tokens: tokenSource{}}).Post(context.Background(), domain.PostingCommand{CommandID: "cmd_1", OrganizationID: "org_a", CorrelationID: "sale_1"})
	if err != nil || result.JournalEntryID != "je_1" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestFiscalClientForwardsCorrelationAndEscapesIdentifiers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Correlation-ID") != "sale/correlation" || r.Header.Get("Authorization") != "Bearer fiscal:org/a" {
			t.Fatalf("headers=%v", r.Header)
		}
		if r.URL.Path != "/internal/v1/organizations/org/a/authorizations" {
			t.Fatalf("decoded path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(domain.FiscalResult{RequestID: "fiscal/request:1", Status: "authorized", CAE: "12345678901234"})
	}))
	defer server.Close()
	snapshot := json.RawMessage(`{"environment":"homologation","issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"21","exempt":"0","total":"121"},"recipient":{"document_type":"CUIT","document_number":"20123456789","vat_condition":"registered"},"lines":[{"description":"Servicio","quantity":"1","unit_price":"100","vat_rate":"21","net":"100"}]}`)
	request := domain.FiscalRequest{RequestID: "fiscal/request:1", OrganizationID: "org/a", CredentialRef: "mock://credential", Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 1}, SnapshotDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", CorrelationID: "sale/correlation", FiscalSnapshot: snapshot}
	result, err := (HTTPFiscalClient{BaseURL: server.URL, Tokens: tokenSource{}}).Authorize(context.Background(), request)
	if err != nil || result.CAE == "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
