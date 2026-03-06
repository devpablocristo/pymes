package rbac

import (
	"context"
	"testing"

	"github.com/google/uuid"

	rbacdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/rbac/usecases/domain"
)

type fakeRepo struct {
	perms map[string]map[string]bool
}

func (f *fakeRepo) ListRoles(orgID uuid.UUID) ([]rbacdomain.Role, error) { return nil, nil }
func (f *fakeRepo) GetRole(orgID, roleID uuid.UUID) (rbacdomain.Role, error) {
	return rbacdomain.Role{}, nil
}
func (f *fakeRepo) CreateRole(in rbacdomain.Role) (rbacdomain.Role, error) { return in, nil }
func (f *fakeRepo) UpdateRole(in rbacdomain.Role) (rbacdomain.Role, error) { return in, nil }
func (f *fakeRepo) DeleteRole(orgID, roleID uuid.UUID) error               { return nil }
func (f *fakeRepo) AssignRole(orgID, roleID, userID uuid.UUID, assignedBy string) error {
	return nil
}
func (f *fakeRepo) RemoveRole(orgID, roleID, userID uuid.UUID) error { return nil }
func (f *fakeRepo) IsSystemRole(orgID, roleID uuid.UUID) (bool, error) {
	return false, nil
}
func (f *fakeRepo) GetUserPermissions(orgID, userID uuid.UUID) (map[string]map[string]bool, error) {
	return f.perms, nil
}
func (f *fakeRepo) GetActorPermissions(orgID uuid.UUID, actor string) (map[string]map[string]bool, error) {
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
