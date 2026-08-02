package helpers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONRejectsTrailingValues(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"ok":true} {}`))
	var value map[string]bool
	if err := DecodeJSON(request, &value); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}

func TestWriteErrorUsesPublicErrorContract(t *testing.T) {
	response := httptest.NewRecorder()
	WriteError(response, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q", contentType)
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("body = %v", body)
	}
}
