// Package store provides shared database initialization and readiness helpers.
// Delega a core/databases/gorm/go para la primitiva de conexión.
package store

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	gormdb "github.com/devpablocristo/core/databases/gorm/go"
)

// NewDB abre una conexión GORM a PostgreSQL con configuración por defecto.
func NewDB(databaseURL string, log zerolog.Logger) (*gorm.DB, error) {
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	db, err := gormdb.Open(databaseURL, gormdb.DefaultConfig())
	if err != nil {
		return nil, err
	}
	log.Info().Msg("connected to PostgreSQL")
	return db, nil
}

// Ping verifica que la conexión esté activa.
func Ping(ctx context.Context, db *gorm.DB) error {
	return gormdb.Ping(ctx, db)
}
