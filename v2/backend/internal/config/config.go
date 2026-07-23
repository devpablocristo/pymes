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
	defaultEnvironment      = "development"
	defaultClerkAudience    = "pymes-v2-api"
)

type ClerkConfig struct {
	PublishableKey    string
	SecretKey         string
	Issuer            string
	Audience          string
	AuthorizedParties []string
	WebhookSecret     string
	JWTKey            string
}

func (c ClerkConfig) Configured() bool {
	return c.PublishableKey != "" && c.SecretKey != "" && c.Issuer != ""
}

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	Environment      string
	Clerk            ClerkConfig
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
		HTTPAddr:    valueOrDefault(getenv("PYMES_HTTP_ADDR"), defaultHTTPAddr),
		DatabaseURL: strings.TrimSpace(getenv("PYMES_DATABASE_URL")),
		Environment: strings.ToLower(valueOrDefault(getenv("PYMES_ENVIRONMENT"), defaultEnvironment)),
		Clerk: ClerkConfig{
			PublishableKey:    strings.TrimSpace(getenv("PYMES_CLERK_PUBLISHABLE_KEY")),
			SecretKey:         strings.TrimSpace(getenv("PYMES_CLERK_SECRET_KEY")),
			Issuer:            strings.TrimRight(strings.TrimSpace(getenv("PYMES_CLERK_ISSUER")), "/"),
			Audience:          valueOrDefault(getenv("PYMES_CLERK_AUDIENCE"), defaultClerkAudience),
			AuthorizedParties: commaSeparated(getenv("PYMES_CLERK_AUTHORIZED_PARTIES")),
			WebhookSecret:     strings.TrimSpace(getenv("PYMES_CLERK_WEBHOOK_SECRET")),
			JWTKey:            strings.TrimSpace(getenv("PYMES_CLERK_JWT_KEY")),
		},
		ShutdownTimeout:  defaultShutdownTimeout,
		ReadinessTimeout: defaultReadinessTimeout,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("PYMES_DATABASE_URL is required")
	}
	if cfg.Environment != "development" && cfg.Environment != "test" && cfg.Environment != "production" {
		return Config{}, fmt.Errorf("PYMES_ENVIRONMENT must be development, test, or production")
	}
	if cfg.Environment == "production" && !cfg.Clerk.Configured() {
		return Config{}, fmt.Errorf("Clerk configuration is required in production")
	}
	if cfg.Clerk.Configured() && len(cfg.Clerk.AuthorizedParties) == 0 {
		return Config{}, fmt.Errorf("PYMES_CLERK_AUTHORIZED_PARTIES is required when Clerk is configured")
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

func commaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimRight(strings.TrimSpace(part), "/")
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return result
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
