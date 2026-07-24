package httpserver

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *IAMAPI) getIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.GetIVASimpleWorkflowParams,
) {
	normalizedPeriod, firstDay, _, err := fiscalPeriod(period)
	if err != nil {
		writeBusinessError(w, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err))
		return
	}
	var response api.IVASimpleWorkflowPeriod
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
			var err error
			response, err = loadIVASimpleWorkflowPeriod(
				ctx,
				tx,
				normalizedPeriod,
				firstDay,
				string(params.Environment),
			)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) prepareIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.PrepareIVASimpleWorkflowParams,
) {
	var input api.IVASimplePrepareInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	normalizedPeriod, firstDay, nextMonth, err := fiscalPeriod(period)
	if err != nil {
		writeBusinessError(w, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err))
		return
	}
	openingBalance, err := optionalIVADecimal(input.OpeningBalance)
	if err != nil {
		writeBusinessError(w, fmt.Errorf("%w: opening balance: %v", errBusinessInvalidRequest, err))
		return
	}

	status := http.StatusOK
	var response api.IVASimpleWorkflowPeriod
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			organizationID, err := uuid.Parse(active.OrganizationID)
			if err != nil {
				return fmt.Errorf("parse active IVA organization: %w", err)
			}
			environment := string(params.Environment)
			if err := lockIVASimpleWorkflow(
				ctx, tx, organizationID, environment, normalizedPeriod,
			); err != nil {
				return err
			}

			var (
				periodID       uuid.UUID
				currentStatus  string
				currentVersion int64
				currentOpening string
			)
			err = tx.QueryRow(ctx, `
				SELECT id, status, version, opening_balance::text
				  FROM fiscal.iva_periods
				 WHERE org_id = $1
				   AND environment = $2
				   AND period_month = $3
				 FOR UPDATE`,
				organizationID,
				environment,
				firstDay,
			).Scan(&periodID, &currentStatus, &currentVersion, &currentOpening)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				if input.Version != nil {
					return fmt.Errorf("%w: IVA period does not exist", errBusinessVersionConflict)
				}
				if openingBalance == nil {
					var prior string
					if priorErr := tx.QueryRow(ctx, `
						SELECT closing_balance::text
						  FROM fiscal.iva_periods
						 WHERE org_id = $1
						   AND environment = $2
						   AND period_month < $3
						   AND status IN ('closed', 'exported')
						   AND closing_balance IS NOT NULL
						 ORDER BY period_month DESC
						 LIMIT 1`,
						organizationID,
						environment,
						firstDay,
					).Scan(&prior); priorErr == nil {
						openingBalance = &prior
					} else if !errors.Is(priorErr, pgx.ErrNoRows) {
						return fmt.Errorf("load prior IVA closing balance: %w", priorErr)
					}
				}
				if openingBalance == nil {
					zero := "0"
					openingBalance = &zero
				}
				periodID = uuid.New()
				if _, err := tx.Exec(ctx, `
					INSERT INTO fiscal.iva_periods (
						org_id,
						id,
						environment,
						period_month,
						opening_balance,
						created_by
					)
					VALUES ($1, $2, $3, $4, $5::numeric, $6)`,
					organizationID,
					periodID,
					environment,
					firstDay,
					*openingBalance,
					active.UserID,
				); err != nil {
					return fmt.Errorf("create IVA Simple period: %w", err)
				}
				status = http.StatusCreated
			case err != nil:
				return fmt.Errorf("load IVA Simple period: %w", err)
			default:
				if currentStatus != string(api.IVASimpleWorkflowStatusDraft) {
					return fmt.Errorf("%w: IVA period is %s", errBusinessInvalidTransition, currentStatus)
				}
				if input.Version == nil || *input.Version != currentVersion {
					return fmt.Errorf("%w: IVA period version", errBusinessVersionConflict)
				}
				if openingBalance == nil {
					openingBalance = &currentOpening
				}
				if _, err := tx.Exec(ctx, `
					DELETE FROM fiscal.iva_period_items
					 WHERE org_id = $1
					   AND iva_period_id = $2`,
					organizationID,
					periodID,
				); err != nil {
					return fmt.Errorf("clear IVA Simple snapshot: %w", err)
				}
				tag, err := tx.Exec(ctx, `
					UPDATE fiscal.iva_periods
					   SET opening_balance = $3::numeric,
					       closing_balance = NULL,
					       version = version + 1,
					       updated_at = now()
					 WHERE org_id = $1
					   AND id = $2
					   AND status = 'draft'
					   AND version = $4`,
					organizationID,
					periodID,
					*openingBalance,
					currentVersion,
				)
				if err != nil {
					return fmt.Errorf("refresh IVA Simple period: %w", err)
				}
				if tag.RowsAffected() != 1 {
					return fmt.Errorf("%w: IVA period version", errBusinessVersionConflict)
				}
			}

			if err := snapshotIVASimplePeriod(
				ctx,
				tx,
				organizationID,
				periodID,
				environment,
				firstDay,
				nextMonth,
			); err != nil {
				return err
			}
			response, err = loadIVASimpleWorkflowPeriod(
				ctx,
				tx,
				normalizedPeriod,
				firstDay,
				environment,
			)
			return err
		},
	) {
		return
	}
	writeJSON(w, status, response)
}

func (h *IAMAPI) closeIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.CloseIVASimpleWorkflowParams,
) {
	var input api.IVASimpleTransitionInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	h.transitionIVASimpleWorkflow(
		w,
		r,
		period,
		string(params.Environment),
		input,
		api.IVASimpleWorkflowStatusClosed,
	)
}

func (h *IAMAPI) reopenIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.ReopenIVASimpleWorkflowParams,
) {
	var input api.IVASimpleTransitionInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	h.transitionIVASimpleWorkflow(
		w,
		r,
		period,
		string(params.Environment),
		input,
		api.IVASimpleWorkflowStatusDraft,
	)
}

func (h *IAMAPI) transitionIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	environment string,
	input api.IVASimpleTransitionInput,
	target api.IVASimpleWorkflowStatus,
) {
	normalizedPeriod, firstDay, _, err := fiscalPeriod(period)
	if err != nil {
		writeBusinessError(w, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err))
		return
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		writeBusinessError(w, fmt.Errorf("%w: transition reason is required", errBusinessInvalidRequest))
		return
	}
	var response api.IVASimpleWorkflowPeriod
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			organizationID, err := uuid.Parse(active.OrganizationID)
			if err != nil {
				return fmt.Errorf("parse active IVA organization: %w", err)
			}
			if err := lockIVASimpleWorkflow(
				ctx, tx, organizationID, environment, normalizedPeriod,
			); err != nil {
				return err
			}
			periodID, status, version, err := lockIVASimplePeriod(
				ctx, tx, organizationID, environment, firstDay,
			)
			if err != nil {
				return err
			}
			if version != input.Version {
				return fmt.Errorf("%w: IVA period version", errBusinessVersionConflict)
			}

			switch target {
			case api.IVASimpleWorkflowStatusClosed:
				if status != string(api.IVASimpleWorkflowStatusDraft) {
					return fmt.Errorf("%w: IVA period is %s", errBusinessInvalidTransition, status)
				}
				_, err = tx.Exec(ctx, `
					UPDATE fiscal.iva_periods AS period
					   SET status = 'closed',
					       closing_balance = (
					           SELECT (
					               period.opening_balance
					               + coalesce(sum(
					                   item.vat_credit_amount
					                   * CASE
					                       WHEN coalesce(
					                           voucher.voucher_type,
					                           purchase.voucher_type
					                       ) IN (3, 8, 13)
					                       THEN -1 ELSE 1
					                     END
					               ), 0)
					               + coalesce(sum(
					                   item.withholding_amount
					                   * CASE
					                       WHEN coalesce(
					                           voucher.voucher_type,
					                           purchase.voucher_type
					                       ) IN (3, 8, 13)
					                       THEN -1 ELSE 1
					                     END
					               ), 0)
					               + coalesce(sum(
					                   item.perception_amount
					                   * CASE
					                       WHEN coalesce(
					                           voucher.voucher_type,
					                           purchase.voucher_type
					                       ) IN (3, 8, 13)
					                       THEN -1 ELSE 1
					                     END
					               ), 0)
					               - coalesce(sum(
					                   item.vat_debit_amount
					                   * CASE
					                       WHEN coalesce(
					                           voucher.voucher_type,
					                           purchase.voucher_type
					                       ) IN (3, 8, 13)
					                       THEN -1 ELSE 1
					                     END
					               ), 0)
					           )::numeric(24, 6)
					             FROM fiscal.iva_period_items AS item
					             LEFT JOIN fiscal.vouchers AS voucher
					               ON voucher.org_id = item.org_id
					              AND voucher.id = item.voucher_id
					             LEFT JOIN fiscal.purchase_vouchers AS purchase
					               ON purchase.org_id = item.org_id
					              AND purchase.id = item.purchase_voucher_id
					            WHERE item.org_id = period.org_id
					              AND item.iva_period_id = period.id
					       ),
					       version = version + 1,
					       status_changed_by = $3,
					       transition_reason = $4,
					       closed_at = now(),
					       exported_at = NULL,
					       updated_at = now()
					 WHERE period.org_id = $1
					   AND period.id = $2`,
					organizationID,
					periodID,
					active.UserID,
					reason,
				)
			case api.IVASimpleWorkflowStatusDraft:
				if status != string(api.IVASimpleWorkflowStatusClosed) &&
					status != string(api.IVASimpleWorkflowStatusExported) {
					return fmt.Errorf("%w: IVA period is %s", errBusinessInvalidTransition, status)
				}
				_, err = tx.Exec(ctx, `
					UPDATE fiscal.iva_periods
					   SET status = 'draft',
					       closing_balance = NULL,
					       version = version + 1,
					       status_changed_by = $3,
					       transition_reason = $4,
					       closed_at = NULL,
					       exported_at = NULL,
					       updated_at = now()
					 WHERE org_id = $1
					   AND id = $2`,
					organizationID,
					periodID,
					active.UserID,
					reason,
				)
			default:
				return fmt.Errorf("%w: unsupported IVA transition", errBusinessInvalidRequest)
			}
			if err != nil {
				return fmt.Errorf("transition IVA Simple period: %w", err)
			}
			response, err = loadIVASimpleWorkflowPeriod(
				ctx, tx, normalizedPeriod, firstDay, environment,
			)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) exportIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.ExportIVASimpleWorkflowParams,
) {
	var input api.IVASimpleTransitionInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	normalizedPeriod, firstDay, nextMonth, err := fiscalPeriod(period)
	if err != nil {
		writeBusinessError(w, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err))
		return
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		writeBusinessError(w, fmt.Errorf("%w: export reason is required", errBusinessInvalidRequest))
		return
	}
	var response api.IVASimpleExportArtifact
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			organizationID, err := uuid.Parse(active.OrganizationID)
			if err != nil {
				return fmt.Errorf("parse active IVA organization: %w", err)
			}
			environment := string(params.Environment)
			if err := lockIVASimpleWorkflow(
				ctx, tx, organizationID, environment, normalizedPeriod,
			); err != nil {
				return err
			}
			periodID, status, version, err := lockIVASimplePeriod(
				ctx, tx, organizationID, environment, firstDay,
			)
			if err != nil {
				return err
			}
			if version != input.Version {
				return fmt.Errorf("%w: IVA period version", errBusinessVersionConflict)
			}
			if status != string(api.IVASimpleWorkflowStatusClosed) {
				return fmt.Errorf("%w: IVA period is %s", errBusinessInvalidTransition, status)
			}
			if _, err := tx.Exec(
				ctx,
				`SELECT fiscal.assert_iva_period_valid($1, $2)`,
				organizationID,
				periodID,
			); err != nil {
				return fmt.Errorf("reconcile IVA Simple period: %w", err)
			}

			records, _, err := loadIVAWorkflowRecords(
				ctx,
				tx,
				periodID,
				firstDay,
				nextMonth,
				environment,
			)
			if err != nil {
				return err
			}
			files, err := ar.ExportIVASimple(
				strings.ReplaceAll(normalizedPeriod, "-", ""),
				records,
			)
			if err != nil {
				return fmt.Errorf("generate IVA Simple files: %w", err)
			}
			artifact, err := fullIVARegistryBundle(files)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(artifact)
			hash := hex.EncodeToString(digest[:])
			exportID := uuid.New()
			var exportVersion int
			if err := tx.QueryRow(ctx, `
				SELECT coalesce(max(export_version), 0) + 1
				  FROM fiscal.iva_exports
				 WHERE org_id = $1
				   AND iva_period_id = $2
				   AND export_type = 'workpaper'`,
				organizationID,
				periodID,
			).Scan(&exportVersion); err != nil {
				return fmt.Errorf("allocate IVA export version: %w", err)
			}
			filename := fmt.Sprintf(
				"iva-simple-%s-%s-v%d.zip",
				normalizedPeriod,
				environment,
				exportVersion,
			)
			if _, err := tx.Exec(ctx, `
				INSERT INTO fiscal.iva_exports (
					org_id,
					id,
					iva_period_id,
					export_type,
					export_version,
					storage_ref,
					filename,
					media_type,
					artifact,
					sha256,
					validation_result,
					created_by
				)
				VALUES (
					$1, $2, $3, 'workpaper', $4, $5, $6,
					'application/zip', $7, $8,
					jsonb_build_object(
						'reason', $9,
						'period_version', $11,
						'document_count', (
							SELECT count(*)
							  FROM fiscal.iva_period_items AS item
							 WHERE item.org_id = $1
							   AND item.iva_period_id = $3
						),
						'accounting_reconciled', true
					),
					$10
				)`,
				organizationID,
				exportID,
				periodID,
				exportVersion,
				"db://fiscal/iva-exports/"+exportID.String(),
				filename,
				artifact,
				hash,
				reason,
				active.UserID,
				input.Version,
			); err != nil {
				return fmt.Errorf("persist IVA Simple export: %w", err)
			}
			if _, err := tx.Exec(ctx, `
				UPDATE fiscal.iva_periods
				   SET status = 'exported',
				       version = version + 1,
				       status_changed_by = $3,
				       transition_reason = $4,
				       exported_at = now(),
				       updated_at = now()
				 WHERE org_id = $1
				   AND id = $2`,
				organizationID,
				periodID,
				active.UserID,
				reason,
			); err != nil {
				return fmt.Errorf("mark IVA Simple period exported: %w", err)
			}
			response, err = loadIVAExportArtifact(
				ctx, tx, organizationID, periodID, exportID,
			)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) downloadIVASimpleExport(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	exportID openapi_types.UUID,
	params api.DownloadIVASimpleExportParams,
) {
	normalizedPeriod, firstDay, _, err := fiscalPeriod(period)
	if err != nil {
		writeBusinessError(w, fmt.Errorf("%w: %v", errBusinessInvalidRequest, err))
		return
	}
	var response api.IVASimpleExportArtifact
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			organizationID, err := uuid.Parse(active.OrganizationID)
			if err != nil {
				return fmt.Errorf("parse active IVA organization: %w", err)
			}
			var periodID uuid.UUID
			if err := tx.QueryRow(ctx, `
				SELECT id
				  FROM fiscal.iva_periods
				 WHERE org_id = $1
				   AND environment = $2
				   AND period_month = $3`,
				organizationID,
				string(params.Environment),
				firstDay,
			).Scan(&periodID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("%w: IVA period %s", errBusinessNotFound, normalizedPeriod)
				}
				return fmt.Errorf("load IVA Simple period for export: %w", err)
			}
			response, err = loadIVAExportArtifact(
				ctx, tx, organizationID, periodID, uuid.UUID(exportID),
			)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func optionalIVADecimal(raw *api.DecimalAmount) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	value, err := fiscal.ParseDecimal(string(*raw))
	if err != nil {
		return nil, err
	}
	normalized := value.String()
	return &normalized, nil
}

func lockIVASimpleWorkflow(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	environment string,
	period string,
) error {
	_, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(
			hashtextextended($1 || ':' || $2 || ':' || $3, 0)
		)`,
		organizationID.String(),
		environment,
		period,
	)
	if err != nil {
		return fmt.Errorf("lock IVA Simple workflow: %w", err)
	}
	return nil
}

func lockIVASimplePeriod(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	environment string,
	firstDay time.Time,
) (uuid.UUID, string, int64, error) {
	var (
		periodID uuid.UUID
		status   string
		version  int64
	)
	err := tx.QueryRow(ctx, `
		SELECT id, status, version
		  FROM fiscal.iva_periods
		 WHERE org_id = $1
		   AND environment = $2
		   AND period_month = $3
		 FOR UPDATE`,
		organizationID,
		environment,
		firstDay,
	).Scan(&periodID, &status, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", 0, fmt.Errorf("%w: IVA period", errBusinessNotFound)
	}
	if err != nil {
		return uuid.Nil, "", 0, fmt.Errorf("lock IVA Simple period: %w", err)
	}
	return periodID, status, version, nil
}

func snapshotIVASimplePeriod(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	periodID uuid.UUID,
	environment string,
	firstDay time.Time,
	nextMonth time.Time,
) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.iva_period_items (
			org_id,
			iva_period_id,
			book,
			voucher_id,
			document_sha256,
			net_amount,
			exempt_amount,
			non_taxed_amount,
			vat_debit_amount,
			withholding_amount,
			perception_amount,
			other_tributes_amount
		)
		SELECT
			voucher.org_id,
			$2,
			'sales',
			voucher.id,
			snapshot.snapshot_sha256,
			snapshot.net_amount,
			snapshot.exempt_amount,
			snapshot.non_taxed_amount,
			snapshot.vat_amount,
			coalesce(sum(tax.amount) FILTER (
				WHERE tax.tax_type = 'withholding'
			), 0),
			coalesce(sum(tax.amount) FILTER (
				WHERE tax.tax_type = 'perception'
			), 0),
			coalesce(sum(tax.amount) FILTER (
				WHERE tax.tax_type = 'tribute'
			), 0)
		  FROM fiscal.vouchers AS voucher
		  JOIN fiscal.voucher_snapshots AS snapshot
		    ON snapshot.org_id = voucher.org_id
		   AND snapshot.voucher_id = voucher.id
		  LEFT JOIN fiscal.voucher_taxes AS tax
		    ON tax.org_id = snapshot.org_id
		   AND tax.snapshot_id = snapshot.id
		 WHERE voucher.org_id = $1
		   AND voucher.environment = $3
		   AND voucher.status = 'authorized'
		   AND voucher.issue_date >= $4
		   AND voucher.issue_date < $5
		 GROUP BY voucher.org_id, voucher.id, snapshot.id`,
		organizationID,
		periodID,
		environment,
		firstDay,
		nextMonth,
	); err != nil {
		return fmt.Errorf("snapshot IVA sales: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.iva_period_items (
			org_id,
			iva_period_id,
			book,
			purchase_voucher_id,
			document_sha256,
			net_amount,
			exempt_amount,
			non_taxed_amount,
			vat_credit_amount,
			withholding_amount,
			perception_amount,
			other_tributes_amount
		)
		SELECT
			purchase.org_id,
			$2,
			'purchases',
			purchase.id,
			purchase.snapshot_sha256,
			purchase.net_amount,
			purchase.exempt_amount,
			purchase.non_taxed_amount,
			coalesce(sum(tax.amount) FILTER (
				WHERE tax.tax_type = 'vat' AND tax.creditable
			), 0),
			purchase.withholding_amount,
			purchase.perception_amount,
			purchase.other_taxes_amount
		  FROM fiscal.purchase_vouchers AS purchase
		  LEFT JOIN fiscal.purchase_voucher_taxes AS tax
		    ON tax.org_id = purchase.org_id
		   AND tax.purchase_voucher_id = purchase.id
		 WHERE purchase.org_id = $1
		   AND purchase.environment = $3
		   AND purchase.issue_date >= $4
		   AND purchase.issue_date < $5
		 GROUP BY purchase.org_id, purchase.id`,
		organizationID,
		periodID,
		environment,
		firstDay,
		nextMonth,
	); err != nil {
		return fmt.Errorf("snapshot IVA purchases: %w", err)
	}
	return nil
}

func loadIVASimpleWorkflowPeriod(
	ctx context.Context,
	tx pgx.Tx,
	normalizedPeriod string,
	firstDay time.Time,
	environment string,
) (api.IVASimpleWorkflowPeriod, error) {
	var (
		response                           api.IVASimpleWorkflowPeriod
		status                             string
		opening, closing                   string
		salesNet, outputVAT                string
		purchasesNet, inputVAT             string
		withholdings, perceptions, balance string
		closingBalance                     *string
	)
	err := tx.QueryRow(ctx, `
		SELECT
			period.id,
			period.status,
			period.opening_balance::text,
			period.closing_balance::text,
			period.version,
			period.created_at,
			period.updated_at,
			period.closed_at,
			period.exported_at,
			coalesce(sum(
				(item.net_amount + item.exempt_amount + item.non_taxed_amount)
				* CASE
					WHEN voucher.voucher_type IN (3, 8, 13) THEN -1
					ELSE 1
				  END
			) FILTER (WHERE item.book = 'sales'), 0)::text,
			coalesce(sum(
				item.vat_debit_amount
				* CASE
					WHEN voucher.voucher_type IN (3, 8, 13) THEN -1
					ELSE 1
				  END
			) FILTER (WHERE item.book = 'sales'), 0)::text,
			coalesce(sum(
				(item.net_amount + item.exempt_amount + item.non_taxed_amount)
				* CASE
					WHEN purchase.voucher_type IN (3, 8, 13) THEN -1
					ELSE 1
				  END
			) FILTER (WHERE item.book = 'purchases'), 0)::text,
			coalesce(sum(
				item.vat_credit_amount
				* CASE
					WHEN purchase.voucher_type IN (3, 8, 13) THEN -1
					ELSE 1
				  END
			) FILTER (WHERE item.book = 'purchases'), 0)::text,
			coalesce(sum(
				item.withholding_amount
				* CASE
					WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
						IN (3, 8, 13)
					THEN -1 ELSE 1
				  END
			), 0)::text,
			coalesce(sum(
				item.perception_amount
				* CASE
					WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
						IN (3, 8, 13)
					THEN -1 ELSE 1
				  END
			), 0)::text,
			(
				coalesce(sum(
					item.vat_debit_amount
					* CASE
						WHEN voucher.voucher_type IN (3, 8, 13) THEN -1
						ELSE 1
					  END
				) FILTER (WHERE item.book = 'sales'), 0)
				- coalesce(sum(
					item.vat_credit_amount
					* CASE
						WHEN purchase.voucher_type IN (3, 8, 13) THEN -1
						ELSE 1
					  END
				) FILTER (WHERE item.book = 'purchases'), 0)
				- coalesce(sum(
					item.withholding_amount
					* CASE
						WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
							IN (3, 8, 13)
						THEN -1 ELSE 1
					  END
				), 0)
				- coalesce(sum(
					item.perception_amount
					* CASE
						WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
							IN (3, 8, 13)
						THEN -1 ELSE 1
					  END
				), 0)
			)::text
		  FROM fiscal.iva_periods AS period
		  LEFT JOIN fiscal.iva_period_items AS item
		    ON item.org_id = period.org_id
		   AND item.iva_period_id = period.id
		  LEFT JOIN fiscal.vouchers AS voucher
		    ON voucher.org_id = item.org_id
		   AND voucher.id = item.voucher_id
		  LEFT JOIN fiscal.purchase_vouchers AS purchase
		    ON purchase.org_id = item.org_id
		   AND purchase.id = item.purchase_voucher_id
		 WHERE period.environment = $1
		   AND period.period_month = $2
		 GROUP BY period.org_id, period.id`,
		environment,
		firstDay,
	).Scan(
		&response.Id,
		&status,
		&opening,
		&closingBalance,
		&response.Version,
		&response.CreatedAt,
		&response.UpdatedAt,
		&response.ClosedAt,
		&response.ExportedAt,
		&salesNet,
		&outputVAT,
		&purchasesNet,
		&inputVAT,
		&withholdings,
		&perceptions,
		&balance,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.IVASimpleWorkflowPeriod{}, fmt.Errorf(
			"%w: IVA period %s",
			errBusinessNotFound,
			normalizedPeriod,
		)
	}
	if err != nil {
		return api.IVASimpleWorkflowPeriod{}, fmt.Errorf("load IVA Simple workflow: %w", err)
	}
	response.Period = normalizedPeriod
	response.Environment = api.FiscalEnvironment(environment)
	response.Status = api.IVASimpleWorkflowStatus(status)
	response.OpeningBalance = api.DecimalAmount(opening)
	if closingBalance != nil {
		closing = *closingBalance
		value := api.DecimalAmount(closing)
		response.ClosingBalance = &value
	}
	response.Report = api.IVASimpleReport{
		Period:           normalizedPeriod,
		SalesNet:         api.DecimalAmount(salesNet),
		OutputVat:        api.DecimalAmount(outputVAT),
		PurchasesNet:     api.DecimalAmount(purchasesNet),
		InputVat:         api.DecimalAmount(inputVAT),
		Withholdings:     api.DecimalAmount(withholdings),
		Perceptions:      api.DecimalAmount(perceptions),
		Balance:          api.DecimalAmount(balance),
		ValidationErrors: make([]string, 0),
	}
	response.Exports, err = listIVAExportSummaries(ctx, tx, uuid.UUID(response.Id))
	if err != nil {
		return api.IVASimpleWorkflowPeriod{}, err
	}
	return response, nil
}

func listIVAExportSummaries(
	ctx context.Context,
	tx pgx.Tx,
	periodID uuid.UUID,
) ([]api.IVASimpleExportSummary, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			id,
			export_version,
			filename,
			media_type,
			sha256::text,
			created_at
		  FROM fiscal.iva_exports
		 WHERE iva_period_id = $1
		 ORDER BY export_version DESC, id DESC`,
		periodID,
	)
	if err != nil {
		return nil, fmt.Errorf("list IVA Simple exports: %w", err)
	}
	defer rows.Close()
	exports := make([]api.IVASimpleExportSummary, 0)
	for rows.Next() {
		var item api.IVASimpleExportSummary
		if err := rows.Scan(
			&item.Id,
			&item.ExportVersion,
			&item.Filename,
			&item.MediaType,
			&item.Sha256,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan IVA Simple export: %w", err)
		}
		exports = append(exports, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate IVA Simple exports: %w", err)
	}
	return exports, nil
}

func loadIVAExportArtifact(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	periodID uuid.UUID,
	exportID uuid.UUID,
) (api.IVASimpleExportArtifact, error) {
	var response api.IVASimpleExportArtifact
	err := tx.QueryRow(ctx, `
		SELECT
			id,
			export_version,
			filename,
			media_type,
			sha256::text,
			artifact,
			created_at
		  FROM fiscal.iva_exports
		 WHERE org_id = $1
		   AND iva_period_id = $2
		   AND id = $3`,
		organizationID,
		periodID,
		exportID,
	).Scan(
		&response.Id,
		&response.ExportVersion,
		&response.Filename,
		&response.MediaType,
		&response.Sha256,
		&response.ContentBase64,
		&response.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.IVASimpleExportArtifact{}, fmt.Errorf("%w: IVA export", errBusinessNotFound)
	}
	if err != nil {
		return api.IVASimpleExportArtifact{}, fmt.Errorf("load IVA Simple export: %w", err)
	}
	return response, nil
}

func loadIVAWorkflowRecords(
	ctx context.Context,
	tx pgx.Tx,
	periodID uuid.UUID,
	firstDay time.Time,
	nextMonth time.Time,
	environment string,
) ([]ar.IVARecord, ivaTotals, error) {
	all, _, err := loadIVARecords(ctx, tx, firstDay, nextMonth, environment)
	if err != nil {
		return nil, ivaTotals{}, err
	}
	rows, err := tx.Query(ctx, `
		SELECT
			item.book,
			coalesce(voucher.voucher_type, purchase.voucher_type),
			coalesce(point.code, purchase.point_of_sale),
			coalesce(voucher.voucher_number, purchase.voucher_number),
			coalesce(purchase.supplier_tax_id::text, '')
		  FROM fiscal.iva_period_items AS item
		  LEFT JOIN fiscal.vouchers AS voucher
		    ON voucher.org_id = item.org_id
		   AND voucher.id = item.voucher_id
		  LEFT JOIN fiscal.points_of_sale AS point
		    ON point.org_id = voucher.org_id
		   AND point.id = voucher.point_of_sale_id
		  LEFT JOIN fiscal.purchase_vouchers AS purchase
		    ON purchase.org_id = item.org_id
		   AND purchase.id = item.purchase_voucher_id
		 WHERE item.iva_period_id = $1`,
		periodID,
	)
	if err != nil {
		return nil, ivaTotals{}, fmt.Errorf("load IVA Simple snapshot keys: %w", err)
	}
	defer rows.Close()
	keys := make(map[string]struct{})
	for rows.Next() {
		var (
			book, supplier           string
			voucherType, pointOfSale int
			number                   int64
		)
		if err := rows.Scan(
			&book, &voucherType, &pointOfSale, &number, &supplier,
		); err != nil {
			return nil, ivaTotals{}, fmt.Errorf("scan IVA Simple snapshot key: %w", err)
		}
		keys[ivaRecordKey(book, supplier, voucherType, pointOfSale, number)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, ivaTotals{}, fmt.Errorf("iterate IVA Simple snapshot keys: %w", err)
	}

	records := make([]ar.IVARecord, 0, len(keys))
	var totals ivaTotals
	for _, record := range all {
		book := "sales"
		supplier := ""
		if record.Direction == ar.IVAPurchase {
			book = "purchases"
			supplier = record.CounterpartyDocument.Number
		}
		key := ivaRecordKey(
			book,
			supplier,
			int(record.VoucherType),
			record.PointOfSale,
			record.Number,
		)
		if _, included := keys[key]; !included {
			continue
		}
		delete(keys, key)
		records = append(records, record)
		sign := purchaseVoucherSign(int(record.VoucherType))
		net := record.Total.
			Sub(record.VAT).
			Sub(record.OtherTaxes)
		if record.Direction == ar.IVASale {
			totals.salesNet = totals.salesNet.Add(net.Mul(sign))
			totals.outputVAT = totals.outputVAT.Add(record.VAT.Mul(sign))
		} else {
			totals.purchasesNet = totals.purchasesNet.Add(net.Mul(sign))
			totals.inputVAT = totals.inputVAT.Add(record.ComputableVATCredit.Mul(sign))
		}
		totals.withholdings = totals.withholdings.Add(record.NationalPerceptions.Mul(sign))
		totals.perceptions = totals.perceptions.Add(record.VATPerceptions.Mul(sign))
	}
	if len(keys) != 0 {
		return nil, ivaTotals{}, fmt.Errorf(
			"%w: IVA snapshot contains %d unavailable documents",
			errBusinessInvalidRequest,
			len(keys),
		)
	}
	return records, totals, nil
}

func ivaRecordKey(
	book string,
	supplier string,
	voucherType int,
	pointOfSale int,
	number int64,
) string {
	return strings.Join([]string{
		book,
		supplier,
		strconv.Itoa(voucherType),
		strconv.Itoa(pointOfSale),
		strconv.FormatInt(number, 10),
	}, ":")
}

func fullIVARegistryBundle(files ar.IVASimpleFiles) ([]byte, error) {
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range []struct {
		name string
		body []byte
	}{
		{name: "ventas-comprobantes.txt", body: files.SalesVouchers},
		{name: "ventas-alicuotas.txt", body: files.SalesVAT},
		{name: "compras-comprobantes.txt", body: files.PurchaseVouchers},
		{name: "compras-alicuotas.txt", body: files.PurchaseVAT},
	} {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("create IVA Simple export: %w", err)
		}
		if _, err := entry.Write(file.body); err != nil {
			return nil, fmt.Errorf("write IVA Simple export: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close IVA Simple export: %w", err)
	}
	return output.Bytes(), nil
}
