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

// metaSendResponse es la respuesta de Meta Graph API al enviar un mensaje o actualizar estado (p. ej. read).
type metaSendResponse struct {
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
	Success bool `json:"success"`
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

// sendMessage envía un payload genérico a Meta Graph API y retorna el wa_message_id.
func (c *MetaClient) sendMessage(ctx context.Context, phoneNumberID, accessToken string, payload any) (string, error) {
	if c == nil || c.baseURL == "" {
		return "", fmt.Errorf("whatsapp graph api base url not configured")
	}
	if strings.TrimSpace(accessToken) == "" {
		return "", fmt.Errorf("whatsapp access token is required")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal whatsapp payload: %w", err)
	}

	endpoint := c.baseURL + "/" + url.PathEscape(strings.TrimSpace(phoneNumberID)) + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create whatsapp request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("whatsapp api request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("meta graph api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var sendResp metaSendResponse
	if err := json.Unmarshal(respBody, &sendResp); err != nil {
		return "", fmt.Errorf("decode whatsapp send response: %w", err)
	}
	if len(sendResp.Messages) > 0 && strings.TrimSpace(sendResp.Messages[0].ID) != "" {
		return sendResp.Messages[0].ID, nil
	}
	// Marcar como leído u otras operaciones devuelven {"success": true} sin messages[].
	if sendResp.Success {
		return "", nil
	}
	return "", fmt.Errorf("meta graph api returned success without message id: %s", strings.TrimSpace(string(respBody)))
}

// SendTextMessage envía un mensaje de texto simple.
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

// SendTemplateMessage envía un template message aprobado por Meta.
func (c *MetaClient) SendTemplateMessage(ctx context.Context, phoneNumberID, accessToken, to, templateName, language string, params []string) (string, error) {
	components := make([]map[string]any, 0)
	if len(params) > 0 {
		parameters := make([]map[string]any, 0, len(params))
		for _, p := range params {
			parameters = append(parameters, map[string]any{
				"type": "text",
				"text": p,
			})
		}
		components = append(components, map[string]any{
			"type":       "body",
			"parameters": parameters,
		})
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
			"name": strings.TrimSpace(templateName),
			"language": map[string]any{
				"code": lang,
			},
			"components": components,
		},
	})
}

// SendMediaMessage envía un mensaje con imagen, documento, audio o video.
func (c *MetaClient) SendMediaMessage(ctx context.Context, phoneNumberID, accessToken, to, mediaType, mediaURL, caption string) (string, error) {
	mt := strings.TrimSpace(strings.ToLower(mediaType))
	mediaPayload := map[string]any{
		"link": strings.TrimSpace(mediaURL),
	}
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

// SendInteractiveButtons envía un mensaje con hasta 3 botones de respuesta rápida.
func (c *MetaClient) SendInteractiveButtons(ctx context.Context, phoneNumberID, accessToken, to, body string, buttons []InteractiveButtonPayload) (string, error) {
	actionButtons := make([]map[string]any, 0, len(buttons))
	for _, b := range buttons {
		actionButtons = append(actionButtons, map[string]any{
			"type": "reply",
			"reply": map[string]any{
				"id":    strings.TrimSpace(b.ID),
				"title": strings.TrimSpace(b.Title),
			},
		})
	}

	return c.sendMessage(ctx, phoneNumberID, accessToken, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                strings.TrimSpace(to),
		"type":              "interactive",
		"interactive": map[string]any{
			"type": "button",
			"body": map[string]any{
				"text": strings.TrimSpace(body),
			},
			"action": map[string]any{
				"buttons": actionButtons,
			},
		},
	})
}

// InteractiveButtonPayload para construir botones de respuesta rápida.
type InteractiveButtonPayload struct {
	ID    string
	Title string
}

// MarkAsRead marca un mensaje como leído.
func (c *MetaClient) MarkAsRead(ctx context.Context, phoneNumberID, accessToken, messageID string) error {
	_, err := c.sendMessage(ctx, phoneNumberID, accessToken, map[string]any{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        strings.TrimSpace(messageID),
	})
	return err
}
