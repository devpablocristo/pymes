package helpers

import "testing"

func TestServerlessAuthorizationRejectsEmptyAndHeaderInjection(t *testing.T) {
	t.Parallel()
	for _, token := range []string{"", "   ", "token\r\nInjected: value"} {
		if _, err := ServerlessAuthorization(token); err == nil {
			t.Fatalf("token %q was accepted", token)
		}
	}
	if got, err := ServerlessAuthorization("  workload-token  "); err != nil ||
		got != "Bearer workload-token" {
		t.Fatalf("authorization=%q err=%v", got, err)
	}
}
