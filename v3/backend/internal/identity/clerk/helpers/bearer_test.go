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

func TestSessionTokenAcceptsSameOriginClerkCookie(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://app.test/callback", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: "__session", Value: "session.jwt"})
	token, err := SessionToken(request)
	if err != nil {
		t.Fatal(err)
	}
	if token != "session.jwt" {
		t.Fatalf("token=%q", token)
	}
}

func TestSessionTokenRejectsAmbiguousOrMalformedTransports(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(*http.Request)
	}{
		{
			name: "duplicate cookie",
			setup: func(request *http.Request) {
				request.AddCookie(&http.Cookie{Name: "__session", Value: "one"})
				request.AddCookie(&http.Cookie{Name: "__session", Value: "two"})
			},
		},
		{
			name: "malformed authorization does not fall back",
			setup: func(request *http.Request) {
				request.Header.Set("Authorization", "Basic invalid")
				request.AddCookie(&http.Cookie{Name: "__session", Value: "valid"})
			},
		},
		{
			name: "blank cookie",
			setup: func(request *http.Request) {
				request.AddCookie(&http.Cookie{Name: "__session", Value: " "})
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request, err := http.NewRequest(
				http.MethodGet,
				"https://app.test/callback",
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}
			test.setup(request)
			if _, err := SessionToken(request); err == nil {
				t.Fatal("expected session transport rejection")
			}
		})
	}
}
