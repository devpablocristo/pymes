// Package notifications contains the private PerGo HTTP adapter.
// architecture:adapter external
package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	pergohelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/helpers"
	pergomodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// PerGoPlatformTokenSource is owned by the PerGo adapter, which consumes a
// workload identity to call a private Cloud Run service without replacing the
// application-level Authorization header.
type PerGoPlatformTokenSource interface {
	PlatformToken(context.Context, string) (string, error)
}

type PerGo struct {
	BaseURL                  string
	APIKey                   string
	Audience                 string
	Channel                  string
	AllowGlobalRouteFallback bool
	PlatformTokens           PerGoPlatformTokenSource
	Client                   HTTPDoer
	Timeout                  time.Duration
}

func NewPerGo(
	baseURL string,
	apiKey string,
	audience string,
	channel string,
	allowGlobalRouteFallback bool,
	platformTokens PerGoPlatformTokenSource,
	client HTTPDoer,
	timeout time.Duration,
) *PerGo {
	return &PerGo{
		BaseURL: baseURL, APIKey: apiKey, Audience: audience, Channel: channel,
		AllowGlobalRouteFallback: allowGlobalRouteFallback,
		PlatformTokens:           platformTokens,
		Client:                   client,
		Timeout:                  timeout,
	}
}

func (adapter PerGo) Send(
	ctx context.Context,
	intent domain.Intent,
) (DeliveryReceipt, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(adapter.BaseURL), "/")
	if baseURL == "" || strings.TrimSpace(adapter.APIKey) == "" {
		return DeliveryReceipt{}, &ProviderError{
			StableCode: "PERGO_NOT_CONFIGURED",
			Cause:      errors.New("PerGo URL and API key are required"),
		}
	}
	channel, senderIdentity, routeErr := pergohelpers.DeliveryRoute(
		intent,
		adapter.Channel,
		adapter.AllowGlobalRouteFallback,
	)
	if routeErr != nil {
		return DeliveryReceipt{}, &ProviderError{
			StableCode: "PERGO_ROUTE_NOT_CONFIGURED",
			Cause:      routeErr,
		}
	}
	payload := pergohelpers.MessageRequest(intent, channel, senderIdentity)
	traceID, err := pergohelpers.TraceID(intent.OrganizationID, intent.ID)
	if err != nil {
		return DeliveryReceipt{}, &ProviderError{
			StableCode: "PERGO_IDENTITY_INVALID", Cause: err,
		}
	}
	ingressIdempotencyKey, err := pergohelpers.IngressIdempotencyKey(
		intent.OrganizationID,
		intent.IdempotencyKey,
	)
	if err != nil {
		return DeliveryReceipt{}, &ProviderError{
			StableCode: "PERGO_IDENTITY_INVALID", Cause: err,
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return DeliveryReceipt{}, &ProviderError{
			StableCode: "PERGO_PAYLOAD_INVALID", Cause: err,
		}
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		baseURL+"/api/v1/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		return DeliveryReceipt{}, &ProviderError{
			StableCode: "PERGO_REQUEST_INVALID", Cause: err,
		}
	}
	audience := strings.TrimSpace(adapter.Audience)
	if audience != "" {
		if adapter.PlatformTokens == nil {
			return DeliveryReceipt{}, &ProviderError{
				StableCode: "PERGO_NOT_CONFIGURED",
				Cause:      errors.New("PerGo platform identity is required"),
			}
		}
		token, tokenErr := adapter.PlatformTokens.PlatformToken(ctx, audience)
		if tokenErr != nil {
			return DeliveryReceipt{}, &ProviderError{
				StableCode: "PERGO_PLATFORM_IDENTITY_UNAVAILABLE",
				Retry:      true,
				Cause:      tokenErr,
			}
		}
		authorization, authorizationErr :=
			pergohelpers.ServerlessAuthorization(token)
		if authorizationErr != nil {
			return DeliveryReceipt{}, &ProviderError{
				StableCode: "PERGO_PLATFORM_IDENTITY_UNAVAILABLE",
				Retry:      true,
				Cause:      authorizationErr,
			}
		}
		request.Header.Set("X-Serverless-Authorization", authorization)
	}
	request.Header.Set("Authorization", "Bearer "+adapter.APIKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Trace-ID", traceID)
	request.Header.Set("Idempotency-Key", ingressIdempotencyKey)
	client := adapter.Client
	if client == nil {
		timeout := adapter.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	response, err := client.Do(request)
	if err != nil {
		code := "PERGO_UNAVAILABLE"
		unknown := false
		var urlError *url.Error
		if errors.As(err, &urlError) || errors.Is(err, context.DeadlineExceeded) {
			code = "PERGO_RESPONSE_UNCERTAIN"
			unknown = true
		}
		return DeliveryReceipt{}, &ProviderError{
			StableCode: code, Retry: true, Unknown: unknown, Cause: err,
		}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusAccepted {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		retry := response.StatusCode == http.StatusRequestTimeout ||
			response.StatusCode == http.StatusTooEarly ||
			response.StatusCode == http.StatusTooManyRequests ||
			response.StatusCode >= http.StatusInternalServerError
		code := "PERGO_REQUEST_REJECTED"
		if retry {
			code = "PERGO_UNAVAILABLE"
		}
		if response.StatusCode == http.StatusConflict {
			code = "PERGO_IDEMPOTENCY_CONFLICT"
		}
		return DeliveryReceipt{}, &ProviderError{
			StableCode: code, Retry: retry,
			Cause: fmt.Errorf("PerGo HTTP status %d", response.StatusCode),
		}
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var result pergomodels.MessageResponse
	if err = decoder.Decode(&result); err != nil ||
		strings.TrimSpace(result.MessageID) == "" ||
		result.Status != "queued" ||
		result.QueuedAt.IsZero() {
		return DeliveryReceipt{}, &ProviderError{
			StableCode: "PERGO_RESPONSE_INVALID", Retry: true, Unknown: true,
			Cause: err,
		}
	}
	return DeliveryReceipt{
		ExternalMessageID: result.MessageID,
		Status:            result.Status,
		QueuedAt:          result.QueuedAt.UTC(),
	}, nil
}
