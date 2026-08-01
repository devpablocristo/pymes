// Package models contains worker-envelope representations for notifications.
// Package models contains payloads owned by the notifications worker adapter.
package models

import "time"

type DeliveryRequested struct {
	NotificationID string `json:"notification_id"`
}

type SchedulingRequested struct {
	Trigger       string            `json:"trigger"`
	AggregateType string            `json:"aggregate_type"`
	AggregateID   string            `json:"aggregate_id"`
	BookingID     string            `json:"booking_id,omitempty"`
	WaitlistID    string            `json:"waitlist_id,omitempty"`
	RecipientE164 string            `json:"recipient_e164,omitempty"`
	CustomerName  string            `json:"customer_name,omitempty"`
	ServiceName   string            `json:"service_name,omitempty"`
	StartAt       time.Time         `json:"start_at,omitempty"`
	EndAt         time.Time         `json:"end_at,omitempty"`
	Timezone      string            `json:"timezone,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	ActionToken   string            `json:"action_token,omitempty"`
	ActionTokens  map[string]string `json:"action_tokens,omitempty"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	SupersedesID  string            `json:"supersedes_booking_id,omitempty"`
}

type Requested struct {
	Delivery   *DeliveryRequested
	Scheduling *SchedulingRequested
}
