package models

import "time"

// FiscalSnapshot contains the persisted fiscal fields needed to associate a
// credit or debit note with its source voucher.
type FiscalSnapshot struct {
	IssueDate string `json:"issue_date"`
}

// ReversalSnapshot is the stable persistence representation hashed for an
// accounting reversal.
type ReversalSnapshot struct {
	ID           string
	DocumentKind string
	DocumentID   string
	Reason       string
	EffectiveAt  time.Time
}

type AccountingApplicationSnapshot struct {
	ID               string
	DebitOpenItemID  string
	CreditOpenItemID string
	Amount           string
	Currency         string
}
