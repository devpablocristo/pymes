package domain

import (
	"time"

	"github.com/google/uuid"
)

type PlanCode string

const (
	PlanStarter    PlanCode = "starter"
	PlanGrowth     PlanCode = "growth"
	PlanEnterprise PlanCode = "enterprise"
)

type BillingStatus string

const (
	BillingTrialing BillingStatus = "trialing"
	BillingActive   BillingStatus = "active"
	BillingPastDue  BillingStatus = "past_due"
	BillingCanceled BillingStatus = "canceled"
)

type HardLimits struct {
	UsersMax    any `json:"users_max"`
	StorageMB   any `json:"storage_mb"`
	APICallsRPM any `json:"api_calls_rpm"`
}

type BillingSummary struct {
	OrgID            uuid.UUID      `json:"org_id"`
	PlanCode         PlanCode       `json:"plan_code"`
	Status           BillingStatus  `json:"status"`
	HardLimits       HardLimits     `json:"hard_limits"`
	Usage            map[string]any `json:"usage"`
	CurrentPeriodEnd time.Time      `json:"current_period_end"`
}
