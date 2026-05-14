package eventbus

import (
	"context"
	"sync"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// Subscriber is a function that receives timeline events.
type Subscriber func(ctx context.Context, event timeline.TimelineEvent)

// Bus is an in-process event fanout for timeline events and status updates.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string]Subscriber
	nextID      int
}

// New creates a new event bus.
func New() *Bus {
	return &Bus{
		subscribers: make(map[string]Subscriber),
	}
}

// Subscribe registers a handler and returns an unsubscribe ID.
func (b *Bus) Subscribe(fn Subscriber) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := string(rune(b.nextID + 64)) // simple ID
	// Use a more stable ID scheme
	id = "sub_" + itoa(b.nextID)
	b.subscribers[id] = fn
	return id
}

// Unsubscribe removes a subscriber by ID.
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, id)
}

// Publish sends an event to all current subscribers.
// Subscriber panics are recovered to avoid crashing the bus.
func (b *Bus) Publish(ctx context.Context, event timeline.TimelineEvent) {
	b.mu.RLock()
	subs := make([]Subscriber, 0, len(b.subscribers))
	for _, fn := range b.subscribers {
		subs = append(subs, fn)
	}
	b.mu.RUnlock()

	for _, fn := range subs {
		func() {
			defer func() { recover() }()
			fn(ctx, event)
		}()
	}
}

// SubscriberCount returns the current number of subscribers.
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
