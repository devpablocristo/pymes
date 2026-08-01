package commerce

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	"github.com/google/uuid"
)

type adjustmentRelayStoreStub struct {
	RelayStore
	failure    domain.AccountingFailure
	adjustment domain.AccountingAdjustment
	marked     string
	resumed    bool
}

func (*adjustmentRelayStoreStub) RecordAccountingPeriodLocked(
	context.Context,
	domain.Event,
	domain.AccountingFailure,
) error {
	return nil
}

func (s *adjustmentRelayStoreStub) GetAccountingFailure(
	context.Context,
	string,
	string,
) (domain.AccountingFailure, error) {
	return s.failure, nil
}

func (s *adjustmentRelayStoreStub) GetAccountingAdjustment(
	context.Context,
	string,
	string,
) (domain.AccountingAdjustment, error) {
	return s.adjustment, nil
}

func (*adjustmentRelayStoreStub) MarkAccountingAdjustmentPeriodLocked(
	context.Context,
	domain.Event,
	domain.AccountingFailure,
	domain.AccountingAdjustment,
	json.RawMessage,
	time.Time,
) error {
	return fmt.Errorf("unexpected PERIOD_LOCKED")
}

func (s *adjustmentRelayStoreStub) ResumeAccountingReversalAfterAdjustment(
	_ context.Context,
	failure domain.AccountingFailure,
	adjustment domain.AccountingAdjustment,
) error {
	if failure.ID != s.failure.ID || adjustment.ID != s.adjustment.ID {
		return fmt.Errorf("unexpected resume identity")
	}
	s.resumed = true
	return nil
}

func (s *adjustmentRelayStoreStub) GetPurchase(
	context.Context,
	string,
	string,
) (domain.Purchase, error) {
	return domain.Purchase{
		ID: s.failure.SourceID, OrganizationID: s.failure.OrganizationID,
		Status:              domain.AccountingAdjustmentPending,
		AccountingFailureID: s.failure.ID,
	}, nil
}

func (s *adjustmentRelayStoreStub) MarkPurchasePosted(
	_ context.Context,
	_ domain.Purchase,
	_ domain.AccountingEvent,
) error {
	s.marked = "posting"
	return nil
}

func (s *adjustmentRelayStoreStub) GetAccountingApplication(
	context.Context,
	string,
	string,
) (domain.PendingAccountingApplication, error) {
	return domain.PendingAccountingApplication{
		ID: s.failure.SourceID, OrganizationID: s.failure.OrganizationID,
		Status:              domain.AccountingAdjustmentPending,
		AccountingFailureID: s.failure.ID,
	}, nil
}

func (s *adjustmentRelayStoreStub) MarkAccountingApplicationApplied(
	_ context.Context,
	_ domain.PendingAccountingApplication,
	_ domain.AccountingEvent,
) error {
	s.marked = "application"
	return nil
}

func (s *adjustmentRelayStoreStub) GetAccountingReversal(
	context.Context,
	string,
	string,
) (domain.AccountingReversal, error) {
	return domain.AccountingReversal{
		ID: s.failure.SourceID, OrganizationID: s.failure.OrganizationID,
		DocumentKind: "purchase", DocumentID: "purchase_1",
		Status:              domain.AccountingAdjustmentPending,
		AccountingFailureID: s.failure.ID,
	}, nil
}

func (s *adjustmentRelayStoreStub) MarkAccountingReversalCompleted(
	_ context.Context,
	_ domain.AccountingReversal,
	_ domain.AccountingEvent,
) error {
	s.marked = "reversal"
	return nil
}

func (s *adjustmentRelayStoreStub) ListAppliedAccountingApplications(
	context.Context,
	string,
	string,
	string,
) ([]domain.PendingAccountingApplication, error) {
	return []domain.PendingAccountingApplication{{
		ID: "application_command_1", OrganizationID: s.failure.OrganizationID,
		ApplicationID: "accounting_application_1", Status: "applied",
	}}, nil
}

func (s *adjustmentRelayStoreStub) MarkAccountingApplicationReversed(
	_ context.Context,
	value domain.PendingAccountingApplication,
	_ domain.AccountingEvent,
) error {
	if value.ApplicationID != "accounting_application_1" {
		return fmt.Errorf("unexpected application")
	}
	s.marked = "application_reversal"
	return nil
}

type adjustmentAccountingStub struct {
	t           *testing.T
	kind        string
	effectiveAt time.Time
	originalKey string
}

func (s adjustmentAccountingStub) result(
	commandID, organizationID, idempotencyKey string,
	sourceVersion int,
	snapshotDigest, correlationID, status string,
) domain.AccountingEvent {
	s.t.Helper()
	if idempotencyKey == s.originalKey {
		s.t.Fatal("adjustment reused the rejected idempotency key")
	}
	return domain.AccountingEvent{
		EventID: uuid.NewString(), CommandID: commandID,
		OrganizationID: organizationID, IdempotencyKey: idempotencyKey,
		SourceVersion: sourceVersion, SnapshotDigest: snapshotDigest,
		Status: status, CorrelationID: correlationID,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func (s adjustmentAccountingStub) Post(
	_ context.Context,
	command domain.PostingCommand,
) (domain.AccountingEvent, error) {
	if s.kind != "posting" || !command.EffectiveAt.Equal(s.effectiveAt) {
		s.t.Fatalf("posting command=%+v", command)
	}
	result := s.result(
		command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID, "posted",
	)
	result.JournalEntryID, result.OpenItemIDs = uuid.NewString(), []string{uuid.NewString()}
	return result, nil
}

func (s adjustmentAccountingStub) ApplyOpenItem(
	_ context.Context,
	command domain.AccountingApplicationCommand,
) (domain.AccountingEvent, error) {
	if s.kind != "application" || !command.AppliedAt.Equal(s.effectiveAt) {
		s.t.Fatalf("application command=%+v", command)
	}
	result := s.result(
		command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID, "applied",
	)
	result.ApplicationID = uuid.NewString()
	return result, nil
}

func (s adjustmentAccountingStub) Reverse(
	_ context.Context,
	command domain.ReversalCommand,
) (domain.AccountingEvent, error) {
	if s.kind != "reversal" || !command.EffectiveAt.Equal(s.effectiveAt) {
		s.t.Fatalf("reversal command=%+v", command)
	}
	result := s.result(
		command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID, "reversed",
	)
	result.JournalEntryID = uuid.NewString()
	return result, nil
}

func (s adjustmentAccountingStub) ReverseOpenItemApplication(
	_ context.Context,
	command domain.AccountingApplicationReversalCommand,
) (domain.AccountingEvent, error) {
	if s.kind != "application_reversal" ||
		!command.ReversedAt.Equal(s.effectiveAt) {
		s.t.Fatalf("application reversal command=%+v", command)
	}
	result := s.result(
		command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID, "reversed",
	)
	result.ApplicationID = command.ApplicationID
	return result, nil
}

func TestAccountingAdjustmentDispatchesEveryAccountingCommandKind(t *testing.T) {
	effectiveAt := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	for _, kind := range []string{
		"posting", "application", "reversal", "application_reversal",
	} {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			organizationID := "org_adjustment"
			failureID := uuid.NewString()
			sourceID := kind + "_source"
			sourceKind := "accounting_reversal"
			originalKey := "original-" + kind
			var command any
			switch kind {
			case "posting":
				sourceKind = "purchase"
				command = domain.PostingCommand{
					CommandID: uuid.NewString(), OrganizationID: organizationID,
					IdempotencyKey: originalKey, SourceType: "purchase_invoice",
					SourceID: sourceID, SourceVersion: 1,
					SnapshotDigest: commandSnapshotDigest(kind),
					CorrelationID:  "correlation-" + kind,
					EffectiveAt:    time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
					Description:    "purchase", Lines: []domain.PostingLine{{}, {}},
				}
			case "application":
				sourceKind = "accounting_application"
				command = domain.AccountingApplicationCommand{
					CommandID: uuid.NewString(), OrganizationID: organizationID,
					IdempotencyKey: originalKey, SourceVersion: 1,
					SnapshotDigest: commandSnapshotDigest(kind),
					CorrelationID:  "correlation-" + kind,
					AppliedAt:      time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
				}
			case "reversal":
				command = domain.ReversalCommand{
					CommandID: uuid.NewString(), OrganizationID: organizationID,
					IdempotencyKey: originalKey, SourceVersion: 1,
					SnapshotDigest: commandSnapshotDigest(kind),
					CorrelationID:  "correlation-" + kind,
					EffectiveAt:    time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
				}
			case "application_reversal":
				command = domain.AccountingApplicationReversalCommand{
					CommandID: uuid.NewString(), OrganizationID: organizationID,
					IdempotencyKey: originalKey, SourceVersion: 1,
					SnapshotDigest: commandSnapshotDigest(kind),
					CorrelationID:  "correlation-" + kind,
					ApplicationID:  "accounting_application_1",
					ReversedAt:     time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
				}
			}
			commandPayload, err := json.Marshal(command)
			if err != nil {
				t.Fatal(err)
			}
			store := &adjustmentRelayStoreStub{
				failure: domain.AccountingFailure{
					ID: failureID, OrganizationID: organizationID,
					SourceKind: sourceKind, SourceID: sourceID,
					CommandKind: kind, CommandPayload: commandPayload,
					Status: "adjustment_pending",
				},
				adjustment: domain.AccountingAdjustment{
					ID: "adjustment_" + kind, OrganizationID: organizationID,
					FailureID: failureID, EffectiveAt: effectiveAt,
					Status: "pending",
					Origin: domain.OriginMetadata{SourceVersion: 2},
				},
			}
			eventPayload, _ := json.Marshal(map[string]string{
				"adjustment_id": store.adjustment.ID,
				"failure_id":    store.failure.ID,
			})
			worker := DurableWorker{
				Store: store, Accounting: adjustmentAccountingStub{
					t: t, kind: kind, effectiveAt: effectiveAt, originalKey: originalKey,
				},
			}
			if err := worker.applyAccountingAdjustment(
				context.Background(),
				domain.Event{
					ID: uuid.NewString(), OrganizationID: organizationID,
					Topic: "AccountingAdjustmentRequested", Payload: eventPayload,
				},
			); err != nil {
				t.Fatal(err)
			}
			if store.marked != kind {
				t.Fatalf("marked=%q", store.marked)
			}
			if (kind == "application_reversal") != store.resumed {
				t.Fatalf("resumed=%v", store.resumed)
			}
		})
	}
}
