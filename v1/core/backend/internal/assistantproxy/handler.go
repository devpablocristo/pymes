package assistantproxy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devpablocristo/platform/http/go/httpclient"
	"github.com/devpablocristo/platform/http/go/pagination"
	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/core/backend/internal/shared/handlers"
)

const productSurface = "pymes"

type Config struct {
	BaseURL             string
	APIKey              string
	InternalJWTSecret   string
	InternalJWTIssuer   string
	InternalJWTAudience string
	HTTP                *http.Client
	Now                 func() time.Time
}

type Handler struct {
	client *Client
}

func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	ai := auth.Group("/ai")
	ai.POST("/chat", h.chat)
	ai.GET("/chat/conversations", h.listConversations)
	ai.GET("/chat/conversations/:id", h.getConversation)
	ai.POST("/notifications", h.notifications)
	ai.GET("/watchers", h.listWatchers)
	ai.POST("/watchers", h.createWatcher)
	ai.PATCH("/watchers/:id", h.updateWatcher)
}

type Client struct {
	caller  *httpclient.Caller
	apiKey  string
	jwt     jwtSigner
	baseURL string
}

type jwtSigner struct {
	secret   string
	issuer   string
	audience string
	now      func() time.Time
}

func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	return &Client{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(cfg.APIKey),
		jwt: jwtSigner{
			secret:   strings.TrimSpace(cfg.InternalJWTSecret),
			issuer:   defaultString(cfg.InternalJWTIssuer, "axis-bff"),
			audience: defaultString(cfg.InternalJWTAudience, "companion"),
			now:      cfg.Now,
		},
		caller: &httpclient.Caller{
			BaseURL:     baseURL,
			HTTP:        firstHTTPClient(cfg.HTTP),
			MaxBodySize: 1 << 20,
		},
	}
}

func (c *Client) configured() bool {
	if c == nil || c.baseURL == "" {
		return false
	}
	return c.jwt.secret != "" || c.apiKey != ""
}

func (c *Client) Do(ctx context.Context, method, path string, body any, auth handlers.AuthContext) (int, []byte, error) {
	if !c.configured() {
		return 0, nil, fmt.Errorf("companion proxy not configured")
	}
	opts := []httpclient.RequestOption{
		httpclient.WithHeader("X-Product-Surface", productSurface),
	}
	if token, ok := c.jwt.Sign(auth); ok {
		opts = append(opts, httpclient.WithHeader("Authorization", "Bearer "+token))
	} else if c.apiKey != "" {
		opts = append(opts, httpclient.WithHeader("X-API-Key", c.apiKey))
	}
	return c.caller.DoJSON(ctx, method, path, body, opts...)
}

type chatRequest struct {
	TaskID            string          `json:"task_id,omitempty"`
	ChatID            string          `json:"chat_id,omitempty"`
	Message           string          `json:"message"`
	Channel           string          `json:"channel,omitempty"`
	ProductSurface    string          `json:"product_surface,omitempty"`
	AgentID           string          `json:"agent_id,omitempty"`
	RouteHint         string          `json:"route_hint,omitempty"`
	PreferredLanguage string          `json:"preferred_language,omitempty"`
	ConfirmedActions  []string        `json:"confirmed_actions,omitempty"`
	Handoff           json.RawMessage `json:"handoff,omitempty"`
}

type companionChatRequest struct {
	TaskID         string `json:"task_id,omitempty"`
	ChatID         string `json:"chat_id,omitempty"`
	Message        string `json:"message"`
	Channel        string `json:"channel,omitempty"`
	ProductSurface string `json:"product_surface"`
	AgentID        string `json:"agent_id,omitempty"`
}

func (h *Handler) chat(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "invalid request body"})
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "message is required"})
		return
	}
	auth := handlers.GetAuthContext(c)
	body := companionChatRequest{
		TaskID:         strings.TrimSpace(req.TaskID),
		ChatID:         strings.TrimSpace(req.ChatID),
		Message:        req.Message,
		Channel:        defaultString(req.Channel, "pymes_ui"),
		ProductSurface: productSurface,
		AgentID:        strings.TrimSpace(req.AgentID),
	}
	status, raw, err := h.client.Do(c.Request.Context(), http.MethodPost, "/v1/chat", body, auth)
	if err != nil {
		writeProxyUnavailable(c, err)
		return
	}
	if status >= http.StatusMultipleChoices {
		c.Data(status, "application/json", raw)
		return
	}
	normalized, err := normalizeChatResponse(raw, req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": "companion_invalid_response", "message": "invalid companion response"})
		return
	}
	c.Data(status, "application/json", normalized)
}

func (h *Handler) listConversations(c *gin.Context) {
	limit := handlers.ParseLimitQuery(c, "limit", "50", pagination.Config{DefaultLimit: 50, MaxLimit: 200})
	path := "/v1/chat/conversations?limit=" + url.QueryEscape(strconv.Itoa(limit))
	h.proxy(c, http.MethodGet, path, nil)
}

func (h *Handler) getConversation(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "conversation id is required"})
		return
	}
	h.proxy(c, http.MethodGet, "/v1/chat/conversations/"+url.PathEscape(id), nil)
}

func (h *Handler) notifications(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		body = map[string]any{}
	}
	h.proxy(c, http.MethodPost, "/v1/notifications", body)
}

func (h *Handler) listWatchers(c *gin.Context) {
	path := "/v1/watchers"
	if raw := c.Request.URL.RawQuery; raw != "" {
		path += "?" + raw
	}
	h.proxy(c, http.MethodGet, path, nil)
}

func (h *Handler) createWatcher(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "invalid request body"})
		return
	}
	h.proxy(c, http.MethodPost, "/v1/watchers", body)
}

func (h *Handler) updateWatcher(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "watcher id is required"})
		return
	}
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "invalid request body"})
		return
	}
	h.proxy(c, http.MethodPatch, "/v1/watchers/"+url.PathEscape(id), body)
}

func (h *Handler) proxy(c *gin.Context, method, path string, body any) {
	status, raw, err := h.client.Do(c.Request.Context(), method, path, body, handlers.GetAuthContext(c))
	if err != nil {
		writeProxyUnavailable(c, err)
		return
	}
	c.Data(status, "application/json", raw)
}

func normalizeChatResponse(raw []byte, req chatRequest) ([]byte, error) {
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if _, ok := out["routed_agent"]; !ok {
		if route := strings.TrimSpace(req.RouteHint); route != "" {
			out["routed_agent"] = route
			out["routing_source"] = "ui_hint"
		} else {
			out["routed_agent"] = "general"
			out["routing_source"] = "orchestrator"
		}
	}
	if _, ok := out["output_kind"]; !ok {
		out["output_kind"] = "chat"
	}
	if _, ok := out["pending_confirmations"]; !ok {
		out["pending_confirmations"] = []any{}
	}
	if _, ok := out["request_id"]; !ok {
		if runID, _ := out["run_id"].(string); runID != "" {
			out["request_id"] = runID
		} else {
			out["request_id"] = ""
		}
	}
	return json.Marshal(out)
}

func writeProxyUnavailable(c *gin.Context, _ error) {
	c.JSON(http.StatusBadGateway, gin.H{
		"code":    "companion_unavailable",
		"message": "No se pudo contactar el servicio Companion",
	})
}

func (s jwtSigner) Sign(auth handlers.AuthContext) (string, bool) {
	if strings.TrimSpace(s.secret) == "" {
		return "", false
	}
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	actor := defaultString(auth.Actor, "pymes-backend")
	claims := map[string]any{
		"iss":               s.issuer,
		"aud":               s.audience,
		"sub":               actor,
		"org_id":            strings.TrimSpace(auth.OrgID),
		"actor_id":          actor,
		"actor_type":        "human",
		"role":              strings.TrimSpace(auth.Role),
		"scope":             "companion:tasks:read companion:tasks:write companion:watchers:read companion:watchers:write",
		"service_principal": true,
		"on_behalf_of":      actor,
		"product_surface":   productSurface,
		"iat":               now.Unix(),
		"nbf":               now.Add(-30 * time.Second).Unix(),
		"exp":               now.Add(5 * time.Minute).Unix(),
	}
	if strings.TrimSpace(auth.AuthMethod) == "api_key" {
		claims["actor_type"] = "service"
	}
	token, err := signHS256(claims, s.secret)
	if err != nil {
		return "", false
	}
	return token, true
}

func signHS256(claims map[string]any, secret string) (string, error) {
	headerJSON, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(header + "." + payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return header + "." + payload + "." + signature, nil
}

func firstHTTPClient(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func defaultString(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}
