package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	"github.com/google/uuid"
)

// RelayStore is the persistence port for the commerce outbox relay. PostgreSQL
// is an adapter of this port; the workflow itself has no pgx dependency.
type RelayStore interface {
	Lease(context.Context, int, time.Duration) ([]domain.Event, error)
	Retry(context.Context, domain.Event) error
	MarkPublished(context.Context, domain.Event) error
	GetSale(context.Context, string, string) (domain.Sale, error)
	ApplyFiscalResult(context.Context, domain.Sale, domain.FiscalResult) error
	MarkSalePosted(context.Context, string, string, string, string) error
	GetPurchase(context.Context, string, string) (domain.Purchase, error)
	MarkPurchasePosted(context.Context, string, string, string, string) error
	GetPayment(context.Context, string, string) (domain.Payment, error)
	MarkPaymentPosted(context.Context, string, string, string, string) error
	GetAccountingApplication(context.Context, string, string) (domain.PendingAccountingApplication, error)
	MarkAccountingApplicationApplied(context.Context, string, string, string) error
	ListAppliedAccountingApplications(context.Context, string, string, string) ([]domain.PendingAccountingApplication, error)
	MarkAccountingApplicationReversed(context.Context, string, string) error
	GetAccountingReversal(context.Context, string, string) (domain.AccountingReversal, error)
	MarkAccountingReversalCompleted(context.Context, domain.AccountingReversal, string) error
	ListUncertainSales(context.Context, int) ([]domain.PendingFiscal, error)
}

// DurableWorker relays commerce commands at-least-once. Private services must
// treat a repeated command identifier as a duplicate.
type DurableWorker struct {
	Store      RelayStore
	Fiscal     domain.FiscalClient
	Accounting domain.AccountingClient
	LeaseFor   time.Duration
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
		if err := w.handle(ctx, event); err != nil {
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
	var payload struct {
		SaleID        string `json:"sale_id"`
		CredentialRef string `json:"credential_ref"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	sale, err := w.Store.GetSale(ctx, event.OrganizationID, payload.SaleID)
	if err != nil {
		return err
	}
	switch event.Topic {
	case "FiscalAuthorizationRequested":
		if sale.Status != domain.SaleFiscalPending && sale.Status != domain.SaleFiscalUncertain {
			return nil
		}
		result, err := w.Fiscal.Authorize(ctx, domain.FiscalRequest{RequestID: "fiscal:" + sale.ID + ":1", OrganizationID: sale.OrganizationID, CredentialRef: payload.CredentialRef, Voucher: sale.Voucher, Total: sale.Total, SnapshotDigest: sale.SnapshotDigest, CorrelationID: sale.CorrelationID, FiscalSnapshot: sale.FiscalSnapshot})
		if err != nil {
			return err
		}
		return w.Store.ApplyFiscalResult(ctx, sale, result)
	case "AccountingPostingRequested":
		if sale.Status != domain.SaleAuthorizedPendingPosting {
			return nil
		}
		zero := domain.Money{Amount: "0", Currency: sale.Total.Currency}
		lines := []domain.PostingLine{
			{AccountCode: "1200", Debit: sale.Total, Credit: zero, OpenItem: true, PartyRef: sale.RecipientRef},
			{AccountCode: "4100", Debit: zero, Credit: sale.Total},
		}
		sourceType := "sales_invoice"
		if strings.HasPrefix(sale.Voucher.DocumentType, "NC") {
			sourceType = "sales_credit_note"
			lines = []domain.PostingLine{
				{AccountCode: "4100", Debit: sale.Total, Credit: zero},
				{AccountCode: "1200", Debit: zero, Credit: sale.Total, OpenItem: true, PartyRef: sale.RecipientRef},
			}
		} else if strings.HasPrefix(sale.Voucher.DocumentType, "ND") {
			sourceType = "sales_debit_note"
		}
		command := domain.PostingCommand{CommandID: accountingCommandID("posting", sale.ID, 1), OrganizationID: sale.OrganizationID, SourceType: sourceType, SourceID: sale.ID, SourceVersion: 1, SnapshotDigest: sale.SnapshotDigest, CorrelationID: sale.CorrelationID, EffectiveAt: sale.CreatedAt, Description: "Venta " + sale.ID, Lines: lines}
		result, err := w.Accounting.Post(ctx, command)
		if err != nil {
			return err
		}
		if result.Status != "posted" && result.Status != "duplicate" {
			return fmt.Errorf("accounting rejected command")
		}
		openItemID, err := singleOpenItem(result)
		if err != nil {
			return err
		}
		return w.Store.MarkSalePosted(ctx, sale.OrganizationID, sale.ID, result.JournalEntryID, openItemID)
	default:
		return fmt.Errorf("unknown outbox topic %q", event.Topic)
	}
}

func (w DurableWorker) postPayment(ctx context.Context, event domain.Event) error {
	var payload struct {
		PaymentID string `json:"payment_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	payment, err := w.Store.GetPayment(ctx, event.OrganizationID, payload.PaymentID)
	if err != nil {
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
	command := domain.PostingCommand{CommandID: accountingCommandID("payment", payment.ID, 1), OrganizationID: payment.OrganizationID, SourceType: "payment_" + payment.Direction, SourceID: payment.ID, SourceVersion: 1, SnapshotDigest: hashPayment(payment), CorrelationID: payment.CorrelationID, EffectiveAt: payment.CreatedAt, Description: "Pago " + payment.ID, Lines: lines}
	result, err := w.Accounting.Post(ctx, command)
	if err != nil {
		return err
	}
	if result.Status != "posted" && result.Status != "duplicate" {
		return fmt.Errorf("accounting rejected payment")
	}
	if len(result.OpenItemIDs) != 1 {
		return fmt.Errorf("accounting payment returned %d open items (line %d)", len(result.OpenItemIDs), openIndex)
	}
	return w.Store.MarkPaymentPosted(ctx, payment.OrganizationID, payment.ID, result.JournalEntryID, result.OpenItemIDs[0])
}

func (w DurableWorker) postPurchase(ctx context.Context, event domain.Event) error {
	var payload struct {
		PurchaseID string `json:"purchase_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	purchase, err := w.Store.GetPurchase(ctx, event.OrganizationID, payload.PurchaseID)
	if err != nil {
		return err
	}
	if purchase.Status != "confirmed" {
		return nil
	}
	zero := domain.Money{Amount: "0", Currency: purchase.Total.Currency}
	command := domain.PostingCommand{CommandID: accountingCommandID("purchase", purchase.ID, 1), OrganizationID: purchase.OrganizationID, SourceType: "purchase_invoice", SourceID: purchase.ID, SourceVersion: 1, SnapshotDigest: purchase.SnapshotDigest, CorrelationID: purchase.CorrelationID, EffectiveAt: purchase.CreatedAt, Description: "Compra " + purchase.ExternalDocumentRef, Lines: []domain.PostingLine{{AccountCode: "5100", Debit: purchase.Total, Credit: zero}, {AccountCode: "2100", Debit: zero, Credit: purchase.Total, OpenItem: true, PartyRef: purchase.SupplierRef}}}
	result, err := w.Accounting.Post(ctx, command)
	if err != nil {
		return err
	}
	if result.Status != "posted" && result.Status != "duplicate" {
		return fmt.Errorf("accounting rejected purchase")
	}
	openItemID, err := singleOpenItem(result)
	if err != nil {
		return err
	}
	return w.Store.MarkPurchasePosted(ctx, purchase.OrganizationID, purchase.ID, result.JournalEntryID, openItemID)
}

func (w DurableWorker) applyOpenItem(ctx context.Context, event domain.Event) error {
	var payload struct {
		ApplicationID string `json:"application_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	value, err := w.Store.GetAccountingApplication(ctx, event.OrganizationID, payload.ApplicationID)
	if err != nil {
		return err
	}
	if value.Status != "pending" {
		return nil
	}
	result, err := w.Accounting.ApplyOpenItem(ctx, domain.AccountingApplicationCommand{
		CommandID: value.ID, OrganizationID: value.OrganizationID, DebitOpenItemID: value.DebitOpenItemID,
		CreditOpenItemID: value.CreditOpenItemID, Amount: value.Amount, AppliedAt: event.AvailableAt,
		CorrelationID: value.CorrelationID,
	})
	if err != nil {
		return err
	}
	if result.Status != "applied" && result.Status != "duplicate" {
		return fmt.Errorf("accounting rejected open-item application")
	}
	return w.Store.MarkAccountingApplicationApplied(ctx, value.OrganizationID, value.ID, result.ApplicationID)
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
	if value.Status != "requested" {
		return nil
	}
	applications, err := w.Store.ListAppliedAccountingApplications(ctx, value.OrganizationID, value.DocumentKind, value.DocumentID)
	if err != nil {
		return err
	}
	for _, application := range applications {
		result, err := w.Accounting.ReverseOpenItemApplication(ctx, domain.AccountingApplicationReversalCommand{
			CommandID:      accountingCommandID("application-reversal", application.ID, 1),
			OrganizationID: value.OrganizationID, ApplicationID: application.ApplicationID,
			ReversedAt: value.EffectiveAt, Reason: value.Reason, CorrelationID: value.CorrelationID,
		})
		if err != nil {
			return err
		}
		if result.Status != "reversed" && result.Status != "duplicate" {
			return fmt.Errorf("accounting rejected application reversal")
		}
		if err := w.Store.MarkAccountingApplicationReversed(ctx, value.OrganizationID, application.ID); err != nil {
			return err
		}
	}
	result, err := w.Accounting.Reverse(ctx, domain.ReversalCommand{
		CommandID: accountingCommandID("journal-reversal", value.ID, 1), OrganizationID: value.OrganizationID,
		OriginalJournalEntryID: value.OriginalJournalEntryID, EffectiveAt: value.EffectiveAt,
		Reason: value.Reason, CorrelationID: value.CorrelationID,
	})
	if err != nil {
		return err
	}
	if result.Status != "reversed" && result.Status != "duplicate" {
		return fmt.Errorf("accounting rejected journal reversal")
	}
	return w.Store.MarkAccountingReversalCompleted(ctx, value, result.JournalEntryID)
}

func hashPayment(payment domain.Payment) string {
	value := sha256.Sum256([]byte(payment.ID + ":" + payment.Total.Amount + ":" + payment.Total.Currency + ":" + payment.Direction))
	return hex.EncodeToString(value[:])
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
