package models

type CalendarSyncRequested struct {
	CommandID      string   `json:"command_id"`
	BookingID      string   `json:"booking_id"`
	Operation      string   `json:"operation"`
	SourceVersion  int      `json:"source_version"`
	SnapshotDigest string   `json:"snapshot_digest"`
	CorrelationID  string   `json:"correlation_id"`
	Summary        string   `json:"summary,omitempty"`
	Description    string   `json:"description,omitempty"`
	Location       string   `json:"location,omitempty"`
	Start          string   `json:"start,omitempty"`
	End            string   `json:"end,omitempty"`
	TimeZone       string   `json:"time_zone,omitempty"`
	AttendeeEmails []string `json:"attendee_emails,omitempty"`
	MeetRequested  bool     `json:"meet_requested,omitempty"`
}
