package organization

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestUsecaseDomainDoesNotDependOnAdapters(t *testing.T) {
	t.Parallel()

	err := filepath.WalkDir(
		"usecases/domain",
		func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() ||
				filepath.Ext(path) != ".go" ||
				strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(
				token.NewFileSet(),
				path,
				nil,
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
				if strings.Contains(
					importPath,
					"github.com/devpablocristo/pymes/v3/backend/internal/",
				) {
					t.Errorf(
						"%s imports internal adapter or composition package %q",
						path,
						importPath,
					)
				}
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}
