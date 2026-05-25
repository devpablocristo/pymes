package invoices

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	archive "github.com/devpablocristo/platform/lifecycle/go/archive"
	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	"github.com/google/uuid"
	"gorm.io/gorm"

	invdomain "github.com/devpablocristo/pymes/core/backend/internal/invoices/usecases/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/shared/status"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
)

func parseDate(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]invdomain.Invoice, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]invdomain.Invoice, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (invdomain.Invoice, error)
	Create(ctx context.Context, in invdomain.Invoice) (invdomain.Invoice, error)
	Update(ctx context.Context, in invdomain.Invoice) (invdomain.Invoice, error)
	UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status string) (invdomain.Invoice, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo      RepositoryPort
	audit     AuditPort
	lifecycle *lifecycle.Service // optional; when nil, legacy path
}

// Option customizes Usecases at construction.
type Option func(*Usecases)

// WithLifecycle wires lifecycle.Service for Soft/Restore/HardDelete.
func WithLifecycle(svc *lifecycle.Service) Option {
	return func(u *Usecases) {
		if svc != nil {
			u.lifecycle = svc
		}
	}
}

func NewUsecases(repo RepositoryPort, audit AuditPort, opts ...Option) *Usecases {
	u := &Usecases{repo: repo, audit: audit}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]invdomain.Invoice, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]invdomain.Invoice, error) {
	return u.repo.ListArchived(ctx, orgID, limit)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (invdomain.Invoice, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invdomain.Invoice{}, domainerr.NotFoundf("invoice", id.String())
		}
		return invdomain.Invoice{}, err
	}
	return out, nil
}

type CreateInput struct {
	OrgID           uuid.UUID
	Number          string
	PartyID         *uuid.UUID
	CustomerName    string
	IssuedDate      string // YYYY-MM-DD
	DueDate         string // YYYY-MM-DD
	Status          string
	DiscountPercent float64
	TaxPercent      float64
	Notes           string
	IsFavorite      bool
	Tags            []string
	Items           []CreateItemInput
	CreatedBy       string
}

type CreateItemInput struct {
	Description string
	Qty         float64
	Unit        string
	UnitPrice   float64
	SortOrder   int
}

func (u *Usecases) Create(ctx context.Context, in CreateInput) (invdomain.Invoice, error) {
	status := normalizeStatus(in.Status)
	if !isValidStatus(status) {
		return invdomain.Invoice{}, fmt.Errorf("invalid status: %w", httperrors.ErrBadInput)
	}
	if strings.TrimSpace(in.CustomerName) == "" {
		return invdomain.Invoice{}, fmt.Errorf("customer_name is required: %w", httperrors.ErrBadInput)
	}
	issued, err := parseDate(in.IssuedDate)
	if err != nil {
		return invdomain.Invoice{}, fmt.Errorf("invalid issued_date: %w", httperrors.ErrBadInput)
	}
	due, err := parseDate(in.DueDate)
	if err != nil {
		return invdomain.Invoice{}, fmt.Errorf("invalid due_date: %w", httperrors.ErrBadInput)
	}
	items := make([]invdomain.InvoiceLineItem, 0, len(in.Items))
	subtotal := 0.0
	for i, it := range in.Items {
		if strings.TrimSpace(it.Description) == "" {
			return invdomain.Invoice{}, fmt.Errorf("item description is required: %w", httperrors.ErrBadInput)
		}
		if it.Qty <= 0 {
			return invdomain.Invoice{}, fmt.Errorf("item qty must be > 0: %w", httperrors.ErrBadInput)
		}
		if it.UnitPrice < 0 {
			return invdomain.Invoice{}, fmt.Errorf("item unit_price must be >= 0: %w", httperrors.ErrBadInput)
		}
		lineTotal := it.Qty * it.UnitPrice
		subtotal += lineTotal
		sortOrder := it.SortOrder
		if sortOrder == 0 {
			sortOrder = i + 1
		}
		items = append(items, invdomain.InvoiceLineItem{
			Description: strings.TrimSpace(it.Description),
			Qty:         it.Qty,
			Unit:        strings.TrimSpace(it.Unit),
			UnitPrice:   it.UnitPrice,
			LineTotal:   lineTotal,
			SortOrder:   sortOrder,
		})
	}
	total := subtotal * (1 - in.DiscountPercent/100.0) * (1 + in.TaxPercent/100.0)

	out, err := u.repo.Create(ctx, invdomain.Invoice{
		OrgID:           in.OrgID,
		Number:          in.Number,
		PartyID:         in.PartyID,
		CustomerName:    strings.TrimSpace(in.CustomerName),
		IssuedDate:      issued,
		DueDate:         due,
		Status:          invdomain.InvoiceStatus(status),
		Subtotal:        subtotal,
		DiscountPercent: in.DiscountPercent,
		TaxPercent:      in.TaxPercent,
		Total:           total,
		Notes:           in.Notes,
		IsFavorite:      in.IsFavorite,
		Tags:            in.Tags,
		CreatedBy:       in.CreatedBy,
		Items:           items,
	})
	if err != nil {
		return invdomain.Invoice{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.CreatedBy, "invoice.created", "invoice", out.ID.String(), map[string]any{
			"number": out.Number,
			"total":  out.Total,
		})
	}
	return out, nil
}

// UpdateInput soporta los campos editables del PATCH genérico de invoices.
// El campo Status fue eliminado intencionalmente: el cambio de status va
// SIEMPRE por PATCH /v1/invoices/:id/status (con FSM). El handler debe
// rechazar explícitamente bodies que traigan "status" antes de bindear este
// DTO (Gin ignora silenciosamente campos JSON desconocidos, no devuelve 400).
type UpdateInput struct {
	OrgID           uuid.UUID
	ID              uuid.UUID
	DiscountPercent *float64
	TaxPercent      *float64
	Notes           *string
	IsFavorite      *bool
	Tags            *[]string
	IssuedDate      *string
	DueDate         *string
	Actor           string
}

// UpdateStatusInput cambia el status de una factura validando contra el FSM.
type UpdateStatusInput struct {
	OrgID  uuid.UUID
	ID     uuid.UUID
	Status string
}

// UpdateStatus aplica un cambio de status validando con invoiceStateMachine
// (ver fsm.go). Side effects:
//   - audit `invoice.status_updated`
//   - same-status idempotente (no DB, no audit)
func (u *Usecases) UpdateStatus(ctx context.Context, in UpdateStatusInput, actor string) (invdomain.Invoice, error) {
	next := strings.TrimSpace(strings.ToLower(in.Status))
	if next == "" {
		return invdomain.Invoice{}, domainerr.Validation("status is required")
	}

	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invdomain.Invoice{}, domainerr.NotFoundf("invoice", in.ID.String())
		}
		return invdomain.Invoice{}, err
	}
	if err := archive.IfArchived(current.ArchivedAt, "invoice"); err != nil {
		return invdomain.Invoice{}, err
	}

	if string(current.Status) == next {
		return current, nil
	}

	if err := invoiceStateMachine.Validate(string(current.Status), next); err != nil {
		return invdomain.Invoice{}, status.MapFSMError(string(current.Status), next, err)
	}

	out, err := u.repo.UpdateStatus(ctx, in.OrgID, in.ID, next)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invdomain.Invoice{}, domainerr.NotFoundf("invoice", in.ID.String())
		}
		return invdomain.Invoice{}, fmt.Errorf("update invoice status: %w", err)
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), actor, "invoice.status_updated", "invoice", out.ID.String(), map[string]any{"status": string(out.Status)})
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, in UpdateInput) (invdomain.Invoice, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invdomain.Invoice{}, domainerr.NotFoundf("invoice", in.ID.String())
		}
		return invdomain.Invoice{}, err
	}
	if err := archive.IfArchived(current.ArchivedAt, "invoice"); err != nil {
		return invdomain.Invoice{}, err
	}
	if in.DiscountPercent != nil {
		current.DiscountPercent = *in.DiscountPercent
	}
	if in.TaxPercent != nil {
		current.TaxPercent = *in.TaxPercent
	}
	if in.Notes != nil {
		current.Notes = *in.Notes
	}
	if in.IsFavorite != nil {
		current.IsFavorite = *in.IsFavorite
	}
	if in.Tags != nil {
		current.Tags = *in.Tags
	}
	if in.IssuedDate != nil {
		d, err := parseDate(*in.IssuedDate)
		if err != nil {
			return invdomain.Invoice{}, fmt.Errorf("invalid issued_date: %w", httperrors.ErrBadInput)
		}
		current.IssuedDate = d
	}
	if in.DueDate != nil {
		d, err := parseDate(*in.DueDate)
		if err != nil {
			return invdomain.Invoice{}, fmt.Errorf("invalid due_date: %w", httperrors.ErrBadInput)
		}
		current.DueDate = d
	}
	// Recomputar total con subtotal existente (los items no se editan vía PATCH en F1).
	current.Total = current.Subtotal * (1 - current.DiscountPercent/100.0) * (1 + current.TaxPercent/100.0)

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invdomain.Invoice{}, domainerr.NotFoundf("invoice", in.ID.String())
		}
		return invdomain.Invoice{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.Actor, "invoice.updated", "invoice", out.ID.String(), nil)
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if u.lifecycle != nil {
		return u.lifecycle.SoftDelete(ctx, &lifecycle.ArchiveRequest{
			ResourceType: ResourceTypeInvoice,
			ResourceID:   id,
			TenantID:     orgID.String(),
			Actor:        actor,
		})
	}
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("invoice", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "invoice.archived", "invoice", id.String(), nil)
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if u.lifecycle != nil {
		return u.lifecycle.Restore(ctx, &lifecycle.RestoreRequest{
			ResourceType: ResourceTypeInvoice,
			ResourceID:   id,
			TenantID:     orgID.String(),
			Actor:        actor,
		})
	}
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("invoice", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "invoice.restored", "invoice", id.String(), nil)
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if u.lifecycle != nil {
		return u.lifecycle.HardDelete(ctx, &lifecycle.HardDeleteRequest{
			ResourceType:   ResourceTypeInvoice,
			ResourceID:     id,
			TenantID:       orgID.String(),
			Actor:          actor,
			MustBeArchived: false,
		})
	}
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("invoice", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "invoice.hard_deleted", "invoice", id.String(), nil)
	}
	return nil
}

func normalizeStatus(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	if s == "" {
		s = string(invdomain.InvoiceStatusPending)
	}
	return s
}

func isValidStatus(s string) bool {
	switch invdomain.InvoiceStatus(s) {
	case invdomain.InvoiceStatusPaid, invdomain.InvoiceStatusPending, invdomain.InvoiceStatusOverdue:
		return true
	}
	return false
}
