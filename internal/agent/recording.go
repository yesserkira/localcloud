package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/localcloud-dev/localcloud/internal/id"
	"github.com/localcloud-dev/localcloud/internal/storage"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// recording holds the active scenario recording state.
type recording struct {
	scenarioID string
	name       string
	startedAt  time.Time
}

// StartRecording begins a new scenario recording.
func (a *Agent) StartRecording(ctx context.Context, name, description string, tags []string, createdBy string) (*timeline.Scenario, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.activeRecording != nil {
		return nil, fmt.Errorf("recording already active: %s", a.activeRecording.name)
	}

	// Check name uniqueness
	scenarioRepo := storage.NewScenarioRepository(a.db)
	existing, err := scenarioRepo.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("agent: check scenario name: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("scenario %q already exists", name)
	}

	now := time.Now().UTC()
	scenario := &timeline.Scenario{
		ID:          id.Scenario(),
		Name:        name,
		Description: description,
		Status:      timeline.ScenarioStatusRecording,
		StartedAt:   now,
		Tags:        tags,
		CreatedBy:   createdBy,
	}

	if err := scenarioRepo.Insert(ctx, scenario); err != nil {
		return nil, fmt.Errorf("agent: insert scenario: %w", err)
	}

	a.activeRecording = &recording{
		scenarioID: scenario.ID,
		name:       name,
		startedAt:  now,
	}

	a.logger.Info("recording started", "scenario", name, "id", scenario.ID)
	return scenario, nil
}

// StopRecording stops the active scenario recording.
func (a *Agent) StopRecording(ctx context.Context) (*timeline.Scenario, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.activeRecording == nil {
		return nil, fmt.Errorf("no active recording")
	}

	scenarioRepo := storage.NewScenarioRepository(a.db)
	eventRepo := storage.NewEventRepository(a.db)

	scenarioID := a.activeRecording.scenarioID
	now := time.Now().UTC()

	// Get events that occurred during the recording window
	events, err := eventRepo.ListByScenario(ctx, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("agent: list scenario events: %w", err)
	}

	// Count replayable events
	replayableCount := 0
	var rootEventIDs []string
	for _, e := range events {
		if e.Request != nil && e.Request.Replayable {
			replayableCount++
			rootEventIDs = append(rootEventIDs, e.ID)
		}
	}

	err = scenarioRepo.UpdateStatus(ctx, scenarioID, timeline.ScenarioStatusCompleted,
		&now, len(events), replayableCount, "")
	if err != nil {
		return nil, fmt.Errorf("agent: update scenario: %w", err)
	}

	a.activeRecording = nil
	a.logger.Info("recording stopped", "scenario", scenarioID, "events", len(events))

	scenario, _ := scenarioRepo.GetByID(ctx, scenarioID)
	return scenario, nil
}

// ActiveRecording returns the current active recording scenario ID, or "".
func (a *Agent) ActiveRecording() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.activeRecording == nil {
		return ""
	}
	return a.activeRecording.scenarioID
}

// ActiveRecordingName returns the name of the active recording, or "".
func (a *Agent) ActiveRecordingName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.activeRecording == nil {
		return ""
	}
	return a.activeRecording.name
}
