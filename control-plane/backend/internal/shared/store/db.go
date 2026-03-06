// Package store provides shared database initialization and readiness helpers.
package store

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(databaseURL string, log zerolog.Logger) (*gorm.DB, error) {
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	log.Info().Msg("connected to PostgreSQL")
	return db, nil
}

func Ping(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}
