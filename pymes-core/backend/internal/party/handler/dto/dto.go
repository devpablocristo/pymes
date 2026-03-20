package dto

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
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
	Config    map[string]any `json:"config"`
	IsActive  bool           `json:"is_active"`
}

type PartyRoleInput struct {
	Role        string         `json:"role" binding:"required,min=2,max=64"`
	PriceListID *string        `json:"price_list_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type CreatePartyRequest struct {
	PartyType    string             `json:"party_type" binding:"required,oneof=person organization automated_agent"`
	DisplayName  string             `json:"display_name" binding:"required,min=2,max=200"`
	Email        string             `json:"email,omitempty" binding:"omitempty,email"`
	Phone        string             `json:"phone,omitempty"`
	Address      Address            `json:"address"`
	TaxID        string             `json:"tax_id,omitempty"`
	Notes        string             `json:"notes,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	Metadata     map[string]any     `json:"metadata,omitempty"`
	Person       *PartyPerson       `json:"person,omitempty"`
	Organization *PartyOrganization `json:"organization,omitempty"`
	Agent        *PartyAgent        `json:"agent,omitempty"`
	Roles        []PartyRoleInput   `json:"roles,omitempty"`
}

type UpdatePartyRequest struct {
	PartyType    string             `json:"party_type" binding:"required,oneof=person organization automated_agent"`
	DisplayName  string             `json:"display_name" binding:"required,min=2,max=200"`
	Email        string             `json:"email,omitempty" binding:"omitempty,email"`
	Phone        string             `json:"phone,omitempty"`
	Address      Address            `json:"address"`
	TaxID        string             `json:"tax_id,omitempty"`
	Notes        string             `json:"notes,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	Metadata     map[string]any     `json:"metadata,omitempty"`
	Person       *PartyPerson       `json:"person,omitempty"`
	Organization *PartyOrganization `json:"organization,omitempty"`
	Agent        *PartyAgent        `json:"agent,omitempty"`
}

type RelationshipInput struct {
	ToPartyID        string         `json:"to_party_id" binding:"required,uuid"`
	RelationshipType string         `json:"relationship_type" binding:"required,min=2,max=100"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	FromDate         *string        `json:"from_date,omitempty"`
	ThruDate         *string        `json:"thru_date,omitempty"`
}
