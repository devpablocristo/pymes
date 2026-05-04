package products

import (
	"strings"
	"testing"
)

func TestNormalizeProductImageURLs_HTTPRejectedOver2048(t *testing.T) {
	t.Parallel()

	long := "https://x.example/" + strings.Repeat("a", 2100)
	_, err := normalizeProductImageURLs([]string{long})
	if err == nil {
		t.Fatal("expected error for long http url")
	}
}

func TestNormalizeProductImageURLs_DataURLAllowedWithinCap(t *testing.T) {
	t.Parallel()

	prefix := "data:image/png;base64,"
	body := strings.Repeat("A", 500_000)
	u := prefix + body
	got, err := normalizeProductImageURLs([]string{u})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 1 || got[0] != u {
		t.Fatalf("unexpected got: %#v", got)
	}
}

func TestNormalizeProductImageURLs_DataURLRejectedOverCap(t *testing.T) {
	t.Parallel()

	prefix := "data:image/png;base64,"
	body := strings.Repeat("A", 4*1024*1024)
	u := prefix + body
	_, err := normalizeProductImageURLs([]string{u})
	if err == nil {
		t.Fatal("expected error for oversized data url")
	}
}
