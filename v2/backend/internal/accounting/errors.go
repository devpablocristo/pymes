package accounting

import "errors"

var (
	ErrInvalidDecimal       = errors.New("accounting: invalid decimal")
	ErrDivisionByZero       = errors.New("accounting: division by zero")
	ErrInvalidArgument      = errors.New("accounting: invalid argument")
	ErrNotFound             = errors.New("accounting: not found")
	ErrConflict             = errors.New("accounting: conflict")
	ErrVersionConflict      = errors.New("accounting: version conflict")
	ErrDuplicate            = errors.New("accounting: duplicate")
	ErrIdempotencyConflict  = errors.New("accounting: idempotency conflict")
	ErrUnbalancedEntry      = errors.New("accounting: journal entry is not balanced")
	ErrPeriodClosed         = errors.New("accounting: period is closed")
	ErrAccountArchived      = errors.New("accounting: account is archived")
	ErrAccountNotPostable   = errors.New("accounting: account is not postable")
	ErrAccountInUse         = errors.New("accounting: account is in use")
	ErrMappingMissing       = errors.New("accounting: account mapping is missing")
	ErrEntryImmutable       = errors.New("accounting: posted journal entry is immutable")
	ErrAlreadyReversed      = errors.New("accounting: journal entry already has a direct reversal")
	ErrReversalNotAllowed   = errors.New("accounting: journal entry cannot be reversed safely")
	ErrReconciliationClosed = errors.New("accounting: reconciliation is closed")
	ErrInflationIncomplete  = errors.New("accounting: inflation index series is incomplete")
)
