// Package helpers contains environment parsing for the tracing adapter.
package helpers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/devpablocristo/pymes/v3/backend/internal/observability/tracing/models"
)

func SettingsFromEnv(serviceName, environment string, getenv func(string) string) (models.Settings, error) {
	sampleRatio := 1.0
	if raw := strings.TrimSpace(getenv("PYMES_TRACE_SAMPLE_RATIO")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value <= 0 || value > 1 {
			return models.Settings{}, fmt.Errorf("PYMES_TRACE_SAMPLE_RATIO must be greater than zero and at most one")
		}
		sampleRatio = value
	}
	exporter := strings.ToLower(strings.TrimSpace(getenv("PYMES_TRACING_EXPORTER")))
	if exporter == "" {
		exporter = "none"
	}
	insecure, err := strconv.ParseBool(DefaultValue(getenv("OTEL_EXPORTER_OTLP_INSECURE"), "false"))
	if err != nil {
		return models.Settings{}, fmt.Errorf("OTEL_EXPORTER_OTLP_INSECURE must be a boolean")
	}
	return models.Settings{
		ServiceName: serviceName, ServiceVersion: DefaultValue(getenv("PYMES_SERVICE_VERSION"), "dev"),
		Environment: environment, Exporter: exporter,
		OTLPEndpoint: strings.TrimSpace(getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
		OTLPInsecure: insecure, SampleRatio: sampleRatio,
	}, nil
}

func DefaultValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
