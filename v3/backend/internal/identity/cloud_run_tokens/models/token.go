// Package models contains cache records owned by the Cloud Run token adapter.
package models

import "time"

type CachedToken struct {
	Value     string
	ExpiresAt time.Time
}
