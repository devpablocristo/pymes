package wire

import (
	"time"

	"github.com/google/uuid"
)

type pymesOrgRow struct {
	ID         uuid.UUID `gorm:"column:id"`
	ExternalID *string   `gorm:"column:external_id"`
	Name       string    `gorm:"column:name"`
	Slug       *string   `gorm:"column:slug"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (pymesOrgRow) TableName() string { return "orgs" }

type pymesUserRow struct {
	ID         uuid.UUID  `gorm:"column:id"`
	ExternalID string     `gorm:"column:external_id"`
	Email      string     `gorm:"column:email"`
	Name       string     `gorm:"column:name"`
	GivenName  string     `gorm:"column:given_name"`
	FamilyName string     `gorm:"column:family_name"`
	Phone      string     `gorm:"column:phone"`
	AvatarURL  string     `gorm:"column:avatar_url"`
	DeletedAt  *time.Time `gorm:"column:deleted_at"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at"`
}

func (pymesUserRow) TableName() string { return "users" }

type pymesOrgMemberRow struct {
	ID        uuid.UUID    `gorm:"column:id"`
	OrgID     uuid.UUID    `gorm:"column:org_id"`
	UserID    uuid.UUID    `gorm:"column:user_id"`
	Role      string       `gorm:"column:role"`
	PartyID   *uuid.UUID   `gorm:"column:party_id"`
	CreatedAt time.Time    `gorm:"column:created_at"`
	User      pymesUserRow `gorm:"foreignKey:UserID;references:ID"`
}

func (pymesOrgMemberRow) TableName() string { return "org_members" }

type pymesAPIKeyRow struct {
	ID         uuid.UUID  `gorm:"column:id"`
	OrgID      uuid.UUID  `gorm:"column:org_id"`
	Name       string     `gorm:"column:name"`
	APIKeyHash string     `gorm:"column:api_key_hash"`
	KeyPrefix  string     `gorm:"column:key_prefix"`
	CreatedBy  *string    `gorm:"column:created_by"`
	RotatedAt  *time.Time `gorm:"column:rotated_at"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
}

func (pymesAPIKeyRow) TableName() string { return "org_api_keys" }

type pymesAPIKeyScopeRow struct {
	ID       uuid.UUID `gorm:"column:id"`
	APIKeyID uuid.UUID `gorm:"column:api_key_id"`
	Scope    string    `gorm:"column:scope"`
}

func (pymesAPIKeyScopeRow) TableName() string { return "org_api_key_scopes" }

type pymesTenantSettingsRow struct {
	OrgID                 uuid.UUID  `gorm:"column:org_id"`
	PlanCode              string     `gorm:"column:plan_code"`
	HardLimits            []byte     `gorm:"column:hard_limits"`
	HardLimitsJSON        []byte     `gorm:"column:hard_limits_json"`
	BillingStatus         string     `gorm:"column:billing_status"`
	StripeCustomerID      *string    `gorm:"column:stripe_customer_id"`
	StripeSubscriptionID  *string    `gorm:"column:stripe_subscription_id"`
	Status                string     `gorm:"column:status"`
	Vertical              string     `gorm:"column:vertical"`
	OnboardingCompletedAt *time.Time `gorm:"column:onboarding_completed_at"`
	DeletedAt             *time.Time `gorm:"column:deleted_at"`
	PastDueSince          *time.Time `gorm:"column:past_due_since"`
	UpdatedBy             *string    `gorm:"column:updated_by"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (pymesTenantSettingsRow) TableName() string { return "tenant_settings" }

type pymesUsageCounterRow struct {
	CounterName string `gorm:"column:counter_name"`
	Value       int64  `gorm:"column:value"`
	Period      string `gorm:"column:period"`
}

func (pymesUsageCounterRow) TableName() string { return "org_usage_counters" }
