package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	commercecompanion "github.com/devpablocristo/pymes/v3/backend/internal/commerce/companion"
	"github.com/devpablocristo/pymes/v3/backend/internal/commerce/repository"
	"github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity/access"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseURL := os.Getenv("PYMES_DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("PYMES_DATABASE_URL is required")
	}
	fiscalURL, accountingURL := os.Getenv("FISCAL_ADAPTER_URL"), os.Getenv("ACCOUNTING_URL")
	if fiscalURL == "" || accountingURL == "" {
		log.Fatal("FISCAL_ADAPTER_URL and ACCOUNTING_URL are required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	var tokens identityaccess.TokenSource
	if os.Getenv("PYMES_ALLOW_INSECURE_LOCAL_SERVICES") != "true" {
		tokens, err = identityaccess.TokenSourceFromRuntime("worker:outbox")
		if err != nil {
			log.Fatal(err)
		}
	}
	fiscalClient := commercecompanion.NewServiceHTTPClient()
	accountingClient := commercecompanion.NewServiceHTTPClient()
	worker := usecases.DurableWorker{
		Store:      repository.New(pool),
		Fiscal:     commercecompanion.HTTPFiscalClient{BaseURL: fiscalURL, Client: fiscalClient, Tokens: tokens},
		Accounting: commercecompanion.HTTPAccountingClient{BaseURL: accountingURL, Client: accountingClient, Tokens: tokens},
		LeaseFor:   30 * time.Second,
	}
	interval := time.Second
	if os.Getenv("PYMES_WORKER_INTERVAL_MS") != "" {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	health := &http.Server{
		Addr:              defaultValue(os.Getenv("PYMES_WORKER_HTTP_ADDR"), ":8080"),
		Handler:           healthHandler(pool, map[string]circuitState{"fiscal": fiscalClient, "accounting": accountingClient}),
		ReadHeaderTimeout: 2 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		if err := health.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := health.Shutdown(shutdown); err != nil {
			log.Printf("worker health shutdown: %v", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-serverErrors:
			log.Fatal(err)
		case <-ticker.C:
			if err := worker.DispatchOnce(ctx); err != nil {
				log.Printf("dispatch: %v", err)
			}
		}
	}
}

type circuitState interface {
	CircuitOpen() bool
}

func healthHandler(pool *pgxpool.Pool, circuits map[string]circuitState) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		var outboxPending, outboxLeased, fiscalUncertain, applicationPending, reversalPending int64
		err := pool.QueryRow(ctx, `
			SELECT
			  (SELECT count(*) FROM app.outbox WHERE published_at IS NULL),
			  (SELECT count(*) FROM app.outbox WHERE published_at IS NULL AND lease_expires_at > now()),
			  (SELECT count(*) FROM app.sales WHERE status = 'fiscal_uncertain'),
			  (SELECT count(*) FROM app.accounting_application_commands WHERE status = 'pending'),
			  (SELECT count(*) FROM app.accounting_reversals WHERE status = 'requested')`).
			Scan(&outboxPending, &outboxLeased, &fiscalUncertain, &applicationPending, &reversalPending)
		if err != nil {
			http.Error(w, "metrics unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = fmt.Fprintf(w,
			"pymes_outbox_pending %d\npymes_outbox_leased %d\npymes_fiscal_uncertain %d\npymes_accounting_applications_pending %d\npymes_accounting_reversals_pending %d\n",
			outboxPending, outboxLeased, fiscalUncertain, applicationPending, reversalPending)
		for name, circuit := range circuits {
			value := 0
			if circuit.CircuitOpen() {
				value = 1
			}
			_, _ = fmt.Fprintf(w, "pymes_dependency_circuit_open{dependency=%q} %d\n", name, value)
		}
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func defaultValue(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
