package analyticsstore_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/analytics"
	"github.com/abeta-dev/sdui-bff/internal/infra/analyticsstore"
)

func TestInMemoryStore_CountersRingBufferAndConcurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Default 100-event ring buffer when maxRecent <= 0.
	store := analyticsstore.NewInMemoryStore(0)
	res, err := store.RecordBatch(ctx, analytics.BatchRequest{
		SessionID: "sess_1",
		Platform:  "jvm",
		Events: []analytics.Event{
			{
				EventID:     "e1",
				EventName:   analytics.EventPageOpen,
				Category:    analytics.CategoryScreen,
				PageID:      "fresh-home",
				ComponentID: "fresh_search_bar",
				Properties:  map[string]string{"city": "Bengaluru"},
			},
			{
				EventID:     "e2",
				EventName:   analytics.EventSnippetImpression,
				Category:    analytics.CategoryImpression,
				PageID:      "fresh-home",
				SnippetID:   "fresh_hero_media_block",
				ComponentID: "fresh_hero_media_block",
			},
			{
				EventID:   "e_dropped",
				EventName: "   ",
			},
		},
	})
	if err != nil {
		t.Fatalf("RecordBatch failed: %v", err)
	}
	if res.Accepted != 2 || res.Dropped != 1 || res.ServerTimestampEpochMs <= 0 {
		t.Fatalf("unexpected IngestResponse: %+v", res)
	}

	summary, err := store.GetSummary(ctx)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.TotalEvents != 2 ||
		summary.ByEventName[analytics.EventPageOpen] != 1 ||
		summary.ByEventName[analytics.EventSnippetImpression] != 1 ||
		summary.ByCategory[analytics.CategoryScreen] != 1 ||
		summary.ByCategory[analytics.CategoryImpression] != 1 ||
		summary.ByPageID["fresh-home"] != 2 ||
		summary.ByComponentID["fresh_search_bar"] != 1 ||
		summary.ByComponentID["fresh_hero_media_block"] != 1 ||
		len(summary.RecentEvents) != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	// Mutating returned summary maps/events must not corrupt internal store state.
	summary.ByEventName[analytics.EventPageOpen] = 999
	summary.RecentEvents[0].Properties["city"] = "Mutated"
	summary2, err := store.GetSummary(ctx)
	if err != nil {
		t.Fatalf("second GetSummary failed: %v", err)
	}
	if summary2.ByEventName[analytics.EventPageOpen] != 1 || summary2.RecentEvents[0].Properties["city"] != "Bengaluru" {
		t.Fatalf("store state mutated through summary snapshot: %+v", summary2)
	}

	// Ring buffer wraparound beyond custom capacity=3 preserves chronological order.
	ringStore := analyticsstore.NewInMemoryStore(3)
	for i := 1; i <= 5; i++ {
		if _, err := ringStore.RecordBatch(ctx, analytics.BatchRequest{
			Events: []analytics.Event{{
				EventID:     fmt.Sprintf("ev_%d", i),
				EventName:   analytics.EventNodeImpression,
				Category:    analytics.CategoryImpression,
				PageID:      "fresh-home",
				ComponentID: fmt.Sprintf("tile_%d", i),
			}},
		}); err != nil {
			t.Fatalf("RecordBatch %d failed: %v", i, err)
		}
	}
	ringSummary, err := ringStore.GetSummary(ctx)
	if err != nil {
		t.Fatalf("ring GetSummary failed: %v", err)
	}
	if ringSummary.TotalEvents != 5 || len(ringSummary.RecentEvents) != 3 {
		t.Fatalf("unexpected ring summary: %+v", ringSummary)
	}
	wantIDs := []string{"ev_3", "ev_4", "ev_5"}
	for i, want := range wantIDs {
		if ringSummary.RecentEvents[i].EventID != want {
			t.Errorf("RecentEvents[%d].EventID = %q, want %q", i, ringSummary.RecentEvents[i].EventID, want)
		}
	}

	// Verify default 100-event ring buffer caps at 100 while preserving total_events.
	capStore := analyticsstore.NewInMemoryStore(0)
	for i := range 125 {
		if _, err := capStore.RecordBatch(ctx, analytics.BatchRequest{
			Events: []analytics.Event{{
				EventID:   fmt.Sprintf("cap_%d", i),
				EventName: analytics.EventInteractionClick,
				Category:  analytics.CategoryAction,
				PageID:    "pay-home",
			}},
		}); err != nil {
			t.Fatalf("cap RecordBatch %d failed: %v", i, err)
		}
	}
	capSummary, err := capStore.GetSummary(ctx)
	if err != nil {
		t.Fatalf("cap GetSummary failed: %v", err)
	}
	if capSummary.TotalEvents != 125 || len(capSummary.RecentEvents) != analyticsstore.DefaultRingCapacity {
		t.Fatalf("expected total=125 and recent=100, got total=%d recent=%d", capSummary.TotalEvents, len(capSummary.RecentEvents))
	}
	if capSummary.RecentEvents[0].EventID != "cap_25" || capSummary.RecentEvents[99].EventID != "cap_124" {
		t.Fatalf("unexpected ring boundaries: first=%q last=%q", capSummary.RecentEvents[0].EventID, capSummary.RecentEvents[99].EventID)
	}

	// Cancelled context is respected.
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := ringStore.RecordBatch(cancelledCtx, analytics.BatchRequest{
		Events: []analytics.Event{{EventName: analytics.EventTrackEvent}},
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on RecordBatch, got %v", err)
	}
	if _, err := ringStore.GetSummary(cancelledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on GetSummary, got %v", err)
	}

	// Concurrent reads and writes under -race.
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := range 10 {
				_, _ = store.RecordBatch(ctx, analytics.BatchRequest{
					Events: []analytics.Event{{
						EventID:     fmt.Sprintf("w%d_%d", worker, j),
						EventName:   analytics.EventInteractionClick,
						Category:    analytics.CategoryAction,
						PageID:      "fresh-home",
						ComponentID: "tile_avocados",
					}},
				})
				_, _ = store.GetSummary(ctx)
			}
		}(g)
	}
	wg.Wait()

	finalSummary, err := store.GetSummary(ctx)
	if err != nil {
		t.Fatalf("final GetSummary failed: %v", err)
	}
	if finalSummary.TotalEvents != 82 {
		t.Fatalf("expected 82 total events after concurrent batches, got %d", finalSummary.TotalEvents)
	}
}
