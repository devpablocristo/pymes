// Package dto contains HTTP metadata owned by the observability handler.
package dto

type RequestIDs struct {
	RequestID     string
	CorrelationID string
}
