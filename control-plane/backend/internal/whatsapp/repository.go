package whatsapp

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) GetQuoteSnapshot(ctx context.Context, orgID, quoteID uuid.UUID) (QuoteSnapshot, error) {
	var row struct {
		ID           uuid.UUID
		Number       string
		PartyID      *uuid.UUID `gorm:"column:party_id"`
		CustomerName string     `gorm:"column:customer_name"`
		Total        float64
	}
	err := r.db.WithContext(ctx).Table("quotes").Select("id, number, party_id, COALESCE(party_name, '') as customer_name, total").Where("org_id = ? AND id = ?", orgID, quoteID).Take(&row).Error
	if err != nil {
		return QuoteSnapshot{}, err
	}
	return QuoteSnapshot(row), nil
}

func (r *Repository) GetSaleSnapshot(ctx context.Context, orgID, saleID uuid.UUID) (SaleSnapshot, error) {
	var row struct {
		ID           uuid.UUID
		Number       string
		PartyID      *uuid.UUID `gorm:"column:party_id"`
		CustomerName string     `gorm:"column:customer_name"`
		Total        float64
	}
	err := r.db.WithContext(ctx).Table("sales").Select("id, number, party_id, COALESCE(party_name, '') as customer_name, total").Where("org_id = ? AND id = ?", orgID, saleID).Take(&row).Error
	if err != nil {
		return SaleSnapshot{}, err
	}
	return SaleSnapshot(row), nil
}

func (r *Repository) GetPartyPhone(ctx context.Context, orgID, partyID uuid.UUID) (string, string, error) {
	var row struct {
		Phone string
		Name  string `gorm:"column:name"`
	}
	err := r.db.WithContext(ctx).Table("parties").Select("COALESCE(phone,'') as phone, COALESCE(display_name,'') as name").Where("org_id = ? AND id = ?", orgID, partyID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", gorm.ErrRecordNotFound
		}
		return "", "", err
	}
	return strings.TrimSpace(row.Phone), strings.TrimSpace(row.Name), nil
}

func (r *Repository) GetTemplates(ctx context.Context, orgID uuid.UUID) (Templates, error) {
	var row struct {
		QuoteTemplate      string `gorm:"column:wa_quote_template"`
		ReceiptTemplate    string `gorm:"column:wa_receipt_template"`
		DefaultCountryCode string `gorm:"column:wa_default_country_code"`
	}
	err := r.db.WithContext(ctx).Table("tenant_settings").Select("wa_quote_template, wa_receipt_template, wa_default_country_code").Where("org_id = ?", orgID).Take(&row).Error
	if err != nil {
		return Templates{}, err
	}
	return Templates{QuoteTemplate: row.QuoteTemplate, ReceiptTemplate: row.ReceiptTemplate, DefaultCountryCode: row.DefaultCountryCode}, nil
}

func (r *Repository) GetConnectionByPhoneNumberID(ctx context.Context, phoneNumberID string) (Connection, error) {
	var row struct {
		OrgID         uuid.UUID
		PhoneNumberID string
		WABAID        string
		AccessToken   string `gorm:"column:access_token_encrypted"`
		IsActive      bool
	}
	err := r.db.WithContext(ctx).
		Table("whatsapp_connections").
		Select("org_id, phone_number_id, waba_id, access_token_encrypted, is_active").
		Where("phone_number_id = ? AND is_active = true", strings.TrimSpace(phoneNumberID)).
		Take(&row).Error
	if err != nil {
		return Connection{}, err
	}
	return Connection{
		OrgID:         row.OrgID,
		PhoneNumberID: strings.TrimSpace(row.PhoneNumberID),
		WABAID:        strings.TrimSpace(row.WABAID),
		AccessToken:   strings.TrimSpace(row.AccessToken),
		IsActive:      row.IsActive,
	}, nil
}
