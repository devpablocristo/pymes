package dataio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/backend/go/apperror"
)

func (u *Usecases) savePreview(job previewJob) (string, error) {
	if err := os.MkdirAll(u.tempDir, 0o755); err != nil {
		return "", err
	}
	job.ID = uuid.NewString()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(u.tempDir, job.ID+".json"), payload, 0o600); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (u *Usecases) loadPreview(previewID string) (previewJob, error) {
	payload, err := os.ReadFile(filepath.Join(u.tempDir, previewID+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return previewJob{}, apperror.NewNotFound("preview", previewID)
		}
		return previewJob{}, err
	}
	var job previewJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return previewJob{}, apperror.NewBadInput("invalid preview payload")
	}
	return job, nil
}
