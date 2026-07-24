package administration

import (
	"context"
	"errors"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
)

// ClerkAdapter is the outbound identity-provider adapter. The application
// service depends only on the narrow Provider port.
type ClerkAdapter struct {
	client *clerk.Client
}

func NewClerkAdapter(client *clerk.Client) (*ClerkAdapter, error) {
	if client == nil {
		return nil, errors.New("administration: Clerk client is required")
	}
	return &ClerkAdapter{client: client}, nil
}

func (adapter *ClerkAdapter) DeleteOrganization(ctx context.Context, externalID string) error {
	return adapter.client.DeleteOrganization(ctx, externalID)
}

func (adapter *ClerkAdapter) DeleteUser(ctx context.Context, externalID string) error {
	return adapter.client.DeleteUser(ctx, externalID)
}

func (adapter *ClerkAdapter) UpdateUserEmail(
	ctx context.Context,
	externalID, email string,
) error {
	_, err := adapter.client.UpdateUserEmail(ctx, externalID, email)
	return err
}
