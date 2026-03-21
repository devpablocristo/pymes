package procurement

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	Create(ctx context.Context, in CreateInput) (domain.ProcurementRequest, error)
	Update(ctx context.Context, in UpdateInput) (domain.ProcurementRequest, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProcurementRequest, error)
	List(ctx context.Context, orgID uuid.UUID, archived bool, limit int) ([]domain.ProcurementRequest, error)
	Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Submit(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.ProcurementRequest, error)
	Approve(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.ProcurementRequest, error)
	Reject(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.ProcurementRequest, error)

	ListPoliciesForOrg(ctx context.Context, orgID uuid.UUID) ([]domain.ProcurementPolicy, error)
	GetPolicy(ctx context.Context, orgID, id uuid.UUID) (domain.ProcurementPolicy, error)
	CreatePolicy(ctx context.Context, in PolicyCreateInput) (domain.ProcurementPolicy, error)
	UpdatePolicy(ctx context.Context, in PolicyUpdateInput) (domain.ProcurementPolicy, error)
	DeletePolicy(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	g := auth.Group("/procurement-requests")
	{
		g.GET("", rbac.RequirePermission("procurement_requests", "read"), h.List)
		g.POST("", rbac.RequirePermission("procurement_requests", "create"), h.Create)
		g.GET("/:id", rbac.RequirePermission("procurement_requests", "read"), h.Get)
		g.PATCH("/:id", rbac.RequirePermission("procurement_requests", "update"), h.Update)
		g.DELETE("/:id", rbac.RequirePermission("procurement_requests", "delete"), h.Delete)
		g.POST("/:id/archive", rbac.RequirePermission("procurement_requests", "update"), h.Archive)
		g.POST("/:id/restore", rbac.RequirePermission("procurement_requests", "update"), h.Restore)
		g.POST("/:id/submit", rbac.RequirePermission("procurement_requests", "submit"), h.Submit)
		g.POST("/:id/approve", rbac.RequirePermission("procurement_requests", "approve"), h.Approve)
		g.POST("/:id/reject", rbac.RequirePermission("procurement_requests", "reject"), h.Reject)
	}

	pg := auth.Group("/procurement-policies")
	{
		pg.GET("", rbac.RequirePermission("procurement_policies", "read"), h.ListPolicies)
		pg.POST("", rbac.RequirePermission("procurement_policies", "create"), h.CreatePolicy)
		pg.GET("/:id", rbac.RequirePermission("procurement_policies", "read"), h.GetPolicy)
		pg.PATCH("/:id", rbac.RequirePermission("procurement_policies", "update"), h.UpdatePolicy)
		pg.DELETE("/:id", rbac.RequirePermission("procurement_policies", "delete"), h.DeletePolicy)
	}
}

func (h *Handler) List(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	archived := strings.EqualFold(strings.TrimSpace(c.Query("archived")), "true")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.uc.List(c.Request.Context(), orgID, archived, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out := make([]dto.RequestResponse, 0, len(items))
	for _, it := range items {
		out = append(out, toResponse(it))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *Handler) Create(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.CreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	lines := toDomainLines(body.Lines)
	created, err := h.uc.Create(c.Request.Context(), CreateInput{
		OrgID:          orgID,
		Actor:          auth.Actor,
		Title:          body.Title,
		Description:    body.Description,
		Category:       body.Category,
		EstimatedTotal: body.EstimatedTotal,
		Currency:       body.Currency,
		Lines:          lines,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toResponse(created))
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	item, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(item))
}

func (h *Handler) Update(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.UpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	lines := toDomainLines(body.Lines)
	updated, err := h.uc.Update(c.Request.Context(), UpdateInput{
		OrgID:          orgID,
		ID:             id,
		Actor:          auth.Actor,
		Title:          body.Title,
		Description:    body.Description,
		Category:       body.Category,
		EstimatedTotal: body.EstimatedTotal,
		Currency:       body.Currency,
		Lines:          lines,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(updated))
}

func (h *Handler) Delete(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.Delete(c.Request.Context(), orgID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Archive(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.Archive(c.Request.Context(), orgID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Restore(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.Restore(c.Request.Context(), orgID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Submit(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.Submit(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(out))
}

func (h *Handler) Approve(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.Approve(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(out))
}

func (h *Handler) Reject(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.Reject(c.Request.Context(), orgID, id, auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(out))
}

func (h *Handler) ListPolicies(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	items, err := h.uc.ListPoliciesForOrg(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out := make([]dto.PolicyResponse, 0, len(items))
	for _, it := range items {
		out = append(out, toPolicyResponse(it))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *Handler) GetPolicy(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	item, err := h.uc.GetPolicy(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toPolicyResponse(item))
}

func (h *Handler) CreatePolicy(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.CreatePolicyRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	created, err := h.uc.CreatePolicy(c.Request.Context(), PolicyCreateInput{
		OrgID:          orgID,
		Actor:          auth.Actor,
		Name:           body.Name,
		Expression:     body.Expression,
		Effect:         body.Effect,
		Priority:       body.Priority,
		Mode:           body.Mode,
		Enabled:        body.Enabled,
		ActionFilter:   body.ActionFilter,
		SystemFilter:   body.SystemFilter,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toPolicyResponse(created))
}

func (h *Handler) UpdatePolicy(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	var body dto.UpdatePolicyRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	updated, err := h.uc.UpdatePolicy(c.Request.Context(), PolicyUpdateInput{
		OrgID:          orgID,
		ID:             id,
		Actor:          auth.Actor,
		Name:           body.Name,
		Expression:     body.Expression,
		Effect:         body.Effect,
		Priority:       body.Priority,
		Mode:           body.Mode,
		Enabled:        body.Enabled,
		ActionFilter:   body.ActionFilter,
		SystemFilter:   body.SystemFilter,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toPolicyResponse(updated))
}

func (h *Handler) DeletePolicy(c *gin.Context) {
	orgID, id, ok := handlers.ParseAuthOrgAndParamID(c, "id", "id")
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.DeletePolicy(c.Request.Context(), orgID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func toPolicyResponse(p domain.ProcurementPolicy) dto.PolicyResponse {
	return dto.PolicyResponse{
		ID:           p.ID,
		OrgID:        p.OrgID,
		Name:         p.Name,
		Expression:   p.Expression,
		Effect:       p.Effect,
		Priority:     p.Priority,
		Mode:         p.Mode,
		Enabled:      p.Enabled,
		ActionFilter: p.ActionFilter,
		SystemFilter: p.SystemFilter,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func toDomainLines(lines []dto.RequestLine) []domain.RequestLine {
	out := make([]domain.RequestLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, domain.RequestLine{
			ID:                l.ID,
			Description:       l.Description,
			ProductID:         l.ProductID,
			Quantity:          l.Quantity,
			UnitPriceEstimate: l.UnitPriceEstimate,
		})
	}
	return out
}

func toResponse(r domain.ProcurementRequest) dto.RequestResponse {
	lines := make([]dto.RequestLine, 0, len(r.Lines))
	for _, l := range r.Lines {
		lines = append(lines, dto.RequestLine{
			ID:                l.ID,
			Description:       l.Description,
			ProductID:         l.ProductID,
			Quantity:          l.Quantity,
			UnitPriceEstimate: l.UnitPriceEstimate,
		})
	}
	var evalJSON []byte
	if len(r.EvaluationJSON) > 0 {
		evalJSON = append([]byte(nil), r.EvaluationJSON...)
	}
	return dto.RequestResponse{
		ID:             r.ID,
		OrgID:          r.OrgID,
		RequesterActor: r.RequesterActor,
		Title:          r.Title,
		Description:    r.Description,
		Category:       r.Category,
		Status:         string(r.Status),
		EstimatedTotal: r.EstimatedTotal,
		Currency:       r.Currency,
		EvaluationJSON: evalJSON,
		PurchaseID:     r.PurchaseID,
		Lines:          lines,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		ArchivedAt:     r.ArchivedAt,
	}
}
