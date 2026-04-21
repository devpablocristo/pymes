package domain

import (
	"time"

	"github.com/google/uuid"
)

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

type Customer struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Type      string
	Name      string
	TaxID     string
	Email     string
	Phone     string
	Address   Address
	Notes     string
	IsFavorite bool
	Tags      []string
	Metadata  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type SaleHistoryItem struct {
	ID            uuid.UUID `json:"id"`
	Number        string    `json:"number"`
	Status        string    `json:"status"`
	PaymentMethod string    `json:"payment_method"`
	Total         float64   `json:"total"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
}
