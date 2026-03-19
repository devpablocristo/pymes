package party

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/party/handler/dto"
	partydomain "github.com/devpablocristo/pymes/control-plane/backend/internal/party/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	apperror "github.com/devpablocristo/pymes/pkgs/go-pkg/apperror"
)

type usecasesPort interface {
	List(ctx context.Context, p ListParams) ([]partydomain.Party, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in partydomain.Party, actor string) (partydomain.Party, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (partydomain.Party, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in partydomain.Party, actor string) (partydomain.Party, error)
	Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	AddRole(ctx context.Context, orgID, partyID uuid.UUID, role string, priceListID *uuid.UUID, metadata map[string]any, actor string) (partydomain.PartyRole, error)
	RemoveRole(ctx context.Context, orgID, partyID uuid.UUID, role, actor string) error
	ListRelationships(ctx context.Context, orgID, partyID uuid.UUID) ([]partydomain.PartyRelationship, error)
	CreateRelationship(ctx context.Context, in partydomain.PartyRelationship, actor string) (partydomain.PartyRelationship, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/parties", rbac.RequirePermission("parties", "read"), h.List)
	auth.POST("/parties", rbac.RequirePermission("parties", "create"), h.Create)
	auth.GET("/parties/:id", rbac.RequirePermission("parties", "read"), h.Get)
	auth.PUT("/parties/:id", rbac.RequirePermission("parties", "update"), h.Update)
	auth.DELETE("/parties/:id", rbac.RequirePermission("parties", "delete"), h.Delete)
	auth.POST("/parties/:id/roles", rbac.RequirePermission("parties", "update"), h.AddRole)
	auth.DELETE("/parties/:id/roles/:role", rbac.RequirePermission("parties", "update"), h.RemoveRole)
	auth.GET("/parties/:id/relationships", rbac.RequirePermission("parties", "read"), h.ListRelationships)
	auth.POST("/parties/:id/relationships", rbac.RequirePermission("parties", "update"), h.CreateRelationship)
}

func (h *Handler) List(c *gin.Context) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	var after *uuid.UUID
	if v := strings.TrimSpace(c.Query("after")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid after"})
			return
		}
		after = &id
	}
	items, total, hasMore, next, err := h.uc.List(c.Request.Context(), ListParams{
		OrgID:     orgID,
		Limit:     limit,
		After:     after,
		Search:    c.Query("search"),
		PartyType: c.Query("party_type"),
		Role:      c.Query("role"),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	resp := gin.H{"items": items, "total": total, "has_more": hasMore}
	if next != nil {
		resp["next_cursor"] = next.String()
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	orgID, actor, ok := parseOrgActor(c)
	if !ok {
		return
	}
	var req dto.CreatePartyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Create(c.Request.Context(), fromCreateRequest(orgID, req), actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) Get(c *gin.Context) {
	orgID, _, ok := parseOrgActor(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	out, err := h.uc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Update(c *gin.Context) {
	orgID, actor, ok := parseOrgActor(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.UpdatePartyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Update(c.Request.Context(), orgID, id, fromUpdateRequest(orgID, id, req), actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Delete(c *gin.Context) {
	orgID, actor, ok := parseOrgActor(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.Delete(c.Request.Context(), orgID, id, actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) AddRole(c *gin.Context) {
	orgID, actor, ok := parseOrgActor(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.PartyRoleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	var priceListID *uuid.UUID
	if req.PriceListID != nil && strings.TrimSpace(*req.PriceListID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.PriceListID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid price_list_id"})
			return
		}
		priceListID = &parsed
	}
	out, err := h.uc.AddRole(c.Request.Context(), orgID, id, req.Role, priceListID, req.Metadata, actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) RemoveRole(c *gin.Context) {
	orgID, actor, ok := parseOrgActor(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.RemoveRole(c.Request.Context(), orgID, id, c.Param("role"), actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListRelationships(c *gin.Context) {
	orgID, _, ok := parseOrgActor(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	items, err := h.uc.ListRelationships(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateRelationship(c *gin.Context) {
	orgID, actor, ok := parseOrgActor(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req dto.RelationshipInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	toPartyID, err := uuid.Parse(req.ToPartyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to_party_id"})
		return
	}
	fromDate, thruDate, err := parseRelationshipDates(req.FromDate, req.ThruDate)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	out, err := h.uc.CreateRelationship(c.Request.Context(), partydomain.PartyRelationship{
		OrgID:            orgID,
		FromPartyID:      id,
		ToPartyID:        toPartyID,
		RelationshipType: strings.TrimSpace(req.RelationshipType),
		Metadata:         req.Metadata,
		FromDate:         fromDate,
		ThruDate:         thruDate,
	}, actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func parseOrgActor(c *gin.Context) (uuid.UUID, string, bool) {
	a := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(a.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, "", false
	}
	return orgID, a.Actor, true
}

func fromCreateRequest(orgID uuid.UUID, req dto.CreatePartyRequest) partydomain.Party {
	roles := make([]partydomain.PartyRole, 0, len(req.Roles))
	for _, role := range req.Roles {
		var priceListID *uuid.UUID
		if role.PriceListID != nil {
			if parsed, err := uuid.Parse(strings.TrimSpace(*role.PriceListID)); err == nil {
				priceListID = &parsed
			}
		}
		roles = append(roles, partydomain.PartyRole{Role: strings.TrimSpace(role.Role), PriceListID: priceListID, Metadata: role.Metadata, IsActive: true})
	}
	return partydomain.Party{
		OrgID:        orgID,
		PartyType:    req.PartyType,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Phone:        req.Phone,
		Address:      toDomainAddress(req.Address),
		TaxID:        req.TaxID,
		Notes:        req.Notes,
		Tags:         req.Tags,
		Metadata:     req.Metadata,
		Person:       toDomainPerson(req.Person),
		Organization: toDomainOrganization(req.Organization),
		Agent:        toDomainAgent(req.Agent),
		Roles:        roles,
	}
}

func fromUpdateRequest(orgID, id uuid.UUID, req dto.UpdatePartyRequest) partydomain.Party {
	return partydomain.Party{
		ID:           id,
		OrgID:        orgID,
		PartyType:    req.PartyType,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Phone:        req.Phone,
		Address:      toDomainAddress(req.Address),
		TaxID:        req.TaxID,
		Notes:        req.Notes,
		Tags:         req.Tags,
		Metadata:     req.Metadata,
		Person:       toDomainPerson(req.Person),
		Organization: toDomainOrganization(req.Organization),
		Agent:        toDomainAgent(req.Agent),
	}
}

func toDomainAddress(in dto.Address) partydomain.Address {
	return partydomain.Address{Street: strings.TrimSpace(in.Street), City: strings.TrimSpace(in.City), State: strings.TrimSpace(in.State), ZipCode: strings.TrimSpace(in.ZipCode), Country: strings.TrimSpace(in.Country)}
}

func toDomainPerson(in *dto.PartyPerson) *partydomain.PartyPerson {
	if in == nil {
		return nil
	}
	return &partydomain.PartyPerson{FirstName: strings.TrimSpace(in.FirstName), LastName: strings.TrimSpace(in.LastName)}
}

func toDomainOrganization(in *dto.PartyOrganization) *partydomain.PartyOrganization {
	if in == nil {
		return nil
	}
	return &partydomain.PartyOrganization{LegalName: strings.TrimSpace(in.LegalName), TradeName: strings.TrimSpace(in.TradeName), TaxCondition: strings.TrimSpace(in.TaxCondition)}
}

func toDomainAgent(in *dto.PartyAgent) *partydomain.PartyAgent {
	if in == nil {
		return nil
	}
	return &partydomain.PartyAgent{AgentKind: strings.TrimSpace(in.AgentKind), Provider: strings.TrimSpace(in.Provider), Config: in.Config, IsActive: in.IsActive}
}

func parseRelationshipDates(fromRaw, thruRaw *string) (time.Time, *time.Time, error) {
	from := time.Now().UTC()
	if fromRaw != nil && strings.TrimSpace(*fromRaw) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*fromRaw))
		if err != nil {
			return time.Time{}, nil, apperror.NewBadInput("invalid from_date")
		}
		from = parsed.UTC()
	}
	var thru *time.Time
	if thruRaw != nil && strings.TrimSpace(*thruRaw) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*thruRaw))
		if err != nil {
			return time.Time{}, nil, apperror.NewBadInput("invalid thru_date")
		}
		parsed = parsed.UTC()
		thru = &parsed
	}
	return from, thru, nil
}
