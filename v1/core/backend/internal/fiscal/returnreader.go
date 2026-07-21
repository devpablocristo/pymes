package fiscal

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/platform/errors/go/domainerr"
)

// gormReturnReader implementa ReturnReader leyendo returns + return_items.
type gormReturnReader struct{ db *gorm.DB }

func NewReturnReader(db *gorm.DB) ReturnReader { return &gormReturnReader{db: db} }

func (g *gormReturnReader) GetReturnFiscalData(ctx context.Context, orgID, returnID uuid.UUID) (ReturnFiscalData, error) {
	var head struct {
		SaleID   uuid.UUID
		Subtotal float64
		TaxTotal float64
		Total    float64
	}
	err := g.db.WithContext(ctx).Table("returns").
		Select("sale_id, subtotal, tax_total, total").
		Where("org_id = ? AND id = ?", orgID, returnID).Take(&head).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReturnFiscalData{}, domainerr.NotFoundf("return", returnID.String())
	}
	if err != nil {
		return ReturnFiscalData{}, err
	}
	var items []struct {
		TaxRate  float64
		Subtotal float64
	}
	if err := g.db.WithContext(ctx).Table("return_items").
		Select("tax_rate, subtotal").Where("return_id = ?", returnID).Scan(&items).Error; err != nil {
		return ReturnFiscalData{}, err
	}
	out := ReturnFiscalData{SaleID: head.SaleID, Subtotal: head.Subtotal, TaxTotal: head.TaxTotal, Total: head.Total}
	for _, it := range items {
		out.Items = append(out.Items, SaleFiscalItem{TaxRate: it.TaxRate, Subtotal: it.Subtotal})
	}
	return out, nil
}
