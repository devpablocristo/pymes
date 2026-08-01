package helpers

import (
	"encoding/json"
	"testing"
)

func TestFreezeFiscalSnapshotMaterializesDefaultConcept(t *testing.T) {
	raw, err := FreezeFiscalSnapshot(
		map[string]any{"environment": "homologation"},
		"ARS",
		"",
	)
	if err != nil {
		t.Fatalf("freeze fiscal snapshot: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode fiscal snapshot: %v", err)
	}
	if got["concept"] != "products" {
		t.Fatalf("concept = %v, want products", got["concept"])
	}
	if got["currency"] != "ARS" {
		t.Fatalf("currency = %v, want ARS", got["currency"])
	}
}

func TestFreezeFiscalSnapshotPreservesServiceConceptAndPeriod(t *testing.T) {
	fiscal := map[string]any{
		"concept": "services",
		"service_period": map[string]any{
			"from":        "2026-08-01",
			"to":          "2026-08-31",
			"payment_due": "2026-09-10",
		},
	}
	raw, err := FreezeFiscalSnapshot(fiscal, "USD", "1340.25")
	if err != nil {
		t.Fatalf("freeze fiscal snapshot: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode fiscal snapshot: %v", err)
	}
	if got["concept"] != "services" {
		t.Fatalf("concept = %v, want services", got["concept"])
	}
	if got["exchange_rate"] != "1340.25" {
		t.Fatalf("exchange_rate = %v, want 1340.25", got["exchange_rate"])
	}
	if got["service_period"] == nil {
		t.Fatal("service_period was not preserved")
	}
}
