package agent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/localcloud-dev/localcloud/internal/config"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	cfg, err := config.Parse([]byte(`
version: 1
project:
  name: agent-test
agent:
  bind: 127.0.0.1
  port: 41777
  studioPort: 0
  database: ` + dbPath + `
`))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestAgentStartStop(t *testing.T) {
	cfg := testConfig(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := New(cfg, "0.1.0-test", logger)

	if a.Status() != StatusStopped {
		t.Fatalf("expected stopped, got %s", a.Status())
	}
	if a.RunID() == "" {
		t.Fatal("expected run ID")
	}

	ctx := context.Background()
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if a.Status() != StatusRunning {
		t.Fatalf("expected running, got %s", a.Status())
	}

	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := a.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}

	if a.Status() != StatusStopped {
		t.Fatalf("expected stopped, got %s", a.Status())
	}
}

func TestAgentSinkEmit(t *testing.T) {
	cfg := testConfig(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := New(cfg, "0.1.0-test", logger)

	ctx := context.Background()
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer a.Stop(ctx)

	event := timeline.TimelineEvent{
		ID:        "evt_test123",
		RunID:     a.RunID(),
		Timestamp: time.Now().UTC(),
		Source:    timeline.SourceHTTPProxy,
		Service:  "api",
		Action:   timeline.ActionHTTPRequest,
		Summary:  "POST /signup",
		Status:   timeline.StatusOK,
	}

	if err := a.sink.Emit(ctx, event); err != nil {
		t.Fatal(err)
	}

	// Verify event was stored
	count, err := a.sink.events.Count(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 event, got %d", count)
	}
}

func TestAgentSinkRejectsInvalid(t *testing.T) {
	cfg := testConfig(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := New(cfg, "0.1.0-test", logger)

	ctx := context.Background()
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer a.Stop(ctx)

	event := timeline.TimelineEvent{} // missing all fields

	if err := a.sink.Emit(ctx, event); err == nil {
		t.Fatal("expected validation error")
	}
}
