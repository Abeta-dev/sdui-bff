package usecase

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync/atomic"
	"time"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/analytics"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

// Compile-time check ensuring AnalyticsService satisfies domain.AnalyticsService.
var _ domain.AnalyticsService = (*AnalyticsService)(nil)

// AnalyticsService validates, enriches, and persists batched client analytics events.
type AnalyticsService struct {
	store domain.AnalyticsStore
	seq   atomic.Uint64
}

// NewAnalyticsService constructs an AnalyticsService backed by store.
func NewAnalyticsService(store domain.AnalyticsStore) *AnalyticsService {
	return &AnalyticsService{store: store}
}

// IngestBatch validates a client BatchRequest, populates missing event_id,
// category, and timestamp_epoch_ms fields, and persists the batch via AnalyticsStore.
func (s *AnalyticsService) IngestBatch(ctx context.Context, req analytics.BatchRequest) (analytics.IngestResponse, error) {
	if err := ctx.Err(); err != nil {
		return analytics.IngestResponse{}, err
	}
	if s.store == nil {
		return analytics.IngestResponse{}, errors.New("analytics store is not configured")
	}
	if err := analytics.ValidateBatch(req); err != nil {
		return analytics.IngestResponse{}, fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}

	nowMs := time.Now().UnixMilli()
	enriched := make([]analytics.Event, len(req.Events))
	for i, ev := range req.Events {
		ev.EventName = strings.TrimSpace(ev.EventName)
		ev.EventID = strings.TrimSpace(ev.EventID)
		ev.Category = strings.TrimSpace(ev.Category)
		ev.PageID = strings.TrimSpace(ev.PageID)
		ev.SnippetID = strings.TrimSpace(ev.SnippetID)
		ev.ComponentID = strings.TrimSpace(ev.ComponentID)
		ev.ActionID = strings.TrimSpace(ev.ActionID)
		ev.ActionType = strings.TrimSpace(ev.ActionType)

		if ev.EventID == "" {
			ev.EventID = fmt.Sprintf("evt_%d_%d", nowMs, s.seq.Add(1))
		}
		if ev.TimestampEpochMs == 0 {
			ev.TimestampEpochMs = nowMs
		}
		if ev.Category == "" {
			ev.Category = defaultEventCategory(ev.EventName)
		}
		if len(ev.Properties) > 0 {
			props := make(map[string]string, len(ev.Properties))
			maps.Copy(props, ev.Properties)
			ev.Properties = props
		}
		enriched[i] = ev
	}

	enrichedReq := req
	enrichedReq.Events = enriched
	return s.store.RecordBatch(ctx, enrichedReq)
}

// Summary returns aggregated live analytics metrics from the underlying store.
func (s *AnalyticsService) Summary(ctx context.Context) (analytics.Summary, error) {
	if err := ctx.Err(); err != nil {
		return analytics.Summary{}, err
	}
	if s.store == nil {
		return analytics.Summary{}, errors.New("analytics store is not configured")
	}
	return s.store.GetSummary(ctx)
}

func defaultEventCategory(eventName string) string {
	switch {
	case eventName == analytics.EventPageOpen:
		return analytics.CategoryScreen
	case strings.HasSuffix(eventName, "_impression"):
		return analytics.CategoryImpression
	default:
		return analytics.CategoryAction
	}
}
