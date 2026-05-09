package wire

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	saasadmindomain "github.com/devpablocristo/core/saas/go/admin/usecases/domain"
	saasbillingdomain "github.com/devpablocristo/core/saas/go/billing/usecases/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *pymesSaaSStore) GetTenantBilling(ctx context.Context, orgID string) (saasbillingdomain.TenantBilling, bool, error) {
	row, ok, err := s.loadTenantSettings(ctx, orgID)
	if err != nil || !ok {
		return saasbillingdomain.TenantBilling{}, ok, err
	}
	return tenantBillingFromRow(row), true, nil
}

func (s *pymesSaaSStore) UpsertTenantBilling(ctx context.Context, item saasbillingdomain.TenantBilling) (saasbillingdomain.TenantBilling, error) {
	row, _, err := s.loadTenantSettings(ctx, item.TenantID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return saasbillingdomain.TenantBilling{}, err
	}
	tenantUUID, err := uuid.Parse(strings.TrimSpace(item.TenantID))
	if err != nil {
		return saasbillingdomain.TenantBilling{}, err
	}
	now := time.Now().UTC()
	payload, err := json.Marshal(item.HardLimits)
	if err != nil {
		return saasbillingdomain.TenantBilling{}, err
	}
	if row.OrgID == uuid.Nil {
		row.OrgID = tenantUUID
		row.CreatedAt = now
	}
	row.PlanCode = string(item.PlanCode)
	row.HardLimits = payload
	row.HardLimitsJSON = payload
	row.BillingStatus = string(item.BillingStatus)
	row.StripeCustomerID = item.ProviderCustomerID
	row.StripeSubscriptionID = item.ProviderContractID
	row.PastDueSince = item.PastDueSince
	if row.Status == "" {
		row.Status = "active"
	}
	row.UpdatedAt = now
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return saasbillingdomain.TenantBilling{}, err
	}
	return tenantBillingFromRow(row), nil
}

func (s *pymesSaaSStore) GetUsageSummary(ctx context.Context, orgID string) (saasbillingdomain.UsageSummary, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return saasbillingdomain.UsageSummary{}, err
	}
	var rows []pymesUsageCounterRow
	if err := s.db.WithContext(ctx).
		Where("org_id = ?", tenantUUID).
		Order("period DESC, counter_name ASC").
		Find(&rows).Error; err != nil {
		return saasbillingdomain.UsageSummary{}, err
	}
	summary := saasbillingdomain.UsageSummary{
		Period:   time.Now().UTC().Format("2006-01"),
		Counters: saasbillingdomain.UsageCounters{},
	}
	if len(rows) == 0 {
		return summary, nil
	}
	period := rows[0].Period
	summary.Period = period
	for _, row := range rows {
		if row.Period != period {
			continue
		}
		switch row.CounterName {
		case "api_calls":
			summary.Counters.APICalls = row.Value
		case "events_ingested":
			summary.Counters.EventsIngested = row.Value
		case "incidents_opened":
			summary.Counters.IncidentsOpened = row.Value
		case "actions_executed":
			summary.Counters.ActionsExecuted = row.Value
		}
	}
	return summary, nil
}

func (s *pymesSaaSStore) GetTenantName(ctx context.Context, orgID string) (string, error) {
	var row pymesTenantRow
	if err := s.db.WithContext(ctx).
		Where("id = ?", strings.TrimSpace(orgID)).
		Take(&row).Error; err != nil {
		return "", err
	}
	return row.Name, nil
}

func (s *pymesSaaSStore) FindTenantIDByCustomerID(ctx context.Context, customerID string) (string, bool, error) {
	return s.findTenantIDByColumn(ctx, "stripe_customer_id", customerID)
}

func (s *pymesSaaSStore) FindTenantIDByContractID(ctx context.Context, contractID string) (string, bool, error) {
	return s.findTenantIDByColumn(ctx, "stripe_subscription_id", contractID)
}

func (s *pymesSaaSStore) FindPastDueBefore(ctx context.Context, before time.Time) ([]saasbillingdomain.TenantBilling, error) {
	var rows []pymesTenantSettingsRow
	if err := s.db.WithContext(ctx).
		Where("billing_status = ? AND past_due_since IS NOT NULL AND past_due_since < ?", "past_due", before).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]saasbillingdomain.TenantBilling, 0, len(rows))
	for _, row := range rows {
		items = append(items, tenantBillingFromRow(row))
	}
	return items, nil
}

func (s *pymesSaaSStore) UpsertTenantSettings(ctx context.Context, item saasadmindomain.TenantSettings) (saasadmindomain.TenantSettings, error) {
	row, _, err := s.loadTenantSettings(ctx, item.TenantID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return saasadmindomain.TenantSettings{}, err
	}
	tenantUUID, err := uuid.Parse(strings.TrimSpace(item.TenantID))
	if err != nil {
		return saasadmindomain.TenantSettings{}, err
	}
	now := time.Now().UTC()
	payload, err := json.Marshal(item.HardLimits)
	if err != nil {
		return saasadmindomain.TenantSettings{}, err
	}
	if row.OrgID == uuid.Nil {
		row.OrgID = tenantUUID
		row.CreatedAt = now
	}
	row.PlanCode = strings.TrimSpace(item.PlanCode)
	row.Status = strings.TrimSpace(string(item.Status))
	row.DeletedAt = item.DeletedAt
	row.HardLimits = payload
	row.HardLimitsJSON = payload
	row.UpdatedBy = item.UpdatedBy
	row.UpdatedAt = now
	if row.BillingStatus == "" {
		row.BillingStatus = "trialing"
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return saasadmindomain.TenantSettings{}, err
	}
	return adminTenantSettingsFromRow(row), nil
}

func (s *pymesSaaSStore) ensureTenantSettings(ctx context.Context, orgID uuid.UUID) error {
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&pymesTenantSettingsRow{}).
		Where("org_id = ?", orgID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := time.Now().UTC()
	row := pymesTenantSettingsRow{
		OrgID:       orgID,
		PlanCode:       "starter",
		BillingStatus:  "trialing",
		Status:         "active",
		HardLimits:     mustJSONBytes(defaultSaaSHardLimits()),
		HardLimitsJSON: mustJSONBytes(defaultSaaSHardLimits()),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

func (s *pymesSaaSStore) loadTenantSettings(ctx context.Context, orgID string) (pymesTenantSettingsRow, bool, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return pymesTenantSettingsRow{}, false, err
	}
	var row pymesTenantSettingsRow
	err = s.db.WithContext(ctx).Where("org_id = ?", tenantUUID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pymesTenantSettingsRow{}, false, nil
	}
	if err != nil {
		return pymesTenantSettingsRow{}, false, err
	}
	return row, true, nil
}

func (s *pymesSaaSStore) findTenantIDByColumn(ctx context.Context, column, value string) (string, bool, error) {
	var row pymesTenantSettingsRow
	err := s.db.WithContext(ctx).
		Where(column+" = ?", strings.TrimSpace(value)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.OrgID.String(), true, nil
}
