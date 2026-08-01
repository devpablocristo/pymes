package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/domain"
	organizationrepository "github.com/devpablocristo/pymes/v3/backend/internal/organization/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRejectsEveryCommerceMutationUnlessOrganizationIsReady(t *testing.T) {
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
	if _, err = pool.Exec(ctx, `
		TRUNCATE app.idempotency_records,app.outbox,app.accounting_reversals,
		         app.accounting_application_commands,app.open_item_applications,
		         app.payments,app.purchases,app.sales,app.parties,app.organizations
		CASCADE`); err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	store.Now = func() time.Time { return time.Date(2026, 7, 31, 16, 0, 0, 0, time.UTC) }

	for _, status := range []organizationdomain.Status{
		organizationdomain.Pending,
		organizationdomain.Failed,
		organizationdomain.Suspended,
	} {
		organizationID := "org_mutation_" + string(status)
		if _, err = organizationrepository.New(pool).Create(ctx, organizationdomain.Organization{
			ID: organizationID, Name: organizationID, Slug: organizationID, Status: status,
		}); err != nil {
			t.Fatal(err)
		}

		party := domain.Party{ID: "party_" + string(status), OrganizationID: organizationID, Kind: "customer", DisplayName: "Alice"}
		purchase := domain.Purchase{
			ID: "purchase_" + string(status), OrganizationID: organizationID,
			SupplierRef: "supplier_1", ExternalDocumentRef: "invoice_1",
			IssueDate:      "2026-07-31",
			Total:          domain.Money{Amount: "100.00", Currency: "ARS"},
			NetAmount:      "0",
			ExemptAmount:   "100.00",
			SnapshotDigest: strings.Repeat("b", 64), CorrelationID: "purchase:" + string(status),
		}
		payment := domain.Payment{
			ID: "payment_" + string(status), OrganizationID: organizationID,
			Direction: "receipt", PartyRef: "party_1",
			Total: domain.Money{Amount: "50.00", Currency: "ARS"}, CorrelationID: "payment:" + string(status),
		}
		sale := domain.Sale{
			ID: "sale_" + string(status), OrganizationID: organizationID, RecipientRef: "party_1",
			Voucher:           domain.VoucherReference{PointOfSale: 1, DocumentType: "FA"},
			FiscalEnvironment: "homologation", Total: domain.Money{Amount: "121.00", Currency: "ARS"},
			Status: domain.SaleFiscalPending, FiscalSnapshot: []byte(`{"environment":"homologation"}`),
			CorrelationID: "sale:" + string(status),
		}
		reversal := domain.AccountingReversal{
			ID: "reversal_" + string(status), OrganizationID: organizationID,
			DocumentKind: "purchase", DocumentID: purchase.ID, EffectiveAt: store.Now(),
			Reason: "supplier cancellation", CorrelationID: "reversal:" + string(status),
		}

		mutations := []struct {
			name string
			run  func() error
		}{
			{name: "party direct", run: func() error { _, err := store.CreateParty(ctx, party); return err }},
			{name: "party idempotent", run: func() error {
				_, err := store.CreatePartyIdempotent(ctx, idempotencyCommand(t, "party-"+string(status), organizationID, domain.OperationCreateParty, party.ID, 1, party), party)
				return err
			}},
			{name: "sale direct", run: func() error { _, err := store.CreateSaleAndQueueFiscal(ctx, sale, "mock://credential"); return err }},
			{name: "sale idempotent", run: func() error {
				_, err := store.CreateSaleAndQueueFiscalIdempotent(ctx, idempotencyCommand(t, "sale-"+string(status), organizationID, domain.OperationCreateSale, sale.ID, 1, sale), sale, "mock://credential")
				return err
			}},
			{name: "purchase direct", run: func() error { return store.CreatePurchaseAndQueue(ctx, purchase) }},
			{name: "purchase idempotent", run: func() error {
				_, err := store.CreatePurchaseAndQueueIdempotent(ctx, idempotencyCommand(t, "purchase-"+string(status), organizationID, domain.OperationCreatePurchase, purchase.ID, 1, purchase), purchase)
				return err
			}},
			{name: "payment direct", run: func() error { return store.CreatePaymentAndApplications(ctx, payment, nil) }},
			{name: "payment idempotent", run: func() error {
				_, err := store.CreatePaymentAndApplicationsIdempotent(ctx, idempotencyCommand(t, "payment-"+string(status), organizationID, domain.OperationCreatePayment, payment.ID, 1, payment), payment, nil)
				return err
			}},
			{name: "reversal direct", run: func() error { _, err := store.CreateAccountingReversal(ctx, reversal); return err }},
			{name: "reversal idempotent", run: func() error {
				_, err := store.CreateAccountingReversalIdempotent(ctx, idempotencyCommand(t, "reversal-"+string(status), organizationID, domain.OperationCreateAccountingReversal, reversal.ID, 1, reversal), reversal)
				return err
			}},
		}
		for _, mutation := range mutations {
			if err := mutation.run(); !errors.Is(err, domain.ErrOrganizationNotReady) {
				t.Fatalf("status=%s mutation=%s err=%v", status, mutation.name, err)
			}
		}

		var persisted int
		if err = pool.QueryRow(ctx, `
			SELECT
			  (SELECT count(*) FROM app.parties WHERE org_id=$1) +
			  (SELECT count(*) FROM app.sales WHERE org_id=$1) +
			  (SELECT count(*) FROM app.purchases WHERE org_id=$1) +
			  (SELECT count(*) FROM app.payments WHERE org_id=$1) +
			  (SELECT count(*) FROM app.accounting_reversals WHERE org_id=$1) +
			  (SELECT count(*) FROM app.outbox WHERE org_id=$1) +
			  (SELECT count(*) FROM app.idempotency_records WHERE org_id=$1)`, organizationID).Scan(&persisted); err != nil {
			t.Fatal(err)
		}
		if persisted != 0 {
			t.Fatalf("status=%s persisted tenant mutations=%d", status, persisted)
		}
	}
}
