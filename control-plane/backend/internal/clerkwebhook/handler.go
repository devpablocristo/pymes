package clerkwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type UsersPort interface {
	UpsertClerkUser(ctx context.Context, externalID, email, name, avatarURL string) error
	DeleteClerkUser(ctx context.Context, externalID string) error
	UpsertOrgMembership(ctx context.Context, orgID, userExternalID, role string) error
	DeleteOrgMembership(ctx context.Context, orgID, userExternalID string) error
}

type NotificationPort interface {
	NotifyUser(ctx context.Context, userExternalID string, notifType string, data map[string]string) error
}

type Handler struct {
	usersUC       UsersPort
	notifications NotificationPort
	webhookSecret string
	frontendURL  string
	logger        zerolog.Logger
}

func NewHandler(usersUC UsersPort, notifications NotificationPort, webhookSecret, frontendURL string, logger zerolog.Logger) *Handler {
	return &Handler{
		usersUC:       usersUC,
		notifications: notifications,
		webhookSecret: webhookSecret,
		frontendURL:  frontendURL,
		logger:        logger,
	}
}

func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	v1.POST("/webhooks/clerk", h.HandleWebhook)
}

func (h *Handler) HandleWebhook(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	if err := h.verifySvix(c, body); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var envelope struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if err := h.dispatch(c.Request.Context(), envelope.Type, envelope.Data); err != nil {
		h.logger.Error().Err(err).Str("event", envelope.Type).Msg("clerk webhook dispatch failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) dispatch(ctx context.Context, eventType string, data map[string]any) error {
	switch eventType {
	case "user.created", "user.updated":
		externalID := asString(data["id"])
		email := firstEmail(data)
		name := strings.TrimSpace(asString(data["first_name"]) + " " + asString(data["last_name"]))
		avatar := asString(data["image_url"])
		if err := h.usersUC.UpsertClerkUser(ctx, externalID, email, name, avatar); err != nil {
			return err
		}
		if eventType == "user.created" && h.notifications != nil {
			if err := h.notifications.NotifyUser(ctx, externalID, "welcome", map[string]string{"frontend_url": h.frontendURL}); err != nil {
				// No fallar el webhook por error de notificación.
				h.logger.Error().Err(err).Str("user_external_id", externalID).Msg("welcome notification failed")
			}
		}
		return nil
	case "user.deleted":
		return h.usersUC.DeleteClerkUser(ctx, asString(data["id"]))
	case "organizationMembership.created":
		return h.usersUC.UpsertOrgMembership(ctx, asString(data["organization_id"]), getNestedString(data, "public_user_data", "user_id"), asString(data["role"]))
	case "organizationMembership.deleted":
		return h.usersUC.DeleteOrgMembership(ctx, asString(data["organization_id"]), getNestedString(data, "public_user_data", "user_id"))
	case "organization.created":
		return nil
	default:
		h.logger.Info().Str("event", eventType).Msg("ignored clerk event")
		return nil
	}
}

func (h *Handler) verifySvix(c *gin.Context, body []byte) error {
	if strings.TrimSpace(h.webhookSecret) == "" {
		return nil
	}
	svixID := strings.TrimSpace(c.GetHeader("svix-id"))
	svixTS := strings.TrimSpace(c.GetHeader("svix-timestamp"))
	svixSig := strings.TrimSpace(c.GetHeader("svix-signature"))
	if svixID == "" || svixTS == "" || svixSig == "" {
		return fmt.Errorf("missing svix headers")
	}
	ts, err := strconv.ParseInt(svixTS, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid svix timestamp")
	}
	if d := time.Since(time.Unix(ts, 0)); d > 5*time.Minute || d < -5*time.Minute {
		return fmt.Errorf("svix timestamp expired")
	}

	secret := strings.TrimPrefix(strings.TrimSpace(h.webhookSecret), "whsec_")
	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return fmt.Errorf("invalid webhook secret")
	}

	msg := svixID + "." + svixTS + "." + string(body)
	mac := hmac.New(sha256.New, decoded)
	mac.Write([]byte(msg))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	for _, candidate := range splitSignatures(svixSig) {
		if hmac.Equal([]byte(candidate), []byte(expected)) {
			return nil
		}
	}
	return fmt.Errorf("invalid svix signature")
}

func splitSignatures(v string) []string {
	replacer := strings.NewReplacer(",", " ", "v1", " ", "=", " ")
	parts := strings.Fields(replacer.Replace(v))
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			res = append(res, p)
		}
	}
	return res
}

func firstEmail(data map[string]any) string {
	raw, ok := data["email_addresses"]
	if !ok {
		return ""
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	obj, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}
	return asString(obj["email_address"])
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func getNestedString(obj map[string]any, keys ...string) string {
	var current any = obj
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[key]
	}
	return asString(current)
}
