package main

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestParseProvisionRequestRequiresCompleteExplicitIdentity(t *testing.T) {
	t.Parallel()
	request, err := parseProvisionRequest([]string{
		"--id", "org_a",
		"--name", "ACME",
		"--slug", "acme",
		"--clerk-organization-id", "clerk_org_a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.ID != "org_a" ||
		request.Name != "ACME" ||
		request.Slug != "acme" ||
		request.ClerkOrganizationID != "clerk_org_a" {
		t.Fatalf("request=%+v", request)
	}
	if _, err = parseProvisionRequest([]string{"--id", "org_a"}); err == nil {
		t.Fatal("expected incomplete command to fail")
	}
}

func TestProvisionOrgCommandDependsOnlyOnConfigAndCompositionBoundaries(t *testing.T) {
	t.Parallel()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source")
	}
	source := filepath.Join(filepath.Dir(filename), "main.go")
	file, err := parser.ParseFile(token.NewFileSet(), source, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		const internalPrefix = "github.com/devpablocristo/pymes/v3/backend/internal/"
		if len(path) >= len(internalPrefix) &&
			path[:len(internalPrefix)] == internalPrefix {
			t.Fatalf("cmd/provision-org crossed hexagonal boundary: %s", path)
		}
	}
}
