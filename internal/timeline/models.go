package timeline

import "time"

// Scenario represents a named recorded backend flow.
type Scenario struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Description      string         `json:"description,omitempty"`
	Status           string         `json:"status"` // recording, completed, exported, failed
	StartedAt        time.Time      `json:"startedAt"`
	StoppedAt        *time.Time     `json:"stoppedAt,omitempty"`
	EventCount       int            `json:"eventCount"`
	ReplayableCount  int            `json:"replayableCount"`
	RootEventIDs     []string       `json:"rootEventIds"`
	Tags             []string       `json:"tags"`
	ConfigSnapshotID string         `json:"configSnapshotId"`
	RedactionSummary map[string]any `json:"redactionSummary"`
	CreatedBy        string         `json:"createdBy"`
	ErrorMessage     string         `json:"errorMessage,omitempty"`
}

// Scenario status values.
const (
	ScenarioStatusRecording = "recording"
	ScenarioStatusCompleted = "completed"
	ScenarioStatusExported  = "exported"
	ScenarioStatusFailed    = "failed"
)

// ReplayRun represents one execution of replaying a scenario.
type ReplayRun struct {
	ID            string         `json:"id"`
	ScenarioID    string         `json:"scenarioId"`
	StartedAt     time.Time      `json:"startedAt"`
	FinishedAt    *time.Time     `json:"finishedAt,omitempty"`
	Status        string         `json:"status"` // running, passed, failed, partial, canceled
	TargetBaseURL string         `json:"targetBaseUrl"`
	RequestCount  int            `json:"requestCount"`
	PassedCount   int            `json:"passedCount"`
	FailedCount   int            `json:"failedCount"`
	DiffSummary   map[string]any `json:"diffSummary"`
	CreatedBy     string         `json:"createdBy"`
	ErrorMessage  string         `json:"errorMessage,omitempty"`
}

// ReplayRun status values.
const (
	ReplayStatusRunning  = "running"
	ReplayStatusPassed   = "passed"
	ReplayStatusFailed   = "failed"
	ReplayStatusPartial  = "partial"
	ReplayStatusCanceled = "canceled"
)

// FaultRule defines a configurable local failure injection rule.
type FaultRule struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Enabled       bool        `json:"enabled"`
	Kind          string      `json:"kind"`
	Scope         string      `json:"scope"` // live, replay, both
	Match         FaultMatch  `json:"match"`
	Action        FaultAction `json:"action"`
	Safety        FaultSafety `json:"safety"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	HitCount      int         `json:"hitCount"`
	LastAppliedAt *time.Time  `json:"lastAppliedAt,omitempty"`
}

// FaultRule kind values.
const (
	FaultKindDelayResponse      = "delay_response"
	FaultKindForceHTTPStatus    = "force_http_status"
	FaultKindDropOutbound       = "drop_outbound_request"
	FaultKindMutateJSON         = "mutate_json_response"
	FaultKindSimulateTimeout    = "simulate_timeout"
	FaultKindDelayQueue         = "delay_queue_processing"
	FaultKindBlockEmail         = "block_email_delivery"
)

// FaultRule scope values.
const (
	FaultScopeLive   = "live"
	FaultScopeReplay = "replay"
	FaultScopeBoth   = "both"
)

// FaultMatch describes which events a fault rule targets.
type FaultMatch struct {
	Source          string            `json:"source,omitempty" yaml:"source,omitempty"`
	Service         string            `json:"service,omitempty" yaml:"service,omitempty"`
	Method          string            `json:"method,omitempty" yaml:"method,omitempty"`
	Path            string            `json:"path,omitempty" yaml:"path,omitempty"`
	PathPrefix      string            `json:"pathPrefix,omitempty" yaml:"pathPrefix,omitempty"`
	Host            string            `json:"host,omitempty" yaml:"host,omitempty"`
	StatusCode      int               `json:"statusCode,omitempty" yaml:"statusCode,omitempty"`
	Queue           string            `json:"queue,omitempty" yaml:"queue,omitempty"`
	EmailToContains string            `json:"emailToContains,omitempty" yaml:"emailToContains,omitempty"`
	Headers         map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// FaultAction describes the effect when a fault rule matches.
type FaultAction struct {
	StatusCode int            `json:"statusCode,omitempty" yaml:"statusCode,omitempty"`
	BodyJSON   map[string]any `json:"bodyJson,omitempty" yaml:"bodyJson,omitempty"`
	DelayMs    int            `json:"delayMs,omitempty" yaml:"delayMs,omitempty"`
	Reason     string         `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// FaultSafety defines limits that prevent unbounded fault impact.
type FaultSafety struct {
	MaxHits      int    `json:"maxHits,omitempty" yaml:"maxHits,omitempty"`
	ExpiresAfter string `json:"expiresAfter,omitempty" yaml:"expiresAfter,omitempty"`
	LocalOnly    bool   `json:"localOnly,omitempty" yaml:"localOnly,omitempty"`
}

// ServiceHealth captures the current health of a configured local service.
type ServiceHealth struct {
	Service       string         `json:"service"`
	Type          string         `json:"type"`
	Status        string         `json:"status"` // healthy, unhealthy, starting, stopped, unknown
	Endpoint      string         `json:"endpoint,omitempty"`
	ContainerID   string         `json:"containerId,omitempty"`
	LastCheckedAt time.Time      `json:"lastCheckedAt"`
	Message       string         `json:"message,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// Service health status values.
const (
	ServiceHealthy   = "healthy"
	ServiceUnhealthy = "unhealthy"
	ServiceStarting  = "starting"
	ServiceStopped   = "stopped"
	ServiceUnknown   = "unknown"
)

// AdapterStatus tracks the runtime state of a capture adapter.
type AdapterStatus struct {
	Adapter     string         `json:"adapter"`
	Service     string         `json:"service,omitempty"`
	Enabled     bool           `json:"enabled"`
	Status      string         `json:"status"` // running, disabled, degraded, error
	LastEventAt *time.Time     `json:"lastEventAt,omitempty"`
	LastError   string         `json:"lastError,omitempty"`
	EventCount  int64          `json:"eventCount"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Adapter status values.
const (
	AdapterRunning  = "running"
	AdapterDisabled = "disabled"
	AdapterDegraded = "degraded"
	AdapterError    = "error"
)

// ConfigSnapshot records the effective configuration at a point in time.
type ConfigSnapshot struct {
	ID             string `json:"id"`
	CreatedAt      time.Time `json:"createdAt"`
	Hash           string `json:"hash"`
	ConfigJSON     string `json:"configJson"`
	ValidationJSON string `json:"validationJson"`
}
