package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v3/backend/wire"
)

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("PYMES_DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("PYMES_DATABASE_URL is required")
	}
	directory := strings.TrimSpace(os.Getenv("PYMES_MIGRATIONS_DIR"))
	if directory == "" {
		directory = "/migrations"
	}
	environment := strings.TrimSpace(os.Getenv("PYMES_DEPLOY_ENV"))
	if environment == "" {
		log.Fatal("PYMES_DEPLOY_ENV is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	files, err := wire.Migrate(ctx, databaseURL, directory, environment)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		log.Printf("migration applied: %s", file)
	}
}
