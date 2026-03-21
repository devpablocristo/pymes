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
		role string
		want string
	}{
		{"owner", "admin"},
		{"admin", "admin"},
		{"secops", "admin"},
		{"viewer", "user"},
		{"service", "admin"},
		{"", "user"},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			t.Parallel()
			if got := ProductRole(tt.role); got != tt.want {
				t.Errorf("ProductRole(%q) = %q, want %q", tt.role, got, tt.want)
			}
		})
	}
}

func TestIsAdmin(t *testing.T) {
	t.Parallel()
	if !IsAdmin("viewer", []string{"admin:console:read"}) {
		t.Fatal("expected viewer with console read scope to be admin")
	}
	if !IsAdmin("owner", nil) {
		t.Fatal("expected owner to be admin")
	}
	if !IsAdmin("service", nil) {
		t.Fatal("expected service (API key) to be admin")
	}
	if IsAdmin("viewer", nil) {
		t.Fatal("expected viewer without scopes not to be admin")
	}
}
