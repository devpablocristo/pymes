package domain

type Dashboard struct {
	SalesToday       float64        `json:"sales_today"`
	SalesMonth       float64        `json:"sales_month"`
	CashflowBalance  float64        `json:"cashflow_balance"`
	PendingQuotes    int64          `json:"pending_quotes"`
	LowStockProducts int64          `json:"low_stock_products"`
	TopProducts      []ProductTotal `json:"top_products_month"`
	RecentSales      []RecentSale   `json:"recent_sales"`
}

type ProductTotal struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	Total     float64 `json:"total"`
}

type RecentSale struct {
	ID           string  `json:"id"`
	Number       string  `json:"number"`
	CustomerName string  `json:"customer_name"`
	Total        float64 `json:"total"`
	Currency     string  `json:"currency"`
	CreatedAt    string  `json:"created_at"`
}
