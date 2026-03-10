package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ParseAuthOrgID(c *gin.Context) (uuid.UUID, bool) {
	auth := GetAuthContext(c)
	orgID, err := uuid.Parse(strings.TrimSpace(auth.OrgID))
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

func ParseEntityRef(c *gin.Context, entityParam string, idParam string) (uuid.UUID, string, uuid.UUID, bool) {
	orgID, ok := ParseAuthOrgID(c)
	if !ok {
		return uuid.Nil, "", uuid.Nil, false
	}
	entity := strings.TrimSpace(strings.ToLower(c.Param(entityParam)))
	if entity == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity"})
		return uuid.Nil, "", uuid.Nil, false
	}
	id, ok := ParseUUIDParam(c, idParam, idParam)
	if !ok {
		return uuid.Nil, "", uuid.Nil, false
	}
	return orgID, entity, id, true
}
