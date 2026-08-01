// Package models contains the PerGo fake's control-plane payloads.
package models

import (
	"net/http"
	"time"
)

type Config struct {
	APIKey        string
	WorkspaceID   string
	WebhookURL    string
	WebhookSecret []byte
	Delay         time.Duration
	Client        *http.Client
}

type Delivery struct {
	IdempotencyKey    string
	TraceID           string
	Digest            string
	MessageID         string
	QueuedAt          time.Time
	Channel           string
	SenderIdentity    string
	Event             []byte
	Requests          int
	WebhookDeliveries int
}

type ScenarioRequest struct {
	Scenario string `json:"scenario"`
}

type ScenarioResponse struct {
	Scenario string `json:"scenario"`
}

type MessageStats struct {
	Requests          int    `json:"requests"`
	WebhookDeliveries int    `json:"webhook_deliveries"`
	Channel           string `json:"channel"`
	SenderIdentity    string `json:"sender_identity"`
}
