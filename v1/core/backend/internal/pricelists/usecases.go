package pricelists

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	archive "github.com/devpablocristo/platform/lifecycle/go/archive"
	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	pricelistdomain "github.com/devpablocristo/pymes/core/backend/internal/pricelists/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]pricelistdomain.PriceList, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]pricelistdomain.PriceList, error)
	Create(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (pricelistdomain.PriceList, error)
	Update(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
}

// Usecases is the pricelists application service. As of Ola C step 3, the
// soft-delete / restore / hard-delete flow optionally delegates to
// platform/lifecycle/go.Service when wired via WithLifecycle. When nil,
// behavior falls back to the legacy direct-repository path (preserves
// backward compatibility for tests and non-wired environments).
type Usecases struct {
	repo      RepositoryPort
	lifecycle *lifecycle.Service // optional; when nil, legacy path
}

// Option customizes Usecases at construction.
type Option func(*Usecases)

// WithLifecycle enables the lifecycle.Service path for archive/restore/hard.
// When this option is applied, SoftDelete/Restore/HardDelete go through the
// Service (audit + policy enforcement) instead of the bare repository.
func WithLifecycle(svc *lifecycle.Service) Option {
	return func(u *Usecases) {
		if svc != nil {
			u.lifecycle = svc
		}
	}
}

func NewUsecases(repo RepositoryPort, opts ...Option) *Usecases {
	u := &Usecases{repo: repo}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]pricelistdomain.PriceList, error) {
	return u.repo.List(ctx, orgID, activeOnly, limit)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]pricelistdomain.PriceList, error) {
	return u.repo.ListArchived(ctx, orgID, limit)
}

func (u *Usecases) Create(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error) {
	if strings.TrimSpace(in.Name) == "" {
		return pricelistdomain.PriceList{}, domainerr.Validation("name is required")
	}
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	return u.repo.Create(ctx, in)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (pricelistdomain.PriceList, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pricelistdomain.PriceList{}, domainerr.NotFoundf("price_list", id.String())
		}
		return pricelistdomain.PriceList{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error) {
	if strings.TrimSpace(in.Name) == "" {
		return pricelistdomain.PriceList{}, domainerr.Validation("name is required")
	}
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pricelistdomain.PriceList{}, domainerr.NotFoundf("price_list", in.ID.String())
		}
		return pricelistdomain.PriceList{}, err
	}
	if err := archive.IfArchived(current.ArchivedAt, "price_list"); err != nil {
		return pricelistdomain.PriceList{}, err
	}
	return u.repo.Update(ctx, in)
}

// SoftDelete archives the price list. When a lifecycle.Service is wired
// (WithLifecycle), it dispatches through it for audit + policy enforcement.
// Otherwise it falls back to the legacy direct-repository path.
//
// `actor` is propagated to the audit log when lifecycle is active and
// ignored on the legacy path.
func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if u.lifecycle != nil {
		return u.lifecycle.SoftDelete(ctx, &lifecycle.ArchiveRequest{
			ResourceType: ResourceTypePriceList,
			ResourceID:   id,
			TenantID:     orgID.String(),
			Actor:        actor,
		})
	}
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("price_list", id.String())
		}
		return err
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if u.lifecycle != nil {
		return u.lifecycle.Restore(ctx, &lifecycle.RestoreRequest{
			ResourceType: ResourceTypePriceList,
			ResourceID:   id,
			TenantID:     orgID.String(),
			Actor:        actor,
		})
	}
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("price_list", id.String())
		}
		return err
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if u.lifecycle != nil {
		return u.lifecycle.HardDelete(ctx, &lifecycle.HardDeleteRequest{
			ResourceType:   ResourceTypePriceList,
			ResourceID:     id,
			TenantID:       orgID.String(),
			Actor:          actor,
			MustBeArchived: false, // pymes admin can hard-delete without soft step
		})
	}
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("price_list", id.String())
		}
		return err
	}
	return nil
}
