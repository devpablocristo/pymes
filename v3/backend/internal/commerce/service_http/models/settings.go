// Package models contains configuration owned by the private HTTP adapter.
package models

import "time"

type Settings struct {
	FailureThreshold int
	OpenFor          time.Duration
	RequestTimeout   time.Duration
}
