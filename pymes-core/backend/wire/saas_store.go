package wire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	saasadmindomain "github.com/devpablocristo/core/saas/go/admin/usecases/domain"
	saasbillingdomain "github.com/devpablocristo/core/saas/go/billing/usecases/domain"
	saasorgdomain "github.com/devpablocristo/core/saas/go/org/usecases/domain"
	saasuserdomain "github.com/devpablocristo/core/saas/go/users/usecases/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrUserProfileNotFound indica que no hay fila users para el external_id dado.
var ErrUserProfileNotFound = errors.New("user profile not found")

type pymesSaaSStore struct {
	db               *gorm.DB
	logger           *slog.Logger
	defaultKeyScopes []string
}

func newPymesSaaSStore(db *gorm.DB, logger *slog.Logger, defaultKeyScopes []string) *pymesSaaSStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &pymesSaaSStore{
		db:               db,
		logger:           logger,
		defaultKeyScopes: append([]string(nil), defaultKeyScopes...),
	}
}

// GetOrgNameByOrgUUID devuelve el nombre legible de `orgs` para el UUID interno (tenant_id / org_id del token).
func (s *pymesSaaSStore) GetOrgNameByOrgUUID(ctx context.Context, orgID string) (string, bool, error) {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return "", false, nil
	}
	id, err := uuid.Parse(orgID)
	if err != nil {
		return "", false, nil
	}
	var row pymesOrgRow
	err = s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	name := strings.TrimSpace(row.Name)
	if name == "" {
		return "", false, nil
	}
	return name, true, nil
}

func (s *pymesSaaSStore) FindOrgIDByExternalID(ctx context.Context, externalID string) (string, bool, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return "", false, nil
	}
	// Token con UUID interno (p. ej. entornos que emiten tenant como id de fila).
	if id, err := uuid.Parse(externalID); err == nil {
		var row pymesOrgRow
		err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		return row.ID.String(), true, nil
	}
	var row pymesOrgRow
	err := s.db.WithContext(ctx).
		Where("external_id = ?", externalID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Clerk emite org_id tipo org_...; sin webhook aún no hay fila. Creamos org mínima (JWT ya verificado).
		if strings.HasPrefix(externalID, "org_") {
			orgID, upErr := s.UpsertOrg(ctx, externalID, "Organization")
			if upErr != nil {
				return "", false, upErr
			}
			return orgID, true, nil
		}
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.ID.String(), true, nil
}

func (s *pymesSaaSStore) FindPrincipalByAPIKeyHash(ctx context.Context, apiKeyHash string) (saasorgdomain.Principal, string, error) {
	var key pymesAPIKeyRow
	err := s.db.WithContext(ctx).
		Where("api_key_hash = ?", strings.TrimSpace(apiKeyHash)).
		Take(&key).Error
	if err != nil {
		return saasorgdomain.Principal{}, "", err
	}
	scopes, err := s.loadKeyScopes(ctx, key.ID)
	if err != nil {
		return saasorgdomain.Principal{}, "", err
	}
	return saasorgdomain.Principal{
		TenantID: key.OrgID.String(),
		Scopes:   scopes,
	}, key.ID.String(), nil
}

const maxUserProfileNameLen = 200
const maxUserProfileGivenNameLen = 100
const maxUserProfileFamilyNameLen = 100
const maxUserProfilePhoneLen = 40

func splitFullNameIntoParts(full string) (given, family string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	idx := strings.IndexByte(full, ' ')
	if idx < 0 {
		return full, ""
	}
	return strings.TrimSpace(full[:idx]), strings.TrimSpace(full[idx+1:])
}

func joinDisplayName(given, family string) string {
	given = strings.TrimSpace(given)
	family = strings.TrimSpace(family)
	if family == "" {
		return given
	}
	if given == "" {
		return family
	}
	return given + " " + family
}

// GetUserProfileExtrasByExternalID devuelve campos de perfil extendidos (Pymes) para enriquecer GET /users/me.
func (s *pymesSaaSStore) GetUserProfileExtrasByExternalID(ctx context.Context, externalID string) (phone, givenName, familyName string, ok bool, err error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return "", "", "", false, nil
	}
	var row struct {
		Phone      string
		GivenName  string `gorm:"column:given_name"`
		FamilyName string `gorm:"column:family_name"`
	}
	e := s.db.WithContext(ctx).
		Table("users").
		Select("phone", "given_name", "family_name").
		Where("external_id = ?", externalID).
		Take(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return "", "", "", false, nil
	}
	if e != nil {
		return "", "", "", false, e
	}
	return strings.TrimSpace(row.Phone), strings.TrimSpace(row.GivenName), strings.TrimSpace(row.FamilyName), true, nil
}

// PatchUserPersonalFromRequest actualiza nombre (given/family o name legado) y/o teléfono.
func (s *pymesSaaSStore) PatchUserPersonalFromRequest(ctx context.Context, externalID string, req *PatchMeProfileRequest) error {
	if req == nil {
		return fmt.Errorf("request required")
	}
	hasName := req.Name != nil || req.GivenName != nil || req.FamilyName != nil
	if !hasName && req.Phone == nil {
		return fmt.Errorf("no fields to update")
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return fmt.Errorf("external_id required")
	}
	var row pymesUserRow
	err := s.db.WithContext(ctx).Where("external_id = ?", externalID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUserProfileNotFound
	}
	if err != nil {
		return err
	}
	updates := map[string]any{}
	if req.GivenName != nil || req.FamilyName != nil {
		g := strings.TrimSpace(row.GivenName)
		if req.GivenName != nil {
			g = strings.TrimSpace(*req.GivenName)
		}
		f := strings.TrimSpace(row.FamilyName)
		if req.FamilyName != nil {
			f = strings.TrimSpace(*req.FamilyName)
		}
		if len([]rune(g)) > maxUserProfileGivenNameLen {
			return fmt.Errorf("given name too long")
		}
		if len([]rune(f)) > maxUserProfileFamilyNameLen {
			return fmt.Errorf("family name too long")
		}
		combined := joinDisplayName(g, f)
		if combined == "" {
			return fmt.Errorf("name cannot be empty")
		}
		if len([]rune(combined)) > maxUserProfileNameLen {
			return fmt.Errorf("name too long")
		}
		updates["given_name"] = g
		updates["family_name"] = f
		updates["name"] = combined
	} else if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			return fmt.Errorf("name cannot be empty")
		}
		if len([]rune(n)) > maxUserProfileNameLen {
			return fmt.Errorf("name too long")
		}
		g, f := splitFullNameIntoParts(n)
		updates["given_name"] = g
		updates["family_name"] = f
		updates["name"] = joinDisplayName(g, f)
	}
	if req.Phone != nil {
		p := strings.TrimSpace(*req.Phone)
		if len([]rune(p)) > maxUserProfilePhoneLen {
			return fmt.Errorf("phone too long")
		}
		updates["phone"] = p
	}
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now().UTC()
	return s.db.WithContext(ctx).Model(&pymesUserRow{}).Where("id = ?", row.ID).Updates(updates).Error
}

func (s *pymesSaaSStore) FindUserByExternalID(ctx context.Context, externalID string) (saasuserdomain.User, bool, error) {
	var row pymesUserRow
	err := s.db.WithContext(ctx).
		Where("external_id = ?", strings.TrimSpace(externalID)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return saasuserdomain.User{}, false, nil
	}
	if err != nil {
		return saasuserdomain.User{}, false, err
	}
	return userDomainFromRow(row), true, nil
}

func (s *pymesSaaSStore) UpsertUser(ctx context.Context, externalID, email, name string, avatarURL *string) (saasuserdomain.User, error) {
	externalID = strings.TrimSpace(externalID)
	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)
	var row pymesUserRow
	err := s.db.WithContext(ctx).Where("external_id = ?", externalID).Take(&row).Error
	now := time.Now().UTC()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		given, family := splitFullNameIntoParts(name)
		row = pymesUserRow{
			ID:         uuid.New(),
			ExternalID: externalID,
			Email:      email,
			Name:       joinDisplayName(given, family),
			GivenName:  given,
			FamilyName: family,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if avatarURL != nil {
			row.AvatarURL = strings.TrimSpace(*avatarURL)
		}
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return saasuserdomain.User{}, err
		}
		return userDomainFromRow(row), nil
	}
	if err != nil {
		return saasuserdomain.User{}, err
	}
	row.Email = email
	given, family := splitFullNameIntoParts(name)
	row.GivenName = given
	row.FamilyName = family
	row.Name = joinDisplayName(given, family)
	if avatarURL != nil {
		row.AvatarURL = strings.TrimSpace(*avatarURL)
	}
	row.UpdatedAt = now
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return saasuserdomain.User{}, err
	}
	return userDomainFromRow(row), nil
}

func (s *pymesSaaSStore) SyncUser(ctx context.Context, externalID, email, name string, avatarURL *string) (saasuserdomain.User, error) {
	return s.UpsertUser(ctx, externalID, email, name, avatarURL)
}

func (s *pymesSaaSStore) UpsertOrg(ctx context.Context, externalID, orgName string) (string, error) {
	externalID = strings.TrimSpace(externalID)
	orgName = strings.TrimSpace(orgName)
	var row pymesOrgRow
	err := s.db.WithContext(ctx).Where("external_id = ?", externalID).Take(&row).Error
	now := time.Now().UTC()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = pymesOrgRow{
			ID:         uuid.New(),
			Name:       orgName,
			CreatedAt:  now,
			UpdatedAt:  now,
			ExternalID: stringPtr(externalID),
		}
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return "", err
		}
		if err := s.ensureTenantSettings(ctx, row.ID); err != nil {
			return "", err
		}
		return row.ID.String(), nil
	}
	if err != nil {
		return "", err
	}
	row.Name = orgName
	row.UpdatedAt = now
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return "", err
	}
	if err := s.ensureTenantSettings(ctx, row.ID); err != nil {
		return "", err
	}
	return row.ID.String(), nil
}

func (s *pymesSaaSStore) SyncOrganization(ctx context.Context, orgExternalID, orgName string) (string, error) {
	return s.UpsertOrg(ctx, orgExternalID, orgName)
}

func (s *pymesSaaSStore) UpsertOrgMember(ctx context.Context, orgID, userID, role string) (saasuserdomain.OrgMember, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	userUUID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	var row pymesOrgMemberRow
	tx := s.db.WithContext(ctx)
	err = tx.Where("org_id = ? AND user_id = ?", orgUUID, userUUID).Preload("User").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = pymesOrgMemberRow{
			ID:        uuid.New(),
			OrgID:     orgUUID,
			UserID:    userUUID,
			Role:      strings.TrimSpace(role),
			CreatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&row).Error; err != nil {
			return saasuserdomain.OrgMember{}, err
		}
		if err := tx.Preload("User").Where("id = ?", row.ID).Take(&row).Error; err != nil {
			return saasuserdomain.OrgMember{}, err
		}
		return memberDomainFromRow(row), nil
	}
	if err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	row.Role = strings.TrimSpace(role)
	if err := tx.Model(&pymesOrgMemberRow{}).Where("id = ?", row.ID).Update("role", row.Role).Error; err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	row.Role = strings.TrimSpace(role)
	return memberDomainFromRow(row), nil
}

func (s *pymesSaaSStore) SyncMembership(ctx context.Context, orgID, userExternalID, email, name string, avatarURL *string, role string) (saasuserdomain.OrgMember, error) {
	user, err := s.UpsertUser(ctx, userExternalID, email, name, avatarURL)
	if err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	return s.UpsertOrgMember(ctx, orgID, user.ID, role)
}

func (s *pymesSaaSStore) ListOrgMembers(ctx context.Context, orgID string) ([]saasuserdomain.OrgMember, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return nil, err
	}
	var rows []pymesOrgMemberRow
	if err := s.db.WithContext(ctx).
		Where("org_id = ?", orgUUID).
		Preload("User").
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]saasuserdomain.OrgMember, 0, len(rows))
	for _, row := range rows {
		items = append(items, memberDomainFromRow(row))
	}
	return items, nil
}

func (s *pymesSaaSStore) ListAPIKeys(ctx context.Context, orgID string) ([]saasuserdomain.APIKey, error) {
	rows, err := s.listAPIKeyRows(ctx, orgID)
	if err != nil {
		return nil, err
	}
	items := make([]saasuserdomain.APIKey, 0, len(rows))
	for _, row := range rows {
		scopes, err := s.loadKeyScopes(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, saasuserdomain.APIKey{
			ID:        row.ID.String(),
			OrgID:     row.OrgID.String(),
			Name:      row.Name,
			Scopes:    scopes,
			CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}

func (s *pymesSaaSStore) CreateAPIKey(ctx context.Context, orgID, name string, scopes []string) (saasuserdomain.CreatedAPIKey, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return saasuserdomain.CreatedAPIKey{}, err
	}
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return saasuserdomain.CreatedAPIKey{}, err
	}
	key := pymesAPIKeyRow{
		ID:         uuid.New(),
		OrgID:      orgUUID,
		Name:       strings.TrimSpace(name),
		APIKeyHash: keyHash,
		KeyPrefix:  keyPrefix,
		CreatedAt:  time.Now().UTC(),
	}
	if key.Name == "" {
		key.Name = "api-key"
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&key).Error; err != nil {
			return err
		}
		return s.replaceKeyScopesTx(ctx, tx, key.ID, normalizeScopes(scopes, s.defaultKeyScopes))
	}); err != nil {
		return saasuserdomain.CreatedAPIKey{}, err
	}
	keyScopes, err := s.loadKeyScopes(ctx, key.ID)
	if err != nil {
		return saasuserdomain.CreatedAPIKey{}, err
	}
	return saasuserdomain.CreatedAPIKey{
		APIKey: saasuserdomain.APIKey{
			ID:        key.ID.String(),
			OrgID:     key.OrgID.String(),
			Name:      key.Name,
			Scopes:    keyScopes,
			CreatedAt: key.CreatedAt,
		},
		Secret: rawKey,
	}, nil
}

func (s *pymesSaaSStore) DeleteAPIKey(ctx context.Context, orgID, keyID string) error {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return err
	}
	keyUUID, err := uuid.Parse(strings.TrimSpace(keyID))
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).
		Where("id = ? AND org_id = ?", keyUUID, orgUUID).
		Delete(&pymesAPIKeyRow{}).Error
}

func (s *pymesSaaSStore) RotateAPIKey(ctx context.Context, orgID, keyID string) (saasuserdomain.RotatedAPIKey, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	keyUUID, err := uuid.Parse(strings.TrimSpace(keyID))
	if err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	var row pymesAPIKeyRow
	if err := s.db.WithContext(ctx).
		Where("id = ? AND org_id = ?", keyUUID, orgUUID).
		Take(&row).Error; err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	now := time.Now().UTC()
	row.APIKeyHash = keyHash
	row.KeyPrefix = keyPrefix
	row.RotatedAt = &now
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	scopes, err := s.loadKeyScopes(ctx, row.ID)
	if err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	return saasuserdomain.RotatedAPIKey{
		APIKey: saasuserdomain.APIKey{
			ID:        row.ID.String(),
			OrgID:     row.OrgID.String(),
			Name:      row.Name,
			Scopes:    scopes,
			CreatedAt: row.CreatedAt,
		},
		Secret: rawKey,
	}, nil
}

func (s *pymesSaaSStore) SoftDeleteUser(ctx context.Context, externalID string) error {
	return s.db.WithContext(ctx).
		Model(&pymesUserRow{}).
		Where("external_id = ?", strings.TrimSpace(externalID)).
		Update("deleted_at", time.Now().UTC()).Error
}

func (s *pymesSaaSStore) RemoveMembership(ctx context.Context, userExternalID, orgExternalID, orgName string) error {
	var user pymesUserRow
	if err := s.db.WithContext(ctx).
		Where("external_id = ?", strings.TrimSpace(userExternalID)).
		Take(&user).Error; err != nil {
		return err
	}
	query := s.db.WithContext(ctx).Table("org_members AS om").
		Joins("JOIN orgs o ON o.id = om.org_id").
		Where("om.user_id = ?", user.ID)
	if value := strings.TrimSpace(orgExternalID); value != "" {
		query = query.Where("o.external_id = ?", value)
	} else if value := strings.TrimSpace(orgName); value != "" {
		query = query.Where("o.name = ?", value)
	}
	return query.Delete(&pymesOrgMemberRow{}).Error
}

func (s *pymesSaaSStore) GetTenantBilling(ctx context.Context, tenantID string) (saasbillingdomain.TenantBilling, bool, error) {
	row, ok, err := s.loadTenantSettings(ctx, tenantID)
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
	orgUUID, err := uuid.Parse(strings.TrimSpace(item.TenantID))
	if err != nil {
		return saasbillingdomain.TenantBilling{}, err
	}
	now := time.Now().UTC()
	payload, err := json.Marshal(item.HardLimits)
	if err != nil {
		return saasbillingdomain.TenantBilling{}, err
	}
	if row.OrgID == uuid.Nil {
		row.OrgID = orgUUID
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

func (s *pymesSaaSStore) GetUsageSummary(ctx context.Context, tenantID string) (saasbillingdomain.UsageSummary, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return saasbillingdomain.UsageSummary{}, err
	}
	var rows []pymesUsageCounterRow
	if err := s.db.WithContext(ctx).
		Where("org_id = ?", orgUUID).
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

func (s *pymesSaaSStore) GetTenantName(ctx context.Context, tenantID string) (string, error) {
	var row pymesOrgRow
	if err := s.db.WithContext(ctx).
		Where("id = ?", strings.TrimSpace(tenantID)).
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

func (s *pymesSaaSStore) FindUserEmailByExternalID(ctx context.Context, externalID string) (string, bool, error) {
	var row pymesUserRow
	err := s.db.WithContext(ctx).
		Where("external_id = ?", strings.TrimSpace(externalID)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.Email, true, nil
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
	orgUUID, err := uuid.Parse(strings.TrimSpace(item.TenantID))
	if err != nil {
		return saasadmindomain.TenantSettings{}, err
	}
	now := time.Now().UTC()
	payload, err := json.Marshal(item.HardLimits)
	if err != nil {
		return saasadmindomain.TenantSettings{}, err
	}
	if row.OrgID == uuid.Nil {
		row.OrgID = orgUUID
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

func (s *pymesSaaSStore) CreateOrgWithDefaultKey(ctx context.Context, name, slug, actor string) (string, string, pymesAPIKeyRow, []string, error) {
	now := time.Now().UTC()
	org := pymesOrgRow{
		ID:         uuid.New(),
		Name:       strings.TrimSpace(name),
		CreatedAt:  now,
		UpdatedAt:  now,
		ExternalID: stringPtr(strings.TrimSpace(slug)),
		Slug:       stringPtr(strings.TrimSpace(slug)),
	}
	if org.Name == "" {
		return "", "", pymesAPIKeyRow{}, nil, fmt.Errorf("name is required")
	}
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return "", "", pymesAPIKeyRow{}, nil, err
	}
	key := pymesAPIKeyRow{
		ID:         uuid.New(),
		OrgID:      org.ID,
		Name:       "default",
		APIKeyHash: keyHash,
		KeyPrefix:  keyPrefix,
		CreatedAt:  now,
		CreatedBy:  stringPtr(strings.TrimSpace(actor)),
	}
	scopes := normalizeScopes(nil, s.defaultKeyScopes)
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&org).Error; err != nil {
			return err
		}
		settings := pymesTenantSettingsRow{
			OrgID:          org.ID,
			PlanCode:       "starter",
			BillingStatus:  "trialing",
			Status:         "active",
			HardLimits:     mustJSONBytes(defaultSaaSHardLimits()),
			HardLimitsJSON: mustJSONBytes(defaultSaaSHardLimits()),
			CreatedAt:      now,
			UpdatedAt:      now,
			UpdatedBy:      stringPtr(strings.TrimSpace(actor)),
		}
		if err := tx.Save(&settings).Error; err != nil {
			return err
		}
		if err := tx.Create(&key).Error; err != nil {
			return err
		}
		return s.replaceKeyScopesTx(ctx, tx, key.ID, scopes)
	}); err != nil {
		return "", "", pymesAPIKeyRow{}, nil, err
	}
	return org.ID.String(), rawKey, key, scopes, nil
}

func (s *pymesSaaSStore) listAPIKeyRows(ctx context.Context, orgID string) ([]pymesAPIKeyRow, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return nil, err
	}
	var rows []pymesAPIKeyRow
	if err := s.db.WithContext(ctx).
		Where("org_id = ?", orgUUID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *pymesSaaSStore) loadKeyScopes(ctx context.Context, keyID uuid.UUID) ([]string, error) {
	var scopes []string
	if err := s.db.WithContext(ctx).
		Table("org_api_key_scopes").
		Where("api_key_id = ?", keyID).
		Order("scope ASC").
		Pluck("scope", &scopes).Error; err != nil {
		return nil, err
	}
	sort.Strings(scopes)
	return scopes, nil
}

func (s *pymesSaaSStore) replaceKeyScopesTx(_ context.Context, tx *gorm.DB, keyID uuid.UUID, scopes []string) error {
	if err := tx.Where("api_key_id = ?", keyID).Delete(&pymesAPIKeyScopeRow{}).Error; err != nil {
		return err
	}
	for _, scope := range scopes {
		row := pymesAPIKeyScopeRow{
			ID:       uuid.New(),
			APIKeyID: keyID,
			Scope:    scope,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
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
		OrgID:          orgID,
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

func (s *pymesSaaSStore) loadTenantSettings(ctx context.Context, tenantID string) (pymesTenantSettingsRow, bool, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return pymesTenantSettingsRow{}, false, err
	}
	var row pymesTenantSettingsRow
	err = s.db.WithContext(ctx).Where("org_id = ?", orgUUID).Take(&row).Error
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
