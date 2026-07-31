package repository

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v3/backend/internal/commerce/companion"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	"github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/domain"
	organizationrepository "github.com/devpablocristo/pymes/v3/backend/internal/organization/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFiscalNumbersAreReservedAtomically(t *testing.T) {
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
	if _, err = createReadyOrganization(t, pool, "org_numbering", "Numbering", "numbering"); err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	const count = 12
	results := make(chan int, count)
	failures := make(chan error, count)
	var wait sync.WaitGroup
	for index := range count {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			sale, err := store.CreateSaleAndQueueFiscal(ctx, domain.Sale{
				ID: fmt.Sprintf("numbered-sale-%d", index), OrganizationID: "org_numbering", RecipientRef: "customer",
				Voucher:           domain.VoucherReference{PointOfSale: 1, DocumentType: "FA"},
				FiscalEnvironment: "homologation", Total: domain.Money{Amount: "121", Currency: "ARS"},
				Status: domain.SaleFiscalPending, FiscalSnapshot: []byte(`{"environment":"homologation"}`),
				CorrelationID: fmt.Sprintf("numbering:%d", index),
			}, "mock://credential")
			if err != nil {
				failures <- err
				return
			}
			results <- sale.Voucher.VoucherNumber
		}(index)
	}
	wait.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	numbers := make(map[int]bool, count)
	for number := range results {
		numbers[number] = true
	}
	if len(numbers) != count {
		t.Fatalf("expected %d unique voucher numbers, got %v", count, numbers)
	}
	for expected := 1; expected <= count; expected++ {
		if !numbers[expected] {
			t.Fatalf("voucher number %d was not reserved", expected)
		}
	}
}

func createReadyOrganization(t *testing.T, pool *pgxpool.Pool, id, name, slug string) (organizationdomain.Organization, error) {
	t.Helper()
	return organizationrepository.New(pool).Create(context.Background(), organizationdomain.Organization{ID: id, Name: name, Slug: slug, Status: organizationdomain.Ready})
}

func TestStorePersistsSaleAndLeasesOutbox(t *testing.T) {
	url := os.Getenv("PYMES_DATABASE_TEST_URL")
	if url == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := New(pool)
	store.Now = func() time.Time { return time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC) }
	if _, err := pool.Exec(context.Background(), "TRUNCATE app.outbox, app.sales, app.organizations CASCADE"); err != nil {
		t.Fatal(err)
	}
	organization, err := createReadyOrganization(t, pool, "org_a", "Acme", "acme")
	if err != nil {
		t.Fatal(err)
	}
	sale := domain.Sale{ID: "sale_a", OrganizationID: organization.ID, RecipientRef: "party_a", Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 1}, Total: domain.Money{Amount: "121.00", Currency: "ARS"}, Status: domain.SaleFiscalPending, SnapshotDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CorrelationID: "request_a"}
	if _, err := store.CreateSaleAndQueueFiscal(context.Background(), sale, "kms://credential/a"); err != nil {
		t.Fatal(err)
	}
	events, err := store.Lease(context.Background(), 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].OrganizationID != "org_a" || events[0].Topic != "FiscalAuthorizationRequested" {
		t.Fatalf("events=%+v", events)
	}
	if err := store.MarkPublished(context.Background(), events[0]); err != nil {
		t.Fatal(err)
	}
}

func TestDurableWorkerRecoversLostFiscalAndAccountingResponses(t *testing.T) {
	url := os.Getenv("PYMES_DATABASE_TEST_URL")
	if url == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := New(pool)
	now := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }
	if _, err := pool.Exec(context.Background(), "TRUNCATE app.outbox, app.sales, app.organizations CASCADE"); err != nil {
		t.Fatal(err)
	}
	organization, err := createReadyOrganization(t, pool, "org_worker", "Worker", "worker")
	if err != nil {
		t.Fatal(err)
	}
	sale := domain.Sale{ID: "sale_worker", OrganizationID: organization.ID, RecipientRef: "party", Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 2}, Total: domain.Money{Amount: "121.00", Currency: "ARS"}, Status: domain.SaleFiscalPending, SnapshotDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CorrelationID: "worker-test"}
	if _, err := store.CreateSaleAndQueueFiscal(context.Background(), sale, "kms://credential/worker"); err != nil {
		t.Fatal(err)
	}
	fiscal, accounting := companion.NewFakeFiscal(), companion.NewFakeAccounting()
	fiscal.LoseAfterPersist, accounting.LoseAfterPersist = true, true
	worker := usecases.DurableWorker{Store: store, Fiscal: fiscal, Accounting: accounting, LeaseFor: time.Minute}
	if err := worker.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err := worker.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The accounting client persisted before losing its response. Advance past
	// relay backoff, retry the exact command and accept its duplicate response.
	now = now.Add(2 * time.Second)
	if err := worker.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	actual, err := store.GetSale(context.Background(), organization.ID, sale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Status != domain.SalePosted || actual.JournalEntryID == "" {
		t.Fatalf("sale did not converge: %+v", actual)
	}
}

func TestDurableWorkerAppliesPartialPaymentAndReversesIt(t *testing.T) {
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
	store := New(pool)
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }
	if _, err := pool.Exec(ctx, "TRUNCATE app.outbox, app.accounting_reversals, app.accounting_application_commands, app.open_item_applications, app.payments, app.purchases, app.sales, app.organizations CASCADE"); err != nil {
		t.Fatal(err)
	}
	organization, err := createReadyOrganization(t, pool, "org_partial", "Partial", "partial")
	if err != nil {
		t.Fatal(err)
	}
	sale := domain.Sale{
		ID: "sale_partial", OrganizationID: organization.ID, RecipientRef: "party_customer",
		Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 10},
		Total:   domain.Money{Amount: "121.00", Currency: "ARS"}, Status: domain.SaleFiscalPending,
		SnapshotDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		CorrelationID:  "partial-sale", CreatedAt: now,
	}
	if _, err := store.CreateSaleAndQueueFiscal(ctx, sale, "mock://credential"); err != nil {
		t.Fatal(err)
	}
	worker := usecases.DurableWorker{Store: store, Fiscal: companion.NewFakeFiscal(), Accounting: companion.NewFakeAccounting(), LeaseFor: time.Minute}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	actualSale, err := store.GetSale(ctx, organization.ID, sale.ID)
	if err != nil || actualSale.Status != domain.SalePosted || actualSale.OpenItemID == "" {
		t.Fatalf("sale=%+v err=%v", actualSale, err)
	}

	payment := domain.Payment{
		ID: "payment_partial", OrganizationID: organization.ID, Direction: "receipt",
		PartyRef: sale.RecipientRef, Total: domain.Money{Amount: "50.00", Currency: "ARS"},
		Status: "confirmed", CorrelationID: "partial-payment", CreatedAt: now,
	}
	application := domain.OpenItemApplication{
		ID: "application_partial", PaymentID: payment.ID, DocumentKind: "sale",
		DocumentID: sale.ID, Amount: payment.Total,
	}
	if err := store.CreatePaymentAndApplications(ctx, payment, []domain.OpenItemApplication{application}); err != nil {
		t.Fatal(err)
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	actualSale, err = store.GetSale(ctx, organization.ID, sale.ID)
	if err != nil || actualSale.Status != "partially_paid" {
		t.Fatalf("partially paid sale=%+v err=%v", actualSale, err)
	}
	actualPayment, err := store.GetPayment(ctx, organization.ID, payment.ID)
	if err != nil || actualPayment.Status != "posted" || actualPayment.OpenItemID == "" {
		t.Fatalf("payment=%+v err=%v", actualPayment, err)
	}

	reversal, err := store.CreateAccountingReversal(ctx, domain.AccountingReversal{
		ID: "reversal_partial", OrganizationID: organization.ID, DocumentKind: "payment",
		DocumentID: payment.ID, EffectiveAt: now, Reason: "payment returned", CorrelationID: "reverse-payment",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reversal.Status != "requested" {
		t.Fatalf("reversal=%+v", reversal)
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	actualPayment, err = store.GetPayment(ctx, organization.ID, payment.ID)
	if err != nil || actualPayment.Status != "reversed" {
		t.Fatalf("reversed payment=%+v err=%v", actualPayment, err)
	}
	actualSale, err = store.GetSale(ctx, organization.ID, sale.ID)
	if err != nil || actualSale.Status != domain.SalePosted {
		t.Fatalf("reopened sale=%+v err=%v", actualSale, err)
	}
}

func TestDurableWorkerPostsAndAppliesCreditNote(t *testing.T) {
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
	store := New(pool)
	now := time.Date(2026, 7, 31, 13, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }
	if _, err := pool.Exec(ctx, "TRUNCATE app.outbox, app.accounting_reversals, app.accounting_application_commands, app.open_item_applications, app.payments, app.purchases, app.sales, app.organizations CASCADE"); err != nil {
		t.Fatal(err)
	}
	organization, err := createReadyOrganization(t, pool, "org_credit_note", "Credit Note", "credit-note")
	if err != nil {
		t.Fatal(err)
	}
	worker := usecases.DurableWorker{Store: store, Fiscal: companion.NewFakeFiscal(), Accounting: companion.NewFakeAccounting(), LeaseFor: time.Minute}
	original := domain.Sale{
		ID: "sale_original", OrganizationID: organization.ID, RecipientRef: "party_customer",
		Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 20},
		Total:   domain.Money{Amount: "121.00", Currency: "ARS"}, Status: domain.SaleFiscalPending,
		SnapshotDigest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		CorrelationID:  "original-sale", CreatedAt: now,
	}
	if _, err := store.CreateSaleAndQueueFiscal(ctx, original, "mock://credential"); err != nil {
		t.Fatal(err)
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	creditNote := domain.Sale{
		ID: "sale_credit_note", OrganizationID: organization.ID, RecipientRef: original.RecipientRef,
		Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "NCA", VoucherNumber: 21},
		Total:   domain.Money{Amount: "21.00", Currency: "ARS"}, Status: domain.SaleFiscalPending,
		SourceDocumentID: original.ID,
		SnapshotDigest:   "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		CorrelationID:    "credit-note", CreatedAt: now,
	}
	if _, err := store.CreateSaleAndQueueFiscal(ctx, creditNote, "mock://credential"); err != nil {
		t.Fatal(err)
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	actualOriginal, err := store.GetSale(ctx, organization.ID, original.ID)
	if err != nil || actualOriginal.Status != "partially_paid" {
		t.Fatalf("original=%+v err=%v", actualOriginal, err)
	}
	actualCredit, err := store.GetSale(ctx, organization.ID, creditNote.ID)
	if err != nil || actualCredit.Status != domain.SalePosted || actualCredit.SourceDocumentID != original.ID || actualCredit.OpenItemID == "" {
		t.Fatalf("credit note=%+v err=%v", actualCredit, err)
	}
	var applied int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.accounting_application_commands WHERE org_id=$1 AND source_kind='credit_note' AND source_id=$2 AND status='applied'`, organization.ID, creditNote.ID).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("applied commands=%d", applied)
	}
}
