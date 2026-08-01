// Package models contains transport records consumed by the commerce worker.
package models

import "time"

type SaleEvent struct {
	SaleID        string `json:"sale_id"`
	CredentialRef string `json:"credential_ref"`
}

type PaymentEvent struct {
	PaymentID string `json:"payment_id"`
}

type PurchaseEvent struct {
	PurchaseID string `json:"purchase_id"`
}

type AccountingApplicationEvent struct {
	ApplicationID string `json:"application_id"`
}

type AccountingReversalEvent struct {
	ReversalID string `json:"reversal_id"`
}

type AccountingApplicationSnapshot struct {
	ID               string
	DebitOpenItemID  string
	CreditOpenItemID string
	Amount           string
	Currency         string
}

type AccountingApplicationReversalSnapshot struct {
	ApplicationID string
	Reason        string
	ReversedAt    time.Time
}

type JournalReversalSnapshot struct {
	ID             string
	JournalEntryID string
	Reason         string
	EffectiveAt    time.Time
}
