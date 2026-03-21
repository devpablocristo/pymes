package wire

import (
	"testing"

	kerneldomain "github.com/devpablocristo/core/saas/go/kernel/usecases/domain"
)

func TestAPIKeyManagementAllowed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		role   string
		scopes []string
		want   bool
	}{
		{"viewer sin scopes", "viewer", nil, false},
		{"owner", "owner", nil, true},
		{"admin", "admin", nil, true},
		{"service api key", "service", nil, true},
		{"viewer con scope lectura consola", "viewer", []string{"admin:console:read"}, true},
		{"viewer sin scope consola", "viewer", []string{"other:scope"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := kerneldomain.Principal{TenantID: "org-1", Role: tc.role, Scopes: tc.scopes}
			if got := apiKeyManagementAllowed(p); got != tc.want {
				t.Fatalf("apiKeyManagementAllowed(%q, %v) = %v, want %v", tc.role, tc.scopes, got, tc.want)
			}
		})
	}
}
