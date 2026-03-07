package domain

import (
	"time"

	"github.com/google/uuid"
)

type Viewer struct {
	OrgID  uuid.UUID
	Actor  string
	Role   string
	Scopes []string
}

type WidgetSize struct {
	W int `json:"w"`
	H int `json:"h"`
}

type WidgetDefinition struct {
	WidgetKey         string         `json:"widget_key"`
	Title             string         `json:"title"`
	Description       string         `json:"description"`
	Domain            string         `json:"domain"`
	Kind              string         `json:"kind"`
	DefaultSize       WidgetSize     `json:"default_size"`
	MinW              int            `json:"min_w"`
	MinH              int            `json:"min_h"`
	MaxW              int            `json:"max_w"`
	MaxH              int            `json:"max_h"`
	SupportedContexts []string       `json:"supported_contexts"`
	AllowedRoles      []string       `json:"allowed_roles"`
	RequiredScopes    []string       `json:"required_scopes,omitempty"`
	SettingsSchema    map[string]any `json:"settings_schema"`
	DataEndpoint      string         `json:"data_endpoint"`
	Status            string         `json:"status"`
}

type LayoutItem struct {
	WidgetKey  string         `json:"widget_key"`
	InstanceID string         `json:"instance_id"`
	X          int            `json:"x"`
	Y          int            `json:"y"`
	W          int            `json:"w"`
	H          int            `json:"h"`
	Visible    bool           `json:"visible"`
	Settings   map[string]any `json:"settings"`
	Pinned     bool           `json:"pinned"`
	OrderHint  int            `json:"order_hint"`
}

type DashboardLayout struct {
	Source    string       `json:"source"`
	LayoutKey string       `json:"layout_key"`
	Version   int          `json:"version"`
	Items     []LayoutItem `json:"items"`
}

type Dashboard struct {
	Context          string             `json:"context"`
	Layout           DashboardLayout    `json:"layout"`
	AvailableWidgets []WidgetDefinition `json:"available_widgets"`
}

type WidgetCatalog struct {
	Context string             `json:"context"`
	Items   []WidgetDefinition `json:"items"`
}

type SaveDashboardInput struct {
	Viewer  Viewer
	Context string
	Items   []LayoutItem
}

type DefaultLayout struct {
	LayoutKey string
	Context   string
	Name      string
	Items     []LayoutItem
}

type UserLayout struct {
	UserID                      *uuid.UUID
	UserActor                   string
	Context                     string
	LayoutVersion               int
	Items                       []LayoutItem
	LastAppliedDefaultLayoutKey string
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
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
