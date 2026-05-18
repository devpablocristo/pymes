package domain

type SalesSummary struct {
	TotalSales    float64 `json:"total_sales"`
	CountSales    int64   `json:"count_sales"`
	AverageTicket float64 `json:"average_ticket"`
}

type SalesByProductItem struct {
	ProductID   string  `json:"product_id,omitempty"`
	ProductName string  `json:"product_name"`
	Quantity    float64 `json:"quantity"`
	Revenue     float64 `json:"revenue"`
}

type SalesByServiceItem struct {
	ServiceID   string  `json:"service_id,omitempty"`
	ServiceName string  `json:"service_name"`
	Quantity    float64 `json:"quantity"`
	Revenue     float64 `json:"revenue"`
}

type SalesByCustomerItem struct {
	CustomerID   string  `json:"customer_id,omitempty"`
	CustomerName string  `json:"customer_name"`
	Total        float64 `json:"total"`
	Count        int64   `json:"count"`
}

type SalesByPaymentItem struct {
	PaymentMethod string  `json:"payment_method"`
	Total         float64 `json:"total"`
	Count         int64   `json:"count"`
}

type InventoryValuationItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	SKU         string  `json:"sku,omitempty"`
	Quantity    float64 `json:"quantity"`
	CostPrice   float64 `json:"cost_price"`
	Valuation   float64 `json:"valuation"`
}

type LowStockItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	SKU         string  `json:"sku,omitempty"`
	Quantity    float64 `json:"quantity"`
	MinQuantity float64 `json:"min_quantity"`
}

type CashflowSummary struct {
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	Balance      float64 `json:"balance"`
}

type ProfitMargin struct {
	Revenue     float64 `json:"revenue"`
	Cost        float64 `json:"cost"`
	GrossProfit float64 `json:"gross_profit"`
	MarginPct   float64 `json:"margin_pct"`
}
