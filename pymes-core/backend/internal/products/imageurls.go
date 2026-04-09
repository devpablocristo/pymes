package products

import (
	"fmt"
	"strings"

	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

const (
	maxProductImages      = 20
	maxProductImageURLLen = 2048
)

// normalizeProductImageURLs recorta, elimina vacíos y duplicados conservando orden.
func normalizeProductImageURLs(urls []string) ([]string, error) {
	if len(urls) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		if len(u) > maxProductImageURLLen {
			return nil, fmt.Errorf("image url too long: %w", httperrors.ErrBadInput)
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
		if len(out) > maxProductImages {
			return nil, fmt.Errorf("too many image urls (max %d): %w", maxProductImages, httperrors.ErrBadInput)
		}
	}
	return out, nil
}

// displayProductImageURLs prioriza image_urls persistidos; si está vacío, usa image_url legacy.
func displayProductImageURLs(p productdomain.Product) []string {
	out := make([]string, 0, len(p.ImageURLs)+1)
	seen := make(map[string]struct{}, len(p.ImageURLs)+1)
	for _, raw := range p.ImageURLs {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	if len(out) > 0 {
		return out
	}
	u := strings.TrimSpace(p.ImageURL)
	if u == "" {
		return nil
	}
	return []string{u}
}
