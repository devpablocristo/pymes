package businessinsights

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	coredomain "github.com/devpablocristo/core/notifications/go/inbox/usecases/domain"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications"
	inventorydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/inventory/usecases/domain"
	paymentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/usecases/domain"
	saledomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/sales/usecases/domain"
)

type inAppRepoStub struct {
	userByExternal map[string]uuid.UUID
	onlyUserByOrg  map[uuid.UUID]uuid.UUID
	appended       []coredomain.Notification
}

type candidateRepoStub struct {
	lastUpsert   CandidateUpsert
	shouldNotify bool
	record       CandidateRecord
	marked       []string
}

func (s *inAppRepoStub) GetUserIDByExternalID(externalID string) (uuid.UUID, bool) {
	id, ok := s.userByExternal[externalID]
	return id, ok
}

func (s *inAppRepoStub) GetOnlyUserIDByOrg(orgID uuid.UUID) (uuid.UUID, bool) {
	id, ok := s.onlyUserByOrg[orgID]
	return id, ok
}

func (s *inAppRepoStub) ListUserIDsByOrg(uuid.UUID) ([]uuid.UUID, error) { return nil, nil }
func (s *inAppRepoStub) ListOrgIDsWithUsers() ([]uuid.UUID, error)       { return nil, nil }
func (s *inAppRepoStub) ResolveApprovalNotifications(context.Context, string, string, string, time.Time) (int64, error) {
	return 0, nil
}

func (s *inAppRepoStub) ListForRecipient(context.Context, string, string, int) ([]coredomain.Notification, error) {
	return nil, nil
}

func (s *inAppRepoStub) CountUnread(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (s *inAppRepoStub) Append(_ context.Context, notification coredomain.Notification) (coredomain.Notification, error) {
	if notification.ID == "" {
		notification.ID = uuid.NewString()
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now().UTC()
	}
	s.appended = append(s.appended, notification)
	return notification, nil
}

func (s *inAppRepoStub) MarkRead(context.Context, string, string, string, time.Time) (time.Time, error) {
	return time.Now().UTC(), nil
}

func (s *candidateRepoStub) Upsert(_ context.Context, in CandidateUpsert) (CandidateRecord, bool, error) {
	s.lastUpsert = in
	record := s.record
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	record.TenantID = in.TenantID
	record.Kind = in.Kind
	record.EventType = in.EventType
	record.EntityType = in.EntityType
	record.EntityID = in.EntityID
	record.Fingerprint = in.Fingerprint
	record.Severity = in.Severity
	record.Title = in.Title
	record.Body = in.Body
	record.Evidence = in.Evidence
	return record, s.shouldNotify, nil
}

func (s *candidateRepoStub) MarkNotified(_ context.Context, _ string, candidateID string, _ time.Time) error {
	s.marked = append(s.marked, candidateID)
	return nil
}

func TestNotifySaleCreatedCreatesNotificationForFeaturedSale(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	repo := &inAppRepoStub{
		userByExternal: map[string]uuid.UUID{"seller-1": userID},
		onlyUserByOrg:  map[uuid.UUID]uuid.UUID{orgID: userID},
	}
	candidates := &candidateRepoStub{shouldNotify: true}
	svc := NewService(candidates, inappnotifications.NewUsecases(repo), Config{})

	err := svc.NotifySaleCreated(context.Background(), saledomain.Sale{
		ID:        uuid.New(),
		OrgID:     orgID,
		Number:    "VTA-0012",
		Total:     120000,
		Currency:  "ARS",
		CreatedBy: "seller-1",
		Items:     []saledomain.SaleItem{{}, {}},
	})
	if err != nil {
		t.Fatalf("NotifySaleCreated() error = %v", err)
	}
	if len(repo.appended) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(repo.appended))
	}
	if repo.appended[0].Title != "Venta destacada registrada" {
		t.Fatalf("unexpected title %q", repo.appended[0].Title)
	}
	var meta map[string]any
	if err := json.Unmarshal(repo.appended[0].Metadata, &meta); err != nil {
		t.Fatalf("invalid metadata: %v", err)
	}
	if meta["event_type"] != "sale.created" {
		t.Fatalf("expected sale.created metadata, got %#v", meta["event_type"])
	}
	if meta["candidate_id"] == nil {
		t.Fatalf("expected candidate_id in metadata")
	}
	if len(candidates.marked) != 1 {
		t.Fatalf("expected mark notified once, got %d", len(candidates.marked))
	}
}

func TestNotifyPaymentCreatedSkipsSmallPayments(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	repo := &inAppRepoStub{
		userByExternal: map[string]uuid.UUID{"cashier-1": userID},
		onlyUserByOrg:  map[uuid.UUID]uuid.UUID{orgID: userID},
	}
	svc := NewService(&candidateRepoStub{shouldNotify: true}, inappnotifications.NewUsecases(repo), Config{})

	err := svc.NotifyPaymentCreated(context.Background(), orgID, uuid.New(), paymentsdomain.Payment{
		ID:        uuid.New(),
		Amount:    1200,
		Method:    "cash",
		CreatedBy: "cashier-1",
	})
	if err != nil {
		t.Fatalf("NotifyPaymentCreated() error = %v", err)
	}
	if len(repo.appended) != 0 {
		t.Fatalf("expected no notifications, got %d", len(repo.appended))
	}
}

func TestNotifyInventoryAdjustedCreatesNotificationOnLowStock(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	repo := &inAppRepoStub{
		userByExternal: map[string]uuid.UUID{"stock-user": userID},
		onlyUserByOrg:  map[uuid.UUID]uuid.UUID{orgID: userID},
	}
	candidates := &candidateRepoStub{shouldNotify: true}
	svc := NewService(candidates, inappnotifications.NewUsecases(repo), Config{})

	err := svc.NotifyInventoryAdjusted(context.Background(), inventorydomain.StockLevel{
		ProductID:   uuid.New(),
		OrgID:       orgID,
		ProductName: "Cubierta Maxxis",
		Quantity:    1,
		MinQuantity: 3,
		IsLowStock:  true,
	}, -2, "stock-user", "ajuste manual")
	if err != nil {
		t.Fatalf("NotifyInventoryAdjusted() error = %v", err)
	}
	if len(repo.appended) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(repo.appended))
	}
	if repo.appended[0].Title != "Stock crítico tras ajuste" {
		t.Fatalf("unexpected title %q", repo.appended[0].Title)
	}
	if candidates.lastUpsert.Severity != "warning" {
		t.Fatalf("expected warning severity, got %q", candidates.lastUpsert.Severity)
	}
}

func TestBucketedIDUsesStableWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	first := bucketedID("inventory.low_stock", "prod-1", 6*time.Hour, now)
	second := bucketedID("inventory.low_stock", "prod-1", 6*time.Hour, now.Add(1*time.Hour))
	third := bucketedID("inventory.low_stock", "prod-1", 6*time.Hour, now.Add(7*time.Hour))

	if first != second {
		t.Fatalf("expected same bucket id inside window, got %q vs %q", first, second)
	}
	if first == third {
		t.Fatalf("expected different bucket id after window, got %q", third)
	}
}
