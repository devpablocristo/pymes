package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	"github.com/devpablocristo/pymes/v3/backend/wire"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config, err := config.LoadPerGoFake()
	if err != nil {
		logger.Error("PerGo fake startup failed", "code", "CONFIG_INVALID")
		return
	}
	server := &http.Server{
		Addr:              config.HTTPAddr,
		Handler:           wire.InitializePerGoFake(config),
		ReadHeaderTimeout: config.Delay,
	}
	if err = server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		logger.Error("PerGo fake stopped", "code", "SERVER_FAILED")
	}
}
