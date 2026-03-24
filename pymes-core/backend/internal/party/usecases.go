package party

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/backend/go/domainerr"
	partydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/party/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]partydomain.Party, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in partydomain.Party) (partydomain.Party, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (partydomain.Party, error)
	Update(ctx context.Context, in partydomain.Party) (partydomain.Party, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	AddRole(ctx context.Context, orgID, partyID uuid.UUID, in partydomain.PartyRole) (partydomain.PartyRole, error)
	RemoveRole(ctx context.Context, orgID, partyID uuid.UUID, role string) error
	ListRelationships(ctx context.Context, orgID, partyID uuid.UUID) ([]partydomain.PartyRelationship, error)
	CreateRelationship(ctx context.Context, in partydomain.PartyRelationship) (partydomain.PartyRelationship, error)
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

type Option func(*Usecases)

func WithTimeline(t TimelinePort) Option { return func(u *Usecases) { u.timeline = t } }
func WithWebhooks(w WebhookPort) Option  { return func(u *Usecases) { u.webhooks = w } }

func NewUsecases(repo RepositoryPort, audit AuditPort, opts ...Option) *Usecases {
	uc := &Usecases{repo: repo, audit: audit}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]partydomain.Party, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in partydomain.Party, actor string) (partydomain.Party, error) {
	if err := validateParty(in); err != nil {
		return partydomain.Party{}, err
	}
	out, err := u.repo.Create(ctx, normalizeParty(in))
	if err != nil {
		return partydomain.Party{}, translateRepoErr(err)
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "party.created", "party", out.ID.String(), map[string]any{"party_type": out.PartyType, "display_name": out.DisplayName})
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, out.OrgID, "parties", out.ID, "party.created", "Party creada", out.DisplayName, actor, map[string]any{"party_type": out.PartyType})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, out.OrgID, "party.created", map[string]any{"party_id": out.ID.String(), "party_type": out.PartyType, "display_name": out.DisplayName})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (partydomain.Party, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return partydomain.Party{}, translateRepoErr(err)
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in partydomain.Party, actor string) (partydomain.Party, error) {
	in.ID = id
	in.OrgID = orgID
	if err := validateParty(in); err != nil {
		return partydomain.Party{}, err
	}
	out, err := u.repo.Update(ctx, normalizeParty(in))
	if err != nil {
		return partydomain.Party{}, translateRepoErr(err)
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "party.updated", "party", id.String(), map[string]any{"party_type": out.PartyType, "display_name": out.DisplayName})
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "parties", id, "party.updated", "Party actualizada", out.DisplayName, actor, map[string]any{"party_type": out.PartyType})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, orgID, "party.updated", map[string]any{"party_id": id.String(), "party_type": out.PartyType, "display_name": out.DisplayName})
	}
	return out, nil
}

func (u *Usecases) Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		return translateRepoErr(err)
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "party.deleted", "party", id.String(), map[string]any{})
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "parties", id, "party.deleted", "Party eliminada", id.String(), actor, map[string]any{})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, orgID, "party.deleted", map[string]any{"party_id": id.String()})
	}
	return nil
}

func (u *Usecases) AddRole(ctx context.Context, orgID, partyID uuid.UUID, role string, priceListID *uuid.UUID, metadata map[string]any, actor string) (partydomain.PartyRole, error) {
	role = strings.TrimSpace(role)
	if role == "" {
		return partydomain.PartyRole{}, domainerr.Validation("role is required")
	}
	out, err := u.repo.AddRole(ctx, orgID, partyID, partydomain.PartyRole{Role: role, IsActive: true, PriceListID: priceListID, Metadata: metadata})
	if err != nil {
		return partydomain.PartyRole{}, translateRepoErr(err)
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "party.role_added", "party", partyID.String(), map[string]any{"role": role})
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "parties", partyID, "party.role_added", "Rol agregado", role, actor, map[string]any{"role": role})
	}
	return out, nil
}

func (u *Usecases) RemoveRole(ctx context.Context, orgID, partyID uuid.UUID, role, actor string) error {
	if err := u.repo.RemoveRole(ctx, orgID, partyID, role); err != nil {
		return translateRepoErr(err)
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "party.role_removed", "party", partyID.String(), map[string]any{"role": role})
	}
	if u.timeline != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "parties", partyID, "party.role_removed", "Rol removido", role, actor, map[string]any{"role": role})
	}
	return nil
}

func (u *Usecases) ListRelationships(ctx context.Context, orgID, partyID uuid.UUID) ([]partydomain.PartyRelationship, error) {
	out, err := u.repo.ListRelationships(ctx, orgID, partyID)
	if err != nil {
		return nil, translateRepoErr(err)
	}
	return out, nil
}

func (u *Usecases) CreateRelationship(ctx context.Context, in partydomain.PartyRelationship, actor string) (partydomain.PartyRelationship, error) {
	if in.FromPartyID == uuid.Nil || in.ToPartyID == uuid.Nil {
		return partydomain.PartyRelationship{}, domainerr.Validation("from_party_id and to_party_id are required")
	}
	if strings.TrimSpace(in.RelationshipType) == "" {
		return partydomain.PartyRelationship{}, domainerr.Validation("relationship_type is required")
	}
	if in.FromDate.IsZero() {
		in.FromDate = time.Now().UTC()
	}
	out, err := u.repo.CreateRelationship(ctx, in)
	if err != nil {
		return partydomain.PartyRelationship{}, translateRepoErr(err)
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), actor, "party.relationship_created", "party_relationship", out.ID.String(), map[string]any{"relationship_type": out.RelationshipType})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, in.OrgID, "party.relationship_created", map[string]any{"relationship_id": out.ID.String(), "from_party_id": out.FromPartyID.String(), "to_party_id": out.ToPartyID.String(), "relationship_type": out.RelationshipType})
	}
	return out, nil
}

func validateParty(in partydomain.Party) error {
	if in.OrgID == uuid.Nil {
		return domainerr.Validation("org_id is required")
	}
	if len(strings.TrimSpace(in.DisplayName)) < 2 {
		return domainerr.Validation("display_name must be at least 2 characters")
	}
	switch strings.TrimSpace(in.PartyType) {
	case "person", "organization", "automated_agent":
	default:
		return domainerr.Validation("invalid party_type")
	}
	return nil
}

func normalizeParty(in partydomain.Party) partydomain.Party {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.Email = strings.TrimSpace(in.Email)
	in.Phone = strings.TrimSpace(in.Phone)
	in.TaxID = strings.TrimSpace(in.TaxID)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
	return in
}

func translateRepoErr(err error) error {
	if err == nil {
		return nil
	}
	var de domainerr.Error
	if errors.As(err, &de) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domainerr.NotFoundf("party", "")
	}
	if httperrors.IsUniqueViolation(err) {
		return domainerr.Conflict("resource already exists")
	}
	return fmt.Errorf("party: %w", err)
}
