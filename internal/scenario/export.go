package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/localcloud-dev/localcloud/internal/redaction"
	"github.com/localcloud-dev/localcloud/internal/storage"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// ExportFormat is the portable scenario export envelope.
type ExportFormat struct {
	Format          string                   `json:"format"`
	Version         string                   `json:"version"`
	ExportedAt      time.Time                `json:"exportedAt"`
	Scenario        timeline.Scenario        `json:"scenario"`
	Events          []timeline.TimelineEvent `json:"events"`
	RedactionReport RedactionReport          `json:"redactionReport"`
}

// RedactionReport summarizes redaction applied during export.
type RedactionReport struct {
	HeadersRedacted int      `json:"headersRedacted"`
	BodiesRedacted  int      `json:"bodiesRedacted"`
	Warnings        []string `json:"warnings"`
	Safe            bool     `json:"safe"`
}

// Export generates a portable JSON export for a completed scenario.
func Export(ctx context.Context, db *storage.DB, scenarioID, lcVersion string) (*ExportFormat, error) {
	scenarioRepo := storage.NewScenarioRepository(db)
	eventRepo := storage.NewEventRepository(db)

	s, err := scenarioRepo.GetByID(ctx, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("export: get scenario: %w", err)
	}
	if s == nil {
		return nil, fmt.Errorf("export: scenario %q not found", scenarioID)
	}
	if s.Status == timeline.ScenarioStatusRecording {
		return nil, fmt.Errorf("export: cannot export while recording is active")
	}

	events, err := eventRepo.ListByScenario(ctx, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("export: list events: %w", err)
	}

	// Run redaction scan on all events
	report := scanRedaction(events)

	return &ExportFormat{
		Format:          "localcloud.scenario.v1",
		Version:         lcVersion,
		ExportedAt:      time.Now().UTC(),
		Scenario:        *s,
		Events:          events,
		RedactionReport: report,
	}, nil
}

// ExportJSON returns the export as formatted JSON bytes.
func ExportJSON(ctx context.Context, db *storage.DB, scenarioID, lcVersion string) ([]byte, error) {
	export, err := Export(ctx, db, scenarioID, lcVersion)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(export, "", "  ")
}

func scanRedaction(events []timeline.TimelineEvent) RedactionReport {
	report := RedactionReport{Safe: true}

	for _, e := range events {
		// Scan request headers and body
		if e.Request != nil {
			if e.Request.BodyRedacted {
				report.BodiesRedacted++
			}
			for _, v := range e.Request.Headers {
				if v == "[REDACTED]" {
					report.HeadersRedacted++
				}
			}
			// Check for unredacted secrets
			if e.Request.BodyPreview != "" {
				warnings := redaction.ScanForSecrets(e.Request.BodyPreview)
				if len(warnings) > 0 {
					for _, w := range warnings {
						report.Warnings = append(report.Warnings,
							fmt.Sprintf("event %s request body: %s", e.ID, w))
					}
					report.Safe = false
				}
			}
		}

		// Scan response
		if e.Response != nil {
			if e.Response.BodyRedacted {
				report.BodiesRedacted++
			}
			for _, v := range e.Response.Headers {
				if v == "[REDACTED]" {
					report.HeadersRedacted++
				}
			}
		}

		// Scan raw payload
		if e.RawPayload != nil && !e.RawPayload.Redacted && e.RawPayload.Preview != "" {
			warnings := redaction.ScanForSecrets(e.RawPayload.Preview)
			if len(warnings) > 0 {
				for _, w := range warnings {
					report.Warnings = append(report.Warnings,
						fmt.Sprintf("event %s raw payload: %s", e.ID, w))
				}
				report.Safe = false
			}
		}
	}

	return report
}
