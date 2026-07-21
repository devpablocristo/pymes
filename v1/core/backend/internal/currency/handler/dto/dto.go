package dto

type CreateExchangeRateRequest struct {
	FromCurrency string  `json:"from_currency" binding:"required"`
	ToCurrency   string  `json:"to_currency" binding:"required"`
	RateType     string  `json:"rate_type" binding:"required"`
	BuyRate      float64 `json:"buy_rate" binding:"required"`
	SellRate     float64 `json:"sell_rate" binding:"required"`
	Source       string  `json:"source"`
	RateDate     string  `json:"rate_date,omitempty"`
}
