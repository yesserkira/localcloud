package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := testDB(t)
	// Verify tables exist
	var name string
	err := db.conn.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='events'`).Scan(&name)
	if err != nil {
		t.Fatalf("events table missing: %v", err)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second open should succeed: %v", err)
	}
	db2.Close()
}

func TestEventInsertAndGet(t *testing.T) {
	db := testDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	e := &timeline.TimelineEvent{
		ID:        "evt_001",
		RunID:     "run_001",
		Timestamp: time.Now().UTC(),
		Source:    timeline.SourceHTTPProxy,
		Service:   "api",
		Action:    timeline.ActionHTTPRequest,
		Summary:   "POST /signup",
		Status:    timeline.StatusOK,
	}

	if err := repo.Insert(ctx, e); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := repo.GetByID(ctx, "evt_001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected event, got nil")
	}
	if got.Service != "api" {
		t.Fatalf("expected service=api, got %s", got.Service)
	}
}

func TestEventListByTimestamp(t *testing.T) {
	db := testDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		e := &timeline.TimelineEvent{
			ID:        "evt_" + string(rune('A'+i)),
			RunID:     "run_001",
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Source:    timeline.SourceHTTPProxy,
			Service:   "api",
			Action:    timeline.ActionHTTPRequest,
			Summary:   "test",
			Status:    timeline.StatusOK,
		}
		if err := repo.Insert(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	events, err := repo.ListByTimestamp(ctx, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	// Should be newest first
	if events[0].Timestamp.Before(events[1].Timestamp) {
		t.Fatal("events not in descending order")
	}
}

func TestEventCount(t *testing.T) {
	db := testDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	e := &timeline.TimelineEvent{
		ID: "evt_001", RunID: "run_001",
		Timestamp: time.Now().UTC(),
		Source: timeline.SourceHTTPProxy, Service: "api",
		Action: timeline.ActionHTTPRequest, Summary: "test",
		Status: timeline.StatusOK,
	}
	repo.Insert(ctx, e)

	count, err := repo.Count(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

func TestScenarioInsertAndGet(t *testing.T) {
	db := testDB(t)
	snapRepo := NewConfigSnapshotRepository(db)
	scenRepo := NewScenarioRepository(db)
	ctx := context.Background()

	snap := &timeline.ConfigSnapshot{
		ID: "snap_001", CreatedAt: time.Now().UTC(),
		Hash: "sha256:test", ConfigJSON: "{}", ValidationJSON: "{}",
	}
	if err := snapRepo.Insert(ctx, snap); err != nil {
		t.Fatal(err)
	}

	s := &timeline.Scenario{
		ID: "scn_001", Name: "test-scenario",
		Status: timeline.ScenarioStatusRecording,
		StartedAt: time.Now().UTC(),
		ConfigSnapshotID: "snap_001",
		Tags: []string{"demo"},
		RootEventIDs: []string{},
		CreatedBy: "cli",
	}
	if err := scenRepo.Insert(ctx, s); err != nil {
		t.Fatal(err)
	}

	got, err := scenRepo.GetByName(ctx, "test-scenario")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Status != "recording" {
		t.Fatalf("unexpected: %+v", got)
	}

	active, err := scenRepo.GetActiveRecording(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active == nil {
		t.Fatal("expected active recording")
	}
}

func TestForeignKeyConstraint(t *testing.T) {
	db := testDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	e := &timeline.TimelineEvent{
		ID: "evt_fk", RunID: "run_001",
		Timestamp: time.Now().UTC(),
		Source: timeline.SourceHTTPProxy, Service: "api",
		Action: timeline.ActionHTTPRequest, Summary: "test",
		Status: timeline.StatusOK,
	}
	badScenarioID := "nonexistent_scenario"
	e.ScenarioID = &badScenarioID

	err := repo.Insert(ctx, e)
	if err == nil {
		t.Fatal("expected foreign key error")
	}
}

func TestFaultRuleInsertAndList(t *testing.T) {
	db := testDB(t)
	repo := NewFaultRuleRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	rule := &timeline.FaultRule{
		ID: "flt_001", Name: "test-fault",
		Enabled: false, Kind: timeline.FaultKindDelayResponse,
		Scope: timeline.FaultScopeReplay,
		Match: timeline.FaultMatch{Service: "api", Method: "POST", Path: "/signup"},
		Action: timeline.FaultAction{DelayMs: 1500},
		Safety: timeline.FaultSafety{MaxHits: 5},
		CreatedAt: now, UpdatedAt: now,
	}

	if err := repo.Insert(ctx, rule); err != nil {
		t.Fatal(err)
	}

	rules, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "test-fault" {
		t.Fatalf("unexpected name: %s", rules[0].Name)
	}
}

func TestOpenBadPath(t *testing.T) {
	_, err := Open(filepath.Join(os.DevNull, "impossible", "path.db"))
	if err == nil {
		t.Fatal("expected error for bad path")
	}
}
