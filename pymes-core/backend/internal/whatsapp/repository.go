package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/whatsapp/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/whatsapp/usecases/domain"
)

var (
	ErrNotFound       = errors.New("whatsapp: not found")
	ErrAlreadyExists  = errors.New("whatsapp: already exists")
	ErrNotConnected   = errors.New("whatsapp: not connected")
	ErrAlreadyOptedIn = errors.New("whatsapp: already opted in")
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// --- Snapshots (existentes) ---

func (r *Repository) GetQuoteSnapshot(ctx context.Context, orgID, quoteID uuid.UUID) (QuoteSnapshot, error) {
	var row struct {
		ID           uuid.UUID
		Number       string
		PartyID      *uuid.UUID `gorm:"column:party_id"`
		CustomerName string     `gorm:"column:customer_name"`
		Total        float64
	}
	err := r.db.WithContext(ctx).Table("quotes").
		Select("id, number, party_id, COALESCE(party_name, '') as customer_name, total").
		Where("org_id = ? AND id = ?", orgID, quoteID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return QuoteSnapshot{}, ErrNotFound
		}
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
	err := r.db.WithContext(ctx).Table("sales").
		Select("id, number, party_id, COALESCE(party_name, '') as customer_name, total").
		Where("org_id = ? AND id = ?", orgID, saleID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SaleSnapshot{}, ErrNotFound
		}
		return SaleSnapshot{}, err
	}
	return SaleSnapshot(row), nil
}

func (r *Repository) GetPartyPhone(ctx context.Context, orgID, partyID uuid.UUID) (string, string, error) {
	var row struct {
		Phone string
		Name  string `gorm:"column:name"`
	}
	err := r.db.WithContext(ctx).Table("parties").
		Select("COALESCE(phone,'') as phone, COALESCE(display_name,'') as name").
		Where("org_id = ? AND id = ?", orgID, partyID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrNotFound
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
	err := r.db.WithContext(ctx).Table("tenant_settings").
		Select("wa_quote_template, wa_receipt_template, wa_default_country_code").
		Where("org_id = ?", orgID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Templates{DefaultCountryCode: "54"}, nil
		}
		return Templates{}, err
	}
	return Templates{
		QuoteTemplate:      row.QuoteTemplate,
		ReceiptTemplate:    row.ReceiptTemplate,
		DefaultCountryCode: row.DefaultCountryCode,
	}, nil
}

// --- Connection CRUD ---

func (r *Repository) GetConnection(ctx context.Context, orgID uuid.UUID) (domain.Connection, error) {
	var m models.WhatsAppConnection
	err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Connection{}, ErrNotConnected
		}
		return domain.Connection{}, err
	}
	return connectionToDomain(m), nil
}

func (r *Repository) GetConnectionByPhoneNumberID(ctx context.Context, phoneNumberID string) (Connection, error) {
	var m models.WhatsAppConnection
	err := r.db.WithContext(ctx).
		Where("phone_number_id = ? AND is_active = true", strings.TrimSpace(phoneNumberID)).
		Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Connection{}, ErrNotConnected
		}
		return Connection{}, err
	}
	return Connection{
		OrgID:         m.OrgID,
		PhoneNumberID: m.PhoneNumberID,
		WABAID:        m.WABAID,
		AccessToken:   m.AccessTokenEncrypt,
		IsActive:      m.IsActive,
	}, nil
}

func (r *Repository) SaveConnection(ctx context.Context, conn domain.Connection, encryptedToken string) error {
	m := models.WhatsAppConnection{
		OrgID:              conn.OrgID,
		PhoneNumberID:      conn.PhoneNumberID,
		WABAID:             conn.WABAID,
		AccessTokenEncrypt: encryptedToken,
		DisplayPhoneNumber: conn.DisplayPhoneNumber,
		VerifiedName:       conn.VerifiedName,
		QualityRating:      conn.QualityRating,
		MessagingLimit:     conn.MessagingLimit,
		IsActive:           true,
		ConnectedAt:        time.Now(),
		CreatedAt:          time.Now(),
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "org_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"phone_number_id", "waba_id", "access_token_encrypted",
				"display_phone_number", "verified_name", "quality_rating",
				"messaging_limit", "is_active", "connected_at", "disconnected_at",
			}),
		}).Create(&m).Error
}

func (r *Repository) DisconnectConnection(ctx context.Context, orgID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WhatsAppConnection{}).
		Where("org_id = ?", orgID).
		Updates(map[string]any{
			"is_active":       false,
			"disconnected_at": &now,
		}).Error
}

func (r *Repository) GetConnectionStats(ctx context.Context, orgID uuid.UUID) (domain.ConnectionStats, error) {
	var stats domain.ConnectionStats
	rows, err := r.db.WithContext(ctx).
		Table("whatsapp_messages").
		Select("direction, status, COUNT(*) as cnt").
		Where("org_id = ?", orgID).
		Group("direction, status").
		Rows()
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var direction, status string
		var cnt int
		if err := rows.Scan(&direction, &status, &cnt); err != nil {
			return stats, err
		}
		switch {
		case direction == "outbound" && status == "failed":
			stats.TotalFailed += cnt
		case direction == "outbound":
			stats.TotalSent += cnt
		case direction == "inbound":
			stats.TotalReceived += cnt
		}
		if status == "delivered" {
			stats.TotalDelivered += cnt
		}
		if status == "read" {
			stats.TotalRead += cnt
		}
	}
	return stats, rows.Err()
}

// --- Messages CRUD ---

func (r *Repository) SaveMessage(ctx context.Context, msg domain.Message) error {
	params, _ := json.Marshal(msg.TemplateParams)
	meta, _ := json.Marshal(msg.Metadata)
	m := models.WhatsAppMessage{
		ID:               msg.ID,
		OrgID:            msg.OrgID,
		PhoneNumberID:    msg.PhoneNumberID,
		Direction:        string(msg.Direction),
		WAMessageID:      msg.WAMessageID,
		ToPhone:          msg.ToPhone,
		FromPhone:        msg.FromPhone,
		MessageType:      string(msg.MessageType),
		Body:             msg.Body,
		TemplateName:     msg.TemplateName,
		TemplateLanguage: msg.TemplateLanguage,
		TemplateParams:   datatypes.JSON(params),
		MediaURL:         msg.MediaURL,
		MediaMimeType:    msg.MediaMimeType,
		MediaCaption:     msg.MediaCaption,
		Status:           string(msg.Status),
		ErrorCode:        msg.ErrorCode,
		ErrorMessage:     msg.ErrorMessage,
		PartyID:          msg.PartyID,
		Metadata:         datatypes.JSON(meta),
		CreatedAt:        msg.CreatedAt,
		UpdatedAt:        msg.UpdatedAt,
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *Repository) UpdateMessageStatus(ctx context.Context, waMessageID string, status domain.MessageStatus, errorCode, errorMsg string) error {
	updates := map[string]any{
		"status":     string(status),
		"updated_at": time.Now(),
	}
	if errorCode != "" {
		updates["error_code"] = errorCode
		updates["error_message"] = errorMsg
	}
	result := r.db.WithContext(ctx).
		Model(&models.WhatsAppMessage{}).
		Where("wa_message_id = ?", strings.TrimSpace(waMessageID)).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListMessages(ctx context.Context, filter domain.MessageFilter) ([]domain.Message, int, error) {
	q := r.db.WithContext(ctx).Model(&models.WhatsAppMessage{}).Where("org_id = ?", filter.OrgID)
	if filter.PartyID != nil {
		q = q.Where("party_id = ?", *filter.PartyID)
	}
	if filter.Direction != nil {
		q = q.Where("direction = ?", string(*filter.Direction))
	}
	if filter.Status != nil {
		q = q.Where("status = ?", string(*filter.Status))
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows []models.WhatsAppMessage
	err := q.Order("created_at DESC").Offset(filter.Offset).Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	messages := make([]domain.Message, 0, len(rows))
	for _, m := range rows {
		messages = append(messages, messageToDomain(m))
	}
	return messages, int(total), nil
}

// --- Templates CRUD ---

func (r *Repository) SaveTemplate(ctx context.Context, tpl domain.Template) error {
	buttons, _ := json.Marshal(tpl.Buttons)
	exParams, _ := json.Marshal(tpl.ExampleParams)
	m := models.WhatsAppTemplate{
		ID:              tpl.ID,
		OrgID:           tpl.OrgID,
		MetaTemplateID:  tpl.MetaTemplateID,
		Name:            tpl.Name,
		Language:        tpl.Language,
		Category:        string(tpl.Category),
		Status:          string(tpl.Status),
		HeaderType:      tpl.HeaderType,
		HeaderText:      tpl.HeaderText,
		BodyText:        tpl.BodyText,
		FooterText:      tpl.FooterText,
		Buttons:         datatypes.JSON(buttons),
		ExampleParams:   datatypes.JSON(exParams),
		RejectionReason: tpl.RejectionReason,
		CreatedAt:        tpl.CreatedAt,
		UpdatedAt:        tpl.UpdatedAt,
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *Repository) GetTemplate(ctx context.Context, orgID, templateID uuid.UUID) (domain.Template, error) {
	var m models.WhatsAppTemplate
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, templateID).Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Template{}, ErrNotFound
		}
		return domain.Template{}, err
	}
	return templateToDomain(m), nil
}

func (r *Repository) GetTemplateByName(ctx context.Context, orgID uuid.UUID, name, language string) (domain.Template, error) {
	var m models.WhatsAppTemplate
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND name = ? AND language = ?", orgID, name, language).
		Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Template{}, ErrNotFound
		}
		return domain.Template{}, err
	}
	return templateToDomain(m), nil
}

func (r *Repository) ListTemplates(ctx context.Context, orgID uuid.UUID) ([]domain.Template, error) {
	var rows []models.WhatsAppTemplate
	err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("name ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Template, 0, len(rows))
	for _, m := range rows {
		out = append(out, templateToDomain(m))
	}
	return out, nil
}

func (r *Repository) UpdateTemplateStatus(ctx context.Context, orgID, templateID uuid.UUID, status domain.TemplateStatus, metaTemplateID, rejectionReason string) error {
	updates := map[string]any{
		"status":     string(status),
		"updated_at": time.Now(),
	}
	if metaTemplateID != "" {
		updates["meta_template_id"] = metaTemplateID
	}
	if rejectionReason != "" {
		updates["rejection_reason"] = rejectionReason
	}
	return r.db.WithContext(ctx).
		Model(&models.WhatsAppTemplate{}).
		Where("org_id = ? AND id = ?", orgID, templateID).
		Updates(updates).Error
}

func (r *Repository) DeleteTemplate(ctx context.Context, orgID, templateID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, templateID).
		Delete(&models.WhatsAppTemplate{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Opt-in CRUD ---

func (r *Repository) SaveOptIn(ctx context.Context, optIn domain.OptIn) error {
	m := models.WhatsAppOptIn{
		ID:        optIn.ID,
		OrgID:     optIn.OrgID,
		PartyID:   optIn.PartyID,
		Phone:     optIn.Phone,
		Status:    string(optIn.Status),
		Source:    string(optIn.Source),
		OptedInAt: optIn.OptedInAt,
		CreatedAt: optIn.CreatedAt,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"status", "source", "opted_in_at", "opted_out_at",
			}),
		}).Create(&m).Error
}

func (r *Repository) GetOptIn(ctx context.Context, orgID, partyID uuid.UUID) (domain.OptIn, error) {
	var m models.WhatsAppOptIn
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND party_id = ? AND status = 'opted_in'", orgID, partyID).
		Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.OptIn{}, ErrNotFound
		}
		return domain.OptIn{}, err
	}
	return optInToDomain(m), nil
}

func (r *Repository) OptOut(ctx context.Context, orgID, partyID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WhatsAppOptIn{}).
		Where("org_id = ? AND party_id = ? AND status = 'opted_in'", orgID, partyID).
		Updates(map[string]any{
			"status":      "opted_out",
			"opted_out_at": &now,
		}).Error
}

func (r *Repository) ListOptIns(ctx context.Context, orgID uuid.UUID) ([]domain.OptIn, error) {
	var rows []models.WhatsAppOptIn
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND status = 'opted_in'", orgID).
		Order("opted_in_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.OptIn, 0, len(rows))
	for _, m := range rows {
		out = append(out, optInToDomain(m))
	}
	return out, nil
}

func (r *Repository) IsOptedIn(ctx context.Context, orgID, partyID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.WhatsAppOptIn{}).
		Where("org_id = ? AND party_id = ? AND status = 'opted_in'", orgID, partyID).
		Count(&count).Error
	return count > 0, err
}

// --- Mappers ---

func connectionToDomain(m models.WhatsAppConnection) domain.Connection {
	return domain.Connection{
		OrgID:              m.OrgID,
		PhoneNumberID:      m.PhoneNumberID,
		WABAID:             m.WABAID,
		AccessToken:        m.AccessTokenEncrypt,
		DisplayPhoneNumber: m.DisplayPhoneNumber,
		VerifiedName:       m.VerifiedName,
		QualityRating:      m.QualityRating,
		MessagingLimit:     m.MessagingLimit,
		IsActive:           m.IsActive,
		ConnectedAt:        m.ConnectedAt,
		DisconnectedAt:     m.DisconnectedAt,
		CreatedAt:          m.CreatedAt,
	}
}

func messageToDomain(m models.WhatsAppMessage) domain.Message {
	var params []string
	_ = json.Unmarshal(m.TemplateParams, &params)
	var meta map[string]any
	_ = json.Unmarshal(m.Metadata, &meta)
	return domain.Message{
		ID:               m.ID,
		OrgID:            m.OrgID,
		PhoneNumberID:    m.PhoneNumberID,
		Direction:        domain.MessageDirection(m.Direction),
		WAMessageID:      m.WAMessageID,
		ToPhone:          m.ToPhone,
		FromPhone:        m.FromPhone,
		MessageType:      domain.MessageType(m.MessageType),
		Body:             m.Body,
		TemplateName:     m.TemplateName,
		TemplateLanguage: m.TemplateLanguage,
		TemplateParams:   params,
		MediaURL:         m.MediaURL,
		MediaMimeType:    m.MediaMimeType,
		MediaCaption:     m.MediaCaption,
		Status:           domain.MessageStatus(m.Status),
		ErrorCode:        m.ErrorCode,
		ErrorMessage:     m.ErrorMessage,
		PartyID:          m.PartyID,
		Metadata:         meta,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

func templateToDomain(m models.WhatsAppTemplate) domain.Template {
	var buttons []domain.TemplateButton
	_ = json.Unmarshal(m.Buttons, &buttons)
	var exParams []string
	_ = json.Unmarshal(m.ExampleParams, &exParams)
	return domain.Template{
		ID:              m.ID,
		OrgID:           m.OrgID,
		MetaTemplateID:  m.MetaTemplateID,
		Name:            m.Name,
		Language:        m.Language,
		Category:        domain.TemplateCategory(m.Category),
		Status:          domain.TemplateStatus(m.Status),
		HeaderType:      m.HeaderType,
		HeaderText:      m.HeaderText,
		BodyText:        m.BodyText,
		FooterText:      m.FooterText,
		Buttons:         buttons,
		ExampleParams:   exParams,
		RejectionReason: m.RejectionReason,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

func optInToDomain(m models.WhatsAppOptIn) domain.OptIn {
	return domain.OptIn{
		ID:         m.ID,
		OrgID:      m.OrgID,
		PartyID:    m.PartyID,
		Phone:      m.Phone,
		Status:     domain.OptInStatus(m.Status),
		Source:     domain.OptInSource(m.Source),
		OptedInAt:  m.OptedInAt,
		OptedOutAt: m.OptedOutAt,
		CreatedAt:  m.CreatedAt,
	}
}
