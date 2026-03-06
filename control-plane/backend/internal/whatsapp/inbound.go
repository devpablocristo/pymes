package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pkgs/go-pkg/apperror"
)

type Connection struct {
	OrgID         uuid.UUID
	PhoneNumberID string
	WABAID        string
	AccessToken   string
	IsActive      bool
}

type TokenCrypto interface {
	Decrypt(cipherText string) (string, error)
}

type AIClientPort interface {
	ProcessWhatsApp(ctx context.Context, req InboundMessage) (AIMessageResponse, error)
}

type MetaClientPort interface {
	SendTextMessage(ctx context.Context, phoneNumberID, accessToken, to, body string) error
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
		return "", apperror.NewBadInput("invalid webhook mode")
	}
	if strings.TrimSpace(challenge) == "" {
		return "", apperror.NewBadInput("missing webhook challenge")
	}
	expected := strings.TrimSpace(u.webhookVerifyToken)
	if expected == "" {
		return "", apperror.NewForbidden("whatsapp webhook verify token not configured")
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(token)), []byte(expected)) != 1 {
		return "", apperror.NewForbidden("invalid webhook verify token")
	}
	return strings.TrimSpace(challenge), nil
}

func (u *Usecases) ValidateWebhookSignature(signatureHeader string, payload []byte) error {
	secret := strings.TrimSpace(u.webhookAppSecret)
	if secret == "" {
		return nil
	}

	provided := strings.ToLower(strings.TrimSpace(signatureHeader))
	if !strings.HasPrefix(provided, "sha256=") {
		return apperror.NewForbidden("invalid whatsapp webhook signature")
	}
	provided = strings.TrimSpace(strings.TrimPrefix(provided, "sha256="))
	if provided == "" {
		return apperror.NewForbidden("invalid whatsapp webhook signature")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return apperror.NewForbidden("invalid whatsapp webhook signature")
	}
	return nil
}

func (u *Usecases) HandleInboundWebhook(ctx context.Context, payload []byte) (InboundResult, error) {
	if u.ai == nil {
		return InboundResult{}, apperror.New("service_unavailable", "whatsapp ai bridge not configured", 503)
	}
	if u.meta == nil {
		return InboundResult{}, apperror.New("service_unavailable", "whatsapp delivery not configured", 503)
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
			return result, apperror.New("upstream_error", "failed to process whatsapp message", 502).WithMeta(map[string]any{
				"phone_number_id": msg.PhoneNumberID,
				"message_id":      msg.MessageID,
			})
		}
		result.Processed++
		if strings.TrimSpace(reply.Reply) == "" {
			continue
		}

		accessToken, err := u.resolveAccessToken(conn.AccessToken)
		if err != nil {
			return result, apperror.New("upstream_error", "failed to decrypt whatsapp access token", 502)
		}
		if err := u.meta.SendTextMessage(ctx, conn.PhoneNumberID, accessToken, msg.FromPhone, reply.Reply); err != nil {
			return result, apperror.New("upstream_error", "failed to send whatsapp response", 502).WithMeta(map[string]any{
				"phone_number_id": msg.PhoneNumberID,
				"message_id":      msg.MessageID,
			})
		}
		result.Replied++
	}
	return result, nil
}

func (u *Usecases) resolveAccessToken(stored string) (string, error) {
	token := strings.TrimSpace(stored)
	if token == "" {
		return "", apperror.NewBadInput("whatsapp access token is empty")
	}
	if u.tokenCrypto == nil {
		return token, nil
	}
	return u.tokenCrypto.Decrypt(token)
}

func parseInboundMessages(payload []byte) ([]InboundMessage, error) {
	var envelope metaWebhookEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, apperror.NewBadInput("invalid whatsapp webhook payload")
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
