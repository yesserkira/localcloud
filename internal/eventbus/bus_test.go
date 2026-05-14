package eventbus

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

func testEvent() timeline.TimelineEvent {
	return timeline.TimelineEvent{
		ID:        "evt_001",
		RunID:     "run_001",
		Timestamp: time.Now().UTC(),
		Source:    timeline.SourceHTTPProxy,
		Service:   "api",
		Action:    timeline.ActionHTTPRequest,
		Summary:   "POST /signup",
		Status:    timeline.StatusOK,
	}
}

func TestPublishAndSubscribe(t *testing.T) {
	bus := New()
	ctx := context.Background()

	var received timeline.TimelineEvent
	var wg sync.WaitGroup
	wg.Add(1)

	bus.Subscribe(func(ctx context.Context, event timeline.TimelineEvent) {
		received = event
		wg.Done()
	})

	bus.Publish(ctx, testEvent())
	wg.Wait()

	if received.ID != "evt_001" {
		t.Fatalf("expected evt_001, got %s", received.ID)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	bus := New()
	ctx := context.Background()

	count := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		bus.Subscribe(func(ctx context.Context, event timeline.TimelineEvent) {
			mu.Lock()
			count++
			mu.Unlock()
			wg.Done()
		})
	}

	bus.Publish(ctx, testEvent())
	wg.Wait()

	if count != 3 {
		t.Fatalf("expected 3 calls, got %d", count)
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := New()
	ctx := context.Background()

	called := false
	id := bus.Subscribe(func(ctx context.Context, event timeline.TimelineEvent) {
		called = true
	})

	bus.Unsubscribe(id)
	bus.Publish(ctx, testEvent())

	if called {
		t.Fatal("subscriber should not be called after unsubscribe")
	}
}

func TestSubscriberPanicRecovery(t *testing.T) {
	bus := New()
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(1)

	bus.Subscribe(func(ctx context.Context, event timeline.TimelineEvent) {
		panic("test panic")
	})
	bus.Subscribe(func(ctx context.Context, event timeline.TimelineEvent) {
		wg.Done()
	})

	bus.Publish(ctx, testEvent())
	wg.Wait()
	// If we reach here, panic didn't crash
}

func TestSubscriberCount(t *testing.T) {
	bus := New()
	if bus.SubscriberCount() != 0 {
		t.Fatal("expected 0")
	}
	id := bus.Subscribe(func(ctx context.Context, event timeline.TimelineEvent) {})
	if bus.SubscriberCount() != 1 {
		t.Fatal("expected 1")
	}
	bus.Unsubscribe(id)
	if bus.SubscriberCount() != 0 {
		t.Fatal("expected 0 after unsubscribe")
	}
}
