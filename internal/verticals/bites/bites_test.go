package bites_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/verticals/bites"
)

func TestBitesDescriptorAndRouting(t *testing.T) {
	t.Parallel()

	mod := bites.New()
	desc := mod.Descriptor()
	if desc.ID != "abeta-bites" || desc.EntryScreen != "bites-home" || desc.ThemePreset != "amber" {
		t.Fatalf("unexpected descriptor: %+v", desc)
	}
	for _, s := range []string{"bites-home", "bites-restaurant", "bites-order-track"} {
		if !mod.CanHandleScreen(s) {
			t.Errorf("expected CanHandleScreen(%q) == true", s)
		}
	}
	if mod.CanHandleScreen("pulse-home") {
		t.Errorf("expected pulse-home not handled")
	}
	if _, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "unknown"}); !errors.Is(err, domain.ErrScreenNotFound) {
		t.Fatalf("expected ErrScreenNotFound, got %v", err)
	}
}

func TestBitesFilteringPureVegTopRatedAndSearch(t *testing.T) {
	t.Parallel()

	mod := bites.New()

	countRestaurantCards := func(page snippets.PageResponse) int {
		n := 0
		for _, env := range page.Snippets {
			if env.SnippetType == snippets.SnippetTypeCard && strings.HasPrefix(env.SnippetID, "bites_rest_") {
				n++
			}
		}
		return n
	}

	allPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "bites-home"})
	if err != nil {
		t.Fatalf("RenderScreen(all) error: %v", err)
	}
	if got := countRestaurantCards(allPage); got != 6 {
		t.Fatalf("expected 6 restaurants on unfiltered bites-home, got %d", got)
	}
	var foundMarquee bool
	for _, env := range allPage.Snippets {
		if env.SnippetType == snippets.SnippetTypeMarqueeStrip {
			foundMarquee = true
			mq, err := snippets.DecodeSnippetData[snippets.MarqueeStripSnippetData](env)
			if err != nil {
				t.Fatalf("decode bites marquee_strip: %v", err)
			}
			if len(mq.Items) < 3 {
				t.Errorf("expected >= 3 marquee items on bites-home, got %d", len(mq.Items))
			}
		}
	}
	if !foundMarquee {
		t.Fatalf("expected marquee_strip on bites-home")
	}
	assertAllTrackingPropsAreStrings(t, allPage)

	vegPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "bites-home", Filter: "veg"})
	if err != nil {
		t.Fatalf("RenderScreen(veg) error: %v", err)
	}
	if got := countRestaurantCards(vegPage); got != 3 {
		t.Fatalf("expected 3 Pure Veg restaurants, got %d", got)
	}

	topRatedPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "bites-home", Filter: "top_rated"})
	if err != nil {
		t.Fatalf("RenderScreen(top_rated) error: %v", err)
	}
	if got := countRestaurantCards(topRatedPage); got != 5 {
		t.Fatalf("expected 5 restaurants rated >= 4.0, got %d", got)
	}

	biryaniPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "bites-home", Query: "biryani"})
	if err != nil {
		t.Fatalf("RenderScreen(search biryani) error: %v", err)
	}
	if got := countRestaurantCards(biryaniPage); got != 2 {
		t.Fatalf("expected 2 biryani matches, got %d", got)
	}

	emptyPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "bites-home", Query: "no-such-cuisine-xyz"})
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
		t.Fatalf("expected empty_state snippet when 0 restaurants match")
	}
}

func TestBitesRestaurantMenuAndLiveOrderTracking(t *testing.T) {
	t.Parallel()

	mod := bites.New()

	menuPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "bites-restaurant",
		Extra:    map[string]string{"id": "verde-bowl"},
		City:     "Pune",
	})
	if err != nil {
		t.Fatalf("RenderScreen(bites-restaurant) error: %v", err)
	}
	if !strings.Contains(menuPage.Title, "Verde Bowl") {
		t.Errorf("expected Verde Bowl title, got %q", menuPage.Title)
	}
	if menuPage.FloatingCart == nil {
		t.Fatalf("expected FloatingCart on bites-restaurant")
	}
	if menuPage.FloatingCart.FreeDeliveryThresholdCents == nil || *menuPage.FloatingCart.FreeDeliveryThresholdCents != 1800 {
		t.Fatalf("expected FreeDeliveryThresholdCents=1800 on bites-restaurant, got %+v", menuPage.FloatingCart.FreeDeliveryThresholdCents)
	}
	if menuPage.FloatingCart.NudgePrefix != "Add" || menuPage.FloatingCart.UnlockedLabel != "FREE Priority Delivery unlocked!" {
		t.Fatalf("unexpected bites floating cart nudge labels: prefix=%q unlocked=%q", menuPage.FloatingCart.NudgePrefix, menuPage.FloatingCart.UnlockedLabel)
	}
	callPayload, err := actions.DecodePayload[actions.CallAPIPayload](*menuPage.FloatingCart.CTA.OnClick)
	if err != nil {
		t.Fatalf("decode floating cart CTA: %v", err)
	}
	if callPayload.Endpoint != "/api/v1/bites/orders" || callPayload.Method != "POST" {
		t.Errorf("unexpected floating cart call_api: %+v", callPayload)
	}
	var foundHighlights bool
	for _, env := range menuPage.Snippets {
		if env.SnippetType == snippets.SnippetTypeComposedBlock {
			cb, decErr := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](env)
			if decErr != nil {
				t.Fatalf("decode bites highlights composed_block: %v", decErr)
			}
			if cb.RecipeName == "restaurant_highlights_card" {
				foundHighlights = true
				rawCB, _ := json.Marshal(cb)
				for _, wantKind := range []string{
					snippets.ComposedNodeRating,
					snippets.ComposedNodePrice,
					snippets.ComposedNodeRadioGroup,
					snippets.ComposedNodeKeyValueRow,
				} {
					if !strings.Contains(string(rawCB), `"`+wantKind+`"`) {
						t.Errorf("restaurant_highlights_card missing composed node kind %q", wantKind)
					}
				}
			}
		}
	}
	if !foundHighlights {
		t.Fatalf("expected restaurant_highlights_card composed_block on bites-restaurant")
	}
	assertAllTrackingPropsAreStrings(t, menuPage)

	// Verify bites-order-track also renders a valid 4-step demo tracker before any order is placed.
	defaultTrackPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "bites-order-track"})
	if err != nil {
		t.Fatalf("RenderScreen(default bites-order-track) error: %v", err)
	}
	assertAllTrackingPropsAreStrings(t, defaultTrackPage)

	// Place a food order and verify follow-up actions.
	res, err := mod.PlaceFoodOrder(context.Background(), domain.ExpressOrderRequest{
		Items:   map[string]int{"ghee_roast_dosa": 2},
		Address: "Baner, Pune",
	})
	if err != nil {
		t.Fatalf("PlaceFoodOrder error: %v", err)
	}
	orderID, _ := res.Data["order_id"].(string)
	if !strings.HasPrefix(orderID, "BTS-") {
		t.Fatalf("expected BTS- order ID, got %q", orderID)
	}
	if len(res.FollowUpActions) != 3 {
		t.Fatalf("expected 3 follow_up_actions (celebration, toast, navigate), got %d", len(res.FollowUpActions))
	}
	if res.FollowUpActions[0].Type != actions.ActionTriggerFeedback {
		t.Errorf("action[0] = %q, want trigger_feedback", res.FollowUpActions[0].Type)
	}
	if res.FollowUpActions[1].Type != actions.ActionShowToast {
		t.Errorf("action[1] = %q, want show_toast", res.FollowUpActions[1].Type)
	}
	if res.FollowUpActions[2].Type != actions.ActionNavigate {
		t.Errorf("action[2] = %q, want navigate", res.FollowUpActions[2].Type)
	}
	navPayload, err := actions.DecodePayload[actions.NavigatePayload](res.FollowUpActions[2])
	if err != nil {
		t.Fatalf("decode navigate payload: %v", err)
	}
	if navPayload.Route != "bites-order-track" {
		t.Errorf("navigate route = %q, want bites-order-track", navPayload.Route)
	}

	// Verify bites-order-track reflects the placed order ID & has animated 4-step order_tracker.
	trackPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "bites-order-track"})
	if err != nil {
		t.Fatalf("RenderScreen(bites-order-track) error: %v", err)
	}
	var foundTracker bool
	for _, env := range trackPage.Snippets {
		if env.SnippetType == snippets.SnippetTypeOrderTracker {
			foundTracker = true
			tr, decErr := snippets.DecodeSnippetData[snippets.OrderTrackerSnippetData](env)
			if decErr != nil {
				t.Fatalf("decode order_tracker: %v", decErr)
			}
			if tr.OrderCode != "#"+orderID {
				t.Errorf("order_code = %q, want #%s", tr.OrderCode, orderID)
			}
			if tr.EtaCountdown == nil || tr.EtaCountdown.RemainingSeconds != 660 {
				t.Errorf("expected 660s eta_countdown, got %+v", tr.EtaCountdown)
			}
			if len(tr.Steps) != 4 || !tr.Steps[0].Completed || !tr.Steps[1].Completed || !tr.Steps[2].Active || tr.Steps[3].Completed || tr.Steps[3].Active {
				t.Errorf("unexpected 4-step order_tracker progression: %+v", tr.Steps)
			}
			if tr.Steps[0].Label != "Order Confirmed" || tr.Steps[1].Label != "Kitchen Preparing" || tr.Steps[2].Label != "Rider En Route" || tr.Steps[3].Label != "Delivered" {
				t.Errorf("unexpected 4-step labels: %+v", tr.Steps)
			}
			if tr.RiderName == "" || tr.PrimaryAction == nil || tr.PrimaryAction.Label != "Call Rider" || tr.SecondaryAction == nil || tr.SecondaryAction.Label != "Back to Restaurants" {
				t.Errorf("unexpected rider/CTA details on order_tracker: rider=%q primary=%+v secondary=%+v", tr.RiderName, tr.PrimaryAction, tr.SecondaryAction)
			}
		}
	}
	if !foundTracker {
		t.Fatalf("expected order_tracker snippet on bites-order-track")
	}
	raw, _ := json.Marshal(trackPage)
	if !strings.Contains(string(raw), orderID) {
		t.Errorf("expected order tracking screen to contain order ID %q", orderID)
	}
	if !strings.Contains(string(raw), "Back to Restaurants") {
		t.Errorf("expected Back to Restaurants CTA on order tracking screen")
	}
	assertAllTrackingPropsAreStrings(t, trackPage)

	// Verify bites-home now displays active order pill.
	homeAfterOrder, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "bites-home"})
	if err != nil {
		t.Fatalf("RenderScreen(bites-home after order) error: %v", err)
	}
	homeRaw, _ := json.Marshal(homeAfterOrder)
	if !strings.Contains(string(homeRaw), orderID) {
		t.Errorf("expected bites-home header to display active order pill %q", orderID)
	}
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
