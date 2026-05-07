package exams

import (
	"context"
	"net/http"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/medical/backend/internal/occupational_health/exams/handler/dto"
	domain "github.com/devpablocristo/pymes/medical/backend/internal/occupational_health/exams/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Exam, int64, error)
	Create(ctx context.Context, in domain.Exam, actor string) (domain.Exam, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (domain.Exam, error)
	Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateInput, actor string) (domain.Exam, error)
	Archive(ctx context.Context, tenantID, id uuid.UUID, actor string) error
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	base := "/occupational-health/exams"
	group.GET(base, h.List)
	group.POST(base, h.Create)
	group.GET(base+"/:id", h.Get)
	group.PATCH(base+"/:id", h.Update)
	group.DELETE(base+"/:id", h.Archive)
	group.POST(base+"/:id/archive", h.Archive)
}

func (h *Handler) List(c *gin.Context) {
	tenantID, ok := verticalgin.ParseAuthTenantID(c)
	if !ok {
		return
	}
	limit := verticalgin.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, total, err := h.uc.List(c.Request.Context(), ListParams{
		TenantID: tenantID,
		Limit:    limit,
		Search:   c.Query("search"),
		Status:   c.Query("status"),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ListExamsResponse{Items: toResponses(items), Total: total})
}

func (h *Handler) Create(c *gin.Context) {
	tenantID, ok := verticalgin.ParseAuthTenantID(c)
	if !ok {
		return
	}
	var req dto.CreateExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	scheduledAt, ok := parseOptionalTime(c, req.ScheduledAt, "scheduled_at")
	if !ok {
		return
	}
	out, err := h.uc.Create(c.Request.Context(), domain.Exam{
		TenantID:        tenantID,
		PatientName:     req.PatientName,
		PatientDocument: req.PatientDocument,
		EmployerName:    req.EmployerName,
		ExamType:        req.ExamType,
		Status:          req.Status,
		ScheduledAt:     scheduledAt,
		Result:          req.Result,
		Notes:           req.Notes,
	}, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, toResponse(out))
}

func (h *Handler) Get(c *gin.Context) {
	tenantID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), tenantID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(out))
}

func (h *Handler) Update(c *gin.Context) {
	tenantID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	var req dto.UpdateExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	scheduledAt, ok := parseOptionalTimePatch(c, req.ScheduledAt, "scheduled_at")
	if !ok {
		return
	}
	completedAt, ok := parseOptionalTimePatch(c, req.CompletedAt, "completed_at")
	if !ok {
		return
	}
	out, err := h.uc.Update(c.Request.Context(), tenantID, id, UpdateInput{
		PatientName:     req.PatientName,
		PatientDocument: req.PatientDocument,
		EmployerName:    req.EmployerName,
		ExamType:        req.ExamType,
		Status:          req.Status,
		ScheduledAt:     scheduledAt,
		CompletedAt:     completedAt,
		Result:          req.Result,
		Notes:           req.Notes,
	}, auth.GetAuthContext(c).Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(out))
}

func (h *Handler) Archive(c *gin.Context) {
	tenantID, id, ok := verticalgin.ParseAuthTenantAndParamID(c, "id", "id")
	if !ok {
		return
	}
	if err := h.uc.Archive(c.Request.Context(), tenantID, id, auth.GetAuthContext(c).Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseOptionalTime(c *gin.Context, value *string, field string) (*time.Time, bool) {
	if value == nil || *value == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		verticalgin.WriteValidation(c, "invalid "+field)
		return nil, false
	}
	return &parsed, true
}

func parseOptionalTimePatch(c *gin.Context, value *string, field string) (**time.Time, bool) {
	if value == nil {
		return nil, true
	}
	parsed, ok := parseOptionalTime(c, value, field)
	if !ok {
		return nil, false
	}
	return &parsed, true
}

func toResponses(items []domain.Exam) []dto.ExamResponse {
	out := make([]dto.ExamResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toResponse(item))
	}
	return out
}

func toResponse(item domain.Exam) dto.ExamResponse {
	return dto.ExamResponse{
		ID:              item.ID.String(),
		TenantID:        item.TenantID.String(),
		PatientName:     item.PatientName,
		PatientDocument: item.PatientDocument,
		EmployerName:    item.EmployerName,
		ExamType:        item.ExamType,
		Status:          item.Status,
		ScheduledAt:     formatTime(item.ScheduledAt),
		CompletedAt:     formatTime(item.CompletedAt),
		Result:          item.Result,
		Notes:           item.Notes,
		CreatedAt:       item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       item.UpdatedAt.Format(time.RFC3339),
	}
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	out := value.Format(time.RFC3339)
	return &out
}
