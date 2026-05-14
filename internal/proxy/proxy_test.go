package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// testSink collects emitted events for assertion.
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

func TestProxyCapture(t *testing.T) {
	// Start a fake upstream
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		fmt.Fprint(w, `{"id":1,"email":"test@example.test"}`)
	}))
	defer upstream.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sink := &testSink{}

	p, err := New(Config{
		ServiceName:   "api",
		TargetBaseURL: upstream.URL,
		ListenAddr:    "127.0.0.1:0",
		RunID:         "run_test",
	}, logger)
	if err != nil {
		t.Fatal(err)
	}

	// Start proxy on random port
	ctx := context.Background()
	if err := p.Start(ctx, sink); err != nil {
		t.Fatal(err)
	}
	defer p.Stop(ctx)

	proxyURL := fmt.Sprintf("http://%s", p.Addr())

	// Send request through proxy
	body := strings.NewReader(`{"email":"test@example.test","name":"Test","password":"secret123"}`)
	resp, err := http.Post(proxyURL+"/signup", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// Wait briefly for async emit
	time.Sleep(100 * time.Millisecond)

	events := sink.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Source != timeline.SourceHTTPProxy {
		t.Fatalf("expected source http_proxy, got %s", event.Source)
	}
	if event.Service != "api" {
		t.Fatalf("expected service api, got %s", event.Service)
	}
	if event.Request == nil {
		t.Fatal("expected request data")
	}
	if event.Request.Method != "POST" {
		t.Fatalf("expected POST, got %s", event.Request.Method)
	}
	if event.Request.Path != "/signup" {
		t.Fatalf("expected /signup, got %s", event.Request.Path)
	}
	if event.Response == nil {
		t.Fatal("expected response data")
	}
	if event.Response.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", event.Response.StatusCode)
	}
	if event.CorrelationID == nil || *event.CorrelationID == "" {
		t.Fatal("expected correlation ID to be injected")
	}
}

func TestProxyRedactsPassword(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer upstream.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sink := &testSink{}

	p, err := New(Config{
		ServiceName:   "api",
		TargetBaseURL: upstream.URL,
		ListenAddr:    "127.0.0.1:0",
		RunID:         "run_test",
	}, logger)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := p.Start(ctx, sink); err != nil {
		t.Fatal(err)
	}
	defer p.Stop(ctx)

	proxyURL := fmt.Sprintf("http://%s", p.Addr())
	body := strings.NewReader(`{"email":"a@b.test","password":"supersecret"}`)
	resp, err := http.Post(proxyURL+"/test", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	events := sink.getEvents()
	if len(events) == 0 {
		t.Fatal("expected event")
	}

	preview := events[0].Request.BodyPreview
	if strings.Contains(preview, "supersecret") {
		t.Fatalf("password not redacted in preview: %s", preview)
	}
	if !events[0].Request.BodyRedacted {
		t.Fatal("expected bodyRedacted=true")
	}
}
