package attachments

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	attachmentdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/attachments/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/apperror"
)

type RepositoryPort interface {
	Create(ctx context.Context, in attachmentdomain.Attachment) (attachmentdomain.Attachment, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (attachmentdomain.Attachment, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	ListByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, limit int) ([]attachmentdomain.Attachment, error)
}

type Usecases struct {
	repo     RepositoryPort
	rootPath string
}

func NewUsecases(repo RepositoryPort, rootPath string) *Usecases {
	if strings.TrimSpace(rootPath) == "" {
		rootPath = "/tmp/attachments"
	}
	return &Usecases{repo: repo, rootPath: rootPath}
}

func (u *Usecases) RequestUpload(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, fileName, contentType string, sizeBytes int64) (attachmentdomain.UploadRequest, error) {
	if orgID == uuid.Nil || entityID == uuid.Nil || strings.TrimSpace(entityType) == "" {
		return attachmentdomain.UploadRequest{}, apperror.NewBadInput("org_id, entity_type and entity_id are required")
	}
	storageKey := filepath.Join(orgID.String(), strings.TrimSpace(entityType), entityID.String(), uuid.NewString()+"-"+sanitizeFileName(fileName))
	if err := os.MkdirAll(filepath.Dir(u.absolutePath(storageKey)), 0o755); err != nil {
		return attachmentdomain.UploadRequest{}, err
	}
	_ = contentType
	_ = sizeBytes
	now := time.Now().UTC()
	return attachmentdomain.UploadRequest{StorageKey: storageKey, UploadURL: "/v1/attachments/uploads/" + storageKey, ExpiresAt: now.Add(15 * time.Minute)}, nil
}

func (u *Usecases) SaveUpload(ctx context.Context, in attachmentdomain.Attachment) (attachmentdomain.Attachment, error) {
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(in.ContentType) == "" {
		in.ContentType = "application/octet-stream"
	}
	path := u.absolutePath(in.StorageKey)
	info, err := os.Stat(path)
	if err != nil {
		return attachmentdomain.Attachment{}, fmt.Errorf("uploaded file not found: %w", err)
	}
	if in.SizeBytes == 0 {
		in.SizeBytes = info.Size()
	}
	return u.repo.Create(ctx, in)
}

func (u *Usecases) UploadContent(ctx context.Context, storageKey string, body io.Reader) error {
	path := u.absolutePath(storageKey)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, body)
	return err
}

func (u *Usecases) GetDownloadLink(ctx context.Context, orgID, id uuid.UUID) (attachmentdomain.Attachment, attachmentdomain.DownloadLink, error) {
	item, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return attachmentdomain.Attachment{}, attachmentdomain.DownloadLink{}, apperror.NewNotFound("attachment", id.String())
		}
		return attachmentdomain.Attachment{}, attachmentdomain.DownloadLink{}, err
	}
	now := time.Now().UTC()
	return item, attachmentdomain.DownloadLink{URL: "/v1/attachments/" + id.String() + "/download", ExpiresAt: now.Add(15 * time.Minute)}, nil
}

func (u *Usecases) OpenContent(ctx context.Context, orgID, id uuid.UUID) (attachmentdomain.Attachment, *os.File, error) {
	item, _, err := u.GetDownloadLink(ctx, orgID, id)
	if err != nil {
		return attachmentdomain.Attachment{}, nil, err
	}
	file, err := os.Open(u.absolutePath(item.StorageKey))
	if err != nil {
		return attachmentdomain.Attachment{}, nil, err
	}
	return item, file, nil
}

func (u *Usecases) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	item, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperror.NewNotFound("attachment", id.String())
		}
		return err
	}
	_ = os.Remove(u.absolutePath(item.StorageKey))
	return u.repo.Delete(ctx, orgID, id)
}

func (u *Usecases) ListByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, limit int) ([]attachmentdomain.Attachment, error) {
	return u.repo.ListByEntity(ctx, orgID, strings.TrimSpace(entityType), entityID, limit)
}

func (u *Usecases) absolutePath(storageKey string) string {
	clean := filepath.Clean(strings.TrimPrefix(storageKey, "/"))
	return filepath.Join(u.rootPath, clean)
}

func sanitizeFileName(v string) string {
	name := strings.TrimSpace(v)
	if name == "" {
		return "file.bin"
	}
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "-")
	return name
}
