package httpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	errBusinessNotFound                 = errors.New("business resource not found")
	errBusinessInvalidRequest           = errors.New("business request is invalid")
	errBusinessIdempotency              = errors.New("business idempotency key conflict")
	errBusinessVersionConflict          = errors.New("business resource version conflict")
	errBusinessDuplicate                = errors.New("business resource already exists")
	errBusinessInvalidTransition        = errors.New("business transition is invalid")
	errBusinessPeriodClosed             = errors.New("accounting period is closed")
	errBusinessUnbalanced               = errors.New("journal entry is unbalanced")
	errBusinessImmutable                = errors.New("business resource is immutable")
	errAccountingAlreadyReversed        = errors.New("accounting entry was already reversed")
	errAccountingReversalBlocked        = errors.New("accounting entry cannot be reversed safely")
	errAccountingAccountArchived        = errors.New("accounting account is archived")
	errAccountingNotPostable            = errors.New("accounting account is not postable")
	errAccountingReconciliationClosed   = errors.New("accounting reconciliation is closed")
	errAccountingAccountStructureLocked = errors.New("accounting account structure is locked")
	errAccountingAccountParentInvalid   = errors.New("accounting account parent is invalid")
	errAccountingAccountHierarchyCycle  = errors.New("accounting account hierarchy contains a cycle")
	errAccountingMappingIncompatible    = errors.New("accounting mapping is incompatible")
	errAccountingAccountMapped          = errors.New("accounting account is mapped")
	errAccountingFinancialDependency    = errors.New("accounting account has a financial dependency")
	errAccountingAccountHasChildren     = errors.New("accounting account has active children")
	errAccountingAccountParentInactive  = errors.New("accounting account parent is inactive")
	errAccountingAccountProtected       = errors.New("accounting account is protected")
	errAccountingFiscalYearOverlap      = errors.New("accounting fiscal year overlaps")
	errAccountingPeriodSequence         = errors.New("accounting period sequence is invalid")
	errAccountingPeriodInFuture         = errors.New("accounting period is in the future")
	errAccountingFiscalYearCloseOrder   = errors.New("accounting fiscal year close order is invalid")
	errAccountingFiscalYearReopenOrder  = errors.New("accounting fiscal year reopen order is invalid")
	errAccountingCloseChecklistBlocked  = errors.New("accounting close checklist is blocked")
	errAccountingAnnualClosePending     = errors.New("accounting annual close is pending")
	errAccountingAnnualCloseNotRequired = errors.New("accounting annual close is not required")
	errFiscalUncertain                  = errors.New("fiscal authorization is uncertain")
	errFiscalProductionNotReady         = errors.New("fiscal production prerequisites are incomplete")
)

type businessWork func(
	context.Context,
	pgx.Tx,
	platformiam.ActiveMembership,
	clerkadapter.SessionClaims,
) error

type businessRequestError struct {
	cause error
}

func (e *businessRequestError) Error() string {
	return e.cause.Error()
}

func (e *businessRequestError) Unwrap() error {
	return e.cause
}

// withinBusinessTx keeps every accounting and fiscal query inside the same
// verified session transaction used by IAM. The platform transactor sets
// app.org_id transaction-locally, so PostgreSQL RLS remains the final tenant
// boundary even if a query forgets an explicit organization predicate.
func (h *IAMAPI) withinBusinessTx(
	w http.ResponseWriter,
	r *http.Request,
	permission productiam.Permission,
	work businessWork,
) bool {
	return h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			if err := requireBusinessPermission(ctx, tx, active, claims, permission); err != nil {
				return err
			}
			if err := work(ctx, tx, active, claims); err != nil {
				return &businessRequestError{cause: err}
			}
			return nil
		},
	)
}

// requireBusinessPermission supports the deliberately small delegation model:
// normal role permissions plus an explicit accounting/fiscal manage grant on
// the active membership. It is not a general-purpose policy language.
func requireBusinessPermission(
	ctx context.Context,
	tx pgx.Tx,
	active platformiam.ActiveMembership,
	claims clerkadapter.SessionClaims,
	permission productiam.Permission,
) error {
	if err := requirePermission(active, claims, permission); err == nil {
		return nil
	}
	if permission != productiam.PermissionAccountingManage &&
		permission != productiam.PermissionFiscalManage {
		return errIAMForbidden
	}

	var granted bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM iam.membership_permissions AS permission_grant
			 WHERE permission_grant.membership_id = $1::uuid
			   AND permission_grant.permission = $2
		)
	`, active.MembershipID, string(permission)).Scan(&granted)
	if err != nil {
		return fmt.Errorf("load delegated business permission: %w", err)
	}
	if !granted {
		return errIAMForbidden
	}
	return nil
}

func writeBusinessError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "BUSINESS_INTERNAL"
	message := "The business operation could not be completed"

	switch {
	case errors.Is(err, errBusinessInvalidRequest):
		status, code, message = http.StatusBadRequest, "REQUEST_INVALID", "The request violates a business rule"
	case errors.Is(err, errBusinessIdempotency):
		status, code, message = http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "The idempotency key was already used for a different request"
	case errors.Is(err, errBusinessNotFound), errors.Is(err, pgx.ErrNoRows):
		status, code, message = http.StatusNotFound, "RESOURCE_NOT_FOUND", "The requested resource does not exist"
	case errors.Is(err, errBusinessVersionConflict):
		status, code, message = http.StatusConflict, "VERSION_CONFLICT", "The resource was changed by another operation"
	case errors.Is(err, errBusinessDuplicate):
		status, code, message = http.StatusConflict, "RESOURCE_DUPLICATE", "The resource already exists"
	case errors.Is(err, errBusinessInvalidTransition):
		status, code, message = http.StatusConflict, "INVALID_TRANSITION", "The requested state transition is not allowed"
	case errors.Is(err, errBusinessPeriodClosed):
		status, code, message = http.StatusConflict, "ACCOUNTING_PERIOD_CLOSED", "The accounting period does not allow posting"
	case errors.Is(err, errBusinessUnbalanced):
		status, code, message = http.StatusUnprocessableEntity, "ACCOUNTING_UNBALANCED", "Debit and credit totals must be equal"
	case errors.Is(err, errBusinessImmutable):
		status, code, message = http.StatusConflict, "RESOURCE_IMMUTABLE", "The resource is immutable"
	case errors.Is(err, errAccountingAlreadyReversed):
		status, code, message = http.StatusConflict, "ACCOUNTING_ENTRY_ALREADY_REVERSED", "El asiento ya fue revertido"
	case errors.Is(err, errAccountingReversalBlocked):
		status, code, message = http.StatusConflict, "ACCOUNTING_REVERSAL_NOT_ALLOWED", "Este asiento tiene efectos vinculados y no puede revertirse desde el Diario"
	case errors.Is(err, errAccountingAccountArchived):
		status, code, message = http.StatusConflict, "ACCOUNTING_ACCOUNT_ARCHIVED", "La cuenta contable está archivada"
	case errors.Is(err, errAccountingNotPostable):
		status, code, message = http.StatusUnprocessableEntity, "ACCOUNTING_ACCOUNT_NOT_POSTABLE", "La cuenta contable no admite imputaciones"
	case errors.Is(err, errAccountingReconciliationClosed):
		status, code, message = http.StatusConflict, "ACCOUNTING_RECONCILIATION_CLOSED", "La cuenta tiene una conciliación cerrada; reabrila antes de contabilizar"
	case errors.Is(err, errAccountingAccountStructureLocked):
		status, code, message = http.StatusConflict, "ACCOUNTING_ACCOUNT_STRUCTURE_LOCKED", "La cuenta ya tiene uso o dependencias; sólo se puede cambiar el nombre"
	case errors.Is(err, errAccountingAccountParentInvalid):
		status, code, message = http.StatusUnprocessableEntity, "ACCOUNTING_ACCOUNT_PARENT_INVALID", "El rubro superior no es compatible con esta cuenta"
	case errors.Is(err, errAccountingAccountHierarchyCycle):
		status, code, message = http.StatusConflict, "ACCOUNTING_ACCOUNT_HIERARCHY_CYCLE", "La jerarquía propuesta formaría un ciclo"
	case errors.Is(err, errAccountingMappingIncompatible):
		status, code, message = http.StatusUnprocessableEntity, "ACCOUNTING_MAPPING_INCOMPATIBLE", "La cuenta no es compatible con el rol funcional"
	case errors.Is(err, errAccountingAccountMapped):
		status, code, message = http.StatusConflict, "ACCOUNTING_ACCOUNT_MAPPED", "Remapeá el rol funcional antes de archivar la cuenta"
	case errors.Is(err, errAccountingFinancialDependency):
		status, code, message = http.StatusConflict, "ACCOUNTING_ACCOUNT_FINANCIAL_DEPENDENCY", "La cuenta está vinculada a una cuenta financiera activa"
	case errors.Is(err, errAccountingAccountHasChildren):
		status, code, message = http.StatusConflict, "ACCOUNTING_ACCOUNT_HAS_ACTIVE_CHILDREN", "Archivá primero las cuentas y rubros dependientes"
	case errors.Is(err, errAccountingAccountParentInactive):
		status, code, message = http.StatusConflict, "ACCOUNTING_ACCOUNT_PARENT_INACTIVE", "Restaurá primero el rubro superior"
	case errors.Is(err, errAccountingAccountProtected):
		status, code, message = http.StatusConflict, "ACCOUNTING_ACCOUNT_PROTECTED", "Este rubro pertenece a la estructura base y no admite la operación"
	case errors.Is(err, errAccountingFiscalYearOverlap):
		status, code, message = http.StatusConflict, "ACCOUNTING_FISCAL_YEAR_OVERLAP", "El ejercicio se superpone con otro existente"
	case errors.Is(err, errAccountingPeriodSequence):
		status, code, message = http.StatusConflict, "ACCOUNTING_PERIOD_SEQUENCE_INVALID", "Los períodos deben cerrarse en orden cronológico"
	case errors.Is(err, errAccountingPeriodInFuture):
		status, code, message = http.StatusConflict, "ACCOUNTING_PERIOD_FUTURE", "No se puede cerrar un período cuya fecha final todavía es futura"
	case errors.Is(err, errAccountingFiscalYearCloseOrder):
		status, code, message = http.StatusConflict, "ACCOUNTING_FISCAL_YEAR_CLOSE_ORDER", "Completá primero los cierres anteriores del ejercicio"
	case errors.Is(err, errAccountingFiscalYearReopenOrder):
		status, code, message = http.StatusConflict, "ACCOUNTING_FISCAL_YEAR_REOPEN_ORDER", "Los cierres deben reabrirse en orden inverso"
	case errors.Is(err, errAccountingCloseChecklistBlocked):
		status, code, message = http.StatusConflict, "ACCOUNTING_CLOSE_CHECKLIST_BLOCKED", "Resolvé los controles pendientes antes de continuar el cierre"
	case errors.Is(err, errAccountingAnnualClosePending):
		status, code, message = http.StatusConflict, "ACCOUNTING_ANNUAL_CLOSE_PENDING", "El cierre anual debe completarse antes de bloquear el último período"
	case errors.Is(err, errAccountingAnnualCloseNotRequired):
		status, code, message = http.StatusConflict, "ACCOUNTING_ANNUAL_CLOSE_NOT_REQUIRED", "El ejercicio no tiene saldos temporales que requieran un asiento de cierre"
	case errors.Is(err, errFiscalUncertain):
		status, code, message = http.StatusConflict, "FISCAL_AUTHORIZATION_UNCERTAIN", "ARCA authorization must be reconciled before retrying"
	case errors.Is(err, errFiscalProductionNotReady):
		status, code, message = http.StatusConflict, "FISCAL_PRODUCTION_NOT_READY", "Fiscal homologation and production prerequisites are incomplete"
	default:
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) {
			if accountingAccountConstraintError(
				w,
				postgresError.ConstraintName,
			) {
				return
			}
			switch postgresError.Code {
			case "23505":
				status, code, message = http.StatusConflict, "RESOURCE_DUPLICATE", "The resource already exists"
			case "23503", "23514", "22023", "22P02":
				status, code, message = http.StatusBadRequest, "REQUEST_INVALID", "The request violates a business rule"
			case "40001", "40P01":
				status, code, message = http.StatusConflict, "CONCURRENT_OPERATION", "The operation conflicted with another request"
			case "42501":
				status, code, message = http.StatusForbidden, "IAM_FORBIDDEN", "Insufficient permission"
			case "55000":
				status, code, message = http.StatusConflict, "INVALID_TRANSITION", "The requested operation is not allowed in the current state"
			}
		}
	}
	writeAPIError(w, status, code, message)
}

func accountingAccountConstraintError(
	w http.ResponseWriter,
	constraint string,
) bool {
	var (
		status  int
		code    string
		message string
	)
	switch constraint {
	case "accounting_accounts_structure_locked",
		"accounting_accounts_node_type_immutable",
		"accounting_accounts_postable_leaf",
		"accounting_accounts_trash_unused":
		status, code, message = http.StatusConflict,
			"ACCOUNTING_ACCOUNT_STRUCTURE_LOCKED",
			"La cuenta ya tiene uso o dependencias; sólo se puede cambiar el nombre"
	case "accounting_accounts_invalid_parent",
		"accounting_accounts_parent_not_postable",
		"accounting_accounts_parent_class":
		status, code, message = http.StatusUnprocessableEntity,
			"ACCOUNTING_ACCOUNT_PARENT_INVALID",
			"El rubro superior no es compatible con esta cuenta"
	case "accounting_accounts_hierarchy_cycle",
		"accounting_accounts_hierarchy_acyclic":
		status, code, message = http.StatusConflict,
			"ACCOUNTING_ACCOUNT_HIERARCHY_CYCLE",
			"La jerarquía propuesta formaría un ciclo"
	case "accounting_account_mappings_incompatible",
		"accounting_account_mappings_unknown_role",
		"accounting_account_mappings_alias_read_only":
		status, code, message = http.StatusUnprocessableEntity,
			"ACCOUNTING_MAPPING_INCOMPATIBLE",
			"La cuenta no es compatible con el rol funcional"
	case "accounting_accounts_mapping_blocks_archive":
		status, code, message = http.StatusConflict,
			"ACCOUNTING_ACCOUNT_MAPPED",
			"Remapeá el rol funcional antes de archivar la cuenta"
	case "accounting_accounts_financial_blocks_archive":
		status, code, message = http.StatusConflict,
			"ACCOUNTING_ACCOUNT_FINANCIAL_DEPENDENCY",
			"La cuenta está vinculada a una cuenta financiera activa"
	case "accounting_accounts_active_children":
		status, code, message = http.StatusConflict,
			"ACCOUNTING_ACCOUNT_HAS_ACTIVE_CHILDREN",
			"Archivá primero las cuentas y rubros dependientes"
	case "accounting_accounts_parent_inactive":
		status, code, message = http.StatusConflict,
			"ACCOUNTING_ACCOUNT_PARENT_INACTIVE",
			"Restaurá primero el rubro superior"
	case "accounting_accounts_system_protected",
		"accounting_accounts_system_key_immutable":
		status, code, message = http.StatusConflict,
			"ACCOUNTING_ACCOUNT_PROTECTED",
			"Este rubro pertenece a la estructura base y no admite la operación"
	default:
		return false
	}
	writeAPIError(w, status, code, message)
	return true
}

func decodeBusinessBody(w http.ResponseWriter, r *http.Request, destination any) bool {
	return decodeIAMCommandBody(w, r, destination)
}

type keysetCursor struct {
	Sort string `json:"s"`
	ID   string `json:"i"`
}

func encodeKeysetCursor(sortValue, id string) *string {
	raw, err := json.Marshal(keysetCursor{Sort: sortValue, ID: id})
	if err != nil {
		return nil
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	return &value
}

func decodeKeysetCursor(raw *string) (keysetCursor, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return keysetCursor{}, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(*raw)
	if err != nil {
		return keysetCursor{}, errors.New("invalid cursor")
	}
	var cursor keysetCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil ||
		strings.TrimSpace(cursor.Sort) == "" ||
		strings.TrimSpace(cursor.ID) == "" {
		return keysetCursor{}, errors.New("invalid cursor")
	}
	return cursor, nil
}

func normalizedLimit(raw *int) int {
	if raw == nil || *raw <= 0 {
		return 50
	}
	if *raw > 200 {
		return 200
	}
	return *raw
}
