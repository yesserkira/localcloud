package mailpit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

type testSink struct {
	mu     sync.Mutex
	events []timeline.TimelineEvent
}

func (s *testSink) Emit(_ context.Context, event timeline.TimelineEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *testSink) ReportStatus(_ context.Context, _ timeline.AdapterStatus) error {
	return nil
}

func (s *testSink) getEvents() []timeline.TimelineEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]timeline.TimelineEvent, len(s.events))
	copy(cp, s.events)
	return cp
}

func TestMailpitFetchNewMessages(t *testing.T) {
	now := time.Now().UTC()

	// Fake Mailpit API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := messagesResponse{
			Total: 1,
			Messages: []message{
				{
					ID:      "msg_001",
					From:    addressField{Name: "Demo", Address: "noreply@demo.localcloud.dev"},
					To:      []addressField{{Name: "Test", Address: "test@example.test"}},
					Subject: "Welcome to LocalCloud Demo, Test!",
					Created: now.Add(1 * time.Second),
					Size:    512,
					Snippet: "Thanks for signing up",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sink := &testSink{}

	a := New("mailpit", server.URL, "run_test", 1*time.Second, false, logger)
	a.sink = sink
	a.lastSeenTime = now

	a.fetchNewMessages(context.Background())

	events := sink.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Source != timeline.SourceMailpit {
		t.Fatalf("expected source mailpit, got %s", event.Source)
	}
	if event.Action != timeline.ActionEmailCaptured {
		t.Fatalf("expected email.captured, got %s", event.Action)
	}
	if event.RawPayload == nil || event.RawPayload.Preview == "" {
		t.Fatal("expected body preview")
	}
}

func TestMailpitRedactsBody(t *testing.T) {
	now := time.Now().UTC()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := messagesResponse{
			Total: 1,
			Messages: []message{
				{
					ID:      "msg_002",
					From:    addressField{Address: "noreply@demo.localcloud.dev"},
					To:      []addressField{{Address: "test@example.test"}},
					Subject: "Welcome",
					Created: now.Add(1 * time.Second),
					Snippet: "Secret body content",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sink := &testSink{}

	a := New("mailpit", server.URL, "run_test", 1*time.Second, true, logger)
	a.sink = sink
	a.lastSeenTime = now

	a.fetchNewMessages(context.Background())

	events := sink.getEvents()
	if len(events) != 1 {
		t.Fatal("expected 1 event")
	}

	if events[0].RawPayload.Preview != "[REDACTED]" {
		t.Fatalf("expected redacted body, got %s", events[0].RawPayload.Preview)
	}
}

func TestMailpitDeduplicates(t *testing.T) {
	now := time.Now().UTC()
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := messagesResponse{
			Total: 1,
			Messages: []message{
				{
					ID:      "msg_003",
					From:    addressField{Address: "noreply@demo.localcloud.dev"},
					To:      []addressField{{Address: "test@example.test"}},
					Subject: "Welcome",
					Created: now.Add(1 * time.Second),
					Snippet: "body",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sink := &testSink{}

	a := New("mailpit", server.URL, "run_test", 1*time.Second, false, logger)
	a.sink = sink
	a.lastSeenTime = now

	// First poll picks up message
	a.fetchNewMessages(context.Background())
	// Second poll should not duplicate
	a.fetchNewMessages(context.Background())

	events := sink.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event (deduped), got %d", len(events))
	}
}
