package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/backend/go/domainerr"
)

type Connection struct {
	OrgID         uuid.UUID
	PhoneNumberID string
	WABAID        string
	AccessToken   string
	IsActive      bool
}

type TokenCrypto interface {
	Encrypt(plainText string) (string, error)
	Decrypt(cipherText string) (string, error)
}

type AIClientPort interface {
	ProcessWhatsApp(ctx context.Context, req InboundMessage) (AIMessageResponse, error)
}

type MetaClientPort interface {
	SendTextMessage(ctx context.Context, phoneNumberID, accessToken, to, body string) (string, error)
	SendTemplateMessage(ctx context.Context, phoneNumberID, accessToken, to, templateName, language string, params []string) (string, error)
	SendMediaMessage(ctx context.Context, phoneNumberID, accessToken, to, mediaType, mediaURL, caption string) (string, error)
	SendInteractiveButtons(ctx context.Context, phoneNumberID, accessToken, to, body string, buttons []InteractiveButtonPayload) (string, error)
	MarkAsRead(ctx context.Context, phoneNumberID, accessToken, messageID string) error
}

type InboundMessage struct {
	OrgID         uuid.UUID
	PhoneNumberID string
	FromPhone     string
	Text          string
	MessageID     string
	ProfileName   string
}

type InboundResult struct {
	Processed int `json:"processed"`
	Replied   int `json:"replied"`
}

type metaWebhookEnvelope struct {
	Object string             `json:"object"`
	Entry  []metaWebhookEntry `json:"entry"`
}

type metaWebhookEntry struct {
	Changes []metaWebhookChange `json:"changes"`
}

type metaWebhookChange struct {
	Field string           `json:"field"`
	Value metaWebhookValue `json:"value"`
}

type metaWebhookValue struct {
	Metadata metaMetadata         `json:"metadata"`
	Contacts []metaContact        `json:"contacts"`
	Messages []metaInboundMessage `json:"messages"`
}

type metaMetadata struct {
	PhoneNumberID string `json:"phone_number_id"`
}

type metaContact struct {
	WaID    string `json:"wa_id"`
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
}

type metaInboundMessage struct {
	ID   string `json:"id"`
	From string `json:"from"`
	Type string `json:"type"`
	Text struct {
		Body string `json:"body"`
	} `json:"text"`
}

func (u *Usecases) VerifyWebhook(mode, token, challenge string) (string, error) {
	if strings.TrimSpace(mode) != "subscribe" {
		return "", domainerr.Validation("invalid webhook mode")
	}
	if strings.TrimSpace(challenge) == "" {
		return "", domainerr.Validation("missing webhook challenge")
	}
	expected := strings.TrimSpace(u.webhookVerifyToken)
	if expected == "" {
		return "", domainerr.Forbidden("whatsapp webhook verify token not configured")
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(token)), []byte(expected)) != 1 {
		return "", domainerr.Forbidden("invalid webhook verify token")
	}
	return strings.TrimSpace(challenge), nil
}

func (u *Usecases) ValidateWebhookSignature(signatureHeader string, payload []byte) error {
	secret := strings.TrimSpace(u.webhookAppSecret)
	if secret == "" {
		return domainerr.Unavailable("whatsapp webhook app secret is not configured")
	}

	provided := strings.ToLower(strings.TrimSpace(signatureHeader))
	if !strings.HasPrefix(provided, "sha256=") {
		return domainerr.Forbidden("invalid whatsapp webhook signature")
	}
	provided = strings.TrimSpace(strings.TrimPrefix(provided, "sha256="))
	if provided == "" {
		return domainerr.Forbidden("invalid whatsapp webhook signature")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return domainerr.Forbidden("invalid whatsapp webhook signature")
	}
	return nil
}

func (u *Usecases) HandleInboundWebhook(ctx context.Context, payload []byte) (InboundResult, error) {
	if u.ai == nil {
		return InboundResult{}, domainerr.Unavailable("whatsapp ai bridge not configured")
	}
	if u.meta == nil {
		return InboundResult{}, domainerr.Unavailable("whatsapp delivery not configured")
	}

	messages, err := parseInboundMessages(payload)
	if err != nil {
		return InboundResult{}, err
	}
	if len(messages) == 0 {
		return InboundResult{}, nil
	}

	result := InboundResult{}
	for _, msg := range messages {
		conn, err := u.repo.GetConnectionByPhoneNumberID(ctx, msg.PhoneNumberID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return result, err
		}
		msg.OrgID = conn.OrgID

		reply, err := u.ai.ProcessWhatsApp(ctx, msg)
		if err != nil {
			return result, fmt.Errorf("process whatsapp message phone=%s msg=%s: %w", msg.PhoneNumberID, msg.MessageID, domainerr.UpstreamError("failed to process whatsapp message"))
		}
		result.Processed++
		if strings.TrimSpace(reply.Reply) == "" {
			continue
		}

		accessToken, err := u.resolveAccessToken(conn.AccessToken)
		if err != nil {
			return result, domainerr.UpstreamError("failed to decrypt whatsapp access token")
		}
		if _, err := u.meta.SendTextMessage(ctx, conn.PhoneNumberID, accessToken, msg.FromPhone, reply.Reply); err != nil {
			return result, fmt.Errorf("send whatsapp response phone=%s msg=%s: %w", msg.PhoneNumberID, msg.MessageID, domainerr.UpstreamError("failed to send whatsapp response"))
		}
		result.Replied++
	}
	return result, nil
}

func (u *Usecases) resolveAccessToken(stored string) (string, error) {
	token := strings.TrimSpace(stored)
	if token == "" {
		return "", domainerr.Validation("whatsapp access token is empty")
	}
	if u.tokenCrypto == nil {
		return token, nil
	}
	return u.tokenCrypto.Decrypt(token)
}

func parseInboundMessages(payload []byte) ([]InboundMessage, error) {
	var envelope metaWebhookEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, domainerr.Validation("invalid whatsapp webhook payload")
	}

	out := make([]InboundMessage, 0)
	for _, entry := range envelope.Entry {
		for _, change := range entry.Changes {
			if field := strings.TrimSpace(change.Field); field != "" && field != "messages" {
				continue
			}
			phoneNumberID := strings.TrimSpace(change.Value.Metadata.PhoneNumberID)
			if phoneNumberID == "" {
				continue
			}

			contactNames := map[string]string{}
			for _, contact := range change.Value.Contacts {
				name := strings.TrimSpace(contact.Profile.Name)
				if name == "" {
					continue
				}
				contactNames[strings.TrimSpace(contact.WaID)] = name
			}

			for _, msg := range change.Value.Messages {
				if strings.TrimSpace(strings.ToLower(msg.Type)) != "text" {
					continue
				}
				body := strings.TrimSpace(msg.Text.Body)
				from := strings.TrimSpace(msg.From)
				if body == "" || from == "" {
					continue
				}
				profileName := contactNames[from]
				if profileName == "" && len(change.Value.Contacts) == 1 {
					profileName = strings.TrimSpace(change.Value.Contacts[0].Profile.Name)
				}
				out = append(out, InboundMessage{
					PhoneNumberID: phoneNumberID,
					FromPhone:     from,
					Text:          body,
					MessageID:     strings.TrimSpace(msg.ID),
					ProfileName:   profileName,
				})
			}
		}
	}
	return out, nil
}
