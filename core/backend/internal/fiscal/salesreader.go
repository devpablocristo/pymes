package fiscal

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/platform/errors/go/domainerr"
)

// gormSalesReader implementa SalesReader leyendo directamente sales + sale_items
// + parties/party_organizations (lectura cross-módulo, sólo para armar el
// comprobante fiscal de la venta).
type gormSalesReader struct{ db *gorm.DB }

func NewSalesReader(db *gorm.DB) SalesReader { return &gormSalesReader{db: db} }

func (g *gormSalesReader) GetSaleFiscalData(ctx context.Context, orgID, saleID uuid.UUID) (SaleFiscalData, error) {
	var head struct {
		Currency string
		Subtotal float64
		TaxTotal float64
		Total    float64
		PartyID  *uuid.UUID
	}
	err := g.db.WithContext(ctx).Table("sales").
		Select("currency, subtotal, tax_total, total, party_id").
		Where("org_id = ? AND id = ?", orgID, saleID).Take(&head).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SaleFiscalData{}, domainerr.NotFoundf("sale", saleID.String())
	}
	if err != nil {
		return SaleFiscalData{}, err
	}

	var items []struct {
		TaxRate  float64
		Subtotal float64
	}
	if err := g.db.WithContext(ctx).Table("sale_items").
		Select("tax_rate, subtotal").Where("sale_id = ?", saleID).Scan(&items).Error; err != nil {
		return SaleFiscalData{}, err
	}

	out := SaleFiscalData{
		Currency: head.Currency, Subtotal: head.Subtotal, TaxTotal: head.TaxTotal, Total: head.Total,
	}
	for _, it := range items {
		out.Items = append(out.Items, SaleFiscalItem{TaxRate: it.TaxRate, Subtotal: it.Subtotal})
	}

	if head.PartyID != nil {
		var p struct{ TaxID string }
		_ = g.db.WithContext(ctx).Table("parties").Select("tax_id").Where("id = ?", *head.PartyID).Take(&p).Error
		out.CustomerTaxID = p.TaxID
		var o struct{ TaxCondition string }
		_ = g.db.WithContext(ctx).Table("party_organizations").Select("tax_condition").Where("party_id = ?", *head.PartyID).Take(&o).Error
		out.CustomerCondIVA = o.TaxCondition
	}
	return out, nil
}
