package pricelists

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	pricelistdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/pricelists/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/apperror"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]pricelistdomain.PriceList, error)
	Create(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (pricelistdomain.PriceList, error)
	Update(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type Usecases struct{ repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]pricelistdomain.PriceList, error) {
	return u.repo.List(ctx, orgID, activeOnly, limit)
}

func (u *Usecases) Create(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error) {
	if strings.TrimSpace(in.Name) == "" {
		return pricelistdomain.PriceList{}, apperror.NewBadInput("name is required")
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
			return pricelistdomain.PriceList{}, apperror.NewNotFound("price_list", id.String())
		}
		return pricelistdomain.PriceList{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error) {
	if strings.TrimSpace(in.Name) == "" {
		return pricelistdomain.PriceList{}, apperror.NewBadInput("name is required")
	}
	return u.repo.Update(ctx, in)
}

func (u *Usecases) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return u.repo.Delete(ctx, orgID, id)
}
