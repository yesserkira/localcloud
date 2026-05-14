package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/localcloud-dev/localcloud/internal/eventbus"
	"github.com/localcloud-dev/localcloud/internal/storage"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// Server is the Studio API HTTP server.
type Server struct {
	mux      *http.ServeMux
	server   *http.Server
	db       *storage.DB
	events   *storage.EventRepository
	scenarios *storage.ScenarioRepository
	replays  *storage.ReplayRunRepository
	faults   *storage.FaultRuleRepository
	services *storage.ServiceRepository
	bus      *eventbus.Bus
	version  string
	runID    string
	startAt  time.Time
	logger   *slog.Logger

	// SSE
	sseClients   map[string]chan []byte
	sseMu        sync.Mutex
	sseNextID    int
}

// NewServer creates a Studio API server.
func NewServer(
	db *storage.DB,
	bus *eventbus.Bus,
	version string,
	runID string,
	logger *slog.Logger,
) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		db:        db,
		events:    storage.NewEventRepository(db),
		scenarios: storage.NewScenarioRepository(db),
		replays:   storage.NewReplayRunRepository(db),
		faults:    storage.NewFaultRuleRepository(db),
		services:  storage.NewServiceRepository(db),
		bus:       bus,
		version:   version,
		runID:     runID,
		startAt:   time.Now().UTC(),
		logger:    logger,
		sseClients: make(map[string]chan []byte),
	}

	s.routes()
	s.subscribeToEvents()

	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/events", s.handleListEvents)
	s.mux.HandleFunc("GET /api/events/stream", s.handleEventStream)
	s.mux.HandleFunc("GET /api/events/{id}", s.handleGetEvent)
	s.mux.HandleFunc("GET /api/services", s.handleListServices)
	s.mux.HandleFunc("GET /api/scenarios", s.handleListScenarios)
	s.mux.HandleFunc("GET /api/scenarios/{id}", s.handleGetScenario)
	s.mux.HandleFunc("POST /api/scenarios/start", s.handleStartRecording)
	s.mux.HandleFunc("POST /api/scenarios/stop", s.handleStopRecording)
	s.mux.HandleFunc("POST /api/scenarios/{id}/replay", s.handleStartReplay)
	s.mux.HandleFunc("POST /api/scenarios/{id}/export", s.handleExportScenario)
	s.mux.HandleFunc("GET /api/fault-rules", s.handleListFaultRules)
	s.mux.HandleFunc("POST /api/fault-rules", s.handleCreateFaultRule)
	s.mux.HandleFunc("PATCH /api/fault-rules/{id}", s.handleUpdateFaultRule)
	s.mux.HandleFunc("DELETE /api/fault-rules/{id}", s.handleDeleteFaultRule)
	s.mux.HandleFunc("GET /api/replay-runs/{id}", s.handleGetReplayRun)
}

// ListenAndServe starts the server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.corsMiddleware(s.mux),
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("api: listen %s: %w", addr, err)
	}
	s.logger.Info("studio api listening", "addr", addr)
	return s.server.Serve(ln)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// Handler returns the underlying http.Handler for testing.
func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s.mux)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isLocalOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalOrigin(origin string) bool {
	for _, local := range []string{"localhost", "127.0.0.1", "::1", "[::1]"} {
		if strings.Contains(origin, local) {
			return true
		}
	}
	return false
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"version":   s.version,
		"runId":     s.runID,
		"startedAt": s.startAt.Format(time.RFC3339),
		"database":  "ok",
		"liveStream": "ok",
	})
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := intQuery(r, "limit", 100)
	cursor := int64Query(r, "cursor", 0)

	events, err := s.events.ListByTimestamp(ctx, limit, cursor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_query_failed", err.Error())
		return
	}

	var nextCursor string
	if len(events) == limit {
		last := events[len(events)-1]
		nextCursor = fmt.Sprintf("%d:%s", last.Timestamp.UnixMilli(), last.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":      events,
		"nextCursor": nextCursor,
	})
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	event, err := s.events.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_query_failed", err.Error())
		return
	}
	if event == nil {
		writeError(w, http.StatusNotFound, "event_not_found", "event not found")
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.services.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"services": services,
	})
}

func (s *Server) handleListScenarios(w http.ResponseWriter, r *http.Request) {
	scenarios, err := s.scenarios.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": scenarios,
	})
}

func (s *Server) handleGetScenario(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	scenario, err := s.scenarios.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_query_failed", err.Error())
		return
	}
	if scenario == nil {
		writeError(w, http.StatusNotFound, "scenario_not_found", "scenario not found")
		return
	}

	events, _ := s.events.ListByScenario(r.Context(), id)
	runs, _ := s.replays.ListByScenario(r.Context(), id)

	writeJSON(w, http.StatusOK, map[string]any{
		"scenario":   scenario,
		"events":     events,
		"replayRuns": runs,
	})
}

func (s *Server) handleListFaultRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.faults.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": rules,
	})
}

func (s *Server) handleGetReplayRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.replays.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_query_failed", err.Error())
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "replay_run_not_found", "replay run not found")
		return
	}

	originalEvents, _ := s.events.ListByScenario(r.Context(), run.ScenarioID)
	replayEvents, _ := s.events.ListByReplayRun(r.Context(), id)

	writeJSON(w, http.StatusOK, map[string]any{
		"run":            run,
		"originalEvents": originalEvents,
		"replayEvents":   replayEvents,
	})
}

// --- SSE ---

func (s *Server) handleEventStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported", "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := make(chan []byte, 64)
	clientID := s.addSSEClient(ch)
	defer s.removeSSEClient(clientID)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			fmt.Fprintf(w, "event: timeline.event\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) addSSEClient(ch chan []byte) string {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	s.sseNextID++
	id := fmt.Sprintf("sse_%d", s.sseNextID)
	s.sseClients[id] = ch
	return id
}

func (s *Server) removeSSEClient(id string) {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	delete(s.sseClients, id)
}

func (s *Server) broadcastSSE(data []byte) {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	for _, ch := range s.sseClients {
		select {
		case ch <- data:
		default:
			// drop if client is slow
		}
	}
}

func (s *Server) subscribeToEvents() {
	s.bus.Subscribe(func(ctx context.Context, event timeline.TimelineEvent) {
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		s.broadcastSSE(data)
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func intQuery(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func int64Query(r *http.Request, key string, def int64) int64 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
