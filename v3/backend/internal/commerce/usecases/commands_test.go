package usecases

import (
	"context"
	"errors"
	"testing"
)

type commandStoreStub struct {
	CommandStore
	ping func(context.Context) error
}

func (s commandStoreStub) Ping(ctx context.Context) error { return s.ping(ctx) }

func TestCommandsReadinessChecksDatabasePort(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("database unavailable")
	commands := Commands{Store: commandStoreStub{ping: func(context.Context) error { return sentinel }}}
	if err := commands.Ready(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("expected database error, got %v", err)
	}
	commands.Store = commandStoreStub{ping: func(context.Context) error { return nil }}
	if err := commands.Ready(context.Background()); err != nil {
		t.Fatalf("expected ready store, got %v", err)
	}
}

func TestCommandsReadinessRejectsMissingStore(t *testing.T) {
	t.Parallel()
	if err := (Commands{}).Ready(context.Background()); err == nil {
		t.Fatal("expected missing store to be not ready")
	}
}
