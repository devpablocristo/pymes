package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/homologation"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const homologationEvidenceValidity = 180 * 24 * time.Hour

func (h *IAMAPI) getLatestFiscalHomologation(
	w http.ResponseWriter,
	r *http.Request,
) {
	var response api.FiscalHomologationRun
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			var (
				status      string
				fingerprint *string
				evidenceRaw []byte
			)
			err := tx.QueryRow(ctx, `
				SELECT
					run.id,
					run.status,
					run.certificate_fingerprint_sha256,
					run.point_of_sale_count,
					run.check_count,
					run.success_count,
					run.failure_count,
					run.evidence_sha256,
					run.evidence,
					run.evidence_note,
					run.started_at,
					run.completed_at
				FROM fiscal.homologation_runs AS run
				WHERE run.status IN ('succeeded', 'failed')
				ORDER BY run.started_at DESC, run.id DESC
				LIMIT 1`,
			).Scan(
				&response.Id,
				&status,
				&fingerprint,
				&response.PointOfSaleCount,
				&response.CheckCount,
				&response.SuccessCount,
				&response.FailureCount,
				&response.EvidenceSha256,
				&evidenceRaw,
				&response.EvidenceNote,
				&response.StartedAt,
				&response.CompletedAt,
			)
			if errors.Is(err, pgx.ErrNoRows) {
				return errBusinessNotFound
			}
			if err != nil {
				return fmt.Errorf("load latest fiscal homologation: %w", err)
			}
			response.Status = api.FiscalHomologationStatus(status)
			if !response.Status.Valid() {
				return fmt.Errorf("unsupported fiscal homologation status %q", status)
			}
			response.CertificateFingerprint = fingerprint
			if err := verifyCanonicalJSONHash(
				evidenceRaw,
				response.EvidenceSha256,
			); err != nil {
				return fmt.Errorf("verify fiscal homologation evidence: %w", err)
			}
			response.Checks = make([]api.FiscalHomologationCheck, 0, response.CheckCount)

			rows, err := tx.Query(ctx, `
				SELECT
					check_result.ordinal,
					check_result.kind,
					check_result.name,
					check_result.status,
					check_result.point_of_sale,
					check_result.voucher_type,
					check_result.detail_redacted,
					check_result.evidence,
					check_result.evidence_sha256,
					check_result.started_at,
					check_result.completed_at
				FROM fiscal.homologation_checks AS check_result
				WHERE check_result.run_id = $1
				ORDER BY check_result.ordinal`,
				response.Id,
			)
			if err != nil {
				return fmt.Errorf("load fiscal homologation checks: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var (
					item        api.FiscalHomologationCheck
					kind        string
					checkStatus string
					evidenceRaw []byte
				)
				if err := rows.Scan(
					&item.Ordinal,
					&kind,
					&item.Name,
					&checkStatus,
					&item.PointOfSale,
					&item.VoucherType,
					&item.Detail,
					&evidenceRaw,
					&item.EvidenceSha256,
					&item.StartedAt,
					&item.CompletedAt,
				); err != nil {
					return fmt.Errorf("scan fiscal homologation check: %w", err)
				}
				item.Kind = api.FiscalHomologationCheckKind(kind)
				item.Status = api.FiscalHomologationStatus(checkStatus)
				if !item.Kind.Valid() || !item.Status.Valid() {
					return fmt.Errorf(
						"unsupported fiscal homologation check %q/%q",
						kind,
						checkStatus,
					)
				}
				if err := json.Unmarshal(evidenceRaw, &item.Evidence); err != nil {
					return fmt.Errorf("decode fiscal homologation evidence: %w", err)
				}
				if err := verifyCanonicalJSONHash(
					evidenceRaw,
					item.EvidenceSha256,
				); err != nil {
					return fmt.Errorf(
						"verify fiscal homologation check %d: %w",
						item.Ordinal,
						err,
					)
				}
				response.Checks = append(response.Checks, item)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate fiscal homologation checks: %w", err)
			}
			if len(response.Checks) != response.CheckCount {
				return errors.New("fiscal homologation evidence is incomplete")
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) enableFiscalProduction(
	w http.ResponseWriter,
	r *http.Request,
	_ api.EnableFiscalProductionParams,
) {
	var input api.VersionedCommandInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if input.Version <= 0 {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Fiscal settings version is required")
		return
	}

	var response api.ArgentinaFiscalSettings
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			var (
				currentVersion int64
				cuit           string
			)
			if err := tx.QueryRow(ctx, `
				SELECT version, cuit::text
				FROM fiscal_ar.settings
				WHERE environment = 'production'`,
			).Scan(&currentVersion, &cuit); errors.Is(err, pgx.ErrNoRows) {
				return errBusinessNotFound
			} else if err != nil {
				return fmt.Errorf("load production fiscal settings: %w", err)
			}
			if currentVersion != input.Version {
				return errBusinessVersionConflict
			}

			ready, certificateExpiry, err := fiscalProductionPrerequisites(
				ctx,
				tx,
				cuit,
				h.now().UTC(),
			)
			if err != nil {
				return err
			}
			if !ready {
				return errFiscalProductionNotReady
			}
			err = tx.QueryRow(ctx, `
				UPDATE fiscal_ar.settings
				   SET enabled = true,
				       version = version + 1,
				       updated_at = now()
				 WHERE environment = 'production'
				   AND version = $1
				RETURNING version`,
				input.Version,
			).Scan(&currentVersion)
			if errors.Is(err, pgx.ErrNoRows) {
				return errBusinessVersionConflict
			}
			if err != nil {
				return fmt.Errorf("enable fiscal production: %w", err)
			}

			loaded, err := loadFiscalSettingsResponse(
				ctx,
				tx,
				api.FiscalEnvironmentProduction,
				h.now().UTC(),
			)
			if err != nil {
				return err
			}
			loaded.Version = currentVersion
			loaded.CertificateExpiresAt = certificateExpiry
			loaded.ProductionReady = true
			response = loaded
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func fiscalProductionPrerequisites(
	ctx context.Context,
	tx pgx.Tx,
	productionCUIT string,
	now time.Time,
) (bool, *time.Time, error) {
	var (
		certificateExpiry *time.Time
		hasPointOfSale    bool
	)
	err := tx.QueryRow(ctx, `
		SELECT
			(
				SELECT max(certificate.valid_until)
				FROM fiscal.certificates AS certificate
				WHERE certificate.environment = 'production'
				  AND certificate.country_code = 'AR'
				  AND certificate.status = 'active'
				  AND certificate.subject_tax_id = $3
				  AND certificate.valid_from <= $1
				  AND certificate.valid_until > $2
			),
			EXISTS (
				SELECT 1
				FROM fiscal.points_of_sale AS point
				WHERE point.environment = 'production'
				  AND point.country_code = 'AR'
				  AND point.issuing_system = 'wsfev1'
				  AND point.enabled
			)`,
		now,
		now.Add(24*time.Hour),
		productionCUIT,
	).Scan(&certificateExpiry, &hasPointOfSale)
	if err != nil {
		return false, nil, fmt.Errorf("evaluate fiscal production prerequisites: %w", err)
	}
	if certificateExpiry == nil || !hasPointOfSale {
		return false, certificateExpiry, nil
	}

	configuration, err := loadCurrentHomologationConfiguration(ctx, tx, now)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, certificateExpiry, nil
	}
	if err != nil {
		return false, nil, err
	}
	if configuration.CUIT != productionCUIT {
		return false, certificateExpiry, nil
	}
	configurationHash, err := homologation.ConfigurationFingerprint(configuration)
	if err != nil {
		return false, nil, err
	}
	var (
		runEvidence []byte
		runHash     string
	)
	err = tx.QueryRow(ctx, `
		SELECT run.evidence, run.evidence_sha256::text
		  FROM fiscal.homologation_runs AS run
		 WHERE run.status = 'succeeded'
		   AND run.configuration_sha256 = $1
		   AND run.certificate_fingerprint_sha256 = $2
		   AND run.completed_at >= $3
		 ORDER BY run.completed_at DESC, run.id DESC
		 LIMIT 1`,
		configurationHash,
		configuration.CertificateFingerprint,
		now.Add(-homologationEvidenceValidity),
	).Scan(&runEvidence, &runHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, certificateExpiry, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("load matching fiscal homologation evidence: %w", err)
	}
	if err := verifyCanonicalJSONHash(runEvidence, runHash); err != nil {
		return false, nil, fmt.Errorf("verify matching fiscal homologation evidence: %w", err)
	}
	return true, certificateExpiry, nil
}

func loadCurrentHomologationConfiguration(
	ctx context.Context,
	tx pgx.Tx,
	at time.Time,
) (homologation.Configuration, error) {
	var configuration homologation.Configuration
	err := tx.QueryRow(ctx, `
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
		 WHERE profile.country_code = 'AR'
		   AND certificate.valid_from <= $1
		   AND certificate.valid_until > $1 + interval '5 minutes'
		 ORDER BY certificate.valid_until DESC, certificate.id DESC
		 LIMIT 1`,
		at,
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
	)
	if err != nil {
		return homologation.Configuration{}, err
	}
	rows, err := tx.Query(ctx, `
		SELECT point.code, point.name
		  FROM fiscal.points_of_sale AS point
		 WHERE point.country_code = 'AR'
		   AND point.environment = 'homologation'
		   AND point.issuing_system = 'wsfev1'
		   AND point.enabled
		 ORDER BY point.code, point.id`)
	if err != nil {
		return homologation.Configuration{}, fmt.Errorf(
			"load current homologation points of sale: %w",
			err,
		)
	}
	defer rows.Close()
	for rows.Next() {
		var point homologation.PointOfSale
		if err := rows.Scan(&point.Code, &point.Name); err != nil {
			return homologation.Configuration{}, fmt.Errorf(
				"scan current homologation point of sale: %w",
				err,
			)
		}
		configuration.PointsOfSale = append(configuration.PointsOfSale, point)
	}
	if err := rows.Err(); err != nil {
		return homologation.Configuration{}, fmt.Errorf(
			"iterate current homologation points of sale: %w",
			err,
		)
	}
	if len(configuration.PointsOfSale) == 0 {
		return homologation.Configuration{}, pgx.ErrNoRows
	}
	return configuration, nil
}

func verifyCanonicalJSONHash(raw []byte, expected string) error {
	var value any
	if len(raw) == 0 || strings.TrimSpace(expected) == "" {
		return errors.New("canonical evidence and digest are required")
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("decode canonical JSON: %w", err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode canonical JSON: %w", err)
	}
	sum := sha256.Sum256(canonical)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, strings.TrimSpace(expected)) {
		return errors.New("canonical JSON digest does not match persisted evidence")
	}
	return nil
}

func loadFiscalSettingsResponse(
	ctx context.Context,
	tx pgx.Tx,
	environment api.FiscalEnvironment,
	now time.Time,
) (api.ArgentinaFiscalSettings, error) {
	var (
		response          api.ArgentinaFiscalSettings
		activityStartDate time.Time
		addressRaw        []byte
		taxCondition      string
		certificateExpiry *time.Time
		enabled           bool
	)
	err := tx.QueryRow(ctx, `
		SELECT
			profile.legal_name,
			profile.legal_address,
			profile.activity_start_date,
			profile.default_currency,
			settings.cuit::text,
			settings.environment,
			settings.iva_condition,
			settings.version,
			settings.enabled,
			(
				SELECT max(certificate.valid_until)
				FROM fiscal.certificates AS certificate
				WHERE certificate.environment = settings.environment
				  AND certificate.status = 'active'
			)
		FROM fiscal.profiles AS profile
		JOIN fiscal_ar.settings AS settings
		  ON settings.org_id = profile.org_id
		WHERE settings.environment = $1`,
		environment,
	).Scan(
		&response.LegalName,
		&addressRaw,
		&activityStartDate,
		&response.FunctionalCurrency,
		&response.Cuit,
		&response.Environment,
		&taxCondition,
		&response.Version,
		&enabled,
		&certificateExpiry,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.ArgentinaFiscalSettings{}, errBusinessNotFound
	}
	if err != nil {
		return api.ArgentinaFiscalSettings{}, fmt.Errorf("load fiscal settings response: %w", err)
	}
	response.CountryCode = api.AR
	response.TaxAddress = fiscalAddress(addressRaw)
	response.TaxCondition = apiTaxCondition(taxCondition)
	response.ActivityStartDate = openapi_types.Date{Time: activityStartDate}
	response.CertificateExpiresAt = certificateExpiry
	response.ProductionReady = false
	if environment == api.FiscalEnvironmentProduction && enabled {
		ready, expiry, err := fiscalProductionPrerequisites(
			ctx,
			tx,
			response.Cuit,
			now,
		)
		if err != nil {
			return api.ArgentinaFiscalSettings{}, err
		}
		response.ProductionReady = ready
		if expiry != nil {
			response.CertificateExpiresAt = expiry
		}
	}
	return response, nil
}
