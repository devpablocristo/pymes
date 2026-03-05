package models

import (
	"time"

	"github.com/google/uuid"
)

type BillingSettingsModel struct {
	OrgID                uuid.UUID `gorm:"type:uuid;primaryKey;column:org_id"`
	PlanCode             string    `gorm:"not null;default:starter"`
	HardLimits           []byte    `gorm:"type:jsonb"`
	StripeCustomerID     *string   `gorm:"uniqueIndex"`
	StripeSubscriptionID *string   `gorm:"uniqueIndex"`
	BillingStatus        string    `gorm:"not null;default:trialing"`
	UpdatedBy            *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (BillingSettingsModel) TableName() string { return "tenant_settings" }
