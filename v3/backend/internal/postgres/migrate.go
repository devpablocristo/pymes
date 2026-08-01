package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ApplyMigrations applies every SQL migration in directory order.
func ApplyMigrations(ctx context.Context, databaseURL, directory string) ([]string, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("database connection failed")
	}
	defer pool.Close()
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("migration directory unavailable")
	}
	var applied []string
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("migration file unavailable")
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return nil, fmt.Errorf("migration failed: %s", file.Name())
		}
		applied = append(applied, file.Name())
	}
	return applied, nil
}
