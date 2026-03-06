package notifications

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/notifications/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/internal/shared/httperrors"
)

type EmailSender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

type RepositoryPort interface {
	GetUserByExternalID(externalID string) (uuid.UUID, string, bool)
	ListMembers(orgID uuid.UUID) []Member
	GetPreferences(userID uuid.UUID) []domain.Preference
	UpsertPreference(userID uuid.UUID, notifType, channel string, enabled bool) domain.Preference
	HasLogByDedupKey(key string) bool
	CreateLog(entry domain.Log)
}

type Member struct {
	UserID uuid.UUID
	Email  string
	Role   string
}

type Usecases struct {
	repo   RepositoryPort
	sender EmailSender
	logger zerolog.Logger
}

func NewUsecases(repo RepositoryPort, sender EmailSender, logger zerolog.Logger) *Usecases {
	return &Usecases{repo: repo, sender: sender, logger: logger}
}

func (u *Usecases) GetPreferencesByActor(ctx context.Context, actor string) ([]domain.Preference, error) {
	_ = ctx
	userID, _, ok := u.repo.GetUserByExternalID(actor)
	if !ok {
		return nil, fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	return u.repo.GetPreferences(userID), nil
}

func (u *Usecases) UpdatePreferenceByActor(ctx context.Context, actor, notifType, channel string, enabled bool) (domain.Preference, error) {
	_ = ctx
	userID, _, ok := u.repo.GetUserByExternalID(actor)
	if !ok {
		return domain.Preference{}, fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	if strings.TrimSpace(notifType) == "" || strings.TrimSpace(channel) == "" {
		return domain.Preference{}, fmt.Errorf("notification_type and channel are required: %w", httperrors.ErrBadInput)
	}
	return u.repo.UpsertPreference(userID, strings.TrimSpace(notifType), strings.TrimSpace(channel), enabled), nil
}

func (u *Usecases) Notify(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) error {
	members := u.repo.ListMembers(orgID)
	for _, m := range members {
		if m.Role != "admin" {
			continue
		}
		if err := u.sendToUser(ctx, orgID, m.UserID, m.Email, notifType, data); err != nil {
			u.logger.Error().Err(err).Str("org_id", orgID.String()).Str("notif_type", notifType).Msg("notify admin failed")
		}
	}
	return nil
}

func (u *Usecases) NotifyUser(ctx context.Context, userExternalID string, notifType string, data map[string]string) error {
	userID, email, ok := u.repo.GetUserByExternalID(userExternalID)
	if !ok {
		return fmt.Errorf("user not found: %w", httperrors.ErrNotFound)
	}
	return u.sendToUser(ctx, uuid.Nil, userID, email, notifType, data)
}

func (u *Usecases) sendToUser(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, email, notifType string, data map[string]string) error {
	hourBucket := time.Now().UTC().Format("2006010215")
	referenceID := data["reference_id"]
	dedupKey := fmt.Sprintf("%s|%s|%s|%s", notifType, userID.String(), referenceID, hourBucket)
	if u.repo.HasLogByDedupKey(dedupKey) {
		return nil
	}

	subject, textBody, htmlBody := templateFor(notifType, data)
	if err := u.sender.Send(ctx, email, subject, htmlBody, textBody); err != nil {
		return err
	}

	u.repo.CreateLog(domain.Log{
		ID:               uuid.New(),
		OrgID:            orgID,
		UserID:           userID,
		NotificationType: notifType,
		Channel:          "email",
		Status:           "sent",
		DedupKey:         dedupKey,
		CreatedAt:        time.Now().UTC(),
	})
	return nil
}

func templateFor(notifType string, data map[string]string) (subject string, textBody string, htmlBody string) {
	switch notifType {
	case "welcome":
		subject = "Bienvenido a Pymes SaaS"
		textBody = "Tu cuenta ya esta activa."
		htmlBody, _ = renderBaseTemplate(templateData{
			Title:   "Bienvenido",
			Message: textBody,
		})
	case "plan_upgraded":
		subject = "Plan actualizado"
		textBody = "Tu organizacion actualizo su plan."
		htmlBody, _ = renderBaseTemplate(templateData{
			Title:   "Plan actualizado",
			Message: textBody,
		})
	case "payment_failed":
		subject = "Error de pago"
		textBody = "No se pudo procesar el pago."
		htmlBody, _ = renderBaseTemplate(templateData{
			Title:   "Error de pago",
			Message: textBody,
		})
	case "subscription_canceled":
		subject = "Suscripcion cancelada"
		textBody = "La suscripcion fue cancelada."
		htmlBody, _ = renderBaseTemplate(templateData{
			Title:   "Suscripcion cancelada",
			Message: textBody,
		})
	default:
		subject = "Notificacion"
		textBody = "Tienes una nueva notificacion."
		htmlBody, _ = renderBaseTemplate(templateData{
			Title:   "Notificacion",
			Message: textBody,
		})
	}
	if msg := strings.TrimSpace(data["message"]); msg != "" {
		textBody = msg
		rendered, err := renderBaseTemplate(templateData{Title: "Notificacion", Message: msg})
		if err == nil {
			htmlBody = rendered
		}
	}
	if strings.TrimSpace(htmlBody) == "" {
		htmlBody = "<h1>Notificacion</h1><p>" + textBody + "</p>"
	}
	return subject, textBody, htmlBody
}
