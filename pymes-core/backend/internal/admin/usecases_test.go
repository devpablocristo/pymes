package admin

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	admindomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/usecases/domain"
)

type fakeAdminRepo struct {
	settings    admindomain.TenantSettings
	updatePatch admindomain.TenantSettingsPatch
}

func (f *fakeAdminRepo) GetTenantSettings(_ uuid.UUID) admindomain.TenantSettings {
	return f.settings
}

func (f *fakeAdminRepo) UpdateTenantSettings(_ uuid.UUID, patch admindomain.TenantSettingsPatch, _ *string) admindomain.TenantSettings {
	f.updatePatch = patch
	f.settings.SchedulingEnabled = patch.SchedulingEnabled != nil && *patch.SchedulingEnabled
	return f.settings
}

func (f *fakeAdminRepo) ListActivity(_ uuid.UUID, _ int) []admindomain.ActivityEvent {
	return nil
}

func TestUsecasesUpdateTenantSettingsForwardsSchedulingEnabled(t *testing.T) {
	t.Parallel()

	repo := &fakeAdminRepo{}
	uc := NewUsecases(repo)
	value := true

	updated, err := uc.UpdateTenantSettings(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		admindomain.TenantSettingsPatch{
			SchedulingEnabled: &value,
		},
		nil,
	)
	if err != nil {
		t.Fatalf("UpdateTenantSettings() error = %v", err)
	}
	if repo.updatePatch.SchedulingEnabled == nil || !*repo.updatePatch.SchedulingEnabled {
		t.Fatalf("expected scheduling_enabled to reach repository")
	}
	if !updated.SchedulingEnabled {
		t.Fatalf("expected returned settings to have scheduling_enabled=true")
	}
}

func TestUsecasesUpdateTenantSettingsRejectsNegativeReminderHours(t *testing.T) {
	t.Parallel()

	repo := &fakeAdminRepo{}
	uc := NewUsecases(repo)
	value := -1

	_, err := uc.UpdateTenantSettings(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		admindomain.TenantSettingsPatch{
			SchedulingReminderHours: &value,
		},
		nil,
	)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !domainerr.IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
