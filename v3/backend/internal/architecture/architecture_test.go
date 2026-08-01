package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const modulePrefix = "github.com/devpablocristo/pymes/v3/backend"

type parsedGoFile struct {
	path       string
	relative   string
	source     []byte
	syntax     *ast.File
	imports    []string
	generated  bool
	adapter    string
	interfaces int
}

func TestAdaptersHaveRootDataAndHelpers(t *testing.T) {
	root := backendRoot(t)
	files := productionGoFiles(t, root)
	byRelative := make(map[string]parsedGoFile, len(files))
	for _, file := range files {
		byRelative[file.relative] = file
	}

	discovered := 0
	for _, file := range files {
		if !strings.HasPrefix(file.relative, "internal/") ||
			strings.Count(filepath.ToSlash(file.relative), "/") != 2 {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(file.relative), ".go")
		if base == "handler" || base == "repository" || base == "worker" {
			if file.adapter != base {
				t.Errorf("%s must declare architecture:adapter %s", file.relative, base)
			}
		}
		if file.adapter == "" {
			continue
		}
		discovered++
		dataDirectory := "models"
		adapterDirectory := base
		switch file.adapter {
		case "handler":
			if base != "handler" {
				t.Errorf("%s: handler adapter root must be handler.go", file.relative)
			}
			dataDirectory = "dto"
		case "repository", "worker":
			if base != file.adapter {
				t.Errorf("%s: %s adapter root must be %s.go", file.relative, file.adapter, file.adapter)
			}
		case "external":
		default:
			t.Errorf("%s: unknown adapter kind %q", file.relative, file.adapter)
			continue
		}
		parent := filepath.Dir(filepath.Join(root, file.relative))
		adapterRoot := filepath.Join(parent, adapterDirectory)
		if filepath.Base(parent) == adapterDirectory {
			adapterRoot = parent
		}
		assertPackageEvidence(t, filepath.Join(adapterRoot, dataDirectory), "model")
		assertPackageEvidence(t, filepath.Join(adapterRoot, "helpers"), "helper")
	}
	if discovered == 0 {
		t.Fatal("no architecture adapters discovered")
	}

	for _, file := range files {
		parts := strings.Split(filepath.ToSlash(file.relative), "/")
		if len(parts) != 5 || parts[0] != "internal" ||
			(parts[3] != "models" && parts[3] != "dto" && parts[3] != "helpers") {
			continue
		}
		rootFile := filepath.ToSlash(filepath.Join(parts[0], parts[1], parts[2]+".go"))
		if _, ok := byRelative[rootFile]; !ok {
			t.Errorf("%s belongs to adapter without root %s", file.relative, rootFile)
		}
	}
}

func TestDependencyDirectionAndConsumerOwnedInterfaces(t *testing.T) {
	root := backendRoot(t)
	for _, file := range productionGoFiles(t, root) {
		path := filepath.ToSlash(file.relative)
		if strings.Contains(path, "/usecases/domain/") {
			for _, imported := range file.imports {
				if strings.HasPrefix(imported, modulePrefix+"/internal/") ||
					imported == "net/http" ||
					strings.Contains(imported, "pgx") ||
					strings.HasPrefix(imported, "cloud.google.com/") ||
					strings.HasPrefix(imported, "github.com/devpablocristo/platform/") {
					t.Errorf("%s domain imports adapter dependency %q", path, imported)
				}
			}
		}
		if strings.Contains(path, "/models/") ||
			strings.Contains(path, "/dto/") ||
			strings.Contains(path, "/helpers/") {
			if file.interfaces > 0 && !file.generated {
				t.Errorf("%s declares interface in adapter data/helper package", path)
			}
		}
		if filepath.Base(path) == "repository.go" && file.interfaces > 0 {
			t.Errorf("%s declares provider-owned repository interface", path)
		}
		if strings.HasPrefix(filepath.Base(path), "usecases") {
			for _, imported := range file.imports {
				if imported == "net/http" || strings.Contains(imported, "pgx") ||
					strings.HasPrefix(imported, "cloud.google.com/") {
					t.Errorf("%s use case imports adapter dependency %q", path, imported)
				}
			}
		}
	}
}

func TestCompositionLivesInWireAndLifecycleLivesInCmd(t *testing.T) {
	root := backendRoot(t)
	for _, file := range productionGoFiles(t, root) {
		path := filepath.ToSlash(file.relative)
		if strings.HasPrefix(path, "cmd/") && !strings.HasPrefix(path, "cmd/config/") {
			for _, imported := range file.imports {
				if strings.HasPrefix(imported, modulePrefix+"/internal/") ||
					strings.Contains(imported, "pgx") ||
					strings.HasPrefix(imported, "cloud.google.com/") ||
					strings.HasPrefix(imported, "github.com/devpablocristo/platform/") {
					t.Errorf("%s composes implementation dependency %q", path, imported)
				}
			}
		}
		if strings.HasPrefix(path, "wire/") {
			ast.Inspect(file.syntax, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch selector.Sel.Name {
				case "ListenAndServe", "ListenAndServeTLS", "Serve", "Shutdown", "NotifyContext":
					t.Errorf("%s owns lifecycle call %s", path, selector.Sel.Name)
				}
				return true
			})
		}
	}
}

func TestLegacyLayersAndContractsStayRemoved(t *testing.T) {
	root := backendRoot(t)
	for _, file := range productionGoFiles(t, root) {
		path := filepath.ToSlash(file.relative)
		if strings.HasPrefix(path, "internal/contracts/") {
			t.Errorf("legacy contract package remains: %s", path)
		}
		parts := strings.Split(path, "/")
		if len(parts) < 3 || parts[0] != "internal" {
			continue
		}
		switch parts[2] {
		case "access", "companion", "ports", "infrastructure":
			t.Errorf("legacy layer remains: %s", path)
		case "domain":
			t.Errorf("domain must live under usecases/domain: %s", path)
		case "handler", "repository", "usecases":
			if len(parts) == 4 {
				t.Errorf("legacy layer package contains Go source: %s", path)
			}
		}
	}
}

func backendRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func productionGoFiles(t *testing.T, root string) []parsedGoFile {
	t.Helper()
	var result []parsedGoFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		syntax, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ParseComments)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file := parsedGoFile{
			path: path, relative: filepath.ToSlash(relative), source: source, syntax: syntax,
			generated: strings.Contains(string(source), "Code generated"),
		}
		for _, imported := range syntax.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			file.imports = append(file.imports, value)
		}
		for _, group := range syntax.Comments {
			for _, comment := range group.List {
				const marker = "architecture:adapter "
				if index := strings.Index(comment.Text, marker); index >= 0 {
					file.adapter = strings.Fields(comment.Text[index+len(marker):])[0]
				}
			}
		}
		ast.Inspect(syntax, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.InterfaceType); ok {
				file.interfaces++
			}
			return true
		})
		result = append(result, file)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertPackageEvidence(t *testing.T, directory, kind string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Errorf("%s package missing: %v", directory, err)
		return
	}
	found := false
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		syntax, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			continue
		}
		for _, declaration := range syntax.Decls {
			switch kind {
			case "helper":
				if _, ok := declaration.(*ast.FuncDecl); ok {
					found = true
				}
			case "model":
				general, ok := declaration.(*ast.GenDecl)
				if !ok || general.Tok != token.TYPE {
					continue
				}
				if len(general.Specs) > 0 {
					found = true
				}
			}
		}
	}
	if !found {
		t.Errorf("%s has no real %s declaration", directory, kind)
	}
}
