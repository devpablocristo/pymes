package fakeservice

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	accountingapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/models"
	fiscalapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/models"
	"github.com/google/uuid"
)

func TestHandlerForKindUsesGeneratedRouters(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"accounting", "fiscal"} {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			handler, err := HandlerForKind(kind)
			if err != nil {
				t.Fatalf("handlerForKind(%q): %v", kind, err)
			}
			request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("health status = %d, want %d", response.Code, http.StatusOK)
			}
		})
	}

	if _, err := HandlerForKind("unknown"); err == nil {
		t.Fatal("handlerForKind(unknown) succeeded")
	}
}

func TestAccountingGeneratedClientServerConformance(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(accountingapi.Handler(newAccountingFakeServer()))
	t.Cleanup(server.Close)
	client, err := accountingapi.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("new accounting client: %v", err)
	}

	commandID := uuid.New()
	digest := strings.Repeat("a", 64)
	openItem := true
	command := accountingapi.PostingCommand{
		CommandId:      commandID,
		CorrelationId:  "corr-accounting",
		Description:    "conformance posting",
		EffectiveAt:    time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC),
		IdempotencyKey: "accounting-conformance-key",
		Lines: []accountingapi.PostingLine{
			{AccountCode: "1.1.01", Debit: "121.00", Credit: "0", Currency: "ARS", OpenItem: &openItem},
			{AccountCode: "4.1.01", Debit: "0", Credit: "121.00", Currency: "ARS"},
		},
		OrganizationId: "org_conformance",
		SnapshotDigest: digest,
		Source: accountingapi.SourceRef{
			Digest:  &digest,
			Id:      "sale_conformance",
			Type:    "sales_invoice",
			Version: 1,
		},
		SourceVersion: 1,
	}
	params := &accountingapi.SubmitPostingCommandParams{
		IdempotencyKey: command.IdempotencyKey,
		XCorrelationID: command.CorrelationId,
	}

	first, err := client.SubmitPostingCommandWithResponse(
		context.Background(),
		command.OrganizationId,
		params,
		command,
	)
	if err != nil {
		t.Fatalf("submit posting: %v", err)
	}
	if first.StatusCode() != http.StatusCreated || first.JSON201 == nil {
		t.Fatalf("first response = %d %s", first.StatusCode(), first.Body)
	}
	if first.JSON201.Status != accountingapi.Posted ||
		first.JSON201.CommandId != commandID ||
		first.JSON201.JournalEntryId == nil ||
		first.JSON201.OpenItemIds == nil ||
		len(*first.JSON201.OpenItemIds) != 1 {
		t.Fatalf("invalid posting event: %#v", first.JSON201)
	}

	replay, err := client.SubmitPostingCommandWithResponse(
		context.Background(),
		command.OrganizationId,
		params,
		command,
	)
	if err != nil {
		t.Fatalf("replay posting: %v", err)
	}
	if replay.StatusCode() != http.StatusOK || replay.JSON200 == nil {
		t.Fatalf("replay response = %d %s", replay.StatusCode(), replay.Body)
	}
	if replay.JSON200.Status != accountingapi.Duplicate ||
		replay.JSON200.EventId != first.JSON201.EventId ||
		*replay.JSON200.JournalEntryId != *first.JSON201.JournalEntryId {
		t.Fatalf("non-idempotent replay: first=%#v replay=%#v", first.JSON201, replay.JSON200)
	}

	assertGeneratedRouterRejectsMissingHeader(
		t,
		accountingapi.Handler(newAccountingFakeServer()),
		"/internal/v1/organizations/org_conformance/posting-commands",
		command,
		"X-Correlation-ID",
		command.CorrelationId,
	)
}

func TestFiscalGeneratedClientServerConformance(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(fiscalapi.Handler(newFiscalFakeServer()))
	t.Cleanup(server.Close)
	client, err := fiscalapi.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("new fiscal client: %v", err)
	}

	request := fiscalRequestFixture(t)
	params := &fiscalapi.RequestAuthorizationParams{
		IdempotencyKey: request.IdempotencyKey,
		XCorrelationID: request.CorrelationId,
	}
	authorized, err := client.RequestAuthorizationWithResponse(
		context.Background(),
		request.OrganizationId,
		params,
		request,
	)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if authorized.StatusCode() != http.StatusCreated || authorized.JSON201 == nil {
		t.Fatalf("authorization response = %d %s", authorized.StatusCode(), authorized.Body)
	}
	if authorized.JSON201.Status != fiscalapi.Authorized ||
		authorized.JSON201.Cae == nil ||
		authorized.JSON201.RequestId != request.RequestId {
		t.Fatalf("invalid fiscal result: %#v", authorized.JSON201)
	}

	consulted, err := client.ConsultAuthorizationWithResponse(
		context.Background(),
		request.OrganizationId,
		request.RequestId,
		&fiscalapi.ConsultAuthorizationParams{
			IdempotencyKey: request.IdempotencyKey,
			XCorrelationID: request.CorrelationId,
		},
		request,
	)
	if err != nil {
		t.Fatalf("consult: %v", err)
	}
	if consulted.StatusCode() != http.StatusOK || consulted.JSON200 == nil {
		t.Fatalf("consult response = %d %s", consulted.StatusCode(), consulted.Body)
	}
	if consulted.JSON200.Status != fiscalapi.Authorized ||
		consulted.JSON200.Cae == nil ||
		*consulted.JSON200.Cae != *authorized.JSON201.Cae {
		t.Fatalf("consult did not return stored result: %#v", consulted.JSON200)
	}

	assertGeneratedRouterRejectsMissingHeader(
		t,
		fiscalapi.Handler(newFiscalFakeServer()),
		"/internal/v1/organizations/org_conformance/authorizations",
		request,
		"X-Correlation-ID",
		request.CorrelationId,
	)
}

func TestAccountingGeneratedServerConcurrentReplay(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(accountingapi.Handler(newAccountingFakeServer()))
	t.Cleanup(server.Close)
	client, err := accountingapi.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("new accounting client: %v", err)
	}

	digest := strings.Repeat("c", 64)
	command := accountingapi.PostingCommand{
		CommandId:      uuid.New(),
		CorrelationId:  "corr-concurrent",
		Description:    "concurrent replay",
		EffectiveAt:    time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC),
		IdempotencyKey: "accounting-concurrent-key",
		Lines: []accountingapi.PostingLine{
			{AccountCode: "1.1.01", Debit: "1.00", Credit: "0", Currency: "ARS"},
			{AccountCode: "4.1.01", Debit: "0", Credit: "1.00", Currency: "ARS"},
		},
		OrganizationId: "org_concurrent",
		SnapshotDigest: digest,
		Source:         accountingapi.SourceRef{Digest: &digest, Id: "sale_concurrent", Type: "sales_invoice", Version: 1},
		SourceVersion:  1,
	}
	params := &accountingapi.SubmitPostingCommandParams{
		IdempotencyKey: command.IdempotencyKey,
		XCorrelationID: command.CorrelationId,
	}

	const requests = 24
	statuses := make(chan int, requests)
	errors := make(chan error, requests)
	var group sync.WaitGroup
	for range requests {
		group.Add(1)
		go func() {
			defer group.Done()
			response, callErr := client.SubmitPostingCommandWithResponse(
				context.Background(),
				command.OrganizationId,
				params,
				command,
			)
			if callErr != nil {
				errors <- callErr
				return
			}
			statuses <- response.StatusCode()
		}()
	}
	group.Wait()
	close(errors)
	close(statuses)

	for callErr := range errors {
		t.Errorf("concurrent replay: %v", callErr)
	}
	created := 0
	duplicates := 0
	for status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			duplicates++
		default:
			t.Errorf("unexpected replay status: %d", status)
		}
	}
	if created != 1 || duplicates != requests-1 {
		t.Fatalf("created=%d duplicate=%d, want 1/%d", created, duplicates, requests-1)
	}
}

func assertGeneratedRouterRejectsMissingHeader(
	t *testing.T,
	handler http.Handler,
	path string,
	body any,
	headerName,
	headerValue string,
) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(headerName, headerValue)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing required generated header accepted: status=%d body=%s", response.Code, response.Body.String())
	}
}

func fiscalRequestFixture(t *testing.T) fiscalapi.FiscalRequest {
	t.Helper()
	const raw = `{
		"correlation_id":"corr-fiscal",
		"credential_ref":"kms://fake/credential",
		"currency":"ARS",
		"document_type":"FA",
		"environment":"homologation",
		"idempotency_key":"fiscal-conformance-key",
		"issue_date":"2026-07-31",
		"lines":[{"description":"service","net":"100.00","quantity":"1","unit_price":"100.00","vat_rate":"21"}],
		"organization_id":"org_conformance",
		"point_of_sale":1,
		"recipient":{"document_number":"20123456789","document_type":"80","vat_condition":"responsable_inscripto"},
		"request_id":"fiscal_request_conformance",
		"snapshot_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"source_version":1,
		"totals":{"exempt":"0","net":"100.00","total":"121.00","vat":"21.00"},
		"voucher_number":1
	}`
	var request fiscalapi.FiscalRequest
	if err := json.Unmarshal([]byte(raw), &request); err != nil {
		t.Fatalf("decode fiscal fixture: %v", err)
	}
	return request
}
