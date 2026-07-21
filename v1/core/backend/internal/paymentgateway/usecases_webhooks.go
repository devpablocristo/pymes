package paymentgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	auditdomain "github.com/devpablocristo/pymes/core/backend/internal/audit/usecases/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/paymentgateway/gateway"
	gatewaydomain "github.com/devpablocristo/pymes/core/backend/internal/paymentgateway/usecases/domain"
)

type webhookPayloadData struct {
	ID any `json:"id"`
}

type webhookPayload struct {
	Type   string             `json:"type"`
	Action string             `json:"action"`
	Data   webhookPayloadData `json:"data"`
}

func (u *Usecases) ProcessWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error {
	if normalizeProvider(provider) != providerMercadoPago {
		return ErrUnsupportedProvider
	}
	if !u.verifyMPSignature(headers, body) {
		return ErrInvalidWebhookSignature
	}

	var in webhookPayload
	if err := json.Unmarshal(body, &in); err != nil {
		return err
	}

	typ := strings.TrimSpace(strings.ToLower(in.Type))
	if typ == "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(in.Action)), "payment.") {
		typ = "payment"
	}
	if typ != "payment" {
		return nil
	}

	paymentID := anyToString(in.Data.ID)
	if paymentID == "" {
		return nil
	}

	return u.repo.StoreWebhookEvent(ctx, gatewaydomain.WebhookEvent{
		Provider:        providerMercadoPago,
		ExternalEventID: paymentID,
		EventType:       typ,
		RawPayload:      body,
		Signature:       strings.TrimSpace(headers.Get("X-Signature")),
	})
}

func (u *Usecases) ProcessPendingWebhookEvents(ctx context.Context, limit int) (int, error) {
	events, err := u.repo.LockPendingWebhookEvents(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, evt := range events {
		if err := u.processStoredWebhookEvent(ctx, evt); err != nil {
			if markErr := u.repo.MarkWebhookEventError(ctx, evt.ID, err.Error()); markErr != nil {
				return processed, markErr
			}
			continue
		}
		if err := u.repo.MarkWebhookEventProcessed(ctx, evt.ID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (u *Usecases) processStoredWebhookEvent(ctx context.Context, evt gatewaydomain.WebhookEvent) error {
	detail, err := u.fetchPaymentDetailAcrossConnections(ctx, evt.ExternalEventID)
	if err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(detail.Status)) != "approved" {
		return nil
	}

	orgID, refType, refID, err := parseExternalReference(detail.ExternalReference)
	if err != nil {
		return err
	}

	switch refType {
	case "sale":
		err := u.repo.ProcessApprovedSalePayment(ctx, ProcessSalePaymentInput{
			OrgID:      orgID,
			SaleID:        refID,
			Amount:        detail.TransactionAmount,
			ExternalPayID: detail.ID,
			ExternalPayer: detail.PayerEmail,
			PaidAt:        u.now(),
		})
		if err != nil {
			return err
		}
		u.logWebhookApproval(ctx, orgID, "sale", refID, evt.ExternalEventID, detail)
		return nil
	case "quote":
		err := u.repo.MarkPreferenceApproved(
			ctx,
			orgID,
			refType,
			refID,
			detail.PayerEmail,
			u.now(),
		)
		if err != nil {
			return err
		}
		u.logWebhookApproval(ctx, orgID, "quote", refID, evt.ExternalEventID, detail)
		return nil
	default:
		return nil
	}
}

func (u *Usecases) logWebhookApproval(
	ctx context.Context,
	orgID uuid.UUID,
	refType string,
	refID uuid.UUID,
	eventID string,
	detail gateway.PaymentDetail,
) {
	if u.audit == nil {
		return
	}

	actor := auditdomain.ActorRef{
		Raw:   mpWebhookServiceName,
		Type:  "service",
		Label: "Mercado Pago webhook",
	}
	if serviceID, err := u.repo.GetServiceIDByName(ctx, mpWebhookServiceName); err == nil && serviceID != uuid.Nil {
		actor.ID = &serviceID
	}

	u.audit.LogWithActor(ctx, auditdomain.LogInput{
		OrgID:     orgID,
		Actor:        actor,
		Action:       "payment_gateway.payment.approved",
		ResourceType: refType,
		ResourceID:   refID.String(),
		Payload: map[string]any{
			"provider":           providerMercadoPago,
			"reference_type":     refType,
			"payment_id":         strings.TrimSpace(detail.ID),
			"event_id":           strings.TrimSpace(eventID),
			"external_reference": strings.TrimSpace(detail.ExternalReference),
			"amount":             detail.TransactionAmount,
			"payer":              strings.TrimSpace(detail.PayerEmail),
			"status":             strings.TrimSpace(detail.Status),
		},
	})
}

func (u *Usecases) fetchPaymentDetailAcrossConnections(ctx context.Context, paymentID string) (gateway.PaymentDetail, error) {
	// Fast path: if we already stored external preference, use org connection directly.
	if pref, err := u.repo.GetPreferenceByExternalID(ctx, providerMercadoPago, paymentID); err == nil {
		if conn, accessToken, err := u.ensureConnectionAccessToken(ctx, pref.OrgID); err == nil {
			_ = conn
			if detail, err := u.mp.GetPaymentDetail(ctx, accessToken, paymentID); err == nil {
				return detail, nil
			}
		}
	}

	conns, err := u.repo.ListActiveConnections(ctx)
	if err != nil {
		return gateway.PaymentDetail{}, err
	}
	if len(conns) == 0 {
		return gateway.PaymentDetail{}, ErrGatewayNotConnected
	}

	var lastErr error
	for _, conn := range conns {
		accessToken, err := u.crypto.Decrypt(conn.AccessToken)
		if err != nil {
			lastErr = err
			continue
		}
		if conn.TokenExpiresAt.Before(u.now().Add(1 * time.Minute)) {
			_, accessToken, err = u.refreshConnection(ctx, conn)
			if err != nil {
				lastErr = err
				continue
			}
		}
		detail, err := u.mp.GetPaymentDetail(ctx, accessToken, paymentID)
		if err == nil && strings.TrimSpace(detail.ID) != "" {
			return detail, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = ErrNotFound
	}
	return gateway.PaymentDetail{}, lastErr
}
