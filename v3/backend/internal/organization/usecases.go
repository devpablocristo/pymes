// Package usecases contains organization application workflows.
package organization

import (
	"context"
	"errors"
	"fmt"
	"strings"

	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
)

const (
	accountingService = "accounting"
	fiscalService     = "fiscal"
)

type Directory interface {
	SyncClerk(context.Context, string, organizationdomain.Organization) error
	SetProvisioningStatus(context.Context, string, string, string, string) error
	SetStatus(context.Context, string, organizationdomain.Status) error
}

type Provisioner interface {
	ProvisionOrganization(context.Context, organizationdomain.Organization) error
}

// PublicDirectory is the read port consumed by public organization lookup.
// It deliberately returns the Organization domain type, never a repository
// row, so consumers cannot bypass lifecycle rules.
type PublicDirectory interface {
	ResolveBySlug(context.Context, string) (organizationdomain.Organization, error)
}

type PublicQueries struct {
	Directory PublicDirectory
}

func (queries PublicQueries) ResolvePublicBySlug(
	ctx context.Context,
	slug string,
) (organizationdomain.Organization, error) {
	slug = strings.TrimSpace(slug)
	if ctx == nil || queries.Directory == nil || slug == "" {
		return organizationdomain.Organization{}, organizationdomain.ErrUnknown
	}
	organization, err := queries.Directory.ResolveBySlug(ctx, slug)
	if err != nil || organization.Status != organizationdomain.Ready {
		return organizationdomain.Organization{}, organizationdomain.ErrUnknown
	}
	return organization, nil
}

type ProvisionOrganizationCommand struct {
	ID                  string
	Name                string
	Slug                string
	ClerkOrganizationID string
}

type ProvisionOrganization struct {
	Directory  Directory
	Fiscal     Provisioner
	Accounting Provisioner
}

func (workflow ProvisionOrganization) Execute(
	ctx context.Context,
	command ProvisionOrganizationCommand,
) error {
	if ctx == nil {
		return fmt.Errorf("provision organization: context is required")
	}
	command.ID = strings.TrimSpace(command.ID)
	command.Name = strings.TrimSpace(command.Name)
	command.Slug = strings.TrimSpace(command.Slug)
	command.ClerkOrganizationID = strings.TrimSpace(command.ClerkOrganizationID)
	if command.ID == "" ||
		command.Name == "" ||
		command.Slug == "" ||
		command.ClerkOrganizationID == "" {
		return fmt.Errorf(
			"provision organization: ID, name, slug and Clerk organization ID are required",
		)
	}
	if workflow.Directory == nil ||
		workflow.Fiscal == nil ||
		workflow.Accounting == nil {
		return fmt.Errorf("provision organization: dependencies are not configured")
	}

	organization := organizationdomain.Organization{
		ID: command.ID, Name: command.Name, Slug: command.Slug,
		Status: organizationdomain.Pending,
	}
	if err := workflow.Directory.SyncClerk(
		ctx,
		command.ClerkOrganizationID,
		organization,
	); err != nil {
		return fmt.Errorf("project Clerk organization: %w", err)
	}
	if err := workflow.Fiscal.ProvisionOrganization(ctx, organization); err != nil {
		return workflow.fail(
			ctx,
			organization.ID,
			fiscalService,
			"FISCAL_PROVISIONING_FAILED",
			err,
		)
	}
	if err := workflow.Directory.SetProvisioningStatus(
		ctx,
		organization.ID,
		fiscalService,
		"ready",
		"",
	); err != nil {
		return fmt.Errorf("mark fiscal provisioning ready: %w", err)
	}
	if err := workflow.Accounting.ProvisionOrganization(ctx, organization); err != nil {
		return workflow.fail(
			ctx,
			organization.ID,
			accountingService,
			"ACCOUNTING_PROVISIONING_FAILED",
			err,
		)
	}
	if err := workflow.Directory.SetProvisioningStatus(
		ctx,
		organization.ID,
		accountingService,
		"ready",
		"",
	); err != nil {
		return fmt.Errorf("mark accounting provisioning ready: %w", err)
	}
	if err := workflow.Directory.SetStatus(
		ctx,
		organization.ID,
		organizationdomain.Ready,
	); err != nil {
		return fmt.Errorf("mark organization ready: %w", err)
	}
	return nil
}

func (workflow ProvisionOrganization) fail(
	ctx context.Context,
	organizationID string,
	service string,
	code string,
	cause error,
) error {
	provisioningErr := workflow.Directory.SetProvisioningStatus(
		ctx,
		organizationID,
		service,
		"failed",
		code,
	)
	statusErr := workflow.Directory.SetStatus(
		ctx,
		organizationID,
		organizationdomain.Failed,
	)
	return errors.Join(
		fmt.Errorf("%s provisioning: %w", service, cause),
		wrapFailure("mark service provisioning failed", provisioningErr),
		wrapFailure("mark organization failed", statusErr),
	)
}

func wrapFailure(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
