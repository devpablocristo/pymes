package outwebhooks

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/outwebhooks/repository/models"
	webhookdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/outwebhooks/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/pagination"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ListEndpoints(ctx context.Context, orgID uuid.UUID) ([]webhookdomain.Endpoint, error) {
	var rows []models.EndpointModel
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]webhookdomain.Endpoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, toEndpointDomain(row))
	}
	return out, nil
}

func (r *Repository) CreateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error) {
	row := models.EndpointModel{ID: in.ID, OrgID: in.OrgID, URL: in.URL, Secret: in.Secret, Events: in.Events, IsActive: in.IsActive, CreatedBy: in.CreatedBy, CreatedAt: in.CreatedAt, UpdatedAt: in.UpdatedAt}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return webhookdomain.Endpoint{}, err
	}
	return toEndpointDomain(row), nil
}

func (r *Repository) GetEndpoint(ctx context.Context, orgID, id uuid.UUID) (webhookdomain.Endpoint, error) {
	var row models.EndpointModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return webhookdomain.Endpoint{}, err
	}
	return toEndpointDomain(row), nil
}

func (r *Repository) UpdateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error) {
	updates := map[string]any{"url": in.URL, "secret": in.Secret, "events": in.Events, "is_active": in.IsActive, "updated_at": in.UpdatedAt}
	if err := r.db.WithContext(ctx).Model(&models.EndpointModel{}).Where("org_id = ? AND id = ?", in.OrgID, in.ID).Updates(updates).Error; err != nil {
		return webhookdomain.Endpoint{}, err
	}
	return r.GetEndpoint(ctx, in.OrgID, in.ID)
}

func (r *Repository) DeleteEndpoint(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Delete(&models.EndpointModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ListDeliveries(ctx context.Context, orgID, endpointID uuid.UUID, limit int) ([]webhookdomain.Delivery, error) {
	limit = pagination.NormalizeLimit(limit, 20, 100)
	var endpoint models.EndpointModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, endpointID).Take(&endpoint).Error; err != nil {
		return nil, err
	}
	var rows []models.DeliveryModel
	if err := r.db.WithContext(ctx).Where("endpoint_id = ?", endpointID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]webhookdomain.Delivery, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDeliveryDomain(row))
	}
	return out, nil
}

func (r *Repository) CreateOutbox(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error {
	body, _ := json.Marshal(payload)
	row := models.OutboxModel{ID: uuid.New(), OrgID: orgID, EventType: eventType, Payload: body, Status: "pending", CreatedAt: time.Now().UTC()}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) ListPendingOutbox(ctx context.Context, limit int) ([]models.OutboxModel, error) {
	limit = pagination.NormalizeLimit(limit, 50, 200)
	var rows []models.OutboxModel
	if err := r.db.WithContext(ctx).Where("status = ?", "pending").Order("created_at ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) MarkOutbox(ctx context.Context, id uuid.UUID, status, lastError string) error {
	updates := map[string]any{"status": status, "last_error": lastError, "dispatched_at": time.Now().UTC()}
	return r.db.WithContext(ctx).Model(&models.OutboxModel{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) ListEndpointsForEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]webhookdomain.Endpoint, error) {
	var rows []models.EndpointModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND is_active = true", orgID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]webhookdomain.Endpoint, 0, len(rows))
	for _, row := range rows {
		if len(row.Events) == 0 || containsEvent(row.Events, eventType) {
			out = append(out, toEndpointDomain(row))
		}
	}
	return out, nil
}

func (r *Repository) CreateDelivery(ctx context.Context, endpointID uuid.UUID, eventType string, payload map[string]any, statusCode *int, responseBody string, attempts int, nextRetry, deliveredAt *time.Time) (webhookdomain.Delivery, error) {
	body, _ := json.Marshal(payload)
	row := models.DeliveryModel{ID: uuid.New(), EndpointID: endpointID, EventType: eventType, Payload: body, StatusCode: statusCode, ResponseBody: responseBody, Attempts: attempts, NextRetry: nextRetry, DeliveredAt: deliveredAt, CreatedAt: time.Now().UTC()}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return webhookdomain.Delivery{}, err
	}
	return toDeliveryDomain(row), nil
}

func (r *Repository) ListRetryableDeliveries(ctx context.Context, limit int) ([]models.DeliveryModel, error) {
	limit = pagination.NormalizeLimit(limit, 50, 200)
	var rows []models.DeliveryModel
	err := r.db.WithContext(ctx).
		Where("delivered_at IS NULL AND attempts < 5 AND (next_retry IS NULL OR next_retry <= ?)", time.Now().UTC()).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *Repository) GetDelivery(ctx context.Context, id uuid.UUID) (webhookdomain.Delivery, error) {
	var row models.DeliveryModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error; err != nil {
		return webhookdomain.Delivery{}, err
	}
	return toDeliveryDomain(row), nil
}

func (r *Repository) UpdateDeliveryResult(ctx context.Context, id uuid.UUID, statusCode *int, responseBody string, attempts int, nextRetry, deliveredAt *time.Time) error {
	updates := map[string]any{"status_code": statusCode, "response_body": responseBody, "attempts": attempts, "next_retry": nextRetry, "delivered_at": deliveredAt}
	return r.db.WithContext(ctx).Model(&models.DeliveryModel{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) GetEndpointByID(ctx context.Context, id uuid.UUID) (webhookdomain.Endpoint, error) {
	var row models.EndpointModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error; err != nil {
		return webhookdomain.Endpoint{}, err
	}
	return toEndpointDomain(row), nil
}

func (r *Repository) DeleteOldDeliveries(ctx context.Context, olderThan time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("created_at < ?", olderThan.UTC()).Delete(&models.DeliveryModel{})
	return res.RowsAffected, res.Error
}

func toEndpointDomain(row models.EndpointModel) webhookdomain.Endpoint {
	return webhookdomain.Endpoint{ID: row.ID, OrgID: row.OrgID, URL: row.URL, Secret: row.Secret, Events: row.Events, IsActive: row.IsActive, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func toDeliveryDomain(row models.DeliveryModel) webhookdomain.Delivery {
	payload := map[string]any{}
	if len(row.Payload) > 0 {
		_ = json.Unmarshal(row.Payload, &payload)
	}
	return webhookdomain.Delivery{ID: row.ID, EndpointID: row.EndpointID, EventType: row.EventType, Payload: payload, StatusCode: row.StatusCode, ResponseBody: row.ResponseBody, Attempts: row.Attempts, NextRetry: row.NextRetry, DeliveredAt: row.DeliveredAt, CreatedAt: row.CreatedAt}
}

func containsEvent(events []string, eventType string) bool {
	for _, item := range events {
		if item == "*" || item == eventType {
			return true
		}
	}
	return false
}

func isNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
