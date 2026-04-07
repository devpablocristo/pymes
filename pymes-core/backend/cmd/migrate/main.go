package main

import (
	"log"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/config"
	"github.com/devpablocristo/pymes/pymes-core/backend/migrations"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/store"
	"github.com/rs/zerolog"
)

func main() {
	cfg := config.LoadFromEnv()
	logger := zerolog.New(log.Writer()).With().Timestamp().Logger()

	db, err := store.NewDB(cfg.DatabaseURL, logger)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	if err := migrations.Run(db, logger); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
}
