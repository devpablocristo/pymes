package authz

import "testing"

func TestIsPrivilegedRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role string
		want bool
	}{
		{"admin", true},
		{"Admin", true},
		{"owner", true},
		{"secops", true},
		{"viewer", false},
		{"", false},
		{"user", false},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			t.Parallel()
			if got := IsPrivilegedRole(tt.role); got != tt.want {
				t.Errorf("IsPrivilegedRole(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestProductRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role   string
		scopes []string
		want   string
	}{
		{"owner", nil, "admin"},
		{"admin", nil, "admin"},
		{"secops", nil, "admin"},
		{"viewer", nil, "user"},
		{"service", nil, "user"},
		{"service", []string{"admin:console:read"}, "admin"},
		{"", nil, "user"},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			t.Parallel()
			if got := ProductRole(tt.role, tt.scopes); got != tt.want {
				t.Errorf("ProductRole(%q, %v) = %q, want %q", tt.role, tt.scopes, got, tt.want)
			}
		})
	}
}

func TestIsAdmin(t *testing.T) {
	t.Parallel()
	if IsAdmin("viewer", []string{"admin:console:read"}) {
		t.Fatal("expected viewer with console read scope not to be admin")
	}
	if !IsAdmin("owner", nil) {
		t.Fatal("expected owner to be admin")
	}
	if IsAdmin("service", nil) {
		t.Fatal("expected service without console scopes not to be admin")
	}
	if !IsAdmin("service", []string{"admin:console:write"}) {
		t.Fatal("expected service with console write scope to be admin")
	}
	if IsAdmin("viewer", nil) {
		t.Fatal("expected viewer without scopes not to be admin")
	}
}
