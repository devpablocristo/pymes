// architecture:adapter handler
package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	publicapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/handler/dto"
	handlerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/commerce/handler/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Authenticator is the BFF's verified Clerk identity boundary. It intentionally
// has no header/default implementation: public mutations stay closed until the
// production Clerk verifier is configured.
type Authenticator interface {
	Principal(*http.Request) (identitydomain.Principal, error)
}

// Commerce is the inbound handler's use-case port. Its implementation is
// chosen exclusively by wire; HTTP never depends on PostgreSQL.
type Commerce interface {
	CreatePartyIdempotent(context.Context, domain.IdempotencyCommand, domain.Party) (domain.Party, error)
	GetParty(context.Context, string, string) (domain.Party, error)
	CreatePurchaseAndQueueIdempotent(context.Context, domain.IdempotencyCommand, domain.Purchase) (domain.Purchase, error)
	GetPurchase(context.Context, string, string) (domain.Purchase, error)
	CreatePaymentAndApplicationsIdempotent(context.Context, domain.IdempotencyCommand, domain.Payment, []domain.OpenItemApplication) (domain.Payment, error)
	GetPayment(context.Context, string, string) (domain.Payment, error)
	CreateSaleAndQueueFiscalIdempotent(context.Context, domain.IdempotencyCommand, domain.Sale, string) (domain.Sale, error)
	CreateAccountingReversalIdempotent(context.Context, domain.IdempotencyCommand, domain.AccountingReversal) (domain.AccountingReversal, error)
	GetAccountingFailure(context.Context, string, string) (domain.AccountingFailure, error)
	RequestAccountingAdjustmentIdempotent(context.Context, domain.IdempotencyCommand, string, domain.AccountingAdjustment) (domain.AccountingAdjustment, error)
	GetSale(context.Context, string, string) (domain.Sale, error)
	RequestFiscalCredentialCSR(context.Context, string, string, string, domain.FiscalCredentialCSRInput) (domain.FiscalCredentialCSRResult, error)
	GetFiscalCredential(context.Context, string, string, string) (domain.FiscalCredential, error)
	UploadFiscalCertificate(context.Context, string, string, string, domain.FiscalCertificateUpload) (domain.FiscalCredential, error)
	ConfigureFiscalPointOfSale(context.Context, string, string, string, int, bool) (domain.FiscalPointOfSale, error)
	ValidateFiscalPointOfSale(context.Context, string, string, string, int, bool) (domain.FiscalPointOfSale, error)
	Ready(context.Context) error
	Clock() time.Time
}
type HTTPServer struct {
	commerce Commerce
	auth     Authenticator
}

var _ publicapi.ServerInterface = (*HTTPServer)(nil)

func NewHTTPServer(commerce Commerce, auth Authenticator) *HTTPServer {
	return &HTTPServer{commerce: commerce, auth: auth}
}

// Handler delegates every public route and parameter binding to the server
// generated from api/openapi.yaml. The only customization is the generated
// binding-error renderer, which keeps the BFF's stable JSON error codes.
func (s *HTTPServer) Handler() http.Handler {
	return publicapi.HandlerWithOptions(s, publicapi.ChiServerOptions{
		BaseRouter:       chi.NewRouter(),
		ErrorHandlerFunc: writeGeneratedParameterError,
	})
}

func (s *HTTPServer) GetHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *HTTPServer) GetReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := s.commerce.Ready(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *HTTPServer) RequestFiscalCredentialCSR(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	params publicapi.RequestFiscalCredentialCSRParams,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, true)
	if !ok {
		return
	}
	var input publicapi.FiscalCredentialCSRInput
	if decodeJSON(r, &input) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	result, err := s.commerce.RequestFiscalCredentialCSR(
		identityusecases.WithPrincipal(r.Context(), principal),
		organizationID,
		params.IdempotencyKey,
		requestCorrelationID(r),
		handlerhelpers.FiscalCredentialCSRInput(input),
	)
	if err != nil {
		writeFiscalSettingsError(w, err)
		return
	}
	response, err := handlerhelpers.PublicFiscalCredentialCSRResult(result)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "FISCAL_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (s *HTTPServer) GetFiscalCredential(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	credentialID publicapi.FiscalCredentialId,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, false)
	if !ok {
		return
	}
	result, err := s.commerce.GetFiscalCredential(
		identityusecases.WithPrincipal(r.Context(), principal),
		organizationID,
		credentialID.String(),
		requestCorrelationID(r),
	)
	if err != nil {
		writeFiscalSettingsError(w, err)
		return
	}
	response, err := handlerhelpers.PublicFiscalCredential(result)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "FISCAL_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) UploadFiscalCertificate(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	credentialID publicapi.FiscalCredentialId,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, true)
	if !ok {
		return
	}
	var input publicapi.FiscalCertificateUpload
	if decodeJSON(r, &input) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	result, err := s.commerce.UploadFiscalCertificate(
		identityusecases.WithPrincipal(r.Context(), principal),
		organizationID,
		credentialID.String(),
		requestCorrelationID(r),
		handlerhelpers.FiscalCertificateUpload(input),
	)
	if err != nil {
		writeFiscalSettingsError(w, err)
		return
	}
	response, err := handlerhelpers.PublicFiscalCredential(result)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "FISCAL_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) ConfigureFiscalPointOfSale(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	credentialID publicapi.FiscalCredentialId,
	pointOfSale publicapi.FiscalPointOfSaleNumber,
) {
	s.configureFiscalPointOfSale(w, r, organizationID, credentialID, pointOfSale, false)
}

func (s *HTTPServer) ValidateFiscalPointOfSale(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	credentialID publicapi.FiscalCredentialId,
	pointOfSale publicapi.FiscalPointOfSaleNumber,
) {
	s.configureFiscalPointOfSale(w, r, organizationID, credentialID, pointOfSale, true)
}

func (s *HTTPServer) configureFiscalPointOfSale(
	w http.ResponseWriter,
	r *http.Request,
	organizationID string,
	credentialID publicapi.FiscalCredentialId,
	pointOfSale int,
	validate bool,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, true)
	if !ok {
		return
	}
	var input publicapi.FiscalPointOfSaleConfiguration
	if decodeJSON(r, &input) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	ctx := identityusecases.WithPrincipal(r.Context(), principal)
	correlationID := requestCorrelationID(r)
	var (
		result domain.FiscalPointOfSale
		err    error
	)
	if validate {
		result, err = s.commerce.ValidateFiscalPointOfSale(
			ctx,
			organizationID,
			credentialID.String(),
			correlationID,
			pointOfSale,
			input.Enabled,
		)
	} else {
		result, err = s.commerce.ConfigureFiscalPointOfSale(
			ctx,
			organizationID,
			credentialID.String(),
			correlationID,
			pointOfSale,
			input.Enabled,
		)
	}
	if err != nil {
		writeFiscalSettingsError(w, err)
		return
	}
	response, err := handlerhelpers.PublicFiscalPointOfSale(result)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "FISCAL_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) GetPurchase(w http.ResponseWriter, r *http.Request, organizationID publicapi.OrganizationId, purchaseID string) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, false)
	if !ok {
		return
	}
	value, err := s.commerce.GetPurchase(identityusecases.WithPrincipal(r.Context(), principal), organizationID, purchaseID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, publicPurchase(value))
}

func (s *HTTPServer) GetPayment(w http.ResponseWriter, r *http.Request, organizationID publicapi.OrganizationId, paymentID string) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, false)
	if !ok {
		return
	}
	value, err := s.commerce.GetPayment(identityusecases.WithPrincipal(r.Context(), principal), organizationID, paymentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, publicPayment(value))
}

func (s *HTTPServer) CreateAccountingReversal(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	params publicapi.CreateAccountingReversalParams,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, true)
	if !ok {
		return
	}
	var input publicapi.CreateAccountingReversalJSONRequestBody
	if decodeJSON(r, &input) != nil || input.Id == "" || !input.DocumentKind.Valid() ||
		input.DocumentId == "" || input.EffectiveAt.IsZero() || strings.TrimSpace(input.Reason) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	command, ok := commandIdentity(r, w, principal.ActorID, organizationID, domain.OperationCreateAccountingReversal, input.Id, params.IdempotencyKey, params.XSourceVersion, input)
	if !ok {
		return
	}
	result, err := s.commerce.CreateAccountingReversalIdempotent(identityusecases.WithPrincipal(r.Context(), principal), command, domain.AccountingReversal{
		ID: input.Id, OrganizationID: organizationID, DocumentKind: string(input.DocumentKind),
		DocumentID: input.DocumentId, EffectiveAt: input.EffectiveAt.UTC(), Reason: input.Reason,
		Status: "requested", SnapshotDigest: command.PayloadHash,
		Origin: originFromCommand(command), CorrelationID: command.CorrelationID,
	})
	if err != nil {
		writeCommandError(w, err)
		return
	}
	w.Header().Set("Idempotency-Key", command.Key)
	writeJSON(w, http.StatusCreated, publicReversal(result))
}

func (s *HTTPServer) GetAccountingFailure(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	failureID openapi_types.UUID,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, false)
	if !ok {
		return
	}
	value, err := s.commerce.GetAccountingFailure(
		identityusecases.WithPrincipal(r.Context(), principal),
		organizationID,
		failureID.String(),
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, publicAccountingFailure(value))
}

func (s *HTTPServer) CreateAccountingAdjustment(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	failureID openapi_types.UUID,
	params publicapi.CreateAccountingAdjustmentParams,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, true)
	if !ok {
		return
	}
	var input publicapi.CreateAccountingAdjustmentJSONRequestBody
	if decodeJSON(r, &input) != nil ||
		strings.TrimSpace(input.Id) == "" ||
		len(input.Id) > 255 ||
		input.EffectiveAt.IsZero() ||
		strings.TrimSpace(input.Reason) == "" ||
		len(input.Reason) > 500 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	command, ok := commandIdentity(
		r, w, principal.ActorID, organizationID,
		domain.OperationCreateAccountingAdjustment, input.Id,
		params.IdempotencyKey, params.XSourceVersion, input,
	)
	if !ok {
		return
	}
	value := domain.AccountingAdjustment{
		ID:             input.Id,
		OrganizationID: organizationID,
		FailureID:      failureID.String(),
		EffectiveAt:    input.EffectiveAt.UTC(),
		Reason:         strings.TrimSpace(input.Reason),
		Origin:         originFromCommand(command),
		CorrelationID:  command.CorrelationID,
	}
	result, err := s.commerce.RequestAccountingAdjustmentIdempotent(
		identityusecases.WithPrincipal(r.Context(), principal),
		command,
		failureID.String(),
		value,
	)
	if err != nil {
		writeCommandError(w, err)
		return
	}
	w.Header().Set("Idempotency-Key", command.Key)
	writeJSON(w, http.StatusCreated, publicAccountingAdjustment(result))
}

func (s *HTTPServer) CreateParty(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	params publicapi.CreatePartyParams,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, true)
	if !ok {
		return
	}
	var input publicapi.CreatePartyJSONRequestBody
	if decodeJSON(r, &input) != nil || input.Id == "" || !input.Kind.Valid() || strings.TrimSpace(input.DisplayName) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	command, ok := commandIdentity(r, w, principal.ActorID, organizationID, domain.OperationCreateParty, input.Id, params.IdempotencyKey, params.XSourceVersion, input)
	if !ok {
		return
	}
	party, err := s.commerce.CreatePartyIdempotent(identityusecases.WithPrincipal(r.Context(), principal), command, domain.Party{
		ID: input.Id, OrganizationID: organizationID, Kind: string(input.Kind),
		DisplayName: input.DisplayName, TaxIdentifier: stringValue(input.TaxIdentifier),
	})
	if err != nil {
		writeCommandError(w, err)
		return
	}
	w.Header().Set("Idempotency-Key", command.Key)
	writeJSON(w, http.StatusCreated, publicParty(party))
}

func (s *HTTPServer) GetParty(w http.ResponseWriter, r *http.Request, organizationID publicapi.OrganizationId, partyID string) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, false)
	if !ok {
		return
	}
	party, err := s.commerce.GetParty(identityusecases.WithPrincipal(r.Context(), principal), organizationID, partyID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, publicParty(party))
}

func (s *HTTPServer) CreatePurchase(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	params publicapi.CreatePurchaseParams,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, true)
	if !ok {
		return
	}
	var input publicapi.CreatePurchaseJSONRequestBody
	if decodeJSON(r, &input) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	total := domain.Money{Amount: input.Amount, Currency: input.Currency}
	vatBreakdown := make([]domain.VATBreakdownItem, len(input.VatBreakdown))
	for index, item := range input.VatBreakdown {
		vatBreakdown[index] = domain.VATBreakdownItem{
			Rate: string(item.Rate), BaseAmount: item.BaseAmount, TaxAmount: item.TaxAmount,
		}
	}
	exchangeRate := ""
	if input.ExchangeRate != nil {
		exchangeRate = *input.ExchangeRate
	}
	if input.Id == "" || input.SupplierRef == "" || input.ExternalDocumentRef == "" || !total.Valid() {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	snapshot, _ := json.Marshal(input)
	digest := sha256.Sum256(snapshot)
	purchase := domain.Purchase{
		ID: input.Id, OrganizationID: organizationID, SupplierRef: input.SupplierRef,
		ExternalDocumentRef: input.ExternalDocumentRef,
		IssueDate:           input.IssueDate.Time.Format(time.DateOnly),
		Total:               total,
		NetAmount:           input.NetAmount,
		ExemptAmount:        input.ExemptAmount,
		VATBreakdown:        vatBreakdown,
		ExchangeRate:        exchangeRate,
		SnapshotDigest:      hex.EncodeToString(digest[:]), CorrelationID: "purchase:" + input.Id,
	}
	if err := purchase.ValidateAccountingAmounts(); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	command, ok := commandIdentity(r, w, principal.ActorID, organizationID, domain.OperationCreatePurchase, input.Id, params.IdempotencyKey, params.XSourceVersion, input)
	if !ok {
		return
	}
	purchase.Origin = originFromCommand(command)
	purchase.CorrelationID = command.CorrelationID
	purchase, err := s.commerce.CreatePurchaseAndQueueIdempotent(identityusecases.WithPrincipal(r.Context(), principal), command, purchase)
	if err != nil {
		writeCommandError(w, err)
		return
	}
	w.Header().Set("Idempotency-Key", command.Key)
	writeJSON(w, http.StatusCreated, publicPurchase(purchase))
}

func (s *HTTPServer) CreatePayment(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	params publicapi.CreatePaymentParams,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, true)
	if !ok {
		return
	}
	var input publicapi.CreatePaymentJSONRequestBody
	if decodeJSON(r, &input) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	total := domain.Money{Amount: input.Amount, Currency: input.Currency}
	if input.Id == "" || input.PartyRef == "" || !input.Direction.Valid() || !total.Valid() {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	p := domain.Payment{
		ID: input.Id, OrganizationID: organizationID, Direction: string(input.Direction),
		PartyRef: input.PartyRef, Total: total, CorrelationID: "payment:" + input.Id,
	}
	apps := make([]domain.OpenItemApplication, 0, len(input.Applications))
	for _, a := range input.Applications {
		amount := domain.Money{Amount: a.Amount, Currency: input.Currency}
		if a.Id == "" || a.DocumentId == "" || !a.DocumentKind.Valid() || !amount.Valid() {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
			return
		}
		apps = append(apps, domain.OpenItemApplication{
			ID: a.Id, PaymentID: p.ID, DocumentKind: string(a.DocumentKind),
			DocumentID: a.DocumentId, Amount: amount,
		})
	}
	command, ok := commandIdentity(r, w, principal.ActorID, organizationID, domain.OperationCreatePayment, input.Id, params.IdempotencyKey, params.XSourceVersion, input)
	if !ok {
		return
	}
	p.Origin = originFromCommand(command)
	p.CorrelationID = command.CorrelationID
	p, err := s.commerce.CreatePaymentAndApplicationsIdempotent(identityusecases.WithPrincipal(r.Context(), principal), command, p, apps)
	if err != nil {
		writeCommandError(w, err)
		return
	}
	w.Header().Set("Idempotency-Key", command.Key)
	writeJSON(w, http.StatusCreated, publicPayment(p))
}

func (s *HTTPServer) authorizedOrganization(w http.ResponseWriter, r *http.Request, organizationID string, mutation bool) (identitydomain.Principal, bool) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED")
		return identitydomain.Principal{}, false
	}
	principal, err := s.auth.Principal(r)
	if err != nil || !principal.CanRead(organizationID) {
		writeError(w, http.StatusForbidden, "FORBIDDEN")
		return identitydomain.Principal{}, false
	}
	if mutation && !principal.CanMutateRole() {
		writeError(w, http.StatusForbidden, "FORBIDDEN")
		return identitydomain.Principal{}, false
	}
	if mutation && !principal.OrganizationReady() {
		writeError(w, http.StatusUnprocessableEntity, "ORG_NOT_PROVISIONED")
		return identitydomain.Principal{}, false
	}
	return principal, true
}

func (s *HTTPServer) GetSale(w http.ResponseWriter, r *http.Request, organizationID publicapi.OrganizationId, saleID string) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, false)
	if !ok {
		return
	}
	sale, err := s.commerce.GetSale(identityusecases.WithPrincipal(r.Context(), principal), organizationID, saleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, publicSale(sale))
}

func (s *HTTPServer) CreateSale(
	w http.ResponseWriter,
	r *http.Request,
	organizationID publicapi.OrganizationId,
	params publicapi.CreateSaleParams,
) {
	principal, ok := s.authorizedOrganization(w, r, organizationID, true)
	if !ok {
		return
	}
	var input publicapi.CreateSaleJSONRequestBody
	if decodeJSON(r, &input) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	credentialRef := stringValue(input.CredentialRef)
	sourceDocumentID := stringValue(input.SourceDocumentId)
	exchangeRate := ""
	if input.ExchangeRate != nil {
		exchangeRate = string(*input.ExchangeRate)
	}
	total := domain.Money{Amount: input.Amount, Currency: input.Currency}
	if input.Id == "" || input.RecipientRef == "" || input.PointOfSale < 1 ||
		credentialRef == "" || !input.DocumentType.Valid() ||
		(isNote(string(input.DocumentType)) && sourceDocumentID == "") ||
		!input.Fiscal.Environment.Valid() || input.Fiscal.IssueDate.Time.IsZero() ||
		input.Fiscal.Totals == nil || input.Fiscal.Recipient == nil || len(input.Fiscal.Lines) == 0 ||
		!total.Valid() {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	if err := domain.ValidateExchangeRate(input.Currency, exchangeRate); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	fiscalSnapshot, err := handlerhelpers.FreezeFiscalSnapshot(input.Fiscal, input.Currency, exchangeRate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	now := s.commerce.Clock().UTC()
	sale := domain.Sale{
		ID: input.Id, OrganizationID: organizationID, RecipientRef: input.RecipientRef,
		Voucher:           domain.VoucherReference{PointOfSale: input.PointOfSale, DocumentType: string(input.DocumentType)},
		FiscalEnvironment: string(input.Fiscal.Environment), Total: total, Status: domain.SaleFiscalPending,
		FiscalSnapshot: fiscalSnapshot, CorrelationID: "sale:" + input.Id,
		SourceDocumentID: sourceDocumentID, CreatedAt: now, UpdatedAt: now,
	}
	command, ok := commandIdentity(r, w, principal.ActorID, organizationID, domain.OperationCreateSale, input.Id, params.IdempotencyKey, params.XSourceVersion, input)
	if !ok {
		return
	}
	sale.Origin = originFromCommand(command)
	sale.CorrelationID = command.CorrelationID
	sale, err = s.commerce.CreateSaleAndQueueFiscalIdempotent(identityusecases.WithPrincipal(r.Context(), principal), command, sale, credentialRef)
	if err != nil {
		writeCommandError(w, err)
		return
	}
	w.Header().Set("Idempotency-Key", command.Key)
	writeJSON(w, http.StatusCreated, publicSale(sale))
}

func isNote(value string) bool {
	return strings.HasPrefix(value, "NC") || strings.HasPrefix(value, "ND")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	handlerhelpers.WriteJSON(w, status, value)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, publicapi.Error{Code: publicapi.ErrorCode(code)})
}

func writeGeneratedParameterError(w http.ResponseWriter, _ *http.Request, err error) {
	var requiredHeader *publicapi.RequiredHeaderError
	if errors.As(err, &requiredHeader) {
		switch requiredHeader.ParamName {
		case "Idempotency-Key":
			writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		case "X-Source-Version":
			writeError(w, http.StatusBadRequest, "SOURCE_VERSION_REQUIRED")
		default:
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		}
		return
	}
	var invalidParameter *publicapi.InvalidParamFormatError
	if errors.As(err, &invalidParameter) && invalidParameter.ParamName == "X-Source-Version" {
		writeError(w, http.StatusBadRequest, "SOURCE_VERSION_REQUIRED")
		return
	}
	writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
}

func decodeJSON(r *http.Request, destination any) error {
	return handlerhelpers.DecodeJSON(r, destination)
}

func commandIdentity(
	r *http.Request,
	w http.ResponseWriter,
	actorRef string,
	organizationID, operation, sourceID string,
	idempotencyKey string,
	sourceVersion int,
	payload any,
) (domain.IdempotencyCommand, bool) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
		return domain.IdempotencyCommand{}, false
	}
	if len(key) > 255 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return domain.IdempotencyCommand{}, false
	}
	if sourceVersion < 1 {
		writeError(w, http.StatusBadRequest, "SOURCE_VERSION_REQUIRED")
		return domain.IdempotencyCommand{}, false
	}
	canonicalPayload, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return domain.IdempotencyCommand{}, false
	}
	digest := sha256.Sum256(canonicalPayload)
	requestID, correlationID := "", ""
	if r != nil {
		if metadata, ok := identityusecases.RequestMetadataFromContext(r.Context()); ok {
			requestID, correlationID = metadata.RequestID, metadata.CorrelationID
		}
		if requestID == "" {
			requestID = strings.TrimSpace(r.Header.Get("X-Request-ID"))
		}
		if correlationID == "" {
			correlationID = strings.TrimSpace(r.Header.Get("X-Correlation-ID"))
		}
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	if correlationID == "" {
		correlationID = requestID
	}
	if len(requestID) > 255 || len(correlationID) > 255 || strings.TrimSpace(actorRef) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return domain.IdempotencyCommand{}, false
	}
	return domain.IdempotencyCommand{
		Key:            key,
		OrganizationID: organizationID,
		Operation:      operation,
		SourceID:       sourceID,
		SourceVersion:  sourceVersion,
		PayloadHash:    hex.EncodeToString(digest[:]),
		RequestID:      requestID,
		CorrelationID:  correlationID,
		ActorRef:       actorRef,
	}, true
}

func originFromCommand(command domain.IdempotencyCommand) domain.OriginMetadata {
	return domain.OriginMetadata{
		RequestID: command.RequestID, CorrelationID: command.CorrelationID,
		ActorRef: command.ActorRef, SourceVersion: command.SourceVersion,
	}
}

func publicParty(value domain.Party) publicapi.Party {
	return publicapi.Party{
		Id: value.ID, OrganizationId: value.OrganizationID, Kind: publicapi.PartyKind(value.Kind),
		DisplayName: value.DisplayName, TaxIdentifier: optionalString(value.TaxIdentifier),
	}
}

func publicPurchase(value domain.Purchase) publicapi.Purchase {
	issueDate, _ := time.Parse(time.DateOnly, value.IssueDate)
	vatBreakdown := make([]publicapi.VatBreakdownItem, len(value.VATBreakdown))
	for index, item := range value.VATBreakdown {
		vatBreakdown[index] = publicapi.VatBreakdownItem{
			Rate: publicapi.VatBreakdownItemRate(item.Rate), BaseAmount: item.BaseAmount, TaxAmount: item.TaxAmount,
		}
	}
	result := publicapi.Purchase{
		Id: value.ID, OrganizationId: value.OrganizationID, SupplierRef: value.SupplierRef,
		ExternalDocumentRef: value.ExternalDocumentRef,
		IssueDate:           openapi_types.Date{Time: issueDate},
		Total:               publicapi.Money{Amount: value.Total.Amount, Currency: value.Total.Currency},
		NetAmount:           value.NetAmount,
		ExemptAmount:        value.ExemptAmount,
		VatBreakdown:        vatBreakdown,
		ExchangeRate:        optionalString(value.ExchangeRate),
		Status:              publicapi.PurchaseStatus(value.Status), SnapshotDigest: value.SnapshotDigest,
		JournalEntryId: optionalString(value.JournalEntryID), OpenItemId: optionalString(value.OpenItemID),
	}
	result.AccountingFailure = publicAccountingFailureRef(
		value.AccountingFailureID, value.AccountingFailureCode,
	)
	return result
}

func publicPayment(value domain.Payment) publicapi.Payment {
	result := publicapi.Payment{
		Id: value.ID, OrganizationId: value.OrganizationID, Direction: publicapi.PaymentDirection(value.Direction),
		PartyRef: value.PartyRef, Total: publicapi.Money{Amount: value.Total.Amount, Currency: value.Total.Currency},
		Status: publicapi.PaymentStatus(value.Status), JournalEntryId: optionalString(value.JournalEntryID),
		OpenItemId: optionalString(value.OpenItemID),
	}
	result.AccountingFailure = publicAccountingFailureRef(
		value.AccountingFailureID, value.AccountingFailureCode,
	)
	return result
}

func publicReversal(value domain.AccountingReversal) publicapi.Reversal {
	result := publicapi.Reversal{
		Id: value.ID, OrganizationId: value.OrganizationID,
		DocumentKind: publicapi.ReversalDocumentKind(value.DocumentKind), DocumentId: value.DocumentID,
		EffectiveAt: value.EffectiveAt, Reason: value.Reason, Status: publicapi.ReversalStatus(value.Status),
		OriginalJournalEntryId: value.OriginalJournalEntryID,
		ReversalJournalEntryId: optionalString(value.ReversalJournalEntryID),
	}
	result.AccountingFailure = publicAccountingFailureRef(
		value.AccountingFailureID, value.AccountingFailureCode,
	)
	return result
}

func publicSale(value domain.Sale) publicapi.Sale {
	result := publicapi.Sale{
		Id: value.ID, OrganizationId: value.OrganizationID, RecipientRef: value.RecipientRef,
		FiscalEnvironment: publicapi.SaleFiscalEnvironment(value.FiscalEnvironment),
		Total:             publicapi.Money{Amount: value.Total.Amount, Currency: value.Total.Currency},
		Status:            publicapi.SaleStatus(value.Status), SnapshotDigest: value.SnapshotDigest,
		SourceDocumentId: optionalString(value.SourceDocumentID), Cae: optionalString(value.CAE),
		JournalEntryId: optionalString(value.JournalEntryID), OpenItemId: optionalString(value.OpenItemID),
	}
	result.Voucher.PointOfSale = value.Voucher.PointOfSale
	result.Voucher.DocumentType = publicapi.SaleVoucherDocumentType(value.Voucher.DocumentType)
	result.Voucher.VoucherNumber = &value.Voucher.VoucherNumber
	result.AccountingFailure = publicAccountingFailureRef(
		value.AccountingFailureID, value.AccountingFailureCode,
	)
	return result
}

func publicAccountingFailure(value domain.AccountingFailure) publicapi.AccountingFailure {
	id, _ := uuid.Parse(value.ID)
	return publicapi.AccountingFailure{
		Id: id, OrganizationId: value.OrganizationID,
		SourceKind: publicapi.AccountingFailureSourceKind(value.SourceKind),
		SourceId:   value.SourceID, FailedEffectiveAt: value.FailedEffectiveAt,
		Status:        publicapi.AccountingFailureStatus(value.Status),
		FailureCode:   publicapi.AccountingFailureFailureCode(value.FailureCode),
		CorrelationId: value.CorrelationID,
		CreatedAt:     value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func publicAccountingAdjustment(value domain.AccountingAdjustment) publicapi.AccountingAdjustment {
	failureID, _ := uuid.Parse(value.FailureID)
	return publicapi.AccountingAdjustment{
		Id: value.ID, OrganizationId: value.OrganizationID, FailureId: failureID,
		EffectiveAt: value.EffectiveAt, Reason: value.Reason,
		Status:        publicapi.AccountingAdjustmentStatus(value.Status),
		CorrelationId: value.CorrelationID,
		CreatedAt:     value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func publicAccountingFailureRef(id, code string) *publicapi.AccountingFailureRef {
	if id == "" || code == "" {
		return nil
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil
	}
	return &publicapi.AccountingFailureRef{
		Id: parsed, Code: publicapi.AccountingFailureRefCode(code),
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func writeCommandError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrFeatureDisabled):
		writeError(w, http.StatusForbidden, "FEATURE_DISABLED")
	case errors.Is(err, domain.ErrIdempotencyKeyReused):
		writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	case errors.Is(err, domain.ErrOrganizationNotReady):
		writeError(w, http.StatusUnprocessableEntity, "ORG_NOT_PROVISIONED")
	default:
		switch err.Error() {
		case "DOCUMENT_NOT_REVERSIBLE", "INVALID_APPLICATION_DOCUMENT",
			"INVALID_SOURCE_DOCUMENT", "OPEN_ITEM_AMOUNT_EXCEEDED",
			"ACCOUNTING_ADJUSTMENT_NOT_ALLOWED", "VALIDATION_ERROR":
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeError(w, http.StatusUnprocessableEntity, "COMMAND_REJECTED")
		}
	}
}

func requestCorrelationID(r *http.Request) string {
	if r != nil {
		if metadata, ok := identityusecases.RequestMetadataFromContext(r.Context()); ok &&
			metadata.CorrelationID != "" {
			return metadata.CorrelationID
		}
		if value := strings.TrimSpace(r.Header.Get("X-Correlation-ID")); value != "" && len(value) <= 255 {
			return value
		}
	}
	return uuid.NewString()
}

func writeFiscalSettingsError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrFeatureDisabled) {
		writeError(w, http.StatusForbidden, "FEATURE_DISABLED")
		return
	}
	var upstream serviceError
	if errors.As(err, &upstream) {
		switch upstream.Code {
		case "CREDENTIAL_NOT_FOUND":
			writeError(w, http.StatusNotFound, upstream.Code)
		case "IDEMPOTENCY_KEY_REUSED", "CREDENTIAL_VERSION_CONFLICT":
			writeError(w, http.StatusConflict, upstream.Code)
		case "AUTHORITY_TIMEOUT":
			writeError(w, http.StatusServiceUnavailable, upstream.Code)
		case "CREDENTIAL_NOT_READY", "CERTIFICATE_INVALID", "CERTIFICATE_CUIT_MISMATCH",
			"CERTIFICATE_ENVIRONMENT_MISMATCH", "POINT_OF_SALE_NOT_VALIDATED",
			"VALIDATION_ERROR":
			writeError(w, http.StatusUnprocessableEntity, upstream.Code)
		default:
			writeError(w, http.StatusServiceUnavailable, "FISCAL_UNAVAILABLE")
		}
		return
	}
	if err != nil && err.Error() == "VALIDATION_ERROR" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
		return
	}
	writeError(w, http.StatusServiceUnavailable, "FISCAL_UNAVAILABLE")
}
