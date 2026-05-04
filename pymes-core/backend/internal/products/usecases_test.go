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

type roundtripProductRepo struct {
	stored productdomain.Product
}

func (r *roundtripProductRepo) List(context.Context, ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}
func (r *roundtripProductRepo) Create(context.Context, productdomain.Product) (productdomain.Product, error) {
	return productdomain.Product{}, nil
}
func (r *roundtripProductRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (productdomain.Product, error) {
	return r.stored, nil
}
func (r *roundtripProductRepo) Update(_ context.Context, in productdomain.Product) (productdomain.Product, error) {
	r.stored = in
	return in, nil
}
func (r *roundtripProductRepo) Archive(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *roundtripProductRepo) Restore(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *roundtripProductRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error  { return nil }

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

func TestCreateReadsImageURLsFromMetadataWhenSliceEmpty(t *testing.T) {
	t.Parallel()

	repo := &captureProductRepo{}
	uc := NewUsecases(repo, nil, nil)

	_, err := uc.Create(context.Background(), productdomain.Product{
		OrgID: uuid.New(),
		Name:  "Desde metadata",
		Metadata: map[string]any{
			"image_urls": []any{"https://cdn.example.com/a.png", "https://cdn.example.com/b.png"},
		},
	}, "tester")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.created.ImageURLs) != 2 {
		t.Fatalf("expected 2 image_urls, got %#v", repo.created.ImageURLs)
	}
	got, ok := repo.created.Metadata["image_urls"].([]string)
	if !ok || len(got) != 2 {
		t.Fatalf("expected metadata.image_urls synced, got %#v ok=%v", repo.created.Metadata["image_urls"], ok)
	}
}

func TestUpdateAppliesMetadataImageURLsWhenTopLevelOmitted(t *testing.T) {
	t.Parallel()

	org := uuid.New()
	id := uuid.New()
	repo := &roundtripProductRepo{
		stored: productdomain.Product{
			ID:       id,
			OrgID:    org,
			Name:     "Producto",
			Metadata: map[string]any{},
		},
	}
	uc := NewUsecases(repo, nil, nil)

	meta := map[string]any{"image_urls": []string{"https://x.example/p.png"}}
	_, err := uc.Update(context.Background(), org, id, UpdateInput{
		Metadata: &meta,
	}, "tester")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.stored.ImageURLs) != 1 || repo.stored.ImageURLs[0] != "https://x.example/p.png" {
		t.Fatalf("expected column image_urls from metadata, got %#v", repo.stored.ImageURLs)
	}
	if repo.stored.ImageURL != "https://x.example/p.png" {
		t.Fatalf("expected primary image_url set, got %q", repo.stored.ImageURL)
	}
}
