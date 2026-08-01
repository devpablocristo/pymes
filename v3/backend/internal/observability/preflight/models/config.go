// Package models contains transport-only configuration for the preflight
// revision gate.
package models

// Config enables a capability gate only when Cloud Run routes a request
// through the tagged pretraffic hostname.
type Config struct {
	Tag   string
	Token string
}
