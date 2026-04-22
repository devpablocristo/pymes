package returns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/errors/go/domainerr"
	archive "github.com/devpablocristo/modules/crud/archive/go/archive"
	returndomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/returns/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID, limit int) ([]returndomain.Return, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]returndomain.Return, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (returndomain.Return, error)
	Create(ctx context.Context, in CreateReturnInput) (returndomain.Return, *returndomain.CreditNote, error)
	Update(ctx context.Context, in returndomain.Return) (returndomain.Return, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	RestoreArchived(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
	Void(ctx context.Context, orgID, id uuid.UUID, actor string) (returndomain.Return, error)
	ListCreditNotes(ctx context.Context, orgID uuid.UUID, partyID *uuid.UUID, limit int) ([]returndomain.CreditNote, error)
	GetCreditNote(ctx context.Context, orgID, id uuid.UUID) (returndomain.CreditNote, error)
	ApplyCredit(ctx context.Context, in ApplyCreditInput) (returndomain.CreditNote, error)
	CreateManualCreditNote(ctx context.Context, in CreateManualCreditNoteInput) (returndomain.CreditNote, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type TimelinePort interface {
	RecordEvent(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, eventType, title, description, actor string, metadata map[string]any) error
}

type WebhookPort interface {
	Enqueue(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error
}

type Usecases struct {
	repo     RepositoryPort
	audit    AuditPort
	timeline TimelinePort
	webhooks WebhookPort
}

func NewUsecases(repo RepositoryPort, audit AuditPort, timeline TimelinePort, webhooks WebhookPort) *Usecases {
	return &Usecases{repo: repo, audit: audit, timeline: timeline, webhooks: webhooks}
}

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, limit int) ([]returndomain.Return, error) {
	return u.repo.List(ctx, orgID, limit)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]returndomain.Return, error) {
	return u.repo.ListArchived(ctx, orgID, limit)
}

func (u *Usecases) Update(ctx context.Context, in returndomain.Return, actor string) (returndomain.Return, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		return returndomain.Return{}, translate(err, "return", in.ID.String())
	}
	if err := archive.IfArchived(current.ArchivedAt, "return"); err != nil {
		return returndomain.Return{}, err
	}
	current.Notes = strings.TrimSpace(in.Notes)
	current.IsFavorite = in.IsFavorite
	current.Tags = in.Tags
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		return returndomain.Return{}, translate(err, "return", in.ID.String())
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "return.updated", "return", out.ID.String(), nil)
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		return translate(err, "return", id.String())
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "return.archived", "return", id.String(), nil)
	}
	return nil
}

func (u *Usecases) RestoreArchived(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.RestoreArchived(ctx, orgID, id); err != nil {
		return translate(err, "return", id.String())
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "return.restored", "return", id.String(), nil)
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		return translate(err, "return", id.String())
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "return.hard_deleted", "return", id.String(), nil)
	}
	return nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (returndomain.Return, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return returndomain.Return{}, domainerr.NotFoundf("return", id.String())
		}
		return returndomain.Return{}, err
	}
	return out, nil
}

func (u *Usecases) Create(ctx context.Context, in CreateReturnInput) (returndomain.Return, *returndomain.CreditNote, error) {
	if in.OrgID == uuid.Nil || in.SaleID == uuid.Nil {
		return returndomain.Return{}, nil, domainerr.Validation("org_id and sale_id are required")
	}
	in.Reason = normalizeReason(in.Reason)
	if !isValidRefundMethod(in.RefundMethod) {
		return returndomain.Return{}, nil, domainerr.Validation("invalid refund_method")
	}
	out, credit, err := u.repo.Create(ctx, in)
	if err != nil {
		return returndomain.Return{}, nil, translate(err, "return", out.ID.String())
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.CreatedBy, "return.created", "return", out.ID.String(), map[string]any{"number": out.Number, "total": out.Total, "refund_method": out.RefundMethod})
	}
	if u.timeline != nil && out.PartyID != nil {
		_ = u.timeline.RecordEvent(ctx, in.OrgID, "parties", *out.PartyID, "return.created", "Devolucion registrada", out.Number+" por "+formatAmount(out.Total), in.CreatedBy, map[string]any{"return_id": out.ID.String(), "sale_id": out.SaleID.String()})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, in.OrgID, "return.created", map[string]any{"return_id": out.ID.String(), "sale_id": out.SaleID.String(), "refund_method": out.RefundMethod, "total": out.Total})
	}
	return out, credit, nil
}

func (u *Usecases) Void(ctx context.Context, orgID, id uuid.UUID, actor string) (returndomain.Return, error) {
	out, err := u.repo.Void(ctx, orgID, id, actor)
	if err != nil {
		return returndomain.Return{}, translate(err, "return", id.String())
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "return.voided", "return", id.String(), map[string]any{"number": out.Number, "total": out.Total})
	}
	if u.timeline != nil && out.PartyID != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "parties", *out.PartyID, "return.voided", "Devolucion anulada", out.Number, actor, map[string]any{"return_id": out.ID.String()})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, orgID, "return.voided", map[string]any{"return_id": out.ID.String(), "sale_id": out.SaleID.String()})
	}
	return out, nil
}

func (u *Usecases) ListCreditNotes(ctx context.Context, orgID uuid.UUID, partyID *uuid.UUID, limit int) ([]returndomain.CreditNote, error) {
	return u.repo.ListCreditNotes(ctx, orgID, partyID, limit)
}

func (u *Usecases) GetCreditNote(ctx context.Context, orgID, id uuid.UUID) (returndomain.CreditNote, error) {
	out, err := u.repo.GetCreditNote(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return returndomain.CreditNote{}, domainerr.NotFoundf("credit_note", id.String())
		}
		return returndomain.CreditNote{}, err
	}
	return out, nil
}

func (u *Usecases) CreateManualCreditNote(ctx context.Context, in CreateManualCreditNoteInput) (returndomain.CreditNote, error) {
	if in.OrgID == uuid.Nil || in.PartyID == uuid.Nil {
		return returndomain.CreditNote{}, domainerr.Validation("org_id and party_id are required")
	}
	out, err := u.repo.CreateManualCreditNote(ctx, in)
	if err != nil {
		return returndomain.CreditNote{}, translate(err, "credit_note", "")
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.Actor, "credit_note.created_manual", "credit_note", out.ID.String(), map[string]any{"number": out.Number, "amount": out.Amount, "party_id": out.PartyID.String()})
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, in.OrgID, "parties", in.PartyID, "credit_note.created", "Nota de credito manual", out.Number, in.Actor, map[string]any{"credit_note_id": out.ID.String()})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, in.OrgID, "credit_note.created", map[string]any{"credit_note_id": out.ID.String(), "party_id": out.PartyID.String(), "amount": out.Amount})
	}
	return out, nil
}

func (u *Usecases) ApplyCredit(ctx context.Context, in ApplyCreditInput) (returndomain.CreditNote, error) {
	if in.OrgID == uuid.Nil || in.SaleID == uuid.Nil || in.CreditNoteID == uuid.Nil {
		return returndomain.CreditNote{}, domainerr.Validation("org_id, sale_id and credit_note_id are required")
	}
	out, err := u.repo.ApplyCredit(ctx, in)
	if err != nil {
		return returndomain.CreditNote{}, translate(err, "credit_note", in.CreditNoteID.String())
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.Actor, "credit_note.applied", "credit_note", out.ID.String(), map[string]any{"sale_id": in.SaleID.String(), "balance": out.Balance})
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, in.OrgID, "sales", in.SaleID, "credit_note.applied", "Nota de credito aplicada", out.Number, in.Actor, map[string]any{"credit_note_id": out.ID.String()})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, in.OrgID, "credit_note.applied", map[string]any{"credit_note_id": out.ID.String(), "sale_id": in.SaleID.String(), "balance": out.Balance})
	}
	return out, nil
}

func normalizeReason(v string) string {
	reason := strings.TrimSpace(strings.ToLower(v))
	switch reason {
	case "defective", "wrong_item", "changed_mind", "other":
		return reason
	default:
		return "other"
	}
}

func isValidRefundMethod(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "cash", "credit_note", "original_method":
		return true
	default:
		return false
	}
}

func translate(err error, kind, id string) error {
	if err == nil {
		return nil
	}
	var de domainerr.Error
	if errors.As(err, &de) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domainerr.NotFoundf(kind, id)
	}
	return err
}

func formatAmount(v float64) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), "."))
}
