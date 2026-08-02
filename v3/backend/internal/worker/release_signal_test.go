package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestReleaseSignalEmitsStableReadyEvent(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	signal, err := NewReleaseSignal(
		slog.New(slog.NewJSONHandler(&output, nil)),
		"0123456789abcdef0123456789abcdef01234567",
		"pymes-v3-stg-worker-00042-abc",
	)
	if err != nil {
		t.Fatal(err)
	}

	signal.SignalReady(context.Background())

	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("decode readiness log: %v\n%s", err, output.String())
	}
	if event["event"] != "worker_release_ready" ||
		event["ready"] != true ||
		event["release_sha"] != "0123456789abcdef0123456789abcdef01234567" ||
		event["revision"] != "pymes-v3-stg-worker-00042-abc" {
		t.Fatalf("readiness event = %#v", event)
	}
}

func TestReleaseSignalRejectsInvalidMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		releaseSHA string
		revision   string
	}{
		{
			name:       "short release SHA",
			releaseSHA: "abc",
			revision:   "pymes-worker-00001-abc",
		},
		{
			name:       "uppercase release SHA",
			releaseSHA: "0123456789ABCDEF0123456789abcdef01234567",
			revision:   "pymes-worker-00001-abc",
		},
		{
			name:       "unsafe revision",
			releaseSHA: "0123456789abcdef0123456789abcdef01234567",
			revision:   "pymes/worker",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewReleaseSignal(
				slog.Default(),
				test.releaseSHA,
				test.revision,
			); err == nil {
				t.Fatal("expected invalid release metadata")
			}
		})
	}
}
