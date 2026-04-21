package domain

import (
	"time"

	"github.com/google/uuid"
)

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

type Party struct {
	ID           uuid.UUID          `json:"id"`
	OrgID        uuid.UUID          `json:"org_id"`
	PartyType    string             `json:"party_type"`
	DisplayName  string             `json:"display_name"`
	Email        string             `json:"email,omitempty"`
	Phone        string             `json:"phone,omitempty"`
	Address      Address            `json:"address"`
	TaxID        string             `json:"tax_id,omitempty"`
	Notes        string             `json:"notes,omitempty"`
	IsFavorite   bool               `json:"is_favorite"`
	Tags         []string           `json:"tags,omitempty"`
	Metadata     map[string]any     `json:"metadata,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
	DeletedAt    *time.Time         `json:"deleted_at,omitempty"`
	Person       *PartyPerson       `json:"person,omitempty"`
	Organization *PartyOrganization `json:"organization,omitempty"`
	Agent        *PartyAgent        `json:"agent,omitempty"`
	Roles        []PartyRole        `json:"roles,omitempty"`
}

type PartyPerson struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type PartyOrganization struct {
	LegalName    string `json:"legal_name"`
	TradeName    string `json:"trade_name"`
	TaxCondition string `json:"tax_condition"`
}

type PartyAgent struct {
	AgentKind string         `json:"agent_kind"`
	Provider  string         `json:"provider"`
	Config    map[string]any `json:"config,omitempty"`
	IsActive  bool           `json:"is_active"`
}

type PartyRole struct {
	ID          uuid.UUID      `json:"id"`
	PartyID     uuid.UUID      `json:"party_id"`
	OrgID       uuid.UUID      `json:"org_id"`
	Role        string         `json:"role"`
	IsActive    bool           `json:"is_active"`
	PriceListID *uuid.UUID     `json:"price_list_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type PartyRelationship struct {
	ID               uuid.UUID      `json:"id"`
	OrgID            uuid.UUID      `json:"org_id"`
	FromPartyID      uuid.UUID      `json:"from_party_id"`
	ToPartyID        uuid.UUID      `json:"to_party_id"`
	RelationshipType string         `json:"relationship_type"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	FromDate         time.Time      `json:"from_date"`
	ThruDate         *time.Time     `json:"thru_date,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}
