package fakeservice

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, target any, correlationID string) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeProblem(w, http.StatusBadRequest, correlationID, "VALIDATION_ERROR", "invalid JSON body", err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, status int, correlationID, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":           code,
		"title":          title,
		"detail":         detail,
		"correlation_id": correlationID,
	})
}

func stableUUID(parts ...string) uuid.UUID {
	name := ""
	for _, part := range parts {
		name += "\x00" + part
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("pymes-v3-fake"+name))
}
