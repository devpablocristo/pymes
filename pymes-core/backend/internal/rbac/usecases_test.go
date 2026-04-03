package rbac

import (
	"context"
	"testing"

	"github.com/google/uuid"

	rbacdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/rbac/usecases/domain"
)

type fakeRepo struct {
	perms map[string]map[string]bool
}

func (f *fakeRepo) ListRoles(_ context.Context, _ uuid.UUID) ([]rbacdomain.Role, error) {
	return nil, nil
}
func (f *fakeRepo) GetRole(_ context.Context, _, _ uuid.UUID) (rbacdomain.Role, error) {
	return rbacdomain.Role{}, nil
}
func (f *fakeRepo) CreateRole(_ context.Context, in rbacdomain.Role) (rbacdomain.Role, error) {
	return in, nil
}
func (f *fakeRepo) UpdateRole(_ context.Context, in rbacdomain.Role) (rbacdomain.Role, error) {
	return in, nil
}
func (f *fakeRepo) DeleteRole(_ context.Context, _, _ uuid.UUID) error { return nil }
func (f *fakeRepo) AssignRole(_ context.Context, _, _, _ uuid.UUID, _ string) error {
	return nil
}
func (f *fakeRepo) RemoveRole(_ context.Context, _, _, _ uuid.UUID) error { return nil }
func (f *fakeRepo) IsSystemRole(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}
func (f *fakeRepo) GetUserPermissions(_ context.Context, _, _ uuid.UUID) (map[string]map[string]bool, error) {
	return f.perms, nil
}
func (f *fakeRepo) GetActorPermissions(_ context.Context, _ uuid.UUID, _ string) (map[string]map[string]bool, error) {
	return f.perms, nil
}

func TestUsecases_HasPermission(t *testing.T) {
	orgID := uuid.New().String()
	uc := NewUsecases(&fakeRepo{perms: map[string]map[string]bool{"sales": {"create": true}}}, nil)

	tests := []struct {
		name       string
		role       string
		scopes     []string
		authMethod string
		resource   string
		action     string
		want       bool
	}{
		{name: "jwt admin allows all", role: "admin", resource: "reports", action: "read", want: true},
		{name: "jwt owner allows all", role: "owner", resource: "reports", action: "read", want: true},
		{name: "jwt secops allows all", role: "secops", resource: "reports", action: "read", want: true},
		{name: "api key scoped allow", authMethod: "api_key", scopes: []string{"sales:create"}, resource: "sales", action: "create", want: true},
		{name: "api key scoped deny", authMethod: "api_key", scopes: []string{"sales:read"}, resource: "sales", action: "create", want: false},
		{name: "actor permission allow", authMethod: "jwt", resource: "sales", action: "create", want: true},
		{name: "member default read allow", authMethod: "jwt", resource: "products", action: "read", want: true},
		{name: "member default write deny", authMethod: "jwt", resource: "products", action: "update", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uc.HasPermission(context.Background(), orgID, "actor-1", tt.role, tt.scopes, tt.authMethod, tt.resource, tt.action)
			if got != tt.want {
				t.Fatalf("HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}
