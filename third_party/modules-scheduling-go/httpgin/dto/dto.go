package dto

type CreateBranchRequest struct {
	Code     string         `json:"code"`
	Name     string         `json:"name"`
	Timezone string         `json:"timezone"`
	Address  string         `json:"address"`
	Active   *bool          `json:"active,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CreateServiceRequest struct {
	CommercialServiceID    *string        `json:"commercial_service_id,omitempty"`
	Code                   string         `json:"code"`
	Name                   string         `json:"name"`
	Description            string         `json:"description"`
	FulfillmentMode        string         `json:"fulfillment_mode"`
	DefaultDurationMinutes int            `json:"default_duration_minutes"`
	BufferBeforeMinutes    int            `json:"buffer_before_minutes"`
	BufferAfterMinutes     int            `json:"buffer_after_minutes"`
	SlotGranularityMinutes int            `json:"slot_granularity_minutes"`
	MaxConcurrentBookings  int            `json:"max_concurrent_bookings"`
	MinCancelNoticeMinutes int            `json:"min_cancel_notice_minutes"`
	AllowWaitlist          *bool          `json:"allow_waitlist,omitempty"`
	Active                 *bool          `json:"active,omitempty"`
	ResourceIDs            []string       `json:"resource_ids,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
}

type CreateResourceRequest struct {
	BranchID string         `json:"branch_id"`
	Code     string         `json:"code"`
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	Capacity int            `json:"capacity"`
	Timezone string         `json:"timezone"`
	Active   *bool          `json:"active,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CreateAvailabilityRuleRequest struct {
	BranchID               string         `json:"branch_id"`
	ResourceID             *string        `json:"resource_id,omitempty"`
	Kind                   string         `json:"kind"`
	Weekday                int            `json:"weekday"`
	StartTime              string         `json:"start_time"`
	EndTime                string         `json:"end_time"`
	SlotGranularityMinutes *int           `json:"slot_granularity_minutes,omitempty"`
	ValidFrom              *string        `json:"valid_from,omitempty"`
	ValidUntil             *string        `json:"valid_until,omitempty"`
	Active                 *bool          `json:"active,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
}

type CreateBlockedRangeRequest struct {
	BranchID   string         `json:"branch_id"`
	ResourceID *string        `json:"resource_id,omitempty"`
	Kind       string         `json:"kind"`
	Reason     string         `json:"reason"`
	StartAt    string         `json:"start_at"`
	EndAt      string         `json:"end_at"`
	AllDay     bool           `json:"all_day"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type UpdateBlockedRangeRequest struct {
	BranchID   string         `json:"branch_id"`
	ResourceID *string        `json:"resource_id,omitempty"`
	Kind       string         `json:"kind"`
	Reason     string         `json:"reason"`
	StartAt    string         `json:"start_at"`
	EndAt      string         `json:"end_at"`
	AllDay     bool           `json:"all_day"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type CreateCalendarEventRequest struct {
	BranchID    *string        `json:"branch_id,omitempty"`
	ResourceID  *string        `json:"resource_id,omitempty"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	StartAt     string         `json:"start_at"`
	EndAt       string         `json:"end_at"`
	AllDay      bool           `json:"all_day,omitempty"`
	Status      string         `json:"status,omitempty"`
	Visibility  string         `json:"visibility,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UpdateCalendarEventRequest struct {
	BranchID    *string        `json:"branch_id,omitempty"`
	ResourceID  *string        `json:"resource_id,omitempty"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	StartAt     string         `json:"start_at"`
	EndAt       string         `json:"end_at"`
	AllDay      bool           `json:"all_day,omitempty"`
	Status      string         `json:"status,omitempty"`
	Visibility  string         `json:"visibility,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type CreateBookingRequest struct {
	BranchID       string                    `json:"branch_id"`
	ServiceID      string                    `json:"service_id"`
	ResourceID     *string                   `json:"resource_id,omitempty"`
	PartyID        *string                   `json:"party_id,omitempty"`
	CustomerName   string                    `json:"customer_name"`
	CustomerPhone  string                    `json:"customer_phone"`
	CustomerEmail  string                    `json:"customer_email,omitempty"`
	StartAt        string                    `json:"start_at"`
	EndAt          *string                   `json:"end_at,omitempty"`
	Status         string                    `json:"status,omitempty"`
	Source         string                    `json:"source,omitempty"`
	IdempotencyKey string                    `json:"idempotency_key,omitempty"`
	HoldUntil      *string                   `json:"hold_until,omitempty"`
	Notes          string                    `json:"notes,omitempty"`
	Metadata       map[string]any            `json:"metadata,omitempty"`
	Recurrence     *BookingRecurrenceRequest `json:"recurrence,omitempty"`
}

type BookingRecurrenceRequest struct {
	Freq      string `json:"freq"`
	Interval  int    `json:"interval,omitempty"`
	Count     int    `json:"count,omitempty"`
	Until     string `json:"until,omitempty"`
	ByWeekday []int  `json:"by_weekday,omitempty"`
}

type RescheduleBookingRequest struct {
	BranchID   *string `json:"branch_id,omitempty"`
	ResourceID *string `json:"resource_id,omitempty"`
	StartAt    string  `json:"start_at"`
	EndAt      *string `json:"end_at,omitempty"`
}

type CancelBookingRequest struct {
	Reason string `json:"reason"`
}

type CreateQueueRequest struct {
	BranchID          string         `json:"branch_id"`
	ServiceID         *string        `json:"service_id,omitempty"`
	Code              string         `json:"code"`
	Name              string         `json:"name"`
	Status            string         `json:"status,omitempty"`
	Strategy          string         `json:"strategy,omitempty"`
	TicketPrefix      string         `json:"ticket_prefix,omitempty"`
	AvgServiceSeconds int            `json:"avg_service_seconds"`
	AllowRemoteJoin   *bool          `json:"allow_remote_join,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type CreateQueueTicketRequest struct {
	PartyID        *string        `json:"party_id,omitempty"`
	CustomerName   string         `json:"customer_name"`
	CustomerPhone  string         `json:"customer_phone"`
	CustomerEmail  string         `json:"customer_email,omitempty"`
	Priority       int            `json:"priority"`
	Source         string         `json:"source,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Notes          string         `json:"notes,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type TicketOperationRequest struct {
	ServingResourceID *string `json:"serving_resource_id,omitempty"`
	OperatorUserID    *string `json:"operator_user_id,omitempty"`
}

type CreateWaitlistRequest struct {
	BranchID         string         `json:"branch_id"`
	ServiceID        string         `json:"service_id"`
	ResourceID       *string        `json:"resource_id,omitempty"`
	PartyID          *string        `json:"party_id,omitempty"`
	CustomerName     string         `json:"customer_name"`
	CustomerPhone    string         `json:"customer_phone"`
	CustomerEmail    string         `json:"customer_email,omitempty"`
	RequestedStartAt string         `json:"requested_start_at"`
	Source           string         `json:"source,omitempty"`
	IdempotencyKey   string         `json:"idempotency_key,omitempty"`
	Notes            string         `json:"notes,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type QueueStatusRequest struct {
	Status string `json:"status"`
}
