package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	accountingpg "github.com/devpablocristo/pymes/v2/backend/internal/accounting/postgres"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *IAMAPI) listAccountingMappings(w http.ResponseWriter, r *http.Request) {
	response := make([]api.AccountingMapping, 0)
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			_ pgx.Tx,
		) error {
			mappings, err := service.ListAccountMappings(ctx, scope)
			if err != nil {
				return err
			}
			for _, mapping := range mappings {
				response = append(response, api.AccountingMapping{
					AccountCode: mapping.AccountCode,
					AccountId:   mapping.AccountID,
					AccountName: mapping.AccountName,
					Role:        mapping.Role,
					Version:     mapping.Version,
				})
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) updateAccountingMappings(
	w http.ResponseWriter,
	r *http.Request,
	_ api.UpdateAccountingMappingsParams,
) {
	var input []api.AccountingMappingInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if len(input) == 0 || len(input) > 100 {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "At least one mapping is required")
		return
	}
	response := make([]api.AccountingMapping, 0, len(input))
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
			seen := make(map[string]struct{}, len(input))
			for _, command := range input {
				role := strings.TrimSpace(command.Role)
				if _, duplicate := seen[role]; duplicate {
					return fmt.Errorf("%w: duplicate mapping role", accounting.ErrInvalidArgument)
				}
				seen[role] = struct{}{}
				expectedVersion := int64(0)
				if command.Version != nil {
					expectedVersion = *command.Version
				}
				mapping, err := service.SetAccountMapping(
					ctx,
					scope,
					role,
					command.AccountId,
					expectedVersion,
				)
				if err != nil {
					return err
				}
				response = append(response, api.AccountingMapping{
					AccountCode: mapping.AccountCode,
					AccountId:   mapping.AccountID,
					AccountName: mapping.AccountName,
					Role:        mapping.Role,
					Version:     mapping.Version,
				})
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) listAccountingAccounts(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListAccountingAccountsParams,
) {
	if params.LifecycleState != nil && !params.LifecycleState.Valid() {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid lifecycle state")
		return
	}
	cursor, err := decodeKeysetCursor((*string)(params.Cursor))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	limit := accountingAPILimit(params.Limit)
	query := ""
	if params.Query != nil {
		query = strings.TrimSpace(*params.Query)
	}
	lifecycle := api.LifecycleStateActive
	if params.LifecycleState != nil {
		lifecycle = *params.LifecycleState
	}

	var (
		items []api.AccountingAccount
		total int
	)
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
			lifecyclePredicate := "account.archived_at IS NULL AND account.trashed_at IS NULL"
			switch lifecycle {
			case api.LifecycleStateArchived:
				lifecyclePredicate = "account.archived_at IS NOT NULL AND account.trashed_at IS NULL"
			case api.LifecycleStateTrashed:
				lifecyclePredicate = "account.trashed_at IS NOT NULL"
			}
			if err := tx.QueryRow(ctx, `
				SELECT count(*)
				  FROM accounting.accounts AS account
				 WHERE (`+lifecyclePredicate+`)
				   AND (
				       $1 = ''
				       OR account.code ILIKE '%' || $1 || '%'
				       OR account.name ILIKE '%' || $1 || '%'
				   )
			`, query).Scan(&total); err != nil {
				return fmt.Errorf("count accounting accounts: %w", err)
			}
			rows, err := tx.Query(ctx, `
				SELECT
					account.id,
					account.code,
					account.name,
					account.account_class,
					account.normal_balance,
					account.monetary_class,
					account.parent_id,
					account.posting_allowed,
					account.version
				  FROM accounting.accounts AS account
				 WHERE (`+lifecyclePredicate+`)
				   AND (
				       $1 = ''
				       OR account.code ILIKE '%' || $1 || '%'
				       OR account.name ILIKE '%' || $1 || '%'
				   )
				   AND (
				       $2 = ''
				       OR (lower(account.code), account.id) > ($2, $3::uuid)
				   )
				 ORDER BY lower(account.code), account.id
				 LIMIT $4
			`, query, cursor.Sort, nullableCursorID(cursor.ID), limit+1)
			if err != nil {
				return fmt.Errorf("list accounting accounts: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				item, err := scanAccountingAccount(rows, lifecycle)
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
	var next *string
	if len(items) > limit {
		last := items[limit-1]
		next = encodeKeysetCursor(strings.ToLower(last.Code), last.Id.String())
		items = items[:limit]
	}
	writeJSON(w, http.StatusOK, api.AccountingAccountList{
		Items: items,
		Page:  api.PageInfo{NextCursor: next, Total: total},
	})
}

func (h *IAMAPI) createAccountingAccount(
	w http.ResponseWriter,
	r *http.Request,
	_ api.CreateAccountingAccountParams,
) {
	var input api.AccountingAccountInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	command, err := accountCommand(input)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	var response api.AccountingAccount
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
			created, err := service.CreateAccount(ctx, scope, command)
			if err != nil {
				return err
			}
			response = apiAccount(created, api.LifecycleStateActive)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) getAccountingAccount(w http.ResponseWriter, r *http.Request, accountID api.AccountID) {
	var response api.AccountingAccount
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
			var lifecycle string
			err := tx.QueryRow(ctx, `
				SELECT
					id,
					code,
					name,
					account_class,
					normal_balance,
					monetary_class,
					parent_id,
					posting_allowed,
					version,
					CASE
						WHEN trashed_at IS NOT NULL THEN 'trashed'
						WHEN archived_at IS NOT NULL THEN 'archived'
						ELSE 'active'
					END
				  FROM accounting.accounts
				 WHERE id = $1
			`, accountID).Scan(
				&response.Id,
				&response.Code,
				&response.Name,
				&response.AccountType,
				&response.NormalBalance,
				&response.MonetaryClassification,
				&response.ParentId,
				&response.Postable,
				&response.Version,
				&lifecycle,
			)
			if errors.Is(err, pgx.ErrNoRows) {
				return errBusinessNotFound
			}
			if err != nil {
				return fmt.Errorf("get accounting account: %w", err)
			}
			response.AccountType = apiAccountTypeFromDB(string(response.AccountType))
			response.LifecycleState = api.LifecycleState(lifecycle)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) updateAccountingAccount(
	w http.ResponseWriter,
	r *http.Request,
	accountID api.AccountID,
	_ api.UpdateAccountingAccountParams,
) {
	var input api.UpdateAccountingAccountInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	base := api.AccountingAccountInput{
		AccountType:            input.AccountType,
		Code:                   input.Code,
		MonetaryClassification: input.MonetaryClassification,
		Name:                   input.Name,
		NormalBalance:          input.NormalBalance,
		ParentId:               input.ParentId,
		Postable:               input.Postable,
	}
	createCommand, err := accountCommand(base)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	var response api.AccountingAccount
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
			updated, err := service.UpdateAccount(ctx, scope, accounting.UpdateAccountCommand{
				ID:              accountID,
				ExpectedVersion: input.Version,
				Code:            createCommand.Code,
				Name:            createCommand.Name,
				Class:           createCommand.Class,
				NormalBalance:   createCommand.NormalBalance,
				Monetary:        createCommand.Monetary,
				ParentID:        createCommand.ParentID,
				Postable:        createCommand.Postable,
			})
			if err != nil {
				return err
			}
			response = apiAccount(updated, api.LifecycleStateActive)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) transitionAccountingAccount(
	w http.ResponseWriter,
	r *http.Request,
	accountID api.AccountID,
	action api.TransitionAccountingAccountParamsLifecycleAction,
	_ api.TransitionAccountingAccountParams,
) {
	var input api.VersionedCommandInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	var response api.AccountingAccount
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
			switch string(action) {
			case "archive":
				account, err := service.ArchiveAccount(ctx, scope, accountID, input.Version)
				if err != nil {
					return err
				}
				response = apiAccount(account, api.LifecycleStateArchived)
			case "unarchive":
				account, err := service.RestoreAccount(ctx, scope, accountID, input.Version)
				if err != nil {
					return err
				}
				response = apiAccount(account, api.LifecycleStateActive)
			case "trash":
				if err := service.TrashUnusedAccount(ctx, scope, accountID, input.Version); err != nil {
					return err
				}
				return loadAccountingAccountAfterTransition(
					ctx,
					tx,
					accountID,
					api.LifecycleStateTrashed,
					&response,
				)
			case "restore":
				tag, err := tx.Exec(ctx, `
					UPDATE accounting.accounts
					   SET trashed_at = NULL,
					       version = version + 1,
					       updated_at = now()
					 WHERE id = $1
					   AND version = $2
					   AND trashed_at IS NOT NULL
					   AND NOT EXISTS (
					       SELECT 1
					         FROM accounting.accounts AS active_account
					        WHERE active_account.org_id = accounting.accounts.org_id
					          AND active_account.code = accounting.accounts.code
					          AND active_account.id <> accounting.accounts.id
					          AND active_account.trashed_at IS NULL
					   )
				`, accountID, input.Version)
				if err != nil {
					return fmt.Errorf("restore trashed accounting account: %w", err)
				}
				if tag.RowsAffected() != 1 {
					return errBusinessVersionConflict
				}
				return loadAccountingAccountAfterTransition(
					ctx,
					tx,
					accountID,
					api.LifecycleStateActive,
					&response,
				)
			default:
				return errBusinessInvalidTransition
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) previewInflationAdjustment(
	w http.ResponseWriter,
	r *http.Request,
	_ api.PreviewInflationAdjustmentParams,
) {
	var input api.InflationAdjustmentInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	var response api.InflationAdjustment
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
			var closingDate time.Time
			if err := tx.QueryRow(ctx, `
				SELECT end_date
				  FROM accounting.periods
				 WHERE id = $1
			`, input.PeriodId).Scan(&closingDate); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return errBusinessNotFound
				}
				return fmt.Errorf("load inflation period: %w", err)
			}
			functionalCurrency, err := loadFunctionalCurrency(ctx, tx)
			if err != nil {
				return err
			}
			workpaper, err := service.CreateInflationAdjustment(
				ctx,
				scope,
				closingDate,
				functionalCurrency,
			)
			if err != nil {
				return err
			}
			response = api.InflationAdjustment{
				ClosingDate:    openapi_types.Date{Time: workpaper.ClosingDate},
				Draft:          apiDraft(workpaper.Draft),
				Id:             workpaper.ID,
				Lines:          apiInflationLines(workpaper.Lines),
				PeriodId:       input.PeriodId,
				Recpam:         workpaper.RECPAM.String(),
				Source:         workpaper.Source,
				SourceChecksum: workpaper.SourceChecksum,
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) listJournalDrafts(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListJournalDraftsParams,
) {
	cursor, err := decodeKeysetCursor((*string)(params.Cursor))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	limit := accountingAPILimit(params.Limit)
	var drafts []accounting.Draft
	var total int
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
			if err := tx.QueryRow(ctx, `
				SELECT count(*)
				  FROM accounting.drafts
				 WHERE status = 'active'
			`).Scan(&total); err != nil {
				return fmt.Errorf("count journal drafts: %w", err)
			}
			rows, err := tx.Query(ctx, `
				SELECT id
				  FROM accounting.drafts
				 WHERE status = 'active'
				   AND (
				       $1 = ''
				       OR (entry_date, id) < ($1::date, $2::uuid)
				   )
				 ORDER BY entry_date DESC, id DESC
				 LIMIT $3
			`, cursor.Sort, nullableCursorID(cursor.ID), limit+1)
			if err != nil {
				return fmt.Errorf("list journal draft ids: %w", err)
			}
			ids := make([]uuid.UUID, 0, limit+1)
			for rows.Next() {
				var id uuid.UUID
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return err
				}
				ids = append(ids, id)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			for _, id := range ids {
				draft, err := loadAccountingDraft(ctx, tx, id, false)
				if err != nil {
					return err
				}
				drafts = append(drafts, draft)
			}
			return nil
		},
	) {
		return
	}
	items := make([]api.JournalDraft, 0, min(len(drafts), limit))
	var next *string
	if len(drafts) > limit {
		last := drafts[limit-1]
		next = encodeKeysetCursor(last.Date.Format("2006-01-02"), last.ID.String())
		drafts = drafts[:limit]
	}
	for _, draft := range drafts {
		items = append(items, apiDraft(draft))
	}
	writeJSON(w, http.StatusOK, api.JournalDraftList{
		Items: items,
		Page:  api.PageInfo{NextCursor: next, Total: total},
	})
}

func (h *IAMAPI) createJournalDraft(
	w http.ResponseWriter,
	r *http.Request,
	_ api.CreateJournalDraftParams,
) {
	var input api.JournalDraftInput
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
			draft, err := draftFromAPI(
				ctx,
				tx,
				input,
				r.Header.Get("Idempotency-Key"),
				scope.ActorID,
			)
			if err != nil {
				return err
			}
			created, err := service.CreateDraft(ctx, scope, draft)
			if err != nil {
				return err
			}
			response = apiDraft(created)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) updateJournalDraft(
	w http.ResponseWriter,
	r *http.Request,
	draftID api.DraftID,
	_ api.UpdateJournalDraftParams,
) {
	var input api.UpdateJournalDraftInput
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
			var originalKey string
			if err := tx.QueryRow(ctx, `
				SELECT idempotency_key
				  FROM accounting.drafts
				 WHERE id = $1
				   AND status = 'active'
			`, draftID).Scan(&originalKey); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return errBusinessNotFound
				}
				return err
			}
			base := api.JournalDraftInput{
				AccountingDate: input.AccountingDate,
				Currency:       input.Currency,
				Description:    input.Description,
				Lines:          input.Lines,
			}
			draft, err := draftFromAPI(ctx, tx, base, originalKey, scope.ActorID)
			if err != nil {
				return err
			}
			draft.ID = draftID
			draft.Version = input.Version
			updated, err := service.UpdateDraft(ctx, scope, draft, input.Version)
			if err != nil {
				return err
			}
			response = apiDraft(updated)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) postJournalDraft(
	w http.ResponseWriter,
	r *http.Request,
	draftID api.DraftID,
	_ api.PostJournalDraftParams,
) {
	var input api.VersionedCommandInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	var response api.JournalEntry
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
			entry, err := service.PostDraft(
				ctx,
				scope,
				draftID,
				input.Version,
				r.Header.Get("Idempotency-Key"),
			)
			if err != nil {
				return err
			}
			response = apiEntry(entry)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) listJournalEntries(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListJournalEntriesParams,
) {
	cursor, err := decodeKeysetCursor((*string)(params.Cursor))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	filter := accounting.JournalFilter{
		Limit: accountingAPILimit(params.Limit) + 1,
		Query: strings.TrimSpace(stringValue(params.Query)),
	}
	if cursor.Sort != "" {
		filter.AfterNumber, err = parseInt64(cursor.Sort)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid journal cursor")
			return
		}
	}
	if params.From != nil {
		value := params.From.Time
		filter.From = &value
	}
	if params.To != nil {
		value := params.To.Time
		filter.To = &value
	}
	limit := filter.Limit - 1
	var (
		result accounting.PageResult[accounting.JournalEntry]
		total  int
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
			var err error
			result, err = service.Journal(ctx, scope, filter)
			if err != nil {
				return err
			}
			return tx.QueryRow(ctx, `
				SELECT count(*)
				  FROM accounting.journal_entries
				 WHERE ($1::date IS NULL OR entry_date >= $1)
				   AND ($2::date IS NULL OR entry_date <= $2)
				   AND (
				       $3 = ''
				       OR description ILIKE '%' || $3 || '%'
				       OR entry_number::text = $3
				   )
			`, nullableDate(filter.From), nullableDate(filter.To), stringValue(params.Query)).Scan(&total)
		},
	) {
		return
	}
	entries := result.Items
	var next *string
	if len(entries) > limit {
		last := entries[limit-1]
		next = encodeKeysetCursor(fmt.Sprint(last.Number), last.ID.String())
		entries = entries[:limit]
	}
	items := make([]api.JournalEntry, 0, len(entries))
	for _, entry := range entries {
		items = append(items, apiEntry(entry))
	}
	writeJSON(w, http.StatusOK, api.JournalEntryList{
		Items: items,
		Page:  api.PageInfo{NextCursor: next, Total: total},
	})
}

func (h *IAMAPI) getJournalEntry(w http.ResponseWriter, r *http.Request, entryID api.EntryID) {
	var response api.JournalEntry
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
			entry, err := loadAccountingEntry(ctx, tx, entryID)
			if err != nil {
				return err
			}
			response = apiEntry(entry)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) reverseJournalEntry(
	w http.ResponseWriter,
	r *http.Request,
	entryID api.EntryID,
	_ api.ReverseJournalEntryParams,
) {
	var input api.ReverseJournalEntryInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Reason) == "" {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "A reversal reason is required")
		return
	}
	var response api.JournalEntry
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
			entry, err := service.ReverseEntry(ctx, scope, accounting.ReverseEntryCommand{
				EntryID:        entryID,
				Date:           input.AccountingDate.Time,
				Reason:         strings.TrimSpace(input.Reason),
				IdempotencyKey: r.Header.Get("Idempotency-Key"),
			})
			if err != nil {
				return err
			}
			response = apiEntry(entry)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) listAccountingPeriods(w http.ResponseWriter, r *http.Request) {
	periods := make([]api.AccountingPeriod, 0)
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
				SELECT id, start_date, end_date, status, version
				  FROM accounting.periods
				 ORDER BY start_date DESC, id DESC
			`)
			if err != nil {
				return fmt.Errorf("list accounting periods: %w", err)
			}
			for rows.Next() {
				var period api.AccountingPeriod
				if err := rows.Scan(
					&period.Id,
					&period.StartDate,
					&period.EndDate,
					&period.State,
					&period.Version,
				); err != nil {
					rows.Close()
					return fmt.Errorf("scan accounting period: %w", err)
				}
				periods = append(periods, period)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			for index := range periods {
				checklist, err := loadPeriodChecklist(ctx, tx, periods[index].Id)
				if err != nil {
					return err
				}
				periods[index].Checklist = &checklist
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, periods)
}

func (h *IAMAPI) transitionAccountingPeriod(
	w http.ResponseWriter,
	r *http.Request,
	periodID api.PeriodID,
	_ api.TransitionAccountingPeriodParams,
) {
	var input api.PeriodTransitionInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	if !input.DesiredState.Valid() {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid period state")
		return
	}
	var response api.AccountingPeriod
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
			period, checklist, err := service.TransitionPeriod(
				ctx,
				scope,
				periodID,
				input.Version,
				accounting.PeriodStatus(input.DesiredState),
				strings.TrimSpace(input.Reason),
			)
			if err != nil {
				return err
			}
			response = apiPeriod(period, checklist)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) listAccountingReconciliations(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListAccountingReconciliationsParams,
) {
	cursor, err := decodeKeysetCursor((*string)(params.Cursor))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	limit := accountingAPILimit(params.Limit)
	var (
		items []api.Reconciliation
		total int
	)
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
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM accounting.reconciliations`).Scan(&total); err != nil {
				return fmt.Errorf("count reconciliations: %w", err)
			}
			rows, err := tx.Query(ctx, `
				SELECT
					reconciliation.id,
					reconciliation.financial_account_id,
					financial_account.currency_code,
					reconciliation.start_date,
					reconciliation.end_date,
					reconciliation.opening_balance::text,
					reconciliation.closing_balance,
					coalesce(sum(movement.signed_amount), 0)::text,
					(reconciliation.closing_balance - coalesce(sum(movement.signed_amount), 0))::text,
					reconciliation.status,
					reconciliation.version
				  FROM accounting.reconciliations AS reconciliation
				  JOIN accounting.financial_accounts AS financial_account
				    ON financial_account.org_id = reconciliation.org_id
				   AND financial_account.id = reconciliation.financial_account_id
				  LEFT JOIN accounting.financial_account_movements_view AS movement
				    ON movement.org_id = reconciliation.org_id
				   AND movement.financial_account_id = reconciliation.financial_account_id
				   AND movement.entry_date <= reconciliation.end_date
				 WHERE (
				       $1 = ''
				       OR (reconciliation.end_date, reconciliation.id) < ($1::date, $2::uuid)
				   )
				 GROUP BY
					reconciliation.org_id,
					reconciliation.id,
					financial_account.currency_code
				 ORDER BY reconciliation.end_date DESC, reconciliation.id DESC
				 LIMIT $3
			`, cursor.Sort, nullableCursorID(cursor.ID), limit+1)
			if err != nil {
				return fmt.Errorf("list reconciliations: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var item api.Reconciliation
				var status string
				if err := rows.Scan(
					&item.Id,
					&item.AccountId,
					&item.Currency,
					&item.PeriodStart,
					&item.StatementDate,
					&item.OpeningBalance,
					&item.StatementBalance,
					&item.LedgerBalance,
					&item.Difference,
					&status,
					&item.Version,
				); err != nil {
					return fmt.Errorf("scan reconciliation: %w", err)
				}
				if status == "closed" {
					item.State = api.ReconciliationStateCompleted
				} else {
					item.State = api.ReconciliationStateDraft
				}
				item.Matches = make([]api.ReconciliationMatch, 0)
				items = append(items, item)
			}
			return rows.Err()
		},
	) {
		return
	}
	var next *string
	if len(items) > limit {
		last := items[limit-1]
		next = encodeKeysetCursor(last.StatementDate.Format("2006-01-02"), last.Id.String())
		items = items[:limit]
	}
	writeJSON(w, http.StatusOK, api.ReconciliationList{
		Items: items,
		Page:  api.PageInfo{NextCursor: next, Total: total},
	})
}

func (h *IAMAPI) createAccountingReconciliation(
	w http.ResponseWriter,
	r *http.Request,
	_ api.CreateAccountingReconciliationParams,
) {
	var input api.ReconciliationInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	openingBalance, err := accounting.ParseAmount(input.OpeningBalance)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid opening balance")
		return
	}
	statementBalance, err := accounting.ParseAmount(input.StatementBalance)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid statement balance")
		return
	}
	currency, err := accounting.NewCurrency(input.Currency)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	if input.StatementDate.Time.Before(input.PeriodStart.Time) {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Statement date cannot precede reconciliation start")
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
			var accountCurrency string
			if err := tx.QueryRow(ctx, `
				SELECT financial_account.currency_code
				  FROM accounting.financial_accounts AS financial_account
				 WHERE financial_account.id = $1
				   AND financial_account.archived_at IS NULL
			`, input.AccountId).Scan(&accountCurrency); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return errBusinessNotFound
				}
				return fmt.Errorf("load financial account for reconciliation: %w", err)
			}
			if accountCurrency != currency.Code() {
				return fmt.Errorf("%w: reconciliation currency differs from account", accounting.ErrInvalidArgument)
			}
			created, err := service.CreateReconciliation(
				ctx,
				scope,
				accounting.CreateReconciliationCommand{
					FinancialAccountID: input.AccountId,
					PeriodStart:        input.PeriodStart.Time,
					PeriodEnd:          input.StatementDate.Time,
					StatementOpening:   openingBalance,
					StatementClosing:   statementBalance,
					Matches:            matches,
				},
			)
			if err != nil {
				return err
			}
			response = apiReconciliation(created, currency)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) getAccountingReport(
	w http.ResponseWriter,
	r *http.Request,
	report api.GetAccountingReportParamsReport,
	params api.GetAccountingReportParams,
) {
	if params.To.Time.Before(params.From.Time) {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Report end date cannot precede start date")
		return
	}
	var response api.AccountingReport
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
			response = api.AccountingReport{
				Currency: "ARS",
				From:     params.From,
				Report:   string(report),
				Rows:     make([]api.AccountingReportRow, 0),
				To:       params.To,
			}
			if err := tx.QueryRow(ctx, `
				SELECT coalesce(functional_currency, 'ARS')
				  FROM accounting.organization_settings
				 LIMIT 1
			`).Scan(&response.Currency); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("load report currency: %w", err)
			}
			switch string(report) {
			case "journal":
				after := int64(0)
				var totalDebit, totalCredit accounting.Decimal
				for {
					page, err := service.Journal(ctx, scope, accounting.JournalFilter{
						From:        &params.From.Time,
						To:          &params.To.Time,
						AfterNumber: after,
						Limit:       200,
					})
					if err != nil {
						return err
					}
					for _, entry := range page.Items {
						for _, line := range entry.Lines {
							label := fmt.Sprintf(
								"%s · #%d · %s · %s",
								entry.Date.Format("02/01/2006"),
								entry.Number,
								line.AccountCode,
								line.AccountName,
							)
							if strings.TrimSpace(line.Memo) != "" {
								label += " · " + strings.TrimSpace(line.Memo)
							}
							response.Rows = append(response.Rows, api.AccountingReportRow{
								Balance: "0",
								Credit:  line.Credit.String(),
								Debit:   line.Debit.String(),
								Key:     journalLineReportKey(entry.ID, line.ID),
								Label:   label,
							})
							totalDebit = totalDebit.Add(line.Debit)
							totalCredit = totalCredit.Add(line.Credit)
						}
					}
					if page.NextCursor == "" {
						break
					}
					after, err = strconv.ParseInt(page.NextCursor, 10, 64)
					if err != nil {
						return fmt.Errorf("parse journal report cursor: %w", err)
					}
				}
				response.TotalDebit = totalDebit.String()
				response.TotalCredit = totalCredit.String()
			case "trial-balance":
				trial, err := service.TrialBalance(ctx, scope, params.From.Time, params.To.Time)
				if err != nil {
					return err
				}
				for _, row := range trial.Rows {
					response.Rows = append(response.Rows, api.AccountingReportRow{
						Balance: row.NetBalance.String(),
						Credit:  row.Credit.String(),
						Debit:   row.Debit.String(),
						Key:     row.AccountID.String(),
						Label:   row.Code + " · " + row.Name,
					})
				}
				response.TotalDebit = trial.TotalDebit.String()
				response.TotalCredit = trial.TotalCredit.String()
			case "general-ledger":
				if params.AccountId == nil {
					return fmt.Errorf("%w: account_id is required for general ledger", accounting.ErrInvalidArgument)
				}
				ledger, err := service.GeneralLedger(
					ctx,
					scope,
					*params.AccountId,
					params.From.Time,
					params.To.Time,
				)
				if err != nil {
					return err
				}
				var debit, credit accounting.Decimal
				for _, line := range ledger.Lines {
					response.Rows = append(response.Rows, api.AccountingReportRow{
						Balance: line.Balance.String(),
						Credit:  line.Credit.String(),
						Debit:   line.Debit.String(),
						Key:     journalLineReportKey(line.EntryID, line.LineID),
						Label: fmt.Sprintf(
							"%s · #%d · %s",
							line.Date.Format("02/01/2006"),
							line.EntryNumber,
							line.Description,
						),
					})
					debit = debit.Add(line.Debit)
					credit = credit.Add(line.Credit)
				}
				response.TotalDebit, response.TotalCredit = debit.String(), credit.String()
			case "financial-activity":
				if params.FinancialAccountId == nil {
					return fmt.Errorf(
						"%w: financial_account_id is required for financial activity",
						accounting.ErrInvalidArgument,
					)
				}
				ledgerAccountID, financialName, err := loadFinancialLedgerAccount(
					ctx,
					tx,
					scope.OrganizationID,
					*params.FinancialAccountId,
				)
				if err != nil {
					return err
				}
				ledger, err := service.GeneralLedger(
					ctx,
					scope,
					ledgerAccountID,
					params.From.Time,
					params.To.Time,
				)
				if err != nil {
					return err
				}
				var debit, credit accounting.Decimal
				for _, line := range ledger.Lines {
					response.Rows = append(response.Rows, api.AccountingReportRow{
						Balance: line.Balance.String(),
						Credit:  line.Credit.String(),
						Debit:   line.Debit.String(),
						Key:     journalLineReportKey(line.EntryID, line.LineID),
						Label: fmt.Sprintf(
							"%s · %s · #%d · %s",
							financialName,
							line.Date.Format("02/01/2006"),
							line.EntryNumber,
							line.Description,
						),
					})
					debit = debit.Add(line.Debit)
					credit = credit.Add(line.Credit)
				}
				if len(ledger.Lines) == 0 {
					response.Rows = append(response.Rows, api.AccountingReportRow{
						Balance: ledger.ClosingBalance.String(),
						Credit:  "0",
						Debit:   "0",
						Key:     params.FinancialAccountId.String() + ":balance",
						Label:   financialName + " · Saldo al cierre",
					})
				}
				response.TotalDebit, response.TotalCredit = debit.String(), credit.String()
			case "balance-sheet":
				statement, err := service.BalanceSheet(ctx, scope, params.From.Time, params.To.Time)
				if err != nil {
					return err
				}
				appendStatementRows(&response, statement.Assets, "Activo")
				appendStatementRows(&response, statement.Liabilities, "Pasivo")
				appendStatementRows(&response, statement.Equity, "Patrimonio")
				response.TotalDebit = statement.TotalAssets.String()
				response.TotalCredit = statement.LiabilitiesAndEquity.String()
			case "income-statement":
				statement, err := service.IncomeStatement(ctx, scope, params.From.Time, params.To.Time)
				if err != nil {
					return err
				}
				appendStatementRows(&response, statement.Revenue, "Ingresos")
				appendStatementRows(&response, statement.Costs, "Costos")
				appendStatementRows(&response, statement.Expenses, "Gastos")
				response.TotalDebit = statement.TotalCosts.Add(statement.TotalExpenses).String()
				response.TotalCredit = statement.TotalRevenue.String()
			case "aging":
				return loadAgingReport(ctx, tx, params.To.Time, &response)
			case "vat-position":
				return loadVATPositionReport(ctx, tx, params.From.Time, params.To.Time, &response)
			default:
				return fmt.Errorf("%w: unsupported accounting report", accounting.ErrInvalidArgument)
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func loadFinancialLedgerAccount(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	financialAccountID uuid.UUID,
) (uuid.UUID, string, error) {
	var (
		ledgerAccountID uuid.UUID
		name            string
	)
	err := tx.QueryRow(ctx, `
		SELECT ledger_account_id, name
		  FROM accounting.financial_accounts
		 WHERE org_id = $1
		   AND id = $2
	`, organizationID, financialAccountID).Scan(&ledgerAccountID, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", accounting.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("load financial account ledger: %w", err)
	}
	return ledgerAccountID, name, nil
}

type accountingWork func(
	context.Context,
	*accounting.Service,
	accounting.Scope,
	pgx.Tx,
) error

func (h *IAMAPI) withAccountingService(
	w http.ResponseWriter,
	r *http.Request,
	permission productiam.Permission,
	work accountingWork,
) bool {
	return h.withinBusinessTx(
		w,
		r,
		permission,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			organizationID, err := uuid.Parse(active.OrganizationID)
			if err != nil {
				return fmt.Errorf("parse active accounting organization: %w", err)
			}
			transactor, err := accountingpg.NewTxTransactor(tx)
			if err != nil {
				return err
			}
			service, err := accounting.NewService(transactor)
			if err != nil {
				return err
			}
			scope := accounting.Scope{
				OrganizationID:      organizationID,
				ActorID:             active.UserID,
				CanPostAdjustments:  permission == productiam.PermissionAccountingManage,
				CanReopenPeriods:    permission == productiam.PermissionAccountingManage,
				CanManageAccounting: permission == productiam.PermissionAccountingManage,
			}
			return mapAccountingError(work(ctx, service, scope, tx))
		},
	)
}

func mapAccountingError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, accounting.ErrNotFound):
		return fmt.Errorf("%w: %v", errBusinessNotFound, err)
	case errors.Is(err, accounting.ErrVersionConflict):
		return fmt.Errorf("%w: %v", errBusinessVersionConflict, err)
	case errors.Is(err, accounting.ErrDuplicate):
		return fmt.Errorf("%w: %v", errBusinessDuplicate, err)
	case errors.Is(err, accounting.ErrIdempotencyConflict):
		return fmt.Errorf("%w: %v", errBusinessIdempotency, err)
	case errors.Is(err, accounting.ErrUnbalancedEntry):
		return fmt.Errorf("%w: %v", errBusinessUnbalanced, err)
	case errors.Is(err, accounting.ErrPeriodClosed):
		return fmt.Errorf("%w: %v", errBusinessPeriodClosed, err)
	case errors.Is(err, accounting.ErrEntryImmutable):
		return fmt.Errorf("%w: %v", errBusinessImmutable, err)
	case errors.Is(err, accounting.ErrAlreadyReversed),
		errors.Is(err, accounting.ErrAccountArchived),
		errors.Is(err, accounting.ErrAccountNotPostable),
		errors.Is(err, accounting.ErrAccountInUse),
		errors.Is(err, accounting.ErrReconciliationClosed),
		errors.Is(err, accounting.ErrConflict):
		return fmt.Errorf("%w: %v", errBusinessInvalidTransition, err)
	case errors.Is(err, accounting.ErrInvalidArgument),
		errors.Is(err, accounting.ErrInvalidDecimal),
		errors.Is(err, accounting.ErrMappingMissing),
		errors.Is(err, accounting.ErrInflationIncomplete):
		return fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	default:
		return err
	}
}

func accountingAPILimit(raw *api.Limit) int {
	if raw == nil || *raw <= 0 {
		return 50
	}
	if *raw > 200 {
		return 200
	}
	return int(*raw)
}

func nullableCursorID(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return raw
}

func scanAccountingAccount(
	row interface{ Scan(...any) error },
	lifecycle api.LifecycleState,
) (api.AccountingAccount, error) {
	var (
		item         api.AccountingAccount
		accountClass string
		monetary     string
		normal       string
	)
	if err := row.Scan(
		&item.Id,
		&item.Code,
		&item.Name,
		&accountClass,
		&normal,
		&monetary,
		&item.ParentId,
		&item.Postable,
		&item.Version,
	); err != nil {
		return api.AccountingAccount{}, fmt.Errorf("scan accounting account: %w", err)
	}
	item.AccountType = apiAccountTypeFromDB(accountClass)
	item.NormalBalance = api.AccountingNormalBalance(normal)
	item.MonetaryClassification = api.MonetaryClassification(monetary)
	item.LifecycleState = lifecycle
	return item, nil
}

func apiAccountTypeFromDB(value string) api.AccountingAccountType {
	if value == string(accounting.AccountRevenue) {
		return api.Income
	}
	return api.AccountingAccountType(value)
}

func domainAccountClass(value api.AccountingAccountType) (accounting.AccountClass, error) {
	if !value.Valid() {
		return "", fmt.Errorf("%w: invalid account type", accounting.ErrInvalidArgument)
	}
	if value == api.Income {
		return accounting.AccountRevenue, nil
	}
	return accounting.AccountClass(value), nil
}

func accountCommand(input api.AccountingAccountInput) (accounting.CreateAccountCommand, error) {
	class, err := domainAccountClass(input.AccountType)
	if err != nil {
		return accounting.CreateAccountCommand{}, err
	}
	if !input.NormalBalance.Valid() || !input.MonetaryClassification.Valid() {
		return accounting.CreateAccountCommand{}, fmt.Errorf(
			"%w: invalid account classification",
			accounting.ErrInvalidArgument,
		)
	}
	command := accounting.CreateAccountCommand{
		Code:          strings.TrimSpace(input.Code),
		Name:          strings.TrimSpace(input.Name),
		Class:         class,
		NormalBalance: accounting.NormalBalance(input.NormalBalance),
		Monetary:      accounting.MonetaryClassification(input.MonetaryClassification),
		ParentID:      input.ParentId,
		Postable:      input.Postable,
	}
	if !command.Postable && command.Monetary != accounting.NotApplicable {
		return accounting.CreateAccountCommand{}, fmt.Errorf(
			"%w: non-postable accounts must be not_applicable",
			accounting.ErrInvalidArgument,
		)
	}
	if command.Postable && command.Monetary == accounting.NotApplicable {
		return accounting.CreateAccountCommand{}, fmt.Errorf(
			"%w: postable accounts need a monetary classification",
			accounting.ErrInvalidArgument,
		)
	}
	return command, nil
}

func apiAccount(account accounting.Account, lifecycle api.LifecycleState) api.AccountingAccount {
	return api.AccountingAccount{
		AccountType:            apiAccountTypeFromDB(string(account.Class)),
		Code:                   account.Code,
		Id:                     account.ID,
		LifecycleState:         lifecycle,
		MonetaryClassification: api.MonetaryClassification(account.Monetary),
		Name:                   account.Name,
		NormalBalance:          api.AccountingNormalBalance(account.NormalBalance),
		ParentId:               account.ParentID,
		Postable:               account.Postable,
		Version:                account.Version,
	}
}

func loadAccountingAccountAfterTransition(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
	lifecycle api.LifecycleState,
	response *api.AccountingAccount,
) error {
	item, err := scanAccountingAccount(tx.QueryRow(ctx, `
		SELECT
			id,
			code,
			name,
			account_class,
			normal_balance,
			monetary_class,
			parent_id,
			posting_allowed,
			version
		  FROM accounting.accounts
		 WHERE id = $1
	`, id), lifecycle)
	if err != nil {
		return err
	}
	*response = item
	return nil
}

func draftFromAPI(
	ctx context.Context,
	tx pgx.Tx,
	input api.JournalDraftInput,
	idempotencyKey string,
	actor string,
) (accounting.Draft, error) {
	if strings.TrimSpace(input.Description) == "" || len(input.Lines) < 2 {
		return accounting.Draft{}, fmt.Errorf(
			"%w: description and at least two lines are required",
			accounting.ErrInvalidArgument,
		)
	}
	functionalCurrency, err := loadFunctionalCurrency(ctx, tx)
	if err != nil {
		return accounting.Draft{}, err
	}
	transactionCurrency, err := accounting.NewCurrency(input.Currency)
	if err != nil {
		return accounting.Draft{}, err
	}
	draft := accounting.Draft{
		IdempotencyKey:     strings.TrimSpace(idempotencyKey),
		Date:               input.AccountingDate.Time,
		Kind:               accounting.EntryManual,
		FunctionalCurrency: functionalCurrency,
		Currency:           transactionCurrency,
		ExchangeRate:       accounting.One,
		ExchangeRateDate:   input.AccountingDate.Time,
		ExchangeRateSource: "manual",
		Description:        strings.TrimSpace(input.Description),
		CreatedBy:          actor,
		UpdatedBy:          actor,
	}
	for index, inputLine := range input.Lines {
		debit, err := accounting.ParseAmount(inputLine.Debit)
		if err != nil {
			return accounting.Draft{}, fmt.Errorf("line %d debit: %w", index+1, err)
		}
		credit, err := accounting.ParseAmount(inputLine.Credit)
		if err != nil {
			return accounting.Draft{}, fmt.Errorf("line %d credit: %w", index+1, err)
		}
		lineCurrency := functionalCurrency
		if inputLine.TransactionCurrency != nil {
			lineCurrency, err = accounting.NewCurrency(*inputLine.TransactionCurrency)
			if err != nil {
				return accounting.Draft{}, fmt.Errorf("line %d currency: %w", index+1, err)
			}
		}
		exchangeRate := accounting.One
		if inputLine.ExchangeRate != nil {
			exchangeRate, err = accounting.ParseExchangeRate(*inputLine.ExchangeRate)
			if err != nil {
				return accounting.Draft{}, fmt.Errorf("line %d exchange rate: %w", index+1, err)
			}
		} else if lineCurrency.Code() != functionalCurrency.Code() {
			return accounting.Draft{}, fmt.Errorf(
				"%w: line %d foreign currency requires an exchange rate",
				accounting.ErrInvalidArgument,
				index+1,
			)
		}
		transactionAmount := debit.Add(credit)
		if inputLine.TransactionAmount != nil {
			transactionAmount, err = accounting.ParseAmount(*inputLine.TransactionAmount)
			if err != nil {
				return accounting.Draft{}, fmt.Errorf("line %d transaction amount: %w", index+1, err)
			}
		}
		line := accounting.JournalLine{
			ID:                 uuid.New(),
			AccountID:          inputLine.AccountId,
			Debit:              debit,
			Credit:             credit,
			Currency:           lineCurrency,
			ExchangeRate:       exchangeRate,
			ExchangeRateDate:   input.AccountingDate.Time,
			ExchangeRateSource: "manual",
			PartyID:            inputLine.PartyId,
			LineNo:             index + 1,
		}
		if inputLine.Memo != nil {
			line.Memo = strings.TrimSpace(*inputLine.Memo)
		}
		if !debit.IsZero() {
			line.TransactionDebit = transactionAmount
		} else {
			line.TransactionCredit = transactionAmount
		}
		draft.Lines = append(draft.Lines, line)
	}
	return draft, nil
}

func loadFunctionalCurrency(ctx context.Context, tx pgx.Tx) (accounting.Currency, error) {
	var code string
	err := tx.QueryRow(ctx, `
		SELECT functional_currency
		  FROM accounting.organization_settings
		 LIMIT 1
	`).Scan(&code)
	if errors.Is(err, pgx.ErrNoRows) {
		code = "ARS"
		err = nil
	}
	if err != nil {
		return accounting.Currency{}, fmt.Errorf("load functional currency: %w", err)
	}
	return accounting.NewCurrency(code)
}

func apiDraft(draft accounting.Draft) api.JournalDraft {
	debit, credit := draftTotals(draft)
	lines := make([]api.JournalLine, 0, len(draft.Lines))
	for _, line := range draft.Lines {
		lines = append(lines, apiJournalLine(line))
	}
	return api.JournalDraft{
		AccountingDate: openapi_types.Date{Time: draft.Date},
		Currency:       draft.Currency.Code(),
		Description:    draft.Description,
		Id:             draft.ID,
		Lines:          lines,
		TotalCredit:    credit.String(),
		TotalDebit:     debit.String(),
		Version:        draft.Version,
	}
}

func draftTotals(draft accounting.Draft) (accounting.Decimal, accounting.Decimal) {
	var debit, credit accounting.Decimal
	for _, line := range draft.Lines {
		debit = debit.Add(line.Debit)
		credit = credit.Add(line.Credit)
	}
	return debit, credit
}

func apiJournalLine(line accounting.JournalLine) api.JournalLine {
	transactionAmount := line.TransactionDebit.Add(line.TransactionCredit).String()
	transactionCurrency := line.Currency.Code()
	exchangeRate := line.ExchangeRate.String()
	var memo *string
	if strings.TrimSpace(line.Memo) != "" {
		value := line.Memo
		memo = &value
	}
	return api.JournalLine{
		AccountId:           line.AccountID,
		Credit:              line.Credit.String(),
		Debit:               line.Debit.String(),
		ExchangeRate:        &exchangeRate,
		Id:                  line.ID,
		LineNumber:          line.LineNo,
		Memo:                memo,
		PartyId:             line.PartyID,
		TransactionAmount:   &transactionAmount,
		TransactionCurrency: &transactionCurrency,
	}
}

func apiEntry(entry accounting.JournalEntry) api.JournalEntry {
	debit, credit := entry.Totals()
	lines := make([]api.JournalLine, 0, len(entry.Lines))
	for _, line := range entry.Lines {
		lines = append(lines, apiJournalLine(line))
	}
	response := api.JournalEntry{
		AccountingDate:  openapi_types.Date{Time: entry.Date},
		CreatedAt:       entry.CreatedAt,
		Currency:        entry.FunctionalCurrency.Code(),
		Description:     entry.Description,
		EntryNumber:     entry.Number,
		Id:              entry.ID,
		Lines:           lines,
		ReversesEntryId: entry.ReversesEntryID,
		TotalCredit:     credit.String(),
		TotalDebit:      debit.String(),
	}
	if entry.ReversalReason != "" {
		reason := entry.ReversalReason
		response.ReversalReason = &reason
	}
	if strings.TrimSpace(entry.Source.Type) != "" {
		sourceType := entry.Source.Type
		response.SourceType = &sourceType
	}
	if entry.Source.ID != uuid.Nil {
		sourceID := entry.Source.ID
		response.SourceId = &sourceID
	}
	return response
}

func loadAccountingDraft(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
	lock bool,
) (accounting.Draft, error) {
	query := `
		SELECT
			id,
			idempotency_key,
			entry_date,
			entry_kind,
			description,
			coalesce(source_type, ''),
			coalesce(source_id, ''),
			version,
			created_by,
			created_at,
			updated_at
		  FROM accounting.drafts
		 WHERE id = $1
	`
	if lock {
		query += " FOR UPDATE"
	}
	var draft accounting.Draft
	if err := tx.QueryRow(ctx, query, id).Scan(
		&draft.ID,
		&draft.IdempotencyKey,
		&draft.Date,
		&draft.Kind,
		&draft.Description,
		&draft.SourceType,
		&draft.SourceID,
		&draft.Version,
		&draft.CreatedBy,
		&draft.CreatedAt,
		&draft.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return accounting.Draft{}, errBusinessNotFound
		}
		return accounting.Draft{}, fmt.Errorf("load accounting draft: %w", err)
	}
	draft.UpdatedBy = draft.CreatedBy
	functionalCurrency, err := loadFunctionalCurrency(ctx, tx)
	if err != nil {
		return accounting.Draft{}, err
	}
	draft.FunctionalCurrency = functionalCurrency
	draft.Currency = functionalCurrency
	draft.ExchangeRate = accounting.One
	draft.ExchangeRateDate = draft.Date
	draft.ExchangeRateSource = "manual"
	rows, err := tx.Query(ctx, `
		SELECT
			id,
			line_no,
			account_id,
			description,
			debit_amount::text,
			credit_amount::text,
			currency_code,
			currency_amount::text,
			exchange_rate::text,
			coalesce(exchange_rate_date, $2::date),
			coalesce(exchange_rate_source, 'manual'),
			party_id
		  FROM accounting.draft_lines
		 WHERE draft_id = $1
		 ORDER BY line_no, id
	`, id, draft.Date)
	if err != nil {
		return accounting.Draft{}, fmt.Errorf("load accounting draft lines: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		line, err := scanDomainJournalLine(rows)
		if err != nil {
			return accounting.Draft{}, err
		}
		draft.Lines = append(draft.Lines, line)
		if len(draft.Lines) == 1 {
			draft.Currency = line.Currency
			draft.ExchangeRate = line.ExchangeRate
			draft.ExchangeRateDate = line.ExchangeRateDate
			draft.ExchangeRateSource = line.ExchangeRateSource
		}
	}
	return draft, rows.Err()
}

func loadAccountingEntry(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (accounting.JournalEntry, error) {
	var (
		entry                  accounting.JournalEntry
		sourceID               string
		sourceType             string
		functionalCurrencyCode string
	)
	err := tx.QueryRow(ctx, `
		SELECT
			id,
			entry_number,
			entry_date,
			entry_kind,
			posting_kind,
			functional_currency,
			coalesce(source_type, ''),
			coalesce(source_id, ''),
			source_event,
			idempotency_key,
			description,
			created_by,
			posted_at,
			reverses_entry_id,
			coalesce(reversal_reason, ''),
			draft_id
		  FROM accounting.journal_entries
		 WHERE id = $1
	`, id).Scan(
		&entry.ID,
		&entry.Number,
		&entry.Date,
		&entry.Kind,
		&entry.PostingKind,
		&functionalCurrencyCode,
		&sourceType,
		&sourceID,
		&entry.Source.Event,
		&entry.Source.IdempotencyKey,
		&entry.Description,
		&entry.CreatedBy,
		&entry.CreatedAt,
		&entry.ReversesEntryID,
		&entry.ReversalReason,
		&entry.DraftID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return accounting.JournalEntry{}, errBusinessNotFound
	}
	if err != nil {
		return accounting.JournalEntry{}, fmt.Errorf("load accounting entry: %w", err)
	}
	functionalCurrency, err := accounting.NewCurrency(functionalCurrencyCode)
	if err != nil {
		return accounting.JournalEntry{}, err
	}
	entry.FunctionalCurrency = functionalCurrency
	entry.Currency = functionalCurrency
	entry.ExchangeRate = accounting.One
	entry.ExchangeRateDate = entry.Date
	entry.ExchangeRateSource = "functional"
	entry.Source.Type = sourceType
	if parsed, parseErr := uuid.Parse(sourceID); parseErr == nil {
		entry.Source.ID = parsed
	}
	rows, err := tx.Query(ctx, `
		SELECT
			id,
			line_no,
			account_id,
			description,
			debit_amount::text,
			credit_amount::text,
			currency_code,
			currency_amount::text,
			exchange_rate::text,
			coalesce(exchange_rate_date, $2::date),
			coalesce(exchange_rate_source, 'functional'),
			party_id
		  FROM accounting.journal_lines
		 WHERE journal_entry_id = $1
		 ORDER BY line_no, id
	`, id, entry.Date)
	if err != nil {
		return accounting.JournalEntry{}, fmt.Errorf("load accounting entry lines: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		line, err := scanDomainJournalLine(rows)
		if err != nil {
			return accounting.JournalEntry{}, err
		}
		entry.Lines = append(entry.Lines, line)
	}
	return entry, rows.Err()
}

func scanDomainJournalLine(row interface{ Scan(...any) error }) (accounting.JournalLine, error) {
	var (
		line              accounting.JournalLine
		debitRaw          string
		creditRaw         string
		currencyCode      string
		transactionAmount string
		exchangeRateRaw   string
		partyIDRaw        *string
	)
	if err := row.Scan(
		&line.ID,
		&line.LineNo,
		&line.AccountID,
		&line.Memo,
		&debitRaw,
		&creditRaw,
		&currencyCode,
		&transactionAmount,
		&exchangeRateRaw,
		&line.ExchangeRateDate,
		&line.ExchangeRateSource,
		&partyIDRaw,
	); err != nil {
		return accounting.JournalLine{}, fmt.Errorf("scan journal line: %w", err)
	}
	var err error
	line.Debit, err = accounting.ParseDecimal(debitRaw)
	if err != nil {
		return accounting.JournalLine{}, err
	}
	line.Credit, err = accounting.ParseDecimal(creditRaw)
	if err != nil {
		return accounting.JournalLine{}, err
	}
	line.Currency, err = accounting.NewCurrency(currencyCode)
	if err != nil {
		return accounting.JournalLine{}, err
	}
	transaction, err := accounting.ParseDecimal(transactionAmount)
	if err != nil {
		return accounting.JournalLine{}, err
	}
	if !line.Debit.IsZero() {
		line.TransactionDebit = transaction
	} else {
		line.TransactionCredit = transaction
	}
	line.ExchangeRate, err = accounting.ParseDecimal(exchangeRateRaw)
	if err != nil {
		return accounting.JournalLine{}, err
	}
	if partyIDRaw != nil {
		if parsed, parseErr := uuid.Parse(*partyIDRaw); parseErr == nil {
			line.PartyID = &parsed
		}
	}
	return line, nil
}

func parseInt64(raw string) (int64, error) {
	var value int64
	_, err := fmt.Sscan(raw, &value)
	return value, err
}

func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func loadPeriodChecklist(
	ctx context.Context,
	tx pgx.Tx,
	periodID uuid.UUID,
) ([]struct {
	Clear bool   `json:"clear"`
	Code  string `json:"code"`
	Count *int   `json:"count,omitempty"`
}, error) {
	order := []string{
		"unposted_documents",
		"fiscal_pending",
		"posting_errors",
		"account_mappings",
		"exchange_rates",
		"unreconciled_accounts",
	}
	counts := make(map[string]int, len(order))
	rows, err := tx.Query(ctx, `
		SELECT
			check_key,
			CASE
				WHEN status = 'passed' THEN 0
				ELSE coalesce((details ->> 'count')::integer, 1)
			END
		  FROM accounting.period_close_checks
		 WHERE period_id = $1
	`, periodID)
	if err != nil {
		return nil, fmt.Errorf("load period close checklist: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		counts[key] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]struct {
		Clear bool   `json:"clear"`
		Code  string `json:"code"`
		Count *int   `json:"count,omitempty"`
	}, 0, len(order))
	for _, code := range order {
		count := counts[code]
		countCopy := count
		result = append(result, struct {
			Clear bool   `json:"clear"`
			Code  string `json:"code"`
			Count *int   `json:"count,omitempty"`
		}{
			Clear: count == 0,
			Code:  code,
			Count: &countCopy,
		})
	}
	return result, nil
}

func apiPeriod(period accounting.Period, checklist accounting.CloseChecklist) api.AccountingPeriod {
	checks := []struct {
		Clear bool   `json:"clear"`
		Code  string `json:"code"`
		Count *int   `json:"count,omitempty"`
	}{
		check("unposted_documents", checklist.UnpostedDocuments),
		check("fiscal_pending", checklist.PendingFiscalDocuments),
		check("posting_errors", checklist.PostingErrors),
		check("account_mappings", checklist.MissingMappings),
		check("exchange_rates", checklist.MissingExchangeRates),
		check("unreconciled_accounts", checklist.UnclosedReconciliations),
	}
	return api.AccountingPeriod{
		Checklist: &checks,
		EndDate:   openapi_types.Date{Time: period.EndDate},
		Id:        period.ID,
		StartDate: openapi_types.Date{Time: period.StartDate},
		State:     api.AccountingPeriodState(period.Status),
		Version:   period.Version,
	}
}

func check(code string, count int) struct {
	Clear bool   `json:"clear"`
	Code  string `json:"code"`
	Count *int   `json:"count,omitempty"`
} {
	value := count
	return struct {
		Clear bool   `json:"clear"`
		Code  string `json:"code"`
		Count *int   `json:"count,omitempty"`
	}{Clear: count == 0, Code: code, Count: &value}
}

func loadInflationInputs(
	ctx context.Context,
	tx pgx.Tx,
	closingDate time.Time,
	closingIndex accounting.Decimal,
	source string,
) (
	[]accounting.InflationIndex,
	[]accounting.InflationPosition,
	uuid.UUID,
	accounting.Currency,
	error,
) {
	functionalCurrency, err := loadFunctionalCurrency(ctx, tx)
	if err != nil {
		return nil, nil, uuid.Nil, accounting.Currency{}, err
	}
	index, err := accounting.NewInflationIndex(
		closingDate,
		closingIndex,
		source,
		source+"|"+closingIndex.String()+"|"+closingDate.Format("2006-01"),
	)
	if err != nil {
		return nil, nil, uuid.Nil, accounting.Currency{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.inflation_indices (
			org_id,
			series_code,
			period_month,
			index_value,
			source_url,
			source_checksum,
			imported_by
		)
		VALUES (
			current_setting('app.org_id')::uuid,
			'FACPCE_RT6_RT54',
			date_trunc('month', $1::date)::date,
			$2,
			$3,
			$4,
			current_setting('app.user_id', true)
		)
		ON CONFLICT (
			org_id,
			series_code,
			period_month,
			source_checksum
		) DO NOTHING
	`, closingDate, closingIndex.String(), source, index.Checksum); err != nil {
		return nil, nil, uuid.Nil, accounting.Currency{}, fmt.Errorf("store closing inflation index: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT period_month, index_value::text, source_url, source_checksum
		  FROM accounting.inflation_indices
		 WHERE series_code = 'FACPCE_RT6_RT54'
		   AND period_month <= date_trunc('month', $1::date)::date
		 ORDER BY period_month
	`, closingDate)
	if err != nil {
		return nil, nil, uuid.Nil, accounting.Currency{}, fmt.Errorf("load inflation indices: %w", err)
	}
	indices := make([]accounting.InflationIndex, 0)
	for rows.Next() {
		var item accounting.InflationIndex
		var value string
		if err := rows.Scan(&item.Period, &value, &item.Source, &item.Checksum); err != nil {
			rows.Close()
			return nil, nil, uuid.Nil, accounting.Currency{}, err
		}
		item.Value, err = accounting.ParseDecimal(value)
		if err != nil {
			rows.Close()
			return nil, nil, uuid.Nil, accounting.Currency{}, err
		}
		indices = append(indices, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, uuid.Nil, accounting.Currency{}, err
	}

	rows, err = tx.Query(ctx, `
		SELECT
			account.id,
			account.code,
			account.name,
			account.normal_balance,
			account.monetary_class,
			coalesce(line.origin_date, entry.entry_date),
			CASE
				WHEN account.normal_balance = 'credit'
					THEN sum(line.credit_amount - line.debit_amount)
				ELSE sum(line.debit_amount - line.credit_amount)
			END::text
		  FROM accounting.accounts AS account
		  JOIN accounting.journal_lines AS line
		    ON line.org_id = account.org_id
		   AND line.account_id = account.id
		  JOIN accounting.journal_entries AS entry
		    ON entry.org_id = line.org_id
		   AND entry.id = line.journal_entry_id
		 WHERE account.monetary_class = 'non_monetary'
		   AND entry.entry_date <= $1
		 GROUP BY
			account.id,
			account.code,
			account.name,
			account.normal_balance,
			account.monetary_class,
			coalesce(line.origin_date, entry.entry_date)
		 ORDER BY account.code, coalesce(line.origin_date, entry.entry_date)
	`, closingDate)
	if err != nil {
		return nil, nil, uuid.Nil, accounting.Currency{}, fmt.Errorf("load inflation positions: %w", err)
	}
	positions := make([]accounting.InflationPosition, 0)
	for rows.Next() {
		var position accounting.InflationPosition
		var balance string
		if err := rows.Scan(
			&position.AccountID,
			&position.AccountCode,
			&position.AccountName,
			&position.NormalBalance,
			&position.Classification,
			&position.OriginDate,
			&balance,
		); err != nil {
			rows.Close()
			return nil, nil, uuid.Nil, accounting.Currency{}, err
		}
		position.Balance, err = accounting.ParseDecimal(balance)
		if err != nil {
			rows.Close()
			return nil, nil, uuid.Nil, accounting.Currency{}, err
		}
		positions = append(positions, position)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, uuid.Nil, accounting.Currency{}, err
	}
	sort.Slice(indices, func(i, j int) bool {
		return indices[i].Period.Before(indices[j].Period)
	})
	var recpamAccountID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT mapping.account_id
		  FROM accounting.account_mappings AS mapping
		  JOIN accounting.accounts AS account
		    ON account.org_id = mapping.org_id
		   AND account.id = mapping.account_id
		 WHERE mapping.mapping_key = 'recpam'
		   AND account.archived_at IS NULL
		   AND account.trashed_at IS NULL
	`).Scan(&recpamAccountID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, uuid.Nil, accounting.Currency{}, accounting.ErrMappingMissing
		}
		return nil, nil, uuid.Nil, accounting.Currency{}, err
	}
	return indices, positions, recpamAccountID, functionalCurrency, nil
}

func appendStatementRows(
	report *api.AccountingReport,
	rows []accounting.StatementRow,
	group string,
) {
	for _, row := range rows {
		debit, credit := accounting.Decimal{}, accounting.Decimal{}
		if row.Class == accounting.AccountAsset ||
			row.Class == accounting.AccountCost ||
			row.Class == accounting.AccountExpense {
			debit = row.Amount
		} else {
			credit = row.Amount
		}
		report.Rows = append(report.Rows, api.AccountingReportRow{
			Balance: row.Amount.String(),
			Credit:  credit.String(),
			Debit:   debit.String(),
			Key:     row.AccountID.String(),
			Label:   group + " · " + row.Code + " · " + row.Name,
		})
	}
}

func journalLineReportKey(entryID, lineID uuid.UUID) string {
	return entryID.String() + ":" + lineID.String()
}

func loadAgingReport(
	ctx context.Context,
	tx pgx.Tx,
	asOf time.Time,
	report *api.AccountingReport,
) error {
	rows, err := tx.Query(ctx, `
		SELECT
			item_type,
			party_id,
			CASE
				WHEN due_date IS NULL OR due_date >= $1 THEN 'current'
				WHEN $1::date - due_date <= 30 THEN '1-30'
				WHEN $1::date - due_date <= 60 THEN '31-60'
				WHEN $1::date - due_date <= 90 THEN '61-90'
				ELSE '90+'
			END AS aging_bucket,
			sum(remaining_functional_amount)::text
		  FROM accounting.open_item_balances_as_of($1::date)
		 WHERE remaining_functional_amount <> 0
		 GROUP BY item_type, party_id, aging_bucket
		 ORDER BY
			item_type,
			party_id,
			CASE aging_bucket
				WHEN 'current' THEN 0
				WHEN '1-30' THEN 1
				WHEN '31-60' THEN 2
				WHEN '61-90' THEN 3
				ELSE 4
			END
	`, asOf)
	if err != nil {
		return fmt.Errorf("load aging report: %w", err)
	}
	defer rows.Close()
	var totalDebit, totalCredit accounting.Decimal
	for rows.Next() {
		var kind, partyID, bucket, balanceRaw string
		if err := rows.Scan(&kind, &partyID, &bucket, &balanceRaw); err != nil {
			return err
		}
		balance, err := accounting.ParseDecimal(balanceRaw)
		if err != nil {
			return err
		}
		row := api.AccountingReportRow{
			Balance: balance.String(),
			Credit:  accounting.Decimal{}.String(),
			Debit:   accounting.Decimal{}.String(),
			Key:     kind + ":" + partyID + ":" + bucket,
			Label:   kind + " · " + partyID + " · " + agingBucketLabel(bucket),
		}
		if kind == "receivable" {
			row.Debit = balance.String()
			totalDebit = totalDebit.Add(balance)
		} else {
			row.Credit = balance.String()
			totalCredit = totalCredit.Add(balance)
		}
		report.Rows = append(report.Rows, row)
	}
	report.TotalDebit = totalDebit.String()
	report.TotalCredit = totalCredit.String()
	return rows.Err()
}

func agingBucketLabel(bucket string) string {
	switch bucket {
	case "current":
		return "A vencer"
	case "1-30":
		return "Vencido 1–30 días"
	case "31-60":
		return "Vencido 31–60 días"
	case "61-90":
		return "Vencido 61–90 días"
	default:
		return "Vencido más de 90 días"
	}
}

func loadVATPositionReport(
	ctx context.Context,
	tx pgx.Tx,
	from, to time.Time,
	report *api.AccountingReport,
) error {
	var (
		salesNet, outputVAT       string
		purchasesNet, inputVAT    string
		withholdings, perceptions string
	)
	if err := tx.QueryRow(ctx, `
		SELECT
			coalesce(sum(
				net_amount
				* CASE WHEN voucher_type IN (3, 8, 13) THEN -1 ELSE 1 END
			), 0)::text,
			coalesce(sum(
				vat_amount
				* CASE WHEN voucher_type IN (3, 8, 13) THEN -1 ELSE 1 END
			), 0)::text
		  FROM fiscal.vouchers
		 WHERE status = 'authorized'
		   AND environment = 'production'
		   AND issue_date BETWEEN $1 AND $2
	`, from, to).Scan(&salesNet, &outputVAT); err != nil {
		return fmt.Errorf("load VAT sales: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT
			coalesce(sum(
				purchase.net_amount
				* CASE WHEN purchase.voucher_type IN (3, 8, 13) THEN -1 ELSE 1 END
			), 0)::text,
			coalesce(sum(
				coalesce((
					SELECT sum(tax.amount)
					  FROM fiscal.purchase_voucher_taxes AS tax
					 WHERE tax.org_id = purchase.org_id
					   AND tax.purchase_voucher_id = purchase.id
					   AND tax.tax_type = 'vat'
					   AND tax.creditable
				), 0)
				* CASE WHEN purchase.voucher_type IN (3, 8, 13) THEN -1 ELSE 1 END
			), 0)::text,
			coalesce(sum(
				purchase.withholding_amount
				* CASE WHEN purchase.voucher_type IN (3, 8, 13) THEN -1 ELSE 1 END
			), 0)::text,
			coalesce(sum(
				purchase.perception_amount
				* CASE WHEN purchase.voucher_type IN (3, 8, 13) THEN -1 ELSE 1 END
			), 0)::text
		  FROM fiscal.purchase_vouchers AS purchase
		 WHERE purchase.environment = 'production'
		   AND purchase.issue_date BETWEEN $1 AND $2
	`, from, to).Scan(
		&purchasesNet,
		&inputVAT,
		&withholdings,
		&perceptions,
	); err != nil {
		return fmt.Errorf("load VAT purchases: %w", err)
	}
	output, err := accounting.ParseDecimal(outputVAT)
	if err != nil {
		return err
	}
	input, err := accounting.ParseDecimal(inputVAT)
	if err != nil {
		return err
	}
	withholding, err := accounting.ParseDecimal(withholdings)
	if err != nil {
		return err
	}
	perception, err := accounting.ParseDecimal(perceptions)
	if err != nil {
		return err
	}
	position := output.Sub(input).Sub(withholding).Sub(perception)
	report.Rows = append(report.Rows,
		api.AccountingReportRow{
			Balance: salesNet,
			Credit:  outputVAT,
			Debit:   "0",
			Key:     "sales",
			Label:   "Libro IVA ventas",
		},
		api.AccountingReportRow{
			Balance: purchasesNet,
			Credit:  "0",
			Debit:   inputVAT,
			Key:     "purchases",
			Label:   "Libro IVA compras",
		},
		api.AccountingReportRow{
			Balance: position.String(),
			Credit:  outputVAT,
			Debit:   input.Add(withholding).Add(perception).String(),
			Key:     "vat-position",
			Label:   "Posición IVA",
		},
	)
	report.TotalDebit = input.Add(withholding).Add(perception).String()
	report.TotalCredit = output.String()
	return nil
}
