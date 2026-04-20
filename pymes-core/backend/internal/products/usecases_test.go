package products

import (
	"context"
	"testing"

	"github.com/google/uuid"

	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
)

type captureProductRepo struct {
	created productdomain.Product
}

func (r *captureProductRepo) List(context.Context, ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}
func (r *captureProductRepo) Create(_ context.Context, in productdomain.Product) (productdomain.Product, error) {
	r.created = in
	return in, nil
}
func (r *captureProductRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (productdomain.Product, error) {
	return productdomain.Product{}, nil
}
func (r *captureProductRepo) Update(context.Context, productdomain.Product) (productdomain.Product, error) {
	return productdomain.Product{}, nil
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
