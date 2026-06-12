package wire

import (
	"time"

	"github.com/google/uuid"
)

type pymesTenantRow struct {
	ID         uuid.UUID `gorm:"column:id"`
	ExternalID *string   `gorm:"column:external_id"`
	ClerkOrgID *string   `gorm:"column:clerk_org_id"`
	Name       string    `gorm:"column:name"`
	Slug       *string   `gorm:"column:slug"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (pymesTenantRow) TableName() string { return "orgs" }

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

type pymesTenantMembershipRow struct {
	ID        uuid.UUID    `gorm:"column:id"`
	OrgID  uuid.UUID    `gorm:"column:org_id"`
	UserID    uuid.UUID    `gorm:"column:user_id"`
	Role      string       `gorm:"column:role"`
	Status    string       `gorm:"column:status"`
	PartyID   *uuid.UUID   `gorm:"column:party_id"`
	RemovedAt *time.Time   `gorm:"column:removed_at"`
	CreatedAt time.Time    `gorm:"column:created_at"`
	UpdatedAt time.Time    `gorm:"column:updated_at"`
	User      pymesUserRow `gorm:"foreignKey:UserID;references:ID"`
}

func (pymesTenantMembershipRow) TableName() string { return "org_members" }

type pymesTenantAPIKeyRow struct {
	ID         uuid.UUID  `gorm:"column:id"`
	OrgID   uuid.UUID  `gorm:"column:org_id"`
	Name       string     `gorm:"column:name"`
	APIKeyHash string     `gorm:"column:api_key_hash"`
	KeyPrefix  string     `gorm:"column:key_prefix"`
	CreatedBy  *string    `gorm:"column:created_by"`
	RotatedAt  *time.Time `gorm:"column:rotated_at"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
}

func (pymesTenantAPIKeyRow) TableName() string { return "org_api_keys" }

type pymesTenantAPIKeyScopeRow struct {
	ID       uuid.UUID `gorm:"column:id"`
	APIKeyID uuid.UUID `gorm:"column:api_key_id"`
	Scope    string    `gorm:"column:scope"`
}

func (pymesTenantAPIKeyScopeRow) TableName() string { return "org_api_key_scopes" }

type pymesTenantSettingsRow struct {
	OrgID              uuid.UUID  `gorm:"column:org_id"`
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
	OrgID       uuid.UUID `gorm:"column:org_id"`
	CounterName string    `gorm:"column:counter"`
	Value       int64     `gorm:"column:value"`
	Period      time.Time `gorm:"column:period"`
}

func (pymesUsageCounterRow) TableName() string { return "org_usage_counters" }

type pymesTenantInvitationRow struct {
	ID                uuid.UUID    `gorm:"column:id"`
	OrgID          uuid.UUID    `gorm:"column:org_id"`
	EmailNormalized   string       `gorm:"column:email_normalized"`
	Role              string       `gorm:"column:role"`
	Status            string       `gorm:"column:status"`
	TokenHash         string       `gorm:"column:token_hash"`
	ClerkInvitationID *string      `gorm:"column:clerk_invitation_id"`
	InvitedByUserID   uuid.UUID    `gorm:"column:invited_by_user_id"`
	AcceptedByUserID  *uuid.UUID   `gorm:"column:accepted_by_user_id"`
	ExpiresAt         time.Time    `gorm:"column:expires_at"`
	AcceptedAt        *time.Time   `gorm:"column:accepted_at"`
	RevokedAt         *time.Time   `gorm:"column:revoked_at"`
	CreatedAt         time.Time    `gorm:"column:created_at"`
	UpdatedAt         time.Time    `gorm:"column:updated_at"`
	InvitedByUser     pymesUserRow `gorm:"foreignKey:InvitedByUserID;references:ID"`
	AcceptedByUser    pymesUserRow `gorm:"foreignKey:AcceptedByUserID;references:ID"`
}

func (pymesTenantInvitationRow) TableName() string { return "tenant_invitations" }
