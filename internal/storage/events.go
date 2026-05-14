package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// EventRepository manages timeline event persistence.
type EventRepository struct {
	db *DB
}

// NewEventRepository creates a repository backed by db.
func NewEventRepository(db *DB) *EventRepository {
	return &EventRepository{db: db}
}

// Insert persists a timeline event.
func (r *EventRepository) Insert(ctx context.Context, e *timeline.TimelineEvent) error {
	reqJSON, _ := json.Marshal(e.Request)
	respJSON, _ := json.Marshal(e.Response)
	metaJSON, _ := json.Marshal(e.Metadata)
	rawJSON, _ := json.Marshal(e.RawPayload)
	faultsJSON, _ := json.Marshal(e.Faults)

	_, err := r.db.conn.ExecContext(ctx, `
		INSERT INTO events (
			id, run_id, scenario_id, replay_run_id, timestamp_ms,
			source, service, action, summary, status,
			duration_ms, correlation_id, parent_event_id,
			request_json, response_json, metadata_json,
			raw_payload_json, faults_json, created_at_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.RunID, nilStr(e.ScenarioID), nilStr(e.ReplayRunID),
		e.Timestamp.UnixMilli(),
		e.Source, e.Service, e.Action, e.Summary, e.Status,
		e.DurationMs, nilStr(e.CorrelationID), nilStr(e.ParentEventID),
		nullJSON(reqJSON), nullJSON(respJSON), string(metaJSON),
		nullJSON(rawJSON), string(faultsJSON),
		time.Now().UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("storage: insert event %s: %w", e.ID, err)
	}
	return nil
}

// ListByTimestamp returns events ordered by newest first.
func (r *EventRepository) ListByTimestamp(ctx context.Context, limit int, cursor int64) ([]timeline.TimelineEvent, error) {
	query := `SELECT id, run_id, scenario_id, replay_run_id, timestamp_ms,
		source, service, action, summary, status,
		duration_ms, correlation_id, parent_event_id,
		request_json, response_json, metadata_json,
		raw_payload_json, faults_json
		FROM events`

	args := []any{}
	if cursor > 0 {
		query += ` WHERE timestamp_ms < ?`
		args = append(args, cursor)
	}
	query += ` ORDER BY timestamp_ms DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list events: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// ListByScenario returns events for a scenario ordered by timestamp ascending.
func (r *EventRepository) ListByScenario(ctx context.Context, scenarioID string) ([]timeline.TimelineEvent, error) {
	rows, err := r.db.conn.QueryContext(ctx, `
		SELECT id, run_id, scenario_id, replay_run_id, timestamp_ms,
			source, service, action, summary, status,
			duration_ms, correlation_id, parent_event_id,
			request_json, response_json, metadata_json,
			raw_payload_json, faults_json
		FROM events WHERE scenario_id = ? ORDER BY timestamp_ms ASC`, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("storage: list events by scenario: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// ListByReplayRun returns events for a replay run.
func (r *EventRepository) ListByReplayRun(ctx context.Context, replayRunID string) ([]timeline.TimelineEvent, error) {
	rows, err := r.db.conn.QueryContext(ctx, `
		SELECT id, run_id, scenario_id, replay_run_id, timestamp_ms,
			source, service, action, summary, status,
			duration_ms, correlation_id, parent_event_id,
			request_json, response_json, metadata_json,
			raw_payload_json, faults_json
		FROM events WHERE replay_run_id = ? ORDER BY timestamp_ms ASC`, replayRunID)
	if err != nil {
		return nil, fmt.Errorf("storage: list events by replay run: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// GetByID returns a single event.
func (r *EventRepository) GetByID(ctx context.Context, id string) (*timeline.TimelineEvent, error) {
	row := r.db.conn.QueryRowContext(ctx, `
		SELECT id, run_id, scenario_id, replay_run_id, timestamp_ms,
			source, service, action, summary, status,
			duration_ms, correlation_id, parent_event_id,
			request_json, response_json, metadata_json,
			raw_payload_json, faults_json
		FROM events WHERE id = ?`, id)
	e, err := scanEvent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: get event %s: %w", id, err)
	}
	return e, nil
}

// Count returns total event count and optionally since a timestamp.
func (r *EventRepository) Count(ctx context.Context, sinceMs int64) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM events`
	args := []any{}
	if sinceMs > 0 {
		query += ` WHERE timestamp_ms >= ?`
		args = append(args, sinceMs)
	}
	err := r.db.conn.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func scanEvents(rows *sql.Rows) ([]timeline.TimelineEvent, error) {
	var events []timeline.TimelineEvent
	for rows.Next() {
		e, err := scanEventRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *e)
	}
	return events, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row *sql.Row) (*timeline.TimelineEvent, error) {
	return scanRow(row)
}

func scanEventRow(rows *sql.Rows) (*timeline.TimelineEvent, error) {
	return scanRow(rows)
}

func scanRow(s rowScanner) (*timeline.TimelineEvent, error) {
	var e timeline.TimelineEvent
	var scenarioID, replayRunID, correlationID, parentEventID sql.NullString
	var durationMs sql.NullInt64
	var timestampMs int64
	var reqJSON, respJSON, metaJSON, rawJSON, faultsJSON sql.NullString

	err := s.Scan(
		&e.ID, &e.RunID, &scenarioID, &replayRunID, &timestampMs,
		&e.Source, &e.Service, &e.Action, &e.Summary, &e.Status,
		&durationMs, &correlationID, &parentEventID,
		&reqJSON, &respJSON, &metaJSON,
		&rawJSON, &faultsJSON,
	)
	if err != nil {
		return nil, err
	}

	e.Timestamp = time.UnixMilli(timestampMs).UTC()
	e.ScenarioID = nullToPtr(scenarioID)
	e.ReplayRunID = nullToPtr(replayRunID)
	e.CorrelationID = nullToPtr(correlationID)
	e.ParentEventID = nullToPtr(parentEventID)
	if durationMs.Valid {
		e.DurationMs = &durationMs.Int64
	}

	if reqJSON.Valid && reqJSON.String != "" && reqJSON.String != "null" {
		var req timeline.RequestData
		if err := json.Unmarshal([]byte(reqJSON.String), &req); err != nil {
			return nil, fmt.Errorf("scan event %s: unmarshal request: %w", e.ID, err)
		}
		e.Request = &req
	}
	if respJSON.Valid && respJSON.String != "" && respJSON.String != "null" {
		var resp timeline.ResponseData
		if err := json.Unmarshal([]byte(respJSON.String), &resp); err != nil {
			return nil, fmt.Errorf("scan event %s: unmarshal response: %w", e.ID, err)
		}
		e.Response = &resp
	}
	if metaJSON.Valid && metaJSON.String != "" && metaJSON.String != "{}" {
		if err := json.Unmarshal([]byte(metaJSON.String), &e.Metadata); err != nil {
			return nil, fmt.Errorf("scan event %s: unmarshal metadata: %w", e.ID, err)
		}
	}
	if rawJSON.Valid && rawJSON.String != "" && rawJSON.String != "null" {
		var raw timeline.RawPayload
		if err := json.Unmarshal([]byte(rawJSON.String), &raw); err != nil {
			return nil, fmt.Errorf("scan event %s: unmarshal raw payload: %w", e.ID, err)
		}
		e.RawPayload = &raw
	}
	if faultsJSON.Valid && faultsJSON.String != "" && faultsJSON.String != "[]" {
		if err := json.Unmarshal([]byte(faultsJSON.String), &e.Faults); err != nil {
			return nil, fmt.Errorf("scan event %s: unmarshal faults: %w", e.ID, err)
		}
	}

	return &e, nil
}

func nilStr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func nullToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func nullJSON(data []byte) any {
	s := string(data)
	if s == "null" || s == "" {
		return nil
	}
	return s
}
