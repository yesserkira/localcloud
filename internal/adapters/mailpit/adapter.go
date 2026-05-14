package mailpit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/localcloud-dev/localcloud/internal/adapters"
	"github.com/localcloud-dev/localcloud/internal/id"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// Adapter captures emails from Mailpit via its REST API.
type Adapter struct {
	name         string
	apiURL       string
	runID        string
	pollInterval time.Duration
	redactBody   bool

	sink       adapters.EventSink
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	logger     *slog.Logger
	httpClient *http.Client

	mu           sync.RWMutex
	eventCount   int64
	status       string
	lastError    string
	lastSeenID   string
	lastSeenTime time.Time
}

// New creates a Mailpit adapter.
func New(name, apiURL, runID string, pollInterval time.Duration, redactBody bool, logger *slog.Logger) *Adapter {
	if pollInterval < 500*time.Millisecond {
		pollInterval = 1 * time.Second
	}
	return &Adapter{
		name:         name,
		apiURL:       strings.TrimRight(apiURL, "/"),
		runID:        runID,
		pollInterval: pollInterval,
		redactBody:   redactBody,
		status:       "stopped",
		logger:       logger,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (a *Adapter) Name() string { return a.name }

func (a *Adapter) Configure(_ context.Context, _ adapters.AdapterConfig) error {
	return nil
}

func (a *Adapter) Start(ctx context.Context, sink adapters.EventSink) error {
	a.sink = sink
	a.lastSeenTime = time.Now().UTC()

	pollCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.setStatus("running")

	a.wg.Add(1)
	go a.poll(pollCtx)

	a.logger.Info("mailpit adapter started", "apiUrl", a.apiURL, "interval", a.pollInterval)
	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	a.setStatus("stopped")
	a.logger.Info("mailpit adapter stopped")
	return nil
}

func (a *Adapter) Status(_ context.Context) timeline.AdapterStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return timeline.AdapterStatus{
		Adapter:    "mailpit",
		Service:    a.name,
		Enabled:    true,
		Status:     a.status,
		EventCount: a.eventCount,
		LastError:  a.lastError,
	}
}

func (a *Adapter) setStatus(s string) {
	a.mu.Lock()
	a.status = s
	a.mu.Unlock()
}

func (a *Adapter) poll(ctx context.Context) {
	defer a.wg.Done()
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.fetchNewMessages(ctx)
		}
	}
}

// Mailpit API response types
type messagesResponse struct {
	Total    int       `json:"total"`
	Messages []message `json:"messages"`
}

type message struct {
	ID      string        `json:"ID"`
	From    addressField  `json:"From"`
	To      []addressField `json:"To"`
	Subject string        `json:"Subject"`
	Created time.Time     `json:"Created"`
	Size    int           `json:"Size"`
	Snippet string        `json:"Snippet"`
}

type addressField struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

func (a *Adapter) fetchNewMessages(ctx context.Context) {
	url := fmt.Sprintf("%s/api/v1/messages?limit=50", a.apiURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		a.logger.Error("mailpit: create request", "err", err)
		return
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.logger.Error("mailpit: fetch messages", "err", err)
		a.mu.Lock()
		a.lastError = err.Error()
		a.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		a.logger.Error("mailpit: unexpected status", "status", resp.StatusCode, "body", string(body))
		return
	}

	var result messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		a.logger.Error("mailpit: decode response", "err", err)
		return
	}

	// Process new messages (newer than lastSeenTime)
	for i := len(result.Messages) - 1; i >= 0; i-- {
		msg := result.Messages[i]

		if !msg.Created.After(a.lastSeenTime) {
			continue
		}
		if msg.ID == a.lastSeenID {
			continue
		}

		a.emitMessage(ctx, msg)

		a.mu.Lock()
		a.lastSeenID = msg.ID
		a.lastSeenTime = msg.Created
		a.eventCount++
		a.mu.Unlock()
	}
}

func (a *Adapter) emitMessage(ctx context.Context, msg message) {
	to := make([]string, len(msg.To))
	for i, addr := range msg.To {
		to[i] = addr.Address
	}

	preview := msg.Snippet
	if a.redactBody {
		preview = "[REDACTED]"
	}

	metadata := map[string]any{
		"from":    msg.From.Address,
		"to":      to,
		"subject": msg.Subject,
		"size":    msg.Size,
	}

	event := timeline.TimelineEvent{
		ID:        id.Event(),
		RunID:     a.runID,
		Timestamp: msg.Created,
		Source:    timeline.SourceMailpit,
		Service:  a.name,
		Action:   timeline.ActionEmailCaptured,
		Summary:  fmt.Sprintf("email to %s: %s", strings.Join(to, ", "), msg.Subject),
		Status:   timeline.StatusOK,
		Metadata: metadata,
		RawPayload: &timeline.RawPayload{
			ContentType: "text/plain",
			Encoding:    "utf-8",
			Preview:     preview,
			Redacted:    a.redactBody,
		},
	}

	if err := a.sink.Emit(ctx, event); err != nil {
		a.logger.Error("mailpit: emit event", "err", err)
	}
}
