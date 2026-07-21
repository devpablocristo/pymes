package wire

import (
	"log/slog"
	"strings"

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

// resolvedFrontendURL devuelve la URL del frontend lista para concatenar
// con un path (sin trailing slash). El config ya garantiza un valor por
// default; este helper sólo normaliza el formato y centraliza la lectura
// para evitar duplicación de TrimRight + fallback en cada caller.
func (s *pymesSaaSStore) resolvedFrontendURL() string {
	return strings.TrimRight(strings.TrimSpace(s.frontendURL), "/")
}
