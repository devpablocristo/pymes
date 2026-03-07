package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/users/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/utils"
)

type RepositoryPort interface {
	GetUserByExternalID(externalID string) (domain.User, bool)
	UpsertUser(externalID, email, name, avatarURL string) domain.User
	SoftDeleteUser(externalID string) bool
	AddMembership(orgID, userID uuid.UUID, role string)
	RemoveMembership(orgID, userID uuid.UUID)
	ListMembers(orgID uuid.UUID) []domain.Member
	ListAPIKeys(orgID uuid.UUID) []domain.APIKey
	CreateAPIKey(orgID uuid.UUID, name, createdBy string, scopes []string, rawKey string) domain.APIKey
	DeleteAPIKey(orgID, keyID uuid.UUID) bool
	RotateAPIKey(orgID, keyID uuid.UUID, rawKey string) (domain.APIKey, bool)
}

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) GetMe(ctx context.Context, actor string) (domain.User, error) {
	_ = ctx
	if strings.TrimSpace(actor) == "" {
		return domain.User{}, fmt.Errorf("actor is required: %w", httperrors.ErrBadInput)
	}
	if user, ok := u.repo.GetUserByExternalID(actor); ok {
		return user, nil
	}
	return u.repo.UpsertUser(actor, actor+"@example.local", actor, ""), nil
}

func (u *Usecases) ListMembers(ctx context.Context, orgID string) ([]domain.Member, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	return u.repo.ListMembers(id), nil
}

func (u *Usecases) ListAPIKeys(ctx context.Context, orgID string) ([]domain.APIKey, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	return u.repo.ListAPIKeys(id), nil
}

func (u *Usecases) CreateAPIKey(ctx context.Context, orgID, name, createdBy string, scopes []string) (domain.APIKey, string, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	if strings.TrimSpace(name) == "" {
		return domain.APIKey{}, "", fmt.Errorf("name is required: %w", httperrors.ErrBadInput)
	}
	raw, err := utils.GenerateAPIKey()
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("generate key: %w", err)
	}
	key := u.repo.CreateAPIKey(id, name, createdBy, scopes, raw)
	return key, raw, nil
}

func (u *Usecases) DeleteAPIKey(ctx context.Context, orgID, keyID string) error {
	_ = ctx
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	keyUUID, err := uuid.Parse(keyID)
	if err != nil {
		return fmt.Errorf("invalid key id: %w", httperrors.ErrBadInput)
	}
	if ok := u.repo.DeleteAPIKey(orgUUID, keyUUID); !ok {
		return fmt.Errorf("key not found: %w", httperrors.ErrNotFound)
	}
	return nil
}

func (u *Usecases) RotateAPIKey(ctx context.Context, orgID, keyID string) (domain.APIKey, string, error) {
	_ = ctx
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	keyUUID, err := uuid.Parse(keyID)
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("invalid key id: %w", httperrors.ErrBadInput)
	}
	raw, err := utils.GenerateAPIKey()
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("generate key: %w", err)
	}
	key, ok := u.repo.RotateAPIKey(orgUUID, keyUUID, raw)
	if !ok {
		return domain.APIKey{}, "", fmt.Errorf("key not found: %w", httperrors.ErrNotFound)
	}
	return key, raw, nil
}

func (u *Usecases) UpsertClerkUser(ctx context.Context, externalID, email, name, avatarURL string) error {
	_ = ctx
	if strings.TrimSpace(externalID) == "" {
		return fmt.Errorf("external_id is required: %w", httperrors.ErrBadInput)
	}
	u.repo.UpsertUser(externalID, email, name, avatarURL)
	return nil
}

func (u *Usecases) DeleteClerkUser(ctx context.Context, externalID string) error {
	_ = ctx
	if strings.TrimSpace(externalID) == "" {
		return fmt.Errorf("external_id is required: %w", httperrors.ErrBadInput)
	}
	if ok := u.repo.SoftDeleteUser(externalID); !ok {
		return fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	return nil
}

func (u *Usecases) UpsertOrgMembership(ctx context.Context, orgID, userExternalID, role string) error {
	_ = ctx
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	user, ok := u.repo.GetUserByExternalID(userExternalID)
	if !ok {
		return fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	if strings.TrimSpace(role) == "" {
		role = "member"
	}
	u.repo.AddMembership(orgUUID, user.ID, role)
	return nil
}

func (u *Usecases) DeleteOrgMembership(ctx context.Context, orgID, userExternalID string) error {
	_ = ctx
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	user, ok := u.repo.GetUserByExternalID(userExternalID)
	if !ok {
		return fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	u.repo.RemoveMembership(orgUUID, user.ID)
	return nil
}
