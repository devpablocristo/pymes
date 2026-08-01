package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	handlerdto "github.com/devpablocristo/pymes/v3/backend/internal/calendars/handler/dto"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
)

func DecodeJSON(request *http.Request, value any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("multiple JSON values are forbidden")
	}
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	var body bytes.Buffer
	if json.NewEncoder(&body).Encode(value) != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}

func WriteError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(handlerdto.ErrorResponse{Code: code})
}

func WriteDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrOAuthStateInvalid),
		errors.Is(err, domain.ErrOAuthStateConsumed):
		WriteError(w, http.StatusBadRequest, "OAUTH_STATE_INVALID")
	case errors.Is(err, domain.ErrOAuthStateExpired):
		WriteError(w, http.StatusGone, "OAUTH_STATE_EXPIRED")
	case errors.Is(err, domain.ErrNotFound):
		WriteError(w, http.StatusNotFound, "NOT_FOUND")
	case errors.Is(err, domain.ErrReauthRequired):
		WriteError(w, http.StatusUnauthorized, "CALENDAR_REAUTH_REQUIRED")
	case errors.Is(err, domain.ErrProviderUnavailable),
		errors.Is(err, domain.ErrUncertain):
		WriteError(w, http.StatusServiceUnavailable, "CALENDAR_PROVIDER_UNAVAILABLE")
	default:
		WriteError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	}
}
