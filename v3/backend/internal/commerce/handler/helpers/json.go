// Package helpers contains HTTP-only codecs for the commerce handler adapter.
package helpers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxRequestBody = 1 << 20

// WriteJSON emits the stable JSON response envelope used by public handlers.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// DecodeJSON accepts exactly one bounded JSON value.
func DecodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBody))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}
		return err
	}
	return nil
}
