package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
)

// Authenticator is the BFF's verified Clerk identity boundary. It intentionally
// has no header/default implementation: public mutations stay closed until the
// production Clerk verifier is configured.
type Authenticator interface {
	OrganizationID(*http.Request) (string, error)
}

// Commerce is the inbound handler's use-case port. Its implementation is
// chosen exclusively by wire; HTTP never depends on PostgreSQL.
type Commerce interface {
	CreateParty(context.Context, domain.Party) (domain.Party, error)
	GetParty(context.Context, string, string) (domain.Party, error)
	CreatePurchaseAndQueue(context.Context, domain.Purchase) error
	GetPurchase(context.Context, string, string) (domain.Purchase, error)
	CreatePaymentAndApplications(context.Context, domain.Payment, []domain.OpenItemApplication) error
	GetPayment(context.Context, string, string) (domain.Payment, error)
	CreateSaleAndQueueFiscal(context.Context, domain.Sale, string) (domain.Sale, error)
	CreateAccountingReversal(context.Context, domain.AccountingReversal) (domain.AccountingReversal, error)
	GetSale(context.Context, string, string) (domain.Sale, error)
	Ready(context.Context) error
	Clock() time.Time
}
type HTTPServer struct {
	commerce Commerce
	auth     Authenticator
}

func NewHTTPServer(commerce Commerce, auth Authenticator) *HTTPServer {
	return &HTTPServer{commerce: commerce, auth: auth}
}
func (s *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := s.commerce.Ready(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /api/v1/organizations/{organizationID}/sales", s.createSale)
	mux.HandleFunc("GET /api/v1/organizations/{organizationID}/sales/{saleID}", s.getSale)
	mux.HandleFunc("POST /api/v1/organizations/{organizationID}/parties", s.createParty)
	mux.HandleFunc("GET /api/v1/organizations/{organizationID}/parties/{partyID}", s.getParty)
	mux.HandleFunc("POST /api/v1/organizations/{organizationID}/purchases", s.createPurchase)
	mux.HandleFunc("GET /api/v1/organizations/{organizationID}/purchases/{purchaseID}", s.getPurchase)
	mux.HandleFunc("POST /api/v1/organizations/{organizationID}/payments", s.createPayment)
	mux.HandleFunc("GET /api/v1/organizations/{organizationID}/payments/{paymentID}", s.getPayment)
	mux.HandleFunc("POST /api/v1/organizations/{organizationID}/reversals", s.createAccountingReversal)
	return mux
}

func (s *HTTPServer) getPurchase(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := s.authorizedOrganization(w, r)
	if !ok {
		return
	}
	value, err := s.commerce.GetPurchase(r.Context(), organizationID, r.PathValue("purchaseID"))
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *HTTPServer) getPayment(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := s.authorizedOrganization(w, r)
	if !ok {
		return
	}
	value, err := s.commerce.GetPayment(r.Context(), organizationID, r.PathValue("paymentID"))
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *HTTPServer) createAccountingReversal(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := s.authorizedOrganization(w, r)
	if !ok {
		return
	}
	var input struct {
		ID           string    `json:"id"`
		DocumentKind string    `json:"document_kind"`
		DocumentID   string    `json:"document_id"`
		EffectiveAt  time.Time `json:"effective_at"`
		Reason       string    `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.ID == "" ||
		(input.DocumentKind != "purchase" && input.DocumentKind != "payment") ||
		input.DocumentID == "" || input.EffectiveAt.IsZero() || strings.TrimSpace(input.Reason) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	result, err := s.commerce.CreateAccountingReversal(r.Context(), domain.AccountingReversal{
		ID: input.ID, OrganizationID: organizationID, DocumentKind: input.DocumentKind,
		DocumentID: input.DocumentID, EffectiveAt: input.EffectiveAt.UTC(), Reason: input.Reason,
		Status: "requested", CorrelationID: "reversal:" + input.ID,
	})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func (s *HTTPServer) createParty(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := s.authorizedOrganization(w, r)
	if !ok {
		return
	}
	var input struct {
		ID            string `json:"id"`
		Kind          string `json:"kind"`
		DisplayName   string `json:"display_name"`
		TaxIdentifier string `json:"tax_identifier"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	party, err := s.commerce.CreateParty(r.Context(), domain.Party{ID: input.ID, OrganizationID: organizationID, Kind: input.Kind, DisplayName: input.DisplayName, TaxIdentifier: input.TaxIdentifier})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
		return
	}
	writeJSON(w, http.StatusCreated, party)
}
func (s *HTTPServer) getParty(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := s.authorizedOrganization(w, r)
	if !ok {
		return
	}
	party, err := s.commerce.GetParty(r.Context(), organizationID, r.PathValue("partyID"))
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, party)
}

func (s *HTTPServer) createPurchase(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := s.authorizedOrganization(w, r)
	if !ok {
		return
	}
	var input struct {
		ID                  string `json:"id"`
		SupplierRef         string `json:"supplier_ref"`
		ExternalDocumentRef string `json:"external_document_ref"`
		Amount              string `json:"amount"`
		Currency            string `json:"currency"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	snapshot, _ := json.Marshal(input)
	digest := sha256.Sum256(snapshot)
	purchase := domain.Purchase{ID: input.ID, OrganizationID: organizationID, SupplierRef: input.SupplierRef, ExternalDocumentRef: input.ExternalDocumentRef, Total: domain.Money{Amount: input.Amount, Currency: input.Currency}, SnapshotDigest: hex.EncodeToString(digest[:]), CorrelationID: "purchase:" + input.ID}
	if err := s.commerce.CreatePurchaseAndQueue(r.Context(), purchase); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
		return
	}
	purchase.Status = "confirmed"
	writeJSON(w, http.StatusCreated, purchase)
}
func (s *HTTPServer) createPayment(w http.ResponseWriter, r *http.Request) {
	org, ok := s.authorizedOrganization(w, r)
	if !ok {
		return
	}
	var input struct {
		ID           string `json:"id"`
		Direction    string `json:"direction"`
		PartyRef     string `json:"party_ref"`
		Amount       string `json:"amount"`
		Currency     string `json:"currency"`
		Applications []struct {
			ID           string `json:"id"`
			DocumentKind string `json:"document_kind"`
			DocumentID   string `json:"document_id"`
			Amount       string `json:"amount"`
		} `json:"applications"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	p := domain.Payment{ID: input.ID, OrganizationID: org, Direction: input.Direction, PartyRef: input.PartyRef, Total: domain.Money{Amount: input.Amount, Currency: input.Currency}, CorrelationID: "payment:" + input.ID}
	apps := make([]domain.OpenItemApplication, 0, len(input.Applications))
	for _, a := range input.Applications {
		apps = append(apps, domain.OpenItemApplication{ID: a.ID, PaymentID: p.ID, DocumentKind: a.DocumentKind, DocumentID: a.DocumentID, Amount: domain.Money{Amount: a.Amount, Currency: input.Currency}})
	}
	if err := s.commerce.CreatePaymentAndApplications(r.Context(), p, apps); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
		return
	}
	p.Status = "confirmed"
	writeJSON(w, http.StatusCreated, p)
}
func (s *HTTPServer) authorizedOrganization(w http.ResponseWriter, r *http.Request) (string, bool) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED")
		return "", false
	}
	organizationID := r.PathValue("organizationID")
	verified, err := s.auth.OrganizationID(r)
	if err != nil || verified != organizationID {
		writeError(w, http.StatusForbidden, "FORBIDDEN")
		return "", false
	}
	return organizationID, true
}
func (s *HTTPServer) getSale(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED")
		return
	}
	organizationID := r.PathValue("organizationID")
	verifiedOrg, err := s.auth.OrganizationID(r)
	if err != nil || verifiedOrg != organizationID {
		writeError(w, http.StatusForbidden, "FORBIDDEN")
		return
	}
	sale, err := s.commerce.GetSale(r.Context(), organizationID, r.PathValue("saleID"))
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, sale)
}
func (s *HTTPServer) createSale(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED")
		return
	}
	organizationID := r.PathValue("organizationID")
	verifiedOrg, err := s.auth.OrganizationID(r)
	if err != nil || verifiedOrg != organizationID {
		writeError(w, http.StatusForbidden, "FORBIDDEN")
		return
	}
	var input struct {
		ID               string          `json:"id"`
		RecipientRef     string          `json:"recipient_ref"`
		PointOfSale      int             `json:"point_of_sale"`
		DocumentType     string          `json:"document_type"`
		Amount           string          `json:"amount"`
		Currency         string          `json:"currency"`
		CredentialRef    string          `json:"credential_ref"`
		SourceDocumentID string          `json:"source_document_id"`
		FiscalSnapshot   json.RawMessage `json:"fiscal"`
	}
	var fiscal map[string]any
	if json.NewDecoder(r.Body).Decode(&input) != nil || json.Unmarshal(input.FiscalSnapshot, &fiscal) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	environment, environmentOK := fiscal["environment"].(string)
	if input.ID == "" || input.RecipientRef == "" || input.PointOfSale < 1 || input.CredentialRef == "" || !validDocumentType(input.DocumentType) || (isNote(input.DocumentType) && input.SourceDocumentID == "") || !environmentOK || (environment != "homologation" && environment != "production") || fiscal["issue_date"] == nil || fiscal["totals"] == nil || fiscal["recipient"] == nil || fiscal["lines"] == nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	now := s.commerce.Clock().UTC()
	sale := domain.Sale{ID: input.ID, OrganizationID: organizationID, RecipientRef: input.RecipientRef, Voucher: domain.VoucherReference{PointOfSale: input.PointOfSale, DocumentType: input.DocumentType}, FiscalEnvironment: environment, Total: domain.Money{Amount: input.Amount, Currency: input.Currency}, Status: domain.SaleFiscalPending, FiscalSnapshot: input.FiscalSnapshot, CorrelationID: "sale:" + input.ID, SourceDocumentID: input.SourceDocumentID, CreatedAt: now, UpdatedAt: now}
	if !sale.Total.Valid() {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	sale, err = s.commerce.CreateSaleAndQueueFiscal(r.Context(), sale, input.CredentialRef)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sale)
}

func validDocumentType(value string) bool {
	switch value {
	case "FA", "NDA", "NCA", "FB", "NDB", "NCB", "FC", "NDC", "NCC":
		return true
	default:
		return false
	}
}

func isNote(value string) bool {
	return strings.HasPrefix(value, "NC") || strings.HasPrefix(value, "ND")
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"code": code})
}
