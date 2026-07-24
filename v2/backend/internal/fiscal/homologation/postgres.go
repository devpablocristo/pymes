package homologation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, errors.New("homologation database pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

func (repository *PostgresRepository) Start(
	ctx context.Context,
	organizationID uuid.UUID,
	requestedBy string,
	startedAt time.Time,
) (uuid.UUID, error) {
	requestedBy = strings.TrimSpace(requestedBy)
	if organizationID == uuid.Nil || requestedBy == "" || startedAt.IsZero() {
		return uuid.Nil, errors.New("homologation organization, actor, and start time are required")
	}
	runID := uuid.New()
	err := repository.withTenant(ctx, organizationID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO fiscal.homologation_runs (
				org_id,
				id,
				environment,
				status,
				requested_by,
				evidence_note,
				started_at
			)
			VALUES ($1, $2, 'homologation', 'running', $3, $4, $5)`,
			organizationID,
			runID,
			requestedBy,
			EvidenceNotice,
			startedAt.UTC(),
		)
		return err
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("start homologation evidence run: %w", err)
	}
	return runID, nil
}

func (repository *PostgresRepository) LoadConfiguration(
	ctx context.Context,
	organizationID uuid.UUID,
	at time.Time,
) (Configuration, error) {
	if organizationID == uuid.Nil || at.IsZero() {
		return Configuration{}, errors.New("homologation organization and validation time are required")
	}
	var configuration Configuration
	err := repository.withTenant(ctx, organizationID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT
				profile.legal_name,
				COALESCE(
					profile.legal_address ->> 'formatted',
					profile.legal_address ->> 'address',
					profile.legal_address::text
				),
				profile.activity_start_date,
				profile.version,
				settings.cuit::text,
				settings.iva_condition,
				settings.version,
				certificate.certificate_ref,
				certificate.private_key_ref,
				certificate.fingerprint_sha256,
				certificate.valid_from,
				certificate.valid_until
			  FROM fiscal.profiles AS profile
			  JOIN fiscal_ar.settings AS settings
			    ON settings.org_id = profile.org_id
			   AND settings.environment = 'homologation'
			   AND settings.enabled
			  JOIN fiscal.certificates AS certificate
			    ON certificate.org_id = settings.org_id
			   AND certificate.country_code = 'AR'
			   AND certificate.environment = settings.environment
			   AND certificate.status = 'active'
			   AND certificate.subject_tax_id = settings.cuit::text
			 WHERE profile.org_id = $1
			   AND profile.country_code = 'AR'
			   AND certificate.valid_from <= $2
			   AND certificate.valid_until > $2 + interval '5 minutes'
			 ORDER BY certificate.valid_until DESC, certificate.id DESC
			 LIMIT 1`,
			organizationID,
			at.UTC(),
		).Scan(
			&configuration.LegalName,
			&configuration.LegalAddress,
			&configuration.ActivityStartDate,
			&configuration.ProfileVersion,
			&configuration.CUIT,
			&configuration.IssuerVATCondition,
			&configuration.SettingsVersion,
			&configuration.CertificateReference,
			&configuration.PrivateKeyReference,
			&configuration.CertificateFingerprint,
			&configuration.CertificateValidFrom,
			&configuration.CertificateValidUntil,
		); err != nil {
			return fmt.Errorf("load active homologation profile and certificate: %w", err)
		}

		rows, err := tx.Query(ctx, `
			SELECT point.code, point.name
			  FROM fiscal.points_of_sale AS point
			 WHERE point.org_id = $1
			   AND point.country_code = 'AR'
			   AND point.environment = 'homologation'
			   AND point.issuing_system = 'wsfev1'
			   AND point.enabled
			 ORDER BY point.code, point.id`,
			organizationID,
		)
		if err != nil {
			return fmt.Errorf("load homologation points of sale: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var point PointOfSale
			if err := rows.Scan(&point.Code, &point.Name); err != nil {
				return fmt.Errorf("scan homologation point of sale: %w", err)
			}
			configuration.PointsOfSale = append(configuration.PointsOfSale, point)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate homologation points of sale: %w", err)
		}
		if len(configuration.PointsOfSale) == 0 {
			return errors.New("no enabled WSFEv1 point of sale exists in homologation")
		}
		return nil
	})
	if err != nil {
		return Configuration{}, err
	}
	return configuration, nil
}

func (repository *PostgresRepository) Complete(
	ctx context.Context,
	organizationID, runID uuid.UUID,
	completion Completion,
) error {
	if organizationID == uuid.Nil || runID == uuid.Nil {
		return errors.New("homologation organization and run are required")
	}
	if completion.Status != StatusSucceeded && completion.Status != StatusFailed {
		return errors.New("homologation completion must be succeeded or failed")
	}
	if completion.CompletedAt.IsZero() || len(completion.Checks) == 0 ||
		len(completion.Evidence) == 0 ||
		strings.TrimSpace(completion.EvidenceHash) == "" {
		return errors.New("homologation completion evidence is incomplete")
	}
	if completion.Status == StatusSucceeded &&
		strings.TrimSpace(completion.ConfigurationHash) == "" {
		return errors.New("successful homologation requires its configuration fingerprint")
	}

	successCount := 0
	failureCount := 0
	for _, check := range completion.Checks {
		switch check.Status {
		case CheckSucceeded:
			successCount++
		case CheckFailed:
			failureCount++
		default:
			return fmt.Errorf("homologation check %d has invalid status", check.Ordinal)
		}
	}
	return repository.withTenant(ctx, organizationID, func(tx pgx.Tx) error {
		for _, check := range completion.Checks {
			_, err := tx.Exec(ctx, `
				INSERT INTO fiscal.homologation_checks (
					org_id,
					run_id,
					ordinal,
					kind,
					name,
					status,
					point_of_sale,
					voucher_type,
					detail_redacted,
					evidence,
					evidence_sha256,
					started_at,
					completed_at
				)
				VALUES (
					$1, $2, $3, $4, $5, $6, $7,
					$8, $9, $10::jsonb, $11, $12, $13
				)`,
				organizationID,
				runID,
				check.Ordinal,
				check.Kind,
				check.Name,
				check.Status,
				check.PointOfSale,
				check.VoucherType,
				check.Detail,
				[]byte(check.Evidence),
				check.EvidenceHash,
				check.StartedAt.UTC(),
				check.CompletedAt.UTC(),
			)
			if err != nil {
				return fmt.Errorf("persist homologation check %d: %w", check.Ordinal, err)
			}
		}

		commandTag, err := tx.Exec(ctx, `
			UPDATE fiscal.homologation_runs
			   SET status = $3,
			       certificate_fingerprint_sha256 = NULLIF($4, ''),
			       configuration_sha256 = NULLIF($5, ''),
			       point_of_sale_count = $6,
			       check_count = $7,
			       success_count = $8,
			       failure_count = $9,
			       evidence = $10::jsonb,
			       evidence_sha256 = $11,
			       completed_at = $12
			 WHERE org_id = $1
			   AND id = $2
			   AND status = 'running'`,
			organizationID,
			runID,
			completion.Status,
			strings.TrimSpace(completion.CertificateFingerprint),
			strings.TrimSpace(completion.ConfigurationHash),
			completion.PointOfSaleCount,
			len(completion.Checks),
			successCount,
			failureCount,
			[]byte(completion.Evidence),
			completion.EvidenceHash,
			completion.CompletedAt.UTC(),
		)
		if err != nil {
			return fmt.Errorf("finalize homologation run: %w", err)
		}
		if commandTag.RowsAffected() != 1 {
			return errors.New("homologation run was not running or does not exist")
		}
		return nil
	})
}

func (repository *PostgresRepository) withTenant(
	ctx context.Context,
	organizationID uuid.UUID,
	work func(pgx.Tx) error,
) error {
	if repository == nil || repository.pool == nil || organizationID == uuid.Nil || work == nil {
		return errors.New("tenant-scoped homologation transaction is incomplete")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(context.WithoutCancel(ctx))
	}()
	var bound string
	if err := tx.QueryRow(ctx, `
		SELECT set_config('app.org_id', $1, true)`,
		organizationID.String(),
	).Scan(&bound); err != nil {
		return fmt.Errorf("bind homologation organization: %w", err)
	}
	if bound != organizationID.String() {
		return errors.New("database did not bind the requested homologation organization")
	}
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
