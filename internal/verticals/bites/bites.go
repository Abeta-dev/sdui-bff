// Package bites implements the Abeta Bites food delivery and restaurant vertical.
package bites

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
	verticalID       = "abeta-bites"
	screenHome       = "bites-home"
	screenRestaurant = "bites-restaurant"
	screenOrderTrack = "bites-order-track"
	defaultCity      = "Bengaluru"
)

// Compile-time check ensuring Module satisfies domain.VerticalModule.
var _ domain.VerticalModule = (*Module)(nil)

type foodOrder struct {
	OrderID        string
	RestaurantID   string
	RestaurantName string
	Address        string
	ItemCount      int
	ETAMins        int
	Progress       float64
	StageLabel     string
	RiderName      string
	PlacedAt       time.Time
}

// Module serves Abeta Bites SDUI screens and live food order orchestration.
type Module struct {
	mu            sync.RWMutex
	restaurants   []restaurantSpec
	orders        map[string]foodOrder
	latestOrderID string
	orderSeq      int
}

// New initializes the Abeta Bites vertical with 6 curated restaurants and live order tracking.
func New() *Module {
	return &Module{
		restaurants: seedRestaurants(),
		orders:      make(map[string]foodOrder),
		orderSeq:    2000,
	}
}

// Descriptor returns the vertical metadata for the Super-App catalog.
func (m *Module) Descriptor() domain.AppDescriptor {
	return domain.AppDescriptor{
		ID:          verticalID,
		Name:        "Abeta Bites",
		Tagline:     "Curated Kitchens, Live Prep Tracking & Hot Delivery",
		Category:    "Food Delivery",
		TenantID:    verticalID,
		EntryScreen: screenHome,
		ThemePreset: "amber",
		Screens:     []string{screenHome, screenRestaurant, screenOrderTrack},
	}
}

// Screens lists discoverable SDUI screen routes handled by Abeta Bites.
func (m *Module) Screens() []domain.ScreenSummary {
	return []domain.ScreenSummary{
		{
			ScreenID:    screenHome,
			Title:       "Abeta Bites · Food Delivery",
			Vertical:    verticalID,
			Description: "Restaurant discovery feed with Pure Veg, rating, fast delivery, and cuisine filters.",
		},
		{
			ScreenID:    screenRestaurant,
			Title:       "Restaurant Menu",
			Vertical:    verticalID,
			Description: "Interactive restaurant menu with inline dish steppers and floating cart checkout.",
		},
		{
			ScreenID:    screenOrderTrack,
			Title:       "Live Order Tracking",
			Vertical:    verticalID,
			Description: "Real-time kitchen preparation progress and delivery rider status tracker.",
		},
	}
}

// CanHandleScreen reports whether screenID belongs to Abeta Bites.
func (m *Module) CanHandleScreen(screenID string) bool {
	return screenID == screenHome || screenID == screenRestaurant || screenID == screenOrderTrack
}

// RenderScreen dispatches rendering across bites-home, bites-restaurant, and bites-order-track.
func (m *Module) RenderScreen(ctx context.Context, req domain.ScreenRequest) (snippets.PageResponse, error) {
	if err := ctx.Err(); err != nil {
		return snippets.PageResponse{}, err
	}
	switch req.ScreenID {
	case screenHome:
		return m.renderHomeScreen(req), nil
	case screenRestaurant:
		return m.renderRestaurantScreen(req), nil
	case screenOrderTrack:
		return m.renderOrderTrackScreen(req), nil
	default:
		return snippets.PageResponse{}, fmt.Errorf("%w: %s", domain.ErrScreenNotFound, req.ScreenID)
	}
}

// PlaceFoodOrder records a live tracked order and triggers celebration, toast, and order-track navigation.
func (m *Module) PlaceFoodOrder(ctx context.Context, req domain.ExpressOrderRequest) (domain.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActionResult{}, err
	}
	if req.ItemCount < 0 {
		return domain.ActionResult{}, fmt.Errorf("%w: item_count cannot be negative", domain.ErrInvalidInput)
	}
	for sku, qty := range req.Items {
		if qty < 0 {
			return domain.ActionResult{}, fmt.Errorf("%w: negative quantity for dish %q", domain.ErrInvalidInput, sku)
		}
	}

	m.mu.Lock()
	m.orderSeq++
	orderID := fmt.Sprintf("BTS-%04d", m.orderSeq)
	rest := m.matchRestaurantLocked(req.Items)
	itemCount := req.ItemCount
	if itemCount == 0 {
		for _, qty := range req.Items {
			itemCount += qty
		}
		if itemCount == 0 {
			itemCount = 2
		}
	}
	address := strings.TrimSpace(req.Address)
	if address == "" {
		address = defaultCity + " Central"
	}
	order := foodOrder{
		OrderID:        orderID,
		RestaurantID:   rest.ID,
		RestaurantName: rest.Name,
		Address:        address,
		ItemCount:      itemCount,
		ETAMins:        rest.DeliveryMins,
		Progress:       0.65,
		StageLabel:     fmt.Sprintf("Kitchen preparing · %d mins", rest.DeliveryMins),
		RiderName:      "Aarav K.",
		PlacedAt:       time.Now().UTC(),
	}
	m.orders[orderID] = order
	m.latestOrderID = orderID
	m.mu.Unlock()

	celebration := actions.NewTriggerFeedbackAction("bites_celebration_"+orderID, actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCelebration,
		Title:      "Kitchen Accepted Your Order!",
		Subtitle:   fmt.Sprintf("Order #%s from %s · ETA %d mins", orderID, rest.Name, rest.DeliveryMins),
		DurationMs: 2800,
	})
	toast := actions.NewShowToastAction("bites_toast_"+orderID, actions.ShowToastPayload{
		Message:    fmt.Sprintf("Order #%s confirmed · Tracking live prep", orderID),
		DurationMs: 2400,
		Variant:    "success",
	})
	navTrack := actions.NewNavigateAction("bites_nav_track_"+orderID, actions.NavigatePayload{
		Route: screenOrderTrack,
	})

	return domain.ActionResult{
		Status:          "ok",
		Message:         fmt.Sprintf("Order #%s accepted by %s", orderID, rest.Name),
		FollowUpActions: []actions.ActionTrigger{celebration, toast, navTrack},
		Data: map[string]any{
			"order_id":        orderID,
			"restaurant_id":   rest.ID,
			"restaurant_name": rest.Name,
			"eta_mins":        rest.DeliveryMins,
			"item_count":      itemCount,
		},
	}, nil
}

func (m *Module) renderHomeScreen(req domain.ScreenRequest) snippets.PageResponse {
	city := resolveCity(req.City)
	query := strings.TrimSpace(req.Query)
	filter := strings.ToLower(strings.TrimSpace(req.Filter))
	if filter == "" {
		filter = "all"
	}

	m.mu.RLock()
	rests := make([]restaurantSpec, len(m.restaurants))
	copy(rests, m.restaurants)
	activeOrder, hasActive := m.orders[m.latestOrderID]
	m.mu.RUnlock()

	filtered := filterRestaurants(rests, filter, query)

	headerBuilder := snippets.NewHeaderSnippet(
		"bites_header",
		atoms.NewText("Abeta Bites · Food Delivery", theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(fmt.Sprintf("Delivering hot in %s · Curated partner kitchens", city), theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithLeadingIcon(atoms.NewIcon("home", 22, theming.ColorBrandPrimary)).
		WithLayout(sectionLayout())

	if hasActive {
		trackNav := actions.NewNavigateAction("bites_active_order_pill", actions.NavigatePayload{Route: screenOrderTrack})
		trackBtn := atoms.NewButton("btn_bites_track_live", "Live · "+activeOrder.OrderID, "secondary", &trackNav)
		trackBtn.Size = "sm"
		headerBuilder.
			WithTrailingTag(atoms.NewTag("LIVE "+activeOrder.OrderID, theming.ColorSuccessContainer)).
			WithTrailingAction(trackBtn)
	} else {
		headerBuilder.WithTrailingTag(atoms.NewTag(city, theming.ColorWarningContainer))
	}

	searchBar := snippets.NewSearchBarSnippet("bites_search_bar", "bites_search", "Search biryani, wood-fired pizza, pure veg...").
		WithQuery(query).
		WithLeadingIcon(atoms.NewIcon("search", 20, theming.ColorTextSecondary)).
		WithSubmitLabel("Search").
		WithOnSubmit(actions.NewNavigateAction("bites_search_submit", actions.NavigatePayload{Route: "bites-home?q={{query}}"})).
		WithOnClear(actions.NewNavigateAction("bites_search_clear", actions.NavigatePayload{Route: screenHome})).
		WithLayout(sectionLayout()).
		Build()

	listSnippets := []snippets.SnippetEnvelope{
		headerBuilder.Build(),
		searchBar,
		buildFilterChipGroup(filter),
		buildCollectionsCarousel(),
	}
	if len(filtered) == 0 {
		resetAct := actions.NewNavigateAction("bites_reset_filters", actions.NavigatePayload{Route: screenHome})
		listSnippets = append(listSnippets, snippets.NewEmptyStateSnippet(
			"bites_empty_state",
			atoms.NewText("No kitchens matched your filter", theming.TypographyTitleMedium, theming.ColorTextPrimary),
		).
			WithMessage(atoms.NewText("Try resetting your search or switching cuisine filters.", theming.TypographyBodyMedium, theming.ColorTextSecondary)).
			WithIcon(atoms.NewIcon("search", 32, theming.ColorTextTertiary)).
			WithActionButton(atoms.NewButton("btn_bites_reset", "Reset Filters", "primary", &resetAct)).
			WithLayout(cardLayout()).
			Build())
	} else {
		for idx, r := range filtered {
			listSnippets = append(listSnippets, buildRestaurantCard(idx, r))
		}
	}

	toolbarTitle := atoms.NewText("Abeta Bites", theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText("Delivering to "+city, theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenHome, "Abeta Bites · Food Delivery").
		WithToolbar(snippets.ToolbarSpec{
			Title:    &toolbarTitle,
			Subtitle: &toolbarSub,
			BgColor:  atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(1080),
		}).
		WithPullToRefresh(actions.NewRefreshPageAction("bites_refresh_home", actions.RefreshPagePayload{
			PageID:     screenHome,
			ShowLoader: true,
		})).
		AddSnippets(listSnippets...).
		WithTracking("page_loaded", map[string]any{
			"vertical":     verticalID,
			"page_id":      screenHome,
			"city":         city,
			"filter":       filter,
			"result_count": strconv.Itoa(len(filtered)),
		}).
		Build()
}

func (m *Module) renderRestaurantScreen(req domain.ScreenRequest) snippets.PageResponse {
	city := resolveCity(req.City)
	restID := strings.TrimSpace(req.Extra["id"])

	m.mu.RLock()
	rest := m.restaurants[0]
	if restID != "" {
		for _, candidate := range m.restaurants {
			if candidate.ID == restID {
				rest = candidate
				break
			}
		}
	}
	m.mu.RUnlock()

	badgeText := fmt.Sprintf("%.1f ★ · %dm", rest.Rating, rest.DeliveryMins)
	badgeColor := theming.ColorWarningContainer
	if rest.PureVeg {
		badgeText = "PURE VEG · " + badgeText
		badgeColor = theming.ColorSuccessContainer
	}

	header := snippets.NewHeaderSnippet(
		"bites_rest_header",
		atoms.NewText(rest.Name, theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(fmt.Sprintf("%s · %s, %s", rest.Cuisine, rest.KitchenArea, city), theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithTrailingTag(atoms.NewTag(badgeText, badgeColor)).
		WithLayout(sectionLayout()).
		Build()

	ratingMetric := snippets.NewMetricCardSnippet(
		"bites_rest_metric",
		atoms.NewText("Rating & Delivery ETA", theming.TypographyLabelMedium, theming.ColorTextSecondary),
		atoms.NewText(fmt.Sprintf("%.1f ★ · %d mins", rest.Rating, rest.DeliveryMins), theming.TypographyHeadlineSmall, theming.ColorBrandPrimary),
	).
		WithDelta(atoms.NewTag(rest.PriceForTwo, theming.ColorBrandContainer)).
		WithIcon(atoms.NewIcon("star", 22, theming.ColorBrandPrimary)).
		WithProgress(atoms.ProgressAtom{
			Kind:     "linear",
			Progress: atoms.Float64Ptr(rest.Rating / 5.0),
			Label:    fmt.Sprintf("Kitchen dispatch from %s", rest.KitchenArea),
			Color:    atoms.ColorRef(theming.ColorSuccess),
		}).
		WithLayout(cardLayout()).
		Build()

	dishCards := make([]snippets.SnippetEnvelope, 0, len(rest.Dishes))
	for idx, d := range rest.Dishes {
		dishCards = append(dishCards, buildDishProductCard(idx, rest.ID, d))
	}

	menuGrid := snippets.NewGridSnippet("bites_menu_grid", 2).
		WithTitle(atoms.NewText("Recommended Menu", theming.TypographyTitleMedium, theming.ColorTextPrimary)).
		WithColumnsByWidth(snippets.ResponsiveColumnsSpec{
			Compact:  atoms.IntPtr(1),
			Medium:   atoms.IntPtr(2),
			Expanded: atoms.IntPtr(3),
		}).
		WithSpacingDp(12).
		AddItems(dishCards...).
		WithLayout(sectionLayout()).
		Build()

	orderAct := actions.NewCallAPIAction("bites_place_order_"+rest.ID, actions.CallAPIPayload{
		Endpoint: "/api/v1/bites/orders",
		Method:   "POST",
		Body: map[string]any{
			"vertical": verticalID,
			"address":  city + " Delivery Zone",
			"items":    map[string]int{rest.Dishes[0].SKU: 1},
		},
	})
	checkoutBtn := atoms.NewButton("btn_bites_place_order", "Place Order", "primary", &orderAct)
	etaTag := atoms.NewTag(fmt.Sprintf("%d mins", rest.DeliveryMins), theming.ColorSuccessContainer)

	backAct := actions.NewNavigateAction("bites_back_home", actions.NavigatePayload{Route: screenHome})
	backIcon := atoms.NewIcon("arrow_back", 22, theming.ColorTextPrimary)
	backIcon.OnClick = &backAct
	toolbarTitle := atoms.NewText(rest.Name, theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText(rest.Cuisine, theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenRestaurant, rest.Name+" · Abeta Bites").
		WithToolbar(snippets.ToolbarSpec{
			Title:          &toolbarTitle,
			Subtitle:       &toolbarSub,
			NavigationIcon: &backIcon,
			BgColor:        atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(1080),
		}).
		AddSnippets(header, ratingMetric, menuGrid).
		WithFloatingCart(snippets.FloatingCartSpec{
			Label:    "Bites Cart · " + rest.Name,
			EtaBadge: &etaTag,
			CTA:      checkoutBtn,
		}).
		WithTracking("page_loaded", map[string]any{
			"vertical":      verticalID,
			"page_id":       screenRestaurant,
			"restaurant_id": rest.ID,
			"city":          city,
		}).
		Build()
}

func (m *Module) renderOrderTrackScreen(req domain.ScreenRequest) snippets.PageResponse {
	city := resolveCity(req.City)

	m.mu.RLock()
	order, hasOrder := m.orders[m.latestOrderID]
	defaultRest := m.restaurants[0]
	m.mu.RUnlock()

	if !hasOrder {
		order = foodOrder{
			OrderID:        "BTS-DEMO",
			RestaurantName: defaultRest.Name,
			Address:        city + " Central",
			ItemCount:      2,
			ETAMins:        defaultRest.DeliveryMins,
			Progress:       0.45,
			StageLabel:     "Ready for your first order",
			RiderName:      "Dispatch standby",
		}
	}

	header := snippets.NewHeaderSnippet(
		"bites_track_header",
		atoms.NewText("Live Order Tracking", theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(fmt.Sprintf("Order #%s · %s", order.OrderID, order.RestaurantName), theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithTrailingTag(atoms.NewTag("PREPARING", theming.ColorSuccessContainer)).
		WithLayout(sectionLayout()).
		Build()

	prepMetric := snippets.NewMetricCardSnippet(
		"bites_kitchen_progress",
		atoms.NewText("Kitchen Prep Progress", theming.TypographyLabelMedium, theming.ColorTextSecondary),
		atoms.NewText(order.StageLabel, theming.TypographyTitleLarge, theming.ColorBrandPrimary),
	).
		WithDelta(atoms.NewTag(fmt.Sprintf("ETA %d mins", order.ETAMins), theming.ColorWarningContainer)).
		WithProgress(atoms.ProgressAtom{
			Kind:     "linear",
			Progress: atoms.Float64Ptr(order.Progress),
			Label:    fmt.Sprintf("Order #%s · %d items firing in kitchen", order.OrderID, order.ItemCount),
			Color:    atoms.ColorRef(theming.ColorBrandPrimary),
		}).
		WithLayout(cardLayout()).
		Build()

	backToRestaurants := actions.NewNavigateAction("bites_back_to_restaurants", actions.NavigatePayload{Route: screenHome})
	refreshTrack := actions.NewRefreshPageAction("bites_refresh_track", actions.RefreshPagePayload{PageID: screenOrderTrack, ShowLoader: true})

	riderCard := snippets.NewCardSnippet(
		"bites_rider_status",
		atoms.NewText("Delivery Partner · "+order.RiderName, theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(fmt.Sprintf("Assigned to %s · Delivering to %s", order.RestaurantName, order.Address), theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithDescription(atoms.NewText("Thermal bag verified · Contactless drop available on arrival.", theming.TypographyBodyMedium, theming.ColorTextSecondary)).
		WithBadge(atoms.NewTag("ON TIME", theming.ColorSuccessContainer)).
		WithPrimaryButton(atoms.NewButton("btn_bites_back_restaurants", "Back to Restaurants", "primary", &backToRestaurants)).
		WithSecondaryButton(atoms.NewButton("btn_bites_refresh_track", "Refresh Status", "outline", &refreshTrack)).
		WithLayout(cardLayout()).
		Build()

	toolbarTitle := atoms.NewText("Order #"+order.OrderID, theming.TypographyTitleLarge, theming.ColorTextPrimary)

	return snippets.NewPage(screenOrderTrack, "Live Order Tracking · Abeta Bites").
		WithToolbar(snippets.ToolbarSpec{
			Title:   &toolbarTitle,
			BgColor: atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(960),
		}).
		AddSnippets(header, prepMetric, riderCard).
		WithTracking("page_loaded", map[string]any{
			"vertical": verticalID,
			"page_id":  screenOrderTrack,
			"order_id": order.OrderID,
		}).
		Build()
}

func (m *Module) matchRestaurantLocked(items map[string]int) restaurantSpec {
	for sku := range items {
		for _, rest := range m.restaurants {
			for _, dish := range rest.Dishes {
				if dish.SKU == sku {
					return rest
				}
			}
		}
	}
	return m.restaurants[0]
}
