package attachments

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	attachmentdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/attachments/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/internal/shared/httperrors"
)

type usecasesPort interface {
	RequestUpload(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, fileName, contentType string, sizeBytes int64) (attachmentdomain.UploadRequest, error)
	SaveUpload(ctx context.Context, in attachmentdomain.Attachment) (attachmentdomain.Attachment, error)
	UploadContent(ctx context.Context, storageKey string, body io.Reader) error
	GetDownloadLink(ctx context.Context, orgID, id uuid.UUID) (attachmentdomain.Attachment, attachmentdomain.DownloadLink, error)
	OpenContent(ctx context.Context, orgID, id uuid.UUID) (attachmentdomain.Attachment, *os.File, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	ListByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, limit int) ([]attachmentdomain.Attachment, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	auth.POST("/attachments/upload-url", h.RequestUpload)
	auth.PUT("/attachments/uploads/*storage_key", h.UploadContent)
	auth.POST("/attachments/confirm", h.ConfirmUpload)
	auth.GET("/attachments/:id/url", h.GetURL)
	auth.GET("/attachments/:id/download", h.Download)
	auth.DELETE("/attachments/:id", h.Delete)
	auth.GET("/:entity/:id/attachments", h.ListByEntity)
}

func (h *Handler) RequestUpload(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	var req struct {
		EntityType  string `json:"entity_type" binding:"required"`
		EntityID    string `json:"entity_id" binding:"required"`
		FileName    string `json:"file_name" binding:"required"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entityID, err := uuid.Parse(strings.TrimSpace(req.EntityID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_id"})
		return
	}
	out, err := h.uc.RequestUpload(c.Request.Context(), orgID, req.EntityType, entityID, req.FileName, req.ContentType, req.SizeBytes)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) UploadContent(c *gin.Context) {
	storageKey := strings.TrimPrefix(c.Param("storage_key"), "/")
	if strings.TrimSpace(storageKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid storage_key"})
		return
	}
	if err := h.uc.UploadContent(c.Request.Context(), storageKey, c.Request.Body); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) ConfirmUpload(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	var req struct {
		EntityType  string `json:"entity_type" binding:"required"`
		EntityID    string `json:"entity_id" binding:"required"`
		FileName    string `json:"file_name" binding:"required"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
		StorageKey  string `json:"storage_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entityID, err := uuid.Parse(strings.TrimSpace(req.EntityID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_id"})
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.SaveUpload(c.Request.Context(), attachmentdomain.Attachment{OrgID: orgID, AttachableType: strings.TrimSpace(req.EntityType), AttachableID: entityID, FileName: strings.TrimSpace(req.FileName), ContentType: strings.TrimSpace(req.ContentType), SizeBytes: req.SizeBytes, StorageKey: strings.TrimSpace(req.StorageKey), UploadedBy: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) GetURL(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	_, link, err := h.uc.GetDownloadLink(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, link)
}

func (h *Handler) Download(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	item, file, err := h.uc.OpenContent(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	defer file.Close()
	c.Header("Content-Disposition", "attachment; filename=\""+item.FileName+"\"")
	c.DataFromReader(http.StatusOK, item.SizeBytes, item.ContentType, file, nil)
}

func (h *Handler) Delete(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.Delete(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListByEntity(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	entityID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.uc.ListByEntity(c.Request.Context(), orgID, strings.TrimSpace(c.Param("entity")), entityID, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func parseOrg(c *gin.Context) (uuid.UUID, bool) {
	auth := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(auth.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, false
	}
	return orgID, true
}

func parseOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := parseOrg(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}
