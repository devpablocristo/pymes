package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/audit/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/utils"
)

type mockAuditRepo struct {
	entries []domain.Entry
}

func (m *mockAuditRepo) Add(in domain.LogInput) domain.Entry {
	prevHash := ""
	if len(m.entries) > 0 {
		prevHash = m.entries[len(m.entries)-1].Hash
	}
	canonical, _ := utils.CanonicalJSON(in.Payload)
	hash := utils.SHA256Hex(prevHash + string(canonical))

	entry := domain.Entry{
		ID:           uuid.New(),
		OrgID:        in.OrgID,
		Actor:        in.Actor.Legacy,
		ActorType:    in.Actor.Type,
		ActorID:      in.Actor.ID,
		ActorLabel:   in.Actor.Label,
		Action:       in.Action,
		ResourceType: in.ResourceType,
		ResourceID:   in.ResourceID,
		Payload:      in.Payload,
		PrevHash:     prevHash,
		Hash:         hash,
	}
	m.entries = append(m.entries, entry)
	return entry
}

func (m *mockAuditRepo) List(orgID uuid.UUID, limit int) []domain.Entry {
	var result []domain.Entry
	for _, e := range m.entries {
		if e.OrgID == orgID {
			result = append(result, e)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func (m *mockAuditRepo) ExportCSV(orgID uuid.UUID) (string, error) {
	return "id,action\n", nil
}

func TestAuditLog(t *testing.T) {
	repo := &mockAuditRepo{}
	uc := NewUsecases(repo)
	ctx := context.Background()
	orgID := uuid.New().String()

	uc.Log(ctx, orgID, "admin", "create", "user", "u1", map[string]any{"email": "test@test.com"})

	if len(repo.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(repo.entries))
	}

	entry := repo.entries[0]
	if entry.Action != "create" {
		t.Errorf("Action = %q; want %q", entry.Action, "create")
	}
	if entry.Hash == "" {
		t.Error("Hash should not be empty")
	}
	if entry.PrevHash != "" {
		t.Error("first entry PrevHash should be empty")
	}
}

func TestAuditLog_InvalidOrgID(t *testing.T) {
	repo := &mockAuditRepo{}
	uc := NewUsecases(repo)

	uc.Log(context.Background(), "bad-uuid", "actor", "action", "type", "id", nil)
	if len(repo.entries) != 0 {
		t.Error("should not log with invalid org_id")
	}
}

func TestAuditList(t *testing.T) {
	repo := &mockAuditRepo{}
	uc := NewUsecases(repo)
	ctx := context.Background()
	orgID := uuid.New().String()

	uc.Log(ctx, orgID, "admin", "create", "user", "u1", nil)
	uc.Log(ctx, orgID, "admin", "update", "user", "u1", nil)

	entries, err := uc.List(ctx, orgID, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestAuditList_InvalidOrgID(t *testing.T) {
	repo := &mockAuditRepo{}
	uc := NewUsecases(repo)

	_, err := uc.List(context.Background(), "bad", 10)
	if err == nil {
		t.Error("expected error for invalid org_id")
	}
}

func TestAuditExport_CSV(t *testing.T) {
	repo := &mockAuditRepo{}
	uc := NewUsecases(repo)
	orgID := uuid.New().String()

	format, content, err := uc.Export(context.Background(), orgID, "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "csv" {
		t.Errorf("format = %q; want %q", format, "csv")
	}
	if content == "" {
		t.Error("content should not be empty")
	}
}

func TestAuditExport_JSONL(t *testing.T) {
	repo := &mockAuditRepo{}
	uc := NewUsecases(repo)
	ctx := context.Background()
	orgID := uuid.New().String()

	uc.Log(ctx, orgID, "admin", "create", "user", "u1", map[string]any{"k": "v"})

	format, content, err := uc.Export(ctx, orgID, "jsonl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "jsonl" {
		t.Errorf("format = %q; want %q", format, "jsonl")
	}
	if !strings.Contains(content, "create") {
		t.Error("JSONL content should contain the action")
	}
}

func TestAuditExport_UnsupportedFormat(t *testing.T) {
	repo := &mockAuditRepo{}
	uc := NewUsecases(repo)
	orgID := uuid.New().String()

	_, _, err := uc.Export(context.Background(), orgID, "xml")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestAuditHashChain(t *testing.T) {
	repo := &mockAuditRepo{}
	uc := NewUsecases(repo)
	ctx := context.Background()
	orgID := uuid.New().String()

	uc.Log(ctx, orgID, "admin", "first", "user", "u1", nil)
	uc.Log(ctx, orgID, "admin", "second", "user", "u2", nil)
	uc.Log(ctx, orgID, "admin", "third", "user", "u3", nil)

	for i := 1; i < len(repo.entries); i++ {
		if repo.entries[i].PrevHash != repo.entries[i-1].Hash {
			t.Errorf("entry %d PrevHash doesn't match entry %d Hash", i, i-1)
		}
	}
}

func TestAuditLogWithActor_Service(t *testing.T) {
	repo := &mockAuditRepo{}
	uc := NewUsecases(repo)
	orgID := uuid.New()
	serviceID := uuid.New()

	uc.LogWithActor(context.Background(), domain.LogInput{
		OrgID: orgID,
		Actor: domain.ActorRef{
			Legacy: "mercadopago_webhook",
			Type:   "service",
			ID:     &serviceID,
			Label:  "Mercado Pago webhook",
		},
		Action:       "payment_gateway.payment.approved",
		ResourceType: "sale",
		ResourceID:   "sale-1",
		Payload:      map[string]any{"provider": "mercadopago"},
	})

	if len(repo.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(repo.entries))
	}
	entry := repo.entries[0]
	if entry.ActorType != "service" {
		t.Fatalf("ActorType = %q, want service", entry.ActorType)
	}
	if entry.ActorID == nil || *entry.ActorID != serviceID {
		t.Fatalf("ActorID = %v, want %s", entry.ActorID, serviceID)
	}
	if entry.ActorLabel != "Mercado Pago webhook" {
		t.Fatalf("ActorLabel = %q, want %q", entry.ActorLabel, "Mercado Pago webhook")
	}
}
