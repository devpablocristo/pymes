package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the common pgx surface implemented by both *pgxpool.Pool and
// pgx.Tx. Passing the verified session transaction keeps RLS context and all
// fiscal writes in the caller's transaction.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type transactionBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Repository struct {
	db             DBTX
	organizationID uuid.UUID
}

func New(db DBTX) (*Repository, error) {
	if db == nil {
		return nil, errors.New("fiscal postgres database is required")
	}
	return &Repository{db: db}, nil
}

// NewTenant creates the worker-facing repository. Unlike request repositories,
// which inherit app.org_id from the verified session transaction, this
// repository opens a short transaction for every durable worker operation and
// binds the tenant inside that transaction. It must be backed by a transaction
// beginner (normally *pgxpool.Pool); accepting a long-lived pgx.Tx here would
// accidentally keep database locks open across authority network calls.
func NewTenant(db DBTX, organizationID uuid.UUID) (*Repository, error) {
	if db == nil {
		return nil, errors.New("fiscal postgres database is required")
	}
	if organizationID == uuid.Nil {
		return nil, errors.New("fiscal worker organization is required")
	}
	if _, ok := db.(transactionBeginner); !ok {
		return nil, errors.New("fiscal worker database must support short transactions")
	}
	return &Repository{db: db, organizationID: organizationID}, nil
}

type transactionContextKey struct{}

func (repository *Repository) queryer(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(transactionContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return repository.db
}

func (repository *Repository) withinTransaction(
	ctx context.Context,
	work func(context.Context, DBTX) error,
) error {
	if tx, ok := ctx.Value(transactionContextKey{}).(pgx.Tx); ok {
		return work(ctx, tx)
	}
	if tx, ok := repository.db.(pgx.Tx); ok {
		return work(ctx, tx)
	}
	beginner, ok := repository.db.(transactionBeginner)
	if !ok {
		// Test doubles and other callers may already provide a transaction-like
		// DBTX without exposing Begin.
		return work(ctx, repository.db)
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin fiscal transaction: %w", err)
	}
	txContext := context.WithValue(ctx, transactionContextKey{}, tx)
	if err := work(txContext, tx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("rollback fiscal transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit fiscal transaction: %w", err)
	}
	return nil
}

func (repository *Repository) withinWorkerTransaction(
	ctx context.Context,
	work func(context.Context, DBTX) error,
) error {
	return repository.withinTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		if repository.organizationID != uuid.Nil {
			if err := bindOrganization(txContext, tx, repository.organizationID); err != nil {
				return err
			}
		}
		return work(txContext, tx)
	})
}

func bindOrganization(ctx context.Context, db DBTX, organizationID uuid.UUID) error {
	var current string
	if err := db.QueryRow(ctx, `
		SELECT coalesce(nullif(current_setting('app.org_id', true), ''), '')`,
	).Scan(&current); err != nil {
		return fmt.Errorf("read PostgreSQL tenant context: %w", err)
	}
	if current != "" && current != organizationID.String() {
		return errors.New("PostgreSQL tenant context does not match fiscal command")
	}
	if current == "" {
		if err := db.QueryRow(ctx, `
			SELECT set_config('app.org_id', $1, true)`,
			organizationID.String(),
		).Scan(&current); err != nil {
			return fmt.Errorf("set PostgreSQL tenant context: %w", err)
		}
	}
	return nil
}

func (repository *Repository) Enqueue(ctx context.Context, command fiscal.QueueCommand) (fiscal.QueueResult, error) {
	if command.OrganizationID == uuid.Nil {
		return fiscal.QueueResult{}, errors.New("organization context is required")
	}
	var result fiscal.QueueResult
	err := repository.withinTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		if err := bindOrganization(txContext, tx, command.OrganizationID); err != nil {
			return err
		}
		existing, fingerprint, found, err := getByIdempotency(
			txContext, tx, command.OrganizationID, command.IdempotencyKey, true,
		)
		if err != nil {
			return err
		}
		if found {
			if fingerprint != command.Fingerprint {
				return fiscal.ErrIdempotencyConflict
			}
			result = fiscal.QueueResult{Voucher: existing, Replay: true}
			return nil
		}

		document, err := command.Snapshot.Document()
		if err != nil {
			return fmt.Errorf("decode fiscal snapshot: %w", err)
		}
		values, err := normalizeSnapshot(document)
		if err != nil {
			return err
		}
		environment := command.Environment
		pointOfSaleID, err := resolvePointOfSale(
			txContext, tx, command.OrganizationID, environment, command.PointOfSale,
		)
		if err != nil {
			return err
		}
		voucherID := uuid.New()
		_, err = tx.Exec(txContext, `
			INSERT INTO fiscal.vouchers (
				org_id, id, country_code, environment, point_of_sale_id,
				operation, voucher_type, source_type, source_id,
				idempotency_key, command_fingerprint, status, concept,
				issue_date, service_from, service_to, payment_due_date,
				currency_code, exchange_rate, exchange_rate_date,
				exchange_rate_source, net_amount, exempt_amount,
				non_taxed_amount, vat_amount, other_tributes_amount,
				total_amount, created_by, created_at
			)
			VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9,
				$10, $11, 'queued', $12,
				$13, $14, $15, $16,
				$17, $18::numeric, $19, $20,
				$21::numeric, $22::numeric, $23::numeric, $24::numeric,
				$25::numeric, $26::numeric, $27, $28
			)
			ON CONFLICT DO NOTHING`,
			command.OrganizationID, voucherID, document.CountryCode, environment, pointOfSaleID,
			string(command.Operation), command.AuthorityType, command.Source.Kind, command.Source.ID.String(),
			command.IdempotencyKey, command.Fingerprint, values.concept,
			values.issueDate, values.serviceFrom, values.serviceTo, values.paymentDue,
			document.Currency.Code, document.Currency.Rate.String(), values.exchangeRateDate,
			values.exchangeRateSource, document.Totals.NetTaxed.String(), document.Totals.Exempt.String(),
			document.Totals.NetUntaxed.String(), document.Totals.VAT.String(),
			document.Totals.OtherTaxes.String(), document.Totals.Total.String(),
			nonEmpty(command.Actor, "system"), command.RequestedAt.UTC(),
		)
		if err != nil {
			return mapDatabaseError(err)
		}

		var inserted bool
		if err := tx.QueryRow(txContext, `
			SELECT EXISTS (
				SELECT 1
				FROM fiscal.vouchers
				WHERE org_id = $1 AND id = $2
			)`,
			command.OrganizationID, voucherID,
		).Scan(&inserted); err != nil {
			return fmt.Errorf("verify fiscal voucher insert: %w", err)
		}
		if !inserted {
			existing, fingerprint, found, err = getByIdempotency(
				txContext, tx, command.OrganizationID, command.IdempotencyKey, true,
			)
			if err != nil {
				return err
			}
			if found {
				if fingerprint != command.Fingerprint {
					return fiscal.ErrIdempotencyConflict
				}
				result = fiscal.QueueResult{Voucher: existing, Replay: true}
				return nil
			}
			return fiscal.ErrSourceAlreadyUsed
		}

		snapshotID := uuid.New()
		if err := insertSnapshot(
			txContext, tx, command.OrganizationID, voucherID, snapshotID,
			command.Snapshot, document, values,
		); err != nil {
			return err
		}
		created, err := repository.getWithDB(txContext, tx, command.OrganizationID, voucherID)
		if err != nil {
			return err
		}
		result = fiscal.QueueResult{Voucher: created}
		return nil
	})
	return result, err
}

func (repository *Repository) Get(
	ctx context.Context,
	organizationID, voucherID uuid.UUID,
) (fiscal.Voucher, error) {
	if organizationID == uuid.Nil || voucherID == uuid.Nil {
		return fiscal.Voucher{}, fiscal.ErrNotFound
	}
	var voucher fiscal.Voucher
	err := repository.withinTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		if err := bindOrganization(txContext, tx, organizationID); err != nil {
			return err
		}
		var err error
		voucher, err = repository.getWithDB(txContext, tx, organizationID, voucherID)
		return err
	})
	return voucher, err
}

func (repository *Repository) getWithDB(
	ctx context.Context,
	db DBTX,
	organizationID, voucherID uuid.UUID,
) (fiscal.Voucher, error) {
	return scanVoucher(voucherRow(db.QueryRow(ctx, voucherSelect+`
		WHERE voucher.org_id = $1
		  AND voucher.id = $2`,
		organizationID, voucherID,
	)))
}

func getByIdempotency(
	ctx context.Context,
	db DBTX,
	organizationID uuid.UUID,
	idempotencyKey string,
	lock bool,
) (fiscal.Voucher, string, bool, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF voucher"
	}
	row := db.QueryRow(ctx, voucherSelect+`
		WHERE voucher.org_id = $1
		  AND voucher.idempotency_key = $2`+suffix,
		organizationID, idempotencyKey,
	)
	voucher, fingerprint, err := scanVoucherWithFingerprint(row)
	if errors.Is(err, fiscal.ErrNotFound) {
		return fiscal.Voucher{}, "", false, nil
	}
	if err != nil {
		return fiscal.Voucher{}, "", false, err
	}
	return voucher, fingerprint, true, nil
}

const voucherSelect = `
	SELECT
		voucher.id,
		voucher.org_id,
		voucher.source_type,
		voucher.source_id,
		voucher.operation,
		voucher.environment,
		point.code,
		voucher.voucher_type,
		coalesce(voucher.voucher_number, 0),
		voucher.status,
		snapshot.canonical_json,
		snapshot.snapshot_sha256,
		coalesce(voucher.arca_result, ''),
		coalesce(voucher.last_error_code, ''),
		coalesce(voucher.last_error_detail_redacted, ''),
		voucher.uncertain_at,
		voucher.created_by,
		voucher.created_at,
		coalesce(
			voucher.authorized_at,
			voucher.rejected_at,
			voucher.uncertain_at,
			voucher.created_at
		),
		voucher.command_fingerprint
	FROM fiscal.vouchers AS voucher
	JOIN fiscal.points_of_sale AS point
	  ON point.org_id = voucher.org_id
	 AND point.id = voucher.point_of_sale_id
	JOIN fiscal.voucher_snapshots AS snapshot
	  ON snapshot.org_id = voucher.org_id
	 AND snapshot.voucher_id = voucher.id`

type rowScanner interface {
	Scan(...any) error
}

type voucherRow pgx.Row

func scanVoucher(row rowScanner) (fiscal.Voucher, error) {
	voucher, _, err := scanVoucherWithFingerprint(row)
	return voucher, err
}

func scanVoucherWithFingerprint(row rowScanner) (fiscal.Voucher, string, error) {
	var (
		voucher          fiscal.Voucher
		sourceID         string
		operation        string
		status           string
		canonical        string
		snapshotHash     string
		authorizationRaw string
		failureCode      string
		failureMessage   string
		failureAt        *time.Time
		fingerprint      string
	)
	err := row.Scan(
		&voucher.ID,
		&voucher.OrganizationID,
		&voucher.Source.Kind,
		&sourceID,
		&operation,
		&voucher.Environment,
		&voucher.PointOfSale,
		&voucher.AuthorityType,
		&voucher.Number,
		&status,
		&canonical,
		&snapshotHash,
		&authorizationRaw,
		&failureCode,
		&failureMessage,
		&failureAt,
		&voucher.CreatedBy,
		&voucher.CreatedAt,
		&voucher.UpdatedAt,
		&fingerprint,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiscal.Voucher{}, "", fiscal.ErrNotFound
		}
		return fiscal.Voucher{}, "", fmt.Errorf("scan fiscal voucher: %w", err)
	}
	parsedSourceID, err := uuid.Parse(sourceID)
	if err != nil {
		return fiscal.Voucher{}, "", fmt.Errorf("parse fiscal voucher source id: %w", err)
	}
	snapshot, err := fiscal.ParseSnapshot([]byte(canonical), snapshotHash)
	if err != nil {
		return fiscal.Voucher{}, "", fmt.Errorf("restore fiscal voucher snapshot: %w", err)
	}
	voucher.Source.ID = parsedSourceID
	voucher.Operation = fiscal.Operation(operation)
	voucher.Status = fiscal.VoucherStatus(status)
	voucher.Snapshot = snapshot
	voucher.CreatedAt = voucher.CreatedAt.UTC()
	voucher.UpdatedAt = voucher.UpdatedAt.UTC()
	if authorizationRaw != "" {
		var authorization fiscal.Authorization
		if err := json.Unmarshal([]byte(authorizationRaw), &authorization); err != nil {
			return fiscal.Voucher{}, "", fmt.Errorf("restore fiscal authorization: %w", err)
		}
		voucher.Authorization = &authorization
	}
	if failureCode != "" || failureMessage != "" {
		occurredAt := voucher.UpdatedAt
		if failureAt != nil {
			occurredAt = failureAt.UTC()
		}
		voucher.Failure = &fiscal.Failure{
			Code: failureCode, Message: failureMessage, OccurredAt: occurredAt,
		}
	}
	return voucher, fingerprint, nil
}

func mapDatabaseError(err error) error {
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) {
		return err
	}
	switch pgError.ConstraintName {
	case "fiscal_vouchers_idempotency_unique":
		return fiscal.ErrIdempotencyConflict
	case "fiscal_vouchers_source_unique":
		return fiscal.ErrSourceAlreadyUsed
	case "fiscal_vouchers_number_uidx",
		"fiscal_voucher_number_reservations_number_uidx",
		"fiscal_voucher_series_unresolved":
		return fiscal.ErrSequenceConflict
	default:
		return err
	}
}

func isBusySeriesError(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) &&
		pgError.ConstraintName == "fiscal_vouchers_one_unresolved_series_uidx"
}

func resolvePointOfSale(
	ctx context.Context,
	db DBTX,
	organizationID uuid.UUID,
	environment string,
	code int,
) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		SELECT id
		FROM fiscal.points_of_sale
		WHERE org_id = $1
		  AND environment = $2
		  AND code = $3
		  AND enabled
		LIMIT 1`,
		organizationID, environment, code,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("%w: fiscal point of sale %d (%s)", fiscal.ErrNotFound, code, environment)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve fiscal point of sale: %w", err)
	}
	return id, nil
}

type normalizedSnapshot struct {
	concept            string
	issueDate          time.Time
	serviceFrom        *time.Time
	serviceTo          *time.Time
	paymentDue         *time.Time
	exchangeRateDate   time.Time
	exchangeRateSource string
	issuerActivityDate time.Time
	recipientDocType   int
	recipientDocNumber string
}

func normalizeSnapshot(document fiscal.FiscalSnapshot) (normalizedSnapshot, error) {
	issueDate, err := parseDate(document.IssueDate, "issue date")
	if err != nil {
		return normalizedSnapshot{}, err
	}
	values := normalizedSnapshot{
		concept:            "products",
		issueDate:          issueDate,
		exchangeRateDate:   issueDate,
		exchangeRateSource: nonEmpty(document.Currency.RateSource, "snapshot"),
		issuerActivityDate: issueDate,
	}
	if document.Currency.RateDate != "" {
		values.exchangeRateDate, err = parseDate(document.Currency.RateDate, "exchange rate date")
		if err != nil {
			return normalizedSnapshot{}, err
		}
	}
	if document.Issuer.ActivityStartDay != "" {
		values.issuerActivityDate, err = parseDate(document.Issuer.ActivityStartDay, "issuer activity start date")
		if err != nil {
			return normalizedSnapshot{}, err
		}
	}
	if document.ServiceFrom != "" || document.ServiceTo != "" || document.PaymentDue != "" {
		if document.ServiceFrom == "" || document.ServiceTo == "" || document.PaymentDue == "" {
			return normalizedSnapshot{}, errors.New("service snapshots require from, to, and payment due dates")
		}
		values.concept = "services"
		from, parseErr := parseDate(document.ServiceFrom, "service from date")
		if parseErr != nil {
			return normalizedSnapshot{}, parseErr
		}
		to, parseErr := parseDate(document.ServiceTo, "service to date")
		if parseErr != nil {
			return normalizedSnapshot{}, parseErr
		}
		due, parseErr := parseDate(document.PaymentDue, "payment due date")
		if parseErr != nil {
			return normalizedSnapshot{}, parseErr
		}
		values.serviceFrom, values.serviceTo, values.paymentDue = &from, &to, &due
	}
	if configured := strings.TrimSpace(document.Metadata["concept"]); configured != "" {
		switch configured {
		case "products":
			if values.serviceFrom != nil {
				return normalizedSnapshot{}, errors.New("products concept cannot include service dates")
			}
			values.concept = configured
		case "services", "mixed":
			if values.serviceFrom == nil {
				return normalizedSnapshot{}, errors.New("services and mixed concepts require service dates")
			}
			values.concept = configured
		default:
			return normalizedSnapshot{}, fmt.Errorf("invalid fiscal concept %q", configured)
		}
	}
	values.recipientDocType, values.recipientDocNumber, err = recipientIdentity(document.Receiver)
	if err != nil {
		return normalizedSnapshot{}, err
	}
	return values, nil
}

func recipientIdentity(party fiscal.PartySnapshot) (int, string, error) {
	number := nonEmpty(party.DocumentNumber, party.TaxID)
	rawType := strings.TrimSpace(party.DocumentType)
	if rawType != "" {
		documentType, err := strconv.Atoi(rawType)
		if err != nil || documentType <= 0 {
			return 0, "", fmt.Errorf("invalid recipient document type %q", rawType)
		}
		if number == "" {
			return 0, "", errors.New("recipient document number is required")
		}
		return documentType, number, nil
	}
	if len(number) == 11 {
		return 80, number, nil
	}
	if number == "" {
		return 99, "0", nil
	}
	return 96, number, nil
}

func parseDate(raw, label string) (time.Time, error) {
	value, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s: %w", label, err)
	}
	return value.UTC(), nil
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
