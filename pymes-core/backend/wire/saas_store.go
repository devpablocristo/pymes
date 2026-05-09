package wire

import (
	"log/slog"

	"gorm.io/gorm"
)

type pymesSaaSStore struct {
	db                 *gorm.DB
	logger             *slog.Logger
	defaultKeyScopes   []string
	clerk              clerkTenantClient
	frontendURL        string
	publicBaseURL      string
	environment        string
	clerkWebhookSecret string
}

func newPymesSaaSStore(db *gorm.DB, logger *slog.Logger, defaultKeyScopes []string) *pymesSaaSStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &pymesSaaSStore{
		db:               db,
		logger:           logger,
		defaultKeyScopes: append([]string(nil), defaultKeyScopes...),
	}
}
