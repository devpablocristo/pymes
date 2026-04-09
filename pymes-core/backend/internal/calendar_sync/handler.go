package calendar_sync

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_sync/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_sync/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
)

type usecasesPort interface {
	StartGoogleConnect(ctx context.Context, orgID uuid.UUID, actor string) (string, error)
	HandleGoogleCallback(ctx context.Context, state, code string) (domain.Connection, error)
	ListMyConnections(ctx context.Context, orgID uuid.UUID, actor string) ([]domain.Connection, error)
	RevokeConnection(ctx context.Context, orgID uuid.UUID, actor string, id uuid.UUID) error
}

type Handler struct {
	uc           usecasesPort
	frontendURL  string // origen del frontend para redirigir al usuario después del callback
}

// NewHandler construye el handler. frontendURL se usa para componer el redirect
// final post-callback (`/agenda?google_sync=ok` o `/ajustes?google_sync=err`).
func NewHandler(uc *Usecases, frontendURL string) *Handler {
	return &Handler{
		uc:          uc,
		frontendURL: strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
	}
}

// RegisterAuthRoutes monta las rutas que requieren auth Clerk: iniciar el
// connect, listar conexiones, revocar.
func (h *Handler) RegisterAuthRoutes(auth *gin.RouterGroup) {
	auth.POST("/calendar-sync/google/connect", h.StartGoogleConnect)
	auth.GET("/calendar-sync/connections", h.ListConnections)
	auth.DELETE("/calendar-sync/connections/:id", h.RevokeConnection)
}

// RegisterPublicRoutes monta el callback de OAuth. Va sin auth Clerk: el
// usuario llega acá redirigido desde Google, no desde nuestra app, así que el
// browser no necesariamente lleva la cookie de sesión. La autenticación del
// callback es el `state` (que validamos contra DB y matchea org+user).
func (h *Handler) RegisterPublicRoutes(v1 *gin.RouterGroup) {
	v1.GET("/calendar-sync/google/callback", h.GoogleCallback)
}

func (h *Handler) StartGoogleConnect(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil || orgID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid org context"})
		return
	}
	authURL, err := h.uc.StartGoogleConnect(c.Request.Context(), orgID, authCtx.Actor)
	if err != nil {
		log.Error().Err(err).Msg("calendar_sync: start google connect failed")
		// Mensaje genérico al cliente; detalle queda en el log.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not start google connect"})
		return
	}
	c.JSON(http.StatusOK, dto.StartConnectResponse{AuthURL: authURL})
}

// GoogleCallback recibe el ?code=&state= de Google y persiste la conexión.
// El response final es un redirect HTTP al frontend con un query param
// indicando el resultado: ?google_sync=connected o ?google_sync=error&reason=...
//
// Por qué redirect en vez de JSON: el browser del usuario llega acá tras el
// flow OAuth de Google; mostrar JSON sería una mala UX. El frontend lee el
// query param para mostrar un toast.
func (h *Handler) GoogleCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	googleErr := c.Query("error")

	// Si Google reportó error en el flow, no llega code. Redirigir con razón.
	if googleErr != "" {
		h.redirectResult(c, "error", googleErr)
		return
	}
	if state == "" || code == "" {
		h.redirectResult(c, "error", "missing_params")
		return
	}
	_, err := h.uc.HandleGoogleCallback(c.Request.Context(), state, code)
	if err != nil {
		log.Error().Err(err).Msg("calendar_sync: handle google callback failed")
		reason := "callback_failed"
		if errors.Is(err, ErrOAuthStateNotFound) {
			reason = "expired_state"
		}
		h.redirectResult(c, "error", reason)
		return
	}
	h.redirectResult(c, "connected", "")
}

func (h *Handler) ListConnections(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil || orgID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid org context"})
		return
	}
	conns, err := h.uc.ListMyConnections(c.Request.Context(), orgID, authCtx.Actor)
	if err != nil {
		log.Error().Err(err).Msg("calendar_sync: list connections failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list connections"})
		return
	}
	items := make([]dto.ConnectionResponse, 0, len(conns))
	for _, conn := range conns {
		items = append(items, toResponseConnection(conn))
	}
	c.JSON(http.StatusOK, dto.ListConnectionsResponse{Items: items})
}

func (h *Handler) RevokeConnection(c *gin.Context) {
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
	if err := h.uc.RevokeConnection(c.Request.Context(), orgID, authCtx.Actor, id); err != nil {
		if errors.Is(err, ErrConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
			return
		}
		log.Error().Err(err).Msg("calendar_sync: revoke connection failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not revoke connection"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) redirectResult(c *gin.Context, status, reason string) {
	target := h.frontendURL + "/agenda"
	if h.frontendURL == "" {
		target = "/agenda"
	}
	q := "?google_sync=" + status
	if reason != "" {
		q += "&reason=" + reason
	}
	c.Redirect(http.StatusFound, target+q)
}

func toResponseConnection(c domain.Connection) dto.ConnectionResponse {
	return dto.ConnectionResponse{
		ID:                   c.ID.String(),
		Provider:             string(c.Provider),
		ProviderAccountEmail: c.ProviderAccountEmail,
		ProviderCalendarID:   c.ProviderCalendarID,
		ProviderCalendarName: c.ProviderCalendarName,
		Scopes:               c.Scopes,
		LastSyncAt:           c.LastSyncAt,
		LastSyncError:        c.LastSyncError,
		RevokedAt:            c.RevokedAt,
		CreatedAt:            c.CreatedAt,
	}
}
