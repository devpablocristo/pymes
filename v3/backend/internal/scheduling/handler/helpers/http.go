package helpers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

func Decode(w http.ResponseWriter, request *http.Request, value any) bool {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		WriteProblem(w, http.StatusBadRequest, domain.CodeValidation, "request body is invalid")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		WriteProblem(w, http.StatusBadRequest, domain.CodeValidation, "request body must contain one JSON value")
		return false
	}
	return true
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteProblem(w http.ResponseWriter, status int, code domain.ErrorCode, message string) {
	WriteJSON(w, status, Problem{Code: string(code), Message: message})
}

func WriteError(w http.ResponseWriter, err error) {
	var schedulingError *domain.Error
	if !errors.As(err, &schedulingError) {
		WriteProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", "scheduling operation failed")
		return
	}
	status := http.StatusBadRequest
	switch schedulingError.Code {
	case domain.CodeNotFound:
		status = http.StatusNotFound
	case domain.CodeForbidden:
		status = http.StatusForbidden
	case domain.CodeFeatureDisabled:
		status = http.StatusForbidden
	case domain.CodeSlotConflict, domain.CodeResourceConflict, domain.CodeCapacityExceeded,
		domain.CodeBookingVersionConflict, domain.CodeIdempotencyKeyReused,
		domain.CodeBookingStateInvalid, domain.CodeHoldExpired:
		status = http.StatusConflict
	case domain.CodeActionTokenInvalid:
		status = http.StatusUnauthorized
	case domain.CodeActionTokenExpired:
		status = http.StatusGone
	}
	WriteProblem(w, status, schedulingError.Code, schedulingError.Message)
}

func ParseUUID(w http.ResponseWriter, value string) (uuid.UUID, bool) {
	result, err := uuid.Parse(value)
	if err != nil {
		WriteProblem(w, http.StatusBadRequest, domain.CodeValidation, "UUID is invalid")
		return uuid.Nil, false
	}
	return result, true
}

func ParseTime(w http.ResponseWriter, value string) (time.Time, bool) {
	result, err := time.Parse(time.RFC3339, value)
	if err != nil {
		WriteProblem(w, http.StatusBadRequest, domain.CodeValidation, "RFC3339 time is invalid")
		return time.Time{}, false
	}
	return result.UTC(), true
}
