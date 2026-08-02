package main

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	workerusecases "github.com/devpablocristo/pymes/v3/backend/internal/worker"
	workerdomain "github.com/devpablocristo/pymes/v3/backend/internal/worker/usecases/domain"
	"github.com/devpablocristo/pymes/v3/backend/wire"
)

func TestWorkerEntrypointOwnsLifecycleWithoutConstructingAdapters(t *testing.T) {
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
			path == "encoding/json" {
			t.Errorf("cmd/worker imports implementation dependency %q", path)
		}
	}
	for _, forbidden := range []string{
		"SELECT ", "INSERT ", "UPDATE ", "DELETE ",
		"pgxpool", "DispatchOnce", "NewTicker",
		"/healthz", "/readyz", "/metrics",
		"ReleaseReady", "worker_release_ready",
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

type workerDispatcherFunc func(context.Context) error

func (function workerDispatcherFunc) DispatchOnce(ctx context.Context) error {
	return function(ctx)
}

type workerMetricsStub struct {
	calls atomic.Int64
}

func (metrics *workerMetricsStub) Collect(context.Context) (workerdomain.Metrics, error) {
	metrics.calls.Add(1)
	return workerdomain.Metrics{}, nil
}

type workerReleaseReadyStub struct {
	calls atomic.Int64
}

func (signal *workerReleaseReadyStub) SignalReady(context.Context) {
	signal.calls.Add(1)
}

func TestRunWorkerOwnsServerAndRunnerLifecycle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	metrics := &workerMetricsStub{}
	releaseReady := &workerReleaseReadyStub{}
	var dispatches atomic.Int64
	app := &wire.WorkerApp{
		Server: &http.Server{
			Addr: "127.0.0.1:0", Handler: http.NewServeMux(), ReadHeaderTimeout: time.Second,
		},
		Runner: workerusecases.Runner{
			Dispatcher: workerDispatcherFunc(func(context.Context) error {
				dispatches.Add(1)
				cancel()
				return nil
			}),
			Metrics: metrics, ReleaseReady: releaseReady,
			Logger:        slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
			DispatchEvery: time.Millisecond, MetricsEvery: time.Hour,
		},
	}
	if err := runWorker(ctx, app); err != nil {
		t.Fatal(err)
	}
	if dispatches.Load() != 1 ||
		metrics.calls.Load() != 1 ||
		releaseReady.calls.Load() != 1 {
		t.Fatalf(
			"dispatches=%d metrics=%d release_ready=%d",
			dispatches.Load(),
			metrics.calls.Load(),
			releaseReady.calls.Load(),
		)
	}
}

func TestRunWorkerClassifiesRuntimeFailure(t *testing.T) {
	t.Parallel()
	app := &wire.WorkerApp{
		Server: &http.Server{Addr: "127.0.0.1:0", Handler: http.NewServeMux()},
		Runner: workerusecases.Runner{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := runWorker(ctx, app)
	if err == nil || workerRunErrorCode(err) != "WORKER_RUNTIME_FAILED" {
		t.Fatalf("err=%v code=%q", err, workerRunErrorCode(err))
	}
}
