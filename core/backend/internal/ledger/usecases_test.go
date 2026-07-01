package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	ledgerdomain "github.com/devpablocristo/pymes/core/backend/internal/ledger/usecases/domain"
)

// fakeRepo es un stub inline del RepositoryPort para testear la lógica de
// validación/balanceo del usecase sin tocar la base.
type fakeRepo struct {
	accounts []ledgerdomain.Account
	posted   *ledgerdomain.JournalEntry
}

func (f *fakeRepo) ListAccounts(_ context.Context, _ uuid.UUID, _ bool) ([]ledgerdomain.Account, error) {
	return f.accounts, nil
}
func (f *fakeRepo) GetAccount(_ context.Context, _, _ uuid.UUID) (ledgerdomain.Account, error) {
	return ledgerdomain.Account{}, nil
}
func (f *fakeRepo) CreateAccount(_ context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error) {
	return in, nil
}
func (f *fakeRepo) UpdateAccount(_ context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error) {
	return in, nil
}
func (f *fakeRepo) ArchiveAccount(_ context.Context, _, _ uuid.UUID) error { return nil }
func (f *fakeRepo) RestoreAccount(_ context.Context, _, _ uuid.UUID) error { return nil }
func (f *fakeRepo) ListLinks(_ context.Context, _ uuid.UUID) ([]ledgerdomain.AccountLink, error) {
	return nil, nil
}
func (f *fakeRepo) SetLink(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (ledgerdomain.AccountLink, error) {
	return ledgerdomain.AccountLink{}, nil
}
func (f *fakeRepo) PostEntry(_ context.Context, in ledgerdomain.JournalEntry) (ledgerdomain.JournalEntry, error) {
	in.EntryNumber = "ASTO-00000001"
	in.Status = "posted"
	f.posted = &in
	return in, nil
}
func (f *fakeRepo) Journal(_ context.Context, _ uuid.UUID, _, _ time.Time, _ int) ([]ledgerdomain.JournalEntry, error) {
	return nil, nil
}
func (f *fakeRepo) AccountLedger(_ context.Context, _, _ uuid.UUID, _, _ time.Time, _ int) (ledgerdomain.AccountLedger, error) {
	return ledgerdomain.AccountLedger{}, nil
}
func (f *fakeRepo) TrialBalance(_ context.Context, _ uuid.UUID, _ time.Time) (ledgerdomain.TrialBalance, error) {
	return ledgerdomain.TrialBalance{}, nil
}
func (f *fakeRepo) OutboxHealth(_ context.Context, _ uuid.UUID) (ledgerdomain.OutboxHealth, error) {
	return ledgerdomain.OutboxHealth{}, nil
}
func (f *fakeRepo) SeedChart(_ context.Context, _ uuid.UUID, _ []SeedAccount) error { return nil }
func (f *fakeRepo) EnqueueOutbox(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, _ string, _ []byte, _ bool) error {
	return nil
}
func (f *fakeRepo) ProcessDueOutbox(_ context.Context, _ *uuid.UUID, _ int) (int, int, int, error) {
	return 0, 0, 0, nil
}

func line(accountID uuid.UUID, debit, credit float64) ledgerdomain.JournalLine {
	return ledgerdomain.JournalLine{AccountID: accountID, Debit: debit, Credit: credit}
}

func TestPostManual(t *testing.T) {
	t.Parallel()

	org := uuid.New()
	cash := uuid.New()    // postable
	revenue := uuid.New() // postable
	heading := uuid.New() // no postable
	accounts := []ledgerdomain.Account{
		{ID: cash, OrgID: org, Code: "1.1.01", Name: "Caja", Type: "A", IsPostable: true},
		{ID: revenue, OrgID: org, Code: "4.1.01", Name: "Ventas", Type: "I", IsPostable: true},
		{ID: heading, OrgID: org, Code: "4", Name: "Ingresos", Type: "I", IsPostable: false},
	}

	tests := []struct {
		name    string
		lines   []ledgerdomain.JournalLine
		wantErr bool
	}{
		{
			name:  "balanced two-line entry posts",
			lines: []ledgerdomain.JournalLine{line(cash, 121, 0), line(revenue, 0, 121)},
		},
		{
			name:    "off by one cent rejected",
			lines:   []ledgerdomain.JournalLine{line(cash, 100.00, 0), line(revenue, 0, 100.01)},
			wantErr: true,
		},
		{
			name:    "unbalanced entry rejected",
			lines:   []ledgerdomain.JournalLine{line(cash, 100, 0), line(revenue, 0, 90)},
			wantErr: true,
		},
		{
			name:    "single line rejected",
			lines:   []ledgerdomain.JournalLine{line(cash, 100, 0)},
			wantErr: true,
		},
		{
			name:    "line with both debit and credit rejected",
			lines:   []ledgerdomain.JournalLine{line(cash, 50, 50), line(revenue, 0, 0)},
			wantErr: true,
		},
		{
			name:    "negative amount rejected",
			lines:   []ledgerdomain.JournalLine{line(cash, -100, 0), line(revenue, 0, -100)},
			wantErr: true,
		},
		{
			name:    "unknown account rejected",
			lines:   []ledgerdomain.JournalLine{line(uuid.New(), 100, 0), line(revenue, 0, 100)},
			wantErr: true,
		},
		{
			name:    "non-postable account rejected",
			lines:   []ledgerdomain.JournalLine{line(cash, 100, 0), line(heading, 0, 100)},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := NewUsecases(&fakeRepo{accounts: accounts})
			out, err := uc.PostManual(context.Background(), ledgerdomain.JournalEntry{OrgID: org, Lines: tc.lines})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got entry %+v", out)
				}
				if !domainerr.IsValidation(err) {
					t.Fatalf("expected validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.EntryNumber == "" {
				t.Fatalf("expected posted entry to have a number")
			}
		})
	}
}

func TestCreateAccountValidation(t *testing.T) {
	t.Parallel()

	org := uuid.New()
	tests := []struct {
		name    string
		in      ledgerdomain.Account
		wantErr bool
	}{
		{name: "valid", in: ledgerdomain.Account{OrgID: org, Code: "1.1.01", Name: "Caja", Type: "A"}},
		{name: "lowercase type normalized", in: ledgerdomain.Account{OrgID: org, Code: "1", Name: "X", Type: "a"}},
		{name: "missing org", in: ledgerdomain.Account{Code: "1", Name: "X", Type: "A"}, wantErr: true},
		{name: "missing code", in: ledgerdomain.Account{OrgID: org, Name: "X", Type: "A"}, wantErr: true},
		{name: "missing name", in: ledgerdomain.Account{OrgID: org, Code: "1", Type: "A"}, wantErr: true},
		{name: "invalid type", in: ledgerdomain.Account{OrgID: org, Code: "1", Name: "X", Type: "Z"}, wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := NewUsecases(&fakeRepo{})
			_, err := uc.CreateAccount(context.Background(), tc.in)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
