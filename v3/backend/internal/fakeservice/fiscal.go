// architecture:adapter external
package fakeservice

import (
	"net/http"
	"sync"
	"time"

	fiscalapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/models"
	fiscalhelpers "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/fiscal/helpers"
	fiscalmodels "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/fiscal/models"
)

var _ fiscalapi.ServerInterface = (*fiscalFakeServer)(nil)

type fiscalFakeServer struct {
	mu            sync.Mutex
	results       map[string]fiscalapi.FiscalResult
	idempotencies map[string]fiscalapi.FiscalResult
}

func newFiscalFakeServer() *fiscalFakeServer {
	return &fiscalFakeServer{
		results:       make(map[string]fiscalapi.FiscalResult),
		idempotencies: make(map[string]fiscalapi.FiscalResult),
	}
}

func (s *fiscalFakeServer) FiscalHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, fiscalapi.HealthStatus{Status: fiscalapi.Ok})
}

func (s *fiscalFakeServer) FiscalReadiness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, fiscalapi.ReadinessStatus{Status: fiscalapi.Ready})
}

func (s *fiscalFakeServer) FiscalMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("pymes_fiscal_fake_ready 1\n"))
}

func (s *fiscalFakeServer) ListDocumentTypes(
	w http.ResponseWriter,
	_ *http.Request,
	_ fiscalapi.ListDocumentTypesParams,
) {
	writeJSON(w, http.StatusOK, []map[string]string{
		{"code": "FA", "letter": "A", "kind": "invoice"},
		{"code": "NCA", "letter": "A", "kind": "credit_note"},
		{"code": "NDA", "letter": "A", "kind": "debit_note"},
		{"code": "FB", "letter": "B", "kind": "invoice"},
		{"code": "NCB", "letter": "B", "kind": "credit_note"},
		{"code": "NDB", "letter": "B", "kind": "debit_note"},
		{"code": "FC", "letter": "C", "kind": "invoice"},
		{"code": "NCC", "letter": "C", "kind": "credit_note"},
		{"code": "NDC", "letter": "C", "kind": "debit_note"},
	})
}

func (s *fiscalFakeServer) RequestAuthorization(
	w http.ResponseWriter,
	r *http.Request,
	organizationID fiscalapi.OrganizationId,
	params fiscalapi.RequestAuthorizationParams,
) {
	var body fiscalapi.FiscalRequest
	if !decodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateFiscalMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body) {
		return
	}

	cae := "CAE-" + body.RequestId
	authorityCode := "A"
	artifactRef := "fake://fiscal/" + organizationID + "/" + body.RequestId
	result := fiscalapi.FiscalResult{
		ArtifactRef:         &artifactRef,
		AuthorityResultCode: &authorityCode,
		Cae:                 &cae,
		CaeExpiresOn:        &body.IssueDate,
		CorrelationId:       body.CorrelationId,
		IdempotencyKey:      body.IdempotencyKey,
		ObservedAt:          time.Now().UTC(),
		OrganizationId:      organizationID,
		RequestId:           body.RequestId,
		SnapshotDigest:      body.SnapshotDigest,
		SourceVersion:       body.SourceVersion,
		Status:              fiscalapi.Authorized,
	}
	requestKey := fiscalResultKey(organizationID, body.RequestId)
	idempotencyKey := fiscalResultKey(organizationID, body.IdempotencyKey)
	s.mu.Lock()
	stored, found := s.idempotencies[idempotencyKey]
	if !found {
		s.idempotencies[idempotencyKey] = result
		s.results[requestKey] = result
		stored = result
	}
	s.mu.Unlock()
	if found && (stored.SnapshotDigest != result.SnapshotDigest || stored.RequestId != result.RequestId) {
		writeProblem(
			w,
			http.StatusConflict,
			body.CorrelationId,
			"IDEMPOTENCY_KEY_REUSED",
			"idempotency key reused",
			"the request was already stored with different content",
		)
		return
	}
	writeJSON(w, http.StatusCreated, stored)
}

func (s *fiscalFakeServer) ConsultAuthorization(
	w http.ResponseWriter,
	r *http.Request,
	organizationID fiscalapi.OrganizationId,
	requestID string,
	params fiscalapi.ConsultAuthorizationParams,
) {
	var body fiscalapi.FiscalRequest
	if !decodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateFiscalMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body) {
		return
	}
	if requestID != body.RequestId {
		writeProblem(w, http.StatusBadRequest, params.XCorrelationID, "VALIDATION_ERROR", "request mismatch", "path and body request_id must match")
		return
	}

	s.mu.Lock()
	result, found := s.results[fiscalResultKey(organizationID, requestID)]
	s.mu.Unlock()
	if !found {
		result = fiscalapi.FiscalResult{
			CorrelationId:  body.CorrelationId,
			IdempotencyKey: body.IdempotencyKey,
			ObservedAt:     time.Now().UTC(),
			OrganizationId: organizationID,
			RequestId:      requestID,
			SnapshotDigest: body.SnapshotDigest,
			SourceVersion:  body.SourceVersion,
			Status:         fiscalapi.NotFound,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func validateFiscalMetadata(
	w http.ResponseWriter,
	pathOrganizationID,
	headerIdempotencyKey,
	headerCorrelationID string,
	body fiscalapi.FiscalRequest,
) bool {
	if fiscalhelpers.MetadataMatches(fiscalmodels.Metadata{
		PathOrganizationID:   pathOrganizationID,
		HeaderIdempotencyKey: headerIdempotencyKey,
		HeaderCorrelationID:  headerCorrelationID,
		BodyOrganizationID:   body.OrganizationId,
		BodyIdempotencyKey:   body.IdempotencyKey,
		BodyCorrelationID:    body.CorrelationId,
	}) {
		return true
	}
	writeProblem(
		w,
		http.StatusBadRequest,
		headerCorrelationID,
		"VALIDATION_ERROR",
		"request metadata mismatch",
		"path, headers and body metadata must match",
	)
	return false
}

func fiscalResultKey(organizationID, requestID string) string {
	return organizationID + "\x00" + requestID
}
