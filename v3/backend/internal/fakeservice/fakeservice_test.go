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
	accountingmodels "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/accounting/models"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
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

func TestAccountingGeneratedReportPaginationConformance(t *testing.T) {
	t.Parallel()

	fake := newAccountingFakeServer()
	server := httptest.NewServer(accountingapi.Handler(fake))
	t.Cleanup(server.Close)
	client, err := accountingapi.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("new accounting client: %v", err)
	}

	const organizationID = "org_report_pagination"
	asOf := openapi_types.Date{
		Time: time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC),
	}
	for index := 0; index < 3; index++ {
		postAccountingFakeLedgerEntry(
			t,
			client,
			organizationID,
			index,
			time.Date(2026, time.July, 31, 12-index, 0, 0, 0, time.UTC),
		)
	}

	defaultPage, err := client.GetReportWithResponse(
		context.Background(),
		organizationID,
		accountingapi.GetReportParamsReportGeneralLedger,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			XCorrelationID: "corr-report-default",
		},
	)
	if err != nil {
		t.Fatalf("get default report page: %v", err)
	}
	if defaultPage.StatusCode() != http.StatusOK || defaultPage.JSON200 == nil {
		t.Fatalf(
			"default report response = %d %s",
			defaultPage.StatusCode(),
			defaultPage.Body,
		)
	}
	if defaultPage.JSON200.Entries == nil ||
		len(*defaultPage.JSON200.Entries) != 3 ||
		defaultPage.JSON200.HasMore == nil ||
		*defaultPage.JSON200.HasMore ||
		defaultPage.JSON200.NextCursor != nil ||
		defaultPage.JSON200.OrganizationId == nil ||
		*defaultPage.JSON200.OrganizationId != organizationID ||
		defaultPage.JSON200.Report == nil ||
		*defaultPage.JSON200.Report != accountingapi.AccountingReportReportGeneralLedger ||
		defaultPage.JSON200.AsOf == nil ||
		defaultPage.JSON200.AsOf.Time.Format("2006-01-02") != "2026-07-31" {
		t.Fatalf("default report page = %#v", defaultPage.JSON200)
	}

	limit := 2
	firstPage, err := client.GetReportWithResponse(
		context.Background(),
		organizationID,
		accountingapi.GetReportParamsReportGeneralLedger,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			Limit:          &limit,
			XCorrelationID: "corr-report-explicit",
		},
	)
	if err != nil {
		t.Fatalf("get first report page: %v", err)
	}
	if firstPage.StatusCode() != http.StatusOK || firstPage.JSON200 == nil ||
		firstPage.JSON200.Entries == nil ||
		len(*firstPage.JSON200.Entries) != limit ||
		firstPage.JSON200.HasMore == nil ||
		!*firstPage.JSON200.HasMore ||
		firstPage.JSON200.NextCursor == nil ||
		*firstPage.JSON200.NextCursor == "" {
		t.Fatalf(
			"first report response = %d %s %#v",
			firstPage.StatusCode(),
			firstPage.Body,
			firstPage.JSON200,
		)
	}
	cursor := *firstPage.JSON200.NextCursor

	repeatedFirstPage, err := client.GetReportWithResponse(
		context.Background(),
		organizationID,
		accountingapi.GetReportParamsReportGeneralLedger,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			Limit:          &limit,
			XCorrelationID: "corr-report-repeat",
		},
	)
	if err != nil || repeatedFirstPage.JSON200 == nil ||
		repeatedFirstPage.JSON200.NextCursor == nil ||
		*repeatedFirstPage.JSON200.NextCursor != cursor {
		t.Fatalf("non-deterministic first-page cursor: response=%#v err=%v", repeatedFirstPage, err)
	}

	secondPage, err := client.GetReportWithResponse(
		context.Background(),
		organizationID,
		accountingapi.GetReportParamsReportGeneralLedger,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			Limit:          &limit,
			Cursor:         &cursor,
			XCorrelationID: "corr-report-continuation",
		},
	)
	if err != nil {
		t.Fatalf("get continuation report page: %v", err)
	}
	if secondPage.StatusCode() != http.StatusOK || secondPage.JSON200 == nil ||
		secondPage.JSON200.Entries == nil ||
		len(*secondPage.JSON200.Entries) != 1 ||
		secondPage.JSON200.HasMore == nil ||
		*secondPage.JSON200.HasMore ||
		secondPage.JSON200.NextCursor != nil {
		t.Fatalf(
			"continuation response = %d %s %#v",
			secondPage.StatusCode(),
			secondPage.Body,
			secondPage.JSON200,
		)
	}
	seen := make(map[uuid.UUID]struct{}, 3)
	for _, entry := range *firstPage.JSON200.Entries {
		seen[entry.Id] = struct{}{}
	}
	for _, entry := range *secondPage.JSON200.Entries {
		if _, duplicate := seen[entry.Id]; duplicate {
			t.Fatalf("entry %s repeated across report pages", entry.Id)
		}
		seen[entry.Id] = struct{}{}
	}
	if len(seen) != 3 {
		t.Fatalf("paginated entries=%d, want 3", len(seen))
	}

	fake.mu.Lock()
	reportPages := append([]accountingmodels.ReportPageRequest(nil), fake.reportPages...)
	fake.mu.Unlock()
	if len(reportPages) != 4 ||
		reportPages[0].Limit != 200 || reportPages[0].Cursor != "" ||
		reportPages[1].Limit != limit || reportPages[1].Cursor != "" ||
		reportPages[2].Limit != limit || reportPages[2].Cursor != "" ||
		reportPages[3].Limit != limit || reportPages[3].Cursor != cursor {
		t.Fatalf("normalized report pages = %#v", reportPages)
	}

	invalidLimit := 501
	rejected, err := client.GetReportWithResponse(
		context.Background(),
		organizationID,
		accountingapi.GetReportParamsReportGeneralLedger,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			Limit:          &invalidLimit,
			XCorrelationID: "corr-report-invalid",
		},
	)
	if err != nil {
		t.Fatalf("get invalid report page: %v", err)
	}
	if rejected.StatusCode() != http.StatusBadRequest ||
		rejected.ApplicationproblemJSON400 == nil {
		t.Fatalf(
			"invalid limit response = %d %s",
			rejected.StatusCode(),
			rejected.Body,
		)
	}

	emptyCursor := ""
	rejected, err = client.GetReportWithResponse(
		context.Background(),
		organizationID,
		accountingapi.GetReportParamsReportGeneralLedger,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			Cursor:         &emptyCursor,
			XCorrelationID: "corr-report-empty-cursor",
		},
	)
	if err != nil {
		t.Fatalf("get empty-cursor report page: %v", err)
	}
	if rejected.StatusCode() != http.StatusBadRequest ||
		rejected.ApplicationproblemJSON400 == nil {
		t.Fatalf(
			"empty cursor response = %d %s",
			rejected.StatusCode(),
			rejected.Body,
		)
	}

	longCursor := strings.Repeat("x", 2049)
	rejected, err = client.GetReportWithResponse(
		context.Background(),
		organizationID,
		accountingapi.GetReportParamsReportGeneralLedger,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			Cursor:         &longCursor,
			XCorrelationID: "corr-report-long-cursor",
		},
	)
	if err != nil {
		t.Fatalf("get long-cursor report page: %v", err)
	}
	if rejected.StatusCode() != http.StatusBadRequest ||
		rejected.ApplicationproblemJSON400 == nil {
		t.Fatalf(
			"long cursor response = %d %s",
			rejected.StatusCode(),
			rejected.Body,
		)
	}

	rejected, err = client.GetReportWithResponse(
		context.Background(),
		organizationID,
		accountingapi.GetReportParamsReportTrialBalance,
		&accountingapi.GetReportParams{
			AsOf:           asOf,
			Limit:          &limit,
			XCorrelationID: "corr-report-incompatible",
		},
	)
	if err != nil {
		t.Fatalf("get incompatible report pagination: %v", err)
	}
	if rejected.StatusCode() != http.StatusBadRequest ||
		rejected.ApplicationproblemJSON400 == nil {
		t.Fatalf(
			"incompatible report response = %d %s",
			rejected.StatusCode(),
			rejected.Body,
		)
	}
}

func postAccountingFakeLedgerEntry(
	t *testing.T,
	client *accountingapi.ClientWithResponses,
	organizationID string,
	index int,
	effectiveAt time.Time,
) {
	t.Helper()
	digest := strings.Repeat(string(rune('a'+index)), 64)
	commandID := uuid.New()
	command := accountingapi.PostingCommand{
		CommandId:      commandID,
		CorrelationId:  "corr-report-posting",
		Description:    "report posting",
		EffectiveAt:    effectiveAt,
		IdempotencyKey: "accounting-report-" + commandID.String(),
		Lines: []accountingapi.PostingLine{
			{
				AccountCode: "1200",
				Credit:      "0",
				Currency:    "ARS",
				Debit:       "1",
			},
			{
				AccountCode: "4100",
				Credit:      "1",
				Currency:    "ARS",
				Debit:       "0",
			},
		},
		OrganizationId: organizationID,
		SnapshotDigest: digest,
		Source: accountingapi.SourceRef{
			Digest:  &digest,
			Id:      "sale-report-" + commandID.String(),
			Type:    "sales_invoice",
			Version: 1,
		},
		SourceVersion: 1,
	}
	response, err := client.SubmitPostingCommandWithResponse(
		context.Background(),
		organizationID,
		&accountingapi.SubmitPostingCommandParams{
			IdempotencyKey: command.IdempotencyKey,
			XCorrelationID: command.CorrelationId,
		},
		command,
	)
	if err != nil {
		t.Fatalf("post report fixture %d: %v", index, err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		t.Fatalf(
			"post report fixture %d response=%d %s",
			index,
			response.StatusCode(),
			response.Body,
		)
	}
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
	if authorized.JSON201.Status != fiscalapi.FiscalResultStatusAuthorized ||
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
	if consulted.JSON200.Status != fiscalapi.FiscalResultStatusAuthorized ||
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

func TestFiscalGeneratedClientCredentialOnboardingConformance(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(fiscalapi.Handler(newFiscalFakeServer()))
	t.Cleanup(server.Close)
	client, err := fiscalapi.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("new fiscal client: %v", err)
	}

	ctx := context.Background()
	organizationID := "org_fiscal_onboarding"
	correlationID := "corr-fiscal-onboarding"
	csr, err := client.RequestFiscalCredentialCSRWithResponse(
		ctx,
		organizationID,
		&fiscalapi.RequestFiscalCredentialCSRParams{
			IdempotencyKey: "csr-key",
			XCorrelationID: correlationID,
		},
		fiscalapi.CSRRequest{
			CommonName:  "org-fiscal-onboarding",
			Cuit:        "30712345678",
			Environment: fiscalapi.CSRRequestEnvironmentHomologation,
			LegalName:   "Pyme de prueba SA",
		},
	)
	if err != nil {
		t.Fatalf("request CSR: %v", err)
	}
	if csr.StatusCode() != http.StatusCreated || csr.JSON201 == nil {
		t.Fatalf("CSR response = %d %s", csr.StatusCode(), csr.Body)
	}
	if !strings.Contains(csr.JSON201.CsrPem, "BEGIN CERTIFICATE REQUEST") ||
		csr.JSON201.Credential.Status != fiscalapi.FiscalCredentialStatusPendingCertificate {
		t.Fatalf("invalid CSR result: %#v", csr.JSON201)
	}

	credentialID := csr.JSON201.Credential.Id
	uploaded, err := client.UploadFiscalCertificateWithResponse(
		ctx,
		organizationID,
		credentialID,
		&fiscalapi.UploadFiscalCertificateParams{XCorrelationID: correlationID},
		fiscalapi.CertificateUpload{
			CertificatePem:  "-----BEGIN CERTIFICATE-----\nFAKE\n-----END CERTIFICATE-----\n",
			ExpectedVersion: csr.JSON201.Credential.Version,
		},
	)
	if err != nil {
		t.Fatalf("upload certificate: %v", err)
	}
	if uploaded.StatusCode() != http.StatusOK || uploaded.JSON200 == nil ||
		uploaded.JSON200.Status != fiscalapi.FiscalCredentialStatusReady ||
		uploaded.JSON200.CertificateFingerprint == nil {
		t.Fatalf("invalid uploaded credential: %#v", uploaded)
	}

	configured, err := client.ConfigureFiscalPointOfSaleWithResponse(
		ctx,
		organizationID,
		credentialID,
		1,
		&fiscalapi.ConfigureFiscalPointOfSaleParams{XCorrelationID: correlationID},
		fiscalapi.ConfigureFiscalPointOfSaleJSONRequestBody{Enabled: false},
	)
	if err != nil {
		t.Fatalf("configure point of sale: %v", err)
	}
	if configured.StatusCode() != http.StatusOK || configured.JSON200 == nil ||
		configured.JSON200.ValidatedAt != nil {
		t.Fatalf("invalid configured point of sale: %#v", configured)
	}

	validated, err := client.ValidateFiscalPointOfSaleWithResponse(
		ctx,
		organizationID,
		credentialID,
		1,
		&fiscalapi.ValidateFiscalPointOfSaleParams{XCorrelationID: correlationID},
		fiscalapi.ValidateFiscalPointOfSaleJSONRequestBody{Enabled: true},
	)
	if err != nil {
		t.Fatalf("validate point of sale: %v", err)
	}
	if validated.StatusCode() != http.StatusOK || validated.JSON200 == nil ||
		!validated.JSON200.Enabled || validated.JSON200.ValidatedAt == nil {
		t.Fatalf("invalid validated point of sale: %#v", validated)
	}

	crossTenant, err := client.GetFiscalCredentialWithResponse(
		ctx,
		"org_other",
		credentialID,
		&fiscalapi.GetFiscalCredentialParams{XCorrelationID: correlationID},
	)
	if err != nil {
		t.Fatalf("cross-tenant credential lookup: %v", err)
	}
	if crossTenant.StatusCode() != http.StatusNotFound {
		t.Fatalf("cross-tenant lookup status = %d, want %d", crossTenant.StatusCode(), http.StatusNotFound)
	}
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
