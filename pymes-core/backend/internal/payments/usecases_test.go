package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	paymentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/usecases/domain"
)

type fakePaymentsRepo struct {
	out paymentsdomain.Payment
	err error
}

func (f *fakePaymentsRepo) ListSalePayments(context.Context, uuid.UUID, uuid.UUID) ([]paymentsdomain.Payment, error) {
	return nil, nil
}

func (f *fakePaymentsRepo) ListArchived(context.Context, uuid.UUID, int) ([]paymentsdomain.Payment, error) {
	return nil, nil
}

func (f *fakePaymentsRepo) CreateSalePayment(context.Context, uuid.UUID, uuid.UUID, paymentsdomain.Payment) (paymentsdomain.Payment, error) {
	if f.err != nil {
		return paymentsdomain.Payment{}, f.err
	}
	return f.out, nil
}

func (f *fakePaymentsRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (paymentsdomain.Payment, error) {
	return f.out, f.err
}

func (f *fakePaymentsRepo) Update(context.Context, paymentsdomain.Payment) (paymentsdomain.Payment, error) {
	return f.out, f.err
}

func (f *fakePaymentsRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return f.err
}

func (f *fakePaymentsRepo) Restore(context.Context, uuid.UUID, uuid.UUID) error {
	return f.err
}

func (f *fakePaymentsRepo) HardDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return f.err
}

type fakeAudit struct {
	calls      int
	lastAction string
}

func (f *fakeAudit) Log(_ context.Context, _ string, _ string, action, _ string, _ string, _ map[string]any) {
	f.calls++
	f.lastAction = action
}

func TestCreateSalePayment_AuditLogOnSuccess(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	saleID := uuid.New()
	paymentID := uuid.New()
	created := time.Date(2025, 3, 21, 12, 0, 0, 0, time.UTC)
	out := paymentsdomain.Payment{
		ID:            paymentID,
		OrgID:         orgID,
		ReferenceType: "sale",
		ReferenceID:   saleID,
		Method:        "cash",
		Amount:        100.5,
		Notes:         "partial",
		ReceivedAt:    created,
		CreatedBy:     "user:test",
		CreatedAt:     created,
	}
	audit := &fakeAudit{}
	uc := NewUsecases(&fakePaymentsRepo{out: out}, audit, nil)
	got, err := uc.CreateSalePayment(context.Background(), orgID, saleID, paymentsdomain.Payment{
		Method:     "cash",
		Amount:     100.5,
		Notes:      "partial",
		ReceivedAt: created,
		CreatedBy:  "user:test",
	})
	if err != nil {
		t.Fatalf("CreateSalePayment: %v", err)
	}
	if got.ID != paymentID {
		t.Fatalf("expected payment id %s, got %s", paymentID, got.ID)
	}
	if audit.calls != 1 || audit.lastAction != "payment.created" {
		t.Fatalf("audit: calls=%d action=%q", audit.calls, audit.lastAction)
	}
}

func TestCreateSalePayment_NilAuditNoPanic(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	saleID := uuid.New()
	paymentID := uuid.New()
	created := time.Date(2025, 3, 21, 12, 0, 0, 0, time.UTC)
	out := paymentsdomain.Payment{
		ID:            paymentID,
		OrgID:         orgID,
		ReferenceType: "sale",
		ReferenceID:   saleID,
		Method:        "transfer",
		Amount:        50,
		ReceivedAt:    created,
		CreatedBy:     "svc",
		CreatedAt:     created,
	}
	uc := NewUsecases(&fakePaymentsRepo{out: out}, nil, nil)
	_, err := uc.CreateSalePayment(context.Background(), orgID, saleID, paymentsdomain.Payment{
		Method:     "transfer",
		Amount:     50,
		ReceivedAt: created,
		CreatedBy:  "svc",
	})
	if err != nil {
		t.Fatalf("CreateSalePayment: %v", err)
	}
}

func TestCreateSalePayment_NoAuditWhenRepoFails(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	saleID := uuid.New()
	audit := &fakeAudit{}
	uc := NewUsecases(&fakePaymentsRepo{err: errors.New("repo failed")}, audit, nil)
	_, err := uc.CreateSalePayment(context.Background(), orgID, saleID, paymentsdomain.Payment{
		Method:    "cash",
		Amount:    10,
		CreatedBy: "user:x",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if audit.calls != 0 {
		t.Fatalf("expected no audit call, got %d", audit.calls)
	}
}
