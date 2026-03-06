package paymentgateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway/repository/models"
	gatewaydomain "github.com/devpablocristo/pymes/control-plane/backend/internal/paymentgateway/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/pagination"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrGatewayNotConnected = errors.New("payment gateway not connected")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ResolveOrgID(ctx context.Context, ref string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return uuid.Nil, ErrNotFound
	}
	if id, err := uuid.Parse(trimmed); err == nil {
		var exists uuid.UUID
		err = r.db.WithContext(ctx).Table("orgs").Select("id").Where("id = ?", id).Take(&exists).Error
		if err == nil {
			return id, nil
		}
	}
	var row struct {
		ID uuid.UUID
	}
	err := r.db.WithContext(ctx).Table("orgs").Select("id").Where("slug = ?", trimmed).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}
	return row.ID, nil
}

func (r *Repository) GetPlanCode(ctx context.Context, orgID uuid.UUID) string {
	var plan string
	if err := r.db.WithContext(ctx).Table("tenant_settings").Select("plan_code").Where("org_id = ?", orgID).Take(&plan).Error; err != nil {
		return "starter"
	}
	plan = strings.TrimSpace(strings.ToLower(plan))
	if plan == "" {
		return "starter"
	}
	return plan
}

func (r *Repository) GetBankInfo(ctx context.Context, orgID uuid.UUID) (gatewaydomain.BankInfo, bool, error) {
	var row struct {
		BankHolder string `gorm:"column:bank_holder"`
		BankCBU    string `gorm:"column:bank_cbu"`
		BankAlias  string `gorm:"column:bank_alias"`
		BankName   string `gorm:"column:bank_name"`
	}
	if err := r.db.WithContext(ctx).
		Table("tenant_settings").
		Select("bank_holder, bank_cbu, bank_alias, bank_name").
		Where("org_id = ?", orgID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gatewaydomain.BankInfo{}, false, nil
		}
		return gatewaydomain.BankInfo{}, false, err
	}
	info := gatewaydomain.BankInfo{
		Holder: strings.TrimSpace(row.BankHolder),
		CBU:    strings.TrimSpace(row.BankCBU),
		Alias:  strings.TrimSpace(row.BankAlias),
		Name:   strings.TrimSpace(row.BankName),
	}
	hasAny := info.Holder != "" || info.CBU != "" || info.Alias != "" || info.Name != ""
	return info, hasAny, nil
}

func (r *Repository) GetWhatsAppTransferTemplate(ctx context.Context, orgID uuid.UUID) string {
	var tpl string
	err := r.db.WithContext(ctx).
		Table("tenant_settings").
		Select("wa_payment_template").
		Where("org_id = ?", orgID).
		Take(&tpl).Error
	if err != nil || strings.TrimSpace(tpl) == "" {
		return "Podes transferir a:\nAlias: {bank_alias}\nCBU: {bank_cbu}\nTitular: {bank_holder}\nBanco: {bank_name}\nMonto: {total}"
	}
	return tpl
}

func (r *Repository) GetWhatsAppLinkTemplate(ctx context.Context, orgID uuid.UUID) string {
	var tpl string
	err := r.db.WithContext(ctx).
		Table("tenant_settings").
		Select("wa_payment_link_template").
		Where("org_id = ?", orgID).
		Take(&tpl).Error
	if err != nil || strings.TrimSpace(tpl) == "" {
		return "Hola {party_name}, podes pagar {total} de tu compra {number} con este link: {payment_url}"
	}
	return tpl
}

func (r *Repository) GetConnection(ctx context.Context, orgID uuid.UUID) (gatewaydomain.PaymentGatewayConnection, error) {
	var row models.PaymentGatewayConnectionModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND is_active = true", orgID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gatewaydomain.PaymentGatewayConnection{}, ErrGatewayNotConnected
		}
		return gatewaydomain.PaymentGatewayConnection{}, err
	}
	return toConnectionDomain(row), nil
}

func (r *Repository) GetConnectionByExternalUserID(ctx context.Context, externalUserID string) (gatewaydomain.PaymentGatewayConnection, error) {
	var row models.PaymentGatewayConnectionModel
	err := r.db.WithContext(ctx).
		Where("external_user_id = ? AND is_active = true", strings.TrimSpace(externalUserID)).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gatewaydomain.PaymentGatewayConnection{}, ErrGatewayNotConnected
		}
		return gatewaydomain.PaymentGatewayConnection{}, err
	}
	return toConnectionDomain(row), nil
}

func (r *Repository) ListActiveConnections(ctx context.Context) ([]gatewaydomain.PaymentGatewayConnection, error) {
	var rows []models.PaymentGatewayConnectionModel
	if err := r.db.WithContext(ctx).Where("is_active = true").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]gatewaydomain.PaymentGatewayConnection, 0, len(rows))
	for _, row := range rows {
		out = append(out, toConnectionDomain(row))
	}
	return out, nil
}

func (r *Repository) SaveConnection(ctx context.Context, in gatewaydomain.PaymentGatewayConnection) error {
	now := time.Now().UTC()
	row := models.PaymentGatewayConnectionModel{
		OrgID:                 in.OrgID,
		Provider:              coalesce(in.Provider, "mercadopago"),
		ExternalUserID:        strings.TrimSpace(in.ExternalUserID),
		AccessTokenEncrypted:  strings.TrimSpace(in.AccessToken),
		RefreshTokenEncrypted: strings.TrimSpace(in.RefreshToken),
		TokenExpiresAt:        in.TokenExpiresAt.UTC(),
		IsActive:              true,
		ConnectedAt:           now,
		UpdatedAt:             now,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "org_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"provider":                row.Provider,
				"external_user_id":        row.ExternalUserID,
				"access_token_encrypted":  row.AccessTokenEncrypted,
				"refresh_token_encrypted": row.RefreshTokenEncrypted,
				"token_expires_at":        row.TokenExpiresAt,
				"is_active":               true,
				"updated_at":              now,
			}),
		}).
		Create(&row).Error
}

func (r *Repository) Disconnect(ctx context.Context, orgID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.PaymentGatewayConnectionModel{}).
		Where("org_id = ?", orgID).
		Updates(map[string]any{"is_active": false, "updated_at": gorm.Expr("now()")}).Error
}

func (r *Repository) CountMonthlyPreferences(ctx context.Context, orgID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Table("payment_preferences").
		Where("org_id = ? AND created_at >= ?", orgID, since.UTC()).
		Count(&n).Error
	return n, err
}

func (r *Repository) SavePreference(ctx context.Context, in gatewaydomain.PaymentPreference) (gatewaydomain.PaymentPreference, error) {
	row := models.PaymentPreferenceModel{
		ID:              uuid.New(),
		OrgID:           in.OrgID,
		Provider:        coalesce(in.Provider, "mercadopago"),
		ExternalID:      strings.TrimSpace(in.ExternalID),
		ReferenceType:   strings.TrimSpace(in.ReferenceType),
		ReferenceID:     in.ReferenceID,
		Amount:          in.Amount,
		Description:     strings.TrimSpace(in.Description),
		PaymentURL:      strings.TrimSpace(in.PaymentURL),
		QRData:          strings.TrimSpace(in.QRData),
		Status:          coalesce(in.Status, "pending"),
		ExternalPayerID: strings.TrimSpace(in.ExternalPayerID),
		PaidAt:          in.PaidAt,
		ExpiresAt:       in.ExpiresAt.UTC(),
		CreatedAt:       time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}
	return toPreferenceDomain(row), nil
}

func (r *Repository) GetLatestPreference(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID) (gatewaydomain.PaymentPreference, error) {
	var row models.PaymentPreferenceModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND reference_type = ? AND reference_id = ?", orgID, strings.TrimSpace(refType), refID).
		Order("created_at DESC").
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gatewaydomain.PaymentPreference{}, ErrNotFound
		}
		return gatewaydomain.PaymentPreference{}, err
	}
	return toPreferenceDomain(row), nil
}

func (r *Repository) GetPreferenceByExternalID(ctx context.Context, provider, externalID string) (gatewaydomain.PaymentPreference, error) {
	var row models.PaymentPreferenceModel
	err := r.db.WithContext(ctx).
		Where("provider = ? AND external_id = ?", strings.TrimSpace(provider), strings.TrimSpace(externalID)).
		Order("created_at DESC").
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gatewaydomain.PaymentPreference{}, ErrNotFound
		}
		return gatewaydomain.PaymentPreference{}, err
	}
	return toPreferenceDomain(row), nil
}

func (r *Repository) GetSaleSnapshot(ctx context.Context, orgID, saleID uuid.UUID) (gatewaydomain.SaleSnapshot, error) {
	var row struct {
		ID            uuid.UUID
		Number        string
		CustomerName  string
		CustomerPhone string
		Total         float64
		Currency      string
	}
	err := r.db.WithContext(ctx).
		Table("sales s").
		Select(`
			s.id,
			s.number,
			COALESCE(s.party_name, '') AS customer_name,
			COALESCE(c.phone, '') AS customer_phone,
			s.total,
			s.currency
		`).
		Joins("LEFT JOIN parties c ON c.id = s.party_id").
		Where("s.org_id = ? AND s.id = ?", orgID, saleID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gatewaydomain.SaleSnapshot{}, ErrNotFound
		}
		return gatewaydomain.SaleSnapshot{}, err
	}
	return gatewaydomain.SaleSnapshot(row), nil
}

func (r *Repository) GetQuoteSnapshot(ctx context.Context, orgID, quoteID uuid.UUID) (gatewaydomain.QuoteSnapshot, error) {
	var row struct {
		ID           uuid.UUID
		Number       string
		CustomerName string
		Total        float64
		Currency     string
	}
	err := r.db.WithContext(ctx).
		Table("quotes").
		Select("id, number, COALESCE(party_name, '') AS customer_name, total, currency").
		Where("org_id = ? AND id = ?", orgID, quoteID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gatewaydomain.QuoteSnapshot{}, ErrNotFound
		}
		return gatewaydomain.QuoteSnapshot{}, err
	}
	return gatewaydomain.QuoteSnapshot(row), nil
}

type ProcessSalePaymentInput struct {
	OrgID         uuid.UUID
	SaleID        uuid.UUID
	Amount        float64
	ExternalPayID string
	ExternalPayer string
	PaidAt        time.Time
}

func (r *Repository) ProcessApprovedSalePayment(ctx context.Context, in ProcessSalePaymentInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pref models.PaymentPreferenceModel
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("org_id = ? AND reference_type = 'sale' AND reference_id = ?", in.OrgID, in.SaleID).
			Order("created_at DESC").
			Take(&pref).Error; err != nil {
			return err
		}
		if pref.Status == "approved" {
			return nil
		}

		var sale struct {
			Number        string
			Total         float64
			AmountPaid    float64 `gorm:"column:amount_paid"`
			Currency      string
			PaymentMethod string `gorm:"column:payment_method"`
		}
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Table("sales").
			Select("number, total, amount_paid, currency, payment_method").
			Where("org_id = ? AND id = ?", in.OrgID, in.SaleID).
			Take(&sale).Error; err != nil {
			return err
		}

		pending := sale.Total - sale.AmountPaid
		if pending < 0 {
			pending = 0
		}
		applied := in.Amount
		if applied > pending {
			applied = pending
		}
		if applied < 0 {
			applied = 0
		}

		now := time.Now().UTC()
		paidAt := in.PaidAt.UTC()
		if paidAt.IsZero() {
			paidAt = now
		}

		if err := tx.Model(&models.PaymentPreferenceModel{}).
			Where("id = ?", pref.ID).
			Updates(map[string]any{
				"status":            "approved",
				"external_payer_id": strings.TrimSpace(in.ExternalPayer),
				"paid_at":           &paidAt,
			}).Error; err != nil {
			return err
		}

		if applied <= 0 {
			return nil
		}

		note := fmt.Sprintf("Pago MP #%s", strings.TrimSpace(in.ExternalPayID))

		var existing int64
		if err := tx.Table("payments").
			Where("org_id = ? AND reference_type = 'sale' AND reference_id = ? AND method = 'mercadopago' AND notes = ?", in.OrgID, in.SaleID, note).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return nil
		}

		if err := tx.Exec(`
			INSERT INTO payments (id, org_id, reference_type, reference_id, method, amount, notes, received_at, created_by, created_at)
			VALUES (gen_random_uuid(), ?, 'sale', ?, 'mercadopago', ?, ?, ?, 'payment-gateway:webhook', now())
		`, in.OrgID, in.SaleID, applied, note, paidAt).Error; err != nil {
			return err
		}

		if err := tx.Exec(`
			UPDATE sales
			SET amount_paid = amount_paid + ?,
			    payment_status = CASE
			        WHEN amount_paid + ? >= total THEN 'paid'
			        WHEN amount_paid + ? > 0 THEN 'partial'
			        ELSE 'pending'
			    END,
			    payment_method = CASE
			        WHEN payment_method = 'cash' THEN 'mixed'
			        ELSE payment_method
			    END
			WHERE org_id = ? AND id = ?
		`, applied, applied, applied, in.OrgID, in.SaleID).Error; err != nil {
			return err
		}

		if err := tx.Exec(`
			INSERT INTO cash_movements (
				id, org_id, type, amount, currency, category, description,
				payment_method, reference_type, reference_id, created_by, created_at
			) VALUES (
				gen_random_uuid(), ?, 'income', ?, ?, 'sale', ?,
				'mercadopago', 'sale', ?, 'payment-gateway:webhook', now()
			)
		`, in.OrgID, applied, coalesce(sale.Currency, "ARS"), note, in.SaleID).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *Repository) MarkPreferenceApproved(
	ctx context.Context,
	orgID uuid.UUID,
	refType string,
	refID uuid.UUID,
	payerID string,
	paidAt time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pref models.PaymentPreferenceModel
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("org_id = ? AND reference_type = ? AND reference_id = ?", orgID, strings.TrimSpace(refType), refID).
			Order("created_at DESC").
			Take(&pref).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if pref.Status == "approved" {
			return nil
		}

		update := map[string]any{
			"status":            "approved",
			"external_payer_id": strings.TrimSpace(payerID),
		}
		if !paidAt.IsZero() {
			update["paid_at"] = paidAt.UTC()
		}

		return tx.Model(&models.PaymentPreferenceModel{}).
			Where("id = ?", pref.ID).
			Updates(update).Error
	})
}

func (r *Repository) StoreWebhookEvent(ctx context.Context, in gatewaydomain.WebhookEvent) error {
	row := models.PaymentGatewayEventModel{
		ID:              coalesceUUID(in.ID),
		Provider:        strings.TrimSpace(in.Provider),
		ExternalEventID: strings.TrimSpace(in.ExternalEventID),
		EventType:       strings.TrimSpace(in.EventType),
		RawPayload:      append([]byte(nil), in.RawPayload...),
		Signature:       strings.TrimSpace(in.Signature),
		CreatedAt:       time.Now().UTC(),
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "external_event_id"}},
			DoNothing: true,
		}).
		Create(&row).Error
}

func (r *Repository) LockPendingWebhookEvents(ctx context.Context, limit int) ([]gatewaydomain.WebhookEvent, error) {
	limit = pagination.NormalizeLimit(limit, 50, 200)
	var rows []models.PaymentGatewayEventModel
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("processed_at IS NULL").
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]gatewaydomain.WebhookEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, gatewaydomain.WebhookEvent{
			ID:              row.ID,
			Provider:        row.Provider,
			ExternalEventID: row.ExternalEventID,
			EventType:       row.EventType,
			RawPayload:      append([]byte(nil), row.RawPayload...),
			Signature:       row.Signature,
			ProcessedAt:     row.ProcessedAt,
			ErrorMessage:    row.ErrorMessage,
			CreatedAt:       row.CreatedAt,
		})
	}
	return out, nil
}

func (r *Repository) MarkWebhookEventProcessed(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.PaymentGatewayEventModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"processed_at":  time.Now().UTC(),
			"error_message": "",
		}).Error
}

func (r *Repository) MarkWebhookEventError(ctx context.Context, id uuid.UUID, errorMessage string) error {
	return r.db.WithContext(ctx).
		Model(&models.PaymentGatewayEventModel{}).
		Where("id = ?", id).
		Update("error_message", strings.TrimSpace(errorMessage)).Error
}

func toConnectionDomain(in models.PaymentGatewayConnectionModel) gatewaydomain.PaymentGatewayConnection {
	return gatewaydomain.PaymentGatewayConnection{
		OrgID:          in.OrgID,
		Provider:       in.Provider,
		ExternalUserID: in.ExternalUserID,
		AccessToken:    in.AccessTokenEncrypted,
		RefreshToken:   in.RefreshTokenEncrypted,
		TokenExpiresAt: in.TokenExpiresAt,
		IsActive:       in.IsActive,
		ConnectedAt:    in.ConnectedAt,
		UpdatedAt:      in.UpdatedAt,
	}
}

func toPreferenceDomain(in models.PaymentPreferenceModel) gatewaydomain.PaymentPreference {
	return gatewaydomain.PaymentPreference{
		ID:              in.ID,
		OrgID:           in.OrgID,
		Provider:        in.Provider,
		ExternalID:      in.ExternalID,
		ReferenceType:   in.ReferenceType,
		ReferenceID:     in.ReferenceID,
		Amount:          in.Amount,
		Description:     in.Description,
		PaymentURL:      in.PaymentURL,
		QRData:          in.QRData,
		Status:          in.Status,
		ExternalPayerID: in.ExternalPayerID,
		PaidAt:          in.PaidAt,
		ExpiresAt:       in.ExpiresAt,
		CreatedAt:       in.CreatedAt,
	}
}

func coalesce(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

func coalesceUUID(id uuid.UUID) uuid.UUID {
	if id == uuid.Nil {
		return uuid.New()
	}
	return id
}
