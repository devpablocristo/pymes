package notifications

// preferenceCatalog lists every notification_type × channel users can toggle.
// Keep aligned with email templates in usecases.go (templateFor + send paths).
func preferenceCatalog() []struct {
	notificationType string
	channel          string
} {
	return []struct {
		notificationType string
		channel          string
	}{
		{notificationType: "welcome", channel: "email"},
		{notificationType: "plan_upgraded", channel: "email"},
		{notificationType: "payment_failed", channel: "email"},
		{notificationType: "subscription_canceled", channel: "email"},
	}
}
