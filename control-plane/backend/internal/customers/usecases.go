package customers

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	customerdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/customers/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]customerdomain.Customer, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in customerdomain.Customer) (customerdomain.Customer, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (customerdomain.Customer, error)
	Update(ctx context.Context, in customerdomain.Customer) (customerdomain.Customer, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
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
		current.Phone = strings.TrimSpace(*in.Phone)
	}
	if in.Address != nil {
		current.Address = *in.Address
	}
	if in.Notes != nil {
		current.Notes = strings.TrimSpace(*in.Notes)
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

func (u *Usecases) ListSales(ctx context.Context, orgID, customerID uuid.UUID) ([]customerdomain.SaleHistoryItem, error) {
	return u.repo.ListSales(ctx, orgID, customerID)
}

func (u *Usecases) ExportCSV(ctx context.Context, orgID uuid.UUID) ([]byte, error) {
	items, _, _, _, err := u.repo.List(ctx, ListParams{OrgID: orgID, Limit: 10000, Order: "asc"})
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"id", "type", "name", "tax_id", "email", "phone", "city", "country", "notes", "tags"})
	for _, it := range items {
		_ = w.Write([]string{
			it.ID.String(), it.Type, it.Name, it.TaxID, it.Email, it.Phone,
			it.Address.City, it.Address.Country, it.Notes, strings.Join(it.Tags, "|"),
		})
	}
	w.Flush()
	return []byte(b.String()), w.Error()
}

func (u *Usecases) ImportCSV(ctx context.Context, orgID uuid.UUID, csvData []byte, actor string) (int, error) {
	r := csv.NewReader(strings.NewReader(string(csvData)))
	rows, err := r.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("invalid csv: %w", httperrors.ErrBadInput)
	}
	count := 0
	for i, row := range rows {
		if i == 0 || len(row) < 3 {
			continue
		}
		name := strings.TrimSpace(row[2])
		if len(name) < 2 {
			continue
		}
		in := customerdomain.Customer{OrgID: orgID, Type: strings.TrimSpace(row[1]), Name: name}
		if in.Type == "" {
			in.Type = "person"
		}
		if len(row) > 3 {
			in.TaxID = strings.TrimSpace(row[3])
		}
		if len(row) > 4 {
			in.Email = strings.TrimSpace(row[4])
		}
		if len(row) > 5 {
			in.Phone = strings.TrimSpace(row[5])
		}
		if len(row) > 6 {
			in.Address.City = strings.TrimSpace(row[6])
		}
		if len(row) > 7 {
			in.Address.Country = strings.TrimSpace(row[7])
		}
		if len(row) > 8 {
			in.Notes = strings.TrimSpace(row[8])
		}
		if len(row) > 9 && strings.TrimSpace(row[9]) != "" {
			in.Tags = strings.Split(strings.TrimSpace(row[9]), "|")
		}
		if _, err := u.Create(ctx, in, actor); err != nil {
			continue
		}
		count++
	}
	return count, nil
}
