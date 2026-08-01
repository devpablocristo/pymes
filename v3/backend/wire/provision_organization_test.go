package wire

import (
	"context"
	"testing"

	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
)

func TestProvisionOrganizationIdentityAlwaysCreatesSignedInternalTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		allowInsecure     bool
		wantPlatformCalls int
	}{
		{name: "secure workload", wantPlatformCalls: 1},
		{name: "local platform bypass", allowInsecure: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tokenCalls := 0
			platformCalls := 0
			resource := &provisionTokenStub{}
			tokens, platform, closer, err := provisionOrganizationIdentity(
				context.Background(),
				config.ProvisionOrganizationConfig{
					Environment:                "test",
					AllowInsecureLocalServices: test.allowInsecure,
				},
				func(_ context.Context, subject string) (provisionTokenResource, error) {
					tokenCalls++
					if subject != "provision-org" {
						t.Fatalf("subject=%q", subject)
					}
					return resource, nil
				},
				func() provisionPlatformTokenSource {
					platformCalls++
					return platformTokenStub{}
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if tokenCalls != 1 || tokens != resource || closer != resource {
				t.Fatalf(
					"token_calls=%d tokens=%T closer=%T",
					tokenCalls,
					tokens,
					closer,
				)
			}
			if platformCalls != test.wantPlatformCalls {
				t.Fatalf(
					"platform_calls=%d want=%d",
					platformCalls,
					test.wantPlatformCalls,
				)
			}
			if test.allowInsecure && platform != nil {
				t.Fatalf("local bypass must omit only the platform token: %T", platform)
			}
			if !test.allowInsecure && platform == nil {
				t.Fatal("secure workload requires a platform token")
			}
		})
	}
}

func TestProvisionOrganizationIdentityRejectsProductionBypass(t *testing.T) {
	t.Parallel()
	tokenCalls := 0
	_, _, _, err := provisionOrganizationIdentity(
		context.Background(),
		config.ProvisionOrganizationConfig{
			Environment:                "production",
			AllowInsecureLocalServices: true,
		},
		func(context.Context, string) (provisionTokenResource, error) {
			tokenCalls++
			return &provisionTokenStub{}, nil
		},
		func() provisionPlatformTokenSource {
			return platformTokenStub{}
		},
	)
	if err == nil ||
		ProvisionOrganizationStartupErrorCode(err) != "WORKLOAD_IDENTITY_INVALID" {
		t.Fatalf(
			"err=%v code=%q",
			err,
			ProvisionOrganizationStartupErrorCode(err),
		)
	}
	if tokenCalls != 0 {
		t.Fatalf("invalid production config crossed identity boundary")
	}
}

func TestProvisionOrganizationIdentityClosesTokenOnCompositionFailure(t *testing.T) {
	t.Parallel()
	resource := &provisionTokenStub{}
	_, _, _, err := provisionOrganizationIdentity(
		context.Background(),
		config.ProvisionOrganizationConfig{Environment: "test"},
		func(context.Context, string) (provisionTokenResource, error) {
			return resource, nil
		},
		func() provisionPlatformTokenSource { return nil },
	)
	if err == nil {
		t.Fatal("expected missing platform identity to fail")
	}
	if resource.closeCalls != 1 {
		t.Fatalf("close_calls=%d", resource.closeCalls)
	}
}

func TestProvisionOrganizationAppClosesIdentityExactlyOnce(t *testing.T) {
	t.Parallel()
	resource := &provisionTokenStub{}
	app := &ProvisionOrganizationApp{identity: resource}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if resource.closeCalls != 1 {
		t.Fatalf("close_calls=%d", resource.closeCalls)
	}
}

type provisionTokenStub struct {
	closeCalls int
}

func (*provisionTokenStub) Token(
	context.Context,
	string,
	string,
) (string, error) {
	return "signed.jwt.token", nil
}

func (stub *provisionTokenStub) Close() error {
	stub.closeCalls++
	return nil
}

type platformTokenStub struct{}

func (platformTokenStub) PlatformToken(
	context.Context,
	string,
) (string, error) {
	return "platform-token", nil
}

var _ provisionTokenResource = (*provisionTokenStub)(nil)
var _ provisionPlatformTokenSource = platformTokenStub{}
