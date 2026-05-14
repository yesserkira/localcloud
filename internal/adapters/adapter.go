package adapters

import (
	"context"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// Adapter is the V1 compiled-in adapter interface.
// No external plugin system until at least three adapters are stable.
type Adapter interface {
	// Name returns the adapter identifier (e.g. "http", "postgres", "redis").
	Name() string

	// Configure validates and applies adapter-specific configuration.
	Configure(ctx context.Context, cfg AdapterConfig) error

	// Start begins capturing events and emitting them to the sink.
	Start(ctx context.Context, sink EventSink) error

	// Stop gracefully shuts down the adapter.
	Stop(ctx context.Context) error

	// Status returns the adapter's current health and event count.
	Status(ctx context.Context) timeline.AdapterStatus
}

// EventSink is the interface adapters use to emit normalized events.
type EventSink interface {
	// Emit sends a validated timeline event to the event bus and storage.
	Emit(ctx context.Context, event timeline.TimelineEvent) error

	// ReportStatus updates the adapter's status in the agent.
	ReportStatus(ctx context.Context, status timeline.AdapterStatus) error
}

// AdapterConfig is the adapter-specific configuration passed from the agent.
type AdapterConfig struct {
	ServiceName string
	ServiceType string
	RawConfig   map[string]any
}
