package models

type WebhookEvent struct {
	Event       string `json:"event"`
	TraceID     string `json:"trace_id"`
	MessageID   string `json:"message_id"`
	Channel     string `json:"channel"`
	Timestamp   string `json:"timestamp"`
	WorkspaceID string `json:"workspace_id"`
	Error       string `json:"error,omitempty"`
}
