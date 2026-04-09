// Package calendar_export expone tokens públicos de suscripción al calendario
// interno de la org. Combina los turnos cliente (scheduling_bookings) y los
// eventos internos (scheduling_calendar_events) en un único feed iCalendar
// que cualquier cliente compatible (Apple Calendar, Google Calendar, Outlook,
// Thunderbird) puede suscribir vía URL pública.
//
// El feed es read-only: el cliente externo nunca escribe nada. Y el flujo de
// emisión de tokens es estrictamente interno (auth Clerk + RBAC). El único
// endpoint público es GET /v1/calendar/feed/:token.ics, y la única forma de
// llegar a los datos de la org es conociendo el plaintext del token.
package calendar_export

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	ics "github.com/devpablocristo/core/calendar/ics/go"
	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_export/usecases/domain"
)

// SchedulingPort es el subset de scheduling.Usecases que necesita este módulo.
// Definido acá (no en el módulo scheduling) para respetar ISP: el módulo
// scheduling no tiene por qué saber que existe calendar_export.
type SchedulingPort interface {
	ListBookings(ctx context.Context, orgID uuid.UUID, filter schedulingdomain.ListBookingsFilter) ([]schedulingdomain.Booking, error)
	ListCalendarEvents(ctx context.Context, orgID uuid.UUID, filter schedulingdomain.ListCalendarEventsFilter) ([]schedulingdomain.CalendarEvent, error)
}

// RepositoryPort abstrae al adapter de DB para que el usecase no dependa de
// GORM. Sirve también para tests con fakes.
type RepositoryPort interface {
	CreateToken(ctx context.Context, t domain.Token) (domain.Token, error)
	ListByCreator(ctx context.Context, orgID uuid.UUID, createdBy string) ([]domain.Token, error)
	RevokeToken(ctx context.Context, orgID uuid.UUID, createdBy string, id uuid.UUID) error
	FindActiveByHash(ctx context.Context, hash string) (domain.Token, error)
	TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error
}

// Config controla parámetros del módulo. Hoy sólo el horizonte del feed; el
// día de mañana puede llevar TTL del token, scopes habilitados, etc.
type Config struct {
	// FeedHorizonDays define cuántos días hacia adelante (desde hoy) se
	// incluyen en el feed. Default 90 si es <=0.
	FeedHorizonDays int
	// FeedHistoryDays define cuántos días hacia atrás incluye el feed. Útil
	// para que apps externas puedan ver los últimos turnos completados.
	// Default 30 si es <=0.
	FeedHistoryDays int
	// ProductID se emite como PRODID del VCALENDAR. Recomendado: identificar
	// al producto en formato "-//Empresa//Producto//Idioma".
	ProductID string
}

func (c Config) horizonDays() int {
	if c.FeedHorizonDays <= 0 {
		return 90
	}
	return c.FeedHorizonDays
}

func (c Config) historyDays() int {
	if c.FeedHistoryDays <= 0 {
		return 30
	}
	return c.FeedHistoryDays
}

func (c Config) prodID() string {
	if strings.TrimSpace(c.ProductID) == "" {
		return "-//Pymes SaaS//Calendar Export//ES"
	}
	return c.ProductID
}

type Usecases struct {
	repo       RepositoryPort
	scheduling SchedulingPort
	cfg        Config
}

func NewUsecases(repo RepositoryPort, scheduling SchedulingPort, cfg Config) *Usecases {
	return &Usecases{repo: repo, scheduling: scheduling, cfg: cfg}
}

// IssueToken genera un token nuevo y devuelve el plaintext UNA SOLA VEZ.
// El plaintext es 32 bytes random codificados en hex (256 bits de entropía).
// Suficiente para que un atacante no pueda enumerarlos por fuerza bruta.
func (u *Usecases) IssueToken(ctx context.Context, orgID uuid.UUID, actor, name string) (domain.IssueResult, error) {
	if orgID == uuid.Nil {
		return domain.IssueResult{}, errors.New("calendar_export: org_id is required")
	}
	plaintext, err := generateTokenPlaintext()
	if err != nil {
		return domain.IssueResult{}, fmt.Errorf("calendar_export: generate token: %w", err)
	}
	hash := hashToken(plaintext)
	token := domain.Token{
		ID:        uuid.New(),
		OrgID:     orgID,
		CreatedBy: strings.TrimSpace(actor),
		Name:      strings.TrimSpace(name),
		TokenHash: hash,
		Scopes:    "all",
	}
	created, err := u.repo.CreateToken(ctx, token)
	if err != nil {
		return domain.IssueResult{}, err
	}
	return domain.IssueResult{Token: created, Plaintext: plaintext}, nil
}

// ListMyTokens devuelve todos los tokens (activos y revocados) que el actor
// emitió en su org. Ordenados por created_at desc en el repo.
func (u *Usecases) ListMyTokens(ctx context.Context, orgID uuid.UUID, actor string) ([]domain.Token, error) {
	return u.repo.ListByCreator(ctx, orgID, strings.TrimSpace(actor))
}

// RevokeToken marca un token como revocado. Sólo el creador puede revocarlo.
func (u *Usecases) RevokeToken(ctx context.Context, orgID uuid.UUID, actor string, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("calendar_export: id is required")
	}
	return u.repo.RevokeToken(ctx, orgID, strings.TrimSpace(actor), id)
}

// RenderFeed es el path público: dado un plaintext, valida que exista un token
// activo con ese hash, carga los datos del calendario de la org dueña, y
// devuelve el iCalendar serializado. NO actualiza last_used_at en el path
// crítico — eso se hace fire-and-forget afuera para no bloquear el response.
func (u *Usecases) RenderFeed(ctx context.Context, plaintext string) (string, domain.Token, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", domain.Token{}, ErrTokenNotFound
	}
	hash := hashToken(plaintext)
	token, err := u.repo.FindActiveByHash(ctx, hash)
	if err != nil {
		return "", domain.Token{}, err
	}
	cal, err := u.buildCalendar(ctx, token)
	if err != nil {
		return "", token, fmt.Errorf("calendar_export: build calendar: %w", err)
	}
	out, err := ics.Render(cal)
	if err != nil {
		return "", token, fmt.Errorf("calendar_export: render ics: %w", err)
	}
	return out, token, nil
}

// MarkFeedUsed actualiza last_used_at fuera del response path. Si falla,
// el caller lo debe loguear pero no propagar.
func (u *Usecases) MarkFeedUsed(ctx context.Context, tokenID uuid.UUID) error {
	return u.repo.TouchLastUsed(ctx, tokenID, time.Now().UTC())
}

func (u *Usecases) buildCalendar(ctx context.Context, token domain.Token) (ics.Calendar, error) {
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -u.cfg.historyDays())
	to := now.AddDate(0, 0, u.cfg.horizonDays())

	events := make([]ics.Event, 0)

	// Bookings: ListBookings filtra por día, así que iteramos día por día.
	// Suboptimal pero correcto. Para feeds del orden de 90 días son ~120
	// queries en frío que después caen en cache. TODO: agregar
	// ListBookingsBetween al módulo scheduling para reemplazar este loop.
	for day := startOfDay(from); !day.After(to); day = day.AddDate(0, 0, 1) {
		dayCopy := day
		bookings, err := u.scheduling.ListBookings(ctx, token.OrgID, schedulingdomain.ListBookingsFilter{
			Date:  &dayCopy,
			Limit: 500,
		})
		if err != nil {
			return ics.Calendar{}, fmt.Errorf("list bookings %s: %w", day.Format("2006-01-02"), err)
		}
		for _, b := range bookings {
			events = append(events, bookingToEvent(b))
		}
	}

	// Calendar events: este sí soporta from/to en una sola query.
	internalEvents, err := u.scheduling.ListCalendarEvents(ctx, token.OrgID, schedulingdomain.ListCalendarEventsFilter{
		From: &from,
		To:   &to,
	})
	if err != nil {
		return ics.Calendar{}, fmt.Errorf("list calendar events: %w", err)
	}
	for _, ev := range internalEvents {
		events = append(events, internalEventToEvent(ev))
	}

	calName := "Pymes — Mi agenda"
	if strings.TrimSpace(token.Name) != "" {
		calName = "Pymes — " + token.Name
	}

	return ics.Calendar{
		ProdID:      u.cfg.prodID(),
		Name:        calName,
		Description: "Turnos cliente y eventos internos exportados desde Pymes SaaS",
		Events:      events,
	}, nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func bookingToEvent(b schedulingdomain.Booking) ics.Event {
	summary := b.CustomerName
	if strings.TrimSpace(summary) == "" {
		summary = "Turno"
	}
	description := buildBookingDescription(b)
	status := ics.StatusConfirmed
	switch string(b.Status) {
	case "cancelled", "no_show", "expired":
		status = ics.StatusCancelled
	case "hold", "pending_confirmation":
		status = ics.StatusTentative
	}
	return ics.Event{
		UID:          fmt.Sprintf("booking-%s@pymes", b.ID.String()),
		DTSTAMP:      b.UpdatedAt.UTC(),
		Start:        b.StartAt.UTC(),
		End:          b.EndAt.UTC(),
		Summary:      summary,
		Description:  description,
		Status:       status,
		LastModified: b.UpdatedAt.UTC(),
	}
}

func internalEventToEvent(ev schedulingdomain.CalendarEvent) ics.Event {
	status := ics.StatusConfirmed
	switch ev.Status {
	case schedulingdomain.CalendarEventStatusCancelled:
		status = ics.StatusCancelled
	case schedulingdomain.CalendarEventStatusDone:
		status = ics.StatusConfirmed
	}
	return ics.Event{
		UID:          fmt.Sprintf("event-%s@pymes", ev.ID.String()),
		DTSTAMP:      ev.UpdatedAt.UTC(),
		Start:        ev.StartAt.UTC(),
		End:          ev.EndAt.UTC(),
		AllDay:       ev.AllDay,
		Summary:      ev.Title,
		Description:  ev.Description,
		Status:       status,
		LastModified: ev.UpdatedAt.UTC(),
	}
}

func buildBookingDescription(b schedulingdomain.Booking) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(b.CustomerPhone) != "" {
		parts = append(parts, "Teléfono: "+b.CustomerPhone)
	}
	if strings.TrimSpace(b.CustomerEmail) != "" {
		parts = append(parts, "Email: "+b.CustomerEmail)
	}
	if strings.TrimSpace(b.Notes) != "" {
		parts = append(parts, b.Notes)
	}
	return strings.Join(parts, "\n")
}

// generateTokenPlaintext devuelve 32 bytes random en hex (64 chars). Es lo que
// el cliente debe copiar y pegar en la URL del feed.
func generateTokenPlaintext() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
