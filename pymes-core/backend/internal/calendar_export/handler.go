package calendar_export

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_export/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_export/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
)

// usecasesPort acota lo que el handler necesita del usecase. Sirve para tests.
type usecasesPort interface {
	IssueToken(ctx context.Context, orgID uuid.UUID, actor, name string) (domain.IssueResult, error)
	ListMyTokens(ctx context.Context, orgID uuid.UUID, actor string) ([]domain.Token, error)
	RevokeToken(ctx context.Context, orgID uuid.UUID, actor string, id uuid.UUID) error
	RenderFeed(ctx context.Context, plaintext string) (string, domain.Token, error)
	MarkFeedUsed(ctx context.Context, tokenID uuid.UUID) error
}

type Handler struct {
	uc          usecasesPort
	publicBase  string // ej: "https://app.pymes.example" — usado para componer feed_url
}

// NewHandler construye el handler. publicBase es el origin público desde el
// que se servirá el feed (sin trailing slash). Si está vacío, feed_url cae a
// path relativo.
func NewHandler(uc *Usecases, publicBase string) *Handler {
	return &Handler{uc: uc, publicBase: strings.TrimRight(strings.TrimSpace(publicBase), "/")}
}

// RegisterAuthRoutes monta las rutas autenticadas que permiten al usuario
// interno listar/emitir/revocar sus tokens. Requieren auth Clerk.
func (h *Handler) RegisterAuthRoutes(auth *gin.RouterGroup) {
	auth.GET("/calendar-export/tokens", h.ListTokens)
	auth.POST("/calendar-export/tokens", h.IssueToken)
	auth.DELETE("/calendar-export/tokens/:id", h.RevokeToken)
}

// RegisterPublicRoutes monta el endpoint público del feed. NO va dentro de
// authGroup porque el cliente externo (Apple Calendar, Google Calendar, etc.)
// no tiene auth Clerk; sólo conoce el plaintext del token.
func (h *Handler) RegisterPublicRoutes(v1 *gin.RouterGroup) {
	v1.GET("/calendar/feed/:token", h.ServeFeed)
}

func (h *Handler) IssueToken(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil || orgID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid org context"})
		return
	}
	var req dto.IssueTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	result, err := h.uc.IssueToken(c.Request.Context(), orgID, authCtx.Actor, req.Name)
	if err != nil {
		log.Error().Err(err).Msg("calendar_export: issue token failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not issue token"})
		return
	}
	c.JSON(http.StatusCreated, dto.IssueTokenResponse{
		Token:     toResponseToken(result.Token),
		Plaintext: result.Plaintext,
		FeedURL:   h.composeFeedURL(result.Plaintext),
	})
}

func (h *Handler) ListTokens(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil || orgID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid org context"})
		return
	}
	tokens, err := h.uc.ListMyTokens(c.Request.Context(), orgID, authCtx.Actor)
	if err != nil {
		log.Error().Err(err).Msg("calendar_export: list tokens failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list tokens"})
		return
	}
	items := make([]dto.TokenResponse, 0, len(tokens))
	for _, t := range tokens {
		items = append(items, toResponseToken(t))
	}
	c.JSON(http.StatusOK, dto.ListTokensResponse{Items: items})
}

func (h *Handler) RevokeToken(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil || orgID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid org context"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.RevokeToken(c.Request.Context(), orgID, authCtx.Actor, id); err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
			return
		}
		log.Error().Err(err).Msg("calendar_export: revoke token failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not revoke token"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ServeFeed es el endpoint público de suscripción. La URL incluye `.ics` por
// convención (Apple Calendar y Google Calendar reconocen la extensión y
// muestran un nombre amigable en la UI de suscripción). Stripeamos el `.ics`
// del path antes de buscar el token.
func (h *Handler) ServeFeed(c *gin.Context) {
	raw := strings.TrimSuffix(c.Param("token"), ".ics")
	body, token, err := h.uc.RenderFeed(c.Request.Context(), raw)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		log.Error().Err(err).Msg("calendar_export: render feed failed")
		c.Status(http.StatusInternalServerError)
		return
	}
	// Mark fire-and-forget: usar context.Background() porque c.Request.Context()
	// se cancela apenas el response termina, y queremos que el UPDATE persista
	// aún si el cliente cierra la conexión inmediatamente.
	go func(id uuid.UUID) {
		if err := h.uc.MarkFeedUsed(context.Background(), id); err != nil {
			log.Warn().Err(err).Str("token_id", id.String()).Msg("calendar_export: touch last_used_at failed")
		}
	}(token.ID)

	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Cache-Control", "private, max-age=300")
	c.String(http.StatusOK, body)
}

func (h *Handler) composeFeedURL(plaintext string) string {
	path := "/v1/calendar/feed/" + plaintext + ".ics"
	if h.publicBase == "" {
		return path
	}
	return h.publicBase + path
}

func toResponseToken(t domain.Token) dto.TokenResponse {
	return dto.TokenResponse{
		ID:         t.ID.String(),
		Name:       t.Name,
		Scopes:     t.Scopes,
		LastUsedAt: t.LastUsedAt,
		RevokedAt:  t.RevokedAt,
		CreatedAt:  t.CreatedAt,
	}
}
