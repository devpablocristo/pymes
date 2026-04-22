package customers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	customerdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/customers/usecases/domain"
	archive "github.com/devpablocristo/modules/crud/archive/go/archive"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]customerdomain.Customer, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID) ([]customerdomain.Customer, error)
	Create(ctx context.Context, in customerdomain.Customer) (customerdomain.Customer, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (customerdomain.Customer, error)
	Update(ctx context.Context, in customerdomain.Customer) (customerdomain.Customer, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
	ListSales(ctx context.Context, orgID, customerID uuid.UUID) ([]customerdomain.SaleHistoryItem, error)
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

var nonDigitPhoneChars = regexp.MustCompile(`\D+`)

func normalizeArgentinaPhone(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	digits := nonDigitPhoneChars.ReplaceAllString(trimmed, "")
	if digits == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(digits, "549"):
		return "+" + digits
	case strings.HasPrefix(digits, "54"):
		return "+" + digits
	case strings.HasPrefix(digits, "0"):
		return "+54" + digits[1:]
	default:
		return "+54" + digits
	}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]customerdomain.Customer, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in customerdomain.Customer, actor string) (customerdomain.Customer, error) {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 {
		return customerdomain.Customer{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	if in.Type == "" {
		in.Type = "person"
	}
	if in.Type != "person" && in.Type != "company" {
		return customerdomain.Customer{}, fmt.Errorf("invalid type: %w", httperrors.ErrBadInput)
	}
	in.Phone = normalizeArgentinaPhone(in.Phone)
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return customerdomain.Customer{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "customer.created", "customer", out.ID.String(), map[string]any{"name": out.Name})
	}
	return out, nil
}

type UpdateInput struct {
	Type     *string
	Name     *string
	TaxID    *string
	Email    *string
	Phone    *string
	Address  *customerdomain.Address
	Notes    *string
	IsFavorite *bool
	Tags     *[]string
	Metadata *map[string]any
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (customerdomain.Customer, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return customerdomain.Customer{}, fmt.Errorf("customer not found: %w", httperrors.ErrNotFound)
		}
		return customerdomain.Customer{}, err
	}
	if err := archive.IfArchived(current.DeletedAt, "customer"); err != nil {
		return customerdomain.Customer{}, err
	}
	if in.Type != nil {
		current.Type = strings.TrimSpace(*in.Type)
	}
	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.TaxID != nil {
		current.TaxID = strings.TrimSpace(*in.TaxID)
	}
	if in.Email != nil {
		current.Email = strings.TrimSpace(*in.Email)
	}
	if in.Phone != nil {
		current.Phone = normalizeArgentinaPhone(*in.Phone)
	}
	if in.Address != nil {
		current.Address = *in.Address
	}
	if in.Notes != nil {
		current.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.IsFavorite != nil {
		current.IsFavorite = *in.IsFavorite
	}
	if in.Tags != nil {
		current.Tags = append([]string(nil), (*in.Tags)...)
	}
	if in.Metadata != nil {
		current.Metadata = *in.Metadata
	}

	if len(current.Name) < 2 {
		return customerdomain.Customer{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	if current.Type != "person" && current.Type != "company" {
		return customerdomain.Customer{}, fmt.Errorf("invalid type: %w", httperrors.ErrBadInput)
	}

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return customerdomain.Customer{}, fmt.Errorf("customer not found: %w", httperrors.ErrNotFound)
		}
		return customerdomain.Customer{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "customer.updated", "customer", out.ID.String(), map[string]any{"name": out.Name})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (customerdomain.Customer, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return customerdomain.Customer{}, fmt.Errorf("customer not found: %w", httperrors.ErrNotFound)
		}
		return customerdomain.Customer{}, err
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("customer not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "customer.deleted", "customer", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID) ([]customerdomain.Customer, error) {
	return u.repo.ListArchived(ctx, orgID)
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("customer not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "customer.restored", "customer", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("customer not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "customer.hard_deleted", "customer", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) ListSales(ctx context.Context, orgID, customerID uuid.UUID) ([]customerdomain.SaleHistoryItem, error) {
	return u.repo.ListSales(ctx, orgID, customerID)
}
