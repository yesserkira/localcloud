package api

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/localcloud-dev/localcloud/internal/eventbus"
	"github.com/localcloud-dev/localcloud/internal/storage"
)

func testServer(t *testing.T) (*Server, *storage.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	bus := eventbus.New()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv := NewServer(db, bus, "0.1.0-test", "run_test", logger)
	return srv, db
}

func TestHealthEndpoint(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %v", body["status"])
	}
	if body["version"] != "0.1.0-test" {
		t.Fatalf("expected version, got %v", body["version"])
	}
}

func TestListEventsEmpty(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetEventNotFound(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/events/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListScenariosEmpty(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/scenarios", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListFaultRulesEmpty(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/fault-rules", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCORSLocalOrigin(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:41778")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:41778" {
		t.Fatal("expected CORS header for local origin")
	}
}

func TestCORSExternalOriginBlocked(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("should not set CORS for external origin")
	}
}

func TestOptionsPreflightReturns204(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("OPTIONS", "/api/events", nil)
	req.Header.Set("Origin", "http://localhost:41778")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}
