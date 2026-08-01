package worker_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestWorkerHexagonalDependencyDirection(t *testing.T) {
	t.Parallel()
	forbiddenImports := map[string][]string{
		"domain":     {"/internal/worker/ports", "/internal/worker/usecases", "/internal/worker/handler", "/internal/worker/repository", "/wire", "pgx", "net/http"},
		"ports":      {"/internal/worker/usecases", "/internal/worker/handler", "/internal/worker/repository", "/wire", "pgx", "net/http"},
		"usecases":   {"/internal/worker/handler", "/internal/worker/repository", "/wire", "pgx", "net/http"},
		"handler":    {"/internal/worker/repository", "/wire", "pgx"},
		"repository": {"/internal/worker/handler", "/internal/worker/usecases", "/wire", "net/http"},
	}
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(path), "/")
		if len(parts) < 2 {
			return nil
		}
		layer := parts[0]
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(
			token.NewFileSet(),
			path,
			source,
			parser.ImportsOnly,
		)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			for _, forbidden := range forbiddenImports[layer] {
				if strings.Contains(importPath, forbidden) {
					t.Errorf(
						"%s imports forbidden dependency %q",
						path,
						importPath,
					)
				}
			}
		}
		if layer != "repository" {
			for _, sql := range []string{
				"SELECT ", "INSERT ", "UPDATE ", "DELETE ",
			} {
				if strings.Contains(string(source), sql) {
					t.Errorf("%s contains SQL concern %q", path, sql)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
