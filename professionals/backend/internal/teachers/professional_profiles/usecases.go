package professional_profiles

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	archive "github.com/devpablocristo/modules/crud/archive/go/archive"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.ProfessionalProfile, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.ProfessionalProfile) (domain.ProfessionalProfile, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (domain.ProfessionalProfile, error)
	GetBySlug(ctx context.Context, tenantID uuid.UUID, slug string) (domain.ProfessionalProfile, error)
	SlugExists(ctx context.Context, tenantID uuid.UUID, slug string, excludeID *uuid.UUID) (bool, error)
	Update(ctx context.Context, in domain.ProfessionalProfile) (domain.ProfessionalProfile, error)
	ListPublic(ctx context.Context, tenantID uuid.UUID) ([]domain.ProfessionalProfile, error)
	Archive(ctx context.Context, tenantID, id uuid.UUID) error
	Restore(ctx context.Context, tenantID, id uuid.UUID) error
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

type AuditPort interface {
	Log(ctx context.Context, tenantID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
}

func NewUsecases(repo RepositoryPort, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, audit: audit}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.ProfessionalProfile, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.ProfessionalProfile, actor string) (domain.ProfessionalProfile, error) {
	if in.PartyID == uuid.Nil {
		return domain.ProfessionalProfile{}, fmt.Errorf("party_id is required: %w", httperrors.ErrBadInput)
	}

	if in.PublicSlug == "" {
		in.PublicSlug = generateSlug(in.Headline, in.PartyID)
	}
	in.PublicSlug = strings.ToLower(strings.TrimSpace(in.PublicSlug))

	exists, err := u.repo.SlugExists(ctx, in.TenantID, in.PublicSlug, nil)
	if err != nil {
		return domain.ProfessionalProfile{}, err
	}
	if exists {
		return domain.ProfessionalProfile{}, fmt.Errorf("slug '%s' already in use: %w", in.PublicSlug, httperrors.ErrConflict)
	}

	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}

	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.ProfessionalProfile{}, err
	}

	if u.audit != nil {
		u.audit.Log(ctx, out.TenantID.String(), actor, "professional_profile.created", "professional_profile", out.ID.String(), map[string]any{"slug": out.PublicSlug})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, tenantID, id uuid.UUID) (domain.ProfessionalProfile, error) {
	out, err := u.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProfessionalProfile{}, fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return domain.ProfessionalProfile{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateInput, actor string) (domain.ProfessionalProfile, error) {
	current, err := u.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProfessionalProfile{}, fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return domain.ProfessionalProfile{}, err
	}
	if err := archive.IfArchived(current.DeletedAt, "professional profile"); err != nil {
		return domain.ProfessionalProfile{}, err
	}

	if in.PublicSlug != nil {
		slug := strings.ToLower(strings.TrimSpace(*in.PublicSlug))
		if slug != current.PublicSlug {
			exists, err := u.repo.SlugExists(ctx, tenantID, slug, &id)
			if err != nil {
				return domain.ProfessionalProfile{}, err
			}
			if exists {
				return domain.ProfessionalProfile{}, fmt.Errorf("slug '%s' already in use: %w", slug, httperrors.ErrConflict)
			}
			current.PublicSlug = slug
		}
	}
	if in.Bio != nil {
		current.Bio = strings.TrimSpace(*in.Bio)
	}
	if in.Headline != nil {
		current.Headline = strings.TrimSpace(*in.Headline)
	}
	if in.IsPublic != nil {
		current.IsPublic = *in.IsPublic
	}
	if in.IsBookable != nil {
		current.IsBookable = *in.IsBookable
	}
	if in.AcceptsNewClients != nil {
		current.AcceptsNewClients = *in.AcceptsNewClients
	}
	if in.IsFavorite != nil {
		current.IsFavorite = *in.IsFavorite
	}
	if in.Tags != nil {
		current.Tags = *in.Tags
	}
	if in.Metadata != nil {
		current.Metadata = *in.Metadata
	}

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProfessionalProfile{}, fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return domain.ProfessionalProfile{}, err
	}

	if u.audit != nil {
		u.audit.Log(ctx, out.TenantID.String(), actor, "professional_profile.updated", "professional_profile", out.ID.String(), map[string]any{"slug": out.PublicSlug})
	}
	return out, nil
}

func (u *Usecases) Archive(ctx context.Context, tenantID, id uuid.UUID, actor string) error {
	if err := u.repo.Archive(ctx, tenantID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, tenantID.String(), actor, "professional_profile.archived", "professional_profile", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, tenantID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, tenantID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, tenantID.String(), actor, "professional_profile.restored", "professional_profile", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Delete(ctx context.Context, tenantID, id uuid.UUID, actor string) error {
	if err := u.repo.Delete(ctx, tenantID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, tenantID.String(), actor, "professional_profile.deleted", "professional_profile", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) ListPublic(ctx context.Context, tenantID uuid.UUID) ([]domain.ProfessionalProfile, error) {
	return u.repo.ListPublic(ctx, tenantID)
}

func (u *Usecases) GetBySlug(ctx context.Context, tenantID uuid.UUID, slug string) (domain.ProfessionalProfile, error) {
	out, err := u.repo.GetBySlug(ctx, tenantID, slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProfessionalProfile{}, fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return domain.ProfessionalProfile{}, err
	}
	if !out.IsPublic {
		return domain.ProfessionalProfile{}, fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
	}
	return out, nil
}

type UpdateInput struct {
	PublicSlug        *string
	Bio               *string
	Headline          *string
	IsPublic          *bool
	IsBookable        *bool
	AcceptsNewClients *bool
	IsFavorite        *bool
	Tags              *[]string
	Metadata          *map[string]any
}

func generateSlug(headline string, partyID uuid.UUID) string {
	slug := strings.ToLower(strings.TrimSpace(headline))
	slug = strings.ReplaceAll(slug, " ", "-")
	if slug == "" {
		slug = partyID.String()[:8]
	}
	return slug
}
