package mart_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/verticals/mart"
)

func TestDescriptorAndScreens(t *testing.T) {
	t.Parallel()
	m := mart.New()

	desc := m.Descriptor()
	if desc.ID != "abeta-mart" || desc.TenantID != "abeta-mart" || desc.EntryScreen != "mart-home" || desc.ThemePreset != "indigo" {
		t.Fatalf("unexpected descriptor: %+v", desc)
	}
	if len(desc.Screens) != 2 || desc.Screens[0] != "mart-home" || desc.Screens[1] != "mart-product-detail" {
		t.Fatalf("unexpected descriptor screens: %v", desc.Screens)
	}
	if !m.CanHandleScreen("mart-home") || !m.CanHandleScreen("mart-product-detail") || m.CanHandleScreen("unknown") {
		t.Fatalf("unexpected CanHandleScreen results")
	}
	if len(m.Screens()) != 2 {
		t.Fatalf("expected 2 screen summaries, got %d", len(m.Screens()))
	}

	_, err := m.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "unknown-screen"})
	if !errors.Is(err, domain.ErrScreenNotFound) {
		t.Fatalf("expected ErrScreenNotFound, got %v", err)
	}
}

func TestRenderHomeStructureAndFiltering(t *testing.T) {
	t.Parallel()
	m := mart.New()

	page, err := m.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "mart-home"})
	if err != nil {
		t.Fatalf("RenderScreen(mart-home) error: %v", err)
	}
	assertStringOnlyTrackingProps(t, page)

	if page.PageID != "mart-home" {
		t.Fatalf("expected page_id mart-home, got %q", page.PageID)
	}
	if page.FloatingCart == nil || page.FloatingCart.CTA.OnClick == nil {
		t.Fatalf("expected floating cart with CTA click action")
	}
	if page.FloatingCart.FreeDeliveryThresholdCents == nil || *page.FloatingCart.FreeDeliveryThresholdCents != 2500 {
		t.Fatalf("expected FreeDeliveryThresholdCents=2500 on mart-home, got %+v", page.FloatingCart.FreeDeliveryThresholdCents)
	}
	checkoutPayload, err := actions.DecodePayload[actions.CallAPIPayload](*page.FloatingCart.CTA.OnClick)
	if err != nil {
		t.Fatalf("decode floating cart CTA: %v", err)
	}
	if checkoutPayload.Endpoint != "/api/v1/mart/orders" || checkoutPayload.Method != "POST" {
		t.Fatalf("unexpected floating cart endpoint: %+v", checkoutPayload)
	}

	// Verify marquee strip, search bar, chip group, flash deal composed block, lightning carousel, and responsive product grid.
	var foundMarquee, foundComposed, foundSearch, foundChips, foundCarousel, foundGrid bool
	for _, env := range page.Snippets {
		switch env.SnippetType {
		case snippets.SnippetTypeMarqueeStrip:
			foundMarquee = true
			mq, err := snippets.DecodeSnippetData[snippets.MarqueeStripSnippetData](env)
			if err != nil {
				t.Fatalf("decode marquee_strip: %v", err)
			}
			if len(mq.Items) < 3 {
				t.Fatalf("expected >= 3 marquee items on mart-home, got %d", len(mq.Items))
			}

		case snippets.SnippetTypeComposedBlock:
			foundComposed = true
			cb, err := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](env)
			if err != nil {
				t.Fatalf("decode composed_block: %v", err)
			}
			if cb.RecipeName != "flash_deal_card" || !strings.HasPrefix(cb.BlockID, "mart_flash_") {
				t.Fatalf("unexpected mart flash deal composed_block: %+v", cb)
			}

		case snippets.SnippetTypeSearchBar:
			foundSearch = true
			data, err := snippets.DecodeSnippetData[snippets.SearchBarSnippetData](env)
			if err != nil {
				t.Fatalf("decode search bar: %v", err)
			}
			if data.SearchID != "mart_search" || data.OnSubmit == nil {
				t.Fatalf("unexpected search bar data: %+v", data)
			}
			nav, err := actions.DecodePayload[actions.NavigatePayload](*data.OnSubmit)
			if err != nil || nav.Route != "mart-home?q={{query}}" {
				t.Fatalf("unexpected search submit route: %+v (err=%v)", nav, err)
			}

		case snippets.SnippetTypeChipGroup:
			foundChips = true
			data, err := snippets.DecodeSnippetData[snippets.ChipGroupSnippetData](env)
			if err != nil {
				t.Fatalf("decode chip group: %v", err)
			}
			if len(data.Chips) != 5 {
				t.Fatalf("expected 5 category chips, got %d", len(data.Chips))
			}

		case snippets.SnippetTypeCarousel:
			foundCarousel = true
			data, err := snippets.DecodeSnippetData[snippets.CarouselSnippetData](env)
			if err != nil {
				t.Fatalf("decode carousel: %v", err)
			}
			if len(data.Items) < 3 {
				t.Fatalf("expected lightning deals carousel items, got %d", len(data.Items))
			}

		case snippets.SnippetTypeGrid:
			foundGrid = true
			data, err := snippets.DecodeSnippetData[snippets.GridSnippetData](env)
			if err != nil {
				t.Fatalf("decode grid: %v", err)
			}
			if data.Columns != 2 || data.ColumnsByWidth == nil {
				t.Fatalf("expected responsive grid columns, got %+v", data)
			}
			if *data.ColumnsByWidth.Compact != 2 || *data.ColumnsByWidth.Medium != 3 || *data.ColumnsByWidth.Expanded != 4 {
				t.Fatalf("unexpected columns_by_width: %+v", data.ColumnsByWidth)
			}
			if len(data.Items) != 8 {
				t.Fatalf("expected 8 catalog products, got %d", len(data.Items))
			}
			firstCard, err := snippets.DecodeSnippetData[snippets.ProductCardSnippetData](data.Items[0])
			if err != nil {
				t.Fatalf("decode product card: %v", err)
			}
			if firstCard.Stepper.FieldID == "" || firstCard.OnClick == nil {
				t.Fatalf("expected stepper and detail navigation on product card")
			}
			nav, err := actions.DecodePayload[actions.NavigatePayload](*firstCard.OnClick)
			if err != nil || !strings.HasPrefix(nav.Route, "mart-product-detail?id=") {
				t.Fatalf("unexpected product card click route: %+v (err=%v)", nav, err)
			}
		}
	}
	if !foundMarquee || !foundComposed || !foundSearch || !foundChips || !foundCarousel || !foundGrid {
		t.Fatalf("missing expected snippets: marquee=%v composed=%v search=%v chips=%v carousel=%v grid=%v", foundMarquee, foundComposed, foundSearch, foundChips, foundCarousel, foundGrid)
	}

	// Category filter test.
	audioPage, err := m.RenderScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "mart-home",
		Filter:   "audio",
	})
	if err != nil {
		t.Fatalf("RenderScreen audio filter error: %v", err)
	}
	gridData := findGrid(t, audioPage)
	if len(gridData.Items) != 2 {
		t.Fatalf("expected 2 audio products, got %d", len(gridData.Items))
	}

	// Search filter test.
	searchPage, err := m.RenderScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "mart-home",
		Query:    "OLED",
	})
	if err != nil {
		t.Fatalf("RenderScreen search error: %v", err)
	}
	searchGrid := findGrid(t, searchPage)
	if len(searchGrid.Items) != 1 {
		t.Fatalf("expected 1 OLED product, got %d", len(searchGrid.Items))
	}

	// Empty search returns empty_state.
	emptyPage, err := m.RenderScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "mart-home",
		Query:    "zz-no-match-999",
	})
	if err != nil {
		t.Fatalf("RenderScreen empty search error: %v", err)
	}
	foundEmpty := false
	for _, env := range emptyPage.Snippets {
		if env.SnippetType == snippets.SnippetTypeEmptyState {
			foundEmpty = true
		}
	}
	if !foundEmpty {
		t.Fatalf("expected empty_state snippet when query matches nothing")
	}
}

func TestRenderProductDetailAccordionsAndShare(t *testing.T) {
	t.Parallel()
	m := mart.New()

	// Default product when Extra["id"] is missing.
	defaultPage, err := m.RenderScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "mart-product-detail",
	})
	if err != nil {
		t.Fatalf("RenderScreen default detail error: %v", err)
	}
	assertStringOnlyTrackingProps(t, defaultPage)

	// Explicit SKU lookup.
	detailPage, err := m.RenderScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "mart-product-detail",
		Extra:    map[string]string{"id": "mrt_anc_buds"},
	})
	if err != nil {
		t.Fatalf("RenderScreen explicit detail error: %v", err)
	}
	assertStringOnlyTrackingProps(t, detailPage)
	if !strings.Contains(detailPage.Title, "AuraPods Pro ANC Earbuds") {
		t.Fatalf("expected title for AuraPods Pro ANC Earbuds, got %q", detailPage.Title)
	}

	var foundHero, foundSpecs, foundReviews bool
	for _, env := range detailPage.Snippets {
		switch env.SnippetID {
		case "mart_detail_hero":
			foundHero = true
			card, err := snippets.DecodeSnippetData[snippets.CardSnippetData](env)
			if err != nil {
				t.Fatalf("decode hero card: %v", err)
			}
			if card.PrimaryButton == nil || card.PrimaryButton.OnClick == nil {
				t.Fatalf("expected Buy Now primary button")
			}
			buyPayload, err := actions.DecodePayload[actions.CallAPIPayload](*card.PrimaryButton.OnClick)
			if err != nil || buyPayload.Endpoint != "/api/v1/mart/orders" {
				t.Fatalf("unexpected Buy Now payload: %+v (err=%v)", buyPayload, err)
			}
			if card.SecondaryButton == nil || card.SecondaryButton.OnClick == nil {
				t.Fatalf("expected Share secondary button")
			}
			if card.SecondaryButton.OnClick.Type != actions.ActionShare {
				t.Fatalf("expected ActionShare on share button, got %q", card.SecondaryButton.OnClick.Type)
			}

		case "mart_specs_accordion":
			foundSpecs = true
			acc, err := snippets.DecodeSnippetData[snippets.AccordionSnippetData](env)
			if err != nil {
				t.Fatalf("decode specs accordion: %v", err)
			}
			if !acc.InitiallyExpanded || len(acc.Snippets) == 0 {
				t.Fatalf("expected specs accordion initially expanded with rows, got %+v", acc)
			}

		case "mart_reviews_accordion":
			foundReviews = true
			acc, err := snippets.DecodeSnippetData[snippets.AccordionSnippetData](env)
			if err != nil {
				t.Fatalf("decode reviews accordion: %v", err)
			}
			if len(acc.Snippets) == 0 {
				t.Fatalf("expected customer review rows")
			}
		}
	}
	if !foundHero || !foundSpecs || !foundReviews {
		t.Fatalf("missing product detail sections: hero=%v specs=%v reviews=%v", foundHero, foundSpecs, foundReviews)
	}
}

func TestPlaceOrderAndConcurrency(t *testing.T) {
	t.Parallel()
	m := mart.New()

	// Invalid negative quantity.
	_, err := m.PlaceOrder(context.Background(), domain.ExpressOrderRequest{
		Items: map[string]int{"mrt_anc_buds": -2},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for negative quantity, got %v", err)
	}

	res, err := m.PlaceOrder(context.Background(), domain.ExpressOrderRequest{
		Items: map[string]int{"mrt_anc_buds": 2},
	})
	if err != nil {
		t.Fatalf("PlaceOrder error: %v", err)
	}
	assertStringOnlyTrackingProps(t, res)

	orderID, _ := res.Data["order_id"].(string)
	if !strings.HasPrefix(orderID, "MRT-") {
		t.Fatalf("expected MRT-xxxx order_id, got %q", orderID)
	}
	if len(res.FollowUpActions) != 2 {
		t.Fatalf("expected 2 follow-up actions, got %d", len(res.FollowUpActions))
	}
	fb, err := actions.DecodePayload[actions.TriggerFeedbackPayload](res.FollowUpActions[0])
	if err != nil || fb.Effect != actions.FeedbackCelebration || fb.Title != "Marketplace Order Placed!" {
		t.Fatalf("unexpected celebration follow-up: %+v (err=%v)", fb, err)
	}
	toast, err := actions.DecodePayload[actions.ShowToastPayload](res.FollowUpActions[1])
	if err != nil || toast.Variant != "success" {
		t.Fatalf("unexpected toast follow-up: %+v (err=%v)", toast, err)
	}

	// Concurrent order placement safety under -race.
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, callErr := m.PlaceOrder(context.Background(), domain.ExpressOrderRequest{ItemCount: 1}); callErr != nil {
				t.Errorf("concurrent PlaceOrder failed: %v", callErr)
			}
		}()
	}
	wg.Wait()

	if m.OrderCount() != 13 {
		t.Fatalf("expected 13 total orders, got %d", m.OrderCount())
	}
}

func findGrid(t *testing.T, page snippets.PageResponse) snippets.GridSnippetData {
	t.Helper()
	for _, env := range page.Snippets {
		if env.SnippetType == snippets.SnippetTypeGrid {
			data, err := snippets.DecodeSnippetData[snippets.GridSnippetData](env)
			if err != nil {
				t.Fatalf("decode grid: %v", err)
			}
			return data
		}
	}
	t.Fatalf("grid snippet not found on page %q", page.PageID)
	return snippets.GridSnippetData{}
}

// assertStringOnlyTrackingProps ensures KMP wire parity: every tracking.properties
// and track_event payload.properties map contains string values only.
func assertStringOnlyTrackingProps(t *testing.T, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	var walk func(node any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			if tr, ok := typed["tracking"].(map[string]any); ok {
				if props, ok := tr["properties"].(map[string]any); ok {
					for k, val := range props {
						if _, isStr := val.(string); !isStr {
							t.Errorf("tracking.properties[%q] must be string, got %T (%v)", k, val, val)
						}
					}
				}
			}
			if typed["type"] == string(actions.ActionTrackEvent) {
				if payload, ok := typed["payload"].(map[string]any); ok {
					if props, ok := payload["properties"].(map[string]any); ok {
						for k, val := range props {
							if _, isStr := val.(string); !isStr {
								t.Errorf("track_event.properties[%q] must be string, got %T (%v)", k, val, val)
							}
						}
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(root)
}
