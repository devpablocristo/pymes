package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
)

type commandStoreStub struct {
	CommandStore
	ping        func(context.Context) error
	createParty func(context.Context, domain.IdempotencyCommand, domain.Party) (domain.Party, error)
}

type adjustmentCommandStoreStub struct {
	value domain.AccountingAdjustment
	calls int
}

func (s *adjustmentCommandStoreStub) GetAccountingFailure(
	context.Context,
	string,
	string,
) (domain.AccountingFailure, error) {
	return domain.AccountingFailure{}, nil
}

func (s *adjustmentCommandStoreStub) RequestAccountingAdjustmentIdempotent(
	_ context.Context,
	_ domain.IdempotencyCommand,
	_ string,
	value domain.AccountingAdjustment,
) (domain.AccountingAdjustment, error) {
	s.calls++
	s.value = value
	return value, nil
}

func (s commandStoreStub) Ping(ctx context.Context) error { return s.ping(ctx) }
func (s commandStoreStub) CreatePartyIdempotent(ctx context.Context, command domain.IdempotencyCommand, party domain.Party) (domain.Party, error) {
	return s.createParty(ctx, command, party)
}

func TestCommandsReadinessChecksDatabasePort(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("database unavailable")
	commands := Commands{Store: commandStoreStub{ping: func(context.Context) error { return sentinel }}}
	if err := commands.Ready(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("expected database error, got %v", err)
	}
	commands.Store = commandStoreStub{ping: func(context.Context) error { return nil }}
	if err := commands.Ready(context.Background()); err != nil {
		t.Fatalf("expected ready store, got %v", err)
	}
}

func TestCommandsReadinessRejectsMissingStore(t *testing.T) {
	t.Parallel()
	if err := (Commands{}).Ready(context.Background()); err == nil {
		t.Fatal("expected missing store to be not ready")
	}
}

func TestCreatePartyIdempotentValidatesCanonicalCommandIdentity(t *testing.T) {
	t.Parallel()
	party := domain.Party{ID: "party_1", OrganizationID: "org_1", Kind: "customer", DisplayName: "Alice"}
	valid := domain.IdempotencyCommand{
		Key: "request-key", OrganizationID: party.OrganizationID,
		Operation: domain.OperationCreateParty, SourceID: party.ID, SourceVersion: 1,
		PayloadHash: strings.Repeat("a", 64),
	}
	calls := 0
	commands := Commands{Store: commandStoreStub{createParty: func(_ context.Context, command domain.IdempotencyCommand, actual domain.Party) (domain.Party, error) {
		calls++
		if command != valid || actual != party {
			t.Fatalf("command=%+v party=%+v", command, actual)
		}
		return actual, nil
	}}}
	if result, err := commands.CreatePartyIdempotent(context.Background(), valid, party); err != nil || result != party {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	invalid := valid
	invalid.OrganizationID = "org_other"
	if _, err := commands.CreatePartyIdempotent(context.Background(), invalid, party); err == nil || err.Error() != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %v", err)
	}
	invalid = valid
	invalid.PayloadHash = strings.Repeat("z", 64)
	if _, err := commands.CreatePartyIdempotent(context.Background(), invalid, party); err == nil || err.Error() != "VALIDATION_ERROR" {
		t.Fatalf("expected invalid hash error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("store calls=%d", calls)
	}
}

func TestRequestAccountingAdjustmentValidatesExplicitIdempotentCommand(t *testing.T) {
	t.Parallel()
	store := &adjustmentCommandStoreStub{}
	value := domain.AccountingAdjustment{
		ID: "adjustment_1", OrganizationID: "org_1", FailureID: "failure_1",
		EffectiveAt: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC),
		Reason:      "authorized open period",
	}
	command := domain.IdempotencyCommand{
		Key: "adjustment-key", OrganizationID: value.OrganizationID,
		Operation: domain.OperationCreateAccountingAdjustment,
		SourceID:  value.ID, SourceVersion: 1, PayloadHash: strings.Repeat("a", 64),
	}
	commands := Commands{AccountingAdjustments: store}
	result, err := commands.RequestAccountingAdjustmentIdempotent(
		context.Background(), command, value.FailureID, value,
	)
	if err != nil || result.ID != value.ID || store.calls != 1 {
		t.Fatalf("result=%+v calls=%d err=%v", result, store.calls, err)
	}
	invalid := command
	invalid.SourceID = "other"
	if _, err := commands.RequestAccountingAdjustmentIdempotent(
		context.Background(), invalid, value.FailureID, value,
	); err == nil || err.Error() != "VALIDATION_ERROR" {
		t.Fatalf("invalid command error=%v", err)
	}
	if store.calls != 1 {
		t.Fatalf("invalid command reached store: calls=%d", store.calls)
	}
}
