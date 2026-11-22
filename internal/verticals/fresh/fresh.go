// Package fresh implements the Abeta Fresh 8-minute quick-commerce grocery vertical.
package fresh

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

const (
	verticalID    = "abeta-fresh"
	screenHome    = "fresh-home"
	screenStore   = "store-home"
	defaultCity   = "Bengaluru"
	expressETAMin = 8
)

// Compile-time check ensuring Module satisfies domain.VerticalModule.
var _ domain.VerticalModule = (*Module)(nil)

type expressOrderRecord struct {
	OrderID   string
	ItemCount int
	TotalUSD  float64
	Address   string
	PlacedAt  time.Time
}

// Module serves Abeta Fresh SDUI screens and instant express checkout mutations.
type Module struct {
	mu           sync.RWMutex
	catalog      []groceryItem
	orderSeq     int
	orderCount   int
	recentOrders []expressOrderRecord
}

// New initializes the Abeta Fresh vertical with an in-memory grocery catalog.
func New() *Module {
	return &Module{
		catalog:  seedCatalog(),
		orderSeq: 1000,
	}
}

// Descriptor returns the vertical metadata for the Super-App catalog.
func (m *Module) Descriptor() domain.AppDescriptor {
	return domain.AppDescriptor{
		ID:          verticalID,
		Name:        "Abeta Fresh",
		Tagline:     "8-Min Quick-Commerce Grocery & Daily Essentials",
		Category:    "Quick Commerce",
		TenantID:    verticalID,
		EntryScreen: screenHome,
		ThemePreset: "emerald",
		Screens:     []string{screenHome, screenStore},
	}
}

// Screens lists discoverable SDUI screen routes handled by Abeta Fresh.
func (m *Module) Screens() []domain.ScreenSummary {
	return []domain.ScreenSummary{
		{
			ScreenID:    screenHome,
			Title:       "Abeta Fresh · 8-Min Delivery",
			Vertical:    verticalID,
			Description: "Quick-commerce grocery storefront with live search, category chips, and floating express cart.",
		},
		{
			ScreenID:    screenStore,
			Title:       "Abeta Fresh Storefront",
			Vertical:    verticalID,
			Description: "Alias storefront route backed by the Abeta Fresh catalog and express checkout.",
		},
	}
}

// CanHandleScreen reports whether screenID belongs to Abeta Fresh.
func (m *Module) CanHandleScreen(screenID string) bool {
	return screenID == screenHome || screenID == screenStore
}

// RenderScreen builds the responsive SDUI PageResponse for fresh-home or store-home.
func (m *Module) RenderScreen(ctx context.Context, req domain.ScreenRequest) (snippets.PageResponse, error) {
	if err := ctx.Err(); err != nil {
		return snippets.PageResponse{}, err
	}
	if !m.CanHandleScreen(req.ScreenID) {
		return snippets.PageResponse{}, fmt.Errorf("%w: %s", domain.ErrScreenNotFound, req.ScreenID)
	}

	city := strings.TrimSpace(req.City)
	if city == "" {
		city = defaultCity
	}
	query := strings.TrimSpace(req.Query)
	filter := strings.ToLower(strings.TrimSpace(req.Filter))
	if filter == "" {
		filter = "all"
	}

	m.mu.RLock()
	items := make([]groceryItem, len(m.catalog))
	copy(items, m.catalog)
	orderCount := m.orderCount
	m.mu.RUnlock()

	filtered := filterCatalog(items, filter, query)
	pageID := req.ScreenID

	headerSnippet := buildHeaderSnippet(city, orderCount)
	marqueeSnippet := buildMarqueeStripSnippet()
	searchSnippet := buildSearchSnippet(query)
	chipsSnippet := buildCategoryChipsSnippet(filter)
	nudgeSnippet := buildFlashNudgeSnippet(city)
	bannerSnippet := buildPromoBannerSnippet(city, filter, query, len(filtered), orderCount)
	loyaltySnippet := buildLoyaltyComposedSnippet(orderCount)

	var catalogSnippet snippets.SnippetEnvelope
	if len(filtered) == 0 {
		catalogSnippet = buildEmptyStateSnippet(query, filter)
	} else {
		catalogSnippet = buildProductGridSnippet(filtered)
	}

	toolbarTitle := atoms.NewText("Abeta Fresh", theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText(fmt.Sprintf("%s · %d mins", city, expressETAMin), theming.TypographyBodySmall, theming.ColorTextSecondary)

	checkoutAction := actions.NewCallAPIAction("fresh_checkout_express", actions.CallAPIPayload{
		Endpoint: "/api/v1/fresh/orders/express",
		Method:   "POST",
		Body: map[string]any{
			"vertical": verticalID,
			"address":  city + " Express Hub",
		},
	})
	checkoutCTA := atoms.NewButton("btn_fresh_express_checkout", "Checkout · 8m", "primary", &checkoutAction)
	etaBadge := atoms.NewTag("8 mins", theming.ColorSuccessContainer)
	etaBadge.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopFloat}

	return snippets.NewPage(pageID, "Abeta Fresh · 8-Min Delivery").
		WithToolbar(snippets.ToolbarSpec{
			Title:    &toolbarTitle,
			Subtitle: &toolbarSub,
			BgColor:  atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(1080),
		}).
		WithPullToRefresh(actions.NewRefreshPageAction("fresh_pull_refresh", actions.RefreshPagePayload{
			PageID:     pageID,
			ShowLoader: true,
		})).
		AddPageAction(actions.NewTrackEventAction("fresh_screen_view", actions.TrackEventPayload{
			EventName: "screen_viewed",
			Properties: map[string]any{
				"vertical":     verticalID,
				"screen_id":    pageID,
				"city":         city,
				"filter":       filter,
				"result_count": strconv.Itoa(len(filtered)),
			},
		})).
		AddSnippets(headerSnippet, marqueeSnippet, searchSnippet, chipsSnippet, nudgeSnippet, bannerSnippet, catalogSnippet, loyaltySnippet).
		WithFloatingCart(snippets.FloatingCartSpec{
			Label:                      "Express Cart",
			EtaBadge:                   &etaBadge,
			CTA:                        checkoutCTA,
			FreeDeliveryThresholdCents: atoms.IntPtr(1500),
			NudgePrefix:                "Add",
			UnlockedLabel:              "FREE 8-min delivery unlocked!",
			Motion: &atoms.MotionSpec{
				Enter:         atoms.MotionTransitionSlideUp,
				Exit:          atoms.MotionTransitionSlideDown,
				DurationToken: atoms.MotionDurationNormal,
			},
		}).
		WithTracking("page_loaded", map[string]any{
			"vertical":     verticalID,
			"page_id":      pageID,
			"city":         city,
			"filter":       filter,
			"order_count":  strconv.Itoa(orderCount),
			"result_count": strconv.Itoa(len(filtered)),
		}).
		Build(), nil
}

// PlaceExpressOrder records an express grocery order and emits celebration + toast follow-up actions.
func (m *Module) PlaceExpressOrder(ctx context.Context, req domain.ExpressOrderRequest) (domain.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActionResult{}, err
	}
	if req.ItemCount < 0 {
		return domain.ActionResult{}, fmt.Errorf("%w: item_count cannot be negative", domain.ErrInvalidInput)
	}
	for sku, qty := range req.Items {
		if qty < 0 {
			return domain.ActionResult{}, fmt.Errorf("%w: negative quantity for sku %q", domain.ErrInvalidInput, sku)
		}
	}

	m.mu.Lock()
	m.orderSeq++
	m.orderCount++
	orderID := fmt.Sprintf("FRS-%04d", m.orderSeq)
	itemCount, totalUSD := m.summarizeOrderLocked(req)
	address := strings.TrimSpace(req.Address)
	if address == "" {
		address = defaultCity + " Express Hub"
	}
	m.recentOrders = append(m.recentOrders, expressOrderRecord{
		OrderID:   orderID,
		ItemCount: itemCount,
		TotalUSD:  totalUSD,
		Address:   address,
		PlacedAt:  time.Now().UTC(),
	})
	currentOrderCount := m.orderCount
	m.mu.Unlock()

	celebration := actions.NewTriggerFeedbackAction("fresh_celebration_"+orderID, actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCelebration,
		Title:      "Fresh Order Confirmed!",
		Subtitle:   fmt.Sprintf("Order #%s arriving in %d minutes", orderID, expressETAMin),
		DurationMs: 2800,
	})
	toast := actions.NewShowToastAction("fresh_toast_"+orderID, actions.ShowToastPayload{
		Message:    fmt.Sprintf("Order #%s confirmed · Chilled rider dispatched", orderID),
		DurationMs: 2500,
		Variant:    "success",
	})

	return domain.ActionResult{
		Status:          "ok",
		Message:         fmt.Sprintf("Order #%s arriving in %d minutes", orderID, expressETAMin),
		FollowUpActions: []actions.ActionTrigger{celebration, toast},
		Data: map[string]any{
			"order_id":    orderID,
			"eta_mins":    expressETAMin,
			"item_count":  itemCount,
			"total_usd":   totalUSD,
			"order_count": currentOrderCount,
		},
	}, nil
}

// OrderCount returns the total number of express orders recorded so far.
func (m *Module) OrderCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.orderCount
}

func (m *Module) summarizeOrderLocked(req domain.ExpressOrderRequest) (int, float64) {
	priceBySKU := make(map[string]int, len(m.catalog))
	for _, item := range m.catalog {
		priceBySKU[item.SKU] = item.UnitPriceCents
	}
	totalItems := 0
	totalCents := 0
	for sku, qty := range req.Items {
		if qty <= 0 {
			continue
		}
		totalItems += qty
		if unitCents, ok := priceBySKU[sku]; ok {
			totalCents += unitCents * qty
		}
	}
	if totalItems == 0 {
		if req.ItemCount > 0 {
			totalItems = req.ItemCount
		} else {
			totalItems = 1
		}
	}
	totalUSD := req.TotalUSD
	if totalUSD <= 0 && totalCents > 0 {
		totalUSD = float64(totalCents) / 100.0
	}
	return totalItems, totalUSD
}

func filterCatalog(items []groceryItem, filter, query string) []groceryItem {
	q := strings.ToLower(query)
	out := make([]groceryItem, 0, len(items))
	for _, item := range items {
		if filter != "all" && item.Category != filter {
			continue
		}
		if q != "" {
			haystack := strings.ToLower(item.Title + " " + item.Subtitle + " " + item.Category + " " + item.Badge)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}
