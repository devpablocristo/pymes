package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stripe/stripe-go/v81"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/billing/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type RepositoryPort interface {
	GetTenantSettings(orgID uuid.UUID) TenantSettings
	UpdateBilling(orgID uuid.UUID, plan, status, subscriptionID, customerID, actor string) TenantSettings
	ResolveOrgByStripeIdentifiers(subscriptionID, customerID string) (uuid.UUID, bool)
}

type StripeClientPort interface {
	CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error)
	CreateCheckoutSession(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
	CreatePortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error)
	GetSubscription(subscriptionID string) (*stripe.Subscription, error)
	ConstructWebhookEvent(payload []byte, sigHeader, secret string) (stripe.Event, error)
}

type NotificationPort interface {
	Notify(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) error
}

type TenantSettings struct {
	OrgID                uuid.UUID
	PlanCode             string
	HardLimits           map[string]any
	BillingStatus        string
	UpdatedAt            time.Time
	StripeCustomerID     *string
	StripeSubscriptionID *string
}

type Usecases struct {
	repo          RepositoryPort
	stripe        StripeClientPort
	notifications NotificationPort
	frontendURL   string
	priceIDs      map[domain.PlanCode]string
	webhookSecret string
	logger        zerolog.Logger
}

func NewUsecases(repo RepositoryPort, stripeClient StripeClientPort, notifications NotificationPort, frontendURL string, priceIDs map[domain.PlanCode]string, webhookSecret string, logger zerolog.Logger) *Usecases {
	return &Usecases{
		repo:          repo,
		stripe:        stripeClient,
		notifications: notifications,
		frontendURL:   frontendURL,
		priceIDs:      priceIDs,
		webhookSecret: webhookSecret,
		logger:        logger,
	}
}

func (u *Usecases) GetBillingStatus(ctx context.Context, orgID string) (domain.BillingSummary, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return domain.BillingSummary{}, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	ts := u.repo.GetTenantSettings(id)
	return domain.BillingSummary{
		OrgID:            id,
		PlanCode:         domain.PlanCode(ts.PlanCode),
		Status:           domain.BillingStatus(ts.BillingStatus),
		HardLimits:       toHardLimits(ts.HardLimits),
		Usage:            map[string]any{"api_calls": 0, "storage_mb": 0, "users": 0},
		CurrentPeriodEnd: time.Now().UTC().AddDate(0, 1, 0),
	}, nil
}

func (u *Usecases) CreateCheckoutSession(ctx context.Context, orgID, planCode, successURL, cancelURL, actor string) (string, error) {
	if u.stripe == nil {
		return "", fmt.Errorf("stripe client not configured")
	}
	id, err := uuid.Parse(orgID)
	if err != nil {
		return "", fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	plan := normalizePlan(planCode)
	priceID := strings.TrimSpace(u.priceIDs[plan])
	if priceID == "" {
		return "", fmt.Errorf("price id not configured for %s", plan)
	}

	ts := u.repo.GetTenantSettings(id)
	customerID := ""
	if ts.StripeCustomerID != nil {
		customerID = *ts.StripeCustomerID
	}

	if customerID == "" {
		email := normalizeActorEmail(actor)
		customer, err := u.stripe.CreateCustomer(&stripe.CustomerParams{Email: stripe.String(email)})
		if err != nil {
			return "", fmt.Errorf("create stripe customer: %w", err)
		}
		customerID = customer.ID
	}

	u.repo.UpdateBilling(id, string(plan), string(domain.BillingTrialing), "", customerID, actor)

	params := &stripe.CheckoutSessionParams{
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:   stripe.String(customerID),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(priceID), Quantity: stripe.Int64(1)},
		},
	}
	params.AddMetadata("org_id", id.String())
	params.AddMetadata("plan_code", string(plan))

	session, err := u.stripe.CreateCheckoutSession(params)
	if err != nil {
		return "", fmt.Errorf("create checkout session: %w", err)
	}
	if strings.TrimSpace(session.URL) == "" {
		return "", fmt.Errorf("stripe checkout session has empty url")
	}
	return session.URL, nil
}

func (u *Usecases) CreatePortalSession(ctx context.Context, orgID, returnURL, actor string) (string, error) {
	_ = ctx
	if u.stripe == nil {
		return "", fmt.Errorf("stripe client not configured")
	}
	id, err := uuid.Parse(orgID)
	if err != nil {
		return "", fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	ts := u.repo.GetTenantSettings(id)
	if ts.StripeCustomerID == nil || *ts.StripeCustomerID == "" {
		return "", fmt.Errorf("stripe customer not found for org: %w", httperrors.ErrNotFound)
	}

	portal, err := u.stripe.CreatePortalSession(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*ts.StripeCustomerID),
		ReturnURL: stripe.String(returnURL),
	})
	if err != nil {
		return "", fmt.Errorf("create portal session: %w", err)
	}
	if strings.TrimSpace(portal.URL) == "" {
		return "", fmt.Errorf("stripe portal session has empty url")
	}
	_ = actor
	return portal.URL, nil
}

func (u *Usecases) ConstructWebhookEvent(payload []byte, sigHeader string) (stripe.Event, error) {
	if u.stripe == nil {
		return stripe.Event{}, fmt.Errorf("stripe client not configured")
	}
	return u.stripe.ConstructWebhookEvent(payload, sigHeader, u.webhookSecret)
}

func (u *Usecases) HandleWebhookEvent(ctx context.Context, event stripe.Event) error {
	switch string(event.Type) {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return fmt.Errorf("decode checkout session: %w", err)
		}
		orgIDRaw := session.Metadata["org_id"]
		if strings.TrimSpace(orgIDRaw) == "" {
			return fmt.Errorf("missing org_id metadata")
		}
		orgID, err := uuid.Parse(orgIDRaw)
		if err != nil {
			return fmt.Errorf("invalid org_id metadata")
		}
		plan := normalizePlan(session.Metadata["plan_code"])
		subID := ""
		if session.Subscription != nil {
			subID = session.Subscription.ID
		}
		customerID := ""
		if session.Customer != nil {
			customerID = session.Customer.ID
		}
		u.repo.UpdateBilling(orgID, string(plan), string(domain.BillingActive), subID, customerID, "stripe:webhook")
		u.notifySync(ctx, orgID, "plan_upgraded", map[string]string{"plan_code": string(plan)})

	case "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("decode subscription: %w", err)
		}
		customerID := ""
		if sub.Customer != nil {
			customerID = sub.Customer.ID
		}
		orgID, ok := u.repo.ResolveOrgByStripeIdentifiers(sub.ID, customerID)
		if !ok {
			return fmt.Errorf("org not found for subscription/customer")
		}
		plan := u.planFromSubscription(&sub)
		status := mapSubscriptionStatus(sub.Status)
		u.repo.UpdateBilling(orgID, string(plan), status, sub.ID, customerID, "stripe:webhook")

	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("decode subscription: %w", err)
		}
		customerID := ""
		if sub.Customer != nil {
			customerID = sub.Customer.ID
		}
		orgID, ok := u.repo.ResolveOrgByStripeIdentifiers(sub.ID, customerID)
		if !ok {
			return fmt.Errorf("org not found for subscription/customer")
		}
		u.repo.UpdateBilling(orgID, string(domain.PlanStarter), string(domain.BillingCanceled), "", customerID, "stripe:webhook")
		u.notifySync(ctx, orgID, "subscription_canceled", map[string]string{})

	case "invoice.payment_succeeded":
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return fmt.Errorf("decode invoice: %w", err)
		}
		subID, customerID := invoiceRefs(&invoice)
		orgID, ok := u.repo.ResolveOrgByStripeIdentifiers(subID, customerID)
		if !ok {
			return fmt.Errorf("org not found for invoice")
		}
		u.repo.UpdateBilling(orgID, "", string(domain.BillingActive), subID, customerID, "stripe:webhook")

	case "invoice.payment_failed":
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return fmt.Errorf("decode invoice: %w", err)
		}
		subID, customerID := invoiceRefs(&invoice)
		orgID, ok := u.repo.ResolveOrgByStripeIdentifiers(subID, customerID)
		if !ok {
			return fmt.Errorf("org not found for invoice")
		}
		u.repo.UpdateBilling(orgID, "", string(domain.BillingPastDue), subID, customerID, "stripe:webhook")
		u.notifySync(ctx, orgID, "payment_failed", map[string]string{})

	default:
		u.logger.Info().Str("event_type", string(event.Type)).Msg("ignored stripe webhook event")
	}
	return nil
}

func (u *Usecases) notifySync(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) {
	if u.notifications == nil {
		return
	}
	if err := u.notifications.Notify(ctx, orgID, notifType, data); err != nil {
		u.logger.Error().Err(err).Str("org_id", orgID.String()).Str("notif_type", notifType).Msg("notification failed")
	}
}

func (u *Usecases) planFromSubscription(sub *stripe.Subscription) domain.PlanCode {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 {
		return domain.PlanStarter
	}
	priceID := sub.Items.Data[0].Price.ID
	for plan, configured := range u.priceIDs {
		if configured == priceID {
			return plan
		}
	}
	return domain.PlanStarter
}

func normalizePlan(plan string) domain.PlanCode {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case string(domain.PlanGrowth):
		return domain.PlanGrowth
	case string(domain.PlanEnterprise):
		return domain.PlanEnterprise
	default:
		return domain.PlanStarter
	}
}

func toHardLimits(in map[string]any) domain.HardLimits {
	if in == nil {
		return domain.HardLimits{}
	}
	return domain.HardLimits{
		UsersMax:    in["users_max"],
		StorageMB:   in["storage_mb"],
		APICallsRPM: in["api_calls_rpm"],
	}
}

func normalizeActorEmail(actor string) string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return "no-reply@pymes.local"
	}
	if strings.Contains(actor, "@") {
		return actor
	}
	return actor + "@pymes.local"
}

func invoiceRefs(inv *stripe.Invoice) (subscriptionID, customerID string) {
	if inv == nil {
		return "", ""
	}
	if inv.Subscription != nil {
		subscriptionID = inv.Subscription.ID
	}
	if inv.Customer != nil {
		customerID = inv.Customer.ID
	}
	return subscriptionID, customerID
}

func mapSubscriptionStatus(status stripe.SubscriptionStatus) string {
	switch status {
	case stripe.SubscriptionStatusActive:
		return string(domain.BillingActive)
	case stripe.SubscriptionStatusPastDue:
		return string(domain.BillingPastDue)
	case stripe.SubscriptionStatusCanceled:
		return string(domain.BillingCanceled)
	case stripe.SubscriptionStatusTrialing:
		return string(domain.BillingTrialing)
	default:
		return string(domain.BillingActive)
	}
}
