package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const modulePrefix = "github.com/devpablocristo/pymes/v3/backend"

var axisWord = regexp.MustCompile(`(?i)\baxis\b`)

var serializedFieldTag = regexp.MustCompile(
	`(?:^|\s)(?:json|xml|yaml|db|form|query|header):`,
)

var misplacedAdapterHelper = regexp.MustCompile(
	`(?i)^(?:attach|decode|digest|encode|fallback|generated.*error|hash|map|normalize|nullable|parse|positive|stable|transcode|translate|write(?:error|json|problem)$)`,
)

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

type adapterDefinition struct {
	name string
	kind string
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

func TestAdapterBoundaryImportsCannotBypassRootMarkers(t *testing.T) {
	root := backendRoot(t)
	files := productionGoFiles(t, root)
	definitions := adapterDefinitions(files)
	for _, file := range files {
		if !strings.HasPrefix(filepath.ToSlash(file.relative), "internal/") ||
			strings.Count(filepath.ToSlash(file.relative), "/") != 2 {
			continue
		}
		imports := adapterBoundaryImports(file)
		if len(imports) == 0 {
			continue
		}
		if _, ok := adapterDefinitionFor(file, definitions); ok {
			continue
		}
		t.Errorf(
			"%s imports adapter boundary %q but has no architecture:adapter marker "+
				"and is not a fragment of a marked adapter",
			file.relative,
			imports,
		)
	}
}

func TestEveryContextRootFileIsUseCaseOrMarkedAdapter(t *testing.T) {
	root := backendRoot(t)
	files := productionGoFiles(t, root)
	definitions := adapterDefinitions(files)
	for _, file := range files {
		path := filepath.ToSlash(file.relative)
		if !strings.HasPrefix(path, "internal/") ||
			strings.Count(path, "/") != 2 {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(path), ".go")
		if base == "usecases" || strings.HasPrefix(base, "usecases_") {
			continue
		}
		if _, ok := adapterDefinitionFor(file, definitions); ok {
			continue
		}
		t.Errorf(
			"%s is an unclassified context-root file; external adapters require "+
				"an architecture:adapter marker and adapter fragments must share "+
				"the marked root prefix",
			file.relative,
		)
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

func TestContextsDoNotImportForeignRepositoryOrHandler(t *testing.T) {
	root := backendRoot(t)
	for _, file := range productionGoFiles(t, root) {
		parts := strings.Split(filepath.ToSlash(file.relative), "/")
		if len(parts) < 3 || parts[0] != "internal" {
			continue
		}
		consumer := parts[1]
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, modulePrefix+"/internal/") {
				continue
			}
			target := strings.TrimPrefix(imported, modulePrefix+"/internal/")
			targetParts := strings.Split(target, "/")
			if len(targetParts) < 2 || targetParts[0] == consumer {
				continue
			}
			if targetParts[1] == "repository" || targetParts[1] == "handler" {
				t.Errorf(
					"%s imports foreign %s adapter %q",
					file.relative,
					targetParts[1],
					imported,
				)
			}
		}
	}
}

func TestAdapterRootsKeepBoundaryDataAndTransformationsInSubpackages(t *testing.T) {
	root := backendRoot(t)
	files := productionGoFiles(t, root)
	definitions := adapterDefinitions(files)
	methodReceivers := make(map[string]map[string]struct{})
	for _, file := range files {
		contextName := contextFor(file.relative)
		if contextName == "" {
			continue
		}
		if methodReceivers[contextName] == nil {
			methodReceivers[contextName] = make(map[string]struct{})
		}
		for _, declaration := range file.syntax.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || len(function.Recv.List) == 0 {
				continue
			}
			if receiver := receiverName(function.Recv.List[0].Type); receiver != "" {
				methodReceivers[contextName][receiver] = struct{}{}
			}
		}
	}

	for _, file := range files {
		if _, ok := adapterDefinitionFor(file, definitions); !ok {
			continue
		}
		contextName := contextFor(file.relative)
		for _, violation := range adapterBoundaryViolations(
			file,
			methodReceivers[contextName],
		) {
			t.Errorf("%s %s", file.relative, violation)
		}
	}
}

func TestAdapterFragmentsCannotHideBoundaryDataOrHelpers(t *testing.T) {
	rootSource := []byte(`// architecture:adapter repository
package sample

type Repository struct{}

func (Repository) Ping() {}
`)
	fragmentSource := []byte(`package sample

type hiddenPayload struct {
	ID string ` + "`json:\"id\"`" + `
}

func decodeHiddenPayload() {}

func writeJSON() {}
`)
	rootFile := mustParseSyntheticGoFile(
		t,
		"internal/sample/repository.go",
		rootSource,
	)
	fragmentFile := mustParseSyntheticGoFile(
		t,
		"internal/sample/repository_payload.go",
		fragmentSource,
	)
	definitions := adapterDefinitions([]parsedGoFile{rootFile, fragmentFile})
	if _, ok := adapterDefinitionFor(fragmentFile, definitions); !ok {
		t.Fatal("repository fragment escaped adapter discovery")
	}
	violations := adapterBoundaryViolations(
		fragmentFile,
		map[string]struct{}{},
	)
	if len(violations) < 3 {
		t.Fatalf("fragment violations = %v, want serialized data and helper violations", violations)
	}
}

func TestAdapterBoundaryImportDetection(t *testing.T) {
	file := mustParseSyntheticGoFile(
		t,
		"internal/sample/http_client.go",
		[]byte(`package sample

import "net/http"

var _ = http.MethodGet
`),
	)
	got := adapterBoundaryImports(file)
	if len(got) != 1 || got[0] != "net/http" {
		t.Fatalf("adapter boundary imports = %v, want [net/http]", got)
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
			continue
		}
		for _, construction := range concreteForeignContextConstructions(file) {
			t.Errorf(
				"%s constructs concrete context dependency %s outside wire",
				path,
				construction,
			)
		}
	}
}

func TestLegacyLayersAndContractsStayRemoved(t *testing.T) {
	root := backendRoot(t)
	for _, name := range []string{
		"access",
		"companion",
		"contracts",
		"domain",
		"handler",
		"infrastructure",
		"ports",
		"repository",
	} {
		path := filepath.Join(root, "internal", name)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("forbidden global layer directory remains: internal/%s", name)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect forbidden global layer %s: %v", path, err)
		}
	}

	for _, file := range productionGoFiles(t, root) {
		path := filepath.ToSlash(file.relative)
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

func TestConcreteForeignContextConstructionDetection(t *testing.T) {
	source := []byte(`package sample

import (
	"net/http"

	"github.com/devpablocristo/pymes/v3/backend/internal/commerce"
	"github.com/devpablocristo/pymes/v3/backend/internal/identity"
)

func build() {
	_ = http.NewServeMux()
	_ = commerce.New(nil)
	_ = identity.New(nil)
}
`)
	syntax, err := parser.ParseFile(token.NewFileSet(), "internal/sample/build.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	file := parsedGoFile{
		relative: "internal/sample/build.go",
		source:   source,
		syntax:   syntax,
	}
	got := concreteForeignContextConstructions(file)
	if len(got) != 2 || got[0] != "commerce.New" || got[1] != "identity.New" {
		t.Fatalf("foreign context constructions = %v, want [commerce.New identity.New]", got)
	}
}

func TestAxisHasNoDependencyPathOrRuntimeContact(t *testing.T) {
	root := backendRoot(t)
	for _, file := range productionGoFiles(t, root) {
		if axisWord.Match(file.source) {
			t.Errorf("Axis technical reference in production Go file %s", file.relative)
		}
	}

	repositoryRoot := filepath.Clean(filepath.Join(root, "..", ".."))
	paths := []string{
		filepath.Join(root, "go.mod"),
		filepath.Join(root, "go.sum"),
		filepath.Join(repositoryRoot, "v3", "Dockerfile"),
		filepath.Join(repositoryRoot, "v3", "docker-compose.yml"),
		filepath.Join(repositoryRoot, "v3", "Makefile"),
		filepath.Join(repositoryRoot, ".github", "workflows"),
		filepath.Join(repositoryRoot, "v3", "scripts"),
	}
	for _, path := range paths {
		err := filepath.WalkDir(path, func(candidate string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Name() == "workflow-policy-check.sh" {
				return nil
			}
			extension := filepath.Ext(entry.Name())
			switch {
			case entry.Name() == "Dockerfile", entry.Name() == "Makefile":
			case extension == ".go" && strings.HasSuffix(entry.Name(), "_test.go"):
				return nil
			case extension == ".go", extension == ".mod", extension == ".sum",
				extension == ".sh", extension == ".yml", extension == ".yaml":
			default:
				return nil
			}
			source, err := os.ReadFile(candidate)
			if err != nil {
				return err
			}
			if axisWord.Match(source) {
				relative, relErr := filepath.Rel(repositoryRoot, candidate)
				if relErr != nil {
					return relErr
				}
				t.Errorf("Axis technical reference in dependency/runtime file %s", relative)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" ||
			!strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		syntax, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, imported := range axisImports(syntax) {
			t.Errorf("Axis test-only import in %s: %s", filepath.ToSlash(relative), imported)
		}
		relative = filepath.ToSlash(relative)
		isArchitectureGuard := relative == "internal/architecture/architecture_test.go" ||
			relative == "internal/scheduling/architecture_test.go"
		if !isArchitectureGuard && axisWord.Match(source) {
			t.Errorf("Axis technical reference in test Go file %s", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	command := exec.Command(
		"go",
		"list",
		"-deps",
		"-test",
		"-buildvcs=false",
		"./...",
	)
	command.Dir = root
	command.Env = append(
		os.Environ(),
		"GOCACHE="+filepath.Join(t.TempDir(), "go-build"),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps -test -buildvcs=false: %v\n%s", err, output)
	}
	if axisWord.Match(output) {
		t.Fatalf("Axis appears in Go dependency graph including tests:\n%s", output)
	}
}

func TestAxisTestOnlyImportDetection(t *testing.T) {
	source := []byte(`package sample

import forbidden "github.com/example/axis/pkg"

var _ = forbidden.Value
`)
	syntax, err := parser.ParseFile(token.NewFileSet(), "sample_test.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := axisImports(syntax)
	if len(got) != 1 || got[0] != "github.com/example/axis/pkg" {
		t.Fatalf("Axis imports = %v", got)
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
		file.adapter = adapterMarker(syntax)
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

func adapterDefinitions(files []parsedGoFile) map[string][]adapterDefinition {
	result := make(map[string][]adapterDefinition)
	for _, file := range files {
		if file.adapter == "" ||
			strings.Count(filepath.ToSlash(file.relative), "/") != 2 {
			continue
		}
		contextName := contextFor(file.relative)
		name := strings.TrimSuffix(filepath.Base(file.relative), ".go")
		result[contextName] = append(result[contextName], adapterDefinition{
			name: name,
			kind: file.adapter,
		})
	}
	return result
}

func adapterDefinitionFor(
	file parsedGoFile,
	definitions map[string][]adapterDefinition,
) (adapterDefinition, bool) {
	if strings.Count(filepath.ToSlash(file.relative), "/") != 2 {
		return adapterDefinition{}, false
	}
	base := strings.TrimSuffix(filepath.Base(file.relative), ".go")
	for _, definition := range definitions[contextFor(file.relative)] {
		if base == definition.name ||
			strings.HasPrefix(base, definition.name+"_") {
			return definition, true
		}
	}
	return adapterDefinition{}, false
}

func adapterBoundaryViolations(
	file parsedGoFile,
	methodReceivers map[string]struct{},
) []string {
	var result []string
	implementationStructs := make(map[*ast.StructType]struct{})
	for _, declaration := range file.syntax.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, raw := range general.Specs {
			spec, ok := raw.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structure, ok := spec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			if _, implementation := methodReceivers[spec.Name.Name]; implementation {
				implementationStructs[structure] = struct{}{}
			}
		}
	}
	ast.Inspect(file.syntax, func(node ast.Node) bool {
		structure, ok := node.(*ast.StructType)
		if !ok {
			return true
		}
		if _, implementation := implementationStructs[structure]; !implementation {
			result = append(
				result,
				"declares adapter data struct in its root; move it to dto/models",
			)
		}
		for _, field := range structure.Fields.List {
			if field.Tag == nil {
				continue
			}
			tag, err := strconv.Unquote(field.Tag.Value)
			if err == nil && serializedFieldTag.MatchString(tag) {
				result = append(
					result,
					"declares serialized adapter data in its root; move it to dto/models",
				)
			}
		}
		return true
	})
	for _, declaration := range file.syntax.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || ast.IsExported(function.Name.Name) ||
			!misplacedAdapterHelper.MatchString(function.Name.Name) {
			continue
		}
		result = append(
			result,
			"declares adapter transformation "+function.Name.Name+
				" in its root; move it to helpers",
		)
	}
	return result
}

func mustParseSyntheticGoFile(
	t *testing.T,
	relative string,
	source []byte,
) parsedGoFile {
	t.Helper()
	syntax, err := parser.ParseFile(
		token.NewFileSet(),
		relative,
		source,
		parser.ParseComments,
	)
	if err != nil {
		t.Fatal(err)
	}
	file := parsedGoFile{
		relative: relative,
		source:   source,
		syntax:   syntax,
		adapter:  adapterMarker(syntax),
	}
	for _, imported := range syntax.Imports {
		value, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		file.imports = append(file.imports, value)
	}
	return file
}

func adapterMarker(file *ast.File) string {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			const marker = "architecture:adapter "
			index := strings.Index(comment.Text, marker)
			if index < 0 {
				continue
			}
			fields := strings.Fields(comment.Text[index+len(marker):])
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return ""
}

func contextFor(relative string) string {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) < 3 || parts[0] != "internal" {
		return ""
	}
	return parts[1]
}

func receiverName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverName(value.X)
	case *ast.IndexExpr:
		return receiverName(value.X)
	case *ast.IndexListExpr:
		return receiverName(value.X)
	default:
		return ""
	}
}

func concreteForeignContextConstructions(file parsedGoFile) []string {
	imports := importAliases(file.syntax)
	consumer := contextFor(file.relative)
	var result []string
	ast.Inspect(file.syntax, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !strings.HasPrefix(selector.Sel.Name, "New") {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		imported := imports[identifier.Name]
		const internalPrefix = modulePrefix + "/internal/"
		if !strings.HasPrefix(imported, internalPrefix) {
			return true
		}
		target := strings.TrimPrefix(imported, internalPrefix)
		if target == "" || strings.Contains(target, "/") || target == consumer {
			return true
		}
		result = append(result, identifier.Name+"."+selector.Sel.Name)
		return true
	})
	return result
}

func axisImports(file *ast.File) []string {
	var result []string
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err == nil && axisWord.MatchString(imported) {
			result = append(result, imported)
		}
	}
	return result
}

func importAliases(file *ast.File) map[string]string {
	result := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(imported)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		result[name] = imported
	}
	return result
}

func adapterBoundaryImports(file parsedGoFile) []string {
	var result []string
	for _, imported := range file.imports {
		switch {
		case imported == "database/sql", imported == "net/http":
			result = append(result, imported)
		case strings.Contains(imported, "pgx"):
			result = append(result, imported)
		case strings.HasPrefix(imported, "cloud.google.com/"):
			result = append(result, imported)
		case strings.HasPrefix(imported, "github.com/devpablocristo/platform/"):
			result = append(result, imported)
		}
	}
	return result
}
