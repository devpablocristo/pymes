// architecture:adapter worker
package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	workerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/commerce/worker/helpers"
	workermodels "github.com/devpablocristo/pymes/v3/backend/internal/commerce/worker/models"
	"github.com/google/uuid"
)

// RelayStore is the persistence port for the commerce outbox relay. PostgreSQL
// is an adapter of this port; the workflow itself has no pgx dependency.
type FiscalClient interface {
	Authorize(context.Context, domain.FiscalRequest) (domain.FiscalResult, error)
	Consult(context.Context, domain.FiscalRequest) (domain.FiscalResult, error)
}

type AccountingClient interface {
	Post(context.Context, domain.PostingCommand) (domain.AccountingEvent, error)
	Reverse(context.Context, domain.ReversalCommand) (domain.AccountingEvent, error)
	ApplyOpenItem(context.Context, domain.AccountingApplicationCommand) (domain.AccountingEvent, error)
	ReverseOpenItemApplication(context.Context, domain.AccountingApplicationReversalCommand) (domain.AccountingEvent, error)
}

// RelayStore is the persistence port for the commerce outbox relay. PostgreSQL
// is an adapter of this port; the workflow itself has no pgx dependency.
type RelayStore interface {
	Lease(context.Context, int, time.Duration) ([]domain.Event, error)
	Retry(context.Context, domain.Event) error
	DeadLetter(context.Context, domain.Event, string) error
	MarkPublished(context.Context, domain.Event) error
	GetSale(context.Context, string, string) (domain.Sale, error)
	ApplyFiscalResult(context.Context, domain.Sale, domain.FiscalResult) error
	MarkSalePosted(context.Context, domain.Sale, domain.AccountingEvent) error
	GetPurchase(context.Context, string, string) (domain.Purchase, error)
	MarkPurchasePosted(context.Context, domain.Purchase, domain.AccountingEvent) error
	GetPayment(context.Context, string, string) (domain.Payment, error)
	MarkPaymentPosted(context.Context, domain.Payment, domain.AccountingEvent) error
	GetAccountingApplication(context.Context, string, string) (domain.PendingAccountingApplication, error)
	MarkAccountingApplicationApplied(context.Context, domain.PendingAccountingApplication, domain.AccountingEvent) error
	ListAppliedAccountingApplications(context.Context, string, string, string) ([]domain.PendingAccountingApplication, error)
	MarkAccountingApplicationReversed(context.Context, domain.PendingAccountingApplication, domain.AccountingEvent) error
	GetAccountingReversal(context.Context, string, string) (domain.AccountingReversal, error)
	MarkAccountingReversalCompleted(context.Context, domain.AccountingReversal, domain.AccountingEvent) error
	ListUncertainSales(context.Context, int) ([]domain.PendingFiscal, error)
	ReserveFiscalConsultAttempt(context.Context, string, string) (int, error)
}

// DurableWorker relays commerce commands at-least-once. Private services must
// treat a repeated command identifier as a duplicate.
type DurableWorker struct {
	Store       RelayStore
	Fiscal      FiscalClient
	Accounting  AccountingClient
	LeaseFor    time.Duration
	MaxAttempts int
}

func (w DurableWorker) DispatchOnce(ctx context.Context) error {
	if w.Store == nil || w.Fiscal == nil || w.Accounting == nil {
		return fmt.Errorf("worker dependencies are not configured")
	}
	leaseFor := w.LeaseFor
	if leaseFor <= 0 {
		leaseFor = 30 * time.Second
	}
	events, err := w.Store.Lease(ctx, 20, leaseFor)
	if err != nil {
		return err
	}
	for _, event := range events {
		deliveryContext := outboxDeliveryContext(ctx, event)
		if err := w.handle(deliveryContext, event); err != nil {
			maxAttempts := w.MaxAttempts
			if maxAttempts <= 0 {
				maxAttempts = 10
			}
			if event.Attempts >= maxAttempts {
				if deadLetterErr := w.Store.DeadLetter(ctx, event, "DELIVERY_FAILED"); deadLetterErr != nil {
					return fmt.Errorf("dead-letter %s: %w (delivery: %v)", event.ID, deadLetterErr, err)
				}
				continue
			}
			if retryErr := w.Store.Retry(ctx, event); retryErr != nil {
				return fmt.Errorf("handle %s: %w (retry: %v)", event.ID, err, retryErr)
			}
			continue
		}
		if err := w.Store.MarkPublished(ctx, event); err != nil {
			return err
		}
	}
	return (ReconcileFiscal{Store: w.Store, Fiscal: w.Fiscal}).Execute(ctx)
}

func (w DurableWorker) handle(ctx context.Context, event domain.Event) error {
	if event.Topic == "AccountingAdjustmentRequested" {
		return w.applyAccountingAdjustment(ctx, event)
	}
	if event.Topic == "PurchasePostingRequested" {
		return w.postPurchase(ctx, event)
	}
	if event.Topic == "PaymentPostingRequested" {
		return w.postPayment(ctx, event)
	}
	if event.Topic == "OpenItemApplicationRequested" {
		return w.applyOpenItem(ctx, event)
	}
	if event.Topic == "AccountingReversalRequested" {
		return w.reverseAccounting(ctx, event)
	}
	var payload workermodels.SaleEvent
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	sale, err := w.Store.GetSale(ctx, event.OrganizationID, payload.SaleID)
	if err != nil {
		return err
	}
	if err := workerhelpers.RequireEventOrganization(event.ID, event.OrganizationID, sale.OrganizationID); err != nil {
		return err
	}
	switch event.Topic {
	case "FiscalAuthorizationRequested":
		if sale.Status != domain.SaleFiscalPending && sale.Status != domain.SaleFiscalUncertain {
			return nil
		}
		sourceVersion := persistedSourceVersion(sale.Origin)
		request := domain.FiscalRequest{
			RequestID:      fmt.Sprintf("fiscal-authorize:%s:%d", sale.ID, sourceVersion),
			OrganizationID: sale.OrganizationID,
			IdempotencyKey: internalIdempotencyKey(sale.OrganizationID, "fiscal.authorize", sale.ID, sourceVersion),
			SourceVersion:  sourceVersion,
			CredentialRef:  payload.CredentialRef,
			Voucher:        sale.Voucher,
			Total:          sale.Total,
			SnapshotDigest: sale.SnapshotDigest,
			CorrelationID:  persistedCorrelationID(sale.Origin, sale.CorrelationID),
			FiscalSnapshot: sale.FiscalSnapshot,
		}
		result, err := w.Fiscal.Authorize(ctx, request)
		if err != nil {
			return err
		}
		if err := validateFiscalResult(request, result); err != nil {
			return err
		}
		return w.Store.ApplyFiscalResult(ctx, sale, result)
	case "AccountingPostingRequested":
		if sale.Status != domain.SaleAuthorizedPendingPosting {
			return nil
		}
		var original *domain.Sale
		if sale.SourceDocumentID != "" {
			value, err := w.Store.GetSale(ctx, sale.OrganizationID, sale.SourceDocumentID)
			if err != nil {
				return err
			}
			original = &value
		}
		command, err := buildSalePostingCommand(sale, original)
		if err != nil {
			return err
		}
		result, err := w.Accounting.Post(ctx, command)
		if err != nil {
			if errors.Is(err, domain.ErrPeriodLocked) {
				return w.recordAccountingPeriodLocked(
					ctx, event, "sale", sale.ID, "posting",
					command, command.EffectiveAt, command.SnapshotDigest,
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
			return fmt.Errorf("accounting rejected command")
		}
		if _, err := singleOpenItem(result); err != nil {
			return err
		}
		return w.Store.MarkSalePosted(ctx, sale, result)
	default:
		return fmt.Errorf("unknown outbox topic %q", event.Topic)
	}
}

func (w DurableWorker) postPayment(ctx context.Context, event domain.Event) error {
	var payload workermodels.PaymentEvent
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	payment, err := w.Store.GetPayment(ctx, event.OrganizationID, payload.PaymentID)
	if err != nil {
		return err
	}
	if err := workerhelpers.RequireEventOrganization(event.ID, event.OrganizationID, payment.OrganizationID); err != nil {
		return err
	}
	if payment.Status != "confirmed" {
		return nil
	}
	debit, credit := "1100", "1200"
	if payment.Direction == "disbursement" {
		debit, credit = "2100", "1100"
	}
	zero := domain.Money{Amount: "0", Currency: payment.Total.Currency}
	lines := []domain.PostingLine{{AccountCode: debit, Debit: payment.Total, Credit: zero}, {AccountCode: credit, Debit: zero, Credit: payment.Total, OpenItem: true, PartyRef: payment.PartyRef}}
	openIndex := 1
	if payment.Direction == "disbursement" {
		lines = []domain.PostingLine{{AccountCode: debit, Debit: payment.Total, Credit: zero, OpenItem: true, PartyRef: payment.PartyRef}, {AccountCode: credit, Debit: zero, Credit: payment.Total}}
		openIndex = 0
	}
	if payment.SnapshotDigest == "" {
		return fmt.Errorf("payment %s has no persisted snapshot digest", payment.ID)
	}
	sourceVersion := persistedSourceVersion(payment.Origin)
	command := domain.PostingCommand{
		CommandID: accountingCommandID("payment", payment.ID, sourceVersion), OrganizationID: payment.OrganizationID,
		IdempotencyKey: internalIdempotencyKey(payment.OrganizationID, "accounting.post", payment.ID, sourceVersion),
		SourceType:     "payment_" + payment.Direction, SourceID: payment.ID, SourceVersion: sourceVersion,
		SnapshotDigest: payment.SnapshotDigest, CorrelationID: persistedCorrelationID(payment.Origin, payment.CorrelationID),
		EffectiveAt: payment.CreatedAt, Description: "Pago " + payment.ID, Lines: lines,
	}
	result, err := w.Accounting.Post(ctx, command)
	if err != nil {
		if errors.Is(err, domain.ErrPeriodLocked) {
			return w.recordAccountingPeriodLocked(
				ctx, event, "payment", payment.ID, "posting",
				command, command.EffectiveAt, command.SnapshotDigest,
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
		return fmt.Errorf("accounting rejected payment")
	}
	if len(result.OpenItemIDs) != 1 {
		return fmt.Errorf("accounting payment returned %d open items (line %d)", len(result.OpenItemIDs), openIndex)
	}
	return w.Store.MarkPaymentPosted(ctx, payment, result)
}

func (w DurableWorker) postPurchase(ctx context.Context, event domain.Event) error {
	var payload workermodels.PurchaseEvent
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	purchase, err := w.Store.GetPurchase(ctx, event.OrganizationID, payload.PurchaseID)
	if err != nil {
		return err
	}
	if err := workerhelpers.RequireEventOrganization(event.ID, event.OrganizationID, purchase.OrganizationID); err != nil {
		return err
	}
	if purchase.Status != "confirmed" {
		return nil
	}
	command, err := buildPurchasePostingCommand(purchase)
	if err != nil {
		return err
	}
	result, err := w.Accounting.Post(ctx, command)
	if err != nil {
		if errors.Is(err, domain.ErrPeriodLocked) {
			return w.recordAccountingPeriodLocked(
				ctx, event, "purchase", purchase.ID, "posting",
				command, command.EffectiveAt, command.SnapshotDigest,
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
		return fmt.Errorf("accounting rejected purchase")
	}
	if _, err := singleOpenItem(result); err != nil {
		return err
	}
	return w.Store.MarkPurchasePosted(ctx, purchase, result)
}

func (w DurableWorker) applyOpenItem(ctx context.Context, event domain.Event) error {
	var payload workermodels.AccountingApplicationEvent
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	value, err := w.Store.GetAccountingApplication(ctx, event.OrganizationID, payload.ApplicationID)
	if err != nil {
		return err
	}
	if err := workerhelpers.RequireEventOrganization(event.ID, event.OrganizationID, value.OrganizationID); err != nil {
		return err
	}
	if value.Status != "pending" {
		return nil
	}
	sourceVersion := persistedSourceVersion(value.Origin)
	snapshotDigest := value.SnapshotDigest
	if snapshotDigest == "" {
		snapshotDigest = commandSnapshotDigest(struct {
			ID, DebitOpenItemID, CreditOpenItemID, Amount, Currency string
		}{value.ID, value.DebitOpenItemID, value.CreditOpenItemID, value.Amount.Amount, value.Amount.Currency})
	}
	command := domain.AccountingApplicationCommand{
		CommandID: value.ID, OrganizationID: value.OrganizationID, DebitOpenItemID: value.DebitOpenItemID,
		CreditOpenItemID: value.CreditOpenItemID, Amount: value.Amount, AppliedAt: event.AvailableAt,
		IdempotencyKey: internalIdempotencyKey(value.OrganizationID, "accounting.apply", value.ID, sourceVersion),
		SourceVersion:  sourceVersion, SnapshotDigest: snapshotDigest,
		CorrelationID: persistedCorrelationID(value.Origin, value.CorrelationID),
	}
	result, err := w.Accounting.ApplyOpenItem(ctx, command)
	if err != nil {
		if errors.Is(err, domain.ErrPeriodLocked) {
			return w.recordAccountingPeriodLocked(
				ctx, event, "accounting_application", value.ID, "application",
				command, command.AppliedAt, command.SnapshotDigest,
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
		return fmt.Errorf("accounting rejected open-item application")
	}
	return w.Store.MarkAccountingApplicationApplied(ctx, value, result)
}

func (w DurableWorker) reverseAccounting(ctx context.Context, event domain.Event) error {
	var payload struct {
		ReversalID string `json:"reversal_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	value, err := w.Store.GetAccountingReversal(ctx, event.OrganizationID, payload.ReversalID)
	if err != nil {
		return err
	}
	if err := workerhelpers.RequireEventOrganization(event.ID, event.OrganizationID, value.OrganizationID); err != nil {
		return err
	}
	if value.Status != "requested" {
		return nil
	}
	applications, err := w.Store.ListAppliedAccountingApplications(ctx, value.OrganizationID, value.DocumentKind, value.DocumentID)
	if err != nil {
		return err
	}
	sourceVersion := persistedSourceVersion(value.Origin)
	correlationID := persistedCorrelationID(value.Origin, value.CorrelationID)
	for _, application := range applications {
		if application.OrganizationID != value.OrganizationID {
			return fmt.Errorf(
				"ACCOUNTING_APPLICATION_ORGANIZATION_MISMATCH: reversal %s application %s",
				value.ID,
				application.ID,
			)
		}
		commandID := accountingCommandID("application-reversal", application.ID, sourceVersion)
		snapshotDigest := commandSnapshotDigest(struct {
			ApplicationID, Reason string
			ReversedAt            time.Time
		}{application.ApplicationID, value.Reason, value.EffectiveAt})
		command := domain.AccountingApplicationReversalCommand{
			CommandID: commandID, OrganizationID: value.OrganizationID,
			IdempotencyKey: internalIdempotencyKey(value.OrganizationID, "accounting.reverse-application", application.ID, sourceVersion),
			SourceVersion:  sourceVersion, SnapshotDigest: snapshotDigest,
			ApplicationID: application.ApplicationID, ReversedAt: value.EffectiveAt,
			Reason: value.Reason, CorrelationID: correlationID,
		}
		result, err := w.Accounting.ReverseOpenItemApplication(ctx, command)
		if err != nil {
			if errors.Is(err, domain.ErrPeriodLocked) {
				return w.recordAccountingPeriodLocked(
					ctx, event, "accounting_reversal", value.ID,
					"application_reversal", command, command.ReversedAt,
					command.SnapshotDigest,
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
			return fmt.Errorf("accounting rejected application reversal")
		}
		if err := w.Store.MarkAccountingApplicationReversed(ctx, application, result); err != nil {
			return err
		}
	}
	snapshotDigest := value.SnapshotDigest
	if snapshotDigest == "" {
		snapshotDigest = commandSnapshotDigest(struct {
			ID, JournalEntryID, Reason string
			EffectiveAt                time.Time
		}{value.ID, value.OriginalJournalEntryID, value.Reason, value.EffectiveAt})
	}
	command := domain.ReversalCommand{
		CommandID: accountingCommandID("journal-reversal", value.ID, sourceVersion), OrganizationID: value.OrganizationID,
		IdempotencyKey: internalIdempotencyKey(value.OrganizationID, "accounting.reverse", value.ID, sourceVersion),
		SourceVersion:  sourceVersion, SnapshotDigest: snapshotDigest,
		OriginalJournalEntryID: value.OriginalJournalEntryID, EffectiveAt: value.EffectiveAt,
		Reason: value.Reason, CorrelationID: correlationID,
	}
	result, err := w.Accounting.Reverse(ctx, command)
	if err != nil {
		if errors.Is(err, domain.ErrPeriodLocked) {
			return w.recordAccountingPeriodLocked(
				ctx, event, "accounting_reversal", value.ID, "reversal",
				command, command.EffectiveAt, command.SnapshotDigest,
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
		return fmt.Errorf("accounting rejected journal reversal")
	}
	return w.Store.MarkAccountingReversalCompleted(ctx, value, result)
}

func accountingCommandID(operation, sourceID string, version int) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("pymes-v3:%s:%s:%d", operation, sourceID, version))).String()
}

func singleOpenItem(result domain.AccountingEvent) (string, error) {
	if len(result.OpenItemIDs) != 1 {
		return "", fmt.Errorf("accounting returned %d open items", len(result.OpenItemIDs))
	}
	return result.OpenItemIDs[0], nil
}
