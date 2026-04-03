package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestParseLimitQuery_TolerantAndNormalize(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	cfg := pagination.Config{DefaultLimit: 20, MaxLimit: 100}

	cases := []struct {
		name string
		path string
		def  string
		want int
	}{
		{"missing uses default string", "/", "20", 20},
		{"invalid uses normalized default", "/?limit=abc", "20", 20},
		{"negative uses default", "/?limit=-3", "20", 20},
		{"explicit ok", "/?limit=15", "20", 15},
		{"clamped to max", "/?limit=500", "20", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", tc.path, nil)
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = r
			got := ParseLimitQuery(ctx, "limit", tc.def, cfg)
			if got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestParseAfterUUIDQuery(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = r
		id, ok := ParseAfterUUIDQuery(ctx)
		if !ok || id != nil {
			t.Fatalf("expected nil, ok=true")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/?after=not-a-uuid", nil)
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = r
		id, ok := ParseAfterUUIDQuery(ctx)
		if ok || id != nil || w.Code != 400 {
			t.Fatalf("expected 400 and ok=false, code=%d", w.Code)
		}
	})

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		u := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/?after="+u.String(), nil)
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = r
		id, ok := ParseAfterUUIDQuery(ctx)
		if !ok || id == nil || *id != u {
			t.Fatalf("unexpected result")
		}
	})
}
