package helpers

import (
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
