package replay

import (
	"testing"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

func TestIsSafeMethod(t *testing.T) {
	safe := []string{"GET", "HEAD", "OPTIONS", "get", "Head"}
	for _, m := range safe {
		if !isSafeMethod(m) {
			t.Errorf("expected %s to be safe", m)
		}
	}

	unsafe := []string{"POST", "PUT", "DELETE", "PATCH", "post"}
	for _, m := range unsafe {
		if isSafeMethod(m) {
			t.Errorf("expected %s to be unsafe", m)
		}
	}
}

func TestCopyHeaders(t *testing.T) {
	orig := map[string]string{"A": "1", "B": "2"}
	cp := copyHeaders(orig)

	if len(cp) != 2 || cp["A"] != "1" || cp["B"] != "2" {
		t.Error("copy mismatch")
	}

	// Mutation should not affect original
	cp["A"] = "modified"
	if orig["A"] != "1" {
		t.Error("mutation leaked to original")
	}
}

func TestCopyHeadersNil(t *testing.T) {
	if copyHeaders(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestPlanEntryFromEvent(t *testing.T) {
	event := timeline.TimelineEvent{
		ID:     "evt_test",
		Source: timeline.SourceHTTPProxy,
		Request: &timeline.RequestData{
			Method:     "POST",
			Path:       "/signup",
			Headers:    map[string]string{"Content-Type": "application/json"},
			Replayable: true,
		},
		Response: &timeline.ResponseData{
			StatusCode: 201,
		},
	}

	entry := PlanEntry{
		OriginalEventID: event.ID,
		Method:          event.Request.Method,
		Path:            event.Request.Path,
		Headers:         copyHeaders(event.Request.Headers),
		OriginalStatus:  event.Response.StatusCode,
		Safe:            isSafeMethod(event.Request.Method),
		OriginalEvent:   &event,
	}

	if entry.Safe {
		t.Error("POST should not be safe")
	}
	if entry.OriginalStatus != 201 {
		t.Errorf("expected status 201, got %d", entry.OriginalStatus)
	}
	if entry.Method != "POST" {
		t.Errorf("expected POST, got %s", entry.Method)
	}
}
