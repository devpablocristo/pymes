package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
)

func ParseAuthOrgID(c *gin.Context) (uuid.UUID, bool) {
	authCtx := auth.GetAuthContext(c)
	orgID, err := uuid.Parse(strings.TrimSpace(authCtx.OrgID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, false
	}
	return orgID, true
}

func ParseUUIDParam(c *gin.Context, param string, field string) (uuid.UUID, bool) {
	value := strings.TrimSpace(c.Param(param))
	id, err := uuid.Parse(value)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + field})
		return uuid.Nil, false
	}
	return id, true
}

func ParseAuthOrgAndParamID(c *gin.Context, param string, field string) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := ParseAuthOrgID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, ok := ParseUUIDParam(c, param, field)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}

func ParseRFC3339(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(raw))
}

func ParseOptionalRFC3339(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func ParseOptionalRFC3339Ptr(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	return ParseOptionalRFC3339(*raw)
}

func ParseNullableRFC3339Ptr(raw *string) (**time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	if strings.TrimSpace(*raw) == "" {
		var value *time.Time
		return &value, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		return nil, err
	}
	value := &parsed
	return &value, nil
}
