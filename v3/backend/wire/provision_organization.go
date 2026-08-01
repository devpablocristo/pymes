package wire

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	commercecompanion "github.com/devpablocristo/pymes/v3/backend/internal/commerce/companion"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity/access"
	postgresadapter "github.com/devpablocristo/pymes/v3/backend/internal/infrastructure/postgres"
	organizationcompanion "github.com/devpablocristo/pymes/v3/backend/internal/organization/companion"
	organizationports "github.com/devpablocristo/pymes/v3/backend/internal/organization/ports"
	organizationrepository "github.com/devpablocristo/pymes/v3/backend/internal/organization/repository"
	organizationusecases "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases"
)

var (
	_ organizationports.Directory   = (*organizationrepository.Postgres)(nil)
	_ organizationports.Provisioner = organizationcompanion.DeferredFiscalProvisioner{}
	_ organizationports.Provisioner = commercecompanion.HTTPAccountingProvisioningClient{}
)

type ProvisionOrganizationRequest struct {
	ID                  string
	Name                string
	Slug                string
	ClerkOrganizationID string
}

type ProvisionOrganizationApp struct {
	workflow  organizationusecases.ProvisionOrganization
	database  *postgresadapter.Database
	identity  provisionCloseResource
	closeOnce sync.Once
	closeErr  error
}

func InitializeOrganizationProvisioner(
	ctx context.Context,
	cfg config.ProvisionOrganizationConfig,
) (*ProvisionOrganizationApp, error) {
	if ctx == nil {
		return nil, provisionOrganizationStartupError(
			"PROVISION_STARTUP_FAILED",
			fmt.Errorf("context is required"),
		)
	}
	database, err := postgresadapter.Open(
		ctx,
		cfg.DatabaseURL,
		"pymes-v3-provision-org",
	)
	if err != nil {
		return nil, provisionOrganizationStartupError(
			"DATABASE_UNAVAILABLE",
			err,
		)
	}
	pool := database.Pool()
	tokens, platformTokens, identity, err := provisionOrganizationIdentity(
		ctx,
		cfg,
		func(
			ctx context.Context,
			subject string,
		) (provisionTokenResource, error) {
			return identityaccess.TokenSourceFromRuntimeContext(ctx, subject)
		},
		func() identityaccess.PlatformTokenSource {
			return identityaccess.NewMetadataIDTokenSource()
		},
	)
	if err != nil {
		database.Close()
		return nil, err
	}
	directory := organizationrepository.New(pool)
	return &ProvisionOrganizationApp{
		workflow: organizationusecases.ProvisionOrganization{
			Directory: directory,
			Fiscal:    organizationcompanion.DeferredFiscalProvisioner{},
			Accounting: commercecompanion.HTTPAccountingProvisioningClient{
				BaseURL:        cfg.AccountingProvisioningURL,
				Tokens:         tokens,
				PlatformTokens: platformTokens,
			},
		},
		database: database,
		identity: identity,
	}, nil
}

func (app *ProvisionOrganizationApp) Provision(
	ctx context.Context,
	request ProvisionOrganizationRequest,
) error {
	if app == nil {
		return fmt.Errorf("organization provisioner is not initialized")
	}
	return app.workflow.Execute(
		ctx,
		organizationusecases.ProvisionOrganizationCommand{
			ID:                  request.ID,
			Name:                request.Name,
			Slug:                request.Slug,
			ClerkOrganizationID: request.ClerkOrganizationID,
		},
	)
}

func (app *ProvisionOrganizationApp) Close() error {
	if app == nil {
		return nil
	}
	app.closeOnce.Do(func() {
		if app.identity != nil {
			app.closeErr = app.identity.Close()
		}
		if app.database != nil {
			app.database.Close()
		}
	})
	return app.closeErr
}

type ProvisionOrganizationStartupError struct {
	Code string
	Err  error
}

func (err *ProvisionOrganizationStartupError) Error() string {
	return err.Err.Error()
}

func (err *ProvisionOrganizationStartupError) Unwrap() error {
	return err.Err
}

func ProvisionOrganizationStartupErrorCode(err error) string {
	var startupErr *ProvisionOrganizationStartupError
	if errors.As(err, &startupErr) && startupErr.Code != "" {
		return startupErr.Code
	}
	return "PROVISION_STARTUP_FAILED"
}

type provisionTokenResource interface {
	identityaccess.TokenSource
	Close() error
}

type provisionCloseResource interface {
	Close() error
}

type provisionTokenFactory func(
	context.Context,
	string,
) (provisionTokenResource, error)

type provisionPlatformTokenFactory func() identityaccess.PlatformTokenSource

func provisionOrganizationIdentity(
	ctx context.Context,
	cfg config.ProvisionOrganizationConfig,
	tokenFactory provisionTokenFactory,
	platformFactory provisionPlatformTokenFactory,
) (
	identityaccess.TokenSource,
	identityaccess.PlatformTokenSource,
	provisionCloseResource,
	error,
) {
	if cfg.AllowInsecureLocalServices &&
		strings.EqualFold(strings.TrimSpace(cfg.Environment), "production") {
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("insecure local platform identity is forbidden in production"),
		)
	}
	if tokenFactory == nil {
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("internal token factory is required"),
		)
	}
	tokens, err := tokenFactory(ctx, "provision-org")
	if err != nil {
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			err,
		)
	}
	if tokens == nil {
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("internal token source is required"),
		)
	}
	if cfg.AllowInsecureLocalServices {
		return tokens, nil, tokens, nil
	}
	if platformFactory == nil {
		_ = tokens.Close()
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("platform token factory is required"),
		)
	}
	platformTokens := platformFactory()
	if platformTokens == nil {
		_ = tokens.Close()
		return nil, nil, nil, provisionOrganizationStartupError(
			"WORKLOAD_IDENTITY_INVALID",
			fmt.Errorf("platform token source is required"),
		)
	}
	return tokens, platformTokens, tokens, nil
}

func provisionOrganizationStartupError(code string, err error) error {
	return &ProvisionOrganizationStartupError{Code: code, Err: err}
}
