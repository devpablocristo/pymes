package helpers

import (
	"encoding/json"
	"net/http"

	handlerdto "github.com/devpablocristo/pymes/v3/backend/internal/identity/handler/dto"
)

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteProblem(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, handlerdto.Problem{Code: code, Message: message})
}
