package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal("database connection failed")
	}
	defer pool.Close()
	files, err := os.ReadDir(directory)
	if err != nil {
		log.Fatal("migration directory unavailable")
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			log.Fatal("migration file unavailable")
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			log.Fatalf("migration failed: %s", file.Name())
		}
		log.Printf("migration applied: %s", file.Name())
	}
}
