package dto

import reportdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/reports/usecases/domain"

type SalesSummaryResponse struct {
	From string                    `json:"from"`
	To   string                    `json:"to"`
	Data reportdomain.SalesSummary `json:"data"`
}

type SalesByProductResponse struct {
	From  string                            `json:"from"`
	To    string                            `json:"to"`
	Items []reportdomain.SalesByProductItem `json:"items"`
}

type SalesByCustomerResponse struct {
	From  string                             `json:"from"`
	To    string                             `json:"to"`
	Items []reportdomain.SalesByCustomerItem `json:"items"`
}

type SalesByPaymentResponse struct {
	From  string                            `json:"from"`
	To    string                            `json:"to"`
	Items []reportdomain.SalesByPaymentItem `json:"items"`
}

type InventoryValuationResponse struct {
	Items []reportdomain.InventoryValuationItem `json:"items"`
	Total float64                               `json:"total"`
}

type LowStockResponse struct {
	Items []reportdomain.LowStockItem `json:"items"`
}

type CashflowSummaryResponse struct {
	From string                       `json:"from"`
	To   string                       `json:"to"`
	Data reportdomain.CashflowSummary `json:"data"`
}

type ProfitMarginResponse struct {
	From string                    `json:"from"`
	To   string                    `json:"to"`
	Data reportdomain.ProfitMargin `json:"data"`
}
