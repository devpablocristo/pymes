package dataio

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

const maxImportFileSize = 5 << 20

type usecasesPort interface {
	Preview(ctx context.Context, entity, filename string, fileData []byte) (Preview, error)
	ConfirmImport(ctx context.Context, entity string, orgID uuid.UUID, previewID, mode, actor string) (ImportResult, error)
	Template(entity, format string) ([]byte, string, string, error)
	Export(ctx context.Context, entity string, orgID uuid.UUID, format string, from, to *time.Time) ([]byte, string, string, error)
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.POST("/import/:entity/preview", rbac.RequirePermission("admin", "update"), h.Preview)
	auth.POST("/import/:entity/confirm", rbac.RequirePermission("admin", "update"), h.Confirm)
	auth.GET("/import/templates/:entity", rbac.RequirePermission("admin", "read"), h.Template)
	auth.GET("/export/:entity", rbac.RequirePermission("admin", "read"), h.Export)
}

func (h *Handler) Preview(c *gin.Context) {
	filename, body, err := readUpload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.Preview(c.Request.Context(), c.Param("entity"), filename, body)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type confirmRequest struct {
	PreviewID string `json:"preview_id"`
	Mode      string `json:"mode"`
}

func (h *Handler) Confirm(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	var req confirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.ConfirmImport(c.Request.Context(), c.Param("entity"), orgID, strings.TrimSpace(req.PreviewID), strings.TrimSpace(req.Mode), auth.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Template(c *gin.Context) {
	content, contentType, filename, err := h.uc.Template(c.Param("entity"), c.Query("format"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, contentType, content)
}

func (h *Handler) Export(c *gin.Context) {
	orgID, ok := handlers.ParseAuthOrgID(c)
	if !ok {
		return
	}
	from, err := parseDate(c.Query("from"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	to, err := parseDate(c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	content, contentType, filename, err := h.uc.Export(c.Request.Context(), c.Param("entity"), orgID, c.Query("format"), from, to)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, contentType, content)
}

func parseDate(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return nil, errors.New("invalid date")
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func readUpload(c *gin.Context) (string, []byte, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return "", nil, errors.New("file field is required")
	}
	if fileHeader.Size > maxImportFileSize {
		return "", nil, errors.New("file too large")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", nil, errors.New("failed to open upload")
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxImportFileSize+1))
	if err != nil {
		return "", nil, errors.New("failed to read upload")
	}
	if int64(len(body)) > maxImportFileSize {
		return "", nil, errors.New("file too large")
	}
	return safeFilename(fileHeader), body, nil
}

func safeFilename(fileHeader *multipart.FileHeader) string {
	name := strings.TrimSpace(fileHeader.Filename)
	if name == "" {
		return "import.csv"
	}
	return name
}
