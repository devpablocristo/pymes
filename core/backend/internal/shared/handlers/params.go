package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sharedauth "github.com/devpablocristo/pymes/core/shared/backend/auth"
)

func ParseAuthTenantID(c *gin.Context) (uuid.UUID, bool) {
	orgID, ok := sharedauth.ParseAuthTenantID(c)
	if !ok {
		WriteValidation(c, "invalid tenant")
		return uuid.Nil, false
	}
	return orgID, true
}

func ParseUUIDParam(c *gin.Context, param string, field string) (uuid.UUID, bool) {
	value := strings.TrimSpace(c.Param(param))
	id, err := uuid.Parse(value)
	if err != nil {
		WriteValidation(c, "invalid "+field)
		return uuid.Nil, false
	}
	return id, true
}

func ParseAuthTenantAndParamID(c *gin.Context, param string, field string) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := ParseAuthTenantID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, ok := ParseUUIDParam(c, param, field)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}

func ParseEntityRef(c *gin.Context, entityParam string, idParam string) (uuid.UUID, string, uuid.UUID, bool) {
	orgID, ok := ParseAuthTenantID(c)
	if !ok {
		return uuid.Nil, "", uuid.Nil, false
	}
	entity := strings.TrimSpace(strings.ToLower(c.Param(entityParam)))
	if entity == "" {
		WriteValidation(c, "invalid entity")
		return uuid.Nil, "", uuid.Nil, false
	}
	id, ok := ParseUUIDParam(c, idParam, idParam)
	if !ok {
		return uuid.Nil, "", uuid.Nil, false
	}
	return orgID, entity, id, true
}

func WriteValidation(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION", "message": message})
}
