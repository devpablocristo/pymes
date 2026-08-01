// Package helpers validates and serializes worker release-readiness metadata.
package helpers

import (
	"errors"
	"log/slog"
	"regexp"

	releasesignalmodels "github.com/devpablocristo/pymes/v3/backend/internal/worker/release_signal/models"
)

var (
	releaseSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
	revision   = regexp.MustCompile(`^[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
)

func Validate(metadata releasesignalmodels.Metadata) error {
	if !releaseSHA.MatchString(metadata.ReleaseSHA) {
		return errors.New("release SHA must contain exactly 40 lowercase hexadecimal characters")
	}
	if !revision.MatchString(metadata.Revision) {
		return errors.New("revision must contain a valid Cloud Run revision name")
	}
	return nil
}

func ReadyAttributes(metadata releasesignalmodels.Metadata) []slog.Attr {
	return []slog.Attr{
		slog.String("event", "worker_release_ready"),
		slog.Bool("ready", true),
		slog.String("release_sha", metadata.ReleaseSHA),
		slog.String("revision", metadata.Revision),
	}
}
