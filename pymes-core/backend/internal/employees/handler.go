package employees

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	crudpaths "github.com/devpablocristo/modules/crud/paths/go/paths"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/employees/handler/dto"
	empdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/employees/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]empdomain.Employee, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]empdomain.Employee, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (empdomain.Employee, error)
	Create(ctx context.Context, in CreateInput) (empdomain.Employee, error)
	Update(ctx context.Context, in UpdateInput) (empdomain.Employee, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	const base = "/employees"
	const item = base + "/:id"

	auth.GET(base, rbac.RequirePermission("employees", "read"), h.List)
	auth.GET(base+"/"+crudpaths.SegmentArchived, rbac.RequirePermission("employees", "read"), h.ListArchived)
	auth.POST(base, rbac.RequirePermission("employees", "create"), h.Create)
	auth.GET(item, rbac.RequirePermission("employees", "read"), h.Get)
	auth.PATCH(item, rbac.RequirePermission("employees", "update"), h.Update)
	auth.DELETE(item, rbac.RequirePermission("employees", "delete"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentArchive, rbac.RequirePermission("employees", "update"), h.Delete)
	auth.POST(item+"/"+crudpaths.SegmentRestore, rbac.RequirePermission("employees", "update"), h.RestoreAction)
	auth.DELETE(item+"/"+crudpaths.SegmentHard, rbac.RequirePermission("employees", "delete"), h.HardDelete)
}

func (h *Handler) List(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	after, ok := handlers.ParseAfterUUIDQuery(c)
	if !ok {
		return
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{OrgID: orgID, Limit: limit, After: after, Status: c.Query("status")})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListEmployeesResponse{Items: make([]dto.EmployeeResponse, 0, len(items)), Total: total, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = next.String()
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toEmployeeResponse(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListArchived(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.uc.ListArchived(c.Request.Context(), orgID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := dto.ListEmployeesResponse{Items: make([]dto.EmployeeResponse, 0, len(items)), Total: int64(len(items)), HasMore: false}
	for _, it := range items {
		resp.Items = append(resp.Items, toEmployeeResponse(it))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	var req dto.CreateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	isFavorite := false
	if req.IsFavorite != nil {
		isFavorite = *req.IsFavorite
	}
	out, err := h.uc.Create(c.Request.Context(), CreateInput{
		OrgID:      orgID,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Email:      req.Email,
		Phone:      req.Phone,
		Position:   req.Position,
		Status:     req.Status,
		HireDate:   req.HireDate,
		EndDate:    req.EndDate,
		Notes:      req.Notes,
		IsFavorite: isFavorite,
		Tags:       req.Tags,
		CreatedBy:  a.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toEmployeeResponse(out))
}

func (h *Handler) Get(c *gin.Context) {
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toEmployeeResponse(out))
}

func (h *Handler) Update(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	var req dto.UpdateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), UpdateInput{
		OrgID:      orgID,
		ID:         id,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Email:      req.Email,
		Phone:      req.Phone,
		Position:   req.Position,
		Status:     req.Status,
		HireDate:   req.HireDate,
		EndDate:    req.EndDate,
		Notes:      req.Notes,
		IsFavorite: req.IsFavorite,
		Tags:       req.Tags,
		Actor:      a.Actor,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toEmployeeResponse(out))
}

func (h *Handler) Delete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.SoftDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RestoreAction(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.Restore(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) HardDelete(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, id, ok := parseOrgAndID(c)
	if !ok {
		return
	}
	if err := h.uc.HardDelete(c.Request.Context(), orgID, id, a.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseOrgAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}

func toEmployeeResponse(in empdomain.Employee) dto.EmployeeResponse {
	tags := in.Tags
	if tags == nil {
		tags = []string{}
	}
	resp := dto.EmployeeResponse{
		ID:         in.ID.String(),
		OrgID:      in.OrgID.String(),
		FirstName:  in.FirstName,
		LastName:   in.LastName,
		Email:      in.Email,
		Phone:      in.Phone,
		Position:   in.Position,
		Status:     string(in.Status),
		Notes:      in.Notes,
		IsFavorite: in.IsFavorite,
		Tags:       tags,
		CreatedBy:  in.CreatedBy,
		CreatedAt:  in.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  in.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if in.HireDate != nil {
		resp.HireDate = in.HireDate.UTC().Format("2006-01-02")
	}
	if in.EndDate != nil {
		resp.EndDate = in.EndDate.UTC().Format("2006-01-02")
	}
	if in.UserID != nil {
		resp.UserID = in.UserID.String()
	}
	if in.ArchivedAt != nil {
		resp.ArchivedAt = in.ArchivedAt.UTC().Format(time.RFC3339)
	}
	return resp
}
