package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
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

func idempotencyCommand(t *testing.T, key, organizationID, operation, sourceID string, sourceVersion int, payload any) domain.IdempotencyCommand {
	t.Helper()
	canonical, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	return domain.IdempotencyCommand{
		Key: key, OrganizationID: organizationID, Operation: operation,
		SourceID: sourceID, SourceVersion: sourceVersion,
		PayloadHash: hex.EncodeToString(digest[:]),
	}
}

func TestOriginMetadataNormalizationAndInternalIdempotencyAreDeterministic(t *testing.T) {
	explicit := domain.OriginMetadata{
		RequestID:     " request-7 ",
		CorrelationID: " correlation-7 ",
		ActorRef:      " actor-7 ",
		SourceVersion: 7,
	}
	normalized := normalizeOrigin(explicit, "fallback", "accounting.post", "purchase-7")
	if normalized.RequestID != "request-7" ||
		normalized.CorrelationID != "correlation-7" ||
		normalized.ActorRef != "actor-7" ||
		normalized.SourceVersion != 7 {
		t.Fatalf("explicit origin was not normalized exactly: %+v", normalized)
	}

	first := normalizeOrigin(domain.OriginMetadata{}, "", "accounting.post", "purchase-legacy")
	second := normalizeOrigin(domain.OriginMetadata{}, "", "accounting.post", "purchase-legacy")
	if first != second || first.RequestID == "" || first.CorrelationID == "" ||
		first.ActorRef != "system:internal" || first.SourceVersion != 1 {
		t.Fatalf("legacy origin is not complete and deterministic: first=%+v second=%+v", first, second)
	}

	key := internalIdempotencyKey("org-a", "accounting.post", "purchase-7", 7)
	if key != internalIdempotencyKey("org-a", "accounting.post", "purchase-7", 7) ||
		key == internalIdempotencyKey("org-b", "accounting.post", "purchase-7", 7) ||
		key == internalIdempotencyKey("org-a", "accounting.reverse", "purchase-7", 7) ||
		key == internalIdempotencyKey("org-a", "accounting.post", "purchase-7", 8) {
		t.Fatalf("internal idempotency key is not scoped by org, operation, source and version: %q", key)
	}
}

func TestPublicCommandsAreTransactionallyIdempotentAndTenantAware(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `
		TRUNCATE app.idempotency_records,app.outbox,app.accounting_reversals,
		         app.accounting_application_commands,app.open_item_applications,
		         app.payments,app.purchases,app.sales,app.parties,app.organizations
		CASCADE`); err != nil {
		t.Fatal(err)
	}
	for _, organization := range []struct{ id, name, slug string }{
		{id: "org_idempotency_a", name: "Idempotency A", slug: "idempotency-a"},
		{id: "org_idempotency_b", name: "Idempotency B", slug: "idempotency-b"},
	} {
		if _, err := createReadyOrganization(t, pool, organization.id, organization.name, organization.slug); err != nil {
			t.Fatal(err)
		}
	}
	store := New(pool)
	store.Now = func() time.Time { return time.Date(2026, 7, 31, 15, 0, 0, 0, time.UTC) }

	partyA := domain.Party{ID: "shared_party", OrganizationID: "org_idempotency_a", Kind: "customer", DisplayName: "Alice"}
	commandPartyA := idempotencyCommand(t, "party-request", partyA.OrganizationID, domain.OperationCreateParty, partyA.ID, 1, partyA)
	firstParty, err := store.CreatePartyIdempotent(ctx, commandPartyA, partyA)
	if err != nil {
		t.Fatal(err)
	}
	replayedParty, err := store.CreatePartyIdempotent(ctx, commandPartyA, partyA)
	if err != nil || replayedParty != firstParty {
		t.Fatalf("party replay=%+v first=%+v err=%v", replayedParty, firstParty, err)
	}
	changedParty := partyA
	changedParty.DisplayName = "Changed"
	changedCommand := idempotencyCommand(t, commandPartyA.Key, partyA.OrganizationID, commandPartyA.Operation, partyA.ID, 1, changedParty)
	if _, err := store.CreatePartyIdempotent(ctx, changedCommand, changedParty); !errors.Is(err, domain.ErrIdempotencyKeyReused) {
		t.Fatalf("changed payload error=%v", err)
	}
	differentSource := partyA
	differentSource.ID = "different_party"
	reusedKey := idempotencyCommand(t, commandPartyA.Key, partyA.OrganizationID, domain.OperationCreateParty, differentSource.ID, 1, differentSource)
	if _, err := store.CreatePartyIdempotent(ctx, reusedKey, differentSource); !errors.Is(err, domain.ErrIdempotencyKeyReused) {
		t.Fatalf("reused public key error=%v", err)
	}
	partyB := partyA
	partyB.OrganizationID, partyB.DisplayName = "org_idempotency_b", "Bob"
	commandPartyB := idempotencyCommand(t, commandPartyA.Key, partyB.OrganizationID, domain.OperationCreateParty, partyB.ID, 1, partyB)
	if _, err := store.CreatePartyIdempotent(ctx, commandPartyB, partyB); err != nil {
		t.Fatalf("same tenant-local ID in second org: %v", err)
	}
	actualPartyA, err := store.GetParty(ctx, partyA.OrganizationID, partyA.ID)
	if err != nil || actualPartyA.OrganizationID != partyA.OrganizationID || actualPartyA.DisplayName != partyA.DisplayName {
		t.Fatalf("party A=%+v err=%v", actualPartyA, err)
	}
	actualPartyB, err := store.GetParty(ctx, partyB.OrganizationID, partyB.ID)
	if err != nil || actualPartyB.OrganizationID != partyB.OrganizationID || actualPartyB.DisplayName != partyB.DisplayName {
		t.Fatalf("party B=%+v err=%v", actualPartyB, err)
	}

	saleA := domain.Sale{
		ID: "shared_sale", OrganizationID: partyA.OrganizationID, RecipientRef: partyA.ID,
		Voucher:           domain.VoucherReference{PointOfSale: 1, DocumentType: "FA"},
		FiscalEnvironment: "homologation", Total: domain.Money{Amount: "121.00", Currency: "ARS"},
		Status: domain.SaleFiscalPending, FiscalSnapshot: json.RawMessage(`{"environment":"homologation"}`),
		CorrelationID: "sale:shared_sale",
	}
	commandSaleA := idempotencyCommand(t, "sale-request", saleA.OrganizationID, domain.OperationCreateSale, saleA.ID, 1, saleA)
	firstSale, err := store.CreateSaleAndQueueFiscalIdempotent(ctx, commandSaleA, saleA, "mock://credential")
	if err != nil {
		t.Fatal(err)
	}
	replayedSale, err := store.CreateSaleAndQueueFiscalIdempotent(ctx, commandSaleA, saleA, "mock://credential")
	if err != nil || replayedSale.Voucher.VoucherNumber != firstSale.Voucher.VoucherNumber ||
		replayedSale.SnapshotDigest != firstSale.SnapshotDigest {
		t.Fatalf("sale replay=%+v first=%+v err=%v", replayedSale, firstSale, err)
	}
	saleB := saleA
	saleB.OrganizationID, saleB.RecipientRef = partyB.OrganizationID, partyB.ID
	commandSaleB := idempotencyCommand(t, commandSaleA.Key, saleB.OrganizationID, domain.OperationCreateSale, saleB.ID, 1, saleB)
	if _, err := store.CreateSaleAndQueueFiscalIdempotent(ctx, commandSaleB, saleB, "mock://credential"); err != nil {
		t.Fatalf("same sale ID in second org: %v", err)
	}
	actualSaleA, err := store.GetSale(ctx, saleA.OrganizationID, saleA.ID)
	if err != nil || actualSaleA.OrganizationID != saleA.OrganizationID || actualSaleA.RecipientRef != saleA.RecipientRef {
		t.Fatalf("sale A=%+v err=%v", actualSaleA, err)
	}
	actualSaleB, err := store.GetSale(ctx, saleB.OrganizationID, saleB.ID)
	if err != nil || actualSaleB.OrganizationID != saleB.OrganizationID || actualSaleB.RecipientRef != saleB.RecipientRef {
		t.Fatalf("sale B=%+v err=%v", actualSaleB, err)
	}

	purchase := domain.Purchase{
		ID: "purchase_idempotent", OrganizationID: partyA.OrganizationID, SupplierRef: "supplier_1",
		ExternalDocumentRef: "invoice_1", Total: domain.Money{Amount: "100.00", Currency: "ARS"},
		IssueDate: "2026-07-31", NetAmount: "0", ExemptAmount: "100.00",
		SnapshotDigest: strings.Repeat("b", 64), CorrelationID: "purchase:purchase_idempotent",
	}
	commandPurchase := idempotencyCommand(t, "purchase-request", purchase.OrganizationID, domain.OperationCreatePurchase, purchase.ID, 3, purchase)
	firstPurchase, err := store.CreatePurchaseAndQueueIdempotent(ctx, commandPurchase, purchase)
	if err != nil {
		t.Fatal(err)
	}
	replayedPurchase, err := store.CreatePurchaseAndQueueIdempotent(ctx, commandPurchase, purchase)
	if err != nil || replayedPurchase.ID != firstPurchase.ID || replayedPurchase.Status != "confirmed" {
		t.Fatalf("purchase replay=%+v first=%+v err=%v", replayedPurchase, firstPurchase, err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE app.purchases
		SET status='posted',journal_entry_id='journal_purchase',open_item_id='open_purchase'
		WHERE org_id=$1 AND id=$2`, purchase.OrganizationID, purchase.ID); err != nil {
		t.Fatal(err)
	}

	payment := domain.Payment{
		ID: "payment_idempotent", OrganizationID: partyA.OrganizationID, Direction: "disbursement",
		PartyRef: purchase.SupplierRef, Total: domain.Money{Amount: "50.00", Currency: "ARS"},
		CorrelationID: "payment:payment_idempotent",
	}
	applications := []domain.OpenItemApplication{{
		ID: "application_idempotent", PaymentID: payment.ID, DocumentKind: "purchase",
		DocumentID: purchase.ID, Amount: payment.Total,
	}}
	paymentPayload := struct {
		Payment      domain.Payment
		Applications []domain.OpenItemApplication
	}{Payment: payment, Applications: applications}
	commandPayment := idempotencyCommand(t, "payment-request", payment.OrganizationID, domain.OperationCreatePayment, payment.ID, 1, paymentPayload)
	firstPayment, err := store.CreatePaymentAndApplicationsIdempotent(ctx, commandPayment, payment, applications)
	if err != nil {
		t.Fatal(err)
	}
	replayedPayment, err := store.CreatePaymentAndApplicationsIdempotent(ctx, commandPayment, payment, applications)
	if err != nil || replayedPayment.ID != firstPayment.ID || replayedPayment.Status != "confirmed" {
		t.Fatalf("payment replay=%+v first=%+v err=%v", replayedPayment, firstPayment, err)
	}

	reversal := domain.AccountingReversal{
		ID: "reversal_idempotent", OrganizationID: purchase.OrganizationID,
		DocumentKind: "purchase", DocumentID: purchase.ID,
		EffectiveAt: store.Now().UTC(), Reason: "supplier cancellation",
		CorrelationID: "reversal:reversal_idempotent",
	}
	commandReversal := idempotencyCommand(t, "reversal-request", reversal.OrganizationID, domain.OperationCreateAccountingReversal, reversal.ID, 1, reversal)
	firstReversal, err := store.CreateAccountingReversalIdempotent(ctx, commandReversal, reversal)
	if err != nil {
		t.Fatal(err)
	}
	replayedReversal, err := store.CreateAccountingReversalIdempotent(ctx, commandReversal, reversal)
	if err != nil || replayedReversal.ID != firstReversal.ID ||
		replayedReversal.OriginalJournalEntryID != firstReversal.OriginalJournalEntryID {
		t.Fatalf("reversal replay=%+v first=%+v err=%v", replayedReversal, firstReversal, err)
	}

	var parties, sales, fiscalEvents, purchaseEvents, paymentEvents, records int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.parties WHERE id='shared_party'`).Scan(&parties); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.sales WHERE id='shared_sale'`).Scan(&sales); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.outbox WHERE org_id=$1 AND topic='FiscalAuthorizationRequested'`, partyA.OrganizationID).Scan(&fiscalEvents); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.outbox WHERE org_id=$1 AND topic='PurchasePostingRequested'`, partyA.OrganizationID).Scan(&purchaseEvents); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.outbox WHERE org_id=$1 AND topic='PaymentPostingRequested'`, partyA.OrganizationID).Scan(&paymentEvents); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.idempotency_records WHERE org_id=$1`, partyA.OrganizationID).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if parties != 2 || sales != 2 || fiscalEvents != 1 || purchaseEvents != 1 ||
		paymentEvents != 1 || records != 5 {
		t.Fatalf("parties=%d sales=%d fiscal=%d purchase=%d payment=%d records=%d", parties, sales, fiscalEvents, purchaseEvents, paymentEvents, records)
	}
}

func TestConcurrentPublicRetriesConvergeToOneMutation(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `TRUNCATE app.idempotency_records,app.parties,app.organizations CASCADE`); err != nil {
		t.Fatal(err)
	}
	organization, err := createReadyOrganization(t, pool, "org_concurrent_idempotency", "Concurrent", "concurrent-idempotency")
	if err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	party := domain.Party{ID: "party_concurrent", OrganizationID: organization.ID, Kind: "customer", DisplayName: "Concurrent"}
	command := idempotencyCommand(t, "concurrent-party-request", organization.ID, domain.OperationCreateParty, party.ID, 1, party)

	const attempts = 12
	start := make(chan struct{})
	failures := make(chan error, attempts)
	results := make(chan domain.Party, attempts)
	var wait sync.WaitGroup
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			result, err := store.CreatePartyIdempotent(ctx, command, party)
			if err != nil {
				failures <- err
				return
			}
			results <- result
		}()
	}
	close(start)
	wait.Wait()
	close(failures)
	close(results)
	for err := range failures {
		t.Fatal(err)
	}
	count := 0
	for result := range results {
		count++
		if result != party {
			t.Fatalf("replayed result=%+v", result)
		}
	}
	if count != attempts {
		t.Fatalf("successful attempts=%d", count)
	}
	var parties, records int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.parties WHERE org_id=$1 AND id=$2`, organization.ID, party.ID).Scan(&parties); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.idempotency_records WHERE org_id=$1 AND operation=$2 AND source_id=$3`, organization.ID, command.Operation, command.SourceID).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if parties != 1 || records != 1 {
		t.Fatalf("parties=%d records=%d", parties, records)
	}
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
	sale := domain.Sale{
		ID: "sale_a", OrganizationID: organization.ID, RecipientRef: "party_a",
		Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 1},
		Total:   domain.Money{Amount: "121.00", Currency: "ARS"}, Status: domain.SaleFiscalPending,
		CorrelationID: "correlation_a",
		Origin: domain.OriginMetadata{
			RequestID: "request_a", CorrelationID: "correlation_a",
			ActorRef: "user_a", SourceVersion: 7,
		},
	}
	created, err := store.CreateSaleAndQueueFiscal(context.Background(), sale, "kms://credential/a")
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.Lease(context.Background(), 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	expectedKey := internalIdempotencyKey("org_a", "fiscal.authorize", "sale_a", 7)
	if len(events) != 1 ||
		events[0].OrganizationID != "org_a" ||
		events[0].Topic != "FiscalAuthorizationRequested" ||
		events[0].RequestID != "request_a" ||
		events[0].ActorRef != "user_a" ||
		events[0].SourceVersion != 7 ||
		events[0].SnapshotDigest != created.SnapshotDigest ||
		events[0].CorrelationID != "correlation_a" ||
		events[0].IdempotencyKey != expectedKey {
		t.Fatalf("events=%+v", events)
	}
	if err := store.MarkPublished(context.Background(), events[0]); err != nil {
		t.Fatal(err)
	}
}

func TestStoreMovesExhaustedEventToDeadLetter(t *testing.T) {
	url := os.Getenv("PYMES_DATABASE_TEST_URL")
	if url == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "TRUNCATE app.outbox_dead_letters, app.outbox, app.sales, app.organizations CASCADE"); err != nil {
		t.Fatal(err)
	}
	organization, err := createReadyOrganization(t, pool, "org_dead_letter", "Dead letter", "dead-letter")
	if err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	store.Now = func() time.Time { return time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC) }
	if _, err := store.CreateSaleAndQueueFiscal(ctx, domain.Sale{
		ID: "sale_dead_letter", OrganizationID: organization.ID, RecipientRef: "party",
		Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 1},
		Total:   domain.Money{Amount: "121", Currency: "ARS"}, Status: domain.SaleFiscalPending,
		SnapshotDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", CorrelationID: "dead-letter",
	}, "mock://credential"); err != nil {
		t.Fatal(err)
	}
	events, err := store.Lease(ctx, 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("leased events = %d", len(events))
	}
	if err := store.DeadLetter(ctx, events[0], "DELIVERY_FAILED"); err != nil {
		t.Fatal(err)
	}
	var active, deadLetters, attempts, sourceVersion int
	var failureCode, requestID, actorRef, snapshotDigest string
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM app.outbox").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*),COALESCE(max(attempts),0),COALESCE(max(failure_code),''),
		       COALESCE(max(request_id),''),COALESCE(max(actor_ref),''),
		       COALESCE(max(source_version),0),COALESCE(max(snapshot_digest),'')
		FROM app.outbox_dead_letters`,
	).Scan(
		&deadLetters, &attempts, &failureCode, &requestID, &actorRef,
		&sourceVersion, &snapshotDigest,
	); err != nil {
		t.Fatal(err)
	}
	if active != 0 || deadLetters != 1 || attempts != 1 || failureCode != "DELIVERY_FAILED" ||
		requestID != events[0].RequestID || actorRef != events[0].ActorRef ||
		sourceVersion != events[0].SourceVersion || snapshotDigest != events[0].SnapshotDigest {
		t.Fatalf(
			"active=%d dead_letters=%d attempts=%d failure_code=%q request_id=%q actor_ref=%q source_version=%d snapshot_digest=%q event=%+v",
			active, deadLetters, attempts, failureCode, requestID, actorRef,
			sourceVersion, snapshotDigest, events[0],
		)
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
	sale := domain.Sale{ID: "sale_worker", OrganizationID: organization.ID, RecipientRef: "party", Voucher: domain.VoucherReference{PointOfSale: 1, DocumentType: "FA", VoucherNumber: 2}, Total: domain.Money{Amount: "121.00", Currency: "ARS"}, Status: domain.SaleFiscalPending, SnapshotDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CorrelationID: "worker-test", FiscalSnapshot: []byte(`{"issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"21","exempt":"0","total":"121"}}`)}
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
		FiscalSnapshot: []byte(`{"issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"21","exempt":"0","total":"121"}}`),
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
		FiscalSnapshot: []byte(`{"issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"21","exempt":"0","total":"121"}}`),
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
		FiscalSnapshot: []byte(`{"issue_date":"2026-07-31","currency":"ARS","totals":{"net":"17.36","vat":"3.64","exempt":"0","total":"21"},"associated_voucher":{"point_of_sale":1,"document_type":"FA","voucher_number":20,"issue_date":"2026-07-31"}}`),
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
