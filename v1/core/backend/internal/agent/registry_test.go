package agent

import "testing"

func TestRegistryPublishesCompanionAutomationCapabilities(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	for _, id := range []string{
		"pymes.customers.search",
		"pymes.services.search",
		"pymes.inventory.search",
		"pymes.cashflow.summary",
		"pymes.accounts.summary",
		"pymes.get_work_orders",
		"pymes.get_appointments",
		"pymes.get_low_stock",
		"pymes.get_customers",
		"pymes.get_revenue_comparison",
		"pymes.quotes.create",
		"pymes.sales.create",
		"pymes.payments.link",
		"pymes.procurement_requests.create",
		"pymes.scheduling.book",
		"pymes.send_whatsapp_text",
		"pymes.send_whatsapp_template",
	} {
		capability, ok := registry.Get(id)
		if !ok {
			t.Fatalf("expected capability %s to be published", id)
		}
		if capability.NexusActionType == "" {
			t.Fatalf("expected capability %s to declare nexus_action_type", id)
		}
	}
}

func TestRegistryKeepsLegacyCapabilityAliases(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	for legacy, canonical := range legacyCapabilityAliases() {
		capability, ok := registry.Get(legacy)
		if !ok {
			t.Fatalf("expected legacy alias %s to resolve", legacy)
		}
		if capability.ID != canonical {
			t.Fatalf("legacy alias %s resolved to %s, want %s", legacy, capability.ID, canonical)
		}
	}
}

func TestRegistryListDoesNotDuplicateAliases(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	seen := map[string]bool{}
	for _, capability := range registry.List() {
		if seen[capability.ID] {
			t.Fatalf("duplicated capability in list: %s", capability.ID)
		}
		seen[capability.ID] = true
	}
}
