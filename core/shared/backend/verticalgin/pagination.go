package verticalgin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ParseLimitQuery normaliza un query param limit con el mismo criterio tolerante de los CRUDs core.
func ParseLimitQuery(c *gin.Context, key string, defaultWhenMissing string, config pagination.Config) int {
	raw := queryRawOrDefault(c, key, defaultWhenMissing)
	return pagination.NormalizeLimit(tolerantAtoi(raw), config)
}

func queryRawOrDefault(c *gin.Context, key string, defaultWhenMissing string) string {
	raw, ok := c.GetQuery(key)
	if !ok {
		return strings.TrimSpace(defaultWhenMissing)
	}
	return strings.TrimSpace(raw)
}

func tolerantAtoi(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}

// ParseAfterUUIDQuery parsea el cursor after como UUID. Si viene invalido, responde 400.
func ParseAfterUUIDQuery(c *gin.Context) (*uuid.UUID, bool) {
	raw := strings.TrimSpace(c.Query("after"))
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		WriteValidation(c, "invalid after")
		return nil, false
	}
	return &id, true
}

func WriteValidation(c *gin.Context, message string) {
	WriteError(c, http.StatusBadRequest, "VALIDATION", message)
}

func WriteError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{"code": code, "message": message})
}

func WriteListResponse(c *gin.Context, items any, total int64, hasMore bool, nextCursor string) {
	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"total":       total,
		"has_more":    hasMore,
		"next_cursor": nextCursor,
	})
}

func WriteOffsetListResponse(c *gin.Context, items any, limit int, total int) {
	WriteListResponse(c, items, int64(total), total > limit, "")
}
