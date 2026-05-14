package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/localcloud-dev/localcloud/internal/fault"
	"github.com/localcloud-dev/localcloud/internal/id"
	"github.com/localcloud-dev/localcloud/internal/replay"
	"github.com/localcloud-dev/localcloud/internal/scenario"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// --- Scenario Recording ---

func (s *Server) handleStartRecording(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	// Check if already recording
	active, _ := s.scenarios.GetActiveRecording(r.Context())
	if active != nil {
		writeError(w, http.StatusConflict, "recording_active", "a recording is already active: "+active.Name)
		return
	}

	// Check name uniqueness
	existing, _ := s.scenarios.GetByName(r.Context(), body.Name)
	if existing != nil {
		writeError(w, http.StatusConflict, "name_exists", "scenario name already exists")
		return
	}

	now := time.Now().UTC()
	sc := &timeline.Scenario{
		ID:          id.Scenario(),
		Name:        body.Name,
		Description: body.Description,
		Status:      timeline.ScenarioStatusRecording,
		StartedAt:   now,
		Tags:        body.Tags,
		CreatedBy:   "api",
	}

	if err := s.scenarios.Insert(r.Context(), sc); err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}

	s.logger.Info("recording started via API", "scenario", sc.Name, "id", sc.ID)
	writeJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleStopRecording(w http.ResponseWriter, r *http.Request) {
	active, _ := s.scenarios.GetActiveRecording(r.Context())
	if active == nil {
		writeError(w, http.StatusNotFound, "no_recording", "no active recording")
		return
	}

	now := time.Now().UTC()
	events, _ := s.events.ListByScenario(r.Context(), active.ID)

	replayableCount := 0
	for _, e := range events {
		if e.Request != nil && e.Request.Replayable {
			replayableCount++
		}
	}

	err := s.scenarios.UpdateStatus(r.Context(), active.ID,
		timeline.ScenarioStatusCompleted, &now, len(events), replayableCount, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}

	sc, _ := s.scenarios.GetByID(r.Context(), active.ID)
	s.logger.Info("recording stopped via API", "scenario", active.Name, "events", len(events))
	writeJSON(w, http.StatusOK, sc)
}

// --- Replay ---

func (s *Server) handleStartReplay(w http.ResponseWriter, r *http.Request) {
	scenarioID := r.PathValue("id")

	var body struct {
		BaseURL       string `json:"baseUrl"`
		SkipUnsafe    bool   `json:"skipUnsafe"`
		ConfirmUnsafe bool   `json:"confirmUnsafe"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}
	if body.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "baseUrl is required")
		return
	}

	opts := replay.Options{
		BaseURL:       body.BaseURL,
		SkipUnsafe:    body.SkipUnsafe,
		ConfirmUnsafe: body.ConfirmUnsafe,
		Logger:        s.logger,
	}

	result, err := replay.Execute(r.Context(), s.db, scenarioID, opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, "replay_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// --- Export ---

func (s *Server) handleExportScenario(w http.ResponseWriter, r *http.Request) {
	scenarioID := r.PathValue("id")

	data, err := scenario.ExportJSON(r.Context(), s.db, scenarioID, s.version)
	if err != nil {
		writeError(w, http.StatusBadRequest, "export_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"scenario-"+scenarioID+".json\"")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// --- Fault Rules ---

func (s *Server) handleCreateFaultRule(w http.ResponseWriter, r *http.Request) {
	var rule timeline.FaultRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}

	// Validate
	errs := fault.ValidateRule(&rule)
	if len(errs) > 0 {
		writeError(w, http.StatusBadRequest, "validation_error", errs[0])
		return
	}

	now := time.Now().UTC()
	rule.ID = id.FaultRule()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	if err := s.faults.Insert(r.Context(), &rule); err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}

	s.logger.Info("fault rule created via API", "name", rule.Name, "kind", rule.Kind)
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleUpdateFaultRule(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("id")

	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}

	if body.Enabled != nil {
		if err := s.faults.SetEnabled(r.Context(), ruleID, *body.Enabled); err != nil {
			writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": ruleID, "updated": true})
}

func (s *Server) handleDeleteFaultRule(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("id")

	if err := s.faults.Delete(r.Context(), ruleID); err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": ruleID, "deleted": true})
}
