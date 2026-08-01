package main

import (
	"strings"
	"testing"
)

const validRerunGuard = `set -euo pipefail
[[ "${GITHUB_RUN_ATTEMPT}" == "1" ]] || {
  echo "dispatch a new release workflow" >&2
  exit 1
}`

func TestValidateDeployRerunGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		steps   []any
		wantErr string
	}{
		{
			name: "first step accepted",
			steps: []any{
				object{"name": "Reject standalone deploy reruns", "run": validRerunGuard},
				object{"name": "Checkout exact Pymes source"},
			},
		},
		{
			name: "checkout before guard rejected",
			steps: []any{
				object{"name": "Checkout exact Pymes source"},
				object{"name": "Reject standalone deploy reruns", "run": validRerunGuard},
			},
			wantErr: "must be the first job step",
		},
		{
			name: "missing attempt check rejected",
			steps: []any{
				object{
					"name": "Reject standalone deploy reruns",
					"run":  `echo "dispatch a new release workflow"`,
				},
			},
			wantErr: "does not reject GITHUB_RUN_ATTEMPT",
		},
		{
			name: "missing fresh dispatch rejected",
			steps: []any{
				object{
					"name": "Reject standalone deploy reruns",
					"run":  `[[ "${GITHUB_RUN_ATTEMPT}" == "1" ]]`,
				},
			},
			wantErr: "does not require a fresh release dispatch",
		},
		{
			name: "duplicate guard rejected",
			steps: []any{
				object{"name": "Reject standalone deploy reruns", "run": validRerunGuard},
				object{"name": "Reject standalone deploy reruns", "run": validRerunGuard},
			},
			wantErr: "expected exactly one step",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateRerunGuard(
				object{"steps": test.steps},
				"Reject standalone deploy reruns",
				true,
			)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateRerunGuard() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validateRerunGuard() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}
