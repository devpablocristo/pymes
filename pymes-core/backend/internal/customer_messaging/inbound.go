package customer_messaging

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/domain"
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

type AIMessageResponse struct {
	ConversationID string   `json:"conversation_id"`
	Reply          string   `json:"reply"`
	TokensUsed     int      `json:"tokens_used"`
	ToolCalls      []string `json:"tool_calls"`
}

type InteractiveButtonPayload struct {
	ID    string
	Title string
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
	Statuses []metaStatus         `json:"statuses"`
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

type metaStatus struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Errors    []metaStatusError `json:"errors"`
}

type metaStatusError struct {
	Code  int    `json:"code"`
	Title string `json:"title"`
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
	messages, statusUpdates, err := parseWebhookPayload(payload)
	if err != nil {
		return InboundResult{}, err
	}
	for _, update := range statusUpdates {
		if err := u.HandleStatusUpdate(ctx, update); err != nil && !errors.Is(err, ErrNotFound) {
			return InboundResult{}, err
		}
	}
	if len(messages) == 0 {
		return InboundResult{}, nil
	}
	if u.ai == nil {
		return InboundResult{}, domainerr.Unavailable("whatsapp ai bridge not configured")
	}
	if u.meta == nil {
		return InboundResult{}, domainerr.Unavailable("whatsapp delivery not configured")
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
		orgID := conn.OrgID
		msg.OrgID = orgID

		var convID *uuid.UUID
		var inboundPartyID *uuid.UUID
		partyID, partyName, _ := u.repo.GetPartyByPhone(ctx, orgID, msg.FromPhone)
		if partyID != uuid.Nil {
			conv, convErr := u.repo.GetOrCreateConversation(ctx, orgID, partyID, msg.FromPhone, partyName)
			if convErr == nil {
				convID = &conv.ID
				inboundPartyID = &partyID
				now := time.Now()
				inboundMsg := domain.Message{
					ID:             uuid.New(),
					OrgID:          orgID,
					PhoneNumberID:  msg.PhoneNumberID,
					Direction:      domain.DirectionInbound,
					WAMessageID:    msg.MessageID,
					FromPhone:      msg.FromPhone,
					ToPhone:        conn.PhoneNumberID,
					MessageType:    domain.TypeText,
					Body:           msg.Text,
					PartyID:        inboundPartyID,
					ConversationID: convID,
					Status:         domain.StatusDelivered,
					CreatedAt:      now,
					UpdatedAt:      now,
				}
				_ = u.repo.SaveMessage(ctx, inboundMsg)
				preview := msg.Text
				if len(preview) > 100 {
					preview = preview[:100]
				}
				_ = u.repo.UpdateConversationLastMessage(ctx, conv.ID, preview, true)
			}
		}

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
		waReplyID, sendErr := u.meta.SendTextMessage(ctx, conn.PhoneNumberID, accessToken, msg.FromPhone, reply.Reply)
		if sendErr != nil {
			return result, fmt.Errorf("send whatsapp response phone=%s msg=%s: %w", msg.PhoneNumberID, msg.MessageID, domainerr.UpstreamError("failed to send whatsapp response"))
		}

		if convID != nil {
			domainConn := domain.Connection{OrgID: orgID, PhoneNumberID: conn.PhoneNumberID}
			outMsg := u.buildOutboundMessage(domainConn, orgID, inboundPartyID, msg.FromPhone, domain.TypeText, reply.Reply, waReplyID)
			outMsg.ConversationID = convID
			outMsg.CreatedBy = "ai"
			_ = u.repo.SaveMessage(ctx, outMsg)
			replyPreview := reply.Reply
			if len(replyPreview) > 100 {
				replyPreview = replyPreview[:100]
			}
			_ = u.repo.UpdateConversationLastMessage(ctx, *convID, replyPreview, false)
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

func parseWebhookPayload(payload []byte) ([]InboundMessage, []domain.StatusUpdate, error) {
	var envelope metaWebhookEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, nil, domainerr.Validation("invalid whatsapp webhook payload")
	}
	messages := make([]InboundMessage, 0)
	statusUpdates := make([]domain.StatusUpdate, 0)
	for _, entry := range envelope.Entry {
		for _, change := range entry.Changes {
			phoneNumberID := strings.TrimSpace(change.Value.Metadata.PhoneNumberID)
			if phoneNumberID == "" {
				continue
			}
			field := strings.TrimSpace(change.Field)
			switch field {
			case "", "messages":
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
					messages = append(messages, InboundMessage{
						PhoneNumberID: phoneNumberID,
						FromPhone:     from,
						Text:          body,
						MessageID:     strings.TrimSpace(msg.ID),
						ProfileName:   profileName,
					})
				}
			case "statuses":
				for _, rawStatus := range change.Value.Statuses {
					status := strings.ToLower(strings.TrimSpace(rawStatus.Status))
					var mapped domain.MessageStatus
					switch status {
					case "sent":
						mapped = domain.StatusSent
					case "delivered":
						mapped = domain.StatusDelivered
					case "read":
						mapped = domain.StatusRead
					case "failed":
						mapped = domain.StatusFailed
					default:
						continue
					}
					update := domain.StatusUpdate{
						WAMessageID: strings.TrimSpace(rawStatus.ID),
						Status:      mapped,
					}
					if ts := strings.TrimSpace(rawStatus.Timestamp); ts != "" {
						if unixSeconds, parseErr := strconv.ParseInt(ts, 10, 64); parseErr == nil {
							update.Timestamp = time.Unix(unixSeconds, 0).UTC()
						}
					}
					if len(rawStatus.Errors) > 0 {
						update.ErrorCode = fmt.Sprintf("%d", rawStatus.Errors[0].Code)
						update.ErrorTitle = strings.TrimSpace(rawStatus.Errors[0].Title)
					}
					if update.WAMessageID != "" {
						statusUpdates = append(statusUpdates, update)
					}
				}
			}
		}
	}
	return messages, statusUpdates, nil
}
