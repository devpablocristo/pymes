package httpserver

import (
	"testing"

	"github.com/devpablocristo/pymes/v2/backend/internal/api"
)

func TestNormalizeDelegatedPermissionsIsStableAndRejectsDuplicates(t *testing.T) {
	t.Parallel()

	got, err := normalizeDelegatedPermissions([]api.DelegatedBusinessPermission{
		api.DelegatedBusinessPermissionFiscalManage,
		api.DelegatedBusinessPermissionAccountingManage,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 ||
		got[0] != api.DelegatedBusinessPermissionAccountingManage ||
		got[1] != api.DelegatedBusinessPermissionFiscalManage {
		t.Fatalf("permissions = %v", got)
	}

	if _, err := normalizeDelegatedPermissions([]api.DelegatedBusinessPermission{
		api.DelegatedBusinessPermissionAccountingManage,
		api.DelegatedBusinessPermissionAccountingManage,
	}); err == nil {
		t.Fatal("expected duplicated delegated permission to fail")
	}
}

func TestNormalizeDelegatedPermissionsRejectsExpandedPermissionVocabulary(t *testing.T) {
	t.Parallel()

	if _, err := normalizeDelegatedPermissions([]api.DelegatedBusinessPermission{
		"team:member:update",
	}); err == nil {
		t.Fatal("expected non-business permission to fail")
	}
}
