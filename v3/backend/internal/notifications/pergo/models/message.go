// Package models contains PerGo wire payloads only.
package models

import "time"

type MessageRequest struct {
	To           string              `json:"to"`
	From         string              `json:"from,omitempty"`
	Channel      string              `json:"channel"`
	Body         string              `json:"body"`
	Metadata     map[string]string   `json:"metadata,omitempty"`
	TTLSeconds   *int                `json:"ttl_seconds,omitempty"`
	TemplateName string              `json:"template_name,omitempty"`
	Language     string              `json:"language,omitempty"`
	Components   []TemplateComponent `json:"components,omitempty"`
}

type TemplateComponent struct {
	Type       string              `json:"type"`
	Parameters []TemplateParameter `json:"parameters"`
}

type TemplateParameter struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type MessageResponse struct {
	MessageID string    `json:"message_id"`
	Status    string    `json:"status"`
	QueuedAt  time.Time `json:"queued_at"`
}
