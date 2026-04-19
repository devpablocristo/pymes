package workorders

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	baseworkorders "github.com/devpablocristo/pymes/workshops/backend/internal/workorders"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

const targetType = "bicycle"

type basePort interface {
	List(ctx context.Context, p baseworkorders.ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, targetType string) ([]domain.WorkOrder, error)
	Create(ctx context.Context, in domain.WorkOrder, actor string) (domain.WorkOrder, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.WorkOrder, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in baseworkorders.UpdateInput, actor string) (domain.WorkOrder, error)
	SaveIntegrations(ctx context.Context, orgID, id uuid.UUID, quoteID, saleID *uuid.UUID, status *string, actor string) (domain.WorkOrder, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
	Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error
}

type ListParams = baseworkorders.ListParams
type UpdateInput = baseworkorders.UpdateInput
type WorkOrder = domain.WorkOrder
type WorkOrderItem = domain.WorkOrderItem

type Usecases struct {
	base basePort
}

func NewUsecases(base basePort) *Usecases {
	return &Usecases{base: base}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]WorkOrder, int64, bool, *uuid.UUID, error) {
	p.TargetType = targetType
	return u.base.List(ctx, p)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, _ string) ([]WorkOrder, error) {
	return u.base.ListArchived(ctx, orgID, branchID, targetType)
}

func (u *Usecases) Create(ctx context.Context, in WorkOrder, actor string) (WorkOrder, error) {
	in.TargetType = targetType
	return u.base.Create(ctx, in, actor)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (WorkOrder, error) {
	out, err := u.base.GetByID(ctx, orgID, id)
	if err != nil {
		return WorkOrder{}, err
	}
	if err := ensureSubverticalOwnership(out); err != nil {
		return WorkOrder{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (WorkOrder, error) {
	if _, err := u.GetByID(ctx, orgID, id); err != nil {
		return WorkOrder{}, err
	}
	return u.base.Update(ctx, orgID, id, in, actor)
}

func (u *Usecases) SaveIntegrations(ctx context.Context, orgID, id uuid.UUID, quoteID, saleID *uuid.UUID, status *string, actor string) (WorkOrder, error) {
	if _, err := u.GetByID(ctx, orgID, id); err != nil {
		return WorkOrder{}, err
	}
	return u.base.SaveIntegrations(ctx, orgID, id, quoteID, saleID, status, actor)
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if _, err := u.GetByID(ctx, orgID, id); err != nil {
		return err
	}
	return u.base.SoftDelete(ctx, orgID, id, actor)
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if _, err := u.GetByID(ctx, orgID, id); err != nil {
		return err
	}
	return u.base.Restore(ctx, orgID, id, actor)
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if _, err := u.GetByID(ctx, orgID, id); err != nil {
		return err
	}
	return u.base.HardDelete(ctx, orgID, id, actor)
}

func ensureSubverticalOwnership(order WorkOrder) error {
	if order.TargetType != targetType {
		return fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
	}
	return nil
}
