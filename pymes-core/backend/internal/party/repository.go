package party

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	utils "github.com/devpablocristo/core/backend/go/tags"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/party/repository/models"
	partydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/party/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListParams struct {
	OrgID     uuid.UUID
	Limit     int
	After     *uuid.UUID
	Search    string
	PartyType string
	Role      string
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]partydomain.Party, int64, bool, *uuid.UUID, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := r.db.WithContext(ctx).
		Model(&models.PartyModel{}).
		Where("org_id = ? AND deleted_at IS NULL", p.OrgID)

	if v := strings.TrimSpace(p.PartyType); v != "" {
		q = q.Where("party_type = ?", v)
	}
	if v := strings.TrimSpace(p.Search); v != "" {
		like := "%" + v + "%"
		q = q.Where("(display_name ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR tax_id ILIKE ?)", like, like, like, like)
	}
	if v := strings.TrimSpace(p.Role); v != "" {
		q = q.Where(`EXISTS (
			SELECT 1 FROM party_roles pr
			WHERE pr.party_id = parties.id AND pr.org_id = parties.org_id AND pr.role = ? AND pr.is_active = true
		)`, v)
	}
	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}

	var rows []models.PartyModel
	if err := q.Order("id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out, err := r.hydrateParties(ctx, rows)
	if err != nil {
		return nil, 0, false, nil, err
	}

	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) Create(ctx context.Context, in partydomain.Party) (partydomain.Party, error) {
	var out partydomain.Party
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		addr, _ := json.Marshal(in.Address)
		meta, _ := json.Marshal(defaultMap(in.Metadata))
		row := models.PartyModel{
			ID:          uuid.New(),
			OrgID:       in.OrgID,
			PartyType:   strings.TrimSpace(in.PartyType),
			DisplayName: strings.TrimSpace(in.DisplayName),
			Email:       strings.TrimSpace(in.Email),
			Phone:       strings.TrimSpace(in.Phone),
			Address:     addr,
			TaxID:       strings.TrimSpace(in.TaxID),
			Notes:       strings.TrimSpace(in.Notes),
			Tags:        pq.StringArray(utils.NormalizeTags(in.Tags)),
			Metadata:    meta,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := upsertExtension(ctx, tx, row.ID, row.PartyType, in); err != nil {
			return err
		}
		for _, role := range in.Roles {
			if strings.TrimSpace(role.Role) == "" {
				continue
			}
			if err := createRoleWithTx(ctx, tx, row.OrgID, row.ID, role); err != nil {
				return err
			}
		}
		var err error
		out, err = getByIDWithTx(ctx, tx, row.OrgID, row.ID)
		return err
	})
	if err != nil {
		return partydomain.Party{}, err
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (partydomain.Party, error) {
	return getByIDWithTx(ctx, r.db.WithContext(ctx), orgID, id)
}

func (r *Repository) Update(ctx context.Context, in partydomain.Party) (partydomain.Party, error) {
	var out partydomain.Party
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		addr, _ := json.Marshal(in.Address)
		meta, _ := json.Marshal(defaultMap(in.Metadata))
		res := tx.Model(&models.PartyModel{}).
			Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
			Updates(map[string]any{
				"party_type":   strings.TrimSpace(in.PartyType),
				"display_name": strings.TrimSpace(in.DisplayName),
				"email":        strings.TrimSpace(in.Email),
				"phone":        strings.TrimSpace(in.Phone),
				"address":      addr,
				"tax_id":       strings.TrimSpace(in.TaxID),
				"notes":        strings.TrimSpace(in.Notes),
				"tags":         pq.StringArray(utils.NormalizeTags(in.Tags)),
				"metadata":     meta,
				"updated_at":   time.Now().UTC(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := upsertExtension(ctx, tx, in.ID, strings.TrimSpace(in.PartyType), in); err != nil {
			return err
		}
		var err error
		out, err = getByIDWithTx(ctx, tx, in.OrgID, in.ID)
		return err
	})
	if err != nil {
		return partydomain.Party{}, err
	}
	return out, nil
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.PartyModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Updates(map[string]any{"deleted_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) AddRole(ctx context.Context, orgID, partyID uuid.UUID, in partydomain.PartyRole) (partydomain.PartyRole, error) {
	var out partydomain.PartyRole
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := createRoleWithTx(ctx, tx, orgID, partyID, in); err != nil {
			return err
		}
		roles, err := loadRoles(ctx, tx, orgID, []uuid.UUID{partyID})
		if err != nil {
			return err
		}
		for _, role := range roles[partyID] {
			if role.Role == strings.TrimSpace(in.Role) {
				out = role
				return nil
			}
		}
		return gorm.ErrRecordNotFound
	})
	if err != nil {
		return partydomain.PartyRole{}, err
	}
	return out, nil
}

func (r *Repository) RemoveRole(ctx context.Context, orgID, partyID uuid.UUID, role string) error {
	res := r.db.WithContext(ctx).Where("org_id = ? AND party_id = ? AND role = ?", orgID, partyID, strings.TrimSpace(role)).Delete(&models.PartyRoleModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ListRelationships(ctx context.Context, orgID, partyID uuid.UUID) ([]partydomain.PartyRelationship, error) {
	var rows []models.PartyRelationshipModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND (from_party_id = ? OR to_party_id = ?)", orgID, partyID, partyID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return relationshipRowsToDomain(rows), nil
}

func (r *Repository) CreateRelationship(ctx context.Context, in partydomain.PartyRelationship) (partydomain.PartyRelationship, error) {
	meta, _ := json.Marshal(defaultMap(in.Metadata))
	row := models.PartyRelationshipModel{
		ID:               uuid.New(),
		OrgID:            in.OrgID,
		FromPartyID:      in.FromPartyID,
		ToPartyID:        in.ToPartyID,
		RelationshipType: strings.TrimSpace(in.RelationshipType),
		Metadata:         meta,
		FromDate:         in.FromDate,
		ThruDate:         in.ThruDate,
		CreatedAt:        time.Now().UTC(),
	}
	if row.FromDate.IsZero() {
		row.FromDate = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return partydomain.PartyRelationship{}, err
	}
	return relationshipRowsToDomain([]models.PartyRelationshipModel{row})[0], nil
}

func getByIDWithTx(ctx context.Context, tx *gorm.DB, orgID, id uuid.UUID) (partydomain.Party, error) {
	var row models.PartyModel
	if err := tx.WithContext(ctx).Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).Take(&row).Error; err != nil {
		return partydomain.Party{}, err
	}
	parties, err := hydrateWithTx(ctx, tx, []models.PartyModel{row})
	if err != nil {
		return partydomain.Party{}, err
	}
	if len(parties) == 0 {
		return partydomain.Party{}, gorm.ErrRecordNotFound
	}
	return parties[0], nil
}

func (r *Repository) hydrateParties(ctx context.Context, rows []models.PartyModel) ([]partydomain.Party, error) {
	return hydrateWithTx(ctx, r.db.WithContext(ctx), rows)
}

func hydrateWithTx(ctx context.Context, tx *gorm.DB, rows []models.PartyModel) ([]partydomain.Party, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	orgID := rows[0].OrgID
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	roles, err := loadRoles(ctx, tx, orgID, ids)
	if err != nil {
		return nil, err
	}
	persons, err := loadPersons(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	orgs, err := loadOrganizations(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	agents, err := loadAgents(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]partydomain.Party, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row, roles[row.ID], persons[row.ID], orgs[row.ID], agents[row.ID]))
	}
	return out, nil
}

func loadRoles(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]partydomain.PartyRole, error) {
	var rows []models.PartyRoleModel
	if err := tx.WithContext(ctx).Where("org_id = ? AND party_id IN ?", orgID, ids).Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]partydomain.PartyRole, len(ids))
	for _, row := range rows {
		meta := map[string]any{}
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &meta)
		}
		out[row.PartyID] = append(out[row.PartyID], partydomain.PartyRole{
			ID:          row.ID,
			PartyID:     row.PartyID,
			OrgID:       row.OrgID,
			Role:        row.Role,
			IsActive:    row.IsActive,
			PriceListID: row.PriceListID,
			Metadata:    meta,
			CreatedAt:   row.CreatedAt,
		})
	}
	return out, nil
}

func loadPersons(ctx context.Context, tx *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]*partydomain.PartyPerson, error) {
	var rows []models.PartyPersonModel
	if err := tx.WithContext(ctx).Where("party_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*partydomain.PartyPerson, len(rows))
	for _, row := range rows {
		copy := &partydomain.PartyPerson{FirstName: row.FirstName, LastName: row.LastName}
		out[row.PartyID] = copy
	}
	return out, nil
}

func loadOrganizations(ctx context.Context, tx *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]*partydomain.PartyOrganization, error) {
	var rows []models.PartyOrganizationModel
	if err := tx.WithContext(ctx).Where("party_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*partydomain.PartyOrganization, len(rows))
	for _, row := range rows {
		copy := &partydomain.PartyOrganization{LegalName: row.LegalName, TradeName: row.TradeName, TaxCondition: row.TaxCondition}
		out[row.PartyID] = copy
	}
	return out, nil
}

func loadAgents(ctx context.Context, tx *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]*partydomain.PartyAgent, error) {
	var rows []models.PartyAgentModel
	if err := tx.WithContext(ctx).Where("party_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*partydomain.PartyAgent, len(rows))
	for _, row := range rows {
		cfg := map[string]any{}
		if len(row.Config) > 0 {
			_ = json.Unmarshal(row.Config, &cfg)
		}
		copy := &partydomain.PartyAgent{AgentKind: row.AgentKind, Provider: row.Provider, Config: cfg, IsActive: row.IsActive}
		out[row.PartyID] = copy
	}
	return out, nil
}

func upsertExtension(ctx context.Context, tx *gorm.DB, partyID uuid.UUID, partyType string, in partydomain.Party) error {
	switch partyType {
	case "person":
		if err := tx.WithContext(ctx).Where("party_id = ?", partyID).Delete(&models.PartyOrganizationModel{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("party_id = ?", partyID).Delete(&models.PartyAgentModel{}).Error; err != nil {
			return err
		}
		person := in.Person
		if person == nil {
			person = &partydomain.PartyPerson{}
		}
		return tx.WithContext(ctx).Save(&models.PartyPersonModel{PartyID: partyID, FirstName: person.FirstName, LastName: person.LastName}).Error
	case "organization":
		if err := tx.WithContext(ctx).Where("party_id = ?", partyID).Delete(&models.PartyPersonModel{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("party_id = ?", partyID).Delete(&models.PartyAgentModel{}).Error; err != nil {
			return err
		}
		org := in.Organization
		if org == nil {
			org = &partydomain.PartyOrganization{}
		}
		return tx.WithContext(ctx).Save(&models.PartyOrganizationModel{PartyID: partyID, LegalName: org.LegalName, TradeName: org.TradeName, TaxCondition: org.TaxCondition}).Error
	case "automated_agent":
		if err := tx.WithContext(ctx).Where("party_id = ?", partyID).Delete(&models.PartyPersonModel{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("party_id = ?", partyID).Delete(&models.PartyOrganizationModel{}).Error; err != nil {
			return err
		}
		agent := in.Agent
		if agent == nil {
			agent = &partydomain.PartyAgent{IsActive: true}
		}
		cfg, _ := json.Marshal(defaultMap(agent.Config))
		return tx.WithContext(ctx).Save(&models.PartyAgentModel{PartyID: partyID, AgentKind: agent.AgentKind, Provider: agent.Provider, Config: cfg, IsActive: agent.IsActive}).Error
	default:
		return errors.New("invalid party type")
	}
}

func createRoleWithTx(ctx context.Context, tx *gorm.DB, orgID, partyID uuid.UUID, in partydomain.PartyRole) error {
	meta, _ := json.Marshal(defaultMap(in.Metadata))
	row := models.PartyRoleModel{
		ID:          uuid.New(),
		PartyID:     partyID,
		OrgID:       orgID,
		Role:        strings.TrimSpace(in.Role),
		IsActive:    true,
		PriceListID: in.PriceListID,
		Metadata:    meta,
		CreatedAt:   time.Now().UTC(),
	}
	if row.Role == "" {
		return errors.New("role is required")
	}
	return tx.WithContext(ctx).Where("party_id = ? AND org_id = ? AND role = ?", partyID, orgID, row.Role).
		Assign(models.PartyRoleModel{IsActive: true, PriceListID: in.PriceListID, Metadata: meta}).
		FirstOrCreate(&row).Error
}

func relationshipRowsToDomain(rows []models.PartyRelationshipModel) []partydomain.PartyRelationship {
	out := make([]partydomain.PartyRelationship, 0, len(rows))
	for _, row := range rows {
		meta := map[string]any{}
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &meta)
		}
		out = append(out, partydomain.PartyRelationship{
			ID:               row.ID,
			OrgID:            row.OrgID,
			FromPartyID:      row.FromPartyID,
			ToPartyID:        row.ToPartyID,
			RelationshipType: row.RelationshipType,
			Metadata:         meta,
			FromDate:         row.FromDate,
			ThruDate:         row.ThruDate,
			CreatedAt:        row.CreatedAt,
		})
	}
	return out
}

func toDomain(row models.PartyModel, roles []partydomain.PartyRole, person *partydomain.PartyPerson, org *partydomain.PartyOrganization, agent *partydomain.PartyAgent) partydomain.Party {
	addr := partydomain.Address{}
	_ = json.Unmarshal(row.Address, &addr)
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return partydomain.Party{
		ID:           row.ID,
		OrgID:        row.OrgID,
		PartyType:    row.PartyType,
		DisplayName:  row.DisplayName,
		Email:        row.Email,
		Phone:        row.Phone,
		Address:      addr,
		TaxID:        row.TaxID,
		Notes:        row.Notes,
		Tags:         append([]string(nil), row.Tags...),
		Metadata:     meta,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
		DeletedAt:    row.DeletedAt,
		Person:       person,
		Organization: org,
		Agent:        agent,
		Roles:        roles,
	}
}

func defaultMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}
