package commerce

import (
	"context"
	"github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestDurableWorkerPostsPaysAndReversesPurchase(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, "TRUNCATE app.outbox, app.accounting_reversals, app.accounting_application_commands, app.open_item_applications, app.payments, app.purchases, app.sales, app.organizations CASCADE"); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	store := New(pool)
	store.Now = func() time.Time { return now }
	organization, err := createReadyOrganization(t, pool, "org_purchase_flow", "Purchase Flow", "purchase-flow")
	if err != nil {
		t.Fatal(err)
	}
	worker := DurableWorker{
		Store: store, Fiscal: NewFakeFiscal(),
		Accounting: NewFakeAccounting(), LeaseFor: time.Minute,
	}

	purchase := domain.Purchase{
		ID: "purchase_flow", OrganizationID: organization.ID,
		SupplierRef: "supplier_flow", ExternalDocumentRef: "FC-A-0001-00000001",
		IssueDate: "2026-07-31",
		Total:     domain.Money{Amount: "242.00", Currency: "ARS"}, Status: "confirmed",
		NetAmount: "200.00", ExemptAmount: "0",
		VATBreakdown: []domain.VATBreakdownItem{{
			Rate: "21", BaseAmount: "200.00", TaxAmount: "42.00",
		}},
		SnapshotDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CorrelationID:  "purchase-flow", CreatedAt: now,
	}
	if err = store.CreatePurchaseAndQueue(ctx, purchase); err != nil {
		t.Fatal(err)
	}
	if err = worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	actualPurchase, err := store.GetPurchase(ctx, organization.ID, purchase.ID)
	if err != nil || actualPurchase.Status != "posted" || actualPurchase.JournalEntryID == "" || actualPurchase.OpenItemID == "" {
		t.Fatalf("posted purchase=%+v err=%v", actualPurchase, err)
	}

	payment := domain.Payment{
		ID: "purchase_payment", OrganizationID: organization.ID,
		Direction: "disbursement", PartyRef: purchase.SupplierRef,
		Total: domain.Money{Amount: "100.00", Currency: "ARS"}, Status: "confirmed",
		CorrelationID: "purchase-payment", CreatedAt: now,
	}
	application := domain.OpenItemApplication{
		ID: "purchase_application", PaymentID: payment.ID, DocumentKind: "purchase",
		DocumentID: purchase.ID, Amount: payment.Total,
	}
	if err = store.CreatePaymentAndApplications(ctx, payment, []domain.OpenItemApplication{application}); err != nil {
		t.Fatal(err)
	}
	if err = worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err = worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	actualPurchase, err = store.GetPurchase(ctx, organization.ID, purchase.ID)
	if err != nil || actualPurchase.Status != "partially_paid" {
		t.Fatalf("partially paid purchase=%+v err=%v", actualPurchase, err)
	}
	actualPayment, err := store.GetPayment(ctx, organization.ID, payment.ID)
	if err != nil || actualPayment.Status != "posted" || actualPayment.JournalEntryID == "" || actualPayment.OpenItemID == "" {
		t.Fatalf("posted payment=%+v err=%v", actualPayment, err)
	}

	if _, err = store.CreateAccountingReversal(ctx, domain.AccountingReversal{
		ID: "purchase_payment_reversal", OrganizationID: organization.ID,
		DocumentKind: "payment", DocumentID: payment.ID, EffectiveAt: now,
		Reason: "supplier payment returned", CorrelationID: "purchase-payment-reversal",
	}); err != nil {
		t.Fatal(err)
	}
	if err = worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	actualPayment, err = store.GetPayment(ctx, organization.ID, payment.ID)
	if err != nil || actualPayment.Status != "reversed" {
		t.Fatalf("reversed payment=%+v err=%v", actualPayment, err)
	}
	actualPurchase, err = store.GetPurchase(ctx, organization.ID, purchase.ID)
	if err != nil || actualPurchase.Status != "posted" {
		t.Fatalf("reopened purchase=%+v err=%v", actualPurchase, err)
	}

	if _, err = store.CreateAccountingReversal(ctx, domain.AccountingReversal{
		ID: "purchase_reversal", OrganizationID: organization.ID,
		DocumentKind: "purchase", DocumentID: purchase.ID, EffectiveAt: now,
		Reason: "supplier document voided", CorrelationID: "purchase-reversal",
	}); err != nil {
		t.Fatal(err)
	}
	if err = worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	actualPurchase, err = store.GetPurchase(ctx, organization.ID, purchase.ID)
	if err != nil || actualPurchase.Status != "reversed" {
		t.Fatalf("reversed purchase=%+v err=%v", actualPurchase, err)
	}
}
