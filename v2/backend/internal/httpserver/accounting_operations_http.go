package httpserver

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const maximumStatementBytes = 10 << 20

func (h *IAMAPI) createAccountingPeriod(
	w http.ResponseWriter,
	r *http.Request,
	_ api.CreateAccountingPeriodParams,
) {
	var input api.AccountingPeriodInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if input.StartDate.Time.IsZero() || input.EndDate.Time.Before(input.StartDate.Time) {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid accounting period")
		return
	}
	var response api.AccountingPeriod
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			periodID := uuid.New()
			code := accountingPeriodCode(input.StartDate.Time, input.EndDate.Time)
			var period accounting.Period
			err := tx.QueryRow(ctx, `
				INSERT INTO accounting.periods (
					org_id,
					id,
					code,
					start_date,
					end_date,
					status
				)
				VALUES ($1::uuid, $2, $3, $4, $5, 'open')
				RETURNING
					id,
					code,
					start_date,
					end_date,
					status,
					version
			`,
				active.OrganizationID,
				periodID,
				code,
				input.StartDate.Time,
				input.EndDate.Time,
			).Scan(
				&period.ID,
				&period.Name,
				&period.StartDate,
				&period.EndDate,
				&period.Status,
				&period.Version,
			)
			if err != nil {
				return mapAccountingError(err)
			}
			response = apiPeriod(period, accounting.CloseChecklist{})
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func accountingPeriodCode(from, to time.Time) string {
	if from.Day() == 1 &&
		from.Year() == to.Year() &&
		from.Month() == to.Month() {
		last := from.AddDate(0, 1, -1)
		if sameDate(last, to) {
			return from.Format("2006-01")
		}
	}
	return from.Format("20060102") + "-" + to.Format("20060102")
}

func sameDate(left, right time.Time) bool {
	leftYear, leftMonth, leftDay := left.Date()
	rightYear, rightMonth, rightDay := right.Date()
	return leftYear == rightYear && leftMonth == rightMonth && leftDay == rightDay
}

func (h *IAMAPI) createAnnualClosingDraft(
	w http.ResponseWriter,
	r *http.Request,
	periodID api.PeriodID,
	params api.CreateAnnualClosingDraftParams,
) {
	var input api.AnnualClosingDraftInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	var response api.JournalDraft
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			var (
				from    time.Time
				to      time.Time
				state   string
				version int64
			)
			if err := tx.QueryRow(ctx, `
				SELECT start_date, end_date, status, version
				  FROM accounting.periods
				 WHERE id = $1
				 FOR UPDATE
			`, periodID).Scan(&from, &to, &state, &version); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return errBusinessNotFound
				}
				return fmt.Errorf("load annual closing period: %w", err)
			}
			if version != input.Version {
				return accounting.ErrVersionConflict
			}
			if state == string(accounting.PeriodLocked) {
				return accounting.ErrPeriodClosed
			}
			currency, err := loadFunctionalCurrency(ctx, tx)
			if err != nil {
				return err
			}
			draft, err := service.CreateAnnualClosingDraft(
				ctx,
				scope,
				accounting.AnnualClosingCommand{
					From:               from,
					To:                 to,
					FunctionalCurrency: currency,
					IdempotencyKey:     string(params.IdempotencyKey),
				},
			)
			if err != nil {
				return err
			}
			response = apiDraft(draft)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) listFinancialAccounts(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListFinancialAccountsParams,
) {
	state := "active"
	if params.LifecycleState != nil {
		state = string(*params.LifecycleState)
	}
	if state != "active" && state != "archived" {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid financial account lifecycle state")
		return
	}
	query := ""
	if params.Query != nil {
		query = strings.TrimSpace(*params.Query)
	}
	items := make([]api.FinancialAccount, 0)
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			rows, err := tx.Query(ctx, `
				SELECT
					financial.id,
					financial.ledger_account_id,
					ledger.code,
					ledger.name,
					financial.account_type,
					financial.name,
					financial.currency_code,
					financial.institution_name,
					financial.external_reference,
					(financial.archived_at IS NOT NULL),
					financial.version
				  FROM accounting.financial_accounts AS financial
				  JOIN accounting.accounts AS ledger
				    ON ledger.org_id = financial.org_id
				   AND ledger.id = financial.ledger_account_id
				 WHERE (
					($1 = 'active' AND financial.archived_at IS NULL)
					OR ($1 = 'archived' AND financial.archived_at IS NOT NULL)
				 )
				   AND (
					$2 = ''
					OR financial.name ILIKE '%' || $2 || '%'
					OR coalesce(financial.institution_name, '') ILIKE '%' || $2 || '%'
					OR ledger.code ILIKE '%' || $2 || '%'
					OR ledger.name ILIKE '%' || $2 || '%'
				   )
				 ORDER BY lower(financial.name), financial.id
			`, state, query)
			if err != nil {
				return fmt.Errorf("list financial accounts: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				item, err := scanFinancialAccount(rows)
				if err != nil {
					return err
				}
				items = append(items, item)
			}
			return rows.Err()
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *IAMAPI) createFinancialAccount(
	w http.ResponseWriter,
	r *http.Request,
	_ api.CreateFinancialAccountParams,
) {
	var input api.FinancialAccountInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if err := validateFinancialAccountInput(input); err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	var response api.FinancialAccount
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			if err := ensureFinancialLedgerAccount(ctx, tx, input.LedgerAccountId); err != nil {
				return err
			}
			id := uuid.New()
			row := tx.QueryRow(ctx, `
				WITH inserted AS (
					INSERT INTO accounting.financial_accounts (
						org_id,
						id,
						ledger_account_id,
						account_type,
						name,
						currency_code,
						institution_name,
						external_reference
					)
					VALUES (
						$1::uuid,
						$2,
						$3,
						$4,
						$5,
						upper($6),
						NULLIF($7, ''),
						NULLIF($8, '')
					)
					RETURNING *
				)
				SELECT
					inserted.id,
					inserted.ledger_account_id,
					ledger.code,
					ledger.name,
					inserted.account_type,
					inserted.name,
					inserted.currency_code,
					inserted.institution_name,
					inserted.external_reference,
					false,
					inserted.version
				  FROM inserted
				  JOIN accounting.accounts AS ledger
				    ON ledger.org_id = inserted.org_id
				   AND ledger.id = inserted.ledger_account_id
			`,
				active.OrganizationID,
				id,
				input.LedgerAccountId,
				string(input.AccountType),
				strings.TrimSpace(input.Name),
				strings.ToUpper(input.Currency),
				trimmedPointer(input.InstitutionName),
				trimmedPointer(input.ExternalReference),
			)
			var err error
			response, err = scanFinancialAccount(row)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) updateFinancialAccount(
	w http.ResponseWriter,
	r *http.Request,
	financialAccountID api.FinancialAccountID,
	_ api.UpdateFinancialAccountParams,
) {
	var input api.UpdateFinancialAccountInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	base := api.FinancialAccountInput{
		AccountType:       input.AccountType,
		Currency:          input.Currency,
		ExternalReference: input.ExternalReference,
		InstitutionName:   input.InstitutionName,
		LedgerAccountId:   input.LedgerAccountId,
		Name:              input.Name,
	}
	if input.Version <= 0 {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Financial account version is required")
		return
	}
	if err := validateFinancialAccountInput(base); err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	var response api.FinancialAccount
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			if err := ensureFinancialLedgerAccount(ctx, tx, input.LedgerAccountId); err != nil {
				return err
			}
			if input.Archived {
				var open int
				if err := tx.QueryRow(ctx, `
					SELECT count(*)
					  FROM accounting.reconciliations
					 WHERE financial_account_id = $1
					   AND status <> 'closed'
				`, financialAccountID).Scan(&open); err != nil {
					return fmt.Errorf("check financial account reconciliations: %w", err)
				}
				if open != 0 {
					return errBusinessInvalidTransition
				}
			}
			row := tx.QueryRow(ctx, `
				WITH updated AS (
					UPDATE accounting.financial_accounts
					   SET ledger_account_id = $2,
					       account_type = $3,
					       name = $4,
					       currency_code = upper($5),
					       institution_name = NULLIF($6, ''),
					       external_reference = NULLIF($7, ''),
					       archived_at = CASE WHEN $8 THEN coalesce(archived_at, now()) ELSE NULL END,
					       version = version + 1,
					       updated_at = now()
					 WHERE id = $1
					   AND version = $9
					RETURNING *
				)
				SELECT
					updated.id,
					updated.ledger_account_id,
					ledger.code,
					ledger.name,
					updated.account_type,
					updated.name,
					updated.currency_code,
					updated.institution_name,
					updated.external_reference,
					(updated.archived_at IS NOT NULL),
					updated.version
				  FROM updated
				  JOIN accounting.accounts AS ledger
				    ON ledger.org_id = updated.org_id
				   AND ledger.id = updated.ledger_account_id
			`,
				financialAccountID,
				input.LedgerAccountId,
				string(input.AccountType),
				strings.TrimSpace(input.Name),
				strings.ToUpper(input.Currency),
				trimmedPointer(input.InstitutionName),
				trimmedPointer(input.ExternalReference),
				input.Archived,
				input.Version,
			)
			var err error
			response, err = scanFinancialAccount(row)
			if errors.Is(err, pgx.ErrNoRows) {
				var exists bool
				if queryErr := tx.QueryRow(ctx, `
					SELECT EXISTS (
						SELECT 1
						  FROM accounting.financial_accounts
						 WHERE id = $1
					)
				`, financialAccountID).Scan(&exists); queryErr != nil {
					return queryErr
				}
				if exists {
					return errBusinessVersionConflict
				}
				return errBusinessNotFound
			}
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func validateFinancialAccountInput(input api.FinancialAccountInput) error {
	if input.LedgerAccountId == uuid.Nil ||
		!input.AccountType.Valid() ||
		strings.TrimSpace(input.Name) == "" {
		return errors.New("Financial account, ledger account, and type are required")
	}
	if _, err := accounting.NewCurrency(input.Currency); err != nil {
		return err
	}
	return nil
}

func ensureFinancialLedgerAccount(
	ctx context.Context,
	tx pgx.Tx,
	accountID uuid.UUID,
) error {
	var valid bool
	if err := tx.QueryRow(ctx, `
		SELECT postable
		   AND archived_at IS NULL
		   AND trashed_at IS NULL
		  FROM accounting.accounts
		 WHERE id = $1
	`, accountID).Scan(&valid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errBusinessNotFound
		}
		return fmt.Errorf("load financial ledger account: %w", err)
	}
	if !valid {
		return fmt.Errorf("%w: financial ledger account must be active and postable", accounting.ErrAccountNotPostable)
	}
	return nil
}

func trimmedPointer(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func scanFinancialAccount(
	row interface{ Scan(...any) error },
) (api.FinancialAccount, error) {
	var (
		item              api.FinancialAccount
		accountType       string
		institutionName   *string
		externalReference *string
	)
	if err := row.Scan(
		&item.Id,
		&item.LedgerAccountId,
		&item.LedgerAccountCode,
		&item.LedgerAccountName,
		&accountType,
		&item.Name,
		&item.Currency,
		&institutionName,
		&externalReference,
		&item.Archived,
		&item.Version,
	); err != nil {
		return api.FinancialAccount{}, err
	}
	item.AccountType = api.FinancialAccountType(accountType)
	item.InstitutionName = institutionName
	item.ExternalReference = externalReference
	return item, nil
}

func (h *IAMAPI) importAccountingStatement(
	w http.ResponseWriter,
	r *http.Request,
	_ api.ImportAccountingStatementParams,
) {
	var input api.StatementImportInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if len(input.ContentBase64) == 0 || len(input.ContentBase64) > maximumStatementBytes {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Statement file must be between 1 byte and 10 MiB")
		return
	}
	format := accounting.StatementFormat(input.Format)
	currency, err := accounting.NewCurrency(input.Currency)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	var response api.StatementImport
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			var configuredCurrency string
			if err := tx.QueryRow(ctx, `
				SELECT currency_code
				  FROM accounting.financial_accounts
				 WHERE id = $1
				   AND archived_at IS NULL
			`, input.FinancialAccountId).Scan(&configuredCurrency); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return errBusinessNotFound
				}
				return fmt.Errorf("load statement financial account: %w", err)
			}
			if configuredCurrency != currency.Code() {
				return fmt.Errorf("%w: statement currency differs from financial account", accounting.ErrInvalidArgument)
			}
			statement, err := service.ImportStatement(
				ctx,
				scope,
				input.FinancialAccountId,
				strings.TrimSpace(input.FileName),
				format,
				input.ContentBase64,
				currency,
			)
			if err != nil {
				return err
			}
			response = apiStatementImport(statement)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) suggestAccountingMatches(
	w http.ResponseWriter,
	r *http.Request,
	statementImportID api.StatementImportID,
	params api.SuggestAccountingMatchesParams,
) {
	if params.To.Time.Before(params.From.Time) {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Suggestion end date cannot precede start date")
		return
	}
	maxDays := 3
	if params.MaxDays != nil {
		maxDays = *params.MaxDays
	}
	response := make([]api.ReconciliationSuggestion, 0)
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			var belongs bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					  FROM accounting.statement_imports
					 WHERE id = $1
					   AND financial_account_id = $2
				)
			`, statementImportID, params.FinancialAccountId).Scan(&belongs); err != nil {
				return fmt.Errorf("validate statement import scope: %w", err)
			}
			if !belongs {
				return errBusinessNotFound
			}
			suggestions, err := service.SuggestReconciliationMatches(
				ctx,
				scope,
				statementImportID,
				params.FinancialAccountId,
				params.From.Time,
				params.To.Time,
				maxDays,
			)
			if err != nil {
				return err
			}
			for _, suggestion := range suggestions {
				response = append(response, api.ReconciliationSuggestion{
					Amount:              suggestion.Amount.String(),
					JournalLineId:       suggestion.JournalLineID,
					Reasons:             append([]string(nil), suggestion.Reasons...),
					Score:               suggestion.Score,
					StatementMovementId: suggestion.StatementMovementID,
				})
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func apiStatementImport(statement accounting.StatementImport) api.StatementImport {
	movements := make([]api.StatementMovement, 0, len(statement.Movements))
	for _, movement := range statement.Movements {
		movements = append(movements, api.StatementMovement{
			Amount:      movement.Amount.String(),
			BookedAt:    openapi_types.Date{Time: movement.BookedAt},
			Currency:    movement.Currency.Code(),
			Description: movement.Description,
			Fingerprint: movement.Fingerprint,
			Id:          movement.ID,
			Reference:   movement.Reference,
			ValueAt:     openapi_types.Date{Time: movement.ValueAt},
		})
	}
	return api.StatementImport{
		Currency:           statement.Currency.Code(),
		FileName:           statement.FileName,
		FinancialAccountId: statement.FinancialAccountID,
		Format:             api.StatementImportFormat(statement.Format),
		Id:                 statement.ID,
		ImportedAt:         statement.ImportedAt,
		Movements:          movements,
		Sha256:             statement.SHA256,
	}
}

func (h *IAMAPI) getAccountingReconciliation(
	w http.ResponseWriter,
	r *http.Request,
	reconciliationID api.ReconciliationID,
) {
	var response api.Reconciliation
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			reconciliation, err := service.GetReconciliation(ctx, scope, reconciliationID)
			if err != nil {
				return err
			}
			currency, err := reconciliationCurrency(ctx, tx, reconciliation.FinancialAccountID)
			if err != nil {
				return err
			}
			response = apiReconciliation(reconciliation, currency)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) updateAccountingReconciliation(
	w http.ResponseWriter,
	r *http.Request,
	reconciliationID api.ReconciliationID,
	_ api.UpdateAccountingReconciliationParams,
) {
	var input api.UpdateReconciliationInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	opening, err := accounting.ParseAmount(input.OpeningBalance)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid opening balance")
		return
	}
	closing, err := accounting.ParseAmount(input.StatementBalance)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid statement balance")
		return
	}
	matches, err := reconciliationMatchesFromAPI(input.Matches)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	var response api.Reconciliation
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			current, err := service.GetReconciliation(ctx, scope, reconciliationID)
			if err != nil {
				return err
			}
			current.StatementOpening = opening
			current.StatementClosing = closing
			current.Matches = matches
			saved, err := service.SaveReconciliation(ctx, scope, current, input.Version)
			if err != nil {
				return err
			}
			currency, err := reconciliationCurrency(ctx, tx, saved.FinancialAccountID)
			if err != nil {
				return err
			}
			response = apiReconciliation(saved, currency)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) transitionAccountingReconciliation(
	w http.ResponseWriter,
	r *http.Request,
	reconciliationID api.ReconciliationID,
	action api.TransitionAccountingReconciliationParamsReconciliationAction,
	_ api.TransitionAccountingReconciliationParams,
) {
	var input api.VersionedCommandInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if input.Version <= 0 {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Reconciliation version is required")
		return
	}
	var response api.Reconciliation
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			var (
				updated accounting.Reconciliation
				err     error
			)
			switch string(action) {
			case "close":
				updated, err = service.CloseReconciliation(
					ctx,
					scope,
					reconciliationID,
					input.Version,
				)
			case "reopen":
				updated, err = service.ReopenReconciliation(
					ctx,
					scope,
					reconciliationID,
					input.Version,
					stringValue(input.Reason),
				)
			default:
				return accounting.ErrInvalidArgument
			}
			if err != nil {
				return err
			}
			currency, err := reconciliationCurrency(ctx, tx, updated.FinancialAccountID)
			if err != nil {
				return err
			}
			response = apiReconciliation(updated, currency)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func reconciliationMatchesFromAPI(
	input []api.ReconciliationMatchInput,
) ([]accounting.ReconciliationMatch, error) {
	matches := make([]accounting.ReconciliationMatch, 0, len(input))
	for index, item := range input {
		statementAmount, err := accounting.ParseAmount(item.StatementAmount)
		if err != nil || statementAmount.Sign() <= 0 {
			return nil, fmt.Errorf("match %d has an invalid statement amount", index+1)
		}
		ledgerAmount, err := accounting.ParseAmount(item.LedgerAmount)
		if err != nil || ledgerAmount.Sign() <= 0 {
			return nil, fmt.Errorf("match %d has an invalid ledger amount", index+1)
		}
		if !statementAmount.Equal(ledgerAmount) {
			return nil, fmt.Errorf("match %d allocations must be equal", index+1)
		}
		matches = append(matches, accounting.ReconciliationMatch{
			StatementMovementID: item.StatementMovementId,
			JournalLineID:       item.JournalLineId,
			StatementAmount:     statementAmount,
			LedgerAmount:        ledgerAmount,
		})
	}
	return matches, nil
}

func reconciliationCurrency(
	ctx context.Context,
	tx pgx.Tx,
	financialAccountID uuid.UUID,
) (accounting.Currency, error) {
	var currencyCode string
	if err := tx.QueryRow(ctx, `
		SELECT currency_code
		  FROM accounting.financial_accounts
		 WHERE id = $1
	`, financialAccountID).Scan(&currencyCode); err != nil {
		return accounting.Currency{}, err
	}
	return accounting.NewCurrency(currencyCode)
}

func apiReconciliation(
	reconciliation accounting.Reconciliation,
	currency accounting.Currency,
) api.Reconciliation {
	matches := make([]api.ReconciliationMatch, 0, len(reconciliation.Matches))
	for _, match := range reconciliation.Matches {
		matches = append(matches, api.ReconciliationMatch{
			CreatedAt:           match.CreatedAt,
			Id:                  match.ID,
			JournalLineId:       match.JournalLineID,
			LedgerAmount:        match.LedgerAmount.String(),
			StatementAmount:     match.StatementAmount.String(),
			StatementMovementId: match.StatementMovementID,
		})
	}
	state := api.ReconciliationStateDraft
	if reconciliation.Status == accounting.ReconciliationClosed {
		state = api.ReconciliationStateCompleted
	} else if reconciliation.ReopenedReason != "" {
		state = api.ReconciliationStateReopened
	}
	return api.Reconciliation{
		AccountId:        reconciliation.FinancialAccountID,
		Currency:         currency.Code(),
		Difference:       reconciliation.Difference().String(),
		Id:               reconciliation.ID,
		LedgerBalance:    reconciliation.LedgerClosing.String(),
		Matches:          matches,
		OpeningBalance:   reconciliation.StatementOpening.String(),
		PeriodStart:      openapi_types.Date{Time: reconciliation.PeriodStart},
		State:            state,
		StatementBalance: reconciliation.StatementClosing.String(),
		StatementDate:    openapi_types.Date{Time: reconciliation.PeriodEnd},
		Version:          reconciliation.Version,
	}
}

func (h *IAMAPI) importInflationIndices(
	w http.ResponseWriter,
	r *http.Request,
	_ api.ImportInflationIndicesParams,
) {
	var input api.ImportInflationIndicesJSONBody
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	indices := make([]accounting.InflationIndex, 0, len(input))
	for index, item := range input {
		period, err := time.Parse("2006-01", item.Period)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", fmt.Sprintf("Invalid index period at row %d", index+1))
			return
		}
		value, err := accounting.ParseDecimal(item.Value)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", fmt.Sprintf("Invalid index value at row %d", index+1))
			return
		}
		inflationIndex, err := accounting.NewInflationIndex(
			period,
			value,
			item.Source,
			item.SourceDocument,
		)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
			return
		}
		indices = append(indices, inflationIndex)
	}
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			_ pgx.Tx,
		) error {
			return service.ImportInflationIndices(ctx, scope, indices)
		},
	) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *IAMAPI) createCurrencyRevaluation(
	w http.ResponseWriter,
	r *http.Request,
	_ api.CreateCurrencyRevaluationParams,
) {
	var input api.CurrencyRevaluationInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	var response api.CurrencyRevaluation
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			functionalCurrency, err := loadFunctionalCurrency(ctx, tx)
			if err != nil {
				return err
			}
			rates := make([]accounting.ClosingExchangeRate, 0, len(input.Rates))
			for _, item := range input.Rates {
				currency, err := accounting.NewCurrency(item.Currency)
				if err != nil {
					return err
				}
				rate, err := accounting.ParseExchangeRate(item.Rate)
				if err != nil {
					return err
				}
				sourceReference := ""
				if item.SourceReference != nil {
					sourceReference = strings.TrimSpace(*item.SourceReference)
				}
				closingRate := accounting.ClosingExchangeRate{
					Date:               item.Date.Time,
					Currency:           currency,
					FunctionalCurrency: functionalCurrency,
					Rate:               rate,
					Source:             strings.TrimSpace(item.Source),
					SourceReference:    sourceReference,
					SourceChecksum:     item.SourceChecksum,
				}
				if err := closingRate.Validate(); err != nil {
					return err
				}
				rates = append(rates, closingRate)
			}
			workpaper, err := service.CreateCurrencyRevaluation(
				ctx,
				scope,
				input.ClosingDate.Time,
				functionalCurrency,
				rates,
			)
			if err != nil {
				return err
			}
			response = apiCurrencyRevaluation(workpaper)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func apiCurrencyRevaluation(
	workpaper accounting.CurrencyRevaluationWorkpaper,
) api.CurrencyRevaluation {
	rates := make([]api.ClosingExchangeRateInput, 0, len(workpaper.Rates))
	for _, rate := range workpaper.Rates {
		var reference *string
		if rate.SourceReference != "" {
			value := rate.SourceReference
			reference = &value
		}
		rates = append(rates, api.ClosingExchangeRateInput{
			Currency:        rate.Currency.Code(),
			Date:            openapi_types.Date{Time: rate.Date},
			Rate:            rate.Rate.String(),
			Source:          rate.Source,
			SourceChecksum:  rate.SourceChecksum,
			SourceReference: reference,
		})
	}
	lines := make([]api.CurrencyRevaluationLine, 0, len(workpaper.Lines))
	for _, line := range workpaper.Lines {
		lines = append(lines, api.CurrencyRevaluationLine{
			AccountCode:        line.AccountCode,
			AccountId:          line.AccountID,
			AccountName:        line.AccountName,
			CarryingAmount:     line.CarryingAmount.String(),
			ClosingRate:        line.ClosingRate.String(),
			Currency:           line.Currency.Code(),
			CurrencyAmount:     line.CurrencyAmount.String(),
			ExchangeDifference: line.ExchangeDifference.String(),
			RevaluedAmount:     line.RevaluedAmount.String(),
		})
	}
	return api.CurrencyRevaluation{
		ClosingDate:        openapi_types.Date{Time: workpaper.ClosingDate},
		Draft:              apiDraft(workpaper.Draft),
		FunctionalCurrency: workpaper.FunctionalCurrency.Code(),
		Id:                 workpaper.ID,
		Lines:              lines,
		NetResult:          workpaper.NetResult.String(),
		Rates:              rates,
		SourceChecksum:     workpaper.SourceChecksum,
		TotalGain:          workpaper.TotalGain.String(),
		TotalLoss:          workpaper.TotalLoss.String(),
	}
}

func apiInflationLines(
	lines []accounting.InflationCalculationLine,
) []api.InflationAdjustmentLine {
	result := make([]api.InflationAdjustmentLine, 0, len(lines))
	for _, line := range lines {
		result = append(result, api.InflationAdjustmentLine{
			AccountCode:  line.AccountCode,
			AccountId:    line.AccountID,
			AccountName:  line.AccountName,
			Adjustment:   line.Adjustment.String(),
			ClosingIndex: line.ClosingIndex.String(),
			Coefficient:  line.Coefficient.String(),
			Historical:   line.Historical.String(),
			OriginDate:   openapi_types.Date{Time: line.OriginDate},
			OriginIndex:  line.OriginIndex.String(),
			Restated:     line.Restated.String(),
		})
	}
	return result
}

func (h *IAMAPI) exportAccountingReport(
	w http.ResponseWriter,
	r *http.Request,
	report api.ExportAccountingReportParamsReport,
	params api.ExportAccountingReportParams,
) {
	if params.To.Time.Before(params.From.Time) {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Report end date cannot precede start date")
		return
	}
	var (
		body        []byte
		contentType string
		extension   string
	)
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			table, err := accountingReportTable(
				ctx,
				service,
				scope,
				tx,
				string(report),
				params.From.Time,
				params.To.Time,
				params.AccountId,
				params.FinancialAccountId,
			)
			if err != nil {
				return err
			}
			switch string(params.Format) {
			case "csv":
				body, err = exportReportCSV(table)
				contentType, extension = "text/csv; charset=utf-8", "csv"
			case "xlsx":
				body, err = accounting.ExportReportXLSX(table)
				contentType, extension = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"
			case "pdf":
				body, err = accounting.ExportReportPDF(table)
				contentType, extension = "application/pdf", "pdf"
			default:
				return accounting.ErrInvalidArgument
			}
			return err
		},
	) {
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s-%s-%s.%s"`,
			report,
			params.From.Time.Format("20060102"),
			params.To.Time.Format("20060102"),
			extension,
		),
	)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func accountingReportTable(
	ctx context.Context,
	service *accounting.Service,
	scope accounting.Scope,
	tx pgx.Tx,
	report string,
	from, to time.Time,
	accountID *openapi_types.UUID,
	financialAccountID *openapi_types.UUID,
) (accounting.ReportTable, error) {
	switch report {
	case "journal":
		entries := make([]accounting.JournalEntry, 0)
		after := int64(0)
		for {
			page, err := service.Journal(ctx, scope, accounting.JournalFilter{
				From:        &from,
				To:          &to,
				AfterNumber: after,
				Limit:       200,
			})
			if err != nil {
				return accounting.ReportTable{}, err
			}
			entries = append(entries, page.Items...)
			if page.NextCursor == "" {
				break
			}
			after, err = strconv.ParseInt(page.NextCursor, 10, 64)
			if err != nil {
				return accounting.ReportTable{}, fmt.Errorf("parse journal export cursor: %w", err)
			}
		}
		return accounting.JournalReportTable(entries), nil
	case "trial-balance":
		trial, err := collectTrialBalance(
			ctx,
			service,
			scope,
			accounting.TrialBalanceFilter{
				From:  from,
				To:    to,
				Limit: 200,
			},
		)
		if err != nil {
			return accounting.ReportTable{}, err
		}
		currency, err := loadFunctionalCurrency(ctx, tx)
		if err != nil {
			return accounting.ReportTable{}, err
		}
		return trialBalanceExportTable(trial, currency), nil
	case "general-ledger":
		if accountID == nil {
			return accounting.ReportTable{}, fmt.Errorf("%w: account_id is required", accounting.ErrInvalidArgument)
		}
		ledger, err := service.GeneralLedger(ctx, scope, *accountID, from, to)
		if err != nil {
			return accounting.ReportTable{}, err
		}
		return accounting.GeneralLedgerReportTable(ledger), nil
	case "financial-activity":
		if financialAccountID == nil {
			return accounting.ReportTable{}, fmt.Errorf(
				"%w: financial_account_id is required",
				accounting.ErrInvalidArgument,
			)
		}
		ledgerAccountID, _, err := loadFinancialLedgerAccount(
			ctx,
			tx,
			scope.OrganizationID,
			*financialAccountID,
		)
		if err != nil {
			return accounting.ReportTable{}, err
		}
		ledger, err := service.GeneralLedger(
			ctx,
			scope,
			ledgerAccountID,
			from,
			to,
		)
		if err != nil {
			return accounting.ReportTable{}, err
		}
		return accounting.GeneralLedgerReportTable(ledger), nil
	case "balance-sheet":
		statement, err := service.BalanceSheet(ctx, scope, from, to)
		if err != nil {
			return accounting.ReportTable{}, err
		}
		return accounting.BalanceSheetReportTable(statement), nil
	case "income-statement":
		statement, err := service.IncomeStatement(ctx, scope, from, to)
		if err != nil {
			return accounting.ReportTable{}, err
		}
		return accounting.IncomeStatementReportTable(statement), nil
	case "aging", "vat-position":
		response := api.AccountingReport{
			Currency: "ARS",
			From:     openapi_types.Date{Time: from},
			Report:   report,
			Rows:     make([]api.AccountingReportRow, 0),
			To:       openapi_types.Date{Time: to},
		}
		var err error
		if report == "aging" {
			err = loadAgingReport(ctx, tx, to, &response)
		} else {
			err = loadVATPositionReport(ctx, tx, from, to, &response)
		}
		if err != nil {
			return accounting.ReportTable{}, err
		}
		return genericAPIReportTable(response), nil
	default:
		return accounting.ReportTable{}, fmt.Errorf("%w: unsupported report", accounting.ErrInvalidArgument)
	}
}

func genericAPIReportTable(report api.AccountingReport) accounting.ReportTable {
	rows := make([][]accounting.ReportCell, 0, len(report.Rows)+1)
	for _, row := range report.Rows {
		rows = append(rows, []accounting.ReportCell{
			accounting.TextReportCell(row.Label),
			{Value: row.Debit, Numeric: true},
			{Value: row.Credit, Numeric: true},
			{Value: row.Balance, Numeric: true},
		})
	}
	rows = append(rows, []accounting.ReportCell{
		accounting.TextReportCell("TOTAL"),
		{Value: report.TotalDebit, Numeric: true},
		{Value: report.TotalCredit, Numeric: true},
		{Value: "0", Numeric: true},
	})
	return accounting.ReportTable{
		Title:    report.Report,
		Subtitle: report.From.Time.Format("2006-01-02") + " — " + report.To.Time.Format("2006-01-02"),
		Columns:  []string{"Cuenta / rubro", "Debe", "Haber", "Saldo"},
		Rows:     rows,
	}
}

func exportReportCSV(report accounting.ReportTable) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	buffer.Write([]byte{0xef, 0xbb, 0xbf})
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{report.Title, report.Subtitle}); err != nil {
		return nil, err
	}
	if err := writer.Write(report.Columns); err != nil {
		return nil, err
	}
	for _, row := range report.Rows {
		values := make([]string, len(row))
		for index, cell := range row {
			values[index] = cell.Value
		}
		if err := writer.Write(values); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
