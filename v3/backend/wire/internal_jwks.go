package wire

import (
	"context"

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/devpablocristo/pymes/v3/backend/internal/identity"
)

type InternalJWKS struct {
	JSON   string
	client *kms.KeyManagementClient
}

func InitializeInternalJWKS(ctx context.Context, versions []string) (*InternalJWKS, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := identity.LoadKMSVerificationKeys(ctx, client, versions)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	encoded, err := identity.JWKSJSON(keys)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return &InternalJWKS{JSON: encoded, client: client}, nil
}

func (value *InternalJWKS) Close() error {
	if value == nil || value.client == nil {
		return nil
	}
	return value.client.Close()
}
