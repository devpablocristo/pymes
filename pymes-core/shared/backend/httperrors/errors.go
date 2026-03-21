package httperrors

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/core/backend/go/apperror"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrForbidden = errors.New("forbidden")
	ErrBadInput  = errors.New("bad input")
	ErrNotDraft  = errors.New("resource is not in draft status")
)

// IsUniqueViolation detecta errores de constraint UNIQUE de PostgreSQL
// sin depender de string matching fragil sobre err.Error().
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "23505") ||
		strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate")
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Meta    any    `json:"meta,omitempty"`
}

func Write(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{Code: code, Message: message})
}

func Respond(c *gin.Context, err error) {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus, ErrorResponse{
			Code:    appErr.Code,
			Message: appErr.Message,
			Meta:    appErr.Meta,
		})
		return
	}
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Code: "not_found", Message: "resource not found"})
	case errors.Is(err, ErrConflict):
		c.JSON(http.StatusConflict, ErrorResponse{Code: "conflict", Message: "resource conflict"})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, ErrorResponse{Code: "forbidden", Message: "access denied"})
	case errors.Is(err, ErrNotDraft):
		c.JSON(http.StatusConflict, ErrorResponse{Code: "not_draft", Message: "resource is not in draft status"})
	default:
		slog.Error("unhandled error in http response", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Code: "internal_error", Message: "unexpected error"})
	}
}
