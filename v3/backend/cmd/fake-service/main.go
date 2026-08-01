package main

import (
	"log"
	"net/http"
	"os"

	"github.com/devpablocristo/pymes/v3/backend/wire"
)

func main() {
	kind := os.Getenv("FAKE_KIND")
	handler, err := wire.InitializeFakeService(kind)
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
