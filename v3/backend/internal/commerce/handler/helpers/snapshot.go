package helpers

import (
	"encoding/json"
	"fmt"
)

const defaultFiscalConcept = "products"

// FreezeFiscalSnapshot creates the exact immutable payload whose digest Pymes
// persists. Defaults are materialized here so retries never change semantics.
func FreezeFiscalSnapshot(fiscal any, currency, exchangeRate string) ([]byte, error) {
	raw, err := json.Marshal(fiscal)
	if err != nil {
		return nil, err
	}
	var snapshot map[string]any
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	if concept, exists := snapshot["concept"]; !exists || concept == nil || concept == "" {
		snapshot["concept"] = defaultFiscalConcept
	}
	snapshot["currency"] = currency
	if exchangeRate != "" {
		snapshot["exchange_rate"] = exchangeRate
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode fiscal snapshot: %w", err)
	}
	return encoded, nil
}
