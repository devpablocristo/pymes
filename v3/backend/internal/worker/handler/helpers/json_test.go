package helpers

import (
	"net/http/httptest"
	"testing"
)

func TestWriteJSONSetsContentType(t *testing.T) {
	response := httptest.NewRecorder()
	WriteJSON(response, 200, map[string]string{"status": "ok"})
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("got %q", got)
	}
}
