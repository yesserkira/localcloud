package timeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Status values for timeline events.
const (
	StatusOK      = "ok"
	StatusError   = "error"
	StatusWarning = "warning"
	StatusPending = "pending"
	StatusBlocked = "blocked"
	StatusFaulted = "faulted"
	StatusUnknown = "unknown"
)

// Source values identifying which adapter produced the event.
const (
	SourceHTTPProxy  = "http_proxy"
	SourcePostgres   = "postgres"
	SourceRedis      = "redis"
	SourceMailpit    = "mailpit"
	SourceDocker     = "docker"
	SourceMinIO      = "minio"
	SourceLocalStack = "localstack"
	SourceStripe     = "stripe"
	SourceReplay     = "replay"
	SourceFault      = "fault"
	SourceAgent      = "agent"
)

// Action constants for well-known event types.
const (
	ActionHTTPRequest  = "http.request"
	ActionHTTPResponse = "http.response"
	ActionHTTPError    = "http.error"

	ActionPostgresInsert = "postgres.insert"
	ActionPostgresUpdate = "postgres.update"
	ActionPostgresDelete = "postgres.delete"
	ActionPostgresHealth = "postgres.health"

	ActionRedisEnqueue      = "redis.enqueue"
	ActionRedisDequeue      = "redis.dequeue"
	ActionRedisPublish      = "redis.publish"
	ActionRedisStreamAppend = "redis.stream.append"
	ActionRedisCommand      = "redis.command"

	ActionEmailCaptured = "email.captured"

	ActionDockerContainerStart = "docker.container.start"
	ActionDockerContainerStop  = "docker.container.stop"

	ActionServiceHealthChanged = "service.health.changed"

	ActionFaultApplied = "fault.applied"

	ActionReplayRequest = "replay.request"
	ActionReplayCompare = "replay.compare"

	ActionWorkerJobStart    = "worker.job.start"
	ActionWorkerJobComplete = "worker.job.complete"
)

// TimelineEvent is the canonical fact model for all captured backend activity.
type TimelineEvent struct {
	ID            string            `json:"id"`
	RunID         string            `json:"runId"`
	ScenarioID    *string           `json:"scenarioId,omitempty"`
	ReplayRunID   *string           `json:"replayRunId,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
	Source        string            `json:"source"`
	Service       string            `json:"service"`
	Action        string            `json:"action"`
	Summary       string            `json:"summary"`
	Status        string            `json:"status"`
	DurationMs    *int64            `json:"durationMs,omitempty"`
	CorrelationID *string           `json:"correlationId,omitempty"`
	ParentEventID *string           `json:"parentEventId,omitempty"`
	Request       *RequestData      `json:"request,omitempty"`
	Response      *ResponseData     `json:"response,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
	RawPayload    *RawPayload       `json:"rawPayload,omitempty"`
	Faults        []FaultAnnotation `json:"faults,omitempty"`
}

// RequestData stores captured HTTP request details after redaction.
type RequestData struct {
	Method        string            `json:"method,omitempty"`
	Scheme        string            `json:"scheme,omitempty"`
	Host          string            `json:"host,omitempty"`
	Path          string            `json:"path,omitempty"`
	Query         string            `json:"query,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	BodyPreview   string            `json:"bodyPreview,omitempty"`
	BodySHA256    string            `json:"bodySha256,omitempty"`
	BodyRedacted  bool              `json:"bodyRedacted"`
	Replayable    bool              `json:"replayable"`
	ReplayWarning string            `json:"replayWarning,omitempty"`
}

// ResponseData stores captured HTTP response details after redaction.
type ResponseData struct {
	StatusCode   int               `json:"statusCode,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	BodyPreview  string            `json:"bodyPreview,omitempty"`
	BodySHA256   string            `json:"bodySha256,omitempty"`
	BodyRedacted bool              `json:"bodyRedacted"`
}

// RawPayload stores a redacted preview/hash for non-HTTP event payloads.
type RawPayload struct {
	ContentType string `json:"contentType"`
	Encoding    string `json:"encoding"`
	Preview     string `json:"preview"`
	SHA256      string `json:"sha256"`
	Redacted    bool   `json:"redacted"`
}

// FaultAnnotation records a fault rule application on an event.
type FaultAnnotation struct {
	RuleID        string    `json:"ruleId"`
	RuleName      string    `json:"ruleName"`
	Kind          string    `json:"kind"`
	AppliedAt     time.Time `json:"appliedAt"`
	EffectSummary string    `json:"effectSummary"`
}

// Validate checks that required fields are present and valid.
func (e *TimelineEvent) Validate() error {
	var errs []error

	if e.ID == "" {
		errs = append(errs, fmt.Errorf("event: id is required"))
	}
	if e.RunID == "" {
		errs = append(errs, fmt.Errorf("event: runId is required"))
	}
	if e.Timestamp.IsZero() {
		errs = append(errs, fmt.Errorf("event: timestamp is required"))
	}
	if e.Source == "" {
		errs = append(errs, fmt.Errorf("event: source is required"))
	}
	if e.Service == "" {
		errs = append(errs, fmt.Errorf("event: service is required"))
	}
	if e.Action == "" {
		errs = append(errs, fmt.Errorf("event: action is required"))
	}
	if e.Summary == "" {
		errs = append(errs, fmt.Errorf("event: summary is required"))
	}
	if !isValidStatus(e.Status) {
		errs = append(errs, fmt.Errorf("event: status %q is invalid", e.Status))
	}

	return errors.Join(errs...)
}

func isValidStatus(s string) bool {
	switch s {
	case StatusOK, StatusError, StatusWarning, StatusPending,
		StatusBlocked, StatusFaulted, StatusUnknown:
		return true
	}
	return false
}

// MarshalJSON implements json.Marshaler using the default encoding.
func (e *TimelineEvent) MarshalJSON() ([]byte, error) {
	type Alias TimelineEvent
	return json.Marshal((*Alias)(e))
}
