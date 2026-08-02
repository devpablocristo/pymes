package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type PerGoFake struct {
	HTTPAddr      string
	APIKey        string
	WorkspaceID   string
	WebhookURL    string
	WebhookSecret string
	Delay         time.Duration
}

func LoadPerGoFake() (PerGoFake, error) {
	return LoadPerGoFakeFrom(os.Getenv)
}

func LoadPerGoFakeFrom(getenv func(string) string) (PerGoFake, error) {
	if getenv == nil {
		return PerGoFake{}, fmt.Errorf("environment reader is required")
	}
	delay := 2 * time.Second
	if value := strings.TrimSpace(getenv("PERGO_FAKE_DELAY")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed < 100*time.Millisecond || parsed > 30*time.Second {
			return PerGoFake{}, fmt.Errorf("PERGO_FAKE_DELAY is invalid")
		}
		delay = parsed
	}
	config := PerGoFake{
		HTTPAddr:      defaultValue(getenv("PERGO_FAKE_HTTP_ADDR"), ":8080"),
		APIKey:        strings.TrimSpace(getenv("PERGO_API_KEY")),
		WorkspaceID:   strings.TrimSpace(getenv("PERGO_WORKSPACE_ID")),
		WebhookURL:    strings.TrimRight(strings.TrimSpace(getenv("PERGO_WEBHOOK_URL")), "/"),
		WebhookSecret: strings.TrimSpace(getenv("PERGO_WEBHOOK_SECRET")),
		Delay:         delay,
	}
	if config.APIKey == "" || config.WorkspaceID == "" ||
		config.WebhookURL == "" || len(config.WebhookSecret) < 16 {
		return PerGoFake{}, fmt.Errorf("complete PerGo fake configuration is required")
	}
	return config, nil
}
