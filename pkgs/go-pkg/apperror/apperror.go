package apperror

import (
	"fmt"
	"net/http"
	"strings"
)

type Error struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	HTTPStatus int            `json:"-"`
	Meta       map[string]any `json:"meta,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) WithMeta(meta map[string]any) *Error {
	if e == nil {
		return nil
	}
	clone := *e
	if len(meta) > 0 {
		clone.Meta = meta
	}
	return &clone
}

func New(code, message string, status int) *Error {
	return &Error{
		Code:       normalizeCode(code, status),
		Message:    strings.TrimSpace(message),
		HTTPStatus: status,
	}
}

func NewBadInput(message string) *Error {
	return New("bad_input", message, http.StatusBadRequest)
}

func NewForbidden(message string) *Error {
	return New("forbidden", message, http.StatusForbidden)
}

func NewConflict(message string) *Error {
	return New("conflict", message, http.StatusConflict)
}

func NewBusinessRule(message string) *Error {
	return New("business_rule_violation", message, http.StatusUnprocessableEntity)
}

func NewNotFound(resource, id string) *Error {
	resource = strings.TrimSpace(resource)
	id = strings.TrimSpace(id)
	if resource == "" {
		resource = "resource"
	}
	message := fmt.Sprintf("%s not found", resource)
	if id != "" {
		message = fmt.Sprintf("%s '%s' not found", resource, id)
	}
	return New("not_found", message, http.StatusNotFound).WithMeta(map[string]any{
		"resource": resource,
		"id":       id,
	})
}

func normalizeCode(code string, status int) string {
	if trimmed := strings.TrimSpace(code); trimmed != "" {
		return trimmed
	}
	switch status {
	case http.StatusBadRequest:
		return "bad_input"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusConflict:
		return "conflict"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusUnprocessableEntity:
		return "business_rule_violation"
	default:
		return "error"
	}
}
