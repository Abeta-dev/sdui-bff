package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/analytics"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/usecase"
)

type stubAnalyticsStore struct {
	recorded analytics.BatchRequest
	resp     analytics.IngestResponse
	summary  analytics.Summary
	err      error
}

func (s *stubAnalyticsStore) RecordBatch(_ context.Context, req analytics.BatchRequest) (analytics.IngestResponse, error) {
	s.recorded = req
	return s.resp, s.err
}

func (s *stubAnalyticsStore) GetSummary(_ context.Context) (analytics.Summary, error) {
	return s.summary, s.err
}

func TestAnalyticsService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil store returns error", func(t *testing.T) {
		t.Parallel()
		nilSvc := usecase.NewAnalyticsService(nil)
		if _, err := nilSvc.IngestBatch(ctx, analytics.BatchRequest{
			Events: []analytics.Event{{EventName: analytics.EventPageOpen}},
		}); err == nil {
			t.Fatal("expected error for nil store IngestBatch")
		}
		if _, err := nilSvc.Summary(ctx); err == nil {
			t.Fatal("expected error for nil store Summary")
		}
	})

	t.Run("cancelled context returns context.Canceled", func(t *testing.T) {
		t.Parallel()
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		svc := usecase.NewAnalyticsService(&stubAnalyticsStore{})
		if _, err := svc.IngestBatch(cctx, analytics.BatchRequest{
			Events: []analytics.Event{{EventName: analytics.EventPageOpen}},
		}); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled on IngestBatch, got %v", err)
		}
		if _, err := svc.Summary(cctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled on Summary, got %v", err)
		}
	})

	t.Run("validation errors wrap ErrInvalidInput", func(t *testing.T) {
		t.Parallel()
		svc := usecase.NewAnalyticsService(&stubAnalyticsStore{})

		// Empty batch
		if _, err := svc.IngestBatch(ctx, analytics.BatchRequest{}); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput on empty batch, got %v", err)
		}
		// Empty event_name
		if _, err := svc.IngestBatch(ctx, analytics.BatchRequest{
			Events: []analytics.Event{{EventName: "  "}},
		}); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput on blank event_name, got %v", err)
		}
		// Negative timestamp
		if _, err := svc.IngestBatch(ctx, analytics.BatchRequest{
			Events: []analytics.Event{{EventName: analytics.EventPageOpen, TimestampEpochMs: -1}},
		}); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput on negative timestamp, got %v", err)
		}
		// Batch > MaxBatchSize
		oversized := make([]analytics.Event, analytics.MaxBatchSize+1)
		for i := range oversized {
			oversized[i] = analytics.Event{EventName: analytics.EventTrackEvent}
		}
		if _, err := svc.IngestBatch(ctx, analytics.BatchRequest{Events: oversized}); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput on oversized batch, got %v", err)
		}
	})

	t.Run("enriches missing fields, defaults categories, and delegates summary", func(t *testing.T) {
		t.Parallel()
		store := &stubAnalyticsStore{
			resp: analytics.IngestResponse{Accepted: 4, Dropped: 0, ServerTimestampEpochMs: 123456},
			summary: analytics.Summary{
				TotalEvents: 4,
				ByEventName: map[string]int64{
					analytics.EventPageOpen:          1,
					analytics.EventSnippetImpression: 1,
					analytics.EventNodeImpression:    1,
					analytics.EventInteractionClick:  1,
				},
			},
		}
		svc := usecase.NewAnalyticsService(store)

		srcProps := map[string]string{"sku": "avocados"}
		resp, err := svc.IngestBatch(ctx, analytics.BatchRequest{
			SessionID:  "sess_42",
			UserID:     "usr_7",
			Platform:   "ios",
			AppVersion: "0.6.0",
			Events: []analytics.Event{
				{EventName: " page_open ", PageID: " fresh-home "},
				{EventName: analytics.EventSnippetImpression, PageID: "fresh-home", SnippetID: " fresh_search_bar "},
				{EventName: analytics.EventNodeImpression, ComponentID: " tile_avocados ", Properties: srcProps},
				{EventName: analytics.EventInteractionClick, EventID: "explicit_click_1", Category: "custom", TimestampEpochMs: 99999},
			},
		})
		if err != nil {
			t.Fatalf("IngestBatch failed: %v", err)
		}
		if resp.Accepted != 4 || resp.Dropped != 0 {
			t.Fatalf("unexpected IngestResponse: %+v", resp)
		}
		srcProps["sku"] = "mutated"

		if len(store.recorded.Events) != 4 {
			t.Fatalf("expected 4 enriched events, got %d", len(store.recorded.Events))
		}
		e0 := store.recorded.Events[0]
		if e0.EventID == "" || e0.TimestampEpochMs == 0 || e0.EventName != analytics.EventPageOpen || e0.PageID != "fresh-home" || e0.Category != analytics.CategoryScreen {
			t.Fatalf("unexpected enriched event[0]: %+v", e0)
		}
		e1 := store.recorded.Events[1]
		if e1.Category != analytics.CategoryImpression || e1.SnippetID != "fresh_search_bar" {
			t.Fatalf("unexpected enriched event[1]: %+v", e1)
		}
		e2 := store.recorded.Events[2]
		if e2.Category != analytics.CategoryImpression || e2.ComponentID != "tile_avocados" || e2.Properties["sku"] != "avocados" {
			t.Fatalf("unexpected enriched event[2]: %+v", e2)
		}
		e3 := store.recorded.Events[3]
		if e3.EventID != "explicit_click_1" || e3.Category != "custom" || e3.TimestampEpochMs != 99999 {
			t.Fatalf("expected explicit fields preserved on event[3]: %+v", e3)
		}

		// Default category for interaction_click / track_event is action.
		if _, err := svc.IngestBatch(ctx, analytics.BatchRequest{
			Events: []analytics.Event{{EventName: analytics.EventInteractionClick}},
		}); err != nil {
			t.Fatalf("IngestBatch interaction_click failed: %v", err)
		}
		if store.recorded.Events[0].Category != analytics.CategoryAction {
			t.Fatalf("expected default category action, got %q", store.recorded.Events[0].Category)
		}

		// Summary delegation
		sum, err := svc.Summary(ctx)
		if err != nil {
			t.Fatalf("Summary failed: %v", err)
		}
		if sum.TotalEvents != 4 {
			t.Fatalf("expected TotalEvents=4, got %d", sum.TotalEvents)
		}
	})
}
