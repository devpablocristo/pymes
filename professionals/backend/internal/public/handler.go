// Package public exposes public professionals routes and bridges to control-plane where needed.
package public

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	profdomain "github.com/devpablocristo/pymes/professionals/backend/internal/professional_profiles/usecases/domain"
	sldomain "github.com/devpablocristo/pymes/professionals/backend/internal/service_links/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type profilePort interface {
	ListPublic(ctx context.Context, orgID uuid.UUID) ([]profdomain.ProfessionalProfile, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (profdomain.ProfessionalProfile, error)
}

type serviceLinkPort interface {
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]sldomain.ServiceLink, error)
}

type bookingPort interface {
	GetAvailability(ctx context.Context, orgRef string, date string, duration int) (map[string]any, error)
	BookAppointment(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error)
}

type orgResolver interface {
	ResolveOrgID(ctx context.Context, orgSlug string) (uuid.UUID, error)
}

type Handler struct {
	profiles     profilePort
	serviceLinks serviceLinkPort
	bookings     bookingPort
	orgs         orgResolver
}

func NewHandler(profiles profilePort, serviceLinks serviceLinkPort, bookings bookingPort, orgs orgResolver) *Handler {
	return &Handler{
		profiles:     profiles,
		serviceLinks: serviceLinks,
		bookings:     bookings,
		orgs:         orgs,
	}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/public/:org_slug/professionals", h.ListProfessionals)
	group.GET("/public/:org_slug/professionals/:slug", h.GetProfessional)
	group.GET("/public/:org_slug/catalog", h.ListCatalog)
	group.GET("/public/:org_slug/availability", h.GetAvailability)
	group.POST("/public/:org_slug/appointments", h.BookAppointment)
}

func (h *Handler) resolveOrgID(c *gin.Context) (uuid.UUID, bool) {
	orgSlug := strings.TrimSpace(c.Param("org_slug"))
	if orgSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_slug is required"})
		return uuid.Nil, false
	}

	// Try parsing as UUID first for backward compatibility
	if orgID, err := uuid.Parse(orgSlug); err == nil {
		return orgID, true
	}

	// Then try slug resolution via control-plane
	if h.orgs != nil {
		orgID, err := h.orgs.ResolveOrgID(c.Request.Context(), orgSlug)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return uuid.Nil, false
		}
		return orgID, true
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org identifier"})
	return uuid.Nil, false
}

func (h *Handler) ListProfessionals(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	profiles, err := h.profiles.ListPublic(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, publicProfileMap(p))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) GetProfessional(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}
	profile, err := h.profiles.GetBySlug(c.Request.Context(), orgID, slug)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, publicProfileMap(profile))
}

func (h *Handler) ListCatalog(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	links, err := h.serviceLinks.ListByOrg(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	items := make([]map[string]any, 0, len(links))
	for _, l := range links {
		items = append(items, map[string]any{
			"id":                 l.ID.String(),
			"profile_id":         l.ProfileID.String(),
			"product_id":         l.ProductID.String(),
			"public_description": l.PublicDescription,
			"display_order":      l.DisplayOrder,
			"is_featured":        l.IsFeatured,
			"metadata":           l.Metadata,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) GetAvailability(c *gin.Context) {
	if h.bookings == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "availability not configured"})
		return
	}
	orgSlug := strings.TrimSpace(c.Param("org_slug"))
	if orgSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_slug is required"})
		return
	}
	duration := 0
	if raw := strings.TrimSpace(c.Query("duration")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duration must be an integer"})
			return
		}
		duration = parsed
	}
	out, err := h.bookings.GetAvailability(c.Request.Context(), orgSlug, strings.TrimSpace(c.Query("date")), duration)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) BookAppointment(c *gin.Context) {
	if h.bookings == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "booking not configured"})
		return
	}
	orgSlug := strings.TrimSpace(c.Param("org_slug"))
	if orgSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_slug is required"})
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.bookings.BookAppointment(c.Request.Context(), orgSlug, payload)
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
