package helpers

import (
	"errors"
	"testing"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func TestDecodeServiceErrorPreservesStableAccountingCode(t *testing.T) {
	err := DecodeServiceError(
		"accounting post",
		"409 Conflict",
		[]byte(`{"code":"PERIOD_LOCKED","title":"Period locked"}`),
	)
	var serviceError ServiceError
	if !errors.As(err, &serviceError) {
		t.Fatalf("expected ServiceError, got %T", err)
	}
	if serviceError.Code != "PERIOD_LOCKED" ||
		serviceError.Title != "Period locked" ||
		!errors.Is(err, domain.ErrPeriodLocked) {
		t.Fatalf("unexpected normalized error: %+v", serviceError)
	}
}

func TestProtocolFallbackAndVersionFailClosed(t *testing.T) {
	if Fallback("", "default") != "default" || Fallback("value", "default") != "value" {
		t.Fatal("fallback changed")
	}
	if PositiveVersion(0) != 1 || PositiveVersion(3) != 3 {
		t.Fatal("source version normalization changed")
	}
}
