package domain

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeValidation             ErrorCode = "VALIDATION_ERROR"
	CodeNotFound               ErrorCode = "NOT_FOUND"
	CodeSlotConflict           ErrorCode = "SLOT_CONFLICT"
	CodeResourceConflict       ErrorCode = "RESOURCE_CONFLICT"
	CodeCapacityExceeded       ErrorCode = "CAPACITY_EXCEEDED"
	CodeBookingVersionConflict ErrorCode = "BOOKING_VERSION_CONFLICT"
	CodeHoldExpired            ErrorCode = "HOLD_EXPIRED"
	CodeActionTokenInvalid     ErrorCode = "ACTION_TOKEN_INVALID"
	CodeActionTokenExpired     ErrorCode = "ACTION_TOKEN_EXPIRED"
	CodeBookingStateInvalid    ErrorCode = "BOOKING_STATE_INVALID"
	CodeIdempotencyKeyReused   ErrorCode = "IDEMPOTENCY_KEY_REUSED"
	CodeForbidden              ErrorCode = "FORBIDDEN"
	CodeFeatureDisabled        ErrorCode = "FEATURE_DISABLED"
)

type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error { return e.Cause }

func NewError(code ErrorCode, message string) error {
	return &Error{Code: code, Message: message}
}

func WrapError(code ErrorCode, message string, cause error) error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func ErrorCodeOf(err error) ErrorCode {
	var target *Error
	if errors.As(err, &target) {
		return target.Code
	}
	return ""
}
