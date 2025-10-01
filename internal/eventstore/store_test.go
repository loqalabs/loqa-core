package eventstore

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/loqalabs/loqa-core/internal/config"
)

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestOpenEphemeral(t *testing.T) {
	ctx := context.Background()
	cfg := config.EventStoreConfig{RetentionMode: "ephemeral"}
	es, err := Open(ctx, cfg, newLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = es.Close() })
	if err := es.Ensure(); err != nil {
		t.Fatalf("ensure failed: %v", err)
	}
}

func TestAppendAndQuery(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.EventStoreConfig{Path: filepath.Join(tmp, "events.db"), RetentionMode: "session"}
	es, err := Open(context.Background(), cfg, newLogger())
	if err != nil {
		t.Fatalf("open event store: %v", err)
	}
	t.Cleanup(func() { _ = es.Close() })

	sessionID := "session-123"
	if err := es.AppendSession(context.Background(), sessionID, "actor-1", "session"); err != nil {
		t.Fatalf("append session: %v", err)
	}
	if err := es.AppendEvent(context.Background(), Event{SessionID: sessionID, Type: "test", Payload: []byte("hello")}); err != nil {
		t.Fatalf("append event: %v", err)
	}
	events, err := es.ListSessionEvents(context.Background(), sessionID, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if string(events[0].Payload) != "hello" {
		t.Fatalf("unexpected payload: %s", events[0].Payload)
	}
}

func TestPruneByDaysAndSessions(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.EventStoreConfig{Path: filepath.Join(tmp, "events.db"), RetentionMode: "persistent", RetentionDays: 1, MaxSessions: 1}
	es, err := Open(context.Background(), cfg, newLogger())
	if err != nil {
		t.Fatalf("open event store: %v", err)
	}
	t.Cleanup(func() { _ = es.Close() })

	es.clock = func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }
	if err := es.AppendSession(context.Background(), "old-session", "actor", "session"); err != nil {
		t.Fatalf("append session: %v", err)
	}
	if err := es.AppendEvent(context.Background(), Event{SessionID: "old-session", Type: "note"}); err != nil {
		t.Fatalf("append event: %v", err)
	}

	es.clock = func() time.Time { return time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC) }
	if err := es.AppendSession(context.Background(), "new-session", "actor", "session"); err != nil {
		t.Fatalf("append session: %v", err)
	}
	if err := es.Prune(context.Background()); err != nil {
		t.Fatalf("prune: %v", err)
	}

	events, err := es.ListSessionEvents(context.Background(), "old-session", 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected old session pruned")
	}
}
