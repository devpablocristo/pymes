package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/devpablocristo/pymes/v3/backend/internal/contracts/accountingapi"
	"github.com/devpablocristo/pymes/v3/backend/internal/contracts/fiscalapi"
)

func main() {
	kind := os.Getenv("FAKE_KIND")
	handler, err := handlerForKind(kind)
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func handlerForKind(kind string) (http.Handler, error) {
	switch kind {
	case "accounting":
		return accountingapi.Handler(newAccountingFakeServer()), nil
	case "fiscal":
		return fiscalapi.Handler(newFiscalFakeServer()), nil
	default:
		return nil, fmt.Errorf("FAKE_KIND must be fiscal or accounting")
	}
}
