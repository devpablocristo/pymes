package billing

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/billing/repository/models"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetTenantSettings(orgID uuid.UUID) TenantSettings {
	var m models.BillingSettingsModel
	if err := r.db.Where("org_id = ?", orgID).First(&m).Error; err != nil {
		return TenantSettings{
			OrgID:         orgID,
			PlanCode:      "starter",
			HardLimits:    defaultHardLimits("starter"),
			BillingStatus: "trialing",
			UpdatedAt:     time.Now().UTC(),
		}
	}
	return billingModelToDomain(m)
}

func (r *Repository) UpdateBilling(orgID uuid.UUID, plan, status, subscriptionID, customerID, actor string) TenantSettings {
	var m models.BillingSettingsModel
	result := r.db.Where("org_id = ?", orgID).First(&m)

	now := time.Now().UTC()

	if result.Error != nil {
		effectivePlan := plan
		if effectivePlan == "" {
			effectivePlan = "starter"
		}
		limitsJSON, _ := json.Marshal(defaultHardLimits(effectivePlan))
		m = models.BillingSettingsModel{
			OrgID:         orgID,
			PlanCode:      effectivePlan,
			HardLimits:    limitsJSON,
			BillingStatus: "trialing",
			CreatedAt:     now,
		}
	}

	if plan != "" {
		m.PlanCode = plan
	}
	if status != "" {
		m.BillingStatus = status
	}
	if subscriptionID != "" {
		m.StripeSubscriptionID = &subscriptionID
	} else if status == "canceled" {
		m.StripeSubscriptionID = nil
	}
	if customerID != "" {
		m.StripeCustomerID = &customerID
	}
	if actor != "" {
		m.UpdatedBy = &actor
	}
	m.UpdatedAt = now

	if result.Error != nil {
		r.db.Create(&m)
	} else {
		r.db.Save(&m)
	}

	return billingModelToDomain(m)
}

func (r *Repository) ResolveOrgByStripeIdentifiers(subscriptionID, customerID string) (uuid.UUID, bool) {
	var m models.BillingSettingsModel

	query := r.db.Model(&models.BillingSettingsModel{})
	if subscriptionID != "" && customerID != "" {
		query = query.Where("stripe_subscription_id = ? OR stripe_customer_id = ?", subscriptionID, customerID)
	} else if subscriptionID != "" {
		query = query.Where("stripe_subscription_id = ?", subscriptionID)
	} else if customerID != "" {
		query = query.Where("stripe_customer_id = ?", customerID)
	} else {
		return uuid.Nil, false
	}

	if err := query.First(&m).Error; err != nil {
		return uuid.Nil, false
	}
	return m.OrgID, true
}

func billingModelToDomain(m models.BillingSettingsModel) TenantSettings {
	var limits map[string]any
	if len(m.HardLimits) > 0 {
		_ = json.Unmarshal(m.HardLimits, &limits)
	}
	if limits == nil {
		limits = defaultHardLimits(m.PlanCode)
	}
	return TenantSettings{
		OrgID:                m.OrgID,
		PlanCode:             m.PlanCode,
		HardLimits:           limits,
		BillingStatus:        m.BillingStatus,
		UpdatedAt:            m.UpdatedAt,
		StripeCustomerID:     m.StripeCustomerID,
		StripeSubscriptionID: m.StripeSubscriptionID,
	}
}

func defaultHardLimits(plan string) map[string]any {
	switch strings.ToLower(plan) {
	case "growth":
		return map[string]any{"users_max": 25, "storage_mb": 5000, "api_calls_rpm": 500}
	case "enterprise":
		return map[string]any{"users_max": "unlimited", "storage_mb": 50000, "api_calls_rpm": 2000}
	default:
		return map[string]any{"users_max": 5, "storage_mb": 500, "api_calls_rpm": 100}
	}
}
