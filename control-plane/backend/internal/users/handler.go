package users

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/users/handler/dto"
	userdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/users/usecases/domain"
)

type usecasesPort interface {
	GetMe(ctx context.Context, actor string) (userdomain.User, error)
	ListMembers(ctx context.Context, orgID string) ([]userdomain.Member, error)
	ListAPIKeys(ctx context.Context, orgID string) ([]userdomain.APIKey, error)
	CreateAPIKey(ctx context.Context, orgID, name, createdBy string, scopes []string) (userdomain.APIKey, string, error)
	DeleteAPIKey(ctx context.Context, orgID, keyID string) error
	RotateAPIKey(ctx context.Context, orgID, keyID string) (userdomain.APIKey, string, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	auth.GET("/users/me", h.GetMe)
	auth.GET("/orgs/:org_id/members", h.ListMembers)
	auth.GET("/orgs/:org_id/api-keys", h.ListAPIKeys)
	auth.POST("/orgs/:org_id/api-keys", h.CreateAPIKey)
	auth.DELETE("/orgs/:org_id/api-keys/:id", h.DeleteAPIKey)
	auth.POST("/orgs/:org_id/api-keys/:id/rotate", h.RotateAPIKey)
}

func (h *Handler) GetMe(c *gin.Context) {
	auth := handlers.GetAuthContext(c)
	me, err := h.uc.GetMe(c.Request.Context(), auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, me)
}

func (h *Handler) ListMembers(c *gin.Context) {
	auth := handlers.GetAuthContext(c)
	orgID := c.Param("org_id")
	if orgID != auth.OrgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cross-org access denied"})
		return
	}
	members, err := h.uc.ListMembers(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": members})
}

func (h *Handler) ListAPIKeys(c *gin.Context) {
	auth := handlers.GetAuthContext(c)
	orgID := c.Param("org_id")
	if orgID != auth.OrgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cross-org access denied"})
		return
	}
	keys, err := h.uc.ListAPIKeys(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": keys})
}

func (h *Handler) CreateAPIKey(c *gin.Context) {
	auth := handlers.GetAuthContext(c)
	orgID := c.Param("org_id")
	if orgID != auth.OrgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cross-org access denied"})
		return
	}

	var req dto.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key, raw, err := h.uc.CreateAPIKey(c.Request.Context(), orgID, req.Name, auth.Actor, req.Scopes)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"key": key, "raw_key": raw})
}

func (h *Handler) DeleteAPIKey(c *gin.Context) {
	auth := handlers.GetAuthContext(c)
	orgID := c.Param("org_id")
	if orgID != auth.OrgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cross-org access denied"})
		return
	}
	if err := h.uc.DeleteAPIKey(c.Request.Context(), orgID, c.Param("id")); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RotateAPIKey(c *gin.Context) {
	auth := handlers.GetAuthContext(c)
	orgID := c.Param("org_id")
	if orgID != auth.OrgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cross-org access denied"})
		return
	}
	key, raw, err := h.uc.RotateAPIKey(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "raw_key": raw})
}
