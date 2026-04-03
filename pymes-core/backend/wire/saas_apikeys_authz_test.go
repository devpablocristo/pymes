package wire

import (
	"testing"

	kerneldomain "github.com/devpablocristo/core/saas/go/kernel/usecases/domain"
)

func TestAPIKeyManagementAllowed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		authMethod string
		role       string
		scopes     []string
		want       bool
	}{
		{"viewer sin scopes", "jwt", "viewer", nil, false},
		{"owner", "jwt", "owner", nil, true},
		{"admin", "jwt", "admin", nil, true},
		{"service api key", "api_key", "service", []string{"admin:console:write"}, false},
		{"viewer con scope lectura consola", "jwt", "viewer", []string{"admin:console:read"}, false},
		{"viewer con scope escritura consola", "jwt", "viewer", []string{"admin:console:write"}, true},
		{"viewer sin scope consola", "jwt", "viewer", []string{"other:scope"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := kerneldomain.Principal{TenantID: "org-1", Role: tc.role, Scopes: tc.scopes, AuthMethod: tc.authMethod}
			if got := apiKeyManagementAllowed(p); got != tc.want {
				t.Fatalf("apiKeyManagementAllowed(%q, %v, %q) = %v, want %v", tc.role, tc.scopes, tc.authMethod, got, tc.want)
			}
		})
	}
}
