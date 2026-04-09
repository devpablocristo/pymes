package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func validConfig(server *httptest.Server) Config {
	return Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8100/callback",
		Scopes:       []string{ScopeCalendarReadonly},
		AuthURL:      server.URL + "/auth",
		TokenURL:     server.URL + "/token",
		HTTPClient:   server.Client(),
	}
}

func TestConfig_ValidateRequiresFields(t *testing.T) {
	t.Parallel()
	cases := map[string]Config{
		"missing client id":     {ClientSecret: "s", RedirectURL: "u"},
		"missing client secret": {ClientID: "c", RedirectURL: "u"},
		"missing redirect url":  {ClientID: "c", ClientSecret: "s"},
	}
	for name, cfg := range cases {
		name, cfg := name, cfg
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestBuildAuthURL_IncludesRequiredParams(t *testing.T) {
	t.Parallel()
	cfg := Config{
		ClientID:     "client-123",
		ClientSecret: "secret-xyz",
		RedirectURL:  "http://localhost:8100/v1/calendar-sync/google/callback",
		Scopes:       []string{ScopeCalendar},
		AuthURL:      "https://example.test/auth",
	}
	out, err := BuildAuthURL(cfg, "csrf-state-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := url.Parse(out)
	if err != nil {
		t.Fatalf("output is not a valid URL: %v", err)
	}
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != "https://example.test/auth" {
		t.Errorf("unexpected base URL: %s", parsed.String())
	}
	q := parsed.Query()
	wantParams := map[string]string{
		"client_id":              "client-123",
		"redirect_uri":           "http://localhost:8100/v1/calendar-sync/google/callback",
		"response_type":          "code",
		"scope":                  ScopeCalendar,
		"access_type":            "offline",
		"prompt":                 "consent",
		"include_granted_scopes": "true",
		"state":                  "csrf-state-abc",
	}
	for k, want := range wantParams {
		if got := q.Get(k); got != want {
			t.Errorf("param %s: want %q, got %q", k, want, got)
		}
	}
}

func TestBuildAuthURL_RejectsEmptyState(t *testing.T) {
	t.Parallel()
	cfg := Config{ClientID: "c", ClientSecret: "s", RedirectURL: "u"}
	if _, err := BuildAuthURL(cfg, ""); err == nil {
		t.Fatal("expected error for empty state")
	}
}

func TestExchangeCode_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.PostForm.Get("grant_type") != "authorization_code" {
			t.Errorf("unexpected grant_type: %s", r.PostForm.Get("grant_type"))
		}
		if r.PostForm.Get("code") != "auth-code-xyz" {
			t.Errorf("unexpected code: %s", r.PostForm.Get("code"))
		}
		if r.PostForm.Get("client_id") != "test-client" {
			t.Errorf("unexpected client_id: %s", r.PostForm.Get("client_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "ya29.access",
			"refresh_token": "1//refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"scope":         ScopeCalendarReadonly,
		})
	}))
	defer server.Close()

	cfg := validConfig(server)
	tok, err := ExchangeCode(context.Background(), cfg, "auth-code-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.AccessToken != "ya29.access" {
		t.Errorf("access token: %q", tok.AccessToken)
	}
	if tok.RefreshToken != "1//refresh" {
		t.Errorf("refresh token: %q", tok.RefreshToken)
	}
	if tok.ExpiresIn != 3600 {
		t.Errorf("expires_in: %d", tok.ExpiresIn)
	}
	if tok.FetchedAt.IsZero() {
		t.Error("FetchedAt should be set")
	}
	if tok.ExpiresAt().IsZero() || time.Until(tok.ExpiresAt()) < 50*time.Minute {
		t.Errorf("ExpiresAt looks wrong: %v", tok.ExpiresAt())
	}
}

func TestExchangeCode_GoogleErrorResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "Bad Request",
		})
	}))
	defer server.Close()

	cfg := validConfig(server)
	_, err := ExchangeCode(context.Background(), cfg, "bad-code")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error should mention google error code, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Bad Request") {
		t.Errorf("error should include description, got: %v", err)
	}
}

func TestExchangeCode_RejectsEmptyCode(t *testing.T) {
	t.Parallel()
	cfg := Config{ClientID: "c", ClientSecret: "s", RedirectURL: "u"}
	if _, err := ExchangeCode(context.Background(), cfg, ""); err == nil {
		t.Fatal("expected error for empty code")
	}
}

func TestExchangeCode_RejectsEmptyAccessTokenResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"refresh_token": "1//refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	cfg := validConfig(server)
	_, err := ExchangeCode(context.Background(), cfg, "auth-code")
	if err == nil {
		t.Fatal("expected error for empty access_token")
	}
}

func TestRefresh_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.PostForm.Get("grant_type") != "refresh_token" {
			t.Errorf("unexpected grant_type: %s", r.PostForm.Get("grant_type"))
		}
		if r.PostForm.Get("refresh_token") != "1//refresh" {
			t.Errorf("unexpected refresh_token: %s", r.PostForm.Get("refresh_token"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "ya29.refreshed",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	cfg := validConfig(server)
	tok, err := Refresh(context.Background(), cfg, "1//refresh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.AccessToken != "ya29.refreshed" {
		t.Errorf("access token: %q", tok.AccessToken)
	}
	// Refresh response típicamente NO incluye refresh_token nuevo.
	if tok.RefreshToken != "" {
		t.Logf("refresh response unexpectedly included refresh_token: %q", tok.RefreshToken)
	}
}

func TestRefresh_RejectsEmptyToken(t *testing.T) {
	t.Parallel()
	cfg := Config{ClientID: "c", ClientSecret: "s", RedirectURL: "u"}
	if _, err := Refresh(context.Background(), cfg, ""); err == nil {
		t.Fatal("expected error for empty refresh_token")
	}
}
