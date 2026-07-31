package main

import (
	"context"
	"flag"
	"log"
	"os"

	commercecompanion "github.com/devpablocristo/pymes/v3/backend/internal/commerce/companion"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity/access"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/domain"
	organizationrepository "github.com/devpablocristo/pymes/v3/backend/internal/organization/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	id := flag.String("id", "", "organization ID")
	name := flag.String("name", "", "organization name")
	slug := flag.String("slug", "", "organization slug")
	clerkOrganizationID := flag.String("clerk-organization-id", "", "verified Clerk organization ID")
	flag.Parse()
	if *id == "" || *name == "" || *slug == "" || *clerkOrganizationID == "" {
		log.Fatal("--id, --name, --slug and --clerk-organization-id are required")
	}
	databaseURL, accountingURL := os.Getenv("PYMES_DATABASE_URL"), os.Getenv("ACCOUNTING_URL")
	if databaseURL == "" || accountingURL == "" {
		log.Fatal("PYMES_DATABASE_URL and ACCOUNTING_URL are required")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	organizations := organizationrepository.New(pool)
	var tokens identityaccess.TokenSource
	if os.Getenv("PYMES_ALLOW_INSECURE_LOCAL_SERVICES") != "true" {
		tokens, err = identityaccess.TokenSourceFromRuntime("provision-org")
		if err != nil {
			log.Fatal(err)
		}
	}
	organization := organizationdomain.Organization{ID: *id, Name: *name, Slug: *slug, Status: organizationdomain.Pending}
	if err := organizations.SyncClerk(context.Background(), *clerkOrganizationID, organization); err != nil {
		log.Fatal(err)
	}
	// The mock Fiscal adapter has no per-organization resources. This explicit
	// transition will be replaced by fiscal credential provisioning when the
	// deferred ARCA implementation begins.
	if err := organizations.SetProvisioningStatus(context.Background(), *id, "fiscal", "ready", ""); err != nil {
		log.Fatal(err)
	}
	if err := (commercecompanion.HTTPAccountingClient{BaseURL: accountingURL, Tokens: tokens}).ProvisionOrganization(context.Background(), organization); err != nil {
		_ = organizations.SetProvisioningStatus(context.Background(), *id, "accounting", "failed", "ACCOUNTING_PROVISIONING_FAILED")
		_ = organizations.SetStatus(context.Background(), *id, organizationdomain.Failed)
		log.Fatal(err)
	}
	if err := organizations.SetProvisioningStatus(context.Background(), *id, "accounting", "ready", ""); err != nil {
		log.Fatal(err)
	}
	if err := organizations.SetStatus(context.Background(), *id, organizationdomain.Ready); err != nil {
		log.Fatal(err)
	}
	log.Printf("organization %s ready", *id)
}
