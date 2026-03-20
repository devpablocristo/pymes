package authz

import "testing"

func TestHasScope(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		target string
		want   bool
	}{
		{"found", []string{"read", "write"}, "write", true},
		{"not found", []string{"read", "write"}, "delete", false},
		{"empty scopes", []string{}, "read", false},
		{"nil scopes", nil, "read", false},
		{"exact match only", []string{"admin:console:read"}, "admin:console", false},
		{"single match", []string{"admin:console:write"}, "admin:console:write", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasScope(tt.scopes, tt.target)
			if got != tt.want {
				t.Errorf("HasScope(%v, %q) = %v; want %v", tt.scopes, tt.target, got, tt.want)
			}
		})
	}
}

func TestIsAdmin(t *testing.T) {
	tests := []struct {
		name   string
		role   string
		scopes []string
		want   bool
	}{
		{"admin role", "admin", nil, true},
		{"admin role with scopes", "admin", []string{"read"}, true},
		{"member with write scope", "member", []string{"admin:console:write"}, true},
		{"member with read scope", "member", []string{"admin:console:read"}, true},
		{"member no admin scopes", "member", []string{"read", "write"}, false},
		{"empty role no scopes", "", nil, false},
		{"member with both scopes", "member", []string{"admin:console:read", "admin:console:write"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAdmin(tt.role, tt.scopes)
			if got != tt.want {
				t.Errorf("IsAdmin(%q, %v) = %v; want %v", tt.role, tt.scopes, got, tt.want)
			}
		})
	}
}
