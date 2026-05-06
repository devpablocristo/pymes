package paymentgateway

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway/gateway"
	gatewaydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway/usecases/domain"
)

func (u *Usecases) GetOrCreatePreference(
	ctx context.Context,
	orgID uuid.UUID,
	req CreatePreferenceRequest,
) (gatewaydomain.PaymentPreference, error) {
	refType := normalizeReferenceType(req.ReferenceType)
	if refType == "" || req.ReferenceID == uuid.Nil {
		return gatewaydomain.PaymentPreference{}, ErrInvalidReference
	}

	latest, err := u.repo.GetLatestPreference(ctx, orgID, refType, req.ReferenceID)
	if err == nil {
		switch latest.Status {
		case "pending":
			if latest.ExpiresAt.After(u.now()) && strings.TrimSpace(latest.PaymentURL) != "" {
				return latest, nil
			}
		case "approved":
			if strings.TrimSpace(latest.PaymentURL) != "" {
				return latest, nil
			}
		}
	} else if !errors.Is(err, ErrNotFound) {
		return gatewaydomain.PaymentPreference{}, err
	}

	return u.CreatePreference(ctx, orgID, req)
}

func (u *Usecases) CreatePreference(
	ctx context.Context,
	orgID uuid.UUID,
	req CreatePreferenceRequest,
) (gatewaydomain.PaymentPreference, error) {
	refType := normalizeReferenceType(req.ReferenceType)
	if refType == "" || req.ReferenceID == uuid.Nil {
		return gatewaydomain.PaymentPreference{}, ErrInvalidReference
	}

	if u.mode == "demo" {
		return u.createDemoPreference(ctx, orgID, refType, req.ReferenceID)
	}

	if err := u.validateMPConfig(); err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	if err := u.checkPlanForNewLink(ctx, orgID); err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	_, accessToken, err := u.ensureConnectionAccessToken(ctx, orgID)
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	amount, currency, description, err := u.resolveReference(ctx, orgID, refType, req.ReferenceID)
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	expiresAt := u.now().Add(mpPreferenceTTL).UTC()
	ref := fmt.Sprintf("%s:%s:%s", orgID.String(), refType, req.ReferenceID.String())

	out, err := u.mp.CreatePreference(ctx, accessToken, gateway.PreferenceInput{
		Title:            description,
		Amount:           amount,
		CurrencyID:       coalesce(currency, "ARS"),
		ExternalRef:      ref,
		NotificationURL:  u.buildWebhookURL("/v1/webhooks/mercadopago"),
		ExpirationDateTo: expiresAt,
		SuccessURL:       u.buildFrontendURL("/payment/success"),
		FailureURL:       u.buildFrontendURL("/payment/failure"),
		PendingURL:       u.buildFrontendURL("/payment/pending"),
	})
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	pref, err := u.repo.SavePreference(ctx, gatewaydomain.PaymentPreference{
		OrgID:         orgID,
		Provider:      providerMercadoPago,
		ExternalID:    out.ID,
		ReferenceType: refType,
		ReferenceID:   req.ReferenceID,
		Amount:        amount,
		Description:   description,
		PaymentURL:    out.PaymentURL,
		QRData:        out.QRData,
		Status:        "pending",
		ExpiresAt:     expiresAt,
	})
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}

	return pref, nil
}

func (u *Usecases) createDemoPreference(
	ctx context.Context,
	orgID uuid.UUID,
	refType string,
	refID uuid.UUID,
) (gatewaydomain.PaymentPreference, error) {
	amount, currency, description, err := u.resolveReference(ctx, orgID, refType, refID)
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}
	expiresAt := u.now().Add(mpPreferenceTTL).UTC()
	ref := fmt.Sprintf("%s:%s:%s", orgID.String(), refType, refID.String())
	query := url.Values{}
	query.Set("reference", ref)
	query.Set("amount", strconv.FormatFloat(amount, 'f', -1, 64))
	query.Set("currency", coalesce(currency, "ARS"))
	paymentURL := u.buildFrontendURL("/payment/demo?" + query.Encode())
	if strings.TrimSpace(paymentURL) == "" {
		paymentURL = "https://example.invalid/payment/demo?" + query.Encode()
	}
	return u.repo.SavePreference(ctx, gatewaydomain.PaymentPreference{
		OrgID:         orgID,
		Provider:      providerMercadoPago,
		ExternalID:    "demo:" + ref,
		ReferenceType: refType,
		ReferenceID:   refID,
		Amount:        amount,
		Description:   description,
		PaymentURL:    paymentURL,
		QRData:        "demo:" + ref,
		Status:        "pending",
		ExpiresAt:     expiresAt,
	})
}

func (u *Usecases) GetPreference(
	ctx context.Context,
	orgID uuid.UUID,
	refType string,
	refID uuid.UUID,
) (gatewaydomain.PaymentPreference, error) {
	norm := normalizeReferenceType(refType)
	if norm == "" || refID == uuid.Nil {
		return gatewaydomain.PaymentPreference{}, ErrInvalidReference
	}
	return u.repo.GetLatestPreference(ctx, orgID, norm, refID)
}

func (u *Usecases) GetPublicQuotePaymentLink(
	ctx context.Context,
	orgRef string,
	quoteID uuid.UUID,
) (gatewaydomain.PaymentPreference, error) {
	orgID, err := u.repo.ResolveOrgID(ctx, orgRef)
	if err != nil {
		return gatewaydomain.PaymentPreference{}, err
	}
	return u.GetOrCreatePreference(ctx, orgID, CreatePreferenceRequest{
		ReferenceType: "quote",
		ReferenceID:   quoteID,
	})
}

func (u *Usecases) checkPlanForNewLink(ctx context.Context, orgID uuid.UUID) error {
	plan := strings.ToLower(strings.TrimSpace(u.repo.GetPlanCode(ctx, orgID)))
	switch plan {
	case "enterprise":
		return nil
	case "growth":
		startOfMonth := time.Date(u.now().Year(), u.now().Month(), 1, 0, 0, 0, 0, time.UTC)
		count, err := u.repo.CountMonthlyPreferences(ctx, orgID, startOfMonth)
		if err != nil {
			return err
		}
		if count >= growthMonthlyLinksLimit {
			return ErrPlanMonthlyLimitReached
		}
		return nil
	case "starter":
		return ErrPlanRestricted
	default:
		return ErrPlanRestricted
	}
}
