package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	"github.com/devpablocristo/pymes/v3/backend/wire"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv); err != nil {
		log.Printf(
			"organization provisioning failed: code=%s",
			provisionErrorCode(err),
		)
		os.Exit(1)
	}
}

func run(
	ctx context.Context,
	args []string,
	getenv func(string) string,
) error {
	request, err := parseProvisionRequest(args)
	if err != nil {
		return &provisionError{Code: "INPUT_INVALID", Err: err}
	}
	cfg, err := config.LoadProvisionOrganizationFrom(getenv)
	if err != nil {
		return &provisionError{
			Code: config.ProvisionOrganizationErrorCode(err),
			Err:  err,
		}
	}
	app, err := wire.InitializeOrganizationProvisioner(ctx, cfg)
	if err != nil {
		return &provisionError{
			Code: wire.ProvisionOrganizationStartupErrorCode(err),
			Err:  err,
		}
	}
	provisionErr := app.Provision(ctx, request)
	closeErr := app.Close()
	if provisionErr != nil {
		return errors.Join(
			&provisionError{Code: "PROVISIONING_FAILED", Err: provisionErr},
			closeProvisionError(closeErr),
		)
	}
	if closeErr != nil {
		return closeProvisionError(closeErr)
	}
	log.Printf("organization %s ready", request.ID)
	return nil
}

func parseProvisionRequest(
	args []string,
) (wire.ProvisionOrganizationRequest, error) {
	flags := flag.NewFlagSet("provision-org", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	id := flags.String("id", "", "organization ID")
	name := flags.String("name", "", "organization name")
	slug := flags.String("slug", "", "organization slug")
	clerkOrganizationID := flags.String(
		"clerk-organization-id",
		"",
		"verified Clerk organization ID",
	)
	if err := flags.Parse(args); err != nil {
		return wire.ProvisionOrganizationRequest{}, err
	}
	if flags.NArg() != 0 ||
		*id == "" ||
		*name == "" ||
		*slug == "" ||
		*clerkOrganizationID == "" {
		return wire.ProvisionOrganizationRequest{}, fmt.Errorf(
			"--id, --name, --slug and --clerk-organization-id are required",
		)
	}
	return wire.ProvisionOrganizationRequest{
		ID:                  *id,
		Name:                *name,
		Slug:                *slug,
		ClerkOrganizationID: *clerkOrganizationID,
	}, nil
}

type provisionError struct {
	Code string
	Err  error
}

func (err *provisionError) Error() string {
	return err.Err.Error()
}

func (err *provisionError) Unwrap() error {
	return err.Err
}

func provisionErrorCode(err error) string {
	var coded *provisionError
	if errors.As(err, &coded) && coded.Code != "" {
		return coded.Code
	}
	return "PROVISIONING_FAILED"
}

func closeProvisionError(err error) error {
	if err == nil {
		return nil
	}
	return &provisionError{Code: "IDENTITY_SHUTDOWN_FAILED", Err: err}
}
