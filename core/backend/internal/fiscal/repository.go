package fiscal

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/platform/http/go/pagination"
	"github.com/devpablocristo/pymes/core/backend/internal/fiscal/repository/models"
	fiscaldomain "github.com/devpablocristo/pymes/core/backend/internal/fiscal/usecases/domain"
)

const caeVtoLayout = "20060102"

var ErrNotFound = errors.New("fiscal: not found")

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) GetSettings(ctx context.Context, orgID uuid.UUID) (SettingsRecord, error) {
	var row models.SettingsModel
	err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SettingsRecord{}, ErrNotFound
	}
	if err != nil {
		return SettingsRecord{}, err
	}
	return SettingsRecord{
		OrgID: row.OrgID, CUIT: row.CUIT, Environment: row.Environment, TaxCondition: row.TaxCondition,
		CertPEM: row.CertPEM, KeyEncrypted: row.KeyEncrypted, DefaultPointOfSale: row.DefaultPointOfSale,
		Enabled: row.Enabled, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *Repository) SaveSettings(ctx context.Context, rec SettingsRecord) error {
	row := models.SettingsModel{
		OrgID: rec.OrgID, CUIT: rec.CUIT, Environment: rec.Environment, TaxCondition: rec.TaxCondition,
		CertPEM: rec.CertPEM, KeyEncrypted: rec.KeyEncrypted, DefaultPointOfSale: rec.DefaultPointOfSale,
		Enabled: rec.Enabled,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "org_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"cuit", "environment", "tax_condition", "cert_pem", "key_encrypted", "default_point_of_sale", "enabled", "updated_at",
		}),
	}).Create(&row).Error
}

func (r *Repository) GetTicket(ctx context.Context, orgID uuid.UUID, service string) (fiscaldomain.AuthTicket, error) {
	var row models.AuthTicketModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND service = ?", orgID, service).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiscaldomain.AuthTicket{}, ErrNotFound
	}
	if err != nil {
		return fiscaldomain.AuthTicket{}, err
	}
	return fiscaldomain.AuthTicket{Token: row.Token, Sign: row.Sign, ExpiresAt: row.ExpiresAt}, nil
}

func (r *Repository) SaveTicket(ctx context.Context, orgID uuid.UUID, service string, ta fiscaldomain.AuthTicket) error {
	row := models.AuthTicketModel{OrgID: orgID, Service: service, Token: ta.Token, Sign: ta.Sign, ExpiresAt: ta.ExpiresAt}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "service"}},
		DoUpdates: clause.AssignmentColumns([]string{"token", "sign", "expires_at", "updated_at"}),
	}).Create(&row).Error
}

func (r *Repository) SaveVoucher(ctx context.Context, v fiscaldomain.FiscalVoucher) error {
	row := voucherToModel(v)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&row).Error
}

func (r *Repository) GetVoucher(ctx context.Context, orgID, id uuid.UUID) (fiscaldomain.FiscalVoucher, error) {
	var row models.VoucherModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiscaldomain.FiscalVoucher{}, ErrNotFound
	}
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	return voucherToDomain(row), nil
}

func (r *Repository) GetAuthorizedVoucherBySale(ctx context.Context, orgID, saleID uuid.UUID) (fiscaldomain.FiscalVoucher, error) {
	var row models.VoucherModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND sale_id = ? AND status = 'authorized' AND voucher_type IN ?", orgID, saleID, invoiceTypes).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiscaldomain.FiscalVoucher{}, ErrNotFound
	}
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	return voucherToDomain(row), nil
}

// invoiceTypes son los tipos de FACTURA (no NC/ND): A, B, C, E.
var invoiceTypes = []int{1, 6, 11, 19}

func (r *Repository) GetAuthorizedVoucherByReturn(ctx context.Context, orgID, returnID uuid.UUID) (fiscaldomain.FiscalVoucher, error) {
	var row models.VoucherModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND return_id = ? AND status = 'authorized'", orgID, returnID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiscaldomain.FiscalVoucher{}, ErrNotFound
	}
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	return voucherToDomain(row), nil
}

func (r *Repository) ListVouchers(ctx context.Context, orgID uuid.UUID, limit int) ([]fiscaldomain.FiscalVoucher, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 50, MaxLimit: 200})
	var rows []models.VoucherModel
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]fiscaldomain.FiscalVoucher, 0, len(rows))
	for _, row := range rows {
		out = append(out, voucherToDomain(row))
	}
	return out, nil
}

func voucherToModel(v fiscaldomain.FiscalVoucher) models.VoucherModel {
	if v.ExchangeRate <= 0 {
		v.ExchangeRate = 1
	}
	row := models.VoucherModel{
		ID: v.ID, OrgID: v.OrgID, SaleID: v.SaleID, ReturnID: v.ReturnID, AssociatedVoucherID: v.AssociatedVoucherID,
		VoucherType: v.VoucherType, PointOfSale: v.PointOfSale,
		CbteNro: v.CbteNro, Concepto: v.Concepto, DocTipo: v.DocTipo, DocNro: v.DocNro,
		Currency: v.Currency, ExchangeRate: v.ExchangeRate, ImpNeto: v.ImpNeto, ImpIVA: v.ImpIVA, ImpTrib: v.ImpTrib,
		ImpOpEx: v.ImpOpEx, ImpTotConc: v.ImpTotConc, ImpTotal: v.ImpTotal, CAE: v.CAE, QRURL: v.QRURL,
		Status: v.Status, AfipResult: v.AfipResult, EmittedAt: v.EmittedAt, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt,
	}
	if v.CondicionIVAReceptor > 0 {
		c := v.CondicionIVAReceptor
		row.CondicionIvaReceptor = &c
	}
	row.IvaBreakdown = mustJSON(v.IvaBreakdown)
	row.Observations = mustJSON(v.Observations)
	row.Errors = mustJSON(v.Errors)
	if t, err := time.Parse(caeVtoLayout, v.CAEVto); err == nil {
		row.CAEVto = &t
	}
	return row
}

func voucherToDomain(row models.VoucherModel) fiscaldomain.FiscalVoucher {
	v := fiscaldomain.FiscalVoucher{
		ID: row.ID, OrgID: row.OrgID, SaleID: row.SaleID, ReturnID: row.ReturnID, AssociatedVoucherID: row.AssociatedVoucherID,
		VoucherType: row.VoucherType, PointOfSale: row.PointOfSale,
		CbteNro: row.CbteNro, Concepto: row.Concepto, DocTipo: row.DocTipo, DocNro: row.DocNro,
		Currency: row.Currency, ExchangeRate: row.ExchangeRate, ImpNeto: row.ImpNeto, ImpIVA: row.ImpIVA, ImpTrib: row.ImpTrib,
		ImpOpEx: row.ImpOpEx, ImpTotConc: row.ImpTotConc, ImpTotal: row.ImpTotal, CAE: row.CAE, QRURL: row.QRURL,
		Status: row.Status, AfipResult: row.AfipResult, EmittedAt: row.EmittedAt, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
	}
	if row.CondicionIvaReceptor != nil {
		v.CondicionIVAReceptor = *row.CondicionIvaReceptor
	}
	_ = json.Unmarshal(row.IvaBreakdown, &v.IvaBreakdown)
	_ = json.Unmarshal(row.Observations, &v.Observations)
	_ = json.Unmarshal(row.Errors, &v.Errors)
	if row.CAEVto != nil {
		v.CAEVto = row.CAEVto.Format(caeVtoLayout)
	}
	return v
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 {
		return []byte("[]")
	}
	return b
}
