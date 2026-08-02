package helpers

import (
	"fmt"
	"strings"
)

// ServerlessAuthorization normalizes a workload ID token for Cloud Run's
// secondary authorization header. The application API key remains in the
// standard Authorization header for PerGo itself.
func ServerlessAuthorization(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("platform identity returned an empty token")
	}
	if strings.ContainsAny(token, "\r\n") {
		return "", fmt.Errorf("platform identity returned an invalid token")
	}
	return "Bearer " + token, nil
}
