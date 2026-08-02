// Package helpers contains filesystem codecs for the migration adapter.
package helpers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migratemodels "github.com/devpablocristo/pymes/v3/backend/internal/postgres/migrate/models"
)

var (
	ErrDirectoryUnavailable = errors.New("migration directory unavailable")
	ErrFileUnavailable      = errors.New("migration file unavailable")
)

func Load(directory string) ([]migratemodels.Migration, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDirectoryUnavailable, err)
	}
	migrations := make([]migratemodels.Migration, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrFileUnavailable, file.Name(), err)
		}
		migrations = append(migrations, migratemodels.Migration{
			Name: file.Name(),
			SQL:  string(body),
		})
	}
	return migrations, nil
}
