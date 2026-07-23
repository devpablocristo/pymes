package provisioning

import (
	"errors"
	"strings"
	"testing"
)

func TestPrepareCanonicalizesProvisioningInput(t *testing.T) {
	prepared, err := prepare(Input{
		Name:       "  Acme Argentina  ",
		Slug:       "  ACME-AR  ",
		OwnerEmail: "  OWNER@EXAMPLE.COM  ",
	})
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}

	if prepared.Name != "Acme Argentina" {
		t.Fatalf("name = %q", prepared.Name)
	}
	if prepared.Slug != "acme-ar" {
		t.Fatalf("slug = %q", prepared.Slug)
	}
	if prepared.OwnerEmail != "owner@example.com" {
		t.Fatalf("owner email = %q", prepared.OwnerEmail)
	}
	if len(prepared.PayloadHash) != 64 {
		t.Fatalf("payload hash length = %d, want 64", len(prepared.PayloadHash))
	}

	repeated, err := prepare(Input{
		Name:       "Acme Argentina",
		Slug:       "acme-ar",
		OwnerEmail: "owner@example.com",
	})
	if err != nil {
		t.Fatalf("prepare(repeated) error = %v", err)
	}
	if repeated != prepared {
		t.Fatalf("canonical payload changed: got %#v want %#v", repeated, prepared)
	}
}

func TestPrepareRejectsInvalidProvisioningInput(t *testing.T) {
	tests := []struct {
		name  string
		input Input
	}{
		{
			name:  "missing name",
			input: Input{Slug: "acme", OwnerEmail: "owner@example.com"},
		},
		{
			name:  "invalid slug",
			input: Input{Name: "Acme", Slug: "acme_argentina", OwnerEmail: "owner@example.com"},
		},
		{
			name:  "long slug",
			input: Input{Name: "Acme", Slug: strings.Repeat("a", 64), OwnerEmail: "owner@example.com"},
		},
		{
			name:  "invalid email",
			input: Input{Name: "Acme", Slug: "acme", OwnerEmail: "not-an-email"},
		},
		{
			name:  "display name email",
			input: Input{Name: "Acme", Slug: "acme", OwnerEmail: "Owner <owner@example.com>"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := prepare(test.input)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}
