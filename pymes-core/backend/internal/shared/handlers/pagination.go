package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ParseLimitQuery lee el query param `key` y aplica pagination.NormalizeLimit.
// Valores no numéricos o <=0 se normalizan al default de config (mismo criterio tolerante
// que strconv.Atoi ignorado + capa de repositorio).
// Si el param no está en la URL, se usa defaultWhenMissing como string (p.ej. "20"), igual que Gin DefaultQuery.
func ParseLimitQuery(c *gin.Context, key string, defaultWhenMissing string, config pagination.Config) int {
	raw := queryRawOrDefault(c, key, defaultWhenMissing)
	return pagination.NormalizeLimit(tolerantAtoi(raw), config)
}

func queryRawOrDefault(c *gin.Context, key string, defaultWhenMissing string) string {
	q, ok := c.GetQuery(key)
	if !ok {
		return strings.TrimSpace(defaultWhenMissing)
	}
	return strings.TrimSpace(q)
}

func tolerantAtoi(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

// ParseAfterUUIDQuery parsea el cursor `after` como UUID. Vacío → (nil, true).
// Presente e inválido → 400 {"error":"invalid after"} y (nil, false).
func ParseAfterUUIDQuery(c *gin.Context) (*uuid.UUID, bool) {
	v := strings.TrimSpace(c.Query("after"))
	if v == "" {
		return nil, true
	}
	id, err := uuid.Parse(v)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid after"})
		return nil, false
	}
	return &id, true
}
