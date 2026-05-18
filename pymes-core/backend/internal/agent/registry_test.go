package agent

import "testing"

func TestRegistryPublishesCompanionAutomationCapabilities(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	for _, id := range []string{
		"pymes.get_work_orders",
		"pymes.get_appointments",
		"pymes.get_low_stock",
		"pymes.get_customers",
		"pymes.get_revenue_comparison",
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
