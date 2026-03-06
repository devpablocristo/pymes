package pagination

import "github.com/google/uuid"

type Params struct {
	Limit int        `json:"limit"`
	After *uuid.UUID `json:"after,omitempty"`
}

type Result[T any] struct {
	Items      []T    `json:"items"`
	Total      int64  `json:"total"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

func NormalizeLimit(limit, def, max int) int {
	if def <= 0 {
		def = 20
	}
	if max <= 0 {
		max = 100
	}
	if limit <= 0 {
		return def
	}
	if limit > max {
		return max
	}
	return limit
}

func NextCursorFromUUID(next *uuid.UUID) string {
	if next == nil || *next == uuid.Nil {
		return ""
	}
	return next.String()
}
