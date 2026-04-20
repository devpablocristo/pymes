package products

import (
	"reflect"
	"testing"

	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
)

func TestNormalizeProductImageURLs_RecomposesSplitDataURLs(t *testing.T) {
	t.Parallel()

	in := []string{
		"data:image/jpeg;base64",
		"/9j/4AAQSkZJRgABAQAAAQABAAD",
		"data:image/png;base64,AAAA",
	}

	got, err := normalizeProductImageURLs(in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{
		"data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD",
		"data:image/png;base64,AAAA",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized urls:\nwant=%#v\ngot=%#v", want, got)
	}
}

func TestDisplayProductImageURLs_RecomposesPersistedSplitDataURLs(t *testing.T) {
	t.Parallel()

	got := displayProductImageURLs(productdomain.Product{
		ImageURLs: []string{
			"data:image/jpeg;base64",
			"/9j/4AAQSkZJRgABAQAAAQABAAD",
		},
	})

	want := []string{"data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected display urls:\nwant=%#v\ngot=%#v", want, got)
	}
}

func TestDisplayProductImageURLs_RebuildsBareBase64Entries(t *testing.T) {
	t.Parallel()

	got := displayProductImageURLs(productdomain.Product{
		ImageURLs: []string{
			"data:image/jpeg;base64,/9j/AAAA",
			"/9j/BBBB",
			"/9j/CCCC",
		},
	})

	want := []string{
		"data:image/jpeg;base64,/9j/AAAA",
		"data:image/jpeg;base64,/9j/BBBB",
		"data:image/jpeg;base64,/9j/CCCC",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected rebuilt urls:\nwant=%#v\ngot=%#v", want, got)
	}
}
