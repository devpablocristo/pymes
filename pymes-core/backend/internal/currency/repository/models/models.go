package models

import (
	"time"

	"github.com/google/uuid"
)

type ExchangeRateModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index;not null"`
	FromCurrency string
	ToCurrency   string
	RateType     string
	BuyRate      float64
	SellRate     float64
	Source       string
	RateDate     time.Time
	CreatedAt    time.Time
}

func (ExchangeRateModel) TableName() string { return "exchange_rates" }
