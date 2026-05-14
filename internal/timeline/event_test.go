package timeline

import (
	"strings"
	"testing"
	"time"
)

func validEvent() TimelineEvent {
	return TimelineEvent{
		ID:        "evt_01H000000000000000000001",
		RunID:     "run_01H000000000000000000001",
		Timestamp: time.Now().UTC(),
		Source:    SourceHTTPProxy,
		Service:   "api",
		Action:    ActionHTTPRequest,
		Summary:   "POST /signup",
		Status:    StatusOK,
	}
}

func TestValidateValidEvent(t *testing.T) {
	e := validEvent()
	if err := e.Validate(); err != nil {
		t.Fatalf("expected valid event, got error: %v", err)
	}
}

func TestValidateMissingID(t *testing.T) {
	e := validEvent()
	e.ID = ""
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMissingRunID(t *testing.T) {
	e := validEvent()
	e.RunID = ""
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error for missing runId")
	}
	if !strings.Contains(err.Error(), "runId is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateZeroTimestamp(t *testing.T) {
	e := validEvent()
	e.Timestamp = time.Time{}
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error for zero timestamp")
	}
}

func TestValidateMissingSource(t *testing.T) {
	e := validEvent()
	e.Source = ""
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestValidateMissingService(t *testing.T) {
	e := validEvent()
	e.Service = ""
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error for missing service")
	}
}

func TestValidateMissingAction(t *testing.T) {
	e := validEvent()
	e.Action = ""
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error for missing action")
	}
}

func TestValidateMissingSummary(t *testing.T) {
	e := validEvent()
	e.Summary = ""
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error for missing summary")
	}
}

func TestValidateInvalidStatus(t *testing.T) {
	e := validEvent()
	e.Status = "bogus"
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	e := TimelineEvent{}
	err := e.Validate()
	if err == nil {
		t.Fatal("expected multiple errors")
	}
	// Should report at least id, runId, timestamp, source, service, action, summary, status
	errStr := err.Error()
	for _, field := range []string{"id", "runId", "timestamp", "source", "service", "action", "summary", "status"} {
		if !strings.Contains(errStr, field) {
			t.Errorf("expected error for %s, not found in: %s", field, errStr)
		}
	}
}

func TestValidateAllStatuses(t *testing.T) {
	for _, s := range []string{StatusOK, StatusError, StatusWarning, StatusPending, StatusBlocked, StatusFaulted, StatusUnknown} {
		e := validEvent()
		e.Status = s
		if err := e.Validate(); err != nil {
			t.Errorf("status %q should be valid, got: %v", s, err)
		}
	}
}
