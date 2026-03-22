package wire

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	utils "github.com/devpablocristo/core/backend/go/hashutil"
	saasadmindomain "github.com/devpablocristo/core/saas/go/admin/usecases/domain"
	saasbillingdomain "github.com/devpablocristo/core/saas/go/billing/usecases/domain"
	saasuserdomain "github.com/devpablocristo/core/saas/go/users/usecases/domain"
)

func userDomainFromRow(row pymesUserRow) saasuserdomain.User {
	var avatarURL *string
	if strings.TrimSpace(row.AvatarURL) != "" {
		value := strings.TrimSpace(row.AvatarURL)
		avatarURL = &value
	}
	return saasuserdomain.User{
		ID:         row.ID.String(),
		ExternalID: row.ExternalID,
		Email:      row.Email,
		Name:       row.Name,
		AvatarURL:  avatarURL,
		DeletedAt:  row.DeletedAt,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}

func memberDomainFromRow(row pymesOrgMemberRow) saasuserdomain.OrgMember {
	return saasuserdomain.OrgMember{
		ID:       row.ID.String(),
		OrgID:    row.OrgID.String(),
		UserID:   row.UserID.String(),
		Role:     row.Role,
		JoinedAt: row.CreatedAt,
		User:     userDomainFromRow(row.User),
	}
}

func tenantBillingFromRow(row pymesTenantSettingsRow) saasbillingdomain.TenantBilling {
	return saasbillingdomain.TenantBilling{
		TenantID:           row.OrgID.String(),
		PlanCode:           saasbillingdomain.PlanCode(strings.TrimSpace(row.PlanCode)),
		HardLimits:         parseHardLimits(row.HardLimitsJSON, row.HardLimits),
		BillingStatus:      saasbillingdomain.BillingStatus(strings.TrimSpace(row.BillingStatus)),
		PastDueSince:       row.PastDueSince,
		ProviderCustomerID: row.StripeCustomerID,
		ProviderContractID: row.StripeSubscriptionID,
		UpdatedAt:          row.UpdatedAt,
		CreatedAt:          row.CreatedAt,
	}
}

func adminTenantSettingsFromRow(row pymesTenantSettingsRow) saasadmindomain.TenantSettings {
	return saasadmindomain.TenantSettings{
		TenantID:   row.OrgID.String(),
		PlanCode:   row.PlanCode,
		Status:     saasadmindomain.TenantStatus(strings.TrimSpace(row.Status)),
		DeletedAt:  row.DeletedAt,
		HardLimits: parseHardLimitsMap(row.HardLimitsJSON, row.HardLimits),
		UpdatedBy:  row.UpdatedBy,
		UpdatedAt:  row.UpdatedAt,
		CreatedAt:  row.CreatedAt,
	}
}

func parseHardLimits(primary, fallback []byte) saasbillingdomain.HardLimits {
	values := parseHardLimitsMap(primary, fallback)
	return saasbillingdomain.HardLimits{
		ToolsMax:           intFromAny(values["tools_max"]),
		RunRPM:             intFromAny(values["run_rpm"]),
		AuditRetentionDays: intFromAny(values["audit_retention_days"]),
	}
}

func parseHardLimitsMap(primary, fallback []byte) map[string]any {
	var values map[string]any
	for _, payload := range [][]byte{primary, fallback} {
		if len(payload) == 0 {
			continue
		}
		if err := json.Unmarshal(payload, &values); err == nil && len(values) > 0 {
			return values
		}
	}
	return defaultSaaSHardLimits()
}

func defaultSaaSHardLimits() map[string]any {
	return map[string]any{
		"tools_max":            10,
		"run_rpm":              30,
		"audit_retention_days": 30,
	}
}

func normalizeScopes(scopes, defaults []string) []string {
	if len(scopes) == 0 {
		scopes = defaults
	}
	out := make([]string, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	sort.Strings(out)
	return out
}

func generateAPIKey() (string, string, string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", err
	}
	raw := "psk_" + hex.EncodeToString(buf)
	prefix := raw
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return raw, prefix, utils.SHA256Hex(raw), nil
}

func mustJSONBytes(values map[string]any) []byte {
	payload, err := json.Marshal(values)
	if err != nil {
		return []byte("{}")
	}
	return payload
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}
