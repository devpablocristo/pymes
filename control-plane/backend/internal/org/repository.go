package org

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/org/repository/models"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/org/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateOrg(name, slug, externalID, actor string) domain.Organization {
	now := time.Now().UTC()
	m := models.OrgModel{
		ID:         uuid.New(),
		ExternalID: externalID,
		Name:       name,
		Slug:       slug,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	r.db.Create(&m)

	return domain.Organization{
		ID:         m.ID,
		ExternalID: m.ExternalID,
		Name:       m.Name,
		Slug:       m.Slug,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}
