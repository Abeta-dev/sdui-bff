package pulse_test

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
	"github.com/abeta-dev/sdui-bff/internal/verticals/pulse"
)

func TestPulseDescriptorAndRouting(t *testing.T) {
	t.Parallel()

	mod := pulse.New()
	desc := mod.Descriptor()
	if desc.ID != "abeta-pulse" || desc.EntryScreen != "pulse-home" || desc.ThemePreset != "indigo" {
		t.Fatalf("unexpected descriptor: %+v", desc)
	}
	if !mod.CanHandleScreen("pulse-home") || !mod.CanHandleScreen("pulse-event-detail") {
		t.Fatalf("expected pulse-home and pulse-event-detail handled")
	}
	if mod.CanHandleScreen("fresh-home") {
		t.Fatalf("expected fresh-home not handled by pulse")
	}
	if _, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "unknown"}); !errors.Is(err, domain.ErrScreenNotFound) {
		t.Fatalf("expected ErrScreenNotFound, got %v", err)
	}
}

func TestPulseCategoryAndSearchFiltering(t *testing.T) {
	t.Parallel()

	mod := pulse.New()

	countGridEvents := func(page snippets.PageResponse) int {
		for _, env := range page.Snippets {
			if env.SnippetType == snippets.SnippetTypeGrid {
				grid, err := snippets.DecodeSnippetData[snippets.GridSnippetData](env)
				if err != nil {
					t.Fatalf("decode grid: %v", err)
				}
				return len(grid.Items)
			}
		}
		return 0
	}

	allPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pulse-home"})
	if err != nil {
		t.Fatalf("RenderScreen(all) error: %v", err)
	}
	if got := countGridEvents(allPage); got != 6 {
		t.Fatalf("expected 6 events on pulse-home, got %d", got)
	}
	var foundNudge bool
	for _, env := range allPage.Snippets {
		if env.SnippetType == snippets.SnippetTypeNudgeBar {
			foundNudge = true
			nudge, decErr := snippets.DecodeSnippetData[snippets.NudgeBarSnippetData](env)
			if decErr != nil {
				t.Fatalf("decode pulse nudge_bar: %v", decErr)
			}
			if nudge.Countdown == nil || nudge.Countdown.RemainingSeconds != 540 || nudge.Countdown.Label != "Early-bird seats closing in" {
				t.Errorf("unexpected pulse nudge countdown: %+v", nudge.Countdown)
			}
			if nudge.Badge == nil || nudge.Badge.Motion == nil || nudge.Badge.Motion.LoopEffect != atoms.MotionLoopWiggle {
				t.Errorf("expected MotionLoopWiggle on pulse nudge badge, got %+v", nudge.Badge)
			}
		}
	}
	if !foundNudge {
		t.Fatalf("expected urgency nudge_bar on pulse-home")
	}
	assertAllTrackingPropsAreStrings(t, allPage)

	moviesPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pulse-home", Filter: "movies"})
	if err != nil {
		t.Fatalf("RenderScreen(movies) error: %v", err)
	}
	if got := countGridEvents(moviesPage); got != 2 {
		t.Fatalf("expected 2 movie events, got %d", got)
	}

	comedyPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pulse-home", Query: "stand-up"})
	if err != nil {
		t.Fatalf("RenderScreen(search stand-up) error: %v", err)
	}
	if got := countGridEvents(comedyPage); got != 1 {
		t.Fatalf("expected 1 stand-up event, got %d", got)
	}

	emptyPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pulse-home", Query: "nonexistent-festival-xyz"})
	if err != nil {
		t.Fatalf("RenderScreen(empty) error: %v", err)
	}
	var emptyFound bool
	for _, env := range emptyPage.Snippets {
		if env.SnippetType == snippets.SnippetTypeEmptyState {
			emptyFound = true
		}
	}
	if !emptyFound {
		t.Fatalf("expected empty_state snippet on zero matching events")
	}
}

func TestPulseEventDetailAndShareAction(t *testing.T) {
	t.Parallel()

	mod := pulse.New()
	page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "pulse-event-detail",
		Extra:    map[string]string{"id": "evt-synth-symphony"},
		City:     "Hyderabad",
	})
	if err != nil {
		t.Fatalf("RenderScreen(pulse-event-detail) error: %v", err)
	}
	if !strings.Contains(page.Title, "Midnight Synth Symphony") {
		t.Errorf("unexpected detail title: %q", page.Title)
	}
	if page.BottomBar == nil || page.BottomBar.PrimaryButton == nil || page.BottomBar.SecondaryButton == nil {
		t.Fatalf("expected BottomBarSpec with primary and secondary buttons, got %+v", page.BottomBar)
	}
	if page.BottomBar.SecondaryButton.OnClick == nil || page.BottomBar.SecondaryButton.OnClick.Type != actions.ActionShare {
		t.Fatalf("expected secondary button to trigger native share action")
	}

	var tierCount int
	var foundEPassPreview bool
	for _, env := range page.Snippets {
		if env.SnippetType == snippets.SnippetTypeComposedBlock {
			cb, err := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](env)
			if err != nil {
				t.Fatalf("decode pulse composed_block: %v", err)
			}
			if cb.RecipeName == "event_epass_preview" {
				foundEPassPreview = true
				rawCB, _ := json.Marshal(cb)
				for _, wantKind := range []string{
					snippets.ComposedNodeAlertBanner,
					snippets.ComposedNodeRating,
					snippets.ComposedNodeQrCode,
				} {
					if !strings.Contains(string(rawCB), `"`+wantKind+`"`) {
						t.Errorf("event_epass_preview missing composed node kind %q", wantKind)
					}
				}
			}
		}
		if env.SnippetType == snippets.SnippetTypeCard && strings.HasPrefix(env.SnippetID, "pulse_tier_") {
			tierCount++
			card, err := snippets.DecodeSnippetData[snippets.CardSnippetData](env)
			if err != nil {
				t.Fatalf("decode tier card: %v", err)
			}
			if card.PrimaryButton == nil || card.PrimaryButton.OnClick == nil {
				t.Fatalf("expected tier booking CTA on %s", env.SnippetID)
			}
			callAPI, err := actions.DecodePayload[actions.CallAPIPayload](*card.PrimaryButton.OnClick)
			if err != nil {
				t.Fatalf("decode call_api: %v", err)
			}
			if callAPI.Endpoint != "/api/v1/pulse/bookings" || callAPI.Method != "POST" {
				t.Errorf("unexpected tier call_api: %+v", callAPI)
			}
		}
	}
	if !foundEPassPreview {
		t.Errorf("expected event_epass_preview composed_block on pulse-event-detail")
	}
	if tierCount != 3 {
		t.Errorf("expected 3 seat tiers (Gold, VIP Lounge, Balcony), got %d", tierCount)
	}
	assertAllTrackingPropsAreStrings(t, page)
}

func TestPulseBookTicketsAndValidation(t *testing.T) {
	t.Parallel()

	mod := pulse.New()

	// Negative quantity rejected.
	_, err := mod.BookTickets(context.Background(), domain.BookingRequest{
		EventID:  "evt-orbital-imax",
		SeatTier: "Gold",
		Quantity: -2,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for negative quantity, got %v", err)
	}

	// Omitted quantity (0) defaults to 2.
	res, err := mod.BookTickets(context.Background(), domain.BookingRequest{
		EventID:  "evt-orbital-imax",
		SeatTier: "VIP Lounge",
		Quantity: 0,
		City:     "Bengaluru",
	})
	if err != nil {
		t.Fatalf("BookTickets error: %v", err)
	}
	bookingID, _ := res.Data["booking_id"].(string)
	if !strings.HasPrefix(bookingID, "PLS-") {
		t.Fatalf("expected PLS- booking ID, got %q", bookingID)
	}
	if qty, _ := res.Data["quantity"].(int); qty != 2 {
		t.Errorf("expected default quantity 2, got %v", res.Data["quantity"])
	}
	if mod.BookingCount() != 1 {
		t.Errorf("BookingCount() = %d, want 1", mod.BookingCount())
	}
	if len(res.FollowUpActions) != 2 {
		t.Fatalf("expected 2 follow_up_actions, got %d", len(res.FollowUpActions))
	}

	fb, err := actions.DecodePayload[actions.TriggerFeedbackPayload](res.FollowUpActions[0])
	if err != nil {
		t.Fatalf("decode trigger_feedback: %v", err)
	}
	if fb.Effect != actions.FeedbackCelebration || fb.Title != "Tickets Booked!" {
		t.Errorf("unexpected celebration feedback: %+v", fb)
	}

	toast, err := actions.DecodePayload[actions.ShowToastPayload](res.FollowUpActions[1])
	if err != nil {
		t.Fatalf("decode show_toast: %v", err)
	}
	if !strings.Contains(toast.Message, bookingID) || !strings.Contains(toast.Message, "e-pass ready!") {
		t.Errorf("unexpected toast message: %q", toast.Message)
	}

	// Verify pulse-home header displays booking count badge after booking.
	homeAfter, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pulse-home"})
	if err != nil {
		t.Fatalf("RenderScreen(pulse-home after booking) error: %v", err)
	}
	raw, _ := json.Marshal(homeAfter)
	if !strings.Contains(string(raw), "1 BOOKED") {
		t.Errorf("expected '1 BOOKED' badge on pulse-home after booking")
	}
	assertAllTrackingPropsAreStrings(t, homeAfter)
}

func assertAllTrackingPropsAreStrings(t *testing.T, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("unmarshal: %v", err)
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
		for _, item := range val {
			walkWireTree(t, item, path+"[]")
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
