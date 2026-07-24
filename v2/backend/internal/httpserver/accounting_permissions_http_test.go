package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestJournalRoutesAllowViewAndRequireManageForMutations(t *testing.T) {
	t.Run("member with accounting view can list drafts", func(t *testing.T) {
		claims := teamReadClaims("org:member")
		active := teamReadActiveMembership("member")
		countCalled := false
		listCalled := false
		tx := &accountingPermissionTx{
			queryRow: func(context.Context, string, ...any) pgx.Row {
				countCalled = true
				return accountingPermissionRow{value: 0}
			},
			query: func(context.Context, string, ...any) (pgx.Rows, error) {
				listCalled = true
				return &teamReadRows{}, nil
			},
		}
		handler := newTeamReadTestHandler(
			t,
			claims,
			teamReadTransactor(t, claims, active, tx),
		)
		response := httptest.NewRecorder()

		handler.ServeHTTP(
			response,
			newTeamReadRequest(http.MethodGet, "/api/v1/accounting/drafts?limit=10"),
		)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body)
		}
		if !countCalled || !listCalled {
			t.Fatalf("journal read queries count=%t list=%t", countCalled, listCalled)
		}
	})

	t.Run("member without delegated manage cannot create draft", func(t *testing.T) {
		claims := teamReadClaims("org:member")
		active := teamReadActiveMembership("member")
		delegatedPermissionChecked := false
		tx := &accountingPermissionTx{
			queryRow: func(context.Context, string, ...any) pgx.Row {
				delegatedPermissionChecked = true
				return accountingPermissionRow{value: false}
			},
		}
		handler := newTeamReadTestHandler(
			t,
			claims,
			teamReadTransactor(t, claims, active, tx),
		)
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/accounting/drafts",
			bytes.NewBufferString(
				`{"accounting_date":"2026-07-24","description":"","currency":"ARS","lines":[]}`,
			),
		)
		request.Header.Set("Authorization", "Bearer team-read-token")
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "journal-permission-test")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		assertAPIError(t, response, http.StatusForbidden, "IAM_FORBIDDEN")
		if !delegatedPermissionChecked {
			t.Fatal("accounting manage delegation was not checked")
		}
	})
}

func TestJournalWorkflowErrorsHaveStableCodes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{
			name:   "already reversed",
			err:    errAccountingAlreadyReversed,
			status: http.StatusConflict,
			code:   "ACCOUNTING_ENTRY_ALREADY_REVERSED",
		},
		{
			name:   "reversal blocked",
			err:    errAccountingReversalBlocked,
			status: http.StatusConflict,
			code:   "ACCOUNTING_REVERSAL_NOT_ALLOWED",
		},
		{
			name:   "account archived",
			err:    errAccountingAccountArchived,
			status: http.StatusConflict,
			code:   "ACCOUNTING_ACCOUNT_ARCHIVED",
		},
		{
			name:   "account not postable",
			err:    errAccountingNotPostable,
			status: http.StatusUnprocessableEntity,
			code:   "ACCOUNTING_ACCOUNT_NOT_POSTABLE",
		},
		{
			name:   "closed reconciliation",
			err:    errAccountingReconciliationClosed,
			status: http.StatusConflict,
			code:   "ACCOUNTING_RECONCILIATION_CLOSED",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeBusinessError(response, test.err)
			if response.Code != test.status {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					response.Code,
					test.status,
					response.Body,
				)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body.Error.Code != test.code {
				t.Fatalf("error code = %q, want %q", body.Error.Code, test.code)
			}
		})
	}
}

type accountingPermissionTx struct {
	pgx.Tx
	queryRow func(context.Context, string, ...any) pgx.Row
	query    func(context.Context, string, ...any) (pgx.Rows, error)
}

func (tx *accountingPermissionTx) QueryRow(
	ctx context.Context,
	query string,
	args ...any,
) pgx.Row {
	if tx.queryRow == nil {
		panic("unexpected QueryRow call")
	}
	return tx.queryRow(ctx, query, args...)
}

func (tx *accountingPermissionTx) Query(
	ctx context.Context,
	query string,
	args ...any,
) (pgx.Rows, error) {
	if tx.query == nil {
		panic("unexpected Query call")
	}
	return tx.query(ctx, query, args...)
}

type accountingPermissionRow struct {
	value any
	err   error
}

func (row accountingPermissionRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(destinations) != 1 {
		return errors.New("unexpected accounting permission scan destination count")
	}
	switch destination := destinations[0].(type) {
	case *int:
		*destination = row.value.(int)
	case *bool:
		*destination = row.value.(bool)
	default:
		return errors.New("unexpected accounting permission scan destination")
	}
	return nil
}
