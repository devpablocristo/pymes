package products

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type captureProductRepo struct {
	created    productdomain.Product
	updated    productdomain.Product
	existing   *productdomain.Product
	getByIDErr error
}

func (r *captureProductRepo) List(context.Context, ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}
func (r *captureProductRepo) Create(_ context.Context, in productdomain.Product) (productdomain.Product, error) {
	r.created = in
	return in, nil
}
func (r *captureProductRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (productdomain.Product, error) {
	if r.getByIDErr != nil {
		return productdomain.Product{}, r.getByIDErr
	}
	if r.existing != nil {
		return *r.existing, nil
	}
	return productdomain.Product{}, nil
}
func (r *captureProductRepo) Update(_ context.Context, in productdomain.Product) (productdomain.Product, error) {
	r.updated = in
	return in, nil
}
func (r *captureProductRepo) Archive(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *captureProductRepo) Restore(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *captureProductRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error  { return nil }

func TestCreateDefaultsCurrency(t *testing.T) {
	t.Parallel()

	repo := &captureProductRepo{}
	uc := NewUsecases(repo, nil, nil)

	out, err := uc.Create(context.Background(), productdomain.Product{
		OrgID:    uuid.New(),
		Name:     "Producto demo",
		ImageURL: "  https://cdn.example.com/p.png  ",
	}, "tester")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.Currency != "ARS" {
		t.Fatalf("expected currency ARS, got %q", out.Currency)
	}
	if repo.created.Currency != "ARS" {
		t.Fatalf("expected repo to receive currency ARS, got %q", repo.created.Currency)
	}
	if out.ImageURL != "https://cdn.example.com/p.png" {
		t.Fatalf("expected trimmed image_url, got %q", out.ImageURL)
	}
	if repo.created.ImageURL != "https://cdn.example.com/p.png" {
		t.Fatalf("expected repo to receive trimmed image_url, got %q", repo.created.ImageURL)
	}
}

func TestCreateSyncsPrimaryFromImageURLs(t *testing.T) {
	t.Parallel()

	repo := &captureProductRepo{}
	uc := NewUsecases(repo, nil, nil)

	out, err := uc.Create(context.Background(), productdomain.Product{
		OrgID:     uuid.New(),
		Name:      "Multifoto",
		ImageURLs: []string{"  https://a.example/x.png  ", "https://b.example/y.png"},
	}, "tester")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.ImageURL != "https://a.example/x.png" {
		t.Fatalf("expected primary image_url from first image_urls, got %q", out.ImageURL)
	}
	if len(repo.created.ImageURLs) != 2 || repo.created.ImageURLs[0] != "https://a.example/x.png" {
		t.Fatalf("unexpected image_urls in repo: %#v", repo.created.ImageURLs)
	}
}

func TestCreateAcceptsLargeDataURLImages(t *testing.T) {
	t.Parallel()

	repo := &captureProductRepo{}
	uc := NewUsecases(repo, nil, nil)
	dataURL := "data:image/png;base64," + string(make([]byte, 20_000))

	out, err := uc.Create(context.Background(), productdomain.Product{
		OrgID:     uuid.New(),
		Name:      "Foto local",
		ImageURLs: []string{dataURL},
	}, "tester")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.ImageURL != dataURL {
		t.Fatalf("expected primary image_url from data url, got %q", out.ImageURL)
	}
	if len(repo.created.ImageURLs) != 1 || repo.created.ImageURLs[0] != dataURL {
		t.Fatalf("unexpected image_urls in repo: %#v", repo.created.ImageURLs)
	}
}

// TestUpdateMapsNotFoundSentinelToHTTPNotFound cubre Bug 1: antes del fix la
// comparación con gorm.ErrRecordNotFound nunca matcheaba y el error caía
// como 500. Ahora se mapea al sentinel httperrors.ErrNotFound.
func TestUpdateMapsNotFoundSentinelToHTTPNotFound(t *testing.T) {
	t.Parallel()

	repo := &captureProductRepo{getByIDErr: ErrNotFound}
	uc := NewUsecases(repo, nil, nil)

	_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{}, "tester")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("expected httperrors.ErrNotFound, got %v", err)
	}
}

// TestUpdateRejectsArchivedProductWithConflict cubre Bug 2: actualizar un
// producto archivado debe devolver Conflict (409), no NotFound (404) y
// menos aún un 500.
func TestUpdateRejectsArchivedProductWithConflict(t *testing.T) {
	t.Parallel()

	archivedAt := time.Now().UTC()
	orgID := uuid.New()
	prodID := uuid.New()
	repo := &captureProductRepo{existing: &productdomain.Product{
		ID:        prodID,
		OrgID:     orgID,
		Name:      "Producto archivado",
		DeletedAt: &archivedAt,
	}}
	uc := NewUsecases(repo, nil, nil)

	_, err := uc.Update(context.Background(), orgID, prodID, UpdateInput{}, "tester")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, httperrors.ErrConflict) {
		t.Fatalf("expected httperrors.ErrConflict, got %v", err)
	}
}

// TestUpdatePreservesImageURLsWhenOnlyImageURLSent cubre Bug 5: enviar sólo
// image_url no debe borrar la galería existente en image_urls.
func TestUpdatePreservesImageURLsWhenOnlyImageURLSent(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	prodID := uuid.New()
	existing := productdomain.Product{
		ID:        prodID,
		OrgID:     orgID,
		Name:      "Galería intacta",
		ImageURL:  "https://a.example/thumb-old.png",
		ImageURLs: []string{"https://a.example/1.png", "https://a.example/2.png"},
	}
	repo := &captureProductRepo{existing: &existing}
	uc := NewUsecases(repo, nil, nil)

	newPrimary := "https://a.example/thumb-new.png"
	_, err := uc.Update(context.Background(), orgID, prodID, UpdateInput{ImageURL: &newPrimary}, "tester")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.updated.ImageURL != newPrimary {
		t.Fatalf("expected primary %q, got %q", newPrimary, repo.updated.ImageURL)
	}
	if len(repo.updated.ImageURLs) != 2 {
		t.Fatalf("expected gallery preserved (2 items), got %d: %#v", len(repo.updated.ImageURLs), repo.updated.ImageURLs)
	}
}

// TestUpdateWithEmptyImageURLsClearsGallery verifica que enviar image_urls=[]
// sí vacía la galería explícitamente (complementa el test anterior).
func TestUpdateWithEmptyImageURLsClearsGallery(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	prodID := uuid.New()
	existing := productdomain.Product{
		ID:        prodID,
		OrgID:     orgID,
		Name:      "Limpiar galería",
		ImageURL:  "https://a.example/1.png",
		ImageURLs: []string{"https://a.example/1.png", "https://a.example/2.png"},
	}
	repo := &captureProductRepo{existing: &existing}
	uc := NewUsecases(repo, nil, nil)

	empty := []string{}
	_, err := uc.Update(context.Background(), orgID, prodID, UpdateInput{ImageURLs: &empty}, "tester")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.updated.ImageURLs) != 0 {
		t.Fatalf("expected gallery cleared, got %#v", repo.updated.ImageURLs)
	}
	if repo.updated.ImageURL != "" {
		t.Fatalf("expected primary cleared when gallery empty, got %q", repo.updated.ImageURL)
	}
}
