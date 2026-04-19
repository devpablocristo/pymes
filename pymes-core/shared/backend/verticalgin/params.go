// Package verticalgin: helpers Gin compartidos por verticales (auth desde shared/backend/auth).
package verticalgin

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
)

func ParseAuthOrgID(c *gin.Context) (uuid.UUID, bool) {
	return auth.ParseAuthOrgID(c)
}

func ParseUUIDParam(c *gin.Context, param string, field string) (uuid.UUID, bool) {
	return ginmw.ParseUUIDParam(c, param)
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
	return ginmw.ParseRFC3339(raw)
}

func ParseOptionalRFC3339(raw string) (*time.Time, error) {
	return ginmw.ParseOptionalRFC3339(raw)
}

func ParseOptionalRFC3339Ptr(raw *string) (*time.Time, error) {
	return ginmw.ParseOptionalRFC3339Ptr(raw)
}

func ParseNullableRFC3339Ptr(raw *string) (**time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	t, err := ginmw.ParseOptionalRFC3339(*raw)
	if err != nil {
		return nil, err
	}
	if t == nil {
		var value *time.Time
		return &value, nil
	}
	return &t, nil
}
