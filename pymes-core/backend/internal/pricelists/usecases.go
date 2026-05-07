package pricelists

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/errors/go/domainerr"
	archive "github.com/devpablocristo/modules/crud/archive/go/archive"
	pricelistdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/pricelists/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, tenantID uuid.UUID, activeOnly bool, limit int) ([]pricelistdomain.PriceList, error)
	ListArchived(ctx context.Context, tenantID uuid.UUID, limit int) ([]pricelistdomain.PriceList, error)
	Create(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (pricelistdomain.PriceList, error)
	Update(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	SoftDelete(ctx context.Context, tenantID, id uuid.UUID) error
	Restore(ctx context.Context, tenantID, id uuid.UUID) error
	HardDelete(ctx context.Context, tenantID, id uuid.UUID) error
}

type Usecases struct{ repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) List(ctx context.Context, tenantID uuid.UUID, activeOnly bool, limit int) ([]pricelistdomain.PriceList, error) {
	return u.repo.List(ctx, tenantID, activeOnly, limit)
}

func (u *Usecases) ListArchived(ctx context.Context, tenantID uuid.UUID, limit int) ([]pricelistdomain.PriceList, error) {
	return u.repo.ListArchived(ctx, tenantID, limit)
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

func (u *Usecases) GetByID(ctx context.Context, tenantID, id uuid.UUID) (pricelistdomain.PriceList, error) {
	out, err := u.repo.GetByID(ctx, tenantID, id)
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
	current, err := u.repo.GetByID(ctx, in.TenantID, in.ID)
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

func (u *Usecases) SoftDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := u.repo.SoftDelete(ctx, tenantID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("price_list", id.String())
		}
		return err
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := u.repo.Restore(ctx, tenantID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("price_list", id.String())
		}
		return err
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := u.repo.HardDelete(ctx, tenantID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("price_list", id.String())
		}
		return err
	}
	return nil
}
