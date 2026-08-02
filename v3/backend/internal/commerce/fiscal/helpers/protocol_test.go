package helpers

import (
	"errors"
	"testing"

	accountinghelpers "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/helpers"
)

func TestDecodeResultPreservesStableProviderError(t *testing.T) {
	t.Parallel()
	_, err := DecodeResult(
		"fiscal",
		"422 Unprocessable Entity",
		422,
		[]byte(`{"code":"CERTIFICATE_INVALID","title":"invalid certificate"}`),
	)
	var provider accountinghelpers.ServiceError
	if !errors.As(err, &provider) || provider.Code != "CERTIFICATE_INVALID" {
		t.Fatalf("provider error = %#v, err = %v", provider, err)
	}
}
