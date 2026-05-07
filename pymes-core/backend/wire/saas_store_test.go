package wire

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResolveTenantIDByExternalRefDoesNotAutoProvisionClerkStyleOrg(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)

	tenantID, ok, err := store.ResolveTenantIDByExternalRef(context.Background(), "org_demo")
	if err != nil {
		t.Fatalf("ResolveTenantIDByExternalRef() error = %v", err)
	}
	if ok {
		t.Fatalf("ResolveTenantIDByExternalRef() ok = true, want false (tenantID=%q)", tenantID)
	}
	if tenantID != "" {
		t.Fatalf("ResolveTenantIDByExternalRef() tenantID = %q, want empty", tenantID)
	}

	var orgCount int64
	if err := db.Model(&pymesTenantRow{}).Where("external_id = ?", "org_demo").Count(&orgCount).Error; err != nil {
		t.Fatalf("count tenants: %v", err)
	}
	if orgCount != 0 {
		t.Fatalf("org count = %d, want 0", orgCount)
	}
}

func TestResolveTenantIDByExternalRefDoesNotAutoProvisionUnknownRef(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)

	tenantID, ok, err := store.ResolveTenantIDByExternalRef(context.Background(), "tenant_demo")
	if err != nil {
		t.Fatalf("ResolveTenantIDByExternalRef() error = %v", err)
	}
	if ok {
		t.Fatalf("ResolveTenantIDByExternalRef() ok = true, want false (tenantID=%q)", tenantID)
	}
	if tenantID != "" {
		t.Fatalf("ResolveTenantIDByExternalRef() tenantID = %q, want empty", tenantID)
	}

	var orgCount int64
	if err := db.Model(&pymesTenantRow{}).Count(&orgCount).Error; err != nil {
		t.Fatalf("count tenants: %v", err)
	}
	if orgCount != 0 {
		t.Fatalf("org count = %d, want 0", orgCount)
	}
}

func TestResolveTenantIDByExternalRefResolvesExistingExternalRef(t *testing.T) {
	db := newTestSaaSStoreDB(t)
	store := newPymesSaaSStore(db, testSaaSStoreLogger(), nil)

	expectedID := uuid.NewString()
	now := time.Now().UTC()
	if err := db.Create(&pymesTenantRow{
		ID:         uuid.MustParse(expectedID),
		ExternalID: stringPtr("org_existing"),
		ClerkOrgID: stringPtr("org_existing"),
		Name:       "Existing Tenant",
		CreatedAt:  now,
		UpdatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	tenantID, ok, err := store.ResolveTenantIDByExternalRef(context.Background(), "org_existing")
	if err != nil {
		t.Fatalf("ResolveTenantIDByExternalRef() error = %v", err)
	}
	if !ok {
		t.Fatal("ResolveTenantIDByExternalRef() ok = false, want true")
	}
	if tenantID != expectedID {
		t.Fatalf("ResolveTenantIDByExternalRef() tenantID = %q, want %q", tenantID, expectedID)
	}
}

func newTestSaaSStoreDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(
		&pymesTenantRow{},
		&pymesUserRow{},
		&pymesTenantMembershipRow{},
		&pymesTenantInvitationRow{},
		&pymesTenantSettingsRow{},
		&pymesTenantAPIKeyRow{},
		&pymesTenantAPIKeyScopeRow{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func testSaaSStoreLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
