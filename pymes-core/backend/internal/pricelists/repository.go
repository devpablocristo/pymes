package pricelists

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/pricelists/repository/models"
	pricelistdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/pricelists/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, activeOnly bool, limit int) ([]pricelistdomain.PriceList, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	q := r.db.WithContext(ctx).Model(&models.PriceListModel{}).Where("org_id = ?", orgID)
	if activeOnly {
		q = q.Where("is_active = true")
	}
	var rows []models.PriceListModel
	if err := q.Order("is_default DESC").Order("name ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]pricelistdomain.PriceList, 0, len(rows))
	for _, row := range rows {
		out = append(out, modelToDomain(row, nil, nil))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error) {
	var out pricelistdomain.PriceList
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if in.IsDefault {
			if err := tx.Model(&models.PriceListModel{}).Where("org_id = ?", in.OrgID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		row := models.PriceListModel{ID: in.ID, OrgID: in.OrgID, Name: in.Name, Description: in.Description, IsDefault: in.IsDefault, Markup: in.Markup, IsActive: in.IsActive, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		productItems, serviceItems := toItemModels(in.ID, in.Items)
		if len(productItems) > 0 {
			if err := tx.Create(&productItems).Error; err != nil {
				return err
			}
		}
		if len(serviceItems) > 0 {
			if err := tx.Create(&serviceItems).Error; err != nil {
				return err
			}
		}
		out = modelToDomain(row, productItems, serviceItems)
		return nil
	})
	if err != nil {
		return pricelistdomain.PriceList{}, err
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (pricelistdomain.PriceList, error) {
	var row models.PriceListModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return pricelistdomain.PriceList{}, err
	}
	var items []models.PriceListItemModel
	if err := r.db.WithContext(ctx).Where("price_list_id = ?", id).Find(&items).Error; err != nil {
		return pricelistdomain.PriceList{}, err
	}
	var serviceItems []models.ServicePriceListItemModel
	if err := r.db.WithContext(ctx).Where("price_list_id = ?", id).Find(&serviceItems).Error; err != nil {
		return pricelistdomain.PriceList{}, err
	}
	return modelToDomain(row, items, serviceItems), nil
}

func (r *Repository) Update(ctx context.Context, in pricelistdomain.PriceList) (pricelistdomain.PriceList, error) {
	var out pricelistdomain.PriceList
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if in.IsDefault {
			if err := tx.Model(&models.PriceListModel{}).Where("org_id = ? AND id <> ?", in.OrgID, in.ID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&models.PriceListModel{}).Where("org_id = ? AND id = ?", in.OrgID, in.ID).Updates(map[string]any{"name": in.Name, "description": in.Description, "is_default": in.IsDefault, "markup": in.Markup, "is_active": in.IsActive, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err := tx.Where("price_list_id = ?", in.ID).Delete(&models.PriceListItemModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("price_list_id = ?", in.ID).Delete(&models.ServicePriceListItemModel{}).Error; err != nil {
			return err
		}
		productItems, serviceItems := toItemModels(in.ID, in.Items)
		if len(productItems) > 0 {
			if err := tx.Create(&productItems).Error; err != nil {
				return err
			}
		}
		if len(serviceItems) > 0 {
			if err := tx.Create(&serviceItems).Error; err != nil {
				return err
			}
		}
		updated, err := r.GetByID(ctx, in.OrgID, in.ID)
		if err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return pricelistdomain.PriceList{}, err
	}
	return out, nil
}

func (r *Repository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Delete(&models.PriceListModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func toItemModels(priceListID uuid.UUID, items []pricelistdomain.PriceListItem) ([]models.PriceListItemModel, []models.ServicePriceListItemModel) {
	productRows := make([]models.PriceListItemModel, 0, len(items))
	serviceRows := make([]models.ServicePriceListItemModel, 0, len(items))
	for _, item := range items {
		if item.ProductID != nil {
			productRows = append(productRows, models.PriceListItemModel{PriceListID: priceListID, ProductID: *item.ProductID, Price: item.Price})
		}
		if item.ServiceID != nil {
			serviceRows = append(serviceRows, models.ServicePriceListItemModel{PriceListID: priceListID, ServiceID: *item.ServiceID, Price: item.Price})
		}
	}
	return productRows, serviceRows
}

func modelToDomain(row models.PriceListModel, items []models.PriceListItemModel, serviceItems []models.ServicePriceListItemModel) pricelistdomain.PriceList {
	out := pricelistdomain.PriceList{ID: row.ID, OrgID: row.OrgID, Name: row.Name, Description: row.Description, IsDefault: row.IsDefault, Markup: row.Markup, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	for _, item := range items {
		productID := item.ProductID
		out.Items = append(out.Items, pricelistdomain.PriceListItem{ProductID: &productID, Price: item.Price})
	}
	for _, item := range serviceItems {
		serviceID := item.ServiceID
		out.Items = append(out.Items, pricelistdomain.PriceListItem{ServiceID: &serviceID, Price: item.Price})
	}
	return out
}
