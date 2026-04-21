package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type PartyModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	PartyType   string    `gorm:"column:party_type;not null"`
	DisplayName string    `gorm:"column:display_name;not null"`
	Email       string
	Phone       string
	Address     []byte `gorm:"type:jsonb"`
	TaxID       string `gorm:"column:tax_id"`
	Notes       string
	IsFavorite  bool           `gorm:"column:is_favorite;not null"`
	Tags        pq.StringArray `gorm:"type:text[]"`
	Metadata    []byte         `gorm:"type:jsonb"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func (PartyModel) TableName() string { return "parties" }

type PartyPersonModel struct {
	PartyID   uuid.UUID `gorm:"column:party_id;type:uuid;primaryKey"`
	FirstName string    `gorm:"column:first_name"`
	LastName  string    `gorm:"column:last_name"`
}

func (PartyPersonModel) TableName() string { return "party_persons" }

type PartyOrganizationModel struct {
	PartyID      uuid.UUID `gorm:"column:party_id;type:uuid;primaryKey"`
	LegalName    string    `gorm:"column:legal_name"`
	TradeName    string    `gorm:"column:trade_name"`
	TaxCondition string    `gorm:"column:tax_condition"`
}

func (PartyOrganizationModel) TableName() string { return "party_organizations" }

type PartyAgentModel struct {
	PartyID   uuid.UUID `gorm:"column:party_id;type:uuid;primaryKey"`
	AgentKind string    `gorm:"column:agent_kind"`
	Provider  string
	Config    []byte `gorm:"type:jsonb"`
	IsActive  bool   `gorm:"column:is_active"`
}

func (PartyAgentModel) TableName() string { return "party_agents" }

type PartyRoleModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	PartyID     uuid.UUID `gorm:"column:party_id;type:uuid;index;not null"`
	OrgID       uuid.UUID `gorm:"column:org_id;type:uuid;index;not null"`
	Role        string
	IsActive    bool       `gorm:"column:is_active"`
	PriceListID *uuid.UUID `gorm:"column:price_list_id;type:uuid"`
	Metadata    []byte     `gorm:"type:jsonb"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
}

func (PartyRoleModel) TableName() string { return "party_roles" }

type PartyRelationshipModel struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID            uuid.UUID  `gorm:"column:org_id;type:uuid;index;not null"`
	FromPartyID      uuid.UUID  `gorm:"column:from_party_id;type:uuid;index;not null"`
	ToPartyID        uuid.UUID  `gorm:"column:to_party_id;type:uuid;index;not null"`
	RelationshipType string     `gorm:"column:relationship_type"`
	Metadata         []byte     `gorm:"type:jsonb"`
	FromDate         time.Time  `gorm:"column:from_date"`
	ThruDate         *time.Time `gorm:"column:thru_date"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
}

func (PartyRelationshipModel) TableName() string { return "party_relationships" }
