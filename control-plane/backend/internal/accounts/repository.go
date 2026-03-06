package accounts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/accounts/repository/models"
	accountsdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/accounts/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/pagination"
)

type Repository struct { db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, accountType, entityType string, onlyNonZero bool, limit int) ([]accountsdomain.Account, error) {
	limit = pagination.NormalizeLimit(limit, 20, 100)
	q := r.db.WithContext(ctx).Model(&models.AccountModel{}).Where("org_id = ?", orgID)
	if accountType != "" { q = q.Where("type = ?", accountType) }
	if entityType != "" { q = q.Where("entity_type = ?", entityType) }
	if onlyNonZero { q = q.Where("balance != 0") }
	var rows []models.AccountModel
	if err := q.Order("balance DESC").Order("updated_at DESC").Limit(limit).Find(&rows).Error; err != nil { return nil, err }
	out := make([]accountsdomain.Account, 0, len(rows))
	for _, row := range rows { out = append(out, toAccountDomain(row, nil)) }
	return out, nil
}

func (r *Repository) ListMovements(ctx context.Context, orgID, accountID uuid.UUID, limit int) ([]accountsdomain.Movement, error) {
	limit = pagination.NormalizeLimit(limit, 20, 100)
	var rows []models.MovementModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND account_id = ?", orgID, accountID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil { return nil, err }
	out := make([]accountsdomain.Movement, 0, len(rows))
	for _, row := range rows { out = append(out, toMovementDomain(row)) }
	return out, nil
}

func (r *Repository) CreateOrAdjust(ctx context.Context, in accountsdomain.Account, amount float64, description, actor string) (accountsdomain.Account, error) {
	var out accountsdomain.Account
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row models.AccountModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("org_id = ? AND entity_type = ? AND entity_id = ?", in.OrgID, in.EntityType, in.EntityID).Take(&row).Error
		now := time.Now().UTC()
		if err != nil {
			if err != gorm.ErrRecordNotFound { return err }
			row = models.AccountModel{ID: uuid.New(), OrgID: in.OrgID, Type: in.Type, EntityType: in.EntityType, EntityID: in.EntityID, EntityName: in.EntityName, Currency: defaultString(in.Currency, "ARS"), CreditLimit: in.CreditLimit, Balance: 0, UpdatedAt: now}
			if err := tx.Create(&row).Error; err != nil { return err }
		}
		row.EntityName = in.EntityName
		if in.CreditLimit > 0 { row.CreditLimit = in.CreditLimit }
		if in.Currency != "" { row.Currency = in.Currency }
		row.Balance += amount
		row.UpdatedAt = now
		if err := tx.Save(&row).Error; err != nil { return err }
		mv := models.MovementModel{ID: uuid.New(), AccountID: row.ID, OrgID: in.OrgID, Type: "adjustment", Amount: amount, Balance: row.Balance, Description: defaultString(description, "manual adjustment"), ReferenceType: "manual", CreatedBy: actor, CreatedAt: now}
		if err := tx.Create(&mv).Error; err != nil { return err }
		out = toAccountDomain(row, []models.MovementModel{mv})
		return nil
	})
	if err != nil { return accountsdomain.Account{}, err }
	return out, nil
}

func toAccountDomain(row models.AccountModel, movements []models.MovementModel) accountsdomain.Account {
	out := accountsdomain.Account{ID: row.ID, OrgID: row.OrgID, Type: row.Type, EntityType: row.EntityType, EntityID: row.EntityID, EntityName: row.EntityName, Balance: row.Balance, Currency: row.Currency, CreditLimit: row.CreditLimit, UpdatedAt: row.UpdatedAt}
	for _, mv := range movements { out.Movements = append(out.Movements, toMovementDomain(mv)) }
	return out
}

func toMovementDomain(row models.MovementModel) accountsdomain.Movement {
	return accountsdomain.Movement{ID: row.ID, AccountID: row.AccountID, OrgID: row.OrgID, Type: row.Type, Amount: row.Amount, Balance: row.Balance, Description: row.Description, ReferenceType: row.ReferenceType, ReferenceID: row.ReferenceID, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt}
}

func defaultString(v, def string) string {
	if v == "" { return def }
	return v
}
