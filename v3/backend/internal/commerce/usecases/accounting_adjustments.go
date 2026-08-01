package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
)

type accountingFailureRecorder interface {
	RecordAccountingPeriodLocked(context.Context, domain.Event, domain.AccountingFailure) error
}

type accountingFailureRelayStore interface {
	accountingFailureRecorder
	GetAccountingFailure(context.Context, string, string) (domain.AccountingFailure, error)
	GetAccountingAdjustment(context.Context, string, string) (domain.AccountingAdjustment, error)
	MarkAccountingAdjustmentPeriodLocked(
		context.Context,
		domain.Event,
		domain.AccountingFailure,
		domain.AccountingAdjustment,
		json.RawMessage,
		time.Time,
	) error
	ResumeAccountingReversalAfterAdjustment(
		context.Context,
		domain.AccountingFailure,
		domain.AccountingAdjustment,
	) error
}

func (w DurableWorker) accountingFailureRecorder() (accountingFailureRecorder, error) {
	store, ok := w.Store.(accountingFailureRecorder)
	if !ok {
		return nil, fmt.Errorf("accounting failure persistence is not configured")
	}
	return store, nil
}

func (w DurableWorker) accountingFailures() (accountingFailureRelayStore, error) {
	store, ok := w.Store.(accountingFailureRelayStore)
	if !ok {
		return nil, fmt.Errorf("accounting adjustment persistence is not configured")
	}
	return store, nil
}

func (w DurableWorker) recordAccountingPeriodLocked(
	ctx context.Context,
	event domain.Event,
	sourceKind, sourceID, commandKind string,
	command any,
	effectiveAt time.Time,
	snapshotDigest string,
) error {
	store, err := w.accountingFailureRecorder()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("encode rejected accounting command: %w", err)
	}
	origin := domain.OriginMetadata{
		RequestID:     strings.TrimSpace(event.RequestID),
		ActorRef:      strings.TrimSpace(event.ActorRef),
		SourceVersion: event.SourceVersion,
		CorrelationID: strings.TrimSpace(event.CorrelationID),
	}
	if origin.RequestID == "" {
		origin.RequestID = event.ID
	}
	if origin.ActorRef == "" {
		origin.ActorRef = "system:outbox"
	}
	if origin.SourceVersion < 1 {
		origin.SourceVersion = 1
	}
	if origin.CorrelationID == "" {
		origin.CorrelationID = origin.RequestID
	}
	return store.RecordAccountingPeriodLocked(ctx, event, domain.AccountingFailure{
		ID: event.ID, OrganizationID: event.OrganizationID,
		OriginalEventID: event.ID, SourceKind: sourceKind, SourceID: sourceID,
		CommandKind: commandKind, CommandPayload: payload,
		CommandDigest:     commandPayloadDigest(payload),
		FailedEffectiveAt: effectiveAt.UTC(),
		Status:            "awaiting_adjustment", FailureCode: domain.ErrPeriodLocked.Error(),
		SnapshotDigest: snapshotDigest, Origin: origin,
		CorrelationID: origin.CorrelationID,
	})
}

func (w DurableWorker) applyAccountingAdjustment(ctx context.Context, event domain.Event) error {
	store, err := w.accountingFailures()
	if err != nil {
		return err
	}
	var payload struct {
		AdjustmentID string `json:"adjustment_id"`
		FailureID    string `json:"failure_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	if payload.AdjustmentID == "" || payload.FailureID == "" {
		return fmt.Errorf("INVALID_ACCOUNTING_ADJUSTMENT_EVENT")
	}
	failure, err := store.GetAccountingFailure(ctx, event.OrganizationID, payload.FailureID)
	if err != nil {
		return err
	}
	adjustment, err := store.GetAccountingAdjustment(ctx, event.OrganizationID, payload.AdjustmentID)
	if err != nil {
		return err
	}
	if failure.OrganizationID != event.OrganizationID ||
		adjustment.OrganizationID != event.OrganizationID ||
		adjustment.FailureID != failure.ID {
		return fmt.Errorf("ACCOUNTING_ADJUSTMENT_ORGANIZATION_MISMATCH")
	}
	if adjustment.Status != "pending" {
		return nil
	}
	if failure.Status != "adjustment_pending" {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: accounting adjustment")
	}
	switch failure.CommandKind {
	case "posting":
		return w.retryPostingAdjustment(ctx, event, store, failure, adjustment)
	case "application":
		return w.retryApplicationAdjustment(ctx, event, store, failure, adjustment)
	case "reversal":
		return w.retryReversalAdjustment(ctx, event, store, failure, adjustment)
	case "application_reversal":
		return w.retryApplicationReversalAdjustment(ctx, event, store, failure, adjustment)
	default:
		return fmt.Errorf("unsupported accounting adjustment command %q", failure.CommandKind)
	}
}

func (w DurableWorker) retryPostingAdjustment(
	ctx context.Context,
	event domain.Event,
	store accountingFailureRelayStore,
	failure domain.AccountingFailure,
	adjustment domain.AccountingAdjustment,
) error {
	var command domain.PostingCommand
	if err := json.Unmarshal(failure.CommandPayload, &command); err != nil {
		return err
	}
	command.CommandID = adjustmentCommandID(adjustment.ID, adjustment.Origin.SourceVersion)
	command.IdempotencyKey = internalIdempotencyKey(
		failure.OrganizationID, "accounting.adjust.post",
		adjustment.ID, adjustment.Origin.SourceVersion,
	)
	command.EffectiveAt = adjustment.EffectiveAt.UTC()
	result, err := w.Accounting.Post(ctx, command)
	if err != nil {
		if errors.Is(err, domain.ErrPeriodLocked) {
			return markAdjustedPeriodLocked(
				ctx, store, event, failure, adjustment, command, command.EffectiveAt,
			)
		}
		return err
	}
	if err := validateAccountingEvent(
		result, command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID,
	); err != nil {
		return err
	}
	if result.Status != "posted" && result.Status != "duplicate" {
		return fmt.Errorf("accounting rejected posting adjustment")
	}
	if _, err := singleOpenItem(result); err != nil {
		return err
	}
	switch failure.SourceKind {
	case "sale":
		value, err := w.Store.GetSale(ctx, failure.OrganizationID, failure.SourceID)
		if err != nil {
			return err
		}
		if value.Status != domain.SaleStatus(domain.AccountingAdjustmentPending) ||
			value.AccountingFailureID != failure.ID {
			return fmt.Errorf("STATE_TRANSITION_REJECTED: sale accounting adjustment")
		}
		return w.Store.MarkSalePosted(ctx, value, result)
	case "purchase":
		value, err := w.Store.GetPurchase(ctx, failure.OrganizationID, failure.SourceID)
		if err != nil {
			return err
		}
		if value.Status != domain.AccountingAdjustmentPending ||
			value.AccountingFailureID != failure.ID {
			return fmt.Errorf("STATE_TRANSITION_REJECTED: purchase accounting adjustment")
		}
		return w.Store.MarkPurchasePosted(ctx, value, result)
	case "payment":
		value, err := w.Store.GetPayment(ctx, failure.OrganizationID, failure.SourceID)
		if err != nil {
			return err
		}
		if value.Status != domain.AccountingAdjustmentPending ||
			value.AccountingFailureID != failure.ID {
			return fmt.Errorf("STATE_TRANSITION_REJECTED: payment accounting adjustment")
		}
		return w.Store.MarkPaymentPosted(ctx, value, result)
	default:
		return fmt.Errorf("unsupported posting adjustment source %q", failure.SourceKind)
	}
}

func (w DurableWorker) retryApplicationAdjustment(
	ctx context.Context,
	event domain.Event,
	store accountingFailureRelayStore,
	failure domain.AccountingFailure,
	adjustment domain.AccountingAdjustment,
) error {
	var command domain.AccountingApplicationCommand
	if err := json.Unmarshal(failure.CommandPayload, &command); err != nil {
		return err
	}
	command.CommandID = adjustmentCommandID(adjustment.ID, adjustment.Origin.SourceVersion)
	command.IdempotencyKey = internalIdempotencyKey(
		failure.OrganizationID, "accounting.adjust.apply",
		adjustment.ID, adjustment.Origin.SourceVersion,
	)
	command.AppliedAt = adjustment.EffectiveAt.UTC()
	result, err := w.Accounting.ApplyOpenItem(ctx, command)
	if err != nil {
		if errors.Is(err, domain.ErrPeriodLocked) {
			return markAdjustedPeriodLocked(
				ctx, store, event, failure, adjustment, command, command.AppliedAt,
			)
		}
		return err
	}
	if err := validateAccountingEvent(
		result, command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID,
	); err != nil {
		return err
	}
	if result.Status != "applied" && result.Status != "duplicate" {
		return fmt.Errorf("accounting rejected application adjustment")
	}
	value, err := w.Store.GetAccountingApplication(
		ctx, failure.OrganizationID, failure.SourceID,
	)
	if err != nil {
		return err
	}
	if value.Status != domain.AccountingAdjustmentPending ||
		value.AccountingFailureID != failure.ID {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: application accounting adjustment")
	}
	return w.Store.MarkAccountingApplicationApplied(ctx, value, result)
}

func (w DurableWorker) retryReversalAdjustment(
	ctx context.Context,
	event domain.Event,
	store accountingFailureRelayStore,
	failure domain.AccountingFailure,
	adjustment domain.AccountingAdjustment,
) error {
	var command domain.ReversalCommand
	if err := json.Unmarshal(failure.CommandPayload, &command); err != nil {
		return err
	}
	command.CommandID = adjustmentCommandID(adjustment.ID, adjustment.Origin.SourceVersion)
	command.IdempotencyKey = internalIdempotencyKey(
		failure.OrganizationID, "accounting.adjust.reverse",
		adjustment.ID, adjustment.Origin.SourceVersion,
	)
	command.EffectiveAt = adjustment.EffectiveAt.UTC()
	result, err := w.Accounting.Reverse(ctx, command)
	if err != nil {
		if errors.Is(err, domain.ErrPeriodLocked) {
			return markAdjustedPeriodLocked(
				ctx, store, event, failure, adjustment, command, command.EffectiveAt,
			)
		}
		return err
	}
	if err := validateAccountingEvent(
		result, command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID,
	); err != nil {
		return err
	}
	if result.Status != "reversed" && result.Status != "duplicate" {
		return fmt.Errorf("accounting rejected reversal adjustment")
	}
	value, err := w.Store.GetAccountingReversal(
		ctx, failure.OrganizationID, failure.SourceID,
	)
	if err != nil {
		return err
	}
	if value.Status != domain.AccountingAdjustmentPending ||
		value.AccountingFailureID != failure.ID {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: reversal accounting adjustment")
	}
	return w.Store.MarkAccountingReversalCompleted(ctx, value, result)
}

func (w DurableWorker) retryApplicationReversalAdjustment(
	ctx context.Context,
	event domain.Event,
	store accountingFailureRelayStore,
	failure domain.AccountingFailure,
	adjustment domain.AccountingAdjustment,
) error {
	var command domain.AccountingApplicationReversalCommand
	if err := json.Unmarshal(failure.CommandPayload, &command); err != nil {
		return err
	}
	command.CommandID = adjustmentCommandID(adjustment.ID, adjustment.Origin.SourceVersion)
	command.IdempotencyKey = internalIdempotencyKey(
		failure.OrganizationID, "accounting.adjust.reverse-application",
		adjustment.ID, adjustment.Origin.SourceVersion,
	)
	command.ReversedAt = adjustment.EffectiveAt.UTC()
	result, err := w.Accounting.ReverseOpenItemApplication(ctx, command)
	if err != nil {
		if errors.Is(err, domain.ErrPeriodLocked) {
			return markAdjustedPeriodLocked(
				ctx, store, event, failure, adjustment, command, command.ReversedAt,
			)
		}
		return err
	}
	if err := validateAccountingEvent(
		result, command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID,
	); err != nil {
		return err
	}
	if result.Status != "reversed" && result.Status != "duplicate" {
		return fmt.Errorf("accounting rejected application reversal adjustment")
	}
	reversal, err := w.Store.GetAccountingReversal(
		ctx, failure.OrganizationID, failure.SourceID,
	)
	if err != nil {
		return err
	}
	if reversal.Status != domain.AccountingAdjustmentPending ||
		reversal.AccountingFailureID != failure.ID {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: application reversal adjustment")
	}
	applications, err := w.Store.ListAppliedAccountingApplications(
		ctx, reversal.OrganizationID, reversal.DocumentKind, reversal.DocumentID,
	)
	if err != nil {
		return err
	}
	for _, application := range applications {
		if application.ApplicationID == command.ApplicationID {
			if err := w.Store.MarkAccountingApplicationReversed(ctx, application, result); err != nil {
				return err
			}
			return store.ResumeAccountingReversalAfterAdjustment(ctx, failure, adjustment)
		}
	}
	return fmt.Errorf("ACCOUNTING_APPLICATION_NOT_FOUND")
}

func markAdjustedPeriodLocked(
	ctx context.Context,
	store accountingFailureRelayStore,
	event domain.Event,
	failure domain.AccountingFailure,
	adjustment domain.AccountingAdjustment,
	command any,
	effectiveAt time.Time,
) error {
	payload, err := json.Marshal(command)
	if err != nil {
		return err
	}
	return store.MarkAccountingAdjustmentPeriodLocked(
		ctx, event, failure, adjustment, payload, effectiveAt,
	)
}

func adjustmentCommandID(adjustmentID string, sourceVersion int) string {
	return accountingCommandID("adjustment", adjustmentID, sourceVersion)
}

func commandPayloadDigest(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
