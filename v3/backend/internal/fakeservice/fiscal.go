// architecture:adapter external
package fakeservice

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	fiscalapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/models"
	fiscalhelpers "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/fiscal/helpers"
	fiscalmodels "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/fiscal/models"
)

var _ fiscalapi.ServerInterface = (*fiscalFakeServer)(nil)

type fiscalFakeServer struct {
	mu                 sync.Mutex
	results            map[string]fiscalapi.FiscalResult
	idempotencies      map[string]fiscalapi.FiscalResult
	credentials        map[string]fiscalapi.FiscalCredential
	credentialReplays  map[string]fiscalmodels.CredentialReplay
	fiscalPointsOfSale map[string]fiscalapi.FiscalPointOfSale
}

func newFiscalFakeServer() *fiscalFakeServer {
	return &fiscalFakeServer{
		results:            make(map[string]fiscalapi.FiscalResult),
		idempotencies:      make(map[string]fiscalapi.FiscalResult),
		credentials:        make(map[string]fiscalapi.FiscalCredential),
		credentialReplays:  make(map[string]fiscalmodels.CredentialReplay),
		fiscalPointsOfSale: make(map[string]fiscalapi.FiscalPointOfSale),
	}
}

func (s *fiscalFakeServer) FiscalHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, fiscalapi.HealthStatus{Status: fiscalapi.Ok})
}

func (s *fiscalFakeServer) FiscalReadiness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, fiscalapi.ReadinessStatus{Status: fiscalapi.ReadinessStatusStatusReady})
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
		Status:              fiscalapi.FiscalResultStatusAuthorized,
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
			Status:         fiscalapi.FiscalResultStatusNotFound,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *fiscalFakeServer) RequestFiscalCredentialCSR(
	w http.ResponseWriter,
	r *http.Request,
	organizationID fiscalapi.OrganizationId,
	params fiscalapi.RequestFiscalCredentialCSRParams,
) {
	var body fiscalapi.CSRRequest
	if !decodeJSON(w, r, &body, params.XCorrelationID) {
		return
	}
	replayKey := fiscalhelpers.ScopedKey(organizationID, params.IdempotencyKey)

	s.mu.Lock()
	if replay, found := s.credentialReplays[replayKey]; found {
		s.mu.Unlock()
		if replay.Request != body {
			writeProblem(w, http.StatusConflict, params.XCorrelationID, "IDEMPOTENCY_KEY_REUSED", "idempotency key reused", "the CSR request was already stored with different content")
			return
		}
		writeJSON(w, http.StatusCreated, replay.Result)
		return
	}

	now := time.Now().UTC()
	credentialID := stableUUID(organizationID, string(body.Environment), body.Cuit, params.IdempotencyKey).String()
	credential := fiscalapi.FiscalCredential{
		CommonName:     body.CommonName,
		CreatedAt:      now,
		Cuit:           body.Cuit,
		Environment:    fiscalapi.FiscalCredentialEnvironment(body.Environment),
		Id:             credentialID,
		LegalName:      body.LegalName,
		OrganizationId: organizationID,
		Status:         fiscalapi.FiscalCredentialStatusPendingCertificate,
		UpdatedAt:      now,
		Version:        1,
	}
	result := fiscalapi.CSRResult{
		Credential: credential,
		CsrPem:     "-----BEGIN CERTIFICATE REQUEST-----\nFAKE-" + credentialID + "\n-----END CERTIFICATE REQUEST-----\n",
	}
	s.credentials[fiscalhelpers.ScopedKey(organizationID, credentialID)] = credential
	s.credentialReplays[replayKey] = fiscalmodels.CredentialReplay{Request: body, Result: result}
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, result)
}

func (s *fiscalFakeServer) GetFiscalCredential(
	w http.ResponseWriter,
	_ *http.Request,
	organizationID fiscalapi.OrganizationId,
	credentialID fiscalapi.CredentialId,
	params fiscalapi.GetFiscalCredentialParams,
) {
	s.mu.Lock()
	credential, found := s.credentials[fiscalhelpers.ScopedKey(organizationID, credentialID)]
	s.mu.Unlock()
	if !found {
		writeProblem(w, http.StatusNotFound, params.XCorrelationID, "CREDENTIAL_NOT_FOUND", "credential not found", "the fiscal credential does not exist")
		return
	}
	writeJSON(w, http.StatusOK, credential)
}

func (s *fiscalFakeServer) UploadFiscalCertificate(
	w http.ResponseWriter,
	r *http.Request,
	organizationID fiscalapi.OrganizationId,
	credentialID fiscalapi.CredentialId,
	params fiscalapi.UploadFiscalCertificateParams,
) {
	var body fiscalapi.CertificateUpload
	if !decodeJSON(w, r, &body, params.XCorrelationID) {
		return
	}
	key := fiscalhelpers.ScopedKey(organizationID, credentialID)
	s.mu.Lock()
	credential, found := s.credentials[key]
	if !found {
		s.mu.Unlock()
		writeProblem(w, http.StatusNotFound, params.XCorrelationID, "CREDENTIAL_NOT_FOUND", "credential not found", "the fiscal credential does not exist")
		return
	}
	if credential.Version != body.ExpectedVersion {
		s.mu.Unlock()
		writeProblem(w, http.StatusConflict, params.XCorrelationID, "CREDENTIAL_VERSION_CONFLICT", "credential version conflict", "the fiscal credential was modified")
		return
	}
	now := time.Now().UTC()
	expiresAt := now.AddDate(1, 0, 0)
	fingerprint := fiscalhelpers.CertificateFingerprint(body.CertificatePem)
	serial := stableUUID(credentialID, fingerprint).String()
	credential.CertificateExpiresAt = &expiresAt
	credential.CertificateFingerprint = &fingerprint
	credential.CertificateSerialNumber = &serial
	credential.CertificateValidFrom = &now
	credential.Status = fiscalapi.FiscalCredentialStatusReady
	credential.UpdatedAt = now
	credential.Version++
	s.credentials[key] = credential
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, credential)
}

func (s *fiscalFakeServer) ConfigureFiscalPointOfSale(
	w http.ResponseWriter,
	r *http.Request,
	organizationID fiscalapi.OrganizationId,
	credentialID fiscalapi.CredentialId,
	pointOfSale int,
	params fiscalapi.ConfigureFiscalPointOfSaleParams,
) {
	var body fiscalapi.ConfigureFiscalPointOfSaleJSONBody
	if !decodeJSON(w, r, &body, params.XCorrelationID) {
		return
	}
	credentialKey := fiscalhelpers.ScopedKey(organizationID, credentialID)
	s.mu.Lock()
	credential, found := s.credentials[credentialKey]
	if !found {
		s.mu.Unlock()
		writeProblem(w, http.StatusNotFound, params.XCorrelationID, "CREDENTIAL_NOT_FOUND", "credential not found", "the fiscal credential does not exist")
		return
	}
	pointKey := fiscalhelpers.ScopedKey(organizationID, credentialID, fiscalPointOfSalePart(pointOfSale))
	existing := s.fiscalPointsOfSale[pointKey]
	if body.Enabled && existing.ValidatedAt == nil {
		s.mu.Unlock()
		writeProblem(w, http.StatusUnprocessableEntity, params.XCorrelationID, "POINT_OF_SALE_NOT_VALIDATED", "point of sale not validated", "validate WSAA and WSFE before enabling the point of sale")
		return
	}
	point := fiscalapi.FiscalPointOfSale{
		CredentialId:   credentialID,
		Enabled:        body.Enabled,
		Environment:    fiscalapi.FiscalPointOfSaleEnvironment(credential.Environment),
		Number:         pointOfSale,
		OrganizationId: organizationID,
		ValidatedAt:    existing.ValidatedAt,
	}
	s.fiscalPointsOfSale[pointKey] = point
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, point)
}

func (s *fiscalFakeServer) ValidateFiscalPointOfSale(
	w http.ResponseWriter,
	r *http.Request,
	organizationID fiscalapi.OrganizationId,
	credentialID fiscalapi.CredentialId,
	pointOfSale int,
	params fiscalapi.ValidateFiscalPointOfSaleParams,
) {
	var body fiscalapi.ValidateFiscalPointOfSaleJSONBody
	if !decodeJSON(w, r, &body, params.XCorrelationID) {
		return
	}
	credentialKey := fiscalhelpers.ScopedKey(organizationID, credentialID)
	s.mu.Lock()
	credential, found := s.credentials[credentialKey]
	if !found {
		s.mu.Unlock()
		writeProblem(w, http.StatusNotFound, params.XCorrelationID, "CREDENTIAL_NOT_FOUND", "credential not found", "the fiscal credential does not exist")
		return
	}
	if credential.Status != fiscalapi.FiscalCredentialStatusReady {
		s.mu.Unlock()
		writeProblem(w, http.StatusUnprocessableEntity, params.XCorrelationID, "CREDENTIAL_NOT_READY", "credential not ready", "upload a valid certificate before validating the point of sale")
		return
	}
	now := time.Now().UTC()
	point := fiscalapi.FiscalPointOfSale{
		CredentialId:   credentialID,
		Enabled:        body.Enabled,
		Environment:    fiscalapi.FiscalPointOfSaleEnvironment(credential.Environment),
		Number:         pointOfSale,
		OrganizationId: organizationID,
		ValidatedAt:    &now,
	}
	s.fiscalPointsOfSale[fiscalhelpers.ScopedKey(organizationID, credentialID, fiscalPointOfSalePart(pointOfSale))] = point
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, point)
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

func fiscalPointOfSalePart(pointOfSale int) string {
	return "point-of-sale-" + fmt.Sprint(pointOfSale)
}
