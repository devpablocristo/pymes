package billing

import (
	"fmt"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/client"
	stripewebhook "github.com/stripe/stripe-go/v81/webhook"
)

type StripeClient struct {
	api *client.API
}

func NewStripeClient(secretKey string) *StripeClient {
	sc := &StripeClient{}
	if secretKey != "" {
		sc.api = &client.API{}
		sc.api.Init(secretKey, nil)
	}
	return sc
}

func (s *StripeClient) CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error) {
	if s.api == nil {
		return nil, fmt.Errorf("stripe API is not configured")
	}
	return s.api.Customers.New(params)
}

func (s *StripeClient) CreateCheckoutSession(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	if s.api == nil {
		return nil, fmt.Errorf("stripe API is not configured")
	}
	return s.api.CheckoutSessions.New(params)
}

func (s *StripeClient) CreatePortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
	if s.api == nil {
		return nil, fmt.Errorf("stripe API is not configured")
	}
	return s.api.BillingPortalSessions.New(params)
}

func (s *StripeClient) GetSubscription(subscriptionID string) (*stripe.Subscription, error) {
	if s.api == nil {
		return nil, fmt.Errorf("stripe API is not configured")
	}
	return s.api.Subscriptions.Get(subscriptionID, nil)
}

func (s *StripeClient) ConstructWebhookEvent(payload []byte, sigHeader, secret string) (stripe.Event, error) {
	if secret == "" {
		return stripe.Event{}, fmt.Errorf("stripe webhook secret is required")
	}
	return stripewebhook.ConstructEvent(payload, sigHeader, secret)
}
