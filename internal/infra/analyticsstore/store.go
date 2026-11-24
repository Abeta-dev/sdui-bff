// Package analyticsstore provides a concurrency-safe in-memory AnalyticsStore
// tracking live SDUI event counters and a bounded recent-events ring buffer.
package analyticsstore

import (
	"context"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/analytics"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

// DefaultRingCapacity is the default number of recent events retained in memory.
const DefaultRingCapacity = 100

// Compile-time check ensuring InMemoryStore satisfies domain.AnalyticsStore.
var _ domain.AnalyticsStore = (*InMemoryStore)(nil)

// InMemoryStore is a thread-safe in-memory AnalyticsStore tracking counters by
// event_name, category, page_id, and component_id alongside a bounded circular
// ring buffer of recent events.
type InMemoryStore struct {
	mu            sync.RWMutex
	maxRecent     int
	totalEvents   int64
	byEventName   map[string]int64
	byCategory    map[string]int64
	byPageID      map[string]int64
	byComponentID map[string]int64
	ring          []analytics.Event
	head          int
	size          int
}

// NewInMemoryStore creates an InMemoryStore keeping at most maxRecent recent
// events (defaults to DefaultRingCapacity = 100 when maxRecent <= 0).
func NewInMemoryStore(maxRecent int) *InMemoryStore {
	if maxRecent <= 0 {
		maxRecent = DefaultRingCapacity
	}
	return &InMemoryStore{
		maxRecent:     maxRecent,
		byEventName:   make(map[string]int64),
		byCategory:    make(map[string]int64),
		byPageID:      make(map[string]int64),
		byComponentID: make(map[string]int64),
		ring:          make([]analytics.Event, maxRecent),
	}
}

// RecordBatch ingests all valid events in req, updates dimension counters,
// appends to the bounded ring buffer, and returns an IngestResponse.
func (s *InMemoryStore) RecordBatch(ctx context.Context, req analytics.BatchRequest) (analytics.IngestResponse, error) {
	if err := ctx.Err(); err != nil {
		return analytics.IngestResponse{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	accepted := 0
	dropped := 0
	for _, ev := range req.Events {
		if strings.TrimSpace(ev.EventName) == "" {
			dropped++
			continue
		}
		cloned := cloneEvent(ev)
		s.totalEvents++
		s.byEventName[cloned.EventName]++
		if cloned.Category != "" {
			s.byCategory[cloned.Category]++
		}
		if cloned.PageID != "" {
			s.byPageID[cloned.PageID]++
		}
		if cloned.ComponentID != "" {
			s.byComponentID[cloned.ComponentID]++
		}

		s.ring[s.head] = cloned
		s.head = (s.head + 1) % s.maxRecent
		if s.size < s.maxRecent {
			s.size++
		}
		accepted++
	}

	return analytics.IngestResponse{
		Accepted:               accepted,
		Dropped:                dropped,
		ServerTimestampEpochMs: time.Now().UnixMilli(),
	}, nil
}

// GetSummary returns a concurrency-safe snapshot of aggregated counters and
// the bounded ring buffer of recent events in chronological order.
func (s *InMemoryStore) GetSummary(ctx context.Context) (analytics.Summary, error) {
	if err := ctx.Err(); err != nil {
		return analytics.Summary{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	recent := make([]analytics.Event, 0, s.size)
	start := 0
	if s.size == s.maxRecent {
		start = s.head
	}
	for i := range s.size {
		idx := (start + i) % s.maxRecent
		recent = append(recent, cloneEvent(s.ring[idx]))
	}

	return analytics.Summary{
		TotalEvents:   s.totalEvents,
		ByEventName:   cloneCounterMap(s.byEventName),
		ByCategory:    cloneCounterMap(s.byCategory),
		ByPageID:      cloneCounterMap(s.byPageID),
		ByComponentID: cloneCounterMap(s.byComponentID),
		RecentEvents:  recent,
	}, nil
}

func cloneEvent(ev analytics.Event) analytics.Event {
	if len(ev.Properties) > 0 {
		props := make(map[string]string, len(ev.Properties))
		maps.Copy(props, ev.Properties)
		ev.Properties = props
	}
	return ev
}

func cloneCounterMap(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	maps.Copy(dst, src)
	return dst
}
