package wire

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFindOrgIDByExternalIDAutoProvisionsClerkStyleOrg(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)

	orgID, ok, err := store.FindOrgIDByExternalID(context.Background(), "org_demo")
	if err != nil {
		t.Fatalf("FindOrgIDByExternalID() error = %v", err)
	}
	if !ok {
		t.Fatal("FindOrgIDByExternalID() ok = false, want true")
	}
	if _, err := uuid.Parse(orgID); err != nil {
		t.Fatalf("FindOrgIDByExternalID() returned invalid UUID %q: %v", orgID, err)
	}

	var orgCount int64
	if err := db.Model(&pymesOrgRow{}).Where("external_id = ?", "org_demo").Count(&orgCount).Error; err != nil {
		t.Fatalf("count orgs: %v", err)
	}
	if orgCount != 1 {
		t.Fatalf("org count = %d, want 1", orgCount)
	}

	var settingsCount int64
	if err := db.Model(&pymesTenantSettingsRow{}).Where("org_id = ?", orgID).Count(&settingsCount).Error; err != nil {
		t.Fatalf("count tenant_settings: %v", err)
	}
	if settingsCount != 1 {
		t.Fatalf("tenant_settings count = %d, want 1", settingsCount)
	}
}

func TestFindOrgIDByExternalIDDoesNotAutoProvisionUnknownRef(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)

	orgID, ok, err := store.FindOrgIDByExternalID(context.Background(), "tenant_demo")
	if err != nil {
		t.Fatalf("FindOrgIDByExternalID() error = %v", err)
	}
	if ok {
		t.Fatalf("FindOrgIDByExternalID() ok = true, want false (orgID=%q)", orgID)
	}
	if orgID != "" {
		t.Fatalf("FindOrgIDByExternalID() orgID = %q, want empty", orgID)
	}

	var orgCount int64
	if err := db.Model(&pymesOrgRow{}).Count(&orgCount).Error; err != nil {
		t.Fatalf("count orgs: %v", err)
	}
	if orgCount != 0 {
		t.Fatalf("org count = %d, want 0", orgCount)
	}
}

func TestFindOrgIDByExternalIDResolvesExistingExternalRef(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)

	expectedID, err := store.UpsertOrg(context.Background(), "org_existing", "Existing Org")
	if err != nil {
		t.Fatalf("UpsertOrg() error = %v", err)
	}

	orgID, ok, err := store.FindOrgIDByExternalID(context.Background(), "org_existing")
	if err != nil {
		t.Fatalf("FindOrgIDByExternalID() error = %v", err)
	}
	if !ok {
		t.Fatal("FindOrgIDByExternalID() ok = false, want true")
	}
	if orgID != expectedID {
		t.Fatalf("FindOrgIDByExternalID() orgID = %q, want %q", orgID, expectedID)
	}
}

func newTestSaaSStoreDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&pymesOrgRow{}, &pymesTenantSettingsRow{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func testSaaSStoreLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
