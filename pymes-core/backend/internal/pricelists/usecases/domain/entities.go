package domain

import (
	"time"

	"github.com/google/uuid"
)

type PriceList struct {
	ID          uuid.UUID       `json:"id"`
	OrgID       uuid.UUID       `json:"org_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	IsDefault   bool            `json:"is_default"`
	Markup      float64         `json:"markup"`
	IsActive    bool            `json:"is_active"`
	IsFavorite  bool            `json:"is_favorite"`
	Tags        []string        `json:"tags"`
	ArchivedAt  *time.Time      `json:"archived_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Items       []PriceListItem `json:"items,omitempty"`
}

type PriceListItem struct {
	ProductID *uuid.UUID `json:"product_id,omitempty"`
	ServiceID *uuid.UUID `json:"service_id,omitempty"`
	Price     float64    `json:"price"`
}
