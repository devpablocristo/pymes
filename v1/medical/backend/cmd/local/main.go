package main

import (
	"log"
	"os"

	"github.com/devpablocristo/pymes/medical/backend/wire"
)

func main() {
	app := wire.InitializeApp()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}
	if err := app.Router.Run(":" + port); err != nil {
		log.Fatalf("run local server: %v", err)
	}
}
