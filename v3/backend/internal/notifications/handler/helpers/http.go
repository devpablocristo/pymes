// Package helpers contains bounded notification HTTP codecs.
package helpers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var ErrBodyTooLarge = errors.New("request body is too large")

func ReadBody(request *http.Request, limit int64) ([]byte, error) {
	if request == nil || request.Body == nil {
		return nil, io.ErrUnexpectedEOF
	}
	if limit <= 0 {
		limit = 64 << 10
	}
	reader := http.MaxBytesReader(nil, request.Body, limit)
	body, err := io.ReadAll(reader)
	if err != nil {
		var maximum *http.MaxBytesError
		if errors.As(err, &maximum) {
			return nil, ErrBodyTooLarge
		}
		return nil, err
	}
	if len(body) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	return body, nil
}

func WriteJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}
