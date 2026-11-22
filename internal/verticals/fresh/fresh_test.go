package fresh_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/verticals/fresh"
)

func TestFreshDescriptorAndScreens(t *testing.T) {
	t.Parallel()

	mod := fresh.New()
	desc := mod.Descriptor()
	if desc.ID != "abeta-fresh" || desc.TenantID != "abeta-fresh" || desc.EntryScreen != "fresh-home" || desc.ThemePreset != "emerald" {
		t.Fatalf("unexpected descriptor: %+v", desc)
	}
	if !mod.CanHandleScreen("fresh-home") || !mod.CanHandleScreen("store-home") {
		t.Fatalf("expected fresh-home and store-home to be handled")
	}
	if mod.CanHandleScreen("bites-home") {
		t.Fatalf("expected bites-home not to be handled by fresh")
	}
	if len(mod.Screens()) != 2 {
		t.Fatalf("expected 2 screen summaries, got %d", len(mod.Screens()))
	}
}

func TestFreshRenderCatalogAndStoreAlias(t *testing.T) {
	t.Parallel()

	mod := fresh.New()
	for _, screenID := range []string{"fresh-home", "store-home"} {
		page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{
			ScreenID: screenID,
			City:     "Mumbai",
		})
		if err != nil {
			t.Fatalf("RenderScreen(%s) error: %v", screenID, err)
		}
		if page.PageID != screenID {
			t.Errorf("page_id = %q, want %q", page.PageID, screenID)
		}
		if page.AdaptiveLayout == nil || page.AdaptiveLayout.LayoutMode != snippets.LayoutModeCentered || page.AdaptiveLayout.MaxWidthDp == nil || *page.AdaptiveLayout.MaxWidthDp != 1080 {
			t.Fatalf("unexpected adaptive_layout: %+v", page.AdaptiveLayout)
		}
		if page.FloatingCart == nil || page.FloatingCart.Label != "Express Cart" {
			t.Fatalf("unexpected floating_cart: %+v", page.FloatingCart)
		}
		if page.FloatingCart.EtaBadge == nil || page.FloatingCart.EtaBadge.Label != "8 mins" {
			t.Fatalf("expected floating cart ETA badge '8 mins', got %+v", page.FloatingCart.EtaBadge)
		}
		if page.FloatingCart.FreeDeliveryThresholdCents == nil || *page.FloatingCart.FreeDeliveryThresholdCents != 1500 {
			t.Fatalf("expected FreeDeliveryThresholdCents=1500, got %+v", page.FloatingCart.FreeDeliveryThresholdCents)
		}
		if page.FloatingCart.NudgePrefix != "Add" || page.FloatingCart.UnlockedLabel != "FREE 8-min delivery unlocked!" {
			t.Fatalf("unexpected floating cart nudge labels: prefix=%q unlocked=%q", page.FloatingCart.NudgePrefix, page.FloatingCart.UnlockedLabel)
		}

		var gridFound, marqueeFound, nudgeFound, composedFound bool
		for _, env := range page.Snippets {
			switch env.SnippetType {
			case snippets.SnippetTypeHeader:
				hdr, err := snippets.DecodeSnippetData[snippets.HeaderSnippetData](env)
				if err != nil {
					t.Fatalf("decode header: %v", err)
				}
				if hdr.TrailingTag == nil || hdr.TrailingTag.Label != "Mumbai" {
					t.Errorf("expected city badge Mumbai, got %+v", hdr.TrailingTag)
				}
			case snippets.SnippetTypeMarqueeStrip:
				marqueeFound = true
				mq, err := snippets.DecodeSnippetData[snippets.MarqueeStripSnippetData](env)
				if err != nil {
					t.Fatalf("decode marquee_strip: %v", err)
				}
				if len(mq.Items) != 3 || mq.Items[0].Text != "8-min dispatch" || mq.Items[1].Text != "Free delivery over $15" || mq.Items[2].Text != "Zero surge pricing" {
					t.Errorf("unexpected fresh marquee_strip items: %+v", mq.Items)
				}
			case snippets.SnippetTypeNudgeBar:
				nudgeFound = true
				nudge, err := snippets.DecodeSnippetData[snippets.NudgeBarSnippetData](env)
				if err != nil {
					t.Fatalf("decode nudge_bar: %v", err)
				}
				if nudge.NudgeID != "fresh_flash_nudge" {
					t.Errorf("nudge_id = %q, want fresh_flash_nudge", nudge.NudgeID)
				}
				if nudge.Countdown == nil || nudge.Countdown.CountdownID != "fresh_drop_timer" || nudge.Countdown.RemainingSeconds != 420 || nudge.Countdown.Label != "Flash drop ends in" {
					t.Errorf("unexpected fresh nudge countdown: %+v", nudge.Countdown)
				}
				if nudge.Badge == nil || nudge.Badge.Motion == nil || nudge.Badge.Motion.LoopEffect != atoms.MotionLoopWiggle {
					t.Errorf("expected MotionLoopWiggle on fresh nudge badge, got %+v", nudge.Badge)
				}
			case snippets.SnippetTypeComposedBlock:
				composedFound = true
				cb, err := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](env)
				if err != nil {
					t.Fatalf("decode composed_block: %v", err)
				}
				if cb.RecipeName != "loyalty_stamp_card" || cb.BlockID != "abeta_fresh_club" {
					t.Errorf("unexpected fresh composed_block: %+v", cb)
				}
			case snippets.SnippetTypeGrid:
				gridFound = true
				grid, err := snippets.DecodeSnippetData[snippets.GridSnippetData](env)
				if err != nil {
					t.Fatalf("decode grid: %v", err)
				}
				if len(grid.Items) < 8 {
					t.Errorf("expected >= 8 catalog items, got %d", len(grid.Items))
				}
				if grid.Columns != 2 || grid.ColumnsByWidth == nil || *grid.ColumnsByWidth.Medium != 3 || *grid.ColumnsByWidth.Expanded != 4 {
					t.Errorf("unexpected grid responsive columns: %+v", grid.ColumnsByWidth)
				}
				firstProd, err := snippets.DecodeSnippetData[snippets.ProductCardSnippetData](grid.Items[0])
				if err != nil {
					t.Fatalf("decode product_card: %v", err)
				}
				if !strings.HasPrefix(firstProd.Stepper.FieldID, "cart_qty_") {
					t.Errorf("expected stepper field_id prefix cart_qty_, got %q", firstProd.Stepper.FieldID)
				}
			}
		}
		if !gridFound || !marqueeFound || !nudgeFound || !composedFound {
			t.Fatalf("missing expected Wave 6 snippets on %s: grid=%v marquee=%v nudge=%v composed=%v", screenID, gridFound, marqueeFound, nudgeFound, composedFound)
		}

		assertAllTrackingPropsAreStrings(t, page)
	}
}

func TestFreshSearchAndCategoryFilteringAndEmptyState(t *testing.T) {
	t.Parallel()

	mod := fresh.New()

	t.Run("category filter selects chip and filters grid", func(t *testing.T) {
		t.Parallel()
		page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{
			ScreenID: "fresh-home",
			Filter:   "dairy",
		})
		if err != nil {
			t.Fatalf("RenderScreen error: %v", err)
		}

		var selectedChip string
		var itemCount int
		for _, env := range page.Snippets {
			if env.SnippetType == snippets.SnippetTypeChipGroup {
				cg, err := snippets.DecodeSnippetData[snippets.ChipGroupSnippetData](env)
				if err != nil {
					t.Fatalf("decode chip_group: %v", err)
				}
				for _, chip := range cg.Chips {
					if chip.Selected {
						selectedChip = chip.ID
					}
				}
			}
			if env.SnippetType == snippets.SnippetTypeGrid {
				grid, err := snippets.DecodeSnippetData[snippets.GridSnippetData](env)
				if err != nil {
					t.Fatalf("decode grid: %v", err)
				}
				itemCount = len(grid.Items)
			}
		}
		if selectedChip != "dairy" {
			t.Errorf("selected chip = %q, want dairy", selectedChip)
		}
		if itemCount != 2 {
			t.Errorf("expected 2 dairy items, got %d", itemCount)
		}
	})

	t.Run("search query matches substring", func(t *testing.T) {
		t.Parallel()
		page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{
			ScreenID: "fresh-home",
			Query:    "avocado",
		})
		if err != nil {
			t.Fatalf("RenderScreen error: %v", err)
		}
		var itemCount int
		for _, env := range page.Snippets {
			if env.SnippetType == snippets.SnippetTypeSearchBar {
				sb, err := snippets.DecodeSnippetData[snippets.SearchBarSnippetData](env)
				if err != nil {
					t.Fatalf("decode search_bar: %v", err)
				}
				if sb.Query != "avocado" {
					t.Errorf("search_bar query = %q, want avocado", sb.Query)
				}
			}
			if env.SnippetType == snippets.SnippetTypeGrid {
				grid, _ := snippets.DecodeSnippetData[snippets.GridSnippetData](env)
				itemCount = len(grid.Items)
			}
		}
		if itemCount != 1 {
			t.Errorf("expected 1 avocado item, got %d", itemCount)
		}
	})

	t.Run("zero matches renders empty state with reset filters button", func(t *testing.T) {
		t.Parallel()
		page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{
			ScreenID: "fresh-home",
			Query:    "nonexistent-unicorn-fruit",
		})
		if err != nil {
			t.Fatalf("RenderScreen error: %v", err)
		}
		var emptyFound bool
		for _, env := range page.Snippets {
			if env.SnippetType == snippets.SnippetTypeGrid {
				t.Fatalf("did not expect grid when search has 0 matches")
			}
			if env.SnippetType == snippets.SnippetTypeEmptyState {
				emptyFound = true
				empty, err := snippets.DecodeSnippetData[snippets.EmptyStateSnippetData](env)
				if err != nil {
					t.Fatalf("decode empty_state: %v", err)
				}
				if empty.ActionButton == nil || empty.ActionButton.Label != "Reset Filters" {
					t.Errorf("expected Reset Filters button, got %+v", empty.ActionButton)
				}
			}
		}
		if !emptyFound {
			t.Fatalf("expected empty_state snippet on zero search results")
		}
	})

	t.Run("unknown screen returns ErrScreenNotFound", func(t *testing.T) {
		t.Parallel()
		_, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "does-not-exist"})
		if !errors.Is(err, domain.ErrScreenNotFound) {
			t.Fatalf("expected ErrScreenNotFound, got %v", err)
		}
	})
}

func TestFreshPlaceExpressOrder(t *testing.T) {
	t.Parallel()

	mod := fresh.New()
	res, err := mod.PlaceExpressOrder(context.Background(), domain.ExpressOrderRequest{
		Items:   map[string]int{"avocados": 2, "a2_milk": 1},
		Address: "Indiranagar, Bengaluru",
	})
	if err != nil {
		t.Fatalf("PlaceExpressOrder error: %v", err)
	}
	if res.Status != "ok" {
		t.Errorf("status = %q, want ok", res.Status)
	}
	orderID, _ := res.Data["order_id"].(string)
	if !strings.HasPrefix(orderID, "FRS-") {
		t.Errorf("expected order_id prefix FRS-, got %q", orderID)
	}
	if mod.OrderCount() != 1 {
		t.Errorf("orderCount = %d, want 1", mod.OrderCount())
	}
	if len(res.FollowUpActions) != 2 {
		t.Fatalf("expected 2 follow_up_actions, got %d", len(res.FollowUpActions))
	}
	if res.FollowUpActions[0].Type != actions.ActionTriggerFeedback {
		t.Errorf("first follow-up action type = %q, want trigger_feedback", res.FollowUpActions[0].Type)
	}
	fb, err := actions.DecodePayload[actions.TriggerFeedbackPayload](res.FollowUpActions[0])
	if err != nil {
		t.Fatalf("decode trigger_feedback: %v", err)
	}
	if fb.Effect != actions.FeedbackCelebration || fb.Title != "Fresh Order Confirmed!" || !strings.Contains(fb.Subtitle, orderID) {
		t.Errorf("unexpected feedback payload: %+v", fb)
	}
	if res.FollowUpActions[1].Type != actions.ActionShowToast {
		t.Errorf("second follow-up action type = %q, want show_toast", res.FollowUpActions[1].Type)
	}
	toast, err := actions.DecodePayload[actions.ShowToastPayload](res.FollowUpActions[1])
	if err != nil {
		t.Fatalf("decode show_toast: %v", err)
	}
	if toast.Variant != "success" {
		t.Errorf("toast variant = %q, want success", toast.Variant)
	}

	// Verify banner and loyalty stamp card reflect updated order counter (clamping at 5/5 reward unlock).
	_, _ = mod.PlaceExpressOrder(context.Background(), domain.ExpressOrderRequest{ItemCount: 1})
	_, _ = mod.PlaceExpressOrder(context.Background(), domain.ExpressOrderRequest{ItemCount: 1})
	page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "fresh-home"})
	if err != nil {
		t.Fatalf("RenderScreen after order error: %v", err)
	}
	rawPage, _ := json.Marshal(page)
	if !strings.Contains(string(rawPage), "Redeem Reward") {
		t.Errorf("expected loyalty composed_block to unlock Redeem Reward after 3 orders")
	}
	assertAllTrackingPropsAreStrings(t, page)

	// Reject negative item quantity.
	_, err = mod.PlaceExpressOrder(context.Background(), domain.ExpressOrderRequest{
		Items: map[string]int{"avocados": -1},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for negative quantity, got %v", err)
	}
}

// assertAllTrackingPropsAreStrings enforces the KMP wire rule that every
// TrackingMeta.properties and track_event payload.properties map contains string values only.
func assertAllTrackingPropsAreStrings(t *testing.T, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal for wire audit: %v", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("unmarshal for wire audit: %v", err)
	}
	walkWireTree(t, tree, "$")
}

func walkWireTree(t *testing.T, node any, path string) {
	t.Helper()
	switch val := node.(type) {
	case map[string]any:
		if tracking, ok := val["tracking"].(map[string]any); ok {
			checkStringMap(t, tracking["properties"], path+".tracking.properties")
		}
		if val["type"] == string(actions.ActionTrackEvent) {
			if payload, ok := val["payload"].(map[string]any); ok {
				checkStringMap(t, payload["properties"], path+".payload.properties")
			}
		}
		for k, child := range val {
			walkWireTree(t, child, path+"."+k)
		}
	case []any:
		for i, item := range val {
			walkWireTree(t, item, path+"["+string(rune('0'+i%10))+"]")
		}
	}
}

func checkStringMap(t *testing.T, raw any, path string) {
	t.Helper()
	if raw == nil {
		return
	}
	props, ok := raw.(map[string]any)
	if !ok {
		t.Errorf("%s is not a JSON object: %T", path, raw)
		return
	}
	for k, v := range props {
		if _, isStr := v.(string); !isStr {
			t.Errorf("KMP wire violation at %s.%s: expected string, got %T (%v)", path, k, v, v)
		}
	}
}
