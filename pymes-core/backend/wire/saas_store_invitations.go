package wire

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const tenantInviteTTL = 7 * 24 * time.Hour

type tenantInvitationDTO struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"tenant_id"`
	Email             string     `json:"email"`
	Role              string     `json:"role"`
	Status            string     `json:"status"`
	ClerkInvitationID *string    `json:"clerk_invitation_id,omitempty"`
	InvitedByUserID   string     `json:"invited_by_user_id"`
	AcceptedByUserID  *string    `json:"accepted_by_user_id,omitempty"`
	ExpiresAt         time.Time  `json:"expires_at"`
	AcceptedAt        *time.Time `json:"accepted_at,omitempty"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type tenantInvitationPreviewDTO struct {
	TenantID   string    `json:"tenant_id"`
	TenantSlug string    `json:"tenant_slug"`
	TenantName string    `json:"tenant_name"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func (s *pymesSaaSStore) ListTenantInvitations(ctx context.Context, tenantID string) ([]tenantInvitationDTO, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return nil, domainerr.Validation("invalid tenant_id")
	}
	var rows []pymesTenantInvitationRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = 'pending'", tenantUUID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]tenantInvitationDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantInvitationDTOFromRow(row))
	}
	return out, nil
}

func (s *pymesSaaSStore) PreviewTenantInvitation(ctx context.Context, token string) (tenantInvitationPreviewDTO, error) {
	tokenHash := hashInviteToken(strings.TrimSpace(token))
	if tokenHash == "" {
		return tenantInvitationPreviewDTO{}, domainerr.Validation("invite token is required")
	}
	var invite pymesTenantInvitationRow
	if err := s.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Take(&invite).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tenantInvitationPreviewDTO{}, domainerr.NotFound("invite not found")
		}
		return tenantInvitationPreviewDTO{}, err
	}
	if invite.Status == "revoked" {
		return tenantInvitationPreviewDTO{}, domainerr.Forbidden("invite revoked")
	}
	if invite.Status == "expired" || time.Now().UTC().After(invite.ExpiresAt) {
		if invite.Status != "expired" {
			now := time.Now().UTC()
			_ = s.db.WithContext(ctx).Model(&pymesTenantInvitationRow{}).Where("id = ?", invite.ID).Updates(map[string]any{
				"status":     "expired",
				"updated_at": now,
			}).Error
		}
		return tenantInvitationPreviewDTO{}, domainerr.BusinessRule("invite_expired")
	}
	if !tenantInviteHasClerkInvitation(invite) {
		return tenantInvitationPreviewDTO{}, domainerr.Conflict("invite was not sent by Clerk")
	}
	tenant, err := s.getTenantRow(ctx, invite.TenantID)
	if err != nil {
		return tenantInvitationPreviewDTO{}, err
	}
	return tenantInvitationPreviewDTO{
		TenantID:   invite.TenantID.String(),
		TenantSlug: tenantRowSlug(tenant),
		TenantName: strings.TrimSpace(tenant.Name),
		Email:      invite.EmailNormalized,
		Role:       invite.Role,
		Status:     invite.Status,
		ExpiresAt:  invite.ExpiresAt,
	}, nil
}

func (s *pymesSaaSStore) CreateTenantInvitation(ctx context.Context, tenantID, actorExternalID, email, role string) (tenantInvitationDTO, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return tenantInvitationDTO{}, domainerr.Validation("invalid tenant_id")
	}
	actor, err := s.requireTenantOwner(ctx, tenantID, actorExternalID)
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	email = normalizeEmail(email)
	if email == "" {
		return tenantInvitationDTO{}, domainerr.Validation("email is required")
	}
	role = normalizeInviteRole(role)
	if err := s.ensureNoActiveMembershipOrPendingInvite(ctx, tenantUUID, email); err != nil {
		return tenantInvitationDTO{}, err
	}
	tenant, err := s.getTenantRow(ctx, tenantUUID)
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	clerkTenantID, err := s.requireClerkPymesOrganizationID()
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	tenantSlug := tenantRowSlug(tenant)
	token, tokenHash, err := generateInviteToken()
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(tenantInviteTTL)
	row := pymesTenantInvitationRow{
		ID:              uuid.New(),
		TenantID:        tenantUUID,
		EmailNormalized: email,
		Role:            role,
		Status:          "pending",
		TokenHash:       tokenHash,
		InvitedByUserID: actor.ID,
		ExpiresAt:       expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return tenantInvitationDTO{}, tenantInviteCreateError(err)
	}
	if s.clerk == nil {
		_ = s.db.WithContext(ctx).Delete(&pymesTenantInvitationRow{}, "id = ?", row.ID).Error
		return tenantInvitationDTO{}, domainerr.Unavailable("clerk backend client is not configured")
	}
	redirectURL := s.inviteRedirectURL(token)
	clerkInvite, err := s.clerk.CreateOrganizationInvitation(ctx, clerkCreateOrganizationInvitationInput{
		OrganizationID: clerkTenantID,
		InviterUserID:  strings.TrimSpace(actorExternalID),
		Email:          email,
		Role:           clerkRoleFromTenantRole(role),
		RedirectURL:    redirectURL,
		PublicMetadata: map[string]any{
			"pymes_tenant_id":   row.TenantID.String(),
			"pymes_tenant_slug": tenantSlug,
			"pymes_invite_id":   row.ID.String(),
			"pymes_role":        role,
		},
	})
	if err != nil {
		_ = s.db.WithContext(ctx).Delete(&pymesTenantInvitationRow{}, "id = ?", row.ID).Error
		return tenantInvitationDTO{}, err
	}
	if strings.TrimSpace(clerkInvite.ID) == "" {
		_ = s.db.WithContext(ctx).Delete(&pymesTenantInvitationRow{}, "id = ?", row.ID).Error
		return tenantInvitationDTO{}, domainerr.UpstreamError("clerk invitation response missing id")
	}
	row.ClerkInvitationID = stringPtr(clerkInvite.ID)
	if clerkInvite.ExpiresAt != nil {
		row.ExpiresAt = *clerkInvite.ExpiresAt
	}
	updatedAt := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&pymesTenantInvitationRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"clerk_invitation_id": row.ClerkInvitationID,
		"expires_at":          row.ExpiresAt,
		"updated_at":          updatedAt,
	}).Error; err != nil {
		return tenantInvitationDTO{}, err
	}
	row.UpdatedAt = updatedAt
	return tenantInvitationDTOFromRow(row), nil
}

func (s *pymesSaaSStore) AcceptTenantInvitation(ctx context.Context, token string, user clerkAuthenticatedUser) (tenantInvitationDTO, string, error) {
	tokenHash := hashInviteToken(strings.TrimSpace(token))
	if tokenHash == "" {
		return tenantInvitationDTO{}, "", domainerr.Validation("invite token is required")
	}
	email := normalizeEmail(user.Email)
	if email == "" {
		return tenantInvitationDTO{}, "", domainerr.Validation("authenticated user email is required")
	}
	var accepted tenantInvitationDTO
	clerkTenantID, err := s.requireClerkPymesOrganizationID()
	if err != nil {
		return tenantInvitationDTO{}, "", err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var invite pymesTenantInvitationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", tokenHash).
			Take(&invite).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domainerr.NotFound("invite not found")
			}
			return err
		}
		if invite.Status == "accepted" {
			if invite.AcceptedByUserID == nil {
				return domainerr.Conflict("invite already used")
			}
			var existingUser pymesUserRow
			if err := tx.Where("id = ?", *invite.AcceptedByUserID).Take(&existingUser).Error; err != nil {
				return err
			}
			if existingUser.ExternalID != strings.TrimSpace(user.ExternalID) {
				return domainerr.Conflict("invite already used")
			}
			if s.clerk == nil {
				return domainerr.Unavailable("clerk backend client is not configured")
			}
			ok, err := s.clerk.UserHasOrganizationMembership(ctx, clerkTenantID, user.ExternalID)
			if err != nil {
				return err
			}
			if !ok {
				return domainerr.Forbidden("clerk pymes organization membership is required")
			}
			accepted = tenantInvitationDTOFromRow(invite)
			return nil
		}
		if invite.Status == "revoked" {
			return domainerr.Forbidden("invite revoked")
		}
		if invite.Status == "expired" || time.Now().UTC().After(invite.ExpiresAt) {
			now := time.Now().UTC()
			_ = tx.Model(&pymesTenantInvitationRow{}).Where("id = ?", invite.ID).Updates(map[string]any{"status": "expired", "updated_at": now}).Error
			return domainerr.BusinessRule("invite_expired")
		}
		if !tenantInviteHasClerkInvitation(invite) {
			return domainerr.Conflict("invite was not sent by Clerk")
		}
		if invite.EmailNormalized != email {
			return domainerr.Forbidden("invite_email_mismatch")
		}
		var tenant pymesTenantRow
		if err := tx.Where("id = ?", invite.TenantID).Take(&tenant).Error; err != nil {
			return err
		}
		if s.clerk == nil {
			return domainerr.Unavailable("clerk backend client is not configured")
		}
		ok, err := s.clerk.UserHasOrganizationMembership(ctx, clerkTenantID, user.ExternalID)
		if err != nil {
			return err
		}
		if !ok {
			return domainerr.Forbidden("clerk pymes organization membership is required")
		}
		localUser, err := s.upsertUserTx(ctx, tx, user.ExternalID, user.Email, user.Name, user.AvatarURL)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		var existing pymesTenantMembershipRow
		err = tx.Where("tenant_id = ? AND user_id = ?", invite.TenantID, localUser.ID).Take(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			existing = pymesTenantMembershipRow{
				ID:        uuid.New(),
				TenantID:  invite.TenantID,
				UserID:    localUser.ID,
				Role:      normalizeInviteRole(invite.Role),
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&existing).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if existing.Status != "active" {
			if err := tx.Model(&pymesTenantMembershipRow{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"role":       normalizeInviteRole(invite.Role),
				"status":     "active",
				"removed_at": nil,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&pymesTenantInvitationRow{}).Where("id = ?", invite.ID).Updates(map[string]any{
			"status":              "accepted",
			"accepted_by_user_id": localUser.ID,
			"accepted_at":         now,
			"updated_at":          now,
		}).Error; err != nil {
			return err
		}
		invite.Status = "accepted"
		invite.AcceptedByUserID = &localUser.ID
		invite.AcceptedAt = &now
		invite.UpdatedAt = now
		accepted = tenantInvitationDTOFromRow(invite)
		return nil
	})
	if err != nil {
		return tenantInvitationDTO{}, "", err
	}
	return accepted, clerkTenantID, nil
}

func (s *pymesSaaSStore) RevokeTenantInvitation(ctx context.Context, tenantID, inviteID, actorExternalID string) (tenantInvitationDTO, error) {
	if _, err := s.requireTenantOwner(ctx, tenantID, actorExternalID); err != nil {
		return tenantInvitationDTO{}, err
	}
	row, tenant, err := s.loadTenantInviteForOwnerAction(ctx, tenantID, inviteID)
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	if row.Status != "pending" {
		return tenantInvitationDTOFromRow(row), nil
	}
	if s.clerk != nil && row.ClerkInvitationID != nil && strings.TrimSpace(*row.ClerkInvitationID) != "" {
		clerkTenantID := s.clerkPymesOrganizationID()
		if clerkTenantID == "" {
			clerkTenantID = clerkTenantIDFromTenant(tenant)
		}
		_ = s.clerk.RevokeOrganizationInvitation(ctx, clerkRevokeOrganizationInvitationInput{
			OrganizationID:   clerkTenantID,
			InvitationID:     *row.ClerkInvitationID,
			RequestingUserID: strings.TrimSpace(actorExternalID),
		})
	}
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&pymesTenantInvitationRow{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status":     "revoked",
		"revoked_at": now,
		"updated_at": now,
	}).Error; err != nil {
		return tenantInvitationDTO{}, err
	}
	row.Status = "revoked"
	row.RevokedAt = &now
	row.UpdatedAt = now
	return tenantInvitationDTOFromRow(row), nil
}

func (s *pymesSaaSStore) ResendTenantInvitation(ctx context.Context, tenantID, inviteID, actorExternalID string) (tenantInvitationDTO, error) {
	actor, err := s.requireTenantOwner(ctx, tenantID, actorExternalID)
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	row, tenant, err := s.loadTenantInviteForOwnerAction(ctx, tenantID, inviteID)
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	if row.Status != "pending" {
		return tenantInvitationDTO{}, domainerr.Conflict("only pending invites can be resent")
	}
	token, tokenHash, err := generateInviteToken()
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	redirectURL := s.inviteRedirectURL(token)
	clerkTenantID, err := s.requireClerkPymesOrganizationID()
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	if s.clerk == nil {
		return tenantInvitationDTO{}, domainerr.Unavailable("clerk backend client is not configured")
	}
	clerkInvite, err := s.clerk.CreateOrganizationInvitation(ctx, clerkCreateOrganizationInvitationInput{
		OrganizationID: clerkTenantID,
		InviterUserID:  strings.TrimSpace(actorExternalID),
		Email:          row.EmailNormalized,
		Role:           clerkRoleFromTenantRole(row.Role),
		RedirectURL:    redirectURL,
		PublicMetadata: map[string]any{
			"pymes_tenant_id":   row.TenantID.String(),
			"pymes_tenant_slug": tenantRowSlug(tenant),
			"pymes_invite_id":   row.ID.String(),
			"pymes_role":        row.Role,
		},
	})
	if err != nil {
		return tenantInvitationDTO{}, err
	}
	if strings.TrimSpace(clerkInvite.ID) == "" {
		return tenantInvitationDTO{}, domainerr.UpstreamError("clerk invitation response missing id")
	}
	now := time.Now().UTC()
	expiresAt := now.Add(tenantInviteTTL)
	if clerkInvite.ExpiresAt != nil {
		expiresAt = *clerkInvite.ExpiresAt
	}
	updates := map[string]any{
		"token_hash":          tokenHash,
		"clerk_invitation_id": stringPtr(clerkInvite.ID),
		"invited_by_user_id":  actor.ID,
		"expires_at":          expiresAt,
		"updated_at":          now,
	}
	if err := s.db.WithContext(ctx).Model(&pymesTenantInvitationRow{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return tenantInvitationDTO{}, err
	}
	row.TokenHash = tokenHash
	row.ClerkInvitationID = stringPtr(clerkInvite.ID)
	row.InvitedByUserID = actor.ID
	row.ExpiresAt = expiresAt
	row.UpdatedAt = now
	return tenantInvitationDTOFromRow(row), nil
}

func (s *pymesSaaSStore) ensureNoActiveMembershipOrPendingInvite(ctx context.Context, tenantUUID uuid.UUID, email string) error {
	var membershipCount int64
	if err := s.db.WithContext(ctx).
		Table("tenant_memberships AS om").
		Joins("JOIN users u ON u.id = om.user_id").
		Where("om.tenant_id = ? AND om.status = 'active' AND lower(trim(u.email)) = ?", tenantUUID, email).
		Count(&membershipCount).Error; err != nil {
		return err
	}
	if membershipCount > 0 {
		return domainerr.Conflict("user is already a tenant member")
	}
	var pendingRows []pymesTenantInvitationRow
	if err := s.db.WithContext(ctx).
		Model(&pymesTenantInvitationRow{}).
		Where("tenant_id = ? AND email_normalized = ? AND status = 'pending'", tenantUUID, email).
		Find(&pendingRows).Error; err != nil {
		return err
	}
	for _, row := range pendingRows {
		if row.ClerkInvitationID != nil && strings.TrimSpace(*row.ClerkInvitationID) != "" {
			return domainerr.Conflict("pending_invite_exists")
		}
	}
	if len(pendingRows) > 0 {
		now := time.Now().UTC()
		ids := make([]uuid.UUID, 0, len(pendingRows))
		for _, row := range pendingRows {
			ids = append(ids, row.ID)
		}
		if err := s.db.WithContext(ctx).Model(&pymesTenantInvitationRow{}).Where("id IN ?", ids).Updates(map[string]any{
			"status":     "revoked",
			"revoked_at": now,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func tenantInviteCreateError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "idx_tenant_invitations_pending_email") || strings.Contains(msg, "unique constraint") {
		return domainerr.Conflict("pending_invite_exists")
	}
	return err
}

func tenantInviteHasClerkInvitation(row pymesTenantInvitationRow) bool {
	return row.ClerkInvitationID != nil && strings.TrimSpace(*row.ClerkInvitationID) != ""
}

func (s *pymesSaaSStore) loadTenantInviteForOwnerAction(ctx context.Context, tenantID, inviteID string) (pymesTenantInvitationRow, pymesTenantRow, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return pymesTenantInvitationRow{}, pymesTenantRow{}, domainerr.Validation("invalid tenant_id")
	}
	inviteUUID, err := uuid.Parse(strings.TrimSpace(inviteID))
	if err != nil {
		return pymesTenantInvitationRow{}, pymesTenantRow{}, domainerr.Validation("invalid invite_id")
	}
	var row pymesTenantInvitationRow
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", inviteUUID, tenantUUID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pymesTenantInvitationRow{}, pymesTenantRow{}, domainerr.NotFound("invite not found")
		}
		return pymesTenantInvitationRow{}, pymesTenantRow{}, err
	}
	tenant, err := s.getTenantRow(ctx, tenantUUID)
	if err != nil {
		return pymesTenantInvitationRow{}, pymesTenantRow{}, err
	}
	return row, tenant, nil
}

func (s *pymesSaaSStore) getTenantRow(ctx context.Context, tenantUUID uuid.UUID) (pymesTenantRow, error) {
	var row pymesTenantRow
	if err := s.db.WithContext(ctx).Where("id = ?", tenantUUID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pymesTenantRow{}, domainerr.NotFound("tenant not found")
		}
		return pymesTenantRow{}, err
	}
	return row, nil
}

func (s *pymesSaaSStore) inviteRedirectURL(token string) string {
	base := strings.TrimRight(strings.TrimSpace(s.frontendURL), "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	u, err := url.Parse(base + "/invite/accept")
	if err != nil {
		return base + "/invite/accept?token=" + url.QueryEscape(token)
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *pymesSaaSStore) clerkPymesOrganizationID() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.clerkPymesOrgID)
}

func (s *pymesSaaSStore) requireClerkPymesOrganizationID() (string, error) {
	clerkOrgID := s.clerkPymesOrganizationID()
	if clerkOrgID == "" {
		return "", domainerr.Unavailable("CLERK_PYMES_ORG_ID is required for tenant invitations")
	}
	return clerkOrgID, nil
}

func tenantInvitationDTOFromRow(row pymesTenantInvitationRow) tenantInvitationDTO {
	var acceptedBy *string
	if row.AcceptedByUserID != nil {
		value := row.AcceptedByUserID.String()
		acceptedBy = &value
	}
	return tenantInvitationDTO{
		ID:                row.ID.String(),
		TenantID:          row.TenantID.String(),
		Email:             row.EmailNormalized,
		Role:              row.Role,
		Status:            row.Status,
		ClerkInvitationID: row.ClerkInvitationID,
		InvitedByUserID:   row.InvitedByUserID.String(),
		AcceptedByUserID:  acceptedBy,
		ExpiresAt:         row.ExpiresAt,
		AcceptedAt:        row.AcceptedAt,
		RevokedAt:         row.RevokedAt,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func tenantRowSlug(row pymesTenantRow) string {
	if row.Slug == nil {
		return ""
	}
	return strings.TrimSpace(*row.Slug)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func generateInviteToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(buf)
	return token, hashInviteToken(token), nil
}

func hashInviteToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func clerkTenantIDFromTenant(row pymesTenantRow) string {
	if row.ClerkOrgID != nil && strings.TrimSpace(*row.ClerkOrgID) != "" {
		return strings.TrimSpace(*row.ClerkOrgID)
	}
	if row.ExternalID != nil && strings.HasPrefix(strings.TrimSpace(*row.ExternalID), "org_") {
		return strings.TrimSpace(*row.ExternalID)
	}
	return ""
}
