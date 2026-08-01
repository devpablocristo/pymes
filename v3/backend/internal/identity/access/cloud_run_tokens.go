package access

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultMetadataHost = "metadata.google.internal"

// PlatformTokenSource produces the Google-signed audience token consumed by
// Cloud Run IAM. It is deliberately separate from TokenSource: the latter is
// the Pymes internal credential that remains in Authorization, while this
// token is sent through X-Serverless-Authorization.
type PlatformTokenSource interface {
	PlatformToken(context.Context, string) (string, error)
}

type cachedPlatformToken struct {
	value     string
	expiresAt time.Time
}

type MetadataIDTokenSource struct {
	client   *http.Client
	endpoint string
	now      func() time.Time

	mu    sync.Mutex
	cache map[string]cachedPlatformToken
}

func NewMetadataIDTokenSource() *MetadataIDTokenSource {
	host := strings.TrimSpace(os.Getenv("GCE_METADATA_HOST"))
	if host == "" {
		host = defaultMetadataHost
	}
	return newMetadataIDTokenSource(&http.Client{Timeout: 3 * time.Second}, "http://"+host, time.Now)
}

func newMetadataIDTokenSource(client *http.Client, endpoint string, now func() time.Time) *MetadataIDTokenSource {
	return &MetadataIDTokenSource{client: client, endpoint: strings.TrimRight(endpoint, "/"), now: now, cache: map[string]cachedPlatformToken{}}
}

func (s *MetadataIDTokenSource) PlatformToken(ctx context.Context, audience string) (string, error) {
	if s == nil || s.client == nil || s.now == nil || strings.TrimSpace(audience) == "" {
		return "", fmt.Errorf("platform identity is not configured")
	}
	now := s.now()
	s.mu.Lock()
	cached, ok := s.cache[audience]
	s.mu.Unlock()
	if ok && now.Add(time.Minute).Before(cached.expiresAt) {
		return cached.value, nil
	}

	endpoint := s.endpoint + "/computeMetadata/v1/instance/service-accounts/default/identity?audience=" + url.QueryEscape(audience) + "&format=full"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create platform identity request: %w", err)
	}
	request.Header.Set("Metadata-Flavor", "Google")
	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch platform identity: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch platform identity: metadata returned %s", response.Status)
	}
	encoded, err := io.ReadAll(io.LimitReader(response.Body, 16<<10))
	if err != nil {
		return "", fmt.Errorf("read platform identity: %w", err)
	}
	token := strings.TrimSpace(string(encoded))
	expiresAt, err := tokenExpiry(token)
	if err != nil {
		return "", fmt.Errorf("validate platform identity: %w", err)
	}
	s.mu.Lock()
	s.cache[audience] = cachedPlatformToken{value: token, expiresAt: expiresAt}
	s.mu.Unlock()
	return token, nil
}

func tokenExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid token shape")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("decode token claims")
	}
	var claims struct {
		ExpiresAt int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.ExpiresAt <= 0 {
		return time.Time{}, fmt.Errorf("invalid token expiry")
	}
	return time.Unix(claims.ExpiresAt, 0), nil
}
