package billing

import (
	"testing"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/billing/usecases/domain"
	"github.com/stripe/stripe-go/v81"
)

func TestNormalizePlan(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  domain.PlanCode
	}{
		{"starter explicit", "starter", domain.PlanStarter},
		{"growth", "growth", domain.PlanGrowth},
		{"enterprise", "enterprise", domain.PlanEnterprise},
		{"uppercase", "GROWTH", domain.PlanGrowth},
		{"mixed case", "Enterprise", domain.PlanEnterprise},
		{"with spaces", "  growth  ", domain.PlanGrowth},
		{"empty defaults to starter", "", domain.PlanStarter},
		{"unknown defaults to starter", "premium", domain.PlanStarter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePlan(tt.input)
			if got != tt.want {
				t.Errorf("normalizePlan(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToHardLimits(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		got := toHardLimits(nil)
		if got.UsersMax != nil || got.StorageMB != nil || got.APICallsRPM != nil {
			t.Errorf("toHardLimits(nil) should return zero HardLimits, got %+v", got)
		}
	})

	t.Run("with values", func(t *testing.T) {
		input := map[string]any{
			"users_max":     5,
			"storage_mb":    500,
			"api_calls_rpm": 100,
		}
		got := toHardLimits(input)
		if got.UsersMax != 5 {
			t.Errorf("UsersMax = %v; want 5", got.UsersMax)
		}
		if got.StorageMB != 500 {
			t.Errorf("StorageMB = %v; want 500", got.StorageMB)
		}
		if got.APICallsRPM != 100 {
			t.Errorf("APICallsRPM = %v; want 100", got.APICallsRPM)
		}
	})
}

func TestNormalizeActorEmail(t *testing.T) {
	tests := []struct {
		name  string
		actor string
		want  string
	}{
		{"empty", "", "no-reply@pymes.local"},
		{"spaces only", "   ", "no-reply@pymes.local"},
		{"email already", "user@example.com", "user@example.com"},
		{"username only", "johndoe", "johndoe@pymes.local"},
		{"with spaces", "  user@test.com  ", "user@test.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeActorEmail(tt.actor)
			if got != tt.want {
				t.Errorf("normalizeActorEmail(%q) = %q; want %q", tt.actor, got, tt.want)
			}
		})
	}
}

func TestMapSubscriptionStatus(t *testing.T) {
	tests := []struct {
		name   string
		status stripe.SubscriptionStatus
		want   string
	}{
		{"active", stripe.SubscriptionStatusActive, string(domain.BillingActive)},
		{"past due", stripe.SubscriptionStatusPastDue, string(domain.BillingPastDue)},
		{"canceled", stripe.SubscriptionStatusCanceled, string(domain.BillingCanceled)},
		{"trialing", stripe.SubscriptionStatusTrialing, string(domain.BillingTrialing)},
		{"unknown defaults active", stripe.SubscriptionStatus("unknown"), string(domain.BillingActive)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapSubscriptionStatus(tt.status)
			if got != tt.want {
				t.Errorf("mapSubscriptionStatus(%q) = %q; want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestInvoiceRefs(t *testing.T) {
	t.Run("nil invoice", func(t *testing.T) {
		subID, custID := invoiceRefs(nil)
		if subID != "" || custID != "" {
			t.Errorf("invoiceRefs(nil) = (%q, %q); want (\"\", \"\")", subID, custID)
		}
	})

	t.Run("with refs", func(t *testing.T) {
		inv := &stripe.Invoice{
			Subscription: &stripe.Subscription{ID: "sub_123"},
			Customer:     &stripe.Customer{ID: "cus_456"},
		}
		subID, custID := invoiceRefs(inv)
		if subID != "sub_123" {
			t.Errorf("subID = %q; want %q", subID, "sub_123")
		}
		if custID != "cus_456" {
			t.Errorf("custID = %q; want %q", custID, "cus_456")
		}
	})

	t.Run("nil sub and customer", func(t *testing.T) {
		inv := &stripe.Invoice{}
		subID, custID := invoiceRefs(inv)
		if subID != "" || custID != "" {
			t.Errorf("invoiceRefs with nil refs = (%q, %q); want (\"\", \"\")", subID, custID)
		}
	})
}
