package domain

type RunResult struct {
	Task             string         `json:"task"`
	RecurringApplied int            `json:"recurring_applied"`
	RatesUpdated     int            `json:"rates_updated"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}
