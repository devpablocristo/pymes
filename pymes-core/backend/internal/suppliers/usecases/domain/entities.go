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

type Supplier struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Name        string
	TaxID       string
	Email       string
	Phone       string
	Address     Address
	ContactName string
	Notes       string
	Tags        []string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}
