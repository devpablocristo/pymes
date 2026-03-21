package main

import (
	"log"
	"os"

	"github.com/devpablocristo/pymes/pymes-core/backend/wire"
)

func main() {
	app := wire.InitializeApp()
	// Default 8100: mismo host que VITE_API_URL en .env.example y que el mapeo Docker 8100:8080.
	// Si necesitás 8080 (p. ej. otro servicio), exportá PORT=8080 y ajustá VITE_API_URL.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8100"
	}
	if err := app.Router.Run(":" + port); err != nil {
		log.Fatalf("run local server: %v", err)
	}
}
