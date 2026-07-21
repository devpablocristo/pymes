// Package handlers — pagination helpers split between this package and
// platform/http/gin/go (v0.2.0+):
//
//   - ParseAfterUUIDQuery / WriteListResponse / WriteOffsetListResponse are
//     re-exported from ginmw (identical semantics, no pymes-specific deps).
//   - ParseLimitQuery is kept here because its signature uses pymes' shared
//     pagination.Config (configurable default + max, distinct from platform's
//     default/max int pair). New code should prefer ginmw.ParseLimitQuery if
//     it doesn't need Config.
package handlers

import (
	"strconv"
	"strings"

	"github.com/devpablocristo/platform/http/go/pagination"
	ginmw "github.com/devpablocristo/platform/http/gin/go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ParseLimitQuery reads the `key` query param and normalizes via
// pagination.NormalizeLimit. Non-numeric / non-positive values fall through
// to the config default (lenient parsing to match repository behavior).
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

// ParseAfterUUIDQuery delegates to platform.
func ParseAfterUUIDQuery(c *gin.Context) (*uuid.UUID, bool) {
	return ginmw.ParseAfterUUIDQuery(c)
}

// WriteListResponse delegates to platform.
func WriteListResponse(c *gin.Context, items any, total int64, hasMore bool, nextCursor string) {
	ginmw.WriteListResponse(c, items, total, hasMore, nextCursor)
}

// WriteOffsetListResponse delegates to platform.
func WriteOffsetListResponse(c *gin.Context, items any, limit int, total int) {
	ginmw.WriteOffsetListResponse(c, items, limit, total)
}
