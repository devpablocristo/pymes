package outwebhooks

import (
	"errors"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	webhookmodels "github.com/devpablocristo/pymes/pymes-core/backend/internal/outwebhooks/repository/models"
	webhookdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/outwebhooks/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/apperror"
)

type RepositoryPort interface {
	ListEndpoints(ctx context.Context, orgID uuid.UUID) ([]webhookdomain.Endpoint, error)
	CreateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error)
	GetEndpoint(ctx context.Context, orgID, id uuid.UUID) (webhookdomain.Endpoint, error)
	UpdateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error)
	DeleteEndpoint(ctx context.Context, orgID, id uuid.UUID) error
	ListDeliveries(ctx context.Context, orgID, endpointID uuid.UUID, limit int) ([]webhookdomain.Delivery, error)
	CreateOutbox(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error
	ListPendingOutbox(ctx context.Context, limit int) ([]webhookmodels.OutboxModel, error)
	MarkOutbox(ctx context.Context, id uuid.UUID, status, lastError string) error
	ListEndpointsForEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]webhookdomain.Endpoint, error)
	CreateDelivery(ctx context.Context, endpointID uuid.UUID, eventType string, payload map[string]any, statusCode *int, responseBody string, attempts int, nextRetry, deliveredAt *time.Time) (webhookdomain.Delivery, error)
	ListRetryableDeliveries(ctx context.Context, limit int) ([]webhookmodels.DeliveryModel, error)
	GetDelivery(ctx context.Context, id uuid.UUID) (webhookdomain.Delivery, error)
	UpdateDeliveryResult(ctx context.Context, id uuid.UUID, statusCode *int, responseBody string, attempts int, nextRetry, deliveredAt *time.Time) error
	GetEndpointByID(ctx context.Context, id uuid.UUID) (webhookdomain.Endpoint, error)
	DeleteOldDeliveries(ctx context.Context, olderThan time.Time) (int64, error)
}

type ipResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type Usecases struct {
	repo     RepositoryPort
	client   *http.Client
	resolver ipResolver
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{
		repo:     repo,
		client:   &http.Client{Timeout: 5 * time.Second},
		resolver: net.DefaultResolver,
	}
}

func (u *Usecases) ListEndpoints(ctx context.Context, orgID uuid.UUID) ([]webhookdomain.Endpoint, error) {
	return u.repo.ListEndpoints(ctx, orgID)
}

func (u *Usecases) CreateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error) {
	if err := u.validateURL(ctx, in.URL); err != nil {
		return webhookdomain.Endpoint{}, err
	}
	if in.OrgID == uuid.Nil {
		return webhookdomain.Endpoint{}, apperror.NewBadInput("org_id is required")
	}
	if len(in.Events) == 0 {
		in.Events = []string{"*"}
	}
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now().UTC()
	}
	in.UpdatedAt = in.CreatedAt
	if strings.TrimSpace(in.Secret) == "" {
		in.Secret = randomSecret()
	}
	return u.repo.CreateEndpoint(ctx, normalizeEndpoint(in))
}

func (u *Usecases) GetEndpoint(ctx context.Context, orgID, id uuid.UUID) (webhookdomain.Endpoint, error) {
	out, err := u.repo.GetEndpoint(ctx, orgID, id)
	if err != nil && gorm.ErrRecordNotFound == err {
		return webhookdomain.Endpoint{}, apperror.NewNotFound("webhook_endpoint", id.String())
	}
	return out, err
}

func (u *Usecases) UpdateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error) {
	if err := u.validateURL(ctx, in.URL); err != nil {
		return webhookdomain.Endpoint{}, err
	}
	if in.OrgID == uuid.Nil || in.ID == uuid.Nil {
		return webhookdomain.Endpoint{}, apperror.NewBadInput("org_id and id are required")
	}
	current, err := u.repo.GetEndpoint(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return webhookdomain.Endpoint{}, apperror.NewNotFound("webhook_endpoint", in.ID.String())
		}
		return webhookdomain.Endpoint{}, err
	}
	if strings.TrimSpace(in.Secret) == "" {
		in.Secret = current.Secret
	}
	in.CreatedAt = current.CreatedAt
	in.CreatedBy = current.CreatedBy
	in.UpdatedAt = time.Now().UTC()
	if len(in.Events) == 0 {
		in.Events = current.Events
	}
	return u.repo.UpdateEndpoint(ctx, normalizeEndpoint(in))
}

func (u *Usecases) DeleteEndpoint(ctx context.Context, orgID, id uuid.UUID) error {
	if err := u.repo.DeleteEndpoint(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NewNotFound("webhook_endpoint", id.String())
		}
		return err
	}
	return nil
}

func (u *Usecases) ListDeliveries(ctx context.Context, orgID, endpointID uuid.UUID, limit int) ([]webhookdomain.Delivery, error) {
	return u.repo.ListDeliveries(ctx, orgID, endpointID, limit)
}

func (u *Usecases) Enqueue(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error {
	if orgID == uuid.Nil || strings.TrimSpace(eventType) == "" {
		return apperror.NewBadInput("org_id and event_type are required")
	}
	return u.repo.CreateOutbox(ctx, orgID, strings.TrimSpace(eventType), payload)
}

func (u *Usecases) SendTest(ctx context.Context, orgID, endpointID uuid.UUID, actor string) error {
	endpoint, err := u.repo.GetEndpoint(ctx, orgID, endpointID)
	if err != nil {
		return err
	}
	payload := map[string]any{"event": "webhook.test", "actor": actor, "timestamp": time.Now().UTC().Format(time.RFC3339)}
	statusCode, responseBody, deliveredAt, nextRetry := u.deliver(ctx, endpoint, "webhook.test", payload, 1)
	_, err = u.repo.CreateDelivery(ctx, endpoint.ID, "webhook.test", payload, statusCode, responseBody, 1, nextRetry, deliveredAt)
	return err
}

func (u *Usecases) ReplayDelivery(ctx context.Context, deliveryID uuid.UUID) error {
	delivery, err := u.repo.GetDelivery(ctx, deliveryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NewNotFound("webhook_delivery", deliveryID.String())
		}
		return err
	}
	endpoint, err := u.repo.GetEndpointByID(ctx, delivery.EndpointID)
	if err != nil {
		return err
	}
	attempts := delivery.Attempts + 1
	statusCode, responseBody, deliveredAt, nextRetry := u.deliver(ctx, endpoint, delivery.EventType, delivery.Payload, attempts)
	return u.repo.UpdateDeliveryResult(ctx, delivery.ID, statusCode, responseBody, attempts, nextRetry, deliveredAt)
}

func (u *Usecases) RetryPending(ctx context.Context) (int, error) {
	processed := 0
	outboxItems, err := u.repo.ListPendingOutbox(ctx, 100)
	if err != nil {
		return 0, err
	}
	for _, item := range outboxItems {
		payload := map[string]any{}
		_ = json.Unmarshal(item.Payload, &payload)
		endpoints, err := u.repo.ListEndpointsForEvent(ctx, item.OrgID, item.EventType)
		if err != nil {
			return processed, err
		}
		for _, endpoint := range endpoints {
			statusCode, responseBody, deliveredAt, nextRetry := u.deliver(ctx, endpoint, item.EventType, payload, 1)
			if _, err := u.repo.CreateDelivery(ctx, endpoint.ID, item.EventType, payload, statusCode, responseBody, 1, nextRetry, deliveredAt); err != nil {
				return processed, err
			}
		}
		status := "sent"
		lastError := ""
		if len(endpoints) == 0 {
			status = "failed"
			lastError = "no active endpoints for event"
		}
		if err := u.repo.MarkOutbox(ctx, item.ID, status, lastError); err != nil {
			return processed, err
		}
		processed++
	}
	retries, err := u.repo.ListRetryableDeliveries(ctx, 100)
	if err != nil {
		return processed, err
	}
	for _, row := range retries {
		payload := map[string]any{}
		_ = json.Unmarshal(row.Payload, &payload)
		endpoint, err := u.repo.GetEndpointByID(ctx, row.EndpointID)
		if err != nil {
			return processed, err
		}
		attempts := row.Attempts + 1
		statusCode, responseBody, deliveredAt, nextRetry := u.deliver(ctx, endpoint, row.EventType, payload, attempts)
		if err := u.repo.UpdateDeliveryResult(ctx, row.ID, statusCode, responseBody, attempts, nextRetry, deliveredAt); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (u *Usecases) CleanupOldDeliveries(ctx context.Context, days int) (int64, error) {
	if days <= 0 {
		days = 30
	}
	return u.repo.DeleteOldDeliveries(ctx, time.Now().UTC().AddDate(0, 0, -days))
}

func (u *Usecases) deliver(ctx context.Context, endpoint webhookdomain.Endpoint, eventType string, payload map[string]any, attempts int) (*int, string, *time.Time, *time.Time) {
	body := []byte("{}")
	if encoded, err := marshal(payload); err == nil {
		body = encoded
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	webhookID := uuid.NewString()
	sig := sign(endpoint.Secret, webhookID, timestamp, body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err.Error(), nil, nextRetryForAttempt(attempts)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-ID", webhookID)
	req.Header.Set("X-Webhook-Event", eventType)
	req.Header.Set("X-Webhook-Timestamp", timestamp)
	req.Header.Set("X-Webhook-Signature", sig)
	res, err := u.client.Do(req)
	if err != nil {
		return nil, err.Error(), nil, nextRetryForAttempt(attempts)
	}
	defer res.Body.Close()
	respBytes, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	statusCode := res.StatusCode
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		now := time.Now().UTC()
		return &statusCode, string(respBytes), &now, nil
	}
	return &statusCode, string(respBytes), nil, nextRetryForAttempt(attempts)
}

func (u *Usecases) validateURL(ctx context.Context, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
		return apperror.NewBadInput("invalid webhook url")
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(parsed.Hostname())), ".")
	if host == "" {
		return apperror.NewBadInput("invalid webhook url")
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return apperror.NewBadInput("webhook url cannot target localhost")
	}
	if ip := net.ParseIP(host); ip != nil {
		if err := validateResolvedIP(ip); err != nil {
			return err
		}
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	addrs, err := u.resolver.LookupIPAddr(lookupCtx, host)
	if err != nil || len(addrs) == 0 {
		return apperror.NewBadInput("webhook host cannot be resolved")
	}
	for _, addr := range addrs {
		if err := validateResolvedIP(addr.IP); err != nil {
			return err
		}
	}
	return nil
}

func validateResolvedIP(ip net.IP) error {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return apperror.NewBadInput("webhook host resolved to invalid ip")
	}
	if addr.IsLoopback() {
		return apperror.NewBadInput("webhook url cannot target localhost")
	}
	if addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return apperror.NewBadInput("webhook url cannot target private network")
	}
	return nil
}

func normalizeEndpoint(in webhookdomain.Endpoint) webhookdomain.Endpoint {
	in.URL = strings.TrimSpace(in.URL)
	in.Secret = strings.TrimSpace(in.Secret)
	for i := range in.Events {
		in.Events[i] = strings.TrimSpace(in.Events[i])
	}
	return in
}

func randomSecret() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func sign(secret, webhookID, timestamp string, body []byte) string {
	signed := webhookID + "." + timestamp + "." + string(body)
	h := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = h.Write([]byte(signed))
	return "v1=" + hex.EncodeToString(h.Sum(nil))
}

func nextRetryForAttempt(attempt int) *time.Time {
	if attempt >= 5 {
		return nil
	}
	next := time.Now().UTC().Add(time.Duration(attempt) * time.Minute)
	return &next
}

func marshal(v any) ([]byte, error) { return json.Marshal(v) }
