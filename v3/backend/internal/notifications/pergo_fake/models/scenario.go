// Package models contains the PerGo fake's control-plane payloads.
package models

type ScenarioRequest struct {
	Scenario string `json:"scenario"`
}

type ScenarioResponse struct {
	Scenario string `json:"scenario"`
}

type MessageStats struct {
	Requests          int `json:"requests"`
	WebhookDeliveries int `json:"webhook_deliveries"`
}
