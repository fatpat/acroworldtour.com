package service

import (
	"sync"
	"sync/atomic"
	"time"
)

// Event tells live pages that something changed. Topic is
// "competition:<code>" or "season:<code>"; Kind is "flight", "run" or
// "competition".
type Event struct {
	Topic string
	Kind  string
	At    time.Time
}

// Event kinds.
const (
	KindFlight      = "flight"
	KindRun         = "run"
	KindCompetition = "competition"
)

// subscriberBuffer is the channel size of a topic subscriber; wildcard
// subscribers (the CDN purger) get a larger one.
const (
	subscriberBuffer = 16
	allBuffer        = 256
)

// Bus is an in-process publish/subscribe hub for live updates (SSE).
// Publish never blocks: an event is dropped for a subscriber whose buffer is
// full. For SSE clients that is harmless: every event only triggers a reload
// of the current state, so a later event (or the next page load) catches up.
type Bus struct {
	mu         sync.Mutex
	subs       map[string]map[chan Event]struct{}
	all        map[chan Event]struct{}
	dropped    atomic.Int64
	droppedAll atomic.Int64
}

// NewBus returns an empty bus.
func NewBus() *Bus {
	return &Bus{subs: map[string]map[chan Event]struct{}{}, all: map[chan Event]struct{}{}}
}

// Subscribe returns a channel receiving the events of topic and a function
// that unsubscribes and closes the channel (safe to call more than once).
func (b *Bus) Subscribe(topic string) (<-chan Event, func()) {
	ch := make(chan Event, subscriberBuffer)
	b.mu.Lock()
	if b.subs[topic] == nil {
		b.subs[topic] = map[chan Event]struct{}{}
	}
	b.subs[topic][ch] = struct{}{}
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs[topic], ch)
			if len(b.subs[topic]) == 0 {
				delete(b.subs, topic)
			}
			b.mu.Unlock()
			close(ch)
		})
	}
}

// SubscribeAll returns a channel receiving the events of every topic (the
// CDN purge subscriber) and a function that unsubscribes and closes it.
func (b *Bus) SubscribeAll() (<-chan Event, func()) {
	ch := make(chan Event, allBuffer)
	b.mu.Lock()
	b.all[ch] = struct{}{}
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.all, ch)
			b.mu.Unlock()
			close(ch)
		})
	}
}

// Publish sends e to every subscriber of e.Topic and to the wildcard
// subscribers without blocking.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.all {
		select {
		case ch <- e:
		default:
			b.droppedAll.Add(1)
		}
	}
	for ch := range b.subs[e.Topic] {
		select {
		case ch <- e:
		default:
			b.dropped.Add(1)
		}
	}
}

// DroppedAll is the number of events dropped for wildcard subscribers.
func (b *Bus) DroppedAll() int64 { return b.droppedAll.Load() }

// Dropped is the number of events dropped for slow subscribers.
func (b *Bus) Dropped() int64 { return b.dropped.Load() }

// publish sends one event per topic; it is called after commit only.
func (s *Service) publish(kind string, topics ...string) {
	if s.bus == nil {
		return
	}
	at := time.Now().UTC()
	for _, t := range topics {
		s.bus.Publish(Event{Topic: t, Kind: kind, At: at})
	}
}

func competitionTopic(code string) string { return "competition:" + code }

func seasonTopic(code string) string { return "season:" + code }
