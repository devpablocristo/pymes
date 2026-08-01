package usecases

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestUsecasesImportOnlyOrganizationDomainAndPorts(t *testing.T) {
	t.Parallel()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source")
	}
	directory := filepath.Dir(filename)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		file, err := parser.ParseFile(
			token.NewFileSet(),
			path,
			nil,
			parser.ImportsOnly,
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			const internalPrefix = "github.com/devpablocristo/pymes/v3/backend/internal/"
			if !strings.HasPrefix(importPath, internalPrefix) {
				continue
			}
			if importPath != internalPrefix+"organization/domain" &&
				importPath != internalPrefix+"organization/ports" {
				t.Fatalf(
					"%s crosses usecase boundary through %s",
					entry.Name(),
					importPath,
				)
			}
		}
	}
}
