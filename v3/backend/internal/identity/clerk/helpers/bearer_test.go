package helpers

import (
	"net/http"
	"testing"
)

func TestBearerTokenRejectsEmptyToken(t *testing.T) {
	header := http.Header{"Authorization": []string{"Bearer "}}
	if _, err := BearerToken(header); err == nil {
		t.Fatal("expected empty bearer token to be rejected")
	}
}
