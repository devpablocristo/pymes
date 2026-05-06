package wire

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	saasuserdomain "github.com/devpablocristo/core/saas/go/users/usecases/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrUserProfileNotFound indica que no hay fila users para el external_id dado.
var ErrUserProfileNotFound = errors.New("user profile not found")

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

func (s *pymesSaaSStore) SoftDeleteUser(ctx context.Context, externalID string) error {
	return s.db.WithContext(ctx).
		Model(&pymesUserRow{}).
		Where("external_id = ?", strings.TrimSpace(externalID)).
		Update("deleted_at", time.Now().UTC()).Error
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
