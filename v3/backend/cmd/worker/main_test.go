package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestWorkerEntrypointIsOnlyLifecycleConfigAndComposition(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(
		token.NewFileSet(),
		"main.go",
		source,
		parser.AllErrors,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(path, "/internal/") ||
			strings.Contains(path, "pgx") ||
			path == "net/http" ||
			path == "time" ||
			path == "encoding/json" {
			t.Errorf("cmd/worker imports implementation dependency %q", path)
		}
	}
	for _, forbidden := range []string{
		"SELECT ", "INSERT ", "UPDATE ", "DELETE ",
		"pgxpool", "DispatchOnce", "NewTicker",
		"ListenAndServe", "/healthz", "/readyz", "/metrics",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("cmd/worker contains runtime concern %q", forbidden)
		}
	}
	var mainFunction *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "main" {
			mainFunction = function
			break
		}
	}
	if mainFunction == nil || mainFunction.Body == nil {
		t.Fatal("main function not found")
	}
	if statements := len(mainFunction.Body.List); statements > 10 {
		t.Errorf("main has %d top-level statements; want at most 10", statements)
	}
}
