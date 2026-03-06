package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AIClient struct {
	baseURL       string
	internalToken string
	client        *http.Client
}

type MetaClient struct {
	baseURL string
	client  *http.Client
}

type AIMessageRequest struct {
	OrgID         string `json:"org_id"`
	PhoneNumberID string `json:"phone_number_id"`
	FromPhone     string `json:"from_phone"`
	Message       string `json:"message"`
	MessageID     string `json:"message_id,omitempty"`
	ProfileName   string `json:"profile_name,omitempty"`
}

type AIMessageResponse struct {
	ConversationID string   `json:"conversation_id"`
	Reply          string   `json:"reply"`
	TokensUsed     int      `json:"tokens_used"`
	ToolCalls      []string `json:"tool_calls"`
}

func NewAIClient(baseURL, internalToken string) *AIClient {
	return &AIClient{
		baseURL:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		internalToken: strings.TrimSpace(internalToken),
		client:        &http.Client{Timeout: 15 * time.Second},
	}
}

func NewMetaClient(baseURL string) *MetaClient {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://graph.facebook.com/v23.0"
	}
	return &MetaClient{
		baseURL: base,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *AIClient) ProcessWhatsApp(ctx context.Context, req InboundMessage) (AIMessageResponse, error) {
	if c == nil || c.baseURL == "" {
		return AIMessageResponse{}, fmt.Errorf("ai service url not configured")
	}
	payload, err := json.Marshal(AIMessageRequest{
		OrgID:         req.OrgID.String(),
		PhoneNumberID: req.PhoneNumberID,
		FromPhone:     req.FromPhone,
		Message:       req.Text,
		MessageID:     req.MessageID,
		ProfileName:   req.ProfileName,
	})
	if err != nil {
		return AIMessageResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/internal/whatsapp/message", bytes.NewReader(payload))
	if err != nil {
		return AIMessageResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.internalToken != "" {
		httpReq.Header.Set("X-Internal-Service-Token", c.internalToken)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return AIMessageResponse{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode >= http.StatusMultipleChoices {
		return AIMessageResponse{}, fmt.Errorf("ai service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out AIMessageResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return AIMessageResponse{}, err
	}
	return out, nil
}

func (c *MetaClient) SendTextMessage(ctx context.Context, phoneNumberID, accessToken, to, body string) error {
	if c == nil || c.baseURL == "" {
		return fmt.Errorf("whatsapp graph api base url not configured")
	}
	if strings.TrimSpace(accessToken) == "" {
		return fmt.Errorf("whatsapp access token is required")
	}

	payload, err := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                strings.TrimSpace(to),
		"type":              "text",
		"text": map[string]any{
			"preview_url": false,
			"body":        strings.TrimSpace(body),
		},
	})
	if err != nil {
		return err
	}

	endpoint := c.baseURL + "/" + url.PathEscape(strings.TrimSpace(phoneNumberID)) + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("meta graph api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
