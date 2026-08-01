// Command internal-jwks resolves one or more explicit Cloud KMS Ed25519 key
// versions and prints a verification-only JWKS. It never requests private key
// material because asymmetric Cloud KMS keys do not expose it.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	kms "cloud.google.com/go/kms/apiv1"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity/access"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	versions := []string{strings.TrimSpace(os.Getenv("PYMES_INTERNAL_KMS_KEY_VERSION"))}
	versions = append(versions, csv(os.Getenv("PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS"))...)

	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		log.Fatalf("create KMS client: %v", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.Printf("close KMS client: %v", closeErr)
		}
	}()
	keys, err := identityaccess.LoadKMSVerificationKeys(ctx, client, versions)
	if err != nil {
		log.Fatal(err)
	}
	encoded, err := identityaccess.JWKSJSON(keys)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(encoded)
}

func csv(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
