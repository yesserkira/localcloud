package agent

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/localcloud-dev/localcloud/internal/adapters"
	"github.com/localcloud-dev/localcloud/internal/api"
	"github.com/localcloud-dev/localcloud/internal/config"
	"github.com/localcloud-dev/localcloud/internal/eventbus"
	"github.com/localcloud-dev/localcloud/internal/id"
	"github.com/localcloud-dev/localcloud/internal/storage"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// Status values for the agent.
const (
	StatusStarting = "starting"
	StatusRunning  = "running"
	StatusStopping = "stopping"
	StatusStopped  = "stopped"
	StatusError    = "error"
)

// Agent is the long-running LocalCloud control plane process.
type Agent struct {
	mu      sync.RWMutex
	cfg     *config.Config
	version string
	runID   string
	status  string

	db              *storage.DB
	bus             *eventbus.Bus
	api             *api.Server
	adapters        []adapters.Adapter
	sink            *agentSink
	activeRecording *recording

	startedAt time.Time
	cancel    context.CancelFunc
	done      chan struct{}
	logger    *slog.Logger
}

// New creates a new agent from configuration.
func New(cfg *config.Config, version string, logger *slog.Logger) *Agent {
	return &Agent{
		cfg:     cfg,
		version: version,
		runID:   id.Run(),
		status:  StatusStopped,
		bus:     eventbus.New(),
		logger:  logger,
		done:    make(chan struct{}),
	}
}

// RunID returns the current run identifier.
func (a *Agent) RunID() string { return a.runID }

// Status returns the current agent status.
func (a *Agent) Status() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

func (a *Agent) setStatus(s string) {
	a.mu.Lock()
	a.status = s
	a.mu.Unlock()
}

// Start initializes storage, adapters, and the Studio API server.
func (a *Agent) Start(ctx context.Context) error {
	a.setStatus(StatusStarting)
	a.startedAt = time.Now().UTC()

	a.logger.Info("agent starting", "runId", a.runID, "project", a.cfg.Project.Name)

	// Open database
	db, err := storage.Open(a.cfg.Agent.Database)
	if err != nil {
		a.setStatus(StatusError)
		return fmt.Errorf("agent: open database: %w", err)
	}
	a.db = db

	// Create the event sink
	a.sink = &agentSink{
		events: storage.NewEventRepository(db),
		bus:    a.bus,
		logger: a.logger,
		agent:  a,
	}

	// Start adapters
	adapterCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	for _, adapter := range a.adapters {
		a.logger.Info("starting adapter", "name", adapter.Name())
		if err := adapter.Start(adapterCtx, a.sink); err != nil {
			a.logger.Error("adapter start failed", "name", adapter.Name(), "err", err)
			// Continue starting other adapters
		}
	}

	// Start Studio API
	studioAddr := fmt.Sprintf("%s:%d", a.cfg.Agent.Bind, a.cfg.Agent.StudioPort)
	a.api = api.NewServer(db, a.bus, a.version, a.runID, a.logger)
	studioLn, err := net.Listen("tcp", studioAddr)
	if err != nil {
		a.setStatus(StatusError)
		return fmt.Errorf("agent: listen studio %s: %w", studioAddr, err)
	}
	go func() {
		if err := a.api.Serve(studioLn); err != nil {
			a.logger.Error("studio api stopped", "err", err)
		}
	}()

	a.setStatus(StatusRunning)
	a.logger.Info("agent running",
		"runId", a.runID,
		"studioAddr", studioAddr,
		"adapterCount", len(a.adapters),
	)

	return nil
}

// Stop gracefully shuts down the agent.
func (a *Agent) Stop(ctx context.Context) error {
	a.setStatus(StatusStopping)
	a.logger.Info("agent stopping", "runId", a.runID)

	// Stop adapters in reverse order
	for i := len(a.adapters) - 1; i >= 0; i-- {
		name := a.adapters[i].Name()
		if err := a.adapters[i].Stop(ctx); err != nil {
			a.logger.Error("adapter stop failed", "name", name, "err", err)
		}
	}

	// Cancel adapter context
	if a.cancel != nil {
		a.cancel()
	}

	// Stop Studio API
	if a.api != nil {
		if err := a.api.Shutdown(ctx); err != nil {
			a.logger.Error("studio api shutdown failed", "err", err)
		}
	}

	// Close database
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.logger.Error("database close failed", "err", err)
		}
	}

	a.setStatus(StatusStopped)
	a.logger.Info("agent stopped", "runId", a.runID)
	close(a.done)
	return nil
}

// Wait blocks until the agent is stopped.
func (a *Agent) Wait() {
	<-a.done
}

// RegisterAdapter adds an adapter to be started with the agent.
func (a *Agent) RegisterAdapter(adapter adapters.Adapter) {
	a.adapters = append(a.adapters, adapter)
}

// DB returns the agent's database handle.
func (a *Agent) DB() *storage.DB { return a.db }

// Bus returns the agent's event bus.
func (a *Agent) Bus() *eventbus.Bus { return a.bus }

// Config returns the agent's configuration.
func (a *Agent) Config() *config.Config { return a.cfg }

// Sink returns the agent's event sink for use by external components like proxies.
func (a *Agent) Sink() adapters.EventSink { return a.sink }

// --- agentSink bridges adapters to the event bus and storage ---

type agentSink struct {
	events *storage.EventRepository
	bus    *eventbus.Bus
	logger *slog.Logger
	agent  *Agent // back-reference for scenario tagging
}

func (s *agentSink) Emit(ctx context.Context, event timeline.TimelineEvent) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("sink: invalid event: %w", err)
	}

	// Tag event with active scenario if recording
	if scenarioID := s.agent.ActiveRecording(); scenarioID != "" && event.ScenarioID == nil {
		event.ScenarioID = &scenarioID
	}
	if err := s.events.Insert(ctx, &event); err != nil {
		s.logger.Error("sink: event insert failed", "id", event.ID, "err", err)
		return fmt.Errorf("sink: insert event: %w", err)
	}
	s.bus.Publish(ctx, event)
	return nil
}

func (s *agentSink) ReportStatus(_ context.Context, _ timeline.AdapterStatus) error {
	// Adapter status is tracked in-memory by the agent; no persistence needed yet.
	return nil
}
