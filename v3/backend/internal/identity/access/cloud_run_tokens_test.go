package access

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetadataIDTokenSourceCachesAudienceToken(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Metadata-Flavor") != "Google" || r.URL.Query().Get("audience") != "https://private.example" {
			t.Fatalf("metadata request headers=%v query=%v", r.Header, r.URL.Query())
		}
		payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, now.Add(time.Hour).Unix())))
		_, _ = fmt.Fprintf(w, "header.%s.signature", payload)
	}))
	defer server.Close()

	source := newMetadataIDTokenSource(server.Client(), server.URL, func() time.Time { return now })
	first, err := source.PlatformToken(context.Background(), "https://private.example")
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.PlatformToken(context.Background(), "https://private.example")
	if err != nil || second != first || calls != 1 {
		t.Fatalf("second=%q calls=%d err=%v", second, calls, err)
	}
}

func TestMetadataIDTokenSourceRejectsInvalidResponseWithoutLeakingBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "sensitive metadata body", http.StatusForbidden)
	}))
	defer server.Close()
	source := newMetadataIDTokenSource(server.Client(), server.URL, time.Now)
	if _, err := source.PlatformToken(context.Background(), "https://private.example"); err == nil || err.Error() != "fetch platform identity: metadata returned 403 Forbidden" {
		t.Fatalf("err=%v", err)
	}
}
