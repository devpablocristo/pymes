// Package models contains transport records consumed by the commerce worker.
package models

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
