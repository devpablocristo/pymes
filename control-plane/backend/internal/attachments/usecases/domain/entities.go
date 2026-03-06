package domain

import (
	"time"

	"github.com/google/uuid"
)

type Attachment struct {
	ID             uuid.UUID `json:"id"`
	OrgID          uuid.UUID `json:"org_id"`
	AttachableType string    `json:"attachable_type"`
	AttachableID   uuid.UUID `json:"attachable_id"`
	FileName       string    `json:"file_name"`
	ContentType    string    `json:"content_type"`
	SizeBytes      int64     `json:"size_bytes"`
	StorageKey     string    `json:"storage_key"`
	UploadedBy     string    `json:"uploaded_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type UploadRequest struct {
	StorageKey string    `json:"storage_key"`
	UploadURL  string    `json:"upload_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type DownloadLink struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}
