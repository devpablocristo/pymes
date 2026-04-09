package whatsapp

import (
	cm "github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging"
	"gorm.io/gorm"
)

var (
	ErrNotFound       = cm.ErrNotFound
	ErrAlreadyExists  = cm.ErrAlreadyExists
	ErrNotConnected   = cm.ErrNotConnected
	ErrAlreadyOptedIn = cm.ErrAlreadyOptedIn
)

type Repository struct {
	*cm.Repository
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{Repository: cm.NewRepository(db)}
}
