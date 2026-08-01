package usecases

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/domain"
)

func TestProvisionOrganizationCoordinatesBoundariesInOrder(t *testing.T) {
	t.Parallel()
	var calls []string
	directory := &directorySpy{calls: &calls}
	workflow := ProvisionOrganization{
		Directory: directory,
		Fiscal: provisionerFunc(func(
			_ context.Context,
			organization organizationdomain.Organization,
		) error {
			calls = append(calls, "fiscal:"+organization.ID)
			return nil
		}),
		Accounting: provisionerFunc(func(
			_ context.Context,
			organization organizationdomain.Organization,
		) error {
			calls = append(calls, "accounting:"+organization.ID)
			return nil
		}),
	}
	err := workflow.Execute(context.Background(), ProvisionOrganizationCommand{
		ID: " org_a ", Name: " ACME ", Slug: " acme ",
		ClerkOrganizationID: " clerk_org_a ",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"sync:clerk_org_a:org_a:pending",
		"fiscal:org_a",
		"provisioning:fiscal:ready:",
		"accounting:org_a",
		"provisioning:accounting:ready:",
		"status:ready",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}

func TestProvisionOrganizationRecordsDownstreamFailure(t *testing.T) {
	t.Parallel()
	var calls []string
	downstreamErr := errors.New("accounting unavailable")
	workflow := ProvisionOrganization{
		Directory: &directorySpy{calls: &calls},
		Fiscal: provisionerFunc(func(
			context.Context,
			organizationdomain.Organization,
		) error {
			return nil
		}),
		Accounting: provisionerFunc(func(
			context.Context,
			organizationdomain.Organization,
		) error {
			return downstreamErr
		}),
	}
	err := workflow.Execute(context.Background(), validProvisionCommand())
	if !errors.Is(err, downstreamErr) {
		t.Fatalf("err=%v", err)
	}
	wantTail := []string{
		"provisioning:accounting:failed:ACCOUNTING_PROVISIONING_FAILED",
		"status:failed",
	}
	if !reflect.DeepEqual(calls[len(calls)-2:], wantTail) {
		t.Fatalf("calls=%v", calls)
	}
}

func TestProvisionOrganizationFailsBeforeCrossingPortsForInvalidInput(t *testing.T) {
	t.Parallel()
	var calls []string
	workflow := ProvisionOrganization{
		Directory: &directorySpy{calls: &calls},
		Fiscal: provisionerFunc(func(
			context.Context,
			organizationdomain.Organization,
		) error {
			return nil
		}),
		Accounting: provisionerFunc(func(
			context.Context,
			organizationdomain.Organization,
		) error {
			return nil
		}),
	}
	command := validProvisionCommand()
	command.ClerkOrganizationID = " "
	err := workflow.Execute(context.Background(), command)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("err=%v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("ports crossed for invalid input: %v", calls)
	}
}

func validProvisionCommand() ProvisionOrganizationCommand {
	return ProvisionOrganizationCommand{
		ID: "org_a", Name: "ACME", Slug: "acme",
		ClerkOrganizationID: "clerk_org_a",
	}
}

type provisionerFunc func(context.Context, organizationdomain.Organization) error

func (function provisionerFunc) ProvisionOrganization(
	ctx context.Context,
	organization organizationdomain.Organization,
) error {
	return function(ctx, organization)
}

type directorySpy struct {
	calls *[]string
}

func (spy *directorySpy) SyncClerk(
	_ context.Context,
	clerkOrganizationID string,
	organization organizationdomain.Organization,
) error {
	*spy.calls = append(
		*spy.calls,
		"sync:"+clerkOrganizationID+":"+organization.ID+":"+string(organization.Status),
	)
	return nil
}

func (spy *directorySpy) SetProvisioningStatus(
	_ context.Context,
	_ string,
	service string,
	status string,
	code string,
) error {
	*spy.calls = append(
		*spy.calls,
		"provisioning:"+service+":"+status+":"+code,
	)
	return nil
}

func (spy *directorySpy) SetStatus(
	_ context.Context,
	_ string,
	status organizationdomain.Status,
) error {
	*spy.calls = append(*spy.calls, "status:"+string(status))
	return nil
}
