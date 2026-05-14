package replay

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/localcloud-dev/localcloud/internal/id"
	"github.com/localcloud-dev/localcloud/internal/storage"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// PlanEntry is a single replayed request from a captured scenario.
type PlanEntry struct {
	OriginalEventID string                   `json:"originalEventId"`
	Method          string                   `json:"method"`
	Path            string                   `json:"path"`
	Headers         map[string]string        `json:"headers,omitempty"`
	Body            string                   `json:"body,omitempty"`
	OriginalStatus  int                      `json:"originalStatus"`
	Safe            bool                     `json:"safe"` // true for GET/HEAD/OPTIONS
	OriginalEvent   *timeline.TimelineEvent  `json:"-"`
}

// Plan generates a replay plan from a completed scenario.
func Plan(ctx context.Context, db *storage.DB, scenarioID string) ([]PlanEntry, error) {
	eventRepo := storage.NewEventRepository(db)
	events, err := eventRepo.ListByScenario(ctx, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("replay: list events: %w", err)
	}

	var plan []PlanEntry
	for i := range events {
		e := &events[i]
		if e.Source != timeline.SourceHTTPProxy {
			continue
		}
		if e.Request == nil || !e.Request.Replayable {
			continue
		}

		entry := PlanEntry{
			OriginalEventID: e.ID,
			Method:          e.Request.Method,
			Path:            e.Request.Path,
			Headers:         copyHeaders(e.Request.Headers),
			Body:            e.Request.BodyPreview,
			OriginalStatus:  0,
			Safe:            isSafeMethod(e.Request.Method),
			OriginalEvent:   e,
		}
		if e.Response != nil {
			entry.OriginalStatus = e.Response.StatusCode
		}
		plan = append(plan, entry)
	}
	return plan, nil
}

// Diff captures a comparison between original and replayed response.
type Diff struct {
	EventID        string            `json:"eventId"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	OriginalStatus int               `json:"originalStatus"`
	ReplayStatus   int               `json:"replayStatus"`
	StatusMatch    bool              `json:"statusMatch"`
	BodyDiffs      []string          `json:"bodyDiffs,omitempty"`
	HeaderDiffs    map[string]string `json:"headerDiffs,omitempty"`
	DurationMs     int64             `json:"durationMs"`
	Error          string            `json:"error,omitempty"`
}

// Result is the outcome of a replay run.
type Result struct {
	RunID       string `json:"runId"`
	ScenarioID  string `json:"scenarioId"`
	Total       int    `json:"total"`
	Passed      int    `json:"passed"`
	Failed      int    `json:"failed"`
	Skipped     int    `json:"skipped"`
	Diffs       []Diff `json:"diffs"`
}

// Options configures a replay execution.
type Options struct {
	BaseURL        string
	SkipUnsafe     bool // skip non-idempotent methods (POST, PUT, DELETE, PATCH)
	ConfirmUnsafe  bool // confirmed by user — allow unsafe methods
	Logger         *slog.Logger
}

// Execute runs a replay plan against the target, recording results.
func Execute(ctx context.Context, db *storage.DB, scenarioID string, opts Options) (*Result, error) {
	scenarioRepo := storage.NewScenarioRepository(db)
	replayRepo := storage.NewReplayRunRepository(db)

	s, err := scenarioRepo.GetByID(ctx, scenarioID)
	if err != nil || s == nil {
		return nil, fmt.Errorf("replay: scenario %q not found", scenarioID)
	}
	if s.Status == timeline.ScenarioStatusRecording {
		return nil, fmt.Errorf("replay: cannot replay active recording")
	}

	plan, err := Plan(ctx, db, scenarioID)
	if err != nil {
		return nil, err
	}
	if len(plan) == 0 {
		return nil, fmt.Errorf("replay: no replayable events in scenario")
	}

	// Check if unsafe methods exist and aren't confirmed
	hasUnsafe := false
	for _, e := range plan {
		if !e.Safe {
			hasUnsafe = true
			break
		}
	}
	if hasUnsafe && !opts.ConfirmUnsafe && !opts.SkipUnsafe {
		return nil, fmt.Errorf("replay: scenario contains unsafe methods (POST/PUT/DELETE/PATCH); use --confirm-unsafe or --skip-unsafe")
	}

	// Create replay run record
	now := time.Now().UTC()
	run := &timeline.ReplayRun{
		ID:            id.ReplayRun(),
		ScenarioID:    scenarioID,
		StartedAt:     now,
		Status:        timeline.ReplayStatusRunning,
		TargetBaseURL: opts.BaseURL,
		RequestCount:  len(plan),
		CreatedBy:     "cli",
	}
	if err := replayRepo.Insert(ctx, run); err != nil {
		return nil, fmt.Errorf("replay: insert run: %w", err)
	}

	result := &Result{
		RunID:      run.ID,
		ScenarioID: scenarioID,
		Total:      len(plan),
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, entry := range plan {
		if ctx.Err() != nil {
			break
		}

		if !entry.Safe && opts.SkipUnsafe {
			result.Skipped++
			continue
		}

		diff := executeEntry(ctx, client, opts.BaseURL, entry)
		result.Diffs = append(result.Diffs, diff)

		if diff.Error == "" && diff.StatusMatch {
			result.Passed++
		} else {
			result.Failed++
		}

		if opts.Logger != nil {
			opts.Logger.Info("replay request",
				"method", entry.Method,
				"path", entry.Path,
				"original", entry.OriginalStatus,
				"replay", diff.ReplayStatus,
				"match", diff.StatusMatch,
			)
		}
	}

	// Update replay run
	finishedAt := time.Now().UTC()
	status := timeline.ReplayStatusPassed
	if result.Failed > 0 {
		status = timeline.ReplayStatusFailed
	}

	diffSummary := map[string]any{
		"total":   result.Total,
		"passed":  result.Passed,
		"failed":  result.Failed,
		"skipped": result.Skipped,
	}
	if err := replayRepo.UpdateStatus(ctx, run.ID, status, &finishedAt, result.Passed, result.Failed, diffSummary, ""); err != nil {
		if opts.Logger != nil {
			opts.Logger.Error("replay: failed to update run status", "runID", run.ID, "err", err)
		}
	}

	return result, nil
}

func executeEntry(ctx context.Context, client *http.Client, baseURL string, entry PlanEntry) Diff {
	d := Diff{
		EventID:        entry.OriginalEventID,
		Method:         entry.Method,
		Path:           entry.Path,
		OriginalStatus: entry.OriginalStatus,
	}

	url := strings.TrimRight(baseURL, "/") + entry.Path

	var body io.Reader
	if entry.Body != "" {
		body = bytes.NewBufferString(entry.Body)
	}

	req, err := http.NewRequestWithContext(ctx, entry.Method, url, body)
	if err != nil {
		d.Error = fmt.Sprintf("build request: %v", err)
		return d
	}

	// Copy headers, skip host/connection
	for k, v := range entry.Headers {
		lower := strings.ToLower(k)
		if lower == "host" || lower == "connection" || lower == "content-length" {
			continue
		}
		if v != "[REDACTED]" {
			req.Header.Set(k, v)
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	d.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		d.Error = fmt.Sprintf("request failed: %v", err)
		return d
	}
	defer resp.Body.Close()

	d.ReplayStatus = resp.StatusCode
	d.StatusMatch = d.OriginalStatus == d.ReplayStatus

	// Compare response bodies if status differs
	if !d.StatusMatch {
		d.BodyDiffs = append(d.BodyDiffs,
			fmt.Sprintf("status: expected %d, got %d", d.OriginalStatus, d.ReplayStatus))
	}

	// Check content-type changes
	origCT := ""
	if entry.OriginalEvent != nil && entry.OriginalEvent.Response != nil {
		origCT = entry.OriginalEvent.Response.Headers["content-type"]
	}
	replayCT := resp.Header.Get("Content-Type")
	if origCT != "" && replayCT != "" {
		origBase := strings.Split(origCT, ";")[0]
		replayBase := strings.Split(replayCT, ";")[0]
		if origBase != replayBase {
			d.BodyDiffs = append(d.BodyDiffs,
				fmt.Sprintf("content-type: expected %s, got %s", origBase, replayBase))
		}
	}

	return d
}

func isSafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

func copyHeaders(h map[string]string) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}
