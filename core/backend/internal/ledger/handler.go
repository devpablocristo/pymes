package ledger

import (
	"context"
	"net/http"
	"strings"
	"time"

	crudpaths "github.com/devpablocristo/platform/features/crud/paths/go/paths"
	"github.com/devpablocristo/platform/http/go/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/core/backend/internal/ledger/handler/dto"
	ledgerdomain "github.com/devpablocristo/pymes/core/backend/internal/ledger/usecases/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
)

type usecasesPort interface {
	ListAccounts(ctx context.Context, orgID uuid.UUID, includeArchived bool) ([]ledgerdomain.Account, error)
	GetAccount(ctx context.Context, orgID, id uuid.UUID) (ledgerdomain.Account, error)
	CreateAccount(ctx context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error)
	UpdateAccount(ctx context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error)
	ArchiveAccount(ctx context.Context, orgID, id uuid.UUID) error
	RestoreAccount(ctx context.Context, orgID, id uuid.UUID) error
	ListLinks(ctx context.Context, orgID uuid.UUID) ([]ledgerdomain.AccountLink, error)
	SetLink(ctx context.Context, orgID uuid.UUID, role string, accountID uuid.UUID) (ledgerdomain.AccountLink, error)
	PostManual(ctx context.Context, in ledgerdomain.JournalEntry) (ledgerdomain.JournalEntry, error)
	Journal(ctx context.Context, orgID uuid.UUID, from, to time.Time, limit int) ([]ledgerdomain.JournalEntry, error)
	AccountLedger(ctx context.Context, orgID, accountID uuid.UUID, from, to time.Time, limit int) (ledgerdomain.AccountLedger, error)
	TrialBalance(ctx context.Context, orgID uuid.UUID, asOf time.Time) (ledgerdomain.TrialBalance, error)
	Health(ctx context.Context, orgID uuid.UUID) (ledgerdomain.OutboxHealth, error)
	Setup(ctx context.Context, orgID uuid.UUID) ([]ledgerdomain.Account, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const base = "/ledger"
	const accounts = base + "/accounts"
	const accountItem = accounts + "/:id"

	// Plan de cuentas (CRUD canónico).
	auth.GET(accounts, rbac.RequirePermission("ledger", "read"), h.ListAccounts)
	auth.GET(accounts+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("ledger", "read"), h.ListArchivedAccounts)
	auth.POST(accounts, rbac.RequirePermission("ledger", "create"), h.CreateAccount)
	auth.GET(accountItem, rbac.RequirePermission("ledger", "read"), h.GetAccount)
	auth.PATCH(accountItem, rbac.RequirePermission("ledger", "update"), h.UpdateAccount)
	auth.DELETE(accountItem, rbac.RequirePermission("ledger", "delete"), h.ArchiveAccount)
	auth.POST(accountItem+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("ledger", "update"), h.ArchiveAccount)
	auth.POST(accountItem+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("ledger", "update"), h.RestoreAccount)
	auth.GET(accountItem+"/ledger", rbac.RequirePermission("ledger", "read"), h.AccountLedger)

	// Account links (rol -> cuenta).
	auth.GET(base+"/account-links", rbac.RequirePermission("ledger", "read"), h.ListLinks)
	auth.PUT(base+"/account-links/:role", rbac.RequirePermission("ledger", "update"), h.SetLink)

	// Asientos y reportes.
	auth.POST(base+"/entries", rbac.RequirePermission("ledger", "create"), h.PostManualEntry)
	auth.GET(base+"/journal", rbac.RequirePermission("ledger", "read"), h.Journal)
	auth.GET(base+"/trial-balance", rbac.RequirePermission("ledger", "read"), h.TrialBalance)
	auth.GET(base+"/health", rbac.RequirePermission("ledger", "read"), h.Health)

	// Activación / seed de la plantilla.
	auth.POST(base+"/setup", rbac.RequirePermission("ledger", "update"), h.Setup)
}

func (h *Handler) ListAccounts(c *gin.Context) { h.listAccounts(c, false) }

func (h *Handler) ListArchivedAccounts(c *gin.Context) { h.listAccounts(c, true) }

func (h *Handler) listAccounts(c *gin.Context, includeArchived bool) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	items, err := h.uc.ListAccounts(c.Request.Context(), orgID, includeArchived)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) GetAccount(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.GetAccount(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CreateAccount(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	var req dto.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	parentID, ok := parseOptionalUUID(c, req.ParentID)
	if !ok {
		return
	}
	out, err := h.uc.CreateAccount(c.Request.Context(), ledgerdomain.Account{
		OrgID:      orgID,
		Code:       req.Code,
		Name:       req.Name,
		Type:       req.Type,
		ParentID:   parentID,
		IsPostable: boolOr(req.IsPostable, true),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) UpdateAccount(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	parentID, ok := parseOptionalUUID(c, req.ParentID)
	if !ok {
		return
	}
	out, err := h.uc.UpdateAccount(c.Request.Context(), ledgerdomain.Account{
		ID:         id,
		OrgID:      orgID,
		Name:       req.Name,
		Type:       req.Type,
		ParentID:   parentID,
		IsPostable: boolOr(req.IsPostable, true),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ArchiveAccount(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.ArchiveAccount(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RestoreAccount(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.RestoreAccount(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) AccountLedger(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	from, okFrom := parseDateQuery(c, "from")
	if !okFrom {
		return
	}
	to, okTo := parseDateQuery(c, "to")
	if !okTo {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "100", pagination.Config{DefaultLimit: 100, MaxLimit: 500})
	out, err := h.uc.AccountLedger(c.Request.Context(), orgID, id, from, to, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListLinks(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	items, err := h.uc.ListLinks(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) SetLink(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	role := strings.TrimSpace(c.Param("role"))
	var req dto.SetLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.AccountID))
	if err != nil {
		handlers.WriteValidation(c, "invalid account_id")
		return
	}
	out, err := h.uc.SetLink(c.Request.Context(), orgID, role, accountID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) PostManualEntry(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		handlers.WriteValidation(c, "invalid tenant")
		return
	}
	var req dto.PostEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	entryDate, ok := parseDateValue(c, req.EntryDate)
	if !ok {
		return
	}
	lines := make([]ledgerdomain.JournalLine, 0, len(req.Lines))
	for _, l := range req.Lines {
		accountID, err := uuid.Parse(strings.TrimSpace(l.AccountID))
		if err != nil {
			handlers.WriteValidation(c, "invalid account_id in line")
			return
		}
		partyID, ok := parseOptionalUUID(c, l.PartyID)
		if !ok {
			return
		}
		lines = append(lines, ledgerdomain.JournalLine{
			OrgID:     orgID,
			AccountID: accountID,
			Debit:     l.Debit,
			Credit:    l.Credit,
			PartyID:   partyID,
			Memo:      strings.TrimSpace(l.Memo),
		})
	}
	out, err := h.uc.PostManual(c.Request.Context(), ledgerdomain.JournalEntry{
		OrgID:       orgID,
		EntryDate:   entryDate,
		Currency:    strings.TrimSpace(req.Currency),
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   authCtx.Actor,
		Lines:       lines,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) Journal(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	from, okFrom := parseDateQuery(c, "from")
	if !okFrom {
		return
	}
	to, okTo := parseDateQuery(c, "to")
	if !okTo {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "50", pagination.Config{DefaultLimit: 50, MaxLimit: 200})
	items, err := h.uc.Journal(c.Request.Context(), orgID, from, to, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) TrialBalance(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	asOf, ok := parseDateQuery(c, "as_of")
	if !ok {
		return
	}
	out, err := h.uc.TrialBalance(c.Request.Context(), orgID, asOf)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Health(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	out, err := h.uc.Health(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Setup(c *gin.Context) {
	orgID, ok := handlers.ParseAuthTenantID(c)
	if !ok {
		return
	}
	items, err := h.uc.Setup(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// --- helpers ---

// parseDateQuery lee un query param de fecha (YYYY-MM-DD). Ausente => zero time
// (sin filtro). Inválido => escribe 400 y retorna ok=false.
func parseDateQuery(c *gin.Context, key string) (time.Time, bool) {
	return parseDateValue(c, c.Query(key))
}

func parseDateValue(c *gin.Context, raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, true
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		handlers.WriteValidation(c, "invalid date (expected YYYY-MM-DD)")
		return time.Time{}, false
	}
	return t, true
}

func parseOptionalUUID(c *gin.Context, raw *string) (*uuid.UUID, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, true
	}
	id, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		handlers.WriteValidation(c, "invalid uuid")
		return nil, false
	}
	return &id, true
}

func boolOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}
