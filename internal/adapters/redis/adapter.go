package redis

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/localcloud-dev/localcloud/internal/adapters"
	"github.com/localcloud-dev/localcloud/internal/id"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// Adapter captures Redis queue operations using the MONITOR command.
type Adapter struct {
	name            string
	addr            string
	runID           string
	queues          []string
	redactJSONPaths []string

	conn   net.Conn
	sink   adapters.EventSink
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *slog.Logger

	mu         sync.RWMutex
	eventCount int64
	status     string
	lastError  string
}

// New creates a Redis adapter.
func New(name, addr, runID string, queues, redactJSONPaths []string, logger *slog.Logger) *Adapter {
	return &Adapter{
		name:            name,
		addr:            addr,
		runID:           runID,
		queues:          queues,
		redactJSONPaths: redactJSONPaths,
		status:          "stopped",
		logger:          logger,
	}
}

func (a *Adapter) Name() string { return a.name }

func (a *Adapter) Configure(_ context.Context, _ adapters.AdapterConfig) error {
	return nil
}

func (a *Adapter) Start(ctx context.Context, sink adapters.EventSink) error {
	conn, err := net.DialTimeout("tcp", a.addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("redis adapter: connect %s: %w", a.addr, err)
	}
	a.conn = conn
	a.sink = sink

	// Send MONITOR command
	if _, err := fmt.Fprintf(conn, "MONITOR\r\n"); err != nil {
		conn.Close()
		return fmt.Errorf("redis adapter: send MONITOR: %w", err)
	}

	monitorCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.setStatus("running")

	a.wg.Add(1)
	go a.readMonitor(monitorCtx)

	a.logger.Info("redis adapter started", "addr", a.addr, "queues", a.queues)
	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.conn != nil {
		a.conn.Close()
	}
	a.wg.Wait()
	a.setStatus("stopped")
	a.logger.Info("redis adapter stopped")
	return nil
}

func (a *Adapter) Status(_ context.Context) timeline.AdapterStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return timeline.AdapterStatus{
		Adapter:    "redis",
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

// readMonitor parses MONITOR output and emits events for configured queue commands.
// MONITOR output format: +1621234567.123456 [0 127.0.0.1:12345] "LPUSH" "email_jobs" "{...}"
func (a *Adapter) readMonitor(ctx context.Context) {
	defer a.wg.Done()
	scanner := bufio.NewScanner(a.conn)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		// Skip the initial "+OK" response
		if line == "+OK" {
			continue
		}

		parsed := a.parseLine(line)
		if parsed == nil {
			continue
		}

		// Filter: only emit events for configured queues
		if !a.isRelevantCommand(parsed) {
			continue
		}

		event := a.buildEvent(parsed)
		if err := a.sink.Emit(ctx, event); err != nil {
			a.logger.Error("redis: emit event", "err", err)
			continue
		}

		a.mu.Lock()
		a.eventCount++
		a.mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-ctx.Done():
			// Expected on shutdown
		default:
			a.logger.Error("redis: monitor read error", "err", err)
			a.mu.Lock()
			a.lastError = err.Error()
			a.mu.Unlock()
		}
	}
}

type monitorEntry struct {
	timestamp time.Time
	command   string
	args      []string
	key       string
}

func (a *Adapter) parseLine(line string) *monitorEntry {
	// Format: +1621234567.123456 [0 127.0.0.1:12345] "CMD" "key" "value" ...
	if !strings.HasPrefix(line, "+") {
		return nil
	}

	// Find timestamp end
	spaceIdx := strings.Index(line, " ")
	if spaceIdx < 0 {
		return nil
	}

	// Parse timestamp (skip '+')
	ts := time.Now().UTC() // Use current time as fallback

	// Find closing bracket of [db addr]
	bracketEnd := strings.Index(line, "] ")
	if bracketEnd < 0 {
		return nil
	}

	rest := line[bracketEnd+2:]

	// Parse quoted strings
	parts := parseQuotedStrings(rest)
	if len(parts) == 0 {
		return nil
	}

	entry := &monitorEntry{
		timestamp: ts,
		command:   strings.ToUpper(parts[0]),
		args:      parts[1:],
	}

	if len(entry.args) > 0 {
		entry.key = entry.args[0]
	}

	return entry
}

func (a *Adapter) isRelevantCommand(e *monitorEntry) bool {
	// Queue commands we care about
	relevant := map[string]bool{
		"LPUSH": true, "RPUSH": true,
		"LPOP": true, "RPOP": true,
		"BRPOP": true, "BLPOP": true,
		"LLEN": false, // Don't capture read-only
	}

	if !relevant[e.command] {
		return false
	}

	// Check if key matches configured queues
	for _, q := range a.queues {
		if e.key == q {
			return true
		}
	}
	return false
}

func (a *Adapter) buildEvent(e *monitorEntry) timeline.TimelineEvent {
	action := timeline.ActionRedisCommand
	summary := fmt.Sprintf("%s %s", e.command, e.key)

	switch e.command {
	case "LPUSH", "RPUSH":
		action = timeline.ActionRedisEnqueue
		summary = fmt.Sprintf("enqueue to %s", e.key)
	case "LPOP", "RPOP", "BRPOP", "BLPOP":
		action = timeline.ActionRedisDequeue
		summary = fmt.Sprintf("dequeue from %s", e.key)
	}

	// Build preview from args (skip key)
	preview := ""
	if len(e.args) > 1 {
		preview = a.redactPayload(e.args[1])
	}

	return timeline.TimelineEvent{
		ID:        id.Event(),
		RunID:     a.runID,
		Timestamp: e.timestamp,
		Source:    timeline.SourceRedis,
		Service:  a.name,
		Action:   action,
		Summary:  summary,
		Status:   timeline.StatusOK,
		Metadata: map[string]any{
			"command": e.command,
			"key":     e.key,
		},
		RawPayload: &timeline.RawPayload{
			ContentType: "application/json",
			Encoding:    "utf-8",
			Preview:     preview,
			Redacted:    len(a.redactJSONPaths) > 0,
		},
	}
}

func (a *Adapter) redactPayload(raw string) string {
	if raw == "" || len(a.redactJSONPaths) == 0 {
		return raw
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return raw
	}

	for _, path := range a.redactJSONPaths {
		field := strings.TrimPrefix(path, "$.")
		if _, ok := data[field]; ok {
			data[field] = "[REDACTED]"
		}
	}

	out, err := json.Marshal(data)
	if err != nil {
		return raw
	}
	return string(out)
}

// parseQuotedStrings extracts quoted values from a MONITOR line.
func parseQuotedStrings(s string) []string {
	var result []string
	for len(s) > 0 {
		idx := strings.Index(s, "\"")
		if idx < 0 {
			break
		}
		s = s[idx+1:]
		// Find the closing quote, handling backslash escapes
		var buf strings.Builder
		closed := false
		for i := 0; i < len(s); i++ {
			if s[i] == '\\' && i+1 < len(s) {
				buf.WriteByte(s[i+1])
				i++ // skip escaped char
			} else if s[i] == '"' {
				s = s[i+1:]
				closed = true
				break
			} else {
				buf.WriteByte(s[i])
			}
		}
		if !closed {
			break
		}
		result = append(result, buf.String())
	}
	return result
}
