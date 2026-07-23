package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/provisioning"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if exitCode := run(ctx, os.Args[1:], os.Stdout, os.Stderr, os.Getenv); exitCode != 0 {
		os.Exit(exitCode)
	}
}

func run(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	getenv func(string) string,
) int {
	flags := flag.NewFlagSet("provision-org", flag.ContinueOnError)
	flags.SetOutput(stderr)
	name := flags.String("name", "", "organization name")
	slug := flags.String("slug", "", "stable organization slug")
	ownerEmail := flags.String("owner-email", "", "initial owner email")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	databaseURL := strings.TrimSpace(getenv("PYMES_DATABASE_URL"))
	if databaseURL == "" {
		writeError(stderr, "IAM_PROVISION_DATABASE_REQUIRED", "PYMES_DATABASE_URL is required")
		return 2
	}

	database, err := postgres.OpenWithConfig(
		ctx,
		databaseURL,
		postgres.DefaultConfig("pymes-v2-provision-org"),
	)
	if err != nil {
		writeError(stderr, "IAM_PROVISION_DATABASE_UNAVAILABLE", err.Error())
		return 1
	}
	defer database.Close()

	service, err := provisioning.NewService(database.Pool())
	if err != nil {
		writeError(stderr, errorCode(err), err.Error())
		return 1
	}
	result, err := service.Provision(ctx, provisioning.Input{
		Name:       *name,
		Slug:       *slug,
		OwnerEmail: *ownerEmail,
	})
	if err != nil {
		writeError(stderr, errorCode(err), err.Error())
		if errors.Is(err, provisioning.ErrInvalidInput) {
			return 2
		}
		if errors.Is(err, provisioning.ErrPayloadConflict) ||
			errors.Is(err, provisioning.ErrSlugConflict) {
			return 3
		}
		return 1
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		writeError(stderr, "IAM_PROVISION_OUTPUT_FAILED", err.Error())
		return 1
	}
	return 0
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, provisioning.ErrInvalidInput):
		return provisioning.ErrInvalidInput.Error()
	case errors.Is(err, provisioning.ErrPayloadConflict):
		return provisioning.ErrPayloadConflict.Error()
	case errors.Is(err, provisioning.ErrSlugConflict):
		return provisioning.ErrSlugConflict.Error()
	default:
		return "IAM_PROVISION_FAILED"
	}
}

func writeError(writer io.Writer, code, message string) {
	_ = json.NewEncoder(writer).Encode(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{
		Code:    code,
		Message: fmt.Sprintf("provision organization: %s", message),
	})
}
