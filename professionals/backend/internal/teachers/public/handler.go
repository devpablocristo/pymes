// Package public exposes public professionals routes and bridges to pymes-core where needed.
package public

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	corepublic "github.com/devpablocristo/pymes/professionals/backend/internal/shared/pymescore"
	profdomain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/usecases/domain"
	sldomain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/service_links/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
)

type profilePort interface {
	ListPublic(ctx context.Context, tenantID uuid.UUID) ([]profdomain.ProfessionalProfile, error)
	GetBySlug(ctx context.Context, tenantID uuid.UUID, slug string) (profdomain.ProfessionalProfile, error)
}

type serviceLinkPort interface {
	ListByOrg(ctx context.Context, tenantID uuid.UUID) ([]sldomain.ServiceLink, error)
}

type bookingPort interface {
	GetAvailability(ctx context.Context, orgRef string, params corepublic.AvailabilityParams) (map[string]any, error)
	BookScheduling(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error)
}

type orgResolver interface {
	ResolveTenantID(ctx context.Context, tenantSlug string) (uuid.UUID, error)
}

type Handler struct {
	profiles     profilePort
	serviceLinks serviceLinkPort
	bookings     bookingPort
	tenants      orgResolver
}

func NewHandler(profiles profilePort, serviceLinks serviceLinkPort, bookings bookingPort, tenants orgResolver) *Handler {
	return &Handler{
		profiles:     profiles,
		serviceLinks: serviceLinks,
		bookings:     bookings,
		tenants:      tenants,
	}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/public/:tenant_slug/teachers", h.ListProfessionals)
	group.GET("/public/:tenant_slug/teachers/:slug", h.GetProfessional)
	group.GET("/public/:tenant_slug/teachers/catalog", h.ListCatalog)
	group.GET("/public/:tenant_slug/teachers/availability", h.GetAvailability)
	group.POST("/public/:tenant_slug/teachers/bookings", h.BookScheduling)

	group.GET("/public/:tenant_slug/professionals", h.ListProfessionals)
	group.GET("/public/:tenant_slug/professionals/:slug", h.GetProfessional)
	group.GET("/public/:tenant_slug/catalog", h.ListCatalog)
	group.GET("/public/:tenant_slug/availability", h.GetAvailability)
	group.POST("/public/:tenant_slug/bookings", h.BookScheduling)
}

func (h *Handler) resolveOrgID(c *gin.Context) (uuid.UUID, bool) {
	tenantSlug := strings.TrimSpace(c.Param("tenant_slug"))
	if tenantSlug == "" {
		verticalgin.WriteValidation(c, "tenant_slug is required")
		return uuid.Nil, false
	}

	// Try parsing as UUID first; public routes also accept the stable slug form.
	if tenantID, err := uuid.Parse(tenantSlug); err == nil {
		return tenantID, true
	}

	// Then try slug resolution via pymes-core
	if h.tenants != nil {
		tenantID, err := h.tenants.ResolveTenantID(c.Request.Context(), tenantSlug)
		if err != nil {
			verticalgin.WriteError(c, http.StatusNotFound, "NOT_FOUND", "organization not found")
			return uuid.Nil, false
		}
		return tenantID, true
	}

	verticalgin.WriteValidation(c, "invalid tenant identifier")
	return uuid.Nil, false
}

func (h *Handler) ListProfessionals(c *gin.Context) {
	tenantID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	profiles, err := h.profiles.ListPublic(c.Request.Context(), tenantID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, publicProfileMap(p))
	}
	verticalgin.WriteListResponse(c, items, int64(len(items)), false, "")
}

func (h *Handler) GetProfessional(c *gin.Context) {
	tenantID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		verticalgin.WriteValidation(c, "slug is required")
		return
	}
	profile, err := h.profiles.GetBySlug(c.Request.Context(), tenantID, slug)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, publicProfileMap(profile))
}

func (h *Handler) ListCatalog(c *gin.Context) {
	tenantID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	links, err := h.serviceLinks.ListByOrg(c.Request.Context(), tenantID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]map[string]any, 0, len(links))
	for _, l := range links {
		item := map[string]any{
			"id":                 l.ID.String(),
			"profile_id":         l.ProfileID.String(),
			"service_id":         l.ServiceID.String(),
			"public_description": l.PublicDescription,
			"display_order":      l.DisplayOrder,
			"is_featured":        l.IsFeatured,
			"metadata":           l.Metadata,
		}
		items = append(items, item)
	}
	verticalgin.WriteListResponse(c, items, int64(len(items)), false, "")
}

func (h *Handler) GetAvailability(c *gin.Context) {
	if h.bookings == nil {
		verticalgin.WriteError(c, http.StatusNotImplemented, "UPSTREAM_UNAVAILABLE", "availability not configured")
		return
	}
	tenantSlug := strings.TrimSpace(c.Param("tenant_slug"))
	if tenantSlug == "" {
		verticalgin.WriteValidation(c, "tenant_slug is required")
		return
	}
	duration := 0
	if raw := strings.TrimSpace(c.Query("duration")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			verticalgin.WriteValidation(c, "duration must be an integer")
			return
		}
		duration = parsed
	}
	out, err := h.bookings.GetAvailability(c.Request.Context(), tenantSlug, corepublic.AvailabilityParams{
		Date:       strings.TrimSpace(c.Query("date")),
		Duration:   duration,
		BranchID:   strings.TrimSpace(c.Query("branch_id")),
		ServiceID:  strings.TrimSpace(c.Query("service_id")),
		ResourceID: strings.TrimSpace(c.Query("resource_id")),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) BookScheduling(c *gin.Context) {
	if h.bookings == nil {
		verticalgin.WriteError(c, http.StatusNotImplemented, "UPSTREAM_UNAVAILABLE", "booking not configured")
		return
	}
	tenantSlug := strings.TrimSpace(c.Param("tenant_slug"))
	if tenantSlug == "" {
		verticalgin.WriteValidation(c, "tenant_slug is required")
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.bookings.BookScheduling(c.Request.Context(), tenantSlug, payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func publicProfileMap(p profdomain.ProfessionalProfile) map[string]any {
	m := map[string]any{
		"id":                  p.ID.String(),
		"public_slug":         p.PublicSlug,
		"headline":            p.Headline,
		"bio":                 p.Bio,
		"is_bookable":         p.IsBookable,
		"accepts_new_clients": p.AcceptsNewClients,
		"created_at":          p.CreatedAt.UTC().Format(time.RFC3339),
	}
	if len(p.Specialties) > 0 {
		specs := make([]map[string]any, 0, len(p.Specialties))
		for _, s := range p.Specialties {
			specs = append(specs, map[string]any{
				"id":   s.ID.String(),
				"code": s.Code,
				"name": s.Name,
			})
		}
		m["specialties"] = specs
	}
	return m
}
