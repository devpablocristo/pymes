// Package domain contains the organization directory model.
package domain

import (
	"errors"
	"time"
)

type Status string

const (
	Pending   Status = "pending"
	Ready     Status = "ready"
	Failed    Status = "failed"
	Suspended Status = "suspended"
)

var ErrUnknown = errors.New("organization not found")

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
