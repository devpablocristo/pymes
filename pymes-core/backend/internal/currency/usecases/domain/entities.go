package domain

import (
	"time"

	"github.com/google/uuid"
)

type ExchangeRate struct {
	ID           uuid.UUID `json:"id"`
	OrgID        uuid.UUID `json:"org_id"`
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	RateType     string    `json:"rate_type"`
	BuyRate      float64   `json:"buy_rate"`
	SellRate     float64   `json:"sell_rate"`
	Source       string    `json:"source"`
	RateDate     time.Time `json:"rate_date"`
	CreatedAt    time.Time `json:"created_at"`
}
