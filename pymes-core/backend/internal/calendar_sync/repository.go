package calendar_sync

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_sync/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_sync/usecases/domain"
)

var (
	ErrConnectionNotFound = errors.New("calendar_sync: connection not found")
	ErrOAuthStateNotFound = errors.New("calendar_sync: oauth state not found or expired")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ── conexiones ──────────────────────────────────────────────────────────────

// UpsertConnection crea o actualiza una conexión por (org, creator, provider).
// Si existe una activa para esa tripleta, la sobrescribe (mismo flujo de
// reconectar el mismo Google Calendar). Si está revocada, la "resucita"
// limpiando revoked_at.
func (r *Repository) UpsertConnection(ctx context.Context, conn domain.Connection) (domain.Connection, error) {
	now := time.Now().UTC()
	row := models.CalendarSyncConnectionModel{
		ID:                    conn.ID,
		OrgID:                 conn.OrgID,
		CreatedBy:             conn.CreatedBy,
		Provider:              string(conn.Provider),
		ProviderAccountEmail:  conn.ProviderAccountEmail,
		ProviderCalendarID:    conn.ProviderCalendarID,
		ProviderCalendarName:  conn.ProviderCalendarName,
		Scopes:                conn.Scopes,
		RefreshTokenEncrypted: conn.RefreshTokenEncrypted,
		AccessTokenEncrypted:  conn.AccessTokenEncrypted,
		AccessTokenExpiresAt:  conn.AccessTokenExpiresAt,
		SyncToken:             conn.SyncToken,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	// Buscar existente activa o revocada para este (org, creator, provider).
	var existing models.CalendarSyncConnectionModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND created_by = ? AND provider = ?", conn.OrgID, conn.CreatedBy, string(conn.Provider)).
		Take(&existing).Error
	if err == nil {
		// Update in place: conserva ID y created_at originales.
		row.ID = existing.ID
		row.CreatedAt = existing.CreatedAt
		row.UpdatedAt = now
		updates := map[string]any{
			"provider_account_email":  row.ProviderAccountEmail,
			"provider_calendar_id":    row.ProviderCalendarID,
			"provider_calendar_name":  row.ProviderCalendarName,
			"scopes":                  row.Scopes,
			"refresh_token_encrypted": row.RefreshTokenEncrypted,
			"access_token_encrypted":  row.AccessTokenEncrypted,
			"access_token_expires_at": row.AccessTokenExpiresAt,
			"sync_token":              row.SyncToken,
			"last_sync_at":            nil,
			"last_sync_error":         "",
			"revoked_at":              nil,
			"updated_at":              now,
		}
		if err := r.db.WithContext(ctx).
			Model(&models.CalendarSyncConnectionModel{}).
			Where("id = ?", existing.ID).
			Updates(updates).Error; err != nil {
			return domain.Connection{}, err
		}
		var fresh models.CalendarSyncConnectionModel
		if err := r.db.WithContext(ctx).Where("id = ?", existing.ID).Take(&fresh).Error; err != nil {
			return domain.Connection{}, err
		}
		return toDomainConnection(fresh), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Connection{}, err
	}
	// Crear nueva.
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Connection{}, err
	}
	return toDomainConnection(row), nil
}

// ListByCreator devuelve todas las conexiones (activas y revocadas) que el
// usuario emitió en su org. Útil para mostrar la sección "mis integraciones".
func (r *Repository) ListByCreator(ctx context.Context, orgID uuid.UUID, createdBy string) ([]domain.Connection, error) {
	var rows []models.CalendarSyncConnectionModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND created_by = ?", orgID, createdBy).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Connection, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainConnection(row))
	}
	return out, nil
}

// RevokeConnection marca como revocada. Defensa: sólo el creador puede
// revocar su propia conexión.
func (r *Repository) RevokeConnection(ctx context.Context, orgID uuid.UUID, createdBy string, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&models.CalendarSyncConnectionModel{}).
		Where("org_id = ? AND created_by = ? AND id = ? AND revoked_at IS NULL", orgID, createdBy, id).
		Updates(map[string]any{"revoked_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrConnectionNotFound
	}
	return nil
}

// ── oauth state (CSRF + cross-request handoff) ──────────────────────────────

func (r *Repository) CreateOAuthState(ctx context.Context, st domain.OAuthState) error {
	row := models.CalendarSyncOAuthStateModel{
		State:     st.State,
		OrgID:     st.OrgID,
		CreatedBy: st.CreatedBy,
		Provider:  string(st.Provider),
		ExpiresAt: st.ExpiresAt,
		CreatedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

// ConsumeOAuthState busca y borra (atomic) un state. Si no existe o expiró,
// devuelve ErrOAuthStateNotFound. Borrar inmediatamente garantiza que cada
// state se usa una sola vez (defensa replay).
func (r *Repository) ConsumeOAuthState(ctx context.Context, state string) (domain.OAuthState, error) {
	var row models.CalendarSyncOAuthStateModel
	err := r.db.WithContext(ctx).Where("state = ?", state).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.OAuthState{}, ErrOAuthStateNotFound
		}
		return domain.OAuthState{}, err
	}
	// Borrar siempre (consumido o expirado), independientemente de si vamos a
	// devolverlo válido al caller.
	if err := r.db.WithContext(ctx).
		Where("state = ?", state).
		Delete(&models.CalendarSyncOAuthStateModel{}).Error; err != nil {
		return domain.OAuthState{}, err
	}
	if row.ExpiresAt.Before(time.Now().UTC()) {
		return domain.OAuthState{}, ErrOAuthStateNotFound
	}
	return toDomainOAuthState(row), nil
}

// PurgeExpiredOAuthStates limpia los states que ya no son válidos. Se llama
// fire-and-forget desde el usecase para evitar que la tabla crezca sin límite.
func (r *Repository) PurgeExpiredOAuthStates(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now().UTC()).
		Delete(&models.CalendarSyncOAuthStateModel{}).Error
}

func toDomainConnection(row models.CalendarSyncConnectionModel) domain.Connection {
	return domain.Connection{
		ID:                    row.ID,
		OrgID:                 row.OrgID,
		CreatedBy:             row.CreatedBy,
		Provider:              domain.Provider(row.Provider),
		ProviderAccountEmail:  row.ProviderAccountEmail,
		ProviderCalendarID:    row.ProviderCalendarID,
		ProviderCalendarName:  row.ProviderCalendarName,
		Scopes:                row.Scopes,
		RefreshTokenEncrypted: row.RefreshTokenEncrypted,
		AccessTokenEncrypted:  row.AccessTokenEncrypted,
		AccessTokenExpiresAt:  row.AccessTokenExpiresAt,
		SyncToken:             row.SyncToken,
		LastSyncAt:            row.LastSyncAt,
		LastSyncError:         row.LastSyncError,
		RevokedAt:             row.RevokedAt,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}

func toDomainOAuthState(row models.CalendarSyncOAuthStateModel) domain.OAuthState {
	return domain.OAuthState{
		State:     row.State,
		OrgID:     row.OrgID,
		CreatedBy: row.CreatedBy,
		Provider:  domain.Provider(row.Provider),
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
	}
}
