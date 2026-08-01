// architecture:adapter external
package identity

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	cloudrunhelpers "github.com/devpablocristo/pymes/v3/backend/internal/identity/cloud_run_tokens/helpers"
	cloudrunmodels "github.com/devpablocristo/pymes/v3/backend/internal/identity/cloud_run_tokens/models"
)

const defaultMetadataHost = "metadata.google.internal"

type MetadataIDTokenSource struct {
	client   *http.Client
	endpoint string
	now      func() time.Time

	mu    sync.Mutex
	cache map[string]cloudrunmodels.CachedToken
}

func NewMetadataIDTokenSource() *MetadataIDTokenSource {
	host := strings.TrimSpace(os.Getenv("GCE_METADATA_HOST"))
	if host == "" {
		host = defaultMetadataHost
	}
	return newMetadataIDTokenSource(&http.Client{Timeout: 3 * time.Second}, "http://"+host, time.Now)
}

func newMetadataIDTokenSource(client *http.Client, endpoint string, now func() time.Time) *MetadataIDTokenSource {
	return &MetadataIDTokenSource{client: client, endpoint: strings.TrimRight(endpoint, "/"), now: now, cache: map[string]cloudrunmodels.CachedToken{}}
}

func (s *MetadataIDTokenSource) PlatformToken(ctx context.Context, audience string) (string, error) {
	if s == nil || s.client == nil || s.now == nil || strings.TrimSpace(audience) == "" {
		return "", fmt.Errorf("platform identity is not configured")
	}
	now := s.now()
	s.mu.Lock()
	cached, ok := s.cache[audience]
	s.mu.Unlock()
	if ok && now.Add(time.Minute).Before(cached.ExpiresAt) {
		return cached.Value, nil
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
	expiresAt, err := cloudrunhelpers.TokenExpiry(token)
	if err != nil {
		return "", fmt.Errorf("validate platform identity: %w", err)
	}
	s.mu.Lock()
	s.cache[audience] = cloudrunmodels.CachedToken{Value: token, ExpiresAt: expiresAt}
	s.mu.Unlock()
	return token, nil
}

func tokenExpiry(token string) (time.Time, error) {
	return cloudrunhelpers.TokenExpiry(token)
}
