// Package helpers contains redaction-safe HTTP identifier normalization.
package helpers

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// OpaqueTraceID is the bounded redaction-safe identifier grammar.
var OpaqueTraceID = regexp.MustCompile(`^[A-Za-z0-9:_./-]{1,255}$`)

func HeaderOrNew(value string) string {
	return HeaderOrDefault(value, uuid.NewString())
}

func HeaderOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); OpaqueTraceID.MatchString(value) {
		return value
	}
	return fallback
}
