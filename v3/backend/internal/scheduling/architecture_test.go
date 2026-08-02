package scheduling

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSchedulingContextKeepsVerticalAdapterBoundaries(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve scheduling context")
	}
	root := filepath.Dir(currentFile)
	required := []string{
		"handler.go",
		"handler/dto",
		"handler/helpers",
		"usecases.go",
		"usecases/domain",
		"repository.go",
		"repository/models",
		"repository/helpers",
		"platform_scheduling.go",
		"platform_scheduling/models",
		"platform_scheduling/helpers",
		"calendar_projection.go",
		"calendar_projection/models",
		"calendar_projection/helpers",
		"worker.go",
		"worker/models",
		"worker/helpers",
	}
	for _, relative := range required {
		if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
			t.Errorf("required scheduling architecture path %s: %v", relative, err)
		}
	}

	domainRoot := filepath.Join(root, "usecases", "domain")
	err := filepath.WalkDir(domainRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		if path == currentFile {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			value := strings.Trim(spec.Path.Value, `"`)
			for _, forbidden := range []string{
				"net/http", "database/sql", "github.com/jackc/pgx",
				"platform/features/scheduling", "clerk", "google.golang.org/api/calendar", "pergo",
				"accounting", "fiscal",
			} {
				if strings.Contains(strings.ToLower(value), strings.ToLower(forbidden)) {
					t.Errorf("domain imports infrastructure %q in %s", value, path)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		if path == currentFile {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		if strings.Contains(lower, "/axis") || strings.Contains(lower, "github.com/devpablocristo/axis") {
			t.Errorf("Axis reference in %s", path)
		}
		if strings.Contains(lower, "platform/features/scheduling") &&
			!strings.Contains(filepath.ToSlash(path), "/platform_scheduling") {
			t.Errorf("Platform type escaped adapter in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
