package wire

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrUserProfileNotFound indica que no hay fila users para el external_id dado.
var ErrUserProfileNotFound = errors.New("user profile not found")

const maxUserProfileNameLen = 200
const maxUserProfileGivenNameLen = 100
const maxUserProfileFamilyNameLen = 100
const maxUserProfilePhoneLen = 40

type orgUserCreateRequest struct {
	ExternalID string  `json:"external_id"`
	Email      string  `json:"email"`
	Name       string  `json:"name"`
	GivenName  string  `json:"given_name"`
	FamilyName string  `json:"family_name"`
	Phone      string  `json:"phone"`
	AvatarURL  *string `json:"avatar_url"`
	Role       string  `json:"role"`
}

type orgUserUpdateRequest struct {
	ExternalID *string `json:"external_id"`
	Email      *string `json:"email"`
	Name       *string `json:"name"`
	GivenName  *string `json:"given_name"`
	FamilyName *string `json:"family_name"`
	Phone      *string `json:"phone"`
	AvatarURL  *string `json:"avatar_url"`
	Status     *string `json:"status"`
}

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

func manualExternalIDForEmail(email string) string {
	email = normalizeEmail(email)
	if email == "" {
		return ""
	}
	return "manual:" + email
}

func normalizeOrgUserStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active":
		return "active", nil
	case "archived", "disabled":
		return "archived", nil
	default:
		return "", domainerr.Validation("invalid user status")
	}
}

// GetUserProfileExtrasByExternalID devuelve campos de perfil extendidos (Pymes) para enriquecer GET /users/me.
type userProfileExtrasRow struct {
	Phone      string
	GivenName  string `gorm:"column:given_name"`
	FamilyName string `gorm:"column:family_name"`
}

func (s *pymesSaaSStore) GetUserProfileExtrasByExternalID(ctx context.Context, externalID string) (phone, givenName, familyName string, ok bool, err error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return "", "", "", false, nil
	}
	var row userProfileExtrasRow
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

func (s *pymesSaaSStore) FindUserByExternalID(ctx context.Context, externalID string) (tenantUserDTO, bool, error) {
	var row pymesUserRow
	err := s.db.WithContext(ctx).
		Where("external_id = ?", strings.TrimSpace(externalID)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tenantUserDTO{}, false, nil
	}
	if err != nil {
		return tenantUserDTO{}, false, err
	}
	return userDTOFromRow(row), true, nil
}

func (s *pymesSaaSStore) ListOrgUsers(ctx context.Context, orgID string) ([]tenantUserDTO, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return nil, domainerr.Validation("invalid org_id")
	}
	var rows []pymesUserRow
	if err := s.db.WithContext(ctx).
		Table("users AS u").
		Select("u.*").
		Joins("JOIN org_members om ON om.user_id = u.id AND om.status = 'active'").
		Where("om.org_id = ?", tenantUUID).
		Order("u.created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]tenantUserDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, userDTOFromRow(row))
	}
	return items, nil
}

func (s *pymesSaaSStore) CreateOrgUser(ctx context.Context, orgID string, req orgUserCreateRequest) (tenantUserDTO, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return tenantUserDTO{}, domainerr.Validation("invalid org_id")
	}
	email := normalizeEmail(req.Email)
	if email == "" {
		return tenantUserDTO{}, domainerr.Validation("email is required")
	}
	externalID := strings.TrimSpace(req.ExternalID)
	if externalID == "" {
		externalID = manualExternalIDForEmail(email)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = joinDisplayName(req.GivenName, req.FamilyName)
	}
	if name == "" {
		name = email
	}
	if len([]rune(name)) > maxUserProfileNameLen {
		return tenantUserDTO{}, domainerr.Validation("name too long")
	}
	row, err := s.upsertUserTx(ctx, s.db.WithContext(ctx), externalID, email, name, req.AvatarURL)
	if err != nil {
		return tenantUserDTO{}, err
	}
	updates := map[string]any{}
	if phone := strings.TrimSpace(req.Phone); phone != "" {
		if len([]rune(phone)) > maxUserProfilePhoneLen {
			return tenantUserDTO{}, domainerr.Validation("phone too long")
		}
		updates["phone"] = phone
	}
	if strings.TrimSpace(req.GivenName) != "" || strings.TrimSpace(req.FamilyName) != "" {
		given := strings.TrimSpace(req.GivenName)
		family := strings.TrimSpace(req.FamilyName)
		if len([]rune(given)) > maxUserProfileGivenNameLen {
			return tenantUserDTO{}, domainerr.Validation("given name too long")
		}
		if len([]rune(family)) > maxUserProfileFamilyNameLen {
			return tenantUserDTO{}, domainerr.Validation("family name too long")
		}
		combined := joinDisplayName(given, family)
		if combined == "" {
			combined = name
		}
		updates["given_name"] = given
		updates["family_name"] = family
		updates["name"] = combined
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now().UTC()
		if err := s.db.WithContext(ctx).Model(&pymesUserRow{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return tenantUserDTO{}, err
		}
		if err := s.db.WithContext(ctx).Where("id = ?", row.ID).Take(&row).Error; err != nil {
			return tenantUserDTO{}, err
		}
	}
	if _, err := s.UpsertTenantMember(ctx, tenantUUID.String(), row.ID.String(), normalizeInviteRole(req.Role)); err != nil {
		return tenantUserDTO{}, err
	}
	return userDTOFromRow(row), nil
}

func (s *pymesSaaSStore) UpdateOrgUser(ctx context.Context, userID string, req orgUserUpdateRequest) (tenantUserDTO, error) {
	userUUID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return tenantUserDTO{}, domainerr.Validation("invalid user_id")
	}
	var row pymesUserRow
	if err := s.db.WithContext(ctx).Where("id = ?", userUUID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tenantUserDTO{}, domainerr.NotFound("user not found")
		}
		return tenantUserDTO{}, err
	}
	updates := map[string]any{}
	if req.ExternalID != nil {
		externalID := strings.TrimSpace(*req.ExternalID)
		if externalID == "" {
			return tenantUserDTO{}, domainerr.Validation("external_id cannot be empty")
		}
		updates["external_id"] = externalID
	}
	if req.Email != nil {
		email := normalizeEmail(*req.Email)
		if email == "" {
			return tenantUserDTO{}, domainerr.Validation("email cannot be empty")
		}
		updates["email"] = email
	}
	if req.GivenName != nil || req.FamilyName != nil {
		given := strings.TrimSpace(row.GivenName)
		if req.GivenName != nil {
			given = strings.TrimSpace(*req.GivenName)
		}
		family := strings.TrimSpace(row.FamilyName)
		if req.FamilyName != nil {
			family = strings.TrimSpace(*req.FamilyName)
		}
		if len([]rune(given)) > maxUserProfileGivenNameLen {
			return tenantUserDTO{}, domainerr.Validation("given name too long")
		}
		if len([]rune(family)) > maxUserProfileFamilyNameLen {
			return tenantUserDTO{}, domainerr.Validation("family name too long")
		}
		combined := joinDisplayName(given, family)
		if combined == "" {
			return tenantUserDTO{}, domainerr.Validation("name cannot be empty")
		}
		updates["given_name"] = given
		updates["family_name"] = family
		updates["name"] = combined
	} else if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return tenantUserDTO{}, domainerr.Validation("name cannot be empty")
		}
		if len([]rune(name)) > maxUserProfileNameLen {
			return tenantUserDTO{}, domainerr.Validation("name too long")
		}
		given, family := splitFullNameIntoParts(name)
		updates["given_name"] = given
		updates["family_name"] = family
		updates["name"] = joinDisplayName(given, family)
	}
	if req.Phone != nil {
		phone := strings.TrimSpace(*req.Phone)
		if len([]rune(phone)) > maxUserProfilePhoneLen {
			return tenantUserDTO{}, domainerr.Validation("phone too long")
		}
		updates["phone"] = phone
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = strings.TrimSpace(*req.AvatarURL)
	}
	if req.Status != nil {
		status, err := normalizeOrgUserStatus(*req.Status)
		if err != nil {
			return tenantUserDTO{}, err
		}
		if status == "active" {
			updates["deleted_at"] = nil
		} else {
			now := time.Now().UTC()
			updates["deleted_at"] = &now
		}
	}
	if len(updates) == 0 {
		return userDTOFromRow(row), nil
	}
	updates["updated_at"] = time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&pymesUserRow{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return tenantUserDTO{}, err
	}
	if err := s.db.WithContext(ctx).Where("id = ?", row.ID).Take(&row).Error; err != nil {
		return tenantUserDTO{}, err
	}
	return userDTOFromRow(row), nil
}

func (s *pymesSaaSStore) UpsertUser(ctx context.Context, externalID, email, name string, avatarURL *string) (tenantUserDTO, error) {
	row, err := s.upsertUserTx(ctx, s.db.WithContext(ctx), externalID, email, name, avatarURL)
	if err != nil {
		return tenantUserDTO{}, err
	}
	return userDTOFromRow(row), nil
}

func (s *pymesSaaSStore) enrichAuthenticatedClerkUser(ctx context.Context, user clerkAuthenticatedUser) clerkAuthenticatedUser {
	user.ExternalID = strings.TrimSpace(user.ExternalID)
	if user.ExternalID == "" || s == nil || s.clerk == nil {
		return user
	}
	needsClerk := normalizeEmail(user.Email) == "" ||
		isPlaceholderClerkEmail(user.Email) ||
		isSyntheticClerkName(user.Name, user.ExternalID) ||
		user.AvatarURL == nil ||
		strings.TrimSpace(*user.AvatarURL) == ""
	if !needsClerk {
		return user
	}
	profile, err := s.clerk.GetUser(ctx, user.ExternalID)
	if err != nil {
		s.logger.Warn("clerk user enrichment failed", "user_id", user.ExternalID, "err", err)
		return user
	}
	if email := normalizeEmail(profile.Email); email != "" {
		user.Email = email
	}
	if name := profile.DisplayName(); name != "" {
		user.Name = name
	}
	if imageURL := strings.TrimSpace(profile.ImageURL); imageURL != "" {
		user.AvatarURL = &imageURL
	}
	return user
}

func (s *pymesSaaSStore) upsertUserTx(ctx context.Context, tx *gorm.DB, externalID, email, name string, avatarURL *string) (pymesUserRow, error) {
	externalID = strings.TrimSpace(externalID)
	email = normalizeEmail(email)
	name = strings.TrimSpace(name)
	if externalID == "" {
		return pymesUserRow{}, fmt.Errorf("external_id required")
	}
	if email == "" {
		email = placeholderClerkEmail(externalID)
	}
	if name == "" {
		name = email
	}
	var row pymesUserRow
	err := tx.WithContext(ctx).Where("external_id = ?", externalID).Take(&row).Error
	now := time.Now().UTC()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if email != "" && !isPlaceholderClerkEmail(email) {
			err = tx.WithContext(ctx).Where("lower(trim(email)) = ?", email).Take(&row).Error
			if err == nil {
				row.ExternalID = externalID
				given, family := splitFullNameIntoParts(name)
				row.Email = email
				row.GivenName = given
				row.FamilyName = family
				row.Name = joinDisplayName(given, family)
				if avatarURL != nil {
					row.AvatarURL = strings.TrimSpace(*avatarURL)
				}
				row.UpdatedAt = now
				if err := tx.WithContext(ctx).Save(&row).Error; err != nil {
					return pymesUserRow{}, err
				}
				return row, nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return pymesUserRow{}, err
			}
		}
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
		if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
			return pymesUserRow{}, err
		}
		return row, nil
	}
	if err != nil {
		return pymesUserRow{}, err
	}
	if isPlaceholderClerkEmail(email) && strings.TrimSpace(row.Email) != "" && !isPlaceholderClerkEmail(row.Email) {
		email = normalizeEmail(row.Email)
	}
	if isSyntheticClerkName(name, externalID) {
		existingName := strings.TrimSpace(joinDisplayName(row.GivenName, row.FamilyName))
		if existingName == "" {
			existingName = strings.TrimSpace(row.Name)
		}
		if !isSyntheticClerkName(existingName, externalID) {
			name = existingName
		}
	}
	if name == "" {
		name = email
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
	if err := tx.WithContext(ctx).Save(&row).Error; err != nil {
		return pymesUserRow{}, err
	}
	return row, nil
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
