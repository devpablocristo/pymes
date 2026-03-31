package procurement

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/usecases/domain"
)

var (
	ErrNotFound = errors.New("procurement request not found")
	ErrConflict = errors.New("procurement request conflict")
	ErrArchived = errors.New("procurement request is archived")
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, req domain.ProcurementRequest) (domain.ProcurementRequest, error) {
	m := models.ProcurementRequest{
		ID:             req.ID,
		OrgID:          req.OrgID,
		RequesterActor: req.RequesterActor,
		Title:          req.Title,
		Description:    req.Description,
		Category:       req.Category,
		Status:         string(req.Status),
		EstimatedTotal: req.EstimatedTotal,
		Currency:       req.Currency,
		EvaluationJSON: nil,
		PurchaseID:     req.PurchaseID,
		CreatedAt:      req.CreatedAt,
		UpdatedAt:      req.UpdatedAt,
		ArchivedAt:     req.ArchivedAt,
	}
	if len(req.EvaluationJSON) > 0 {
		m.EvaluationJSON = append([]byte(nil), req.EvaluationJSON...)
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return domain.ProcurementRequest{}, err
	}
	for _, line := range req.Lines {
		lm := models.ProcurementRequestLine{
			ID:                line.ID,
			RequestID:         m.ID,
			Description:       line.Description,
			ProductID:         line.ProductID,
			Quantity:          line.Quantity,
			UnitPriceEstimate: line.UnitPriceEstimate,
			SortOrder:         line.SortOrder,
		}
		if err := r.db.WithContext(ctx).Create(&lm).Error; err != nil {
			return domain.ProcurementRequest{}, err
		}
	}
	return r.GetByID(ctx, req.OrgID, m.ID)
}

func (r *Repository) Update(ctx context.Context, req domain.ProcurementRequest) (domain.ProcurementRequest, error) {
	var cur models.ProcurementRequest
	if err := r.db.WithContext(ctx).Where("id = ? AND org_id = ?", req.ID, req.OrgID).First(&cur).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProcurementRequest{}, ErrNotFound
		}
		return domain.ProcurementRequest{}, err
	}
	if cur.ArchivedAt != nil {
		return domain.ProcurementRequest{}, ErrArchived
	}
	cur.Title = req.Title
	cur.Description = req.Description
	cur.Category = req.Category
	cur.EstimatedTotal = req.EstimatedTotal
	cur.Currency = req.Currency
	cur.Status = string(req.Status)
	cur.PurchaseID = req.PurchaseID
	cur.UpdatedAt = time.Now()
	if len(req.EvaluationJSON) > 0 {
		cur.EvaluationJSON = append([]byte(nil), req.EvaluationJSON...)
	} else {
		cur.EvaluationJSON = nil
	}
	if err := r.db.WithContext(ctx).Save(&cur).Error; err != nil {
		return domain.ProcurementRequest{}, err
	}
	if err := r.db.WithContext(ctx).Where("request_id = ?", req.ID).Delete(&models.ProcurementRequestLine{}).Error; err != nil {
		return domain.ProcurementRequest{}, err
	}
	for _, line := range req.Lines {
		lm := models.ProcurementRequestLine{
			ID:                line.ID,
			RequestID:         cur.ID,
			Description:       line.Description,
			ProductID:         line.ProductID,
			Quantity:          line.Quantity,
			UnitPriceEstimate: line.UnitPriceEstimate,
			SortOrder:         line.SortOrder,
		}
		if err := r.db.WithContext(ctx).Create(&lm).Error; err != nil {
			return domain.ProcurementRequest{}, err
		}
	}
	return r.GetByID(ctx, req.OrgID, req.ID)
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProcurementRequest, error) {
	var m models.ProcurementRequest
	if err := r.db.WithContext(ctx).Where("id = ? AND org_id = ?", id, orgID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProcurementRequest{}, ErrNotFound
		}
		return domain.ProcurementRequest{}, err
	}
	var lines []models.ProcurementRequestLine
	if err := r.db.WithContext(ctx).Where("request_id = ?", id).Order("sort_order ASC").Find(&lines).Error; err != nil {
		return domain.ProcurementRequest{}, err
	}
	return toDomain(m, lines), nil
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, includeArchived bool, limit int) ([]domain.ProcurementRequest, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.ProcurementRequest{}).Where("org_id = ?", orgID)
	if !includeArchived {
		q = q.Where("archived_at IS NULL")
	}
	var rows []models.ProcurementRequest
	if err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.ProcurementRequest, 0, len(rows))
	for _, row := range rows {
		var lines []models.ProcurementRequestLine
		if err := r.db.WithContext(ctx).Where("request_id = ?", row.ID).Order("sort_order ASC").Find(&lines).Error; err != nil {
			return nil, err
		}
		out = append(out, toDomain(row, lines))
	}
	return out, nil
}

func (r *Repository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("id = ? AND org_id = ? AND archived_at IS NULL", id, orgID).Delete(&models.ProcurementRequest{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Archive(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&models.ProcurementRequest{}).
		Where("id = ? AND org_id = ? AND archived_at IS NULL", id, orgID).
		Updates(map[string]any{"archived_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.ProcurementRequest{}).
		Where("id = ? AND org_id = ? AND archived_at IS NOT NULL", id, orgID).
		Updates(map[string]any{"archived_at": nil, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListPolicies(ctx context.Context, orgID uuid.UUID) ([]models.ProcurementPolicy, error) {
	var rows []models.ProcurementPolicy
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("priority ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) GetPolicyByID(ctx context.Context, orgID, id uuid.UUID) (domain.ProcurementPolicy, error) {
	var m models.ProcurementPolicy
	if err := r.db.WithContext(ctx).Where("id = ? AND org_id = ?", id, orgID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProcurementPolicy{}, ErrNotFound
		}
		return domain.ProcurementPolicy{}, err
	}
	return policyModelToDomain(m), nil
}

func (r *Repository) SavePolicy(ctx context.Context, p domain.ProcurementPolicy) (domain.ProcurementPolicy, error) {
	m := policyDomainToModel(p)
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if err := r.db.WithContext(ctx).Save(&m).Error; err != nil {
		return domain.ProcurementPolicy{}, err
	}
	return policyModelToDomain(m), nil
}

func (r *Repository) DeletePolicy(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("id = ? AND org_id = ?", id, orgID).Delete(&models.ProcurementPolicy{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func policyModelToDomain(m models.ProcurementPolicy) domain.ProcurementPolicy {
	return domain.ProcurementPolicy{
		ID:           m.ID,
		OrgID:        m.OrgID,
		Name:         m.Name,
		Expression:   m.Expression,
		Effect:       m.Effect,
		Priority:     m.Priority,
		Mode:         m.Mode,
		Enabled:      m.Enabled,
		ActionFilter: m.ActionFilter,
		SystemFilter: m.SystemFilter,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func policyDomainToModel(p domain.ProcurementPolicy) models.ProcurementPolicy {
	return models.ProcurementPolicy{
		ID:           p.ID,
		OrgID:        p.OrgID,
		Name:         p.Name,
		Expression:   p.Expression,
		Effect:       p.Effect,
		Priority:     p.Priority,
		Mode:         p.Mode,
		Enabled:      p.Enabled,
		ActionFilter: p.ActionFilter,
		SystemFilter: p.SystemFilter,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func toDomain(m models.ProcurementRequest, lines []models.ProcurementRequestLine) domain.ProcurementRequest {
	dl := make([]domain.RequestLine, 0, len(lines))
	for _, l := range lines {
		dl = append(dl, domain.RequestLine{
			ID:                l.ID,
			RequestID:         l.RequestID,
			Description:       l.Description,
			ProductID:         l.ProductID,
			Quantity:          l.Quantity,
			UnitPriceEstimate: l.UnitPriceEstimate,
			SortOrder:         l.SortOrder,
		})
	}
	var evalJSON []byte
	if len(m.EvaluationJSON) > 0 {
		evalJSON = append([]byte(nil), m.EvaluationJSON...)
	}
	return domain.ProcurementRequest{
		ID:             m.ID,
		OrgID:          m.OrgID,
		RequesterActor: m.RequesterActor,
		Title:          m.Title,
		Description:    m.Description,
		Category:       m.Category,
		Status:         domain.RequestStatus(m.Status),
		EstimatedTotal: m.EstimatedTotal,
		Currency:       m.Currency,
		EvaluationJSON: evalJSON,
		PurchaseID:     m.PurchaseID,
		Lines:          dl,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
		ArchivedAt:     m.ArchivedAt,
	}
}
