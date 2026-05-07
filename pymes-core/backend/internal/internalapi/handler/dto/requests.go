package dto

import "encoding/json"

type ResolveAPIKeyRequest struct {
	APIKey string `json:"api_key" binding:"required"`
}

type CreateInAppNotificationRequest struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id" binding:"required"`
	Actor       string          `json:"actor" binding:"required"`
	Title       string          `json:"title" binding:"required"`
	Body        string          `json:"body" binding:"required"`
	Kind        string          `json:"kind"`
	EntityType  string          `json:"entity_type"`
	EntityID    string          `json:"entity_id"`
	ChatContext json.RawMessage `json:"chat_context"`
}

type ResolveCustomerRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}

type InternalAPILineItem struct {
	ProductID   string   `json:"product_id"`
	Description string   `json:"description"`
	Quantity    float64  `json:"quantity"`
	UnitPrice   float64  `json:"unit_price"`
	TaxRate     *float64 `json:"tax_rate,omitempty"`
}

type CreateQuoteRequest struct {
	TenantID     string                `json:"tenant_id" binding:"required"`
	CustomerID   string                `json:"customer_id"`
	CustomerName string                `json:"customer_name"`
	Items        []InternalAPILineItem `json:"items" binding:"required"`
	Notes        string                `json:"notes"`
	ValidUntil   *string               `json:"valid_until,omitempty"`
}

type CreateSaleRequest struct {
	TenantID      string                `json:"tenant_id" binding:"required"`
	CustomerID    string                `json:"customer_id"`
	CustomerName  string                `json:"customer_name"`
	QuoteID       string                `json:"quote_id"`
	PaymentMethod string                `json:"payment_method"`
	Items         []InternalAPILineItem `json:"items" binding:"required"`
	Notes         string                `json:"notes"`
}

type SendCustomerMessagingTextRequest struct {
	PartyID string `json:"party_id" binding:"required"`
	Body    string `json:"body" binding:"required"`
}
