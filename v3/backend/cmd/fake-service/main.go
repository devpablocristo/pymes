package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
)

func main() {
	kind := os.Getenv("FAKE_KIND")
	if kind != "fiscal" && kind != "accounting" {
		log.Fatal("FAKE_KIND must be fiscal or accounting")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	var mu sync.Mutex
	results := map[string]map[string]any{}
	handler := http.NewServeMux()
	handler.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	handler.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})
	handler.HandleFunc("PUT /internal/v1/organizations/{organizationID}", func(w http.ResponseWriter, r *http.Request) {
		if kind != "accounting" {
			http.NotFound(w, r)
			return
		}
		key := "provision:" + r.PathValue("organizationID")
		mu.Lock()
		_, found := results[key]
		if !found {
			results[key] = map[string]any{"organization_id": r.PathValue("organizationID")}
		}
		mu.Unlock()
		if found {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
	handler.HandleFunc("POST /internal/v1/organizations/{organizationID}/authorizations", func(w http.ResponseWriter, r *http.Request) {
		if kind != "fiscal" {
			http.NotFound(w, r)
			return
		}
		var request struct {
			RequestID     string `json:"request_id"`
			VoucherNumber int    `json:"voucher_number"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		mu.Lock()
		defer mu.Unlock()
		result, found := results[request.RequestID]
		if !found {
			result = map[string]any{"request_id": request.RequestID, "organization_id": r.PathValue("organizationID"), "status": "authorized", "cae": fmt.Sprintf("CAE-%d", request.VoucherNumber)}
			results[request.RequestID] = result
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
	handler.HandleFunc("POST /internal/v1/organizations/{organizationID}/posting-commands", func(w http.ResponseWriter, r *http.Request) {
		if kind != "accounting" {
			http.NotFound(w, r)
			return
		}
		var request struct {
			CommandID string `json:"command_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		mu.Lock()
		defer mu.Unlock()
		result, found := results[request.CommandID]
		status := "posted"
		if found {
			status = "duplicate"
		} else {
			result = map[string]any{"command_id": request.CommandID, "organization_id": r.PathValue("organizationID"), "journal_entry_id": "fake-je-1"}
			results[request.CommandID] = result
		}
		result["status"] = status
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
