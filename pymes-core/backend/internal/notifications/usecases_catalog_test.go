package notifications

import (
	"testing"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/notifications/usecases/domain"
)

func TestMergePreferenceCatalog_DefaultEnabledWhenNoRows(t *testing.T) {
	t.Parallel()
	uid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	got := mergePreferenceCatalog(uid, nil)
	if len(got) == 0 {
		t.Fatal("expected catalog entries")
	}
	for _, p := range got {
		if p.UserID != uid {
			t.Errorf("user id: got %v want %v", p.UserID, uid)
		}
		if !p.Enabled {
			t.Errorf("%s/%s: expected default enabled", p.NotificationType, p.Channel)
		}
	}
}

func TestMergePreferenceCatalog_RespectsStored(t *testing.T) {
	t.Parallel()
	uid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	stored := []domain.Preference{
		{
			UserID:           uid,
			NotificationType: "welcome",
			Channel:          "email",
			Enabled:          false,
		},
	}
	got := mergePreferenceCatalog(uid, stored)
	var welcome *domain.Preference
	for i := range got {
		if got[i].NotificationType == "welcome" && got[i].Channel == "email" {
			welcome = &got[i]
			break
		}
	}
	if welcome == nil {
		t.Fatal("welcome/email missing from merged catalog")
	}
	if welcome.Enabled {
		t.Fatal("expected welcome disabled from stored row")
	}
}
