package config

import (
	"fmt"
	"os"
	"strings"
)

type Clerk struct {
	SecretKey, JWTKey, Issuer, Audience, WebhookSecret string
	AuthorizedParties                                  []string
}
type Config struct {
	HTTPAddr, DatabaseURL, FiscalURL, Environment string
	AllowInsecureLocalServices                    bool
	Clerk                                         Clerk
}

func Load() (Config, error) { return LoadFrom(os.Getenv) }
func LoadFrom(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, fmt.Errorf("environment reader is required")
	}
	env := strings.ToLower(defaultValue(getenv("PYMES_ENVIRONMENT"), "development"))
	if env != "development" && env != "test" && env != "production" {
		return Config{}, fmt.Errorf("PYMES_ENVIRONMENT must be development, test, or production")
	}
	allowInsecure := strings.EqualFold(
		strings.TrimSpace(getenv("PYMES_ALLOW_INSECURE_LOCAL_SERVICES")),
		"true",
	)
	if allowInsecure && env == "production" {
		return Config{}, fmt.Errorf("insecure local services are forbidden in production")
	}
	cfg := Config{
		HTTPAddr:                   defaultValue(getenv("PYMES_HTTP_ADDR"), ":8080"),
		DatabaseURL:                strings.TrimSpace(getenv("PYMES_DATABASE_URL")),
		FiscalURL:                  strings.TrimSpace(getenv("FISCAL_ADAPTER_URL")),
		Environment:                env,
		AllowInsecureLocalServices: allowInsecure,
		Clerk: Clerk{
			SecretKey:         strings.TrimSpace(getenv("PYMES_CLERK_SECRET_KEY")),
			JWTKey:            strings.TrimSpace(getenv("PYMES_CLERK_JWT_KEY")),
			Issuer:            strings.TrimRight(strings.TrimSpace(getenv("PYMES_CLERK_ISSUER")), "/"),
			Audience:          defaultValue(getenv("PYMES_CLERK_AUDIENCE"), "pymes-v3"),
			AuthorizedParties: csv(getenv("PYMES_CLERK_AUTHORIZED_PARTIES")),
			WebhookSecret:     strings.TrimSpace(getenv("PYMES_CLERK_WEBHOOK_SECRET")),
		},
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("PYMES_DATABASE_URL is required")
	}
	if cfg.FiscalURL == "" {
		return Config{}, fmt.Errorf("FISCAL_ADAPTER_URL is required")
	}
	if cfg.Clerk.Issuer == "" || len(cfg.Clerk.AuthorizedParties) == 0 || cfg.Clerk.WebhookSecret == "" || (cfg.Clerk.SecretKey == "" && cfg.Clerk.JWTKey == "") {
		return Config{}, fmt.Errorf("complete Clerk configuration is required")
	}
	return cfg, nil
}
func defaultValue(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}
func csv(value string) []string {
	var out []string
	for _, p := range strings.Split(value, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
