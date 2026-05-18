package status_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/devpablocristo/core/concurrency/go/fsm"
	"github.com/devpablocristo/core/errors/go/domainerr"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/status"
)

func TestMapFSMError(t *testing.T) {
	t.Parallel()

	customErr := errors.New("boom")

	cases := []struct {
		name       string
		current    string
		next       string
		err        error
		wantNil    bool
		wantKind   domainerr.Kind
		wantSubstr string // verifica que el mensaje formatee bien
	}{
		{
			name:    "nil error passes through",
			err:     nil,
			wantNil: true,
		},
		{
			name:       "ErrInvalidTransition wraps as Conflict",
			current:    "draft",
			next:       "paid",
			err:        fsm.ErrInvalidTransition,
			wantKind:   domainerr.KindConflict,
			wantSubstr: "draft -> paid",
		},
		{
			name:       "ErrTerminal wraps as Conflict",
			current:    "voided",
			next:       "draft",
			err:        fsm.ErrTerminal,
			wantKind:   domainerr.KindConflict,
			wantSubstr: `"voided" is terminal`,
		},
		{
			name:       "wrapped ErrInvalidTransition still detected",
			current:    "a",
			next:       "b",
			err:        errors.Join(errors.New("ctx"), fsm.ErrInvalidTransition),
			wantKind:   domainerr.KindConflict,
			wantSubstr: "a -> b",
		},
		{
			name: "unknown error passes through (wrapped)",
			err:  customErr,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := status.MapFSMError(tc.current, tc.next, tc.err)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil error")
			}
			if tc.wantKind != "" {
				if !domainerr.IsKind(got, tc.wantKind) {
					t.Fatalf("expected kind %q, got error %v", tc.wantKind, got)
				}
				if tc.wantSubstr != "" && !strings.Contains(got.Error(), tc.wantSubstr) {
					t.Fatalf("expected message to contain %q, got %q", tc.wantSubstr, got.Error())
				}
				return
			}
			// caso passthrough: el error original debe ser detectable.
			if !errors.Is(got, customErr) {
				t.Fatalf("expected to wrap original error, got %v", got)
			}
		})
	}
}
