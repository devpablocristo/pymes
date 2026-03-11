package professional_profiles

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.ProfessionalProfile, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.ProfessionalProfile) (domain.ProfessionalProfile, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProfessionalProfile, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (domain.ProfessionalProfile, error)
	SlugExists(ctx context.Context, orgID uuid.UUID, slug string, excludeID *uuid.UUID) (bool, error)
	Update(ctx context.Context, in domain.ProfessionalProfile) (domain.ProfessionalProfile, error)
	ListPublic(ctx context.Context, orgID uuid.UUID) ([]domain.ProfessionalProfile, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
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

	exists, err := u.repo.SlugExists(ctx, in.OrgID, in.PublicSlug, nil)
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
		u.audit.Log(ctx, out.OrgID.String(), actor, "professional_profile.created", "professional_profile", out.ID.String(), map[string]any{"slug": out.PublicSlug})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProfessionalProfile, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ProfessionalProfile{}, fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return domain.ProfessionalProfile{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.ProfessionalProfile, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ProfessionalProfile{}, fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return domain.ProfessionalProfile{}, err
	}

	if in.PublicSlug != nil {
		slug := strings.ToLower(strings.TrimSpace(*in.PublicSlug))
		if slug != current.PublicSlug {
			exists, err := u.repo.SlugExists(ctx, orgID, slug, &id)
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
	if in.Metadata != nil {
		current.Metadata = *in.Metadata
	}

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ProfessionalProfile{}, fmt.Errorf("professional profile not found: %w", httperrors.ErrNotFound)
		}
		return domain.ProfessionalProfile{}, err
	}

	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "professional_profile.updated", "professional_profile", out.ID.String(), map[string]any{"slug": out.PublicSlug})
	}
	return out, nil
}

func (u *Usecases) ListPublic(ctx context.Context, orgID uuid.UUID) ([]domain.ProfessionalProfile, error) {
	return u.repo.ListPublic(ctx, orgID)
}

func (u *Usecases) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (domain.ProfessionalProfile, error) {
	out, err := u.repo.GetBySlug(ctx, orgID, slug)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
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
