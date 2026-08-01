package helpers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	handlerdto "github.com/devpablocristo/pymes/v3/backend/internal/organization/handler/dto"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
)

func Decode(request *http.Request, value any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request must contain exactly one JSON value")
	}
	return nil
}

func WriteJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func WriteError(writer http.ResponseWriter, status int, code string) {
	WriteJSON(writer, status, handlerdto.Error{Code: code})
}

func WriteDomainError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrFeatureVersionConflict):
		WriteError(writer, http.StatusConflict, "FEATURE_VERSION_CONFLICT")
	case errors.Is(err, domain.ErrUnknown):
		WriteError(writer, http.StatusNotFound, "NOT_FOUND")
	default:
		WriteError(writer, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	}
}
