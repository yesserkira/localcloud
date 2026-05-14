package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// ScenarioRepository manages scenario persistence.
type ScenarioRepository struct {
	db *DB
}

func NewScenarioRepository(db *DB) *ScenarioRepository {
	return &ScenarioRepository{db: db}
}

func (r *ScenarioRepository) Insert(ctx context.Context, s *timeline.Scenario) error {
	rootIDs, _ := json.Marshal(s.RootEventIDs)
	tags, _ := json.Marshal(s.Tags)
	redaction, _ := json.Marshal(s.RedactionSummary)

	_, err := r.db.conn.ExecContext(ctx, `
		INSERT INTO scenarios (
			id, name, description, status, started_at_ms, stopped_at_ms,
			event_count, replayable_count, root_event_ids_json, tags_json,
			config_snapshot_id, redaction_summary_json, created_by, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Description, s.Status,
		s.StartedAt.UnixMilli(), nilTime(s.StoppedAt),
		s.EventCount, s.ReplayableCount,
		string(rootIDs), string(tags),
		s.ConfigSnapshotID, string(redaction),
		s.CreatedBy, s.ErrorMessage,
	)
	return err
}

func (r *ScenarioRepository) UpdateStatus(ctx context.Context, id, status string, stoppedAt *time.Time, eventCount, replayableCount int, errMsg string) error {
	_, err := r.db.conn.ExecContext(ctx, `
		UPDATE scenarios SET status = ?, stopped_at_ms = ?, event_count = ?,
			replayable_count = ?, error_message = ?
		WHERE id = ?`,
		status, nilTime(stoppedAt), eventCount, replayableCount, errMsg, id,
	)
	return err
}

func (r *ScenarioRepository) GetByID(ctx context.Context, id string) (*timeline.Scenario, error) {
	row := r.db.conn.QueryRowContext(ctx, `
		SELECT id, name, description, status, started_at_ms, stopped_at_ms,
			event_count, replayable_count, root_event_ids_json, tags_json,
			config_snapshot_id, redaction_summary_json, created_by, error_message
		FROM scenarios WHERE id = ?`, id)
	return scanScenario(row)
}

func (r *ScenarioRepository) GetByName(ctx context.Context, name string) (*timeline.Scenario, error) {
	row := r.db.conn.QueryRowContext(ctx, `
		SELECT id, name, description, status, started_at_ms, stopped_at_ms,
			event_count, replayable_count, root_event_ids_json, tags_json,
			config_snapshot_id, redaction_summary_json, created_by, error_message
		FROM scenarios WHERE name = ?`, name)
	return scanScenario(row)
}

func (r *ScenarioRepository) List(ctx context.Context) ([]timeline.Scenario, error) {
	rows, err := r.db.conn.QueryContext(ctx, `
		SELECT id, name, description, status, started_at_ms, stopped_at_ms,
			event_count, replayable_count, root_event_ids_json, tags_json,
			config_snapshot_id, redaction_summary_json, created_by, error_message
		FROM scenarios ORDER BY started_at_ms DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scenarios []timeline.Scenario
	for rows.Next() {
		s, err := scanScenarioRow(rows)
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, *s)
	}
	return scenarios, rows.Err()
}

func (r *ScenarioRepository) GetActiveRecording(ctx context.Context) (*timeline.Scenario, error) {
	row := r.db.conn.QueryRowContext(ctx, `
		SELECT id, name, description, status, started_at_ms, stopped_at_ms,
			event_count, replayable_count, root_event_ids_json, tags_json,
			config_snapshot_id, redaction_summary_json, created_by, error_message
		FROM scenarios WHERE status = 'recording' LIMIT 1`)
	s, err := scanScenario(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func (r *ScenarioRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.conn.ExecContext(ctx, `DELETE FROM scenarios WHERE id = ?`, id)
	return err
}

func scanScenario(row *sql.Row) (*timeline.Scenario, error) {
	var s timeline.Scenario
	var startMs int64
	var stopMs sql.NullInt64
	var rootIDsJSON, tagsJSON, redactionJSON string

	err := row.Scan(
		&s.ID, &s.Name, &s.Description, &s.Status,
		&startMs, &stopMs,
		&s.EventCount, &s.ReplayableCount,
		&rootIDsJSON, &tagsJSON,
		&s.ConfigSnapshotID, &redactionJSON,
		&s.CreatedBy, &s.ErrorMessage,
	)
	if err != nil {
		return nil, err
	}

	s.StartedAt = time.UnixMilli(startMs).UTC()
	if stopMs.Valid {
		t := time.UnixMilli(stopMs.Int64).UTC()
		s.StoppedAt = &t
	}
	json.Unmarshal([]byte(rootIDsJSON), &s.RootEventIDs)
	json.Unmarshal([]byte(tagsJSON), &s.Tags)
	json.Unmarshal([]byte(redactionJSON), &s.RedactionSummary)

	return &s, nil
}

func scanScenarioRow(rows *sql.Rows) (*timeline.Scenario, error) {
	var s timeline.Scenario
	var startMs int64
	var stopMs sql.NullInt64
	var rootIDsJSON, tagsJSON, redactionJSON string

	err := rows.Scan(
		&s.ID, &s.Name, &s.Description, &s.Status,
		&startMs, &stopMs,
		&s.EventCount, &s.ReplayableCount,
		&rootIDsJSON, &tagsJSON,
		&s.ConfigSnapshotID, &redactionJSON,
		&s.CreatedBy, &s.ErrorMessage,
	)
	if err != nil {
		return nil, err
	}

	s.StartedAt = time.UnixMilli(startMs).UTC()
	if stopMs.Valid {
		t := time.UnixMilli(stopMs.Int64).UTC()
		s.StoppedAt = &t
	}
	json.Unmarshal([]byte(rootIDsJSON), &s.RootEventIDs)
	json.Unmarshal([]byte(tagsJSON), &s.Tags)
	json.Unmarshal([]byte(redactionJSON), &s.RedactionSummary)

	return &s, nil
}

func nilTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UnixMilli()
}

// ReplayRunRepository manages replay run persistence.
type ReplayRunRepository struct {
	db *DB
}

func NewReplayRunRepository(db *DB) *ReplayRunRepository {
	return &ReplayRunRepository{db: db}
}

func (r *ReplayRunRepository) Insert(ctx context.Context, run *timeline.ReplayRun) error {
	diffJSON, _ := json.Marshal(run.DiffSummary)
	_, err := r.db.conn.ExecContext(ctx, `
		INSERT INTO replay_runs (
			id, scenario_id, started_at_ms, finished_at_ms, status,
			target_base_url, request_count, passed_count, failed_count,
			diff_summary_json, created_by, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.ScenarioID, run.StartedAt.UnixMilli(), nilTime(run.FinishedAt),
		run.Status, run.TargetBaseURL,
		run.RequestCount, run.PassedCount, run.FailedCount,
		string(diffJSON), run.CreatedBy, run.ErrorMessage,
	)
	return err
}

func (r *ReplayRunRepository) UpdateStatus(ctx context.Context, id, status string, finishedAt *time.Time, passed, failed int, diffSummary map[string]any, errMsg string) error {
	diffJSON, _ := json.Marshal(diffSummary)
	_, err := r.db.conn.ExecContext(ctx, `
		UPDATE replay_runs SET status = ?, finished_at_ms = ?,
			passed_count = ?, failed_count = ?, diff_summary_json = ?, error_message = ?
		WHERE id = ?`,
		status, nilTime(finishedAt), passed, failed, string(diffJSON), errMsg, id,
	)
	return err
}

func (r *ReplayRunRepository) GetByID(ctx context.Context, id string) (*timeline.ReplayRun, error) {
	var run timeline.ReplayRun
	var startMs int64
	var finMs sql.NullInt64
	var diffJSON string

	err := r.db.conn.QueryRowContext(ctx, `
		SELECT id, scenario_id, started_at_ms, finished_at_ms, status,
			target_base_url, request_count, passed_count, failed_count,
			diff_summary_json, created_by, error_message
		FROM replay_runs WHERE id = ?`, id).Scan(
		&run.ID, &run.ScenarioID, &startMs, &finMs, &run.Status,
		&run.TargetBaseURL, &run.RequestCount, &run.PassedCount, &run.FailedCount,
		&diffJSON, &run.CreatedBy, &run.ErrorMessage,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	run.StartedAt = time.UnixMilli(startMs).UTC()
	if finMs.Valid {
		t := time.UnixMilli(finMs.Int64).UTC()
		run.FinishedAt = &t
	}
	json.Unmarshal([]byte(diffJSON), &run.DiffSummary)

	return &run, nil
}

func (r *ReplayRunRepository) ListByScenario(ctx context.Context, scenarioID string) ([]timeline.ReplayRun, error) {
	rows, err := r.db.conn.QueryContext(ctx, `
		SELECT id, scenario_id, started_at_ms, finished_at_ms, status,
			target_base_url, request_count, passed_count, failed_count,
			diff_summary_json, created_by, error_message
		FROM replay_runs WHERE scenario_id = ? ORDER BY started_at_ms DESC`, scenarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []timeline.ReplayRun
	for rows.Next() {
		var run timeline.ReplayRun
		var startMs int64
		var finMs sql.NullInt64
		var diffJSON string

		err := rows.Scan(
			&run.ID, &run.ScenarioID, &startMs, &finMs, &run.Status,
			&run.TargetBaseURL, &run.RequestCount, &run.PassedCount, &run.FailedCount,
			&diffJSON, &run.CreatedBy, &run.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		run.StartedAt = time.UnixMilli(startMs).UTC()
		if finMs.Valid {
			t := time.UnixMilli(finMs.Int64).UTC()
			run.FinishedAt = &t
		}
		json.Unmarshal([]byte(diffJSON), &run.DiffSummary)
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// FaultRuleRepository manages fault rule persistence.
type FaultRuleRepository struct {
	db *DB
}

func NewFaultRuleRepository(db *DB) *FaultRuleRepository {
	return &FaultRuleRepository{db: db}
}

func (r *FaultRuleRepository) Insert(ctx context.Context, rule *timeline.FaultRule) error {
	matchJSON, _ := json.Marshal(rule.Match)
	actionJSON, _ := json.Marshal(rule.Action)
	safetyJSON, _ := json.Marshal(rule.Safety)

	enabled := 0
	if rule.Enabled {
		enabled = 1
	}

	_, err := r.db.conn.ExecContext(ctx, `
		INSERT INTO fault_rules (
			id, name, enabled, kind, scope,
			match_json, action_json, safety_json,
			hit_count, last_applied_at_ms, created_at_ms, updated_at_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Name, enabled, rule.Kind, rule.Scope,
		string(matchJSON), string(actionJSON), string(safetyJSON),
		rule.HitCount, nilTime(rule.LastAppliedAt),
		rule.CreatedAt.UnixMilli(), rule.UpdatedAt.UnixMilli(),
	)
	return err
}

func (r *FaultRuleRepository) List(ctx context.Context) ([]timeline.FaultRule, error) {
	rows, err := r.db.conn.QueryContext(ctx, `
		SELECT id, name, enabled, kind, scope,
			match_json, action_json, safety_json,
			hit_count, last_applied_at_ms, created_at_ms, updated_at_ms
		FROM fault_rules ORDER BY created_at_ms ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []timeline.FaultRule
	for rows.Next() {
		var rule timeline.FaultRule
		var enabled int
		var matchJSON, actionJSON, safetyJSON string
		var createdMs, updatedMs int64
		var lastAppliedMs sql.NullInt64

		err := rows.Scan(
			&rule.ID, &rule.Name, &enabled, &rule.Kind, &rule.Scope,
			&matchJSON, &actionJSON, &safetyJSON,
			&rule.HitCount, &lastAppliedMs, &createdMs, &updatedMs,
		)
		if err != nil {
			return nil, err
		}
		rule.Enabled = enabled == 1
		rule.CreatedAt = time.UnixMilli(createdMs).UTC()
		rule.UpdatedAt = time.UnixMilli(updatedMs).UTC()
		if lastAppliedMs.Valid {
			t := time.UnixMilli(lastAppliedMs.Int64).UTC()
			rule.LastAppliedAt = &t
		}
		json.Unmarshal([]byte(matchJSON), &rule.Match)
		json.Unmarshal([]byte(actionJSON), &rule.Action)
		json.Unmarshal([]byte(safetyJSON), &rule.Safety)
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *FaultRuleRepository) SetEnabled(ctx context.Context, id string, enabled bool) error {
	e := 0
	if enabled {
		e = 1
	}
	_, err := r.db.conn.ExecContext(ctx, `
		UPDATE fault_rules SET enabled = ?, updated_at_ms = ? WHERE id = ?`,
		e, time.Now().UnixMilli(), id,
	)
	return err
}

func (r *FaultRuleRepository) IncrementHitCount(ctx context.Context, id string) error {
	_, err := r.db.conn.ExecContext(ctx, `
		UPDATE fault_rules SET hit_count = hit_count + 1, last_applied_at_ms = ?, updated_at_ms = ?
		WHERE id = ?`,
		time.Now().UnixMilli(), time.Now().UnixMilli(), id,
	)
	return err
}

func (r *FaultRuleRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.conn.ExecContext(ctx, `DELETE FROM fault_rules WHERE id = ?`, id)
	return err
}

// ConfigSnapshotRepository manages config snapshot persistence.
type ConfigSnapshotRepository struct {
	db *DB
}

func NewConfigSnapshotRepository(db *DB) *ConfigSnapshotRepository {
	return &ConfigSnapshotRepository{db: db}
}

func (r *ConfigSnapshotRepository) Insert(ctx context.Context, snap *timeline.ConfigSnapshot) error {
	_, err := r.db.conn.ExecContext(ctx, `
		INSERT OR IGNORE INTO config_snapshots (id, created_at_ms, hash, config_json, validation_json)
		VALUES (?, ?, ?, ?, ?)`,
		snap.ID, snap.CreatedAt.UnixMilli(), snap.Hash, snap.ConfigJSON, snap.ValidationJSON,
	)
	return err
}

func (r *ConfigSnapshotRepository) GetByHash(ctx context.Context, hash string) (*timeline.ConfigSnapshot, error) {
	var snap timeline.ConfigSnapshot
	var createdMs int64
	err := r.db.conn.QueryRowContext(ctx, `
		SELECT id, created_at_ms, hash, config_json, validation_json
		FROM config_snapshots WHERE hash = ?`, hash).Scan(
		&snap.ID, &createdMs, &snap.Hash, &snap.ConfigJSON, &snap.ValidationJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: get config snapshot: %w", err)
	}
	snap.CreatedAt = time.UnixMilli(createdMs).UTC()
	return &snap, nil
}

// ServiceRepository manages service health persistence.
type ServiceRepository struct {
	db *DB
}

func NewServiceRepository(db *DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

func (r *ServiceRepository) Upsert(ctx context.Context, h *timeline.ServiceHealth) error {
	metaJSON, _ := json.Marshal(h.Metadata)
	_, err := r.db.conn.ExecContext(ctx, `
		INSERT INTO services (service, type, status, endpoint, container_id, last_checked_at_ms, message, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(service) DO UPDATE SET
			status = excluded.status,
			endpoint = excluded.endpoint,
			container_id = excluded.container_id,
			last_checked_at_ms = excluded.last_checked_at_ms,
			message = excluded.message,
			metadata_json = excluded.metadata_json`,
		h.Service, h.Type, h.Status, h.Endpoint, h.ContainerID,
		h.LastCheckedAt.UnixMilli(), h.Message, string(metaJSON),
	)
	return err
}

func (r *ServiceRepository) List(ctx context.Context) ([]timeline.ServiceHealth, error) {
	rows, err := r.db.conn.QueryContext(ctx, `
		SELECT service, type, status, endpoint, container_id, last_checked_at_ms, message, metadata_json
		FROM services ORDER BY service ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []timeline.ServiceHealth
	for rows.Next() {
		var s timeline.ServiceHealth
		var checkedMs int64
		var metaJSON string

		err := rows.Scan(&s.Service, &s.Type, &s.Status, &s.Endpoint, &s.ContainerID, &checkedMs, &s.Message, &metaJSON)
		if err != nil {
			return nil, err
		}
		s.LastCheckedAt = time.UnixMilli(checkedMs).UTC()
		json.Unmarshal([]byte(metaJSON), &s.Metadata)
		services = append(services, s)
	}
	return services, rows.Err()
}
