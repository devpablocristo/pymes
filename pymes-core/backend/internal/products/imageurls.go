package products

import (
	"fmt"
	"strings"

	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

const (
	maxProductImages      = 20
	maxProductImageURLLen = 5 * 1024 * 1024
)

func inferProductImageDataPrefix(raw string) string {
	trimmed := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(trimmed, "/9j/"):
		return "data:image/jpeg;base64,"
	case strings.HasPrefix(trimmed, "iVBOR"):
		return "data:image/png;base64,"
	case strings.HasPrefix(trimmed, "R0lGOD"):
		return "data:image/gif;base64,"
	case strings.HasPrefix(trimmed, "UklGR"):
		return "data:image/webp;base64,"
	default:
		return ""
	}
}

func looksLikeProductImageBase64(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < 8 || inferProductImageDataPrefix(trimmed) == "" {
		return false
	}
	for _, r := range trimmed {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '+', r == '/', r == '=':
		default:
			return false
		}
	}
	return true
}

func normalizedProductImageURLCandidates(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	out := make([]string, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))
	lastDataPrefix := ""
	for index := 0; index < len(urls); index++ {
		current := strings.TrimSpace(urls[index])
		if current == "" {
			continue
		}
		if strings.HasPrefix(current, "data:image/") && !strings.Contains(current, ",") && index+1 < len(urls) {
			next := strings.TrimSpace(urls[index+1])
			if next != "" {
				current = current + "," + next
				index++
			}
		}
		if strings.HasPrefix(current, "data:image/") {
			if cut := strings.Index(current, ","); cut > 0 {
				lastDataPrefix = current[:cut+1]
			}
		} else if looksLikeProductImageBase64(current) {
			prefix := lastDataPrefix
			if prefix == "" {
				prefix = inferProductImageDataPrefix(current)
			}
			if prefix != "" {
				current = prefix + current
			}
		}
		if _, ok := seen[current]; ok {
			continue
		}
		seen[current] = struct{}{}
		out = append(out, current)
	}
	return out
}

// normalizeProductImageURLs recorta, recompone data URLs partidas, elimina vacíos y duplicados conservando orden.
func normalizeProductImageURLs(urls []string) ([]string, error) {
	candidates := normalizedProductImageURLCandidates(urls)
	if len(candidates) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if len(candidate) > maxProductImageURLLen {
			return nil, fmt.Errorf("image url too long: %w", httperrors.ErrBadInput)
		}
		out = append(out, candidate)
		if len(out) > maxProductImages {
			return nil, fmt.Errorf("too many image urls (max %d): %w", maxProductImages, httperrors.ErrBadInput)
		}
	}
	return out, nil
}

// displayProductImageURLs prioriza image_urls persistidos; si está vacío, usa image_url legacy.
func displayProductImageURLs(p productdomain.Product) []string {
	out := normalizedProductImageURLCandidates(p.ImageURLs)
	if len(out) > 0 {
		return out
	}
	u := strings.TrimSpace(p.ImageURL)
	if u == "" {
		return nil
	}
	return []string{u}
}
