// Package worker contains the release-readiness logging adapter.
// architecture:adapter external
package worker

import (
	"context"
	"log/slog"

	releasesignalhelpers "github.com/devpablocristo/pymes/v3/backend/internal/worker/release_signal/helpers"
	releasesignalmodels "github.com/devpablocristo/pymes/v3/backend/internal/worker/release_signal/models"
)

type ReleaseSignal struct {
	logger   *slog.Logger
	metadata releasesignalmodels.Metadata
}

var _ ReleaseReadySignal = (*ReleaseSignal)(nil)

func NewReleaseSignal(
	logger *slog.Logger,
	releaseSHA string,
	revision string,
) (*ReleaseSignal, error) {
	metadata := releasesignalmodels.Metadata{
		ReleaseSHA: releaseSHA,
		Revision:   revision,
	}
	if err := releasesignalhelpers.Validate(metadata); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ReleaseSignal{logger: logger, metadata: metadata}, nil
}

func (signal *ReleaseSignal) SignalReady(ctx context.Context) {
	signal.logger.LogAttrs(
		ctx,
		slog.LevelInfo,
		"worker release ready",
		releasesignalhelpers.ReadyAttributes(signal.metadata)...,
	)
}
