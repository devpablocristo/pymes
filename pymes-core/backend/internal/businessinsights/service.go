package businessinsights

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	corecandidates "github.com/devpablocristo/core/notifications/go/candidates"
	candidatesdomain "github.com/devpablocristo/core/notifications/go/candidates/usecases/domain"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications"
	inventorydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/inventory/usecases/domain"
	paymentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/usecases/domain"
	saledomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/sales/usecases/domain"
)

type Config struct {
	FeaturedSaleThreshold    float64
	FeaturedPaymentThreshold float64
	LowStockDedupWindow      time.Duration
}

type CandidateUpsert = candidatesdomain.UpsertInput
type CandidateRecord = candidatesdomain.Candidate

type CandidateRepository interface {
	Upsert(ctx context.Context, in CandidateUpsert) (CandidateRecord, bool, error)
	MarkNotified(ctx context.Context, tenantID, candidateID string, notifiedAt time.Time) error
}

type Service struct {
	candidates *corecandidates.Usecases
	inbox      *inappnotifications.Usecases
	config     Config
}

func NewService(repo CandidateRepository, inbox *inappnotifications.Usecases, cfg Config) *Service {
	if cfg.FeaturedSaleThreshold <= 0 {
		cfg.FeaturedSaleThreshold = 75000
	}
	if cfg.FeaturedPaymentThreshold <= 0 {
		cfg.FeaturedPaymentThreshold = 50000
	}
	if cfg.LowStockDedupWindow <= 0 {
		cfg.LowStockDedupWindow = 6 * time.Hour
	}
	return &Service{candidates: corecandidates.NewWriteUsecases(repo, repo), inbox: inbox, config: cfg}
}

func (s *Service) NotifySaleCreated(ctx context.Context, sale saledomain.Sale) error {
	if s == nil || s.inbox == nil {
		return nil
	}
	if sale.Total < s.config.FeaturedSaleThreshold && len(sale.Items) < 3 {
		return nil
	}

	body := fmt.Sprintf(
		"Se registró la venta %s por %s con %d ítems. Conviene revisar si este movimiento cambia prioridades comerciales de la semana.",
		nonEmpty(sale.Number, "sin número"),
		formatCurrency(sale.Total, sale.Currency),
		len(sale.Items),
	)
	return s.recordAndNotify(ctx, candidateInput{
		orgID:       sale.OrgID,
		actor:       sale.CreatedBy,
		fingerprint: stableID("sale.created", sale.ID.String()),
		title:       "Venta destacada registrada",
		body:        body,
		entityType:  "sale",
		entityID:    sale.ID.String(),
		eventType:   "sale.created",
		severity:    "info",
		chatContext: map[string]any{
			"scope":                  "sales_collections",
			"routed_agent":           "sales",
			"content_language":       "es",
			"event_type":             "sale.created",
			"sale_id":                sale.ID.String(),
			"suggested_user_message": "Analizá esta venta destacada y decime si cambia mis prioridades comerciales de la semana.",
		},
		evidence: map[string]any{
			"sale_id":        sale.ID.String(),
			"sale_number":    sale.Number,
			"total":          sale.Total,
			"currency":       sale.Currency,
			"items_count":    len(sale.Items),
			"payment_method": sale.PaymentMethod,
		},
	})
}

func (s *Service) NotifyPaymentCreated(ctx context.Context, orgID, saleID uuid.UUID, payment paymentsdomain.Payment) error {
	if s == nil || s.inbox == nil {
		return nil
	}
	if payment.Amount < s.config.FeaturedPaymentThreshold {
		return nil
	}

	body := fmt.Sprintf(
		"Se registró un cobro por %s vía %s. Vale la pena revisar el impacto inmediato en caja y cobranzas pendientes.",
		formatCurrency(payment.Amount, ""),
		humanPaymentMethod(payment.Method),
	)
	return s.recordAndNotify(ctx, candidateInput{
		orgID:       orgID,
		actor:       payment.CreatedBy,
		fingerprint: stableID("payment.created", payment.ID.String()),
		title:       "Cobro destacado registrado",
		body:        body,
		entityType:  "sale_payment",
		entityID:    payment.ID.String(),
		eventType:   "payment.created",
		severity:    "info",
		chatContext: map[string]any{
			"scope":                  "sales_collections",
			"routed_agent":           "collections",
			"content_language":       "es",
			"event_type":             "payment.created",
			"sale_id":                saleID.String(),
			"suggested_user_message": "Explicame este cobro destacado y cómo impacta en la caja del negocio.",
		},
		evidence: map[string]any{
			"payment_id": payment.ID.String(),
			"sale_id":    saleID.String(),
			"amount":     payment.Amount,
			"method":     payment.Method,
		},
	})
}

func (s *Service) NotifyInventoryAdjusted(ctx context.Context, level inventorydomain.StockLevel, delta float64, actor, notes string) error {
	if s == nil || s.inbox == nil {
		return nil
	}
	if !level.IsLowStock {
		return nil
	}

	body := fmt.Sprintf(
		"%s quedó con stock crítico: %s disponibles sobre un mínimo de %s. Revisá reposición y riesgo de quiebre.",
		nonEmpty(level.ProductName, "El producto"),
		formatNumber(level.Quantity),
		formatNumber(level.MinQuantity),
	)
	return s.recordAndNotify(ctx, candidateInput{
		orgID:       level.OrgID,
		actor:       actor,
		fingerprint: bucketedID("inventory.low_stock", level.ProductID.String(), s.config.LowStockDedupWindow, time.Now().UTC()),
		title:       "Stock crítico tras ajuste",
		body:        body,
		entityType:  "inventory",
		entityID:    level.ProductID.String(),
		eventType:   "inventory.adjusted",
		severity:    "warning",
		chatContext: map[string]any{
			"scope":                  "inventory_profit",
			"routed_agent":           "products",
			"content_language":       "es",
			"event_type":             "inventory.adjusted",
			"delta":                  delta,
			"notes":                  strings.TrimSpace(notes),
			"suggested_user_message": "Decime si este stock crítico requiere reposición urgente y qué riesgo comercial tiene.",
		},
		evidence: map[string]any{
			"product_id":   level.ProductID.String(),
			"product_name": level.ProductName,
			"quantity":     level.Quantity,
			"min_quantity": level.MinQuantity,
			"is_low_stock": level.IsLowStock,
			"delta":        delta,
			"adjust_notes": strings.TrimSpace(notes),
			"dedup_window": s.config.LowStockDedupWindow.Seconds(),
		},
	})
}

type candidateInput struct {
	orgID       uuid.UUID
	actor       string
	fingerprint string
	title       string
	body        string
	entityType  string
	entityID    string
	eventType   string
	severity    string
	chatContext map[string]any
	evidence    map[string]any
}

func (s *Service) recordAndNotify(ctx context.Context, in candidateInput) error {
	if s == nil || s.inbox == nil || s.candidates == nil {
		return nil
	}
	record, shouldNotify, err := s.candidates.Record(ctx, CandidateUpsert{
		TenantID:    in.orgID.String(),
		Kind:        "insight",
		EventType:   in.eventType,
		EntityType:  in.entityType,
		EntityID:    in.entityID,
		Fingerprint: in.fingerprint,
		Severity:    in.severity,
		Title:       in.title,
		Body:        in.body,
		Evidence:    in.evidence,
		Actor:       in.actor,
		Now:         time.Now().UTC(),
	})
	if err != nil || !shouldNotify {
		return err
	}

	in.chatContext["candidate_id"] = record.ID
	err = s.createNotification(ctx, record, in.actor, in.chatContext)
	if err != nil {
		return err
	}
	return s.candidates.MarkNotified(ctx, record.TenantID, record.ID)
}

func (s *Service) createNotification(ctx context.Context, record CandidateRecord, actor string, chatContext map[string]any) error {
	if strings.TrimSpace(actor) == "" {
		actor = "api_key:" + strings.TrimSpace(record.TenantID)
	}
	raw, err := json.Marshal(chatContext)
	if err != nil {
		return err
	}
	_, err = s.inbox.CreateForActor(ctx, record.TenantID, actor, inappnotifications.CreateInput{
		ID:          record.ID,
		Title:       record.Title,
		Body:        record.Body,
		Kind:        record.Kind,
		EntityType:  record.EntityType,
		EntityID:    record.EntityID,
		ChatContext: raw,
	})
	return err
}

func stableID(eventType string, entityID string) string {
	return eventType + ":" + strings.TrimSpace(entityID)
}

func bucketedID(eventType string, entityID string, window time.Duration, now time.Time) string {
	seconds := int64(window / time.Second)
	if seconds <= 0 {
		seconds = int64((6 * time.Hour) / time.Second)
	}
	bucket := int64(math.Floor(float64(now.UTC().Unix()) / float64(seconds)))
	return fmt.Sprintf("%s:%s:%d", eventType, strings.TrimSpace(entityID), bucket)
}

func formatCurrency(amount float64, currency string) string {
	if strings.TrimSpace(currency) == "" {
		return fmt.Sprintf("$%.2f", amount)
	}
	return fmt.Sprintf("%s %.2f", strings.ToUpper(strings.TrimSpace(currency)), amount)
}

func formatNumber(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.2f", v)
}

func nonEmpty(v string, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func humanPaymentMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "cash":
		return "efectivo"
	case "card":
		return "tarjeta"
	case "transfer":
		return "transferencia"
	case "check":
		return "cheque"
	case "mercadopago":
		return "Mercado Pago"
	default:
		return "otro medio"
	}
}
