package calendar_export

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_export/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/calendar_export/usecases/domain"
)

// Sentinel errors visibles al usecase para mapear a HTTP correctamente.
var (
	ErrTokenNotFound = errors.New("calendar_export: token not found")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// CreateToken persiste un token nuevo. El caller ya debe haber computado el
// hash; el repository nunca ve plaintext.
func (r *Repository) CreateToken(ctx context.Context, t domain.Token) (domain.Token, error) {
	row := models.CalendarExportTokenModel{
		ID:        t.ID,
		OrgID:     t.OrgID,
		CreatedBy: t.CreatedBy,
		Name:      t.Name,
		TokenHash: t.TokenHash,
		Scopes:    t.Scopes,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Token{}, err
	}
	return toDomainToken(row), nil
}

// ListByCreator devuelve los tokens activos y revocados de un creador en una
// org. El caller decide si los quiere todos o filtrar revocados afuera.
func (r *Repository) ListByCreator(ctx context.Context, orgID uuid.UUID, createdBy string) ([]domain.Token, error) {
	var rows []models.CalendarExportTokenModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND created_by = ?", orgID, createdBy).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Token, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainToken(row))
	}
	return out, nil
}

// RevokeToken marca un token como revocado. Devuelve ErrTokenNotFound si el
// id/creador no matchean (defensa contra "revocar el token de otro usuario").
func (r *Repository) RevokeToken(ctx context.Context, orgID uuid.UUID, createdBy string, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&models.CalendarExportTokenModel{}).
		Where("org_id = ? AND created_by = ? AND id = ? AND revoked_at IS NULL", orgID, createdBy, id).
		Updates(map[string]any{"revoked_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// FindActiveByHash es el único path de lectura del feed público. Sólo devuelve
// tokens activos (no revocados). El caller decide qué hacer con el resultado;
// el repo no actualiza last_used_at acá para mantener la responsabilidad de
// observabilidad en el usecase.
func (r *Repository) FindActiveByHash(ctx context.Context, hash string) (domain.Token, error) {
	var row models.CalendarExportTokenModel
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL", hash).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Token{}, ErrTokenNotFound
		}
		return domain.Token{}, err
	}
	return toDomainToken(row), nil
}

// TouchLastUsed actualiza last_used_at sin bloquear el feed si falla. El caller
// debe ignorar el error si el feed ya respondió OK — la marca de uso es
// observabilidad, no correctness.
func (r *Repository) TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.CalendarExportTokenModel{}).
		Where("id = ?", id).
		Update("last_used_at", at).Error
}

func toDomainToken(row models.CalendarExportTokenModel) domain.Token {
	return domain.Token{
		ID:         row.ID,
		OrgID:      row.OrgID,
		CreatedBy:  row.CreatedBy,
		Name:       row.Name,
		TokenHash:  row.TokenHash,
		Scopes:     row.Scopes,
		LastUsedAt: row.LastUsedAt,
		RevokedAt:  row.RevokedAt,
		CreatedAt:  row.CreatedAt,
	}
}
