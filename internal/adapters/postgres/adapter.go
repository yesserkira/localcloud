package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/localcloud-dev/localcloud/internal/adapters"
	"github.com/localcloud-dev/localcloud/internal/id"
	"github.com/localcloud-dev/localcloud/internal/timeline"

	_ "github.com/lib/pq"
)

// Adapter captures Postgres row inserts via an audit trigger.
type Adapter struct {
	name          string
	dsn           string
	tables        []string
	redactColumns []string
	schemas       []string
	runID         string

	conn   *sql.DB
	sink   adapters.EventSink
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *slog.Logger

	mu         sync.RWMutex
	eventCount int64
	status     string
	lastError  string
}

// New creates a Postgres adapter.
func New(name, dsn, runID string, tables, redactColumns, schemas []string, logger *slog.Logger) *Adapter {
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}
	return &Adapter{
		name:          name,
		dsn:           dsn,
		tables:        tables,
		redactColumns: redactColumns,
		schemas:       schemas,
		runID:         runID,
		status:        "stopped",
		logger:        logger,
	}
}

func (a *Adapter) Name() string { return a.name }

func (a *Adapter) Configure(_ context.Context, _ adapters.AdapterConfig) error {
	return nil
}

func (a *Adapter) Start(ctx context.Context, sink adapters.EventSink) error {
	conn, err := sql.Open("postgres", a.dsn)
	if err != nil {
		return fmt.Errorf("postgres adapter: connect: %w", err)
	}
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return fmt.Errorf("postgres adapter: ping: %w", err)
	}

	a.conn = conn
	a.sink = sink

	// Ensure audit trigger infrastructure
	if err := a.ensureAuditTrigger(ctx); err != nil {
		a.logger.Warn("postgres: audit trigger setup failed, will retry", "err", err)
	}

	pollCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.setStatus("running")

	a.wg.Add(1)
	go a.pollAuditLog(pollCtx)

	a.logger.Info("postgres adapter started", "tables", a.tables)
	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	if a.conn != nil {
		a.conn.Close()
	}
	a.setStatus("stopped")
	a.logger.Info("postgres adapter stopped")
	return nil
}

func (a *Adapter) Status(_ context.Context) timeline.AdapterStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return timeline.AdapterStatus{
		Adapter:    "postgres",
		Service:    a.name,
		Enabled:    true,
		Status:     a.status,
		EventCount: a.eventCount,
		LastError:  a.lastError,
	}
}

func (a *Adapter) setStatus(s string) {
	a.mu.Lock()
	a.status = s
	a.mu.Unlock()
}

// ensureAuditTrigger creates the localcloud_audit table and triggers if needed.
func (a *Adapter) ensureAuditTrigger(ctx context.Context) error {
	// Create audit log table
	_, err := a.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _localcloud_audit (
			id BIGSERIAL PRIMARY KEY,
			table_name TEXT NOT NULL,
			operation TEXT NOT NULL,
			row_data JSONB,
			captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			processed BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	if err != nil {
		return fmt.Errorf("create audit table: %w", err)
	}

	// Create or replace the trigger function
	_, err = a.conn.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION _localcloud_audit_fn()
		RETURNS TRIGGER AS $$
		BEGIN
			INSERT INTO _localcloud_audit (table_name, operation, row_data)
			VALUES (TG_TABLE_NAME, TG_OP, row_to_json(NEW));
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql
	`)
	if err != nil {
		return fmt.Errorf("create audit function: %w", err)
	}

	// Attach trigger to each configured table
	for _, table := range a.tables {
		triggerName := fmt.Sprintf("_localcloud_audit_%s", table)
		_, err := a.conn.ExecContext(ctx, fmt.Sprintf(`
			DO $$
			BEGIN
				IF NOT EXISTS (
					SELECT 1 FROM pg_trigger WHERE tgname = '%s'
				) THEN
					CREATE TRIGGER %s
					AFTER INSERT ON %s
					FOR EACH ROW
					EXECUTE FUNCTION _localcloud_audit_fn();
				END IF;
			END $$
		`, triggerName, triggerName, table))
		if err != nil {
			return fmt.Errorf("create trigger for %s: %w", table, err)
		}
	}

	return nil
}

// pollAuditLog periodically checks for new audit rows and emits events.
func (a *Adapter) pollAuditLog(ctx context.Context) {
	defer a.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.processAuditRows(ctx)
		}
	}
}

func (a *Adapter) processAuditRows(ctx context.Context) {
	rows, err := a.conn.QueryContext(ctx, `
		SELECT id, table_name, operation, row_data, captured_at
		FROM _localcloud_audit
		WHERE processed = FALSE
		ORDER BY id ASC
		LIMIT 100
	`)
	if err != nil {
		a.logger.Error("postgres: poll audit rows", "err", err)
		return
	}
	defer rows.Close()

	var processedIDs []int64

	for rows.Next() {
		var (
			auditID    int64
			tableName  string
			operation  string
			rowDataRaw sql.NullString
			capturedAt time.Time
		)
		if err := rows.Scan(&auditID, &tableName, &operation, &rowDataRaw, &capturedAt); err != nil {
			a.logger.Error("postgres: scan audit row", "err", err)
			continue
		}

		// Redact columns
		preview := a.redactRowData(rowDataRaw.String, tableName)

		action := timeline.ActionPostgresInsert
		if operation == "UPDATE" {
			action = timeline.ActionPostgresUpdate
		} else if operation == "DELETE" {
			action = timeline.ActionPostgresDelete
		}

		event := timeline.TimelineEvent{
			ID:        id.Event(),
			RunID:     a.runID,
			Timestamp: capturedAt,
			Source:    timeline.SourcePostgres,
			Service:  a.name,
			Action:   action,
			Summary:  fmt.Sprintf("%s on %s", operation, tableName),
			Status:   timeline.StatusOK,
			Metadata: map[string]any{
				"table":     tableName,
				"operation": operation,
			},
			RawPayload: &timeline.RawPayload{
				ContentType: "application/json",
				Encoding:    "utf-8",
				Preview:     preview,
				Redacted:    len(a.redactColumns) > 0,
			},
		}

		if err := a.sink.Emit(ctx, event); err != nil {
			a.logger.Error("postgres: emit event", "err", err)
			continue
		}

		a.mu.Lock()
		a.eventCount++
		a.mu.Unlock()

		processedIDs = append(processedIDs, auditID)
	}

	if len(processedIDs) > 0 {
		placeholders := make([]string, len(processedIDs))
		args := make([]any, len(processedIDs))
		for i, pid := range processedIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = pid
		}
		query := fmt.Sprintf("UPDATE _localcloud_audit SET processed = TRUE WHERE id IN (%s)",
			strings.Join(placeholders, ","))
		if _, err := a.conn.ExecContext(ctx, query, args...); err != nil {
			a.logger.Error("postgres: mark processed", "err", err)
		}
	}
}

func (a *Adapter) redactRowData(rawJSON, tableName string) string {
	if rawJSON == "" {
		return ""
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &data); err != nil {
		return rawJSON
	}

	for _, col := range a.redactColumns {
		// Format: "table.column" or just "column"
		parts := strings.SplitN(col, ".", 2)
		var table, column string
		if len(parts) == 2 {
			table, column = parts[0], parts[1]
		} else {
			column = parts[0]
		}

		if table != "" && table != tableName {
			continue
		}

		if _, ok := data[column]; ok {
			data[column] = "[REDACTED]"
		}
	}

	out, err := json.Marshal(data)
	if err != nil {
		return rawJSON
	}
	return string(out)
}
