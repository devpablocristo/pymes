package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestClerkCompactOrgIDFromClaims(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		c    jwt.MapClaims
		want string
	}{
		{
			name: "empty",
			c:    jwt.MapClaims{},
			want: "",
		},
		{
			name: "o map string id",
			c: jwt.MapClaims{
				"o": map[string]any{"id": "org_2abc123"},
			},
			want: "org_2abc123",
		},
		{
			name: "o map interface id",
			c: jwt.MapClaims{
				"o": map[string]interface{}{"id": "org_legacy"},
			},
			want: "org_legacy",
		},
		{
			name: "o not a map",
			c: jwt.MapClaims{
				"o": "nope",
			},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := clerkCompactOrgIDFromClaims(tc.c)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
