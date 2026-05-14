package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps a SQLite connection with LocalCloud-specific operations.
type DB struct {
	conn *sql.DB
}

// Open opens or creates a SQLite database at path and runs migrations.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("storage: open %s: %w", path, err)
	}

	// Verify connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("storage: ping %s: %w", path, err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("storage: migrate: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB for advanced queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) migrate() error {
	for i, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	return nil
}

var migrations = []string{
	// 0: config_snapshots
	`CREATE TABLE IF NOT EXISTS config_snapshots (
		id TEXT PRIMARY KEY,
		created_at_ms INTEGER NOT NULL,
		hash TEXT NOT NULL UNIQUE,
		config_json TEXT NOT NULL,
		validation_json TEXT NOT NULL DEFAULT '{}'
	);`,

	// 1: scenarios
	`CREATE TABLE IF NOT EXISTS scenarios (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK (status IN ('recording', 'completed', 'exported', 'failed')),
		started_at_ms INTEGER NOT NULL,
		stopped_at_ms INTEGER,
		event_count INTEGER NOT NULL DEFAULT 0,
		replayable_count INTEGER NOT NULL DEFAULT 0,
		root_event_ids_json TEXT NOT NULL DEFAULT '[]',
		tags_json TEXT NOT NULL DEFAULT '[]',
		config_snapshot_id TEXT NOT NULL,
		redaction_summary_json TEXT NOT NULL DEFAULT '{}',
		created_by TEXT NOT NULL DEFAULT 'cli',
		error_message TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (config_snapshot_id) REFERENCES config_snapshots(id)
	);`,

	// 2: replay_runs
	`CREATE TABLE IF NOT EXISTS replay_runs (
		id TEXT PRIMARY KEY,
		scenario_id TEXT NOT NULL,
		started_at_ms INTEGER NOT NULL,
		finished_at_ms INTEGER,
		status TEXT NOT NULL CHECK (status IN ('running', 'passed', 'failed', 'partial', 'canceled')),
		target_base_url TEXT NOT NULL,
		request_count INTEGER NOT NULL DEFAULT 0,
		passed_count INTEGER NOT NULL DEFAULT 0,
		failed_count INTEGER NOT NULL DEFAULT 0,
		diff_summary_json TEXT NOT NULL DEFAULT '{}',
		created_by TEXT NOT NULL DEFAULT 'cli',
		error_message TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (scenario_id) REFERENCES scenarios(id)
	);`,

	// 3: events
	`CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		run_id TEXT NOT NULL,
		scenario_id TEXT,
		replay_run_id TEXT,
		timestamp_ms INTEGER NOT NULL,
		source TEXT NOT NULL,
		service TEXT NOT NULL,
		action TEXT NOT NULL,
		summary TEXT NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('ok', 'error', 'warning', 'pending', 'blocked', 'faulted', 'unknown')),
		duration_ms INTEGER,
		correlation_id TEXT,
		parent_event_id TEXT,
		request_json TEXT,
		response_json TEXT,
		metadata_json TEXT NOT NULL DEFAULT '{}',
		raw_payload_json TEXT,
		faults_json TEXT NOT NULL DEFAULT '[]',
		created_at_ms INTEGER NOT NULL,
		FOREIGN KEY (scenario_id) REFERENCES scenarios(id),
		FOREIGN KEY (replay_run_id) REFERENCES replay_runs(id),
		FOREIGN KEY (parent_event_id) REFERENCES events(id)
	);`,

	// 4: services
	`CREATE TABLE IF NOT EXISTS services (
		service TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('healthy', 'unhealthy', 'starting', 'stopped', 'unknown')),
		endpoint TEXT NOT NULL DEFAULT '',
		container_id TEXT NOT NULL DEFAULT '',
		last_checked_at_ms INTEGER NOT NULL,
		message TEXT NOT NULL DEFAULT '',
		metadata_json TEXT NOT NULL DEFAULT '{}'
	);`,

	// 5: adapter_status
	`CREATE TABLE IF NOT EXISTS adapter_status (
		adapter TEXT NOT NULL,
		service TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
		status TEXT NOT NULL CHECK (status IN ('running', 'disabled', 'degraded', 'error')),
		last_event_at_ms INTEGER,
		last_error TEXT NOT NULL DEFAULT '',
		event_count INTEGER NOT NULL DEFAULT 0,
		metadata_json TEXT NOT NULL DEFAULT '{}',
		updated_at_ms INTEGER NOT NULL,
		PRIMARY KEY (adapter, service)
	);`,

	// 6: fault_rules
	`CREATE TABLE IF NOT EXISTS fault_rules (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
		kind TEXT NOT NULL,
		scope TEXT NOT NULL CHECK (scope IN ('live', 'replay', 'both')),
		match_json TEXT NOT NULL,
		action_json TEXT NOT NULL,
		safety_json TEXT NOT NULL DEFAULT '{}',
		hit_count INTEGER NOT NULL DEFAULT 0,
		last_applied_at_ms INTEGER,
		created_at_ms INTEGER NOT NULL,
		updated_at_ms INTEGER NOT NULL
	);`,

	// 7: indexes
	`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp_ms DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_events_scenario_timestamp ON events(scenario_id, timestamp_ms ASC);`,
	`CREATE INDEX IF NOT EXISTS idx_events_replay_run_timestamp ON events(replay_run_id, timestamp_ms ASC);`,
	`CREATE INDEX IF NOT EXISTS idx_events_correlation_timestamp ON events(correlation_id, timestamp_ms ASC);`,
	`CREATE INDEX IF NOT EXISTS idx_events_source_timestamp ON events(source, timestamp_ms DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_events_service_timestamp ON events(service, timestamp_ms DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_events_action_timestamp ON events(action, timestamp_ms DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_events_status_timestamp ON events(status, timestamp_ms DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_events_parent ON events(parent_event_id);`,
	`CREATE INDEX IF NOT EXISTS idx_scenarios_status_started ON scenarios(status, started_at_ms DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_replay_runs_scenario_started ON replay_runs(scenario_id, started_at_ms DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_fault_rules_enabled_kind ON fault_rules(enabled, kind);`,
	`CREATE INDEX IF NOT EXISTS idx_adapter_status_status ON adapter_status(status);`,
}
