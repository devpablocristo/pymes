package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/httpclient"
	cm "github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging"
)

type AIClient struct {
	caller *httpclient.Caller
}

type MetaClient struct {
	caller *httpclient.Caller
}

type CustomerMessagingInboundRequest struct {
	OrgID         string `json:"org_id"`
	PhoneNumberID string `json:"phone_number_id"`
	FromPhone     string `json:"from_phone"`
	Message       string `json:"message"`
	MessageID     string `json:"message_id,omitempty"`
	ProfileName   string `json:"profile_name,omitempty"`
}

type metaSendResponse struct {
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
	Success bool `json:"success"`
}

func NewAIClient(baseURL, internalToken string) *AIClient {
	h := make(http.Header)
	if t := strings.TrimSpace(internalToken); t != "" {
		h.Set("X-Internal-Service-Token", t)
	}
	return &AIClient{
		caller: &httpclient.Caller{
			BaseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
			Header:      h,
			HTTP:        &http.Client{Timeout: 15 * time.Second},
			MaxBodySize: 32 * 1024,
		},
	}
}

func NewMetaClient(baseURL string) *MetaClient {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://graph.facebook.com/v23.0"
	}
	return &MetaClient{
		caller: &httpclient.Caller{
			BaseURL:     base,
			HTTP:        &http.Client{Timeout: 15 * time.Second},
			MaxBodySize: 32 * 1024,
		},
	}
}

func (c *AIClient) ProcessWhatsApp(ctx context.Context, req cm.InboundMessage) (cm.AIMessageResponse, error) {
	if c == nil || c.caller.BaseURL == "" {
		return cm.AIMessageResponse{}, fmt.Errorf("ai service url not configured")
	}
	body := CustomerMessagingInboundRequest{
		OrgID:         req.OrgID.String(),
		PhoneNumberID: req.PhoneNumberID,
		FromPhone:     req.FromPhone,
		Message:       req.Text,
		MessageID:     req.MessageID,
		ProfileName:   req.ProfileName,
	}
	st, raw, err := c.caller.DoJSON(ctx, http.MethodPost, "/v1/internal/customer-messaging/inbound", body)
	if err != nil {
		return cm.AIMessageResponse{}, err
	}
	if st >= http.StatusMultipleChoices {
		return cm.AIMessageResponse{}, fmt.Errorf("ai service returned %d: %s", st, strings.TrimSpace(string(raw)))
	}
	var out cm.AIMessageResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return cm.AIMessageResponse{}, err
	}
	return out, nil
}

func (c *MetaClient) sendMessage(ctx context.Context, phoneNumberID, accessToken string, payload any) (string, error) {
	if c == nil || c.caller.BaseURL == "" {
		return "", fmt.Errorf("whatsapp graph api base url not configured")
	}
	if strings.TrimSpace(accessToken) == "" {
		return "", fmt.Errorf("whatsapp access token is required")
	}

	path := "/" + url.PathEscape(strings.TrimSpace(phoneNumberID)) + "/messages"
	st, raw, err := c.caller.DoJSON(ctx, http.MethodPost, path, payload,
		httpclient.WithHeader("Authorization", "Bearer "+strings.TrimSpace(accessToken)),
	)
	if err != nil {
		return "", fmt.Errorf("whatsapp api request: %w", err)
	}
	if st >= http.StatusMultipleChoices {
		return "", fmt.Errorf("meta graph api returned %d: %s", st, strings.TrimSpace(string(raw)))
	}

	var sendResp metaSendResponse
	if err := json.Unmarshal(raw, &sendResp); err != nil {
		return "", fmt.Errorf("decode whatsapp send response: %w", err)
	}
	if len(sendResp.Messages) > 0 && strings.TrimSpace(sendResp.Messages[0].ID) != "" {
		return sendResp.Messages[0].ID, nil
	}
	if sendResp.Success {
		return "", nil
	}
	return "", fmt.Errorf("meta graph api returned success without message id: %s", strings.TrimSpace(string(raw)))
}

func (c *MetaClient) SendTextMessage(ctx context.Context, phoneNumberID, accessToken, to, body string) (string, error) {
	return c.sendMessage(ctx, phoneNumberID, accessToken, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                strings.TrimSpace(to),
		"type":              "text",
		"text": map[string]any{
			"preview_url": false,
			"body":        strings.TrimSpace(body),
		},
	})
}

func (c *MetaClient) SendTemplateMessage(ctx context.Context, phoneNumberID, accessToken, to, templateName, language string, params []string) (string, error) {
	components := make([]map[string]any, 0)
	if len(params) > 0 {
		parameters := make([]map[string]any, 0, len(params))
		for _, p := range params {
			parameters = append(parameters, map[string]any{"type": "text", "text": p})
		}
		components = append(components, map[string]any{"type": "body", "parameters": parameters})
	}
	lang := strings.TrimSpace(language)
	if lang == "" {
		lang = "es"
	}
	return c.sendMessage(ctx, phoneNumberID, accessToken, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                strings.TrimSpace(to),
		"type":              "template",
		"template": map[string]any{
			"name":       strings.TrimSpace(templateName),
			"language":   map[string]any{"code": lang},
			"components": components,
		},
	})
}

func (c *MetaClient) SendMediaMessage(ctx context.Context, phoneNumberID, accessToken, to, mediaType, mediaURL, caption string) (string, error) {
	mt := strings.TrimSpace(strings.ToLower(mediaType))
	mediaPayload := map[string]any{"link": strings.TrimSpace(mediaURL)}
	if strings.TrimSpace(caption) != "" {
		mediaPayload["caption"] = strings.TrimSpace(caption)
	}
	return c.sendMessage(ctx, phoneNumberID, accessToken, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                strings.TrimSpace(to),
		"type":              mt,
		mt:                  mediaPayload,
	})
}

func (c *MetaClient) SendInteractiveButtons(ctx context.Context, phoneNumberID, accessToken, to, body string, buttons []cm.InteractiveButtonPayload) (string, error) {
	actionButtons := make([]map[string]any, 0, len(buttons))
	for _, b := range buttons {
		actionButtons = append(actionButtons, map[string]any{
			"type":  "reply",
			"reply": map[string]any{"id": strings.TrimSpace(b.ID), "title": strings.TrimSpace(b.Title)},
		})
	}
	return c.sendMessage(ctx, phoneNumberID, accessToken, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                strings.TrimSpace(to),
		"type":              "interactive",
		"interactive": map[string]any{
			"type":   "button",
			"body":   map[string]any{"text": strings.TrimSpace(body)},
			"action": map[string]any{"buttons": actionButtons},
		},
	})
}

func (c *MetaClient) MarkAsRead(ctx context.Context, phoneNumberID, accessToken, messageID string) error {
	_, err := c.sendMessage(ctx, phoneNumberID, accessToken, map[string]any{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        strings.TrimSpace(messageID),
	})
	return err
}
