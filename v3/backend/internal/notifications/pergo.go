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

type PerGo struct {
	BaseURL string
	APIKey  string
	Channel string
	Client  HTTPDoer
	Timeout time.Duration
}

func NewPerGo(
	baseURL string,
	apiKey string,
	channel string,
	client HTTPDoer,
	timeout time.Duration,
) *PerGo {
	return &PerGo{
		BaseURL: baseURL, APIKey: apiKey, Channel: channel,
		Client: client, Timeout: timeout,
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
	channel := strings.TrimSpace(adapter.Channel)
	if channel == "" {
		channel = "whatsapp"
	}
	if channel != "whatsapp" && channel != "whatsapp_cloud" && channel != "whatsapp_mock" {
		return DeliveryReceipt{}, &ProviderError{
			StableCode: "PERGO_CHANNEL_INVALID",
			Cause:      errors.New("unsupported PerGo channel"),
		}
	}
	payload := pergohelpers.MessageRequest(intent, channel)
	traceID, err := pergohelpers.TraceID(intent.OrganizationID, intent.ID)
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
	request.Header.Set("Authorization", "Bearer "+adapter.APIKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Trace-ID", traceID)
	request.Header.Set("Idempotency-Key", intent.IdempotencyKey)
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
