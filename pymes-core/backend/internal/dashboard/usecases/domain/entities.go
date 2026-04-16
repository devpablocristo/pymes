package domain

import (
	"time"

	"github.com/google/uuid"
)

type Viewer struct {
	OrgID  uuid.UUID
	BranchID *uuid.UUID
	Actor  string
	Role   string
	Scopes []string
}

type WidgetDefinition struct {
	WidgetKey         string         `json:"widget_key"`
	Title             string         `json:"title"`
	Description       string         `json:"description"`
	Domain            string         `json:"domain"`
	Kind              string         `json:"kind"`
	SupportedContexts []string       `json:"supported_contexts"`
	AllowedRoles      []string       `json:"allowed_roles"`
	RequiredScopes    []string       `json:"required_scopes,omitempty"`
	SettingsSchema    map[string]any `json:"settings_schema"`
	DataEndpoint      string         `json:"data_endpoint"`
	Status            string         `json:"status"`
}

type SalesSummaryData struct {
	Period        string  `json:"period"`
	TotalSales    float64 `json:"total_sales"`
	CountSales    int64   `json:"count_sales"`
	AverageTicket float64 `json:"average_ticket"`
}

type CashflowSummaryData struct {
	Period       string  `json:"period"`
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	Balance      float64 `json:"balance"`
}

type QuotesPipelineData struct {
	Draft        int64 `json:"draft"`
	Sent         int64 `json:"sent"`
	Accepted     int64 `json:"accepted"`
	Rejected     int64 `json:"rejected"`
	PendingTotal int64 `json:"pending_total"`
}

type LowStockItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	SKU         string  `json:"sku,omitempty"`
	Quantity    float64 `json:"quantity"`
	MinQuantity float64 `json:"min_quantity"`
}

type LowStockData struct {
	Total int64          `json:"total"`
	Items []LowStockItem `json:"items"`
}

type RecentSale struct {
	ID           string  `json:"id"`
	Number       string  `json:"number"`
	CustomerName string  `json:"customer_name"`
	Total        float64 `json:"total"`
	Currency     string  `json:"currency"`
	CreatedAt    string  `json:"created_at"`
}

type RecentSalesData struct {
	Items []RecentSale `json:"items"`
}

type TopProduct struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	Total     float64 `json:"total"`
}

type TopProductsData struct {
	Period string       `json:"period"`
	Items  []TopProduct `json:"items"`
}

type TopService struct {
	ServiceID string  `json:"service_id"`
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	Total     float64 `json:"total"`
}

type TopServicesData struct {
	Period string       `json:"period"`
	Items  []TopService `json:"items"`
}

type BillingStatusData struct {
	PlanCode   string         `json:"plan_code"`
	Status     string         `json:"status"`
	HardLimits map[string]any `json:"hard_limits"`
	UpdatedAt  *time.Time     `json:"updated_at,omitempty"`
}

type AuditActivityItem struct {
	ID           string `json:"id"`
	Actor        string `json:"actor"`
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type AuditActivityData struct {
	Items []AuditActivityItem `json:"items"`
}
