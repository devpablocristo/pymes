package models

type RunResult struct {
	ExpiredHolds   int `json:"expired_holds"`
	ReminderEvents int `json:"reminder_events"`
	WaitlistOffers int `json:"waitlist_offers"`
}
