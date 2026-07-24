package postgres

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	_ DBTX                     = (*pgxpool.Pool)(nil)
	_ DBTX                     = (pgx.Tx)(nil)
	_ fiscal.CommandRepository = (*Repository)(nil)
	_ fiscal.VoucherRepository = (*Repository)(nil)
	_ fiscal.SerialLocker      = (*Repository)(nil)
)

type fakeRow struct {
	values []any
	err    error
}

func (row fakeRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(destinations) != len(row.values) {
		return errors.New("fake row destination count differs")
	}
	for index, destination := range destinations {
		target := reflect.ValueOf(destination)
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return errors.New("fake row destination is not a pointer")
		}
		value := row.values[index]
		if value == nil {
			target.Elem().SetZero()
			continue
		}
		source := reflect.ValueOf(value)
		if source.Type().AssignableTo(target.Elem().Type()) {
			target.Elem().Set(source)
			continue
		}
		if source.Type().ConvertibleTo(target.Elem().Type()) {
			target.Elem().Set(source.Convert(target.Elem().Type()))
			continue
		}
		return errors.New("fake row value is not assignable")
	}
	return nil
}

type fakeDB struct {
	row          pgx.Row
	rows         []pgx.Row
	commandTag   pgconn.CommandTag
	execErr      error
	lastQuery    string
	lastArgs     []any
	queriedSQL   []string
	executedSQL  []string
	executedArgs [][]any
}

func (db *fakeDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	db.executedSQL = append(db.executedSQL, sql)
	db.executedArgs = append(db.executedArgs, append([]any(nil), args...))
	return db.commandTag, db.execErr
}

func (db *fakeDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (db *fakeDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	db.lastQuery = sql
	db.lastArgs = append([]any(nil), args...)
	db.queriedSQL = append(db.queriedSQL, sql)
	if len(db.rows) > 0 {
		row := db.rows[0]
		db.rows = db.rows[1:]
		return row
	}
	return db.row
}

func TestEnqueueReplaysOnlyMatchingFingerprint(t *testing.T) {
	t.Parallel()

	snapshot := testSnapshot(t)
	organizationID := uuid.New()
	voucherID := uuid.New()
	sourceID := uuid.New()
	fingerprint := strings.Repeat("a", 64)
	createdAt := time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC)
	voucherValues := []any{
		voucherID, organizationID, "sale", sourceID.String(), "invoice", "homologation", 3, 1,
		int64(0), "queued", string(snapshot.CanonicalJSON()), snapshot.Hash(),
		"", "", "", nil, "accountant", createdAt, createdAt, fingerprint,
	}

	for _, test := range []struct {
		name        string
		fingerprint string
		wantError   error
	}{
		{name: "same command", fingerprint: fingerprint},
		{name: "different command", fingerprint: strings.Repeat("b", 64), wantError: fiscal.ErrIdempotencyConflict},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			database := &fakeDB{rows: []pgx.Row{
				fakeRow{values: []any{organizationID.String()}},
				fakeRow{values: voucherValues},
			}}
			repository, err := New(database)
			if err != nil {
				t.Fatal(err)
			}
			result, err := repository.Enqueue(context.Background(), fiscal.QueueCommand{
				OrganizationID: organizationID,
				IdempotencyKey: "fiscal-command-1234",
				Fingerprint:    test.fingerprint,
				Source:         fiscal.SourceReference{Kind: "sale", ID: sourceID},
				Operation:      fiscal.OperationInvoice,
				Environment:    "homologation",
				PointOfSale:    3,
				AuthorityType:  1,
				Snapshot:       snapshot,
				Actor:          "accountant",
				RequestedAt:    createdAt,
			})
			if !errors.Is(err, test.wantError) {
				t.Fatalf("enqueue error = %v, want %v", err, test.wantError)
			}
			if test.wantError == nil && (!result.Replay || result.Voucher.ID != voucherID) {
				t.Fatalf("matching idempotency replay = %#v", result)
			}
			if len(database.executedSQL) != 0 {
				t.Fatal("idempotency replay performed a write")
			}
		})
	}
}

func TestScanVoucherRestoresExactCanonicalSnapshot(t *testing.T) {
	t.Parallel()

	snapshot := testSnapshot(t)
	organizationID := uuid.New()
	voucherID := uuid.New()
	sourceID := uuid.New()
	createdAt := time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC)
	fingerprint := strings.Repeat("a", 64)

	voucher, gotFingerprint, err := scanVoucherWithFingerprint(fakeRow{values: []any{
		voucherID,
		organizationID,
		"sale",
		sourceID.String(),
		"invoice",
		"homologation",
		3,
		1,
		int64(42),
		"processing",
		string(snapshot.CanonicalJSON()),
		snapshot.Hash(),
		"",
		"",
		"",
		nil,
		"accountant",
		createdAt,
		createdAt,
		fingerprint,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if voucher.OrganizationID != organizationID || voucher.ID != voucherID {
		t.Fatalf("voucher tenant identity changed: %#v", voucher)
	}
	if voucher.Source.ID != sourceID || voucher.Snapshot.Hash() != snapshot.Hash() {
		t.Fatalf("voucher source or immutable snapshot changed: %#v", voucher)
	}
	if gotFingerprint != fingerprint {
		t.Fatalf("fingerprint = %q, want %q", gotFingerprint, fingerprint)
	}
}

func TestScanVoucherRejectsSnapshotHashMismatch(t *testing.T) {
	t.Parallel()

	snapshot := testSnapshot(t)
	_, _, err := scanVoucherWithFingerprint(fakeRow{values: []any{
		uuid.New(), uuid.New(), "sale", uuid.NewString(), "invoice", "homologation", 3, 1,
		int64(0), "queued", string(snapshot.CanonicalJSON()), strings.Repeat("f", 64),
		"", "", "", nil, "actor", time.Now(), time.Now(), strings.Repeat("a", 64),
	}})
	if err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("expected immutable snapshot hash error, got %v", err)
	}
}

func TestRenewLeaseRequiresExactToken(t *testing.T) {
	t.Parallel()

	database := &fakeDB{commandTag: pgconn.NewCommandTag("UPDATE 0")}
	repository, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	err = repository.RenewLease(
		context.Background(), uuid.New(), "stale-token", time.Now().Add(time.Minute),
	)
	if !errors.Is(err, fiscal.ErrLeaseLost) {
		t.Fatalf("renew stale lease error = %v, want ErrLeaseLost", err)
	}
	if !strings.Contains(database.executedSQL[0], "lease_owner = $2") ||
		!strings.Contains(database.executedSQL[0], "org_id = app.current_org_id()") {
		t.Fatalf("lease update is missing tenant/token predicate: %s", database.executedSQL[0])
	}
}

func TestLeaseNextReclaimsExpiredProcessingBeforeAdvancingSeries(t *testing.T) {
	t.Parallel()

	snapshot := testSnapshot(t)
	organizationID := uuid.New()
	voucherID := uuid.New()
	sourceID := uuid.New()
	now := time.Date(2026, 7, 24, 14, 0, 0, 0, time.UTC)
	database := &fakeDB{rows: []pgx.Row{
		fakeRow{values: []any{organizationID, voucherID}},
		fakeRow{values: []any{
			voucherID, organizationID, "sale", sourceID.String(), "invoice",
			"homologation", 3, 1, int64(42), "processing",
			string(snapshot.CanonicalJSON()), snapshot.Hash(),
			"", "", "", nil, "accountant", now.Add(-time.Hour), now,
			strings.Repeat("a", 64),
		}},
	}}
	repository, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := repository.LeaseNext(
		context.Background(), "worker-a", now, now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Voucher.ID != voucherID || lease.Voucher.Number != 42 {
		t.Fatalf("lease = %#v, want expired numbered voucher", lease)
	}
	leaseQuery := database.queriedSQL[0]
	if !strings.Contains(leaseQuery, "voucher.status = 'processing'") ||
		!strings.Contains(leaseQuery, "blocker.status IN ('processing', 'uncertain')") ||
		!strings.Contains(leaseQuery, "WHEN 'processing' THEN 0") {
		t.Fatalf("lease query does not reclaim and serialize unresolved work: %s", leaseQuery)
	}
}

func TestLeaseNextTreatsConcurrentSeriesOwnerAsNoWork(t *testing.T) {
	t.Parallel()

	database := &fakeDB{row: fakeRow{err: &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "fiscal_vouchers_one_unresolved_series_uidx",
	}}}
	repository, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.LeaseNext(
		context.Background(),
		"worker-b",
		time.Now(),
		time.Now().Add(time.Minute),
	)
	if !errors.Is(err, fiscal.ErrNoWork) {
		t.Fatalf("LeaseNext() error = %v, want ErrNoWork", err)
	}
}

func TestWithinSerialValidatesTenantAndLocksBeforeWork(t *testing.T) {
	t.Parallel()

	organizationID := uuid.New()
	pointOfSaleID := uuid.New()
	database := &fakeDB{
		rows: []pgx.Row{
			fakeRow{values: []any{organizationID.String()}},
			fakeRow{values: []any{pointOfSaleID}},
		},
		commandTag: pgconn.NewCommandTag("SELECT 1"),
	}
	repository, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	err = repository.WithinSerial(context.Background(), fiscal.SerialKey{
		OrganizationID: organizationID,
		Environment:    "homologation",
		PointOfSale:    3,
		AuthorityType:  1,
	}, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("serial work was not called")
	}
	if len(database.executedSQL) != 1 ||
		!strings.Contains(database.executedSQL[0], "fiscal.lock_voucher_series") {
		t.Fatalf("serial lock was not acquired: %#v", database.executedSQL)
	}
}

func TestWithinSerialRejectsDifferentTenantContext(t *testing.T) {
	t.Parallel()

	database := &fakeDB{
		row:        fakeRow{values: []any{uuid.NewString()}},
		commandTag: pgconn.NewCommandTag("SELECT 1"),
	}
	repository, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	err = repository.WithinSerial(context.Background(), fiscal.SerialKey{
		OrganizationID: uuid.New(),
		Environment:    "homologation",
		PointOfSale:    3,
		AuthorityType:  1,
	}, func(context.Context) error {
		called = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected tenant context mismatch, got %v", err)
	}
	if called || len(database.executedSQL) != 0 {
		t.Fatal("cross-tenant serial work acquired a lock or ran work")
	}
}

func TestDatabaseErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		constraint string
		want       error
	}{
		{"fiscal_vouchers_idempotency_unique", fiscal.ErrIdempotencyConflict},
		{"fiscal_vouchers_source_unique", fiscal.ErrSourceAlreadyUsed},
		{"fiscal_voucher_number_reservations_number_uidx", fiscal.ErrSequenceConflict},
	}
	for _, test := range tests {
		test := test
		t.Run(test.constraint, func(t *testing.T) {
			t.Parallel()
			got := mapDatabaseError(&pgconn.PgError{
				Code: "23505", ConstraintName: test.constraint,
			})
			if !errors.Is(got, test.want) {
				t.Fatalf("mapped error = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDatabaseArtifactTypeUsesStableSchemaNames(t *testing.T) {
	t.Parallel()

	if got, err := databaseArtifactType("authority_response"); err != nil || got != "authority_response" {
		t.Fatalf("authority response mapping = %q, %v", got, err)
	}
	if _, err := databaseArtifactType("mutable_preview"); err == nil {
		t.Fatal("expected unsupported artifact kind to fail")
	}
}

func TestFinalizeAuthorizedPersistsAccountingIntentInSameUnit(t *testing.T) {
	t.Parallel()

	organizationID := uuid.New()
	voucherID := uuid.New()
	sourceID := uuid.New()
	snapshot := testSnapshot(t)
	database := &fakeDB{
		row:        fakeRow{values: []any{organizationID, snapshot.Hash()}},
		commandTag: pgconn.NewCommandTag("UPDATE 1"),
	}
	repository, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	processedAt := time.Date(2026, 7, 24, 14, 0, 0, 0, time.UTC)
	err = repository.FinalizeAuthorized(context.Background(), fiscal.Finalization{
		VoucherID:  voucherID,
		LeaseToken: "worker:lease",
		Authorization: fiscal.Authorization{
			Decision:    fiscal.DecisionAuthorized,
			Code:        "74212345678901",
			ExpiresOn:   "2026-08-03",
			Number:      42,
			ProcessedAt: processedAt,
		},
		Posting: fiscal.PostingIntent{
			OrganizationID: organizationID,
			VoucherID:      voucherID,
			Source:         fiscal.SourceReference{Kind: "sale", ID: sourceID},
			Operation:      fiscal.OperationInvoice,
			SnapshotHash:   snapshot.Hash(),
			AuthorityCode:  "74212345678901",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(database.executedSQL) != 2 {
		t.Fatalf("finalization statements = %d, want voucher + posting intent", len(database.executedSQL))
	}
	if !strings.Contains(database.executedSQL[0], "status = 'authorized'") {
		t.Fatalf("first finalization statement did not authorize voucher: %s", database.executedSQL[0])
	}
	if !strings.Contains(database.executedSQL[1], "fiscal.accounting_posting_intents") {
		t.Fatalf("accounting posting intent was not persisted: %s", database.executedSQL[1])
	}
}

func testSnapshot(t *testing.T) fiscal.Snapshot {
	t.Helper()
	snapshot, err := fiscal.NewSnapshot(fiscal.FiscalSnapshot{
		Version:     fiscal.SnapshotVersion,
		CountryCode: "AR",
		IssueDate:   "2026-07-24",
		Issuer: fiscal.PartySnapshot{
			Name: "Emisor SA", TaxID: "30000000007",
		},
		Receiver: fiscal.PartySnapshot{
			Name: "Cliente SA", TaxID: "30710158211",
		},
		Currency: fiscal.CurrencySnapshot{
			Code: "PES", Rate: fiscal.MustDecimal("1"),
		},
		Lines: []fiscal.FiscalLineSnapshot{{
			Position: 1, Description: "Servicio",
			Quantity: fiscal.MustDecimal("1"), UnitPrice: fiscal.MustDecimal("100"),
			NetAmount: fiscal.MustDecimal("100"), TaxCode: "IVA21",
			TaxRate: fiscal.MustDecimal("21"), TaxAmount: fiscal.MustDecimal("21"),
			TotalAmount: fiscal.MustDecimal("121"),
		}},
		Totals: fiscal.FiscalTotalsSnapshot{
			NetTaxed:   fiscal.MustDecimal("100"),
			VAT:        fiscal.MustDecimal("21"),
			Total:      fiscal.MustDecimal("121"),
			Functional: fiscal.MustDecimal("121"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
