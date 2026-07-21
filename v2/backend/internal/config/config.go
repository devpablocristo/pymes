package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const ServiceName = "pymes-v2-api"

const (
	defaultHTTPAddr         = ":8080"
	defaultShutdownTimeout  = 10 * time.Second
	defaultReadinessTimeout = 2 * time.Second
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	ShutdownTimeout  time.Duration
	ReadinessTimeout time.Duration
}

type Getenv func(string) string

func Load() (Config, error) {
	return LoadFrom(os.Getenv)
}

func LoadFrom(getenv Getenv) (Config, error) {
	if getenv == nil {
		return Config{}, fmt.Errorf("configuration environment reader is nil")
	}

	cfg := Config{
		HTTPAddr:         valueOrDefault(getenv("PYMES_HTTP_ADDR"), defaultHTTPAddr),
		DatabaseURL:      strings.TrimSpace(getenv("PYMES_DATABASE_URL")),
		ShutdownTimeout:  defaultShutdownTimeout,
		ReadinessTimeout: defaultReadinessTimeout,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("PYMES_DATABASE_URL is required")
	}

	var err error
	cfg.ShutdownTimeout, err = durationOrDefault(getenv("PYMES_SHUTDOWN_TIMEOUT"), defaultShutdownTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("PYMES_SHUTDOWN_TIMEOUT: %w", err)
	}
	cfg.ReadinessTimeout, err = durationOrDefault(getenv("PYMES_READINESS_TIMEOUT"), defaultReadinessTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("PYMES_READINESS_TIMEOUT: %w", err)
	}
	return cfg, nil
}

func valueOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func durationOrDefault(value string, fallback time.Duration) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("must be greater than zero")
	}
	return duration, nil
}
