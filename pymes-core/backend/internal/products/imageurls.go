package products

import (
	"fmt"
	"strings"

	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

const (
	maxProductImages             = 20
	maxProductHTTPImageURLLen    = 2048
	maxProductDataURLImageURLLen = 3 * 1024 * 1024 // data:image/* desde cliente (FileReader); URLs http(s) siguen acotadas
	metadataImageURLsKey         = "image_urls"
)

// maxLenForProductImageURL limita enlaces remotos cortos y permite data URLs más largas que el header legacy de 2048.
func maxLenForProductImageURL(u string) int {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(u)), "data:image/") {
		return maxProductDataURLImageURLLen
	}
	return maxProductHTTPImageURLLen
}

// parseImageURLsFromMetadata devuelve las URLs bajo metadata.image_urls si la clave existe.
func parseImageURLsFromMetadata(meta map[string]any) (urls []string, keyPresent bool) {
	if meta == nil {
		return nil, false
	}
	raw, ok := meta[metadataImageURLsKey]
	if !ok {
		return nil, false
	}
	keyPresent = true
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...), true
	case []any:
		out := make([]string, 0, len(v))
		for _, it := range v {
			s, ok := it.(string)
			if !ok {
				continue
			}
			out = append(out, s)
		}
		return out, true
	case nil:
		return nil, true
	default:
		return nil, true
	}
}

// mergeProductMetadataImageURLs copia metadata y fija image_urls al slice canónico (vacío permitido).
func mergeProductMetadataImageURLs(meta map[string]any, urls []string) map[string]any {
	out := make(map[string]any, len(meta)+1)
	for k, v := range meta {
		out[k] = v
	}
	cp := append([]string(nil), urls...)
	out[metadataImageURLsKey] = cp
	return out
}

// urlsForMetadataSync lista de URLs para persistir en metadata alineada a columnas image_urls / image_url legacy.
func urlsForMetadataSync(p productdomain.Product) []string {
	if len(p.ImageURLs) > 0 {
		return append([]string(nil), p.ImageURLs...)
	}
	u := strings.TrimSpace(p.ImageURL)
	if u != "" {
		return []string{u}
	}
	return []string{}
}

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
		if len(u) > maxLenForProductImageURL(u) {
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
