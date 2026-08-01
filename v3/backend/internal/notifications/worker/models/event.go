// Package models contains worker-envelope representations for notifications.
package models

type NotificationRequested struct {
	NotificationID string `json:"notification_id"`
}
