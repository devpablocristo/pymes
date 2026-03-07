package httperrors

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/pkgs/go-pkg/apperror"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrForbidden = errors.New("forbidden")
	ErrBadInput  = errors.New("bad input")
	ErrNotDraft  = errors.New("resource is not in draft status")
)

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
		c.JSON(http.StatusNotFound, ErrorResponse{Code: "not_found", Message: err.Error()})
	case errors.Is(err, ErrConflict):
		c.JSON(http.StatusConflict, ErrorResponse{Code: "conflict", Message: err.Error()})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, ErrorResponse{Code: "forbidden", Message: err.Error()})
	case errors.Is(err, ErrNotDraft):
		c.JSON(http.StatusConflict, ErrorResponse{Code: "not_draft", Message: err.Error()})
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "bad_input", Message: err.Error()})
	}
}
