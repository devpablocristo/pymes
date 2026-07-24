package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestRunRequiresDatabaseURL(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		context.Background(),
		[]string{"--name", "Acme", "--slug", "acme", "--owner-email", "owner@example.com"},
		&stdout,
		&stderr,
		func(string) string { return "" },
	)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}

	var response struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &response); err != nil {
		t.Fatalf("decode stderr: %v", err)
	}
	if response.Code != "IAM_PROVISION_DATABASE_REQUIRED" {
		t.Fatalf("error code = %q", response.Code)
	}
}
