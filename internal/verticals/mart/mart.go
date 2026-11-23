// Package mart implements the Abeta Mart multi-category marketplace vertical.
package mart

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
	verticalID    = "abeta-mart"
	screenHome    = "mart-home"
	screenDetail  = "mart-product-detail"
	defaultCity   = "Bengaluru"
	orderEndpoint = "/api/v1/mart/orders"
)

// Compile-time check ensuring Module satisfies domain.VerticalModule.
var _ domain.VerticalModule = (*Module)(nil)

type martOrderRecord struct {
	OrderID   string
	ItemCount int
	TotalUSD  float64
	PlacedAt  time.Time
}

// Module serves Abeta Mart marketplace SDUI screens and order placement mutations.
type Module struct {
	mu         sync.RWMutex
	catalog    []catalogProduct
	orderSeq   int
	orderCount int
	orders     []martOrderRecord
}

// New initializes the Abeta Mart vertical with a curated multi-category catalog.
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
		Name:        "Abeta Mart",
		Tagline:     "Multi-Category Marketplace, Lightning Deals & Fast Checkout",
		Category:    "Marketplace",
		TenantID:    verticalID,
		EntryScreen: screenHome,
		ThemePreset: "indigo",
		Screens:     []string{screenHome, screenDetail},
	}
}

// Screens lists discoverable SDUI routes handled by Abeta Mart.
func (m *Module) Screens() []domain.ScreenSummary {
	return []domain.ScreenSummary{
		{
			ScreenID:    screenHome,
			Title:       "Abeta Mart · Marketplace",
			Vertical:    verticalID,
			Description: "Multi-category storefront with search, category chips, lightning deals carousel, and responsive product grid.",
		},
		{
			ScreenID:    screenDetail,
			Title:       "Abeta Mart · Product Detail",
			Vertical:    verticalID,
			Description: "Product detail page with instant Buy Now checkout, technical specs accordion, reviews, and native share.",
		},
	}
}

// CanHandleScreen reports whether screenID belongs to Abeta Mart.
func (m *Module) CanHandleScreen(screenID string) bool {
	return screenID == screenHome || screenID == screenDetail
}

// RenderScreen builds the SDUI PageResponse for mart-home or mart-product-detail.
func (m *Module) RenderScreen(ctx context.Context, req domain.ScreenRequest) (snippets.PageResponse, error) {
	if err := ctx.Err(); err != nil {
		return snippets.PageResponse{}, err
	}
	switch req.ScreenID {
	case screenHome:
		return m.renderHome(req), nil
	case screenDetail:
		return m.renderProductDetail(req), nil
	default:
		return snippets.PageResponse{}, fmt.Errorf("%w: %s", domain.ErrScreenNotFound, req.ScreenID)
	}
}

// PlaceOrder persists a marketplace order and returns celebration + toast follow-up actions.
func (m *Module) PlaceOrder(ctx context.Context, req domain.ExpressOrderRequest) (domain.ActionResult, error) {
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
	orderID := fmt.Sprintf("MRT-%04d", m.orderSeq)
	itemCount, totalUSD := m.summarizeOrderLocked(req)
	m.orders = append(m.orders, martOrderRecord{
		OrderID:   orderID,
		ItemCount: itemCount,
		TotalUSD:  totalUSD,
		PlacedAt:  time.Now().UTC(),
	})
	currentCount := m.orderCount
	m.mu.Unlock()

	celebration := actions.NewTriggerFeedbackAction("mart_celebration_"+orderID, actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCelebration,
		Title:      "Marketplace Order Placed!",
		Subtitle:   fmt.Sprintf("Order #%s confirmed · Next-day dispatch", orderID),
		DurationMs: 2800,
	})
	toast := actions.NewShowToastAction("mart_toast_"+orderID, actions.ShowToastPayload{
		Message:    fmt.Sprintf("Order #%s placed ($%.2f) · Tracking updates enabled", orderID, totalUSD),
		DurationMs: 2500,
		Variant:    "success",
	})

	return domain.ActionResult{
		Status:          "ok",
		Message:         fmt.Sprintf("Marketplace order #%s placed", orderID),
		FollowUpActions: []actions.ActionTrigger{celebration, toast},
		Data: map[string]any{
			"order_id":    orderID,
			"vertical":    verticalID,
			"item_count":  itemCount,
			"total_usd":   totalUSD,
			"order_count": currentCount,
		},
	}, nil
}

// OrderCount returns the total number of placed Abeta Mart orders.
func (m *Module) OrderCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.orderCount
}

func (m *Module) renderHome(req domain.ScreenRequest) snippets.PageResponse {
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
	allProducts := make([]catalogProduct, len(m.catalog))
	copy(allProducts, m.catalog)
	orderCount := m.orderCount
	m.mu.RUnlock()

	filtered := filterProducts(allProducts, filter, query)
	lightning := filterLightningDeals(allProducts)

	header := snippets.NewHeaderSnippet(
		"mart_header",
		atoms.NewText("Abeta Mart", theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(
			fmt.Sprintf("Next-day delivery to %s · Verified sellers", city),
			theming.TypographyBodyMedium,
			theming.ColorTextSecondary,
		)).
		WithTrailingTag(atoms.NewTag(fmt.Sprintf("ORDERS: %d", orderCount), theming.ColorBrandContainer)).
		WithLayout(sectionPadding()).
		Build()

	searchSubmit := actions.NewNavigateAction("mart_search_submit", actions.NavigatePayload{
		Route: "mart-home?q={{query}}",
	})
	searchClear := actions.NewNavigateAction("mart_search_clear", actions.NavigatePayload{
		Route: screenHome,
	})
	searchBar := snippets.NewSearchBarSnippet("mart_search_bar", "mart_search", "Search electronics, audio, home, fashion...").
		WithQuery(query).
		WithSubmitLabel("Search").
		WithOnSubmit(searchSubmit).
		WithOnClear(searchClear).
		WithLayout(cardLayout()).
		Build()

	marqueeStrip := buildMarqueeStrip()
	chipGroup := buildCategoryChips(filter, query)
	flashDealBlock := buildFlashDealComposedBlock(allProducts)
	highlightBlock := buildPriceRatingHighlightBlock(allProducts[0])
	carousel := buildLightningCarousel(lightning)
	catalogSnippet := buildCatalogSection(filtered, filter, query)

	checkoutAct := actions.NewCallAPIAction("mart_floating_checkout", actions.CallAPIPayload{
		Endpoint: orderEndpoint,
		Method:   "POST",
		Body: map[string]any{
			"vertical":   verticalID,
			"address":    city + " Express Locker",
			"item_count": 1,
		},
	})
	checkoutCTA := atoms.NewButton("btn_mart_checkout", "Express Checkout", "primary", &checkoutAct)
	etaBadge := atoms.NewTag("NEXT DAY", theming.ColorInfoContainer)
	etaBadge.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopFloat}

	toolbarTitle := atoms.NewText("Abeta Mart", theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText(fmt.Sprintf("%s · Marketplace", city), theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenHome, "Abeta Mart · Marketplace").
		WithToolbar(snippets.ToolbarSpec{
			Title:    &toolbarTitle,
			Subtitle: &toolbarSub,
			BgColor:  atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(1140),
		}).
		WithPullToRefresh(actions.NewRefreshPageAction("mart_pull_refresh", actions.RefreshPagePayload{
			PageID:     screenHome,
			ShowLoader: true,
		})).
		AddPageAction(actions.NewTrackEventAction("mart_screen_view", actions.TrackEventPayload{
			EventName: "screen_viewed",
			Properties: map[string]any{
				"vertical":     verticalID,
				"screen_id":    screenHome,
				"filter":       filter,
				"query":        query,
				"result_count": strconv.Itoa(len(filtered)),
			},
		})).
		AddSnippets(header, marqueeStrip, searchBar, chipGroup, flashDealBlock, highlightBlock, carousel, catalogSnippet).
		WithFloatingCart(snippets.FloatingCartSpec{
			Label:                      "Mart Cart",
			EtaBadge:                   &etaBadge,
			CTA:                        checkoutCTA,
			FreeDeliveryThresholdCents: atoms.IntPtr(2500),
			NudgePrefix:                "Add",
			UnlockedLabel:              "FREE Next-Day Delivery unlocked!",
			Motion: &atoms.MotionSpec{
				Enter:         atoms.MotionTransitionSlideUp,
				Exit:          atoms.MotionTransitionSlideDown,
				DurationToken: atoms.MotionDurationNormal,
			},
		}).
		WithTracking("page_loaded", map[string]any{
			"vertical":     verticalID,
			"page_id":      screenHome,
			"filter":       filter,
			"order_count":  strconv.Itoa(orderCount),
			"result_count": strconv.Itoa(len(filtered)),
		}).
		Build()
}

func (m *Module) renderProductDetail(req domain.ScreenRequest) snippets.PageResponse {
	requestedID := strings.TrimSpace(req.Extra["id"])
	m.mu.RLock()
	prod := m.catalog[0]
	for _, candidate := range m.catalog {
		if candidate.SKU == requestedID {
			prod = candidate
			break
		}
	}
	m.mu.RUnlock()

	backAct := actions.NewNavigateAction("mart_detail_back", actions.NavigatePayload{Route: screenHome})
	backIcon := atoms.NewIcon("arrow_back", 22, theming.ColorTextPrimary)
	backIcon.ContentDescription = "Back to Abeta Mart"
	backIcon.OnClick = &backAct

	buyAct := actions.NewCallAPIAction("mart_buy_now_"+prod.SKU, actions.CallAPIPayload{
		Endpoint: orderEndpoint,
		Method:   "POST",
		Body: map[string]any{
			"vertical":   verticalID,
			"items":      map[string]any{prod.SKU: 1},
			"item_count": 1,
			"total_usd":  float64(prod.UnitPriceCents) / 100.0,
		},
	})
	shareAct := actions.NewShareAction("mart_share_"+prod.SKU, actions.SharePayload{
		Title:        prod.Title,
		Text:         fmt.Sprintf("Check out %s (%s) on Abeta Mart — %s", prod.Title, prod.PriceDisplay, prod.Subtitle),
		URL:          "https://mart.abeta.dev/products/" + prod.SKU,
		ChooserTitle: "Share Product",
	})

	buyBtn := atoms.NewButton("btn_buy_"+prod.SKU, "Buy Now · "+prod.PriceDisplay, "primary", &buyAct)
	shareBtn := atoms.NewButton("btn_share_"+prod.SKU, "Share", "outline", &shareAct)

	heroCard := snippets.NewCardSnippet(
		"mart_detail_hero",
		atoms.NewText(prod.Title, theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(
			fmt.Sprintf("%s (was %s) · %s", prod.PriceDisplay, prod.OriginalPrice, prod.Rating),
			theming.TypographyTitleSmall,
			theming.ColorBrandPrimary,
		)).
		WithDescription(atoms.NewText(prod.Description, theming.TypographyBodyMedium, theming.ColorTextSecondary)).
		WithMedia(atoms.ImageAtom{
			URL:                fmt.Sprintf("https://cdn.abeta.dev/mart/%s.png", prod.SKU),
			ContentDescription: prod.Title,
			HeightDp:           atoms.IntPtr(180),
			ScaleMode:          "crop",
			CornerRadius:       atoms.UniformRadius(theming.RadiusMD.Dp()),
		}).
		WithBadge(atoms.NewTag(prod.Badge, prod.BadgeColor)).
		WithPrimaryButton(buyBtn).
		WithSecondaryButton(shareBtn).
		WithLayout(cardLayout()).
		Build()

	specsAcc := snippets.NewAccordionSnippet(
		"mart_specs_accordion",
		atoms.NewText("Technical Specifications", theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText("Verified manufacturer datasheet", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithInitiallyExpanded(true).
		WithLayout(cardLayout())
	for idx, spec := range prod.Specs {
		specsAcc.AddSnippet(
			snippets.NewActionRowSnippet(
				fmt.Sprintf("mart_spec_%s_%d", prod.SKU, idx),
				atoms.NewText(spec.Key, theming.TypographyLabelLarge, theming.ColorTextPrimary),
			).
				WithParentID("mart_specs_accordion").
				WithSubtitle(atoms.NewText(spec.Value, theming.TypographyBodySmall, theming.ColorTextSecondary)).
				WithTrailingTag(atoms.NewTag("SPEC", theming.ColorInfoContainer)).
				Build(),
		)
	}

	reviewsAcc := snippets.NewAccordionSnippet(
		"mart_reviews_accordion",
		atoms.NewText("Customer Reviews", theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(prod.Rating+" verified buyer feedback", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithInitiallyExpanded(false).
		WithLayout(cardLayout())
	for idx, rev := range prod.Reviews {
		reviewsAcc.AddSnippet(
			snippets.NewActionRowSnippet(
				fmt.Sprintf("mart_review_%s_%d", prod.SKU, idx),
				atoms.NewText(rev.Author, theming.TypographyLabelLarge, theming.ColorTextPrimary),
			).
				WithParentID("mart_reviews_accordion").
				WithSubtitle(atoms.NewText(rev.Comment, theming.TypographyBodySmall, theming.ColorTextSecondary)).
				WithTrailingTag(atoms.NewTag(rev.Rating, theming.ColorSuccessContainer)).
				Build(),
		)
	}

	toolbarTitle := atoms.NewText(prod.Title, theming.TypographyTitleMedium, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText(strings.ToUpper(prod.Category)+" · Abeta Mart", theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenDetail, prod.Title+" · Abeta Mart").
		WithToolbar(snippets.ToolbarSpec{
			Title:          &toolbarTitle,
			Subtitle:       &toolbarSub,
			NavigationIcon: &backIcon,
			BgColor:        atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode:          snippets.LayoutModeCentered,
			MaxWidthDp:          atoms.IntPtr(960),
			SplitPaneEnabled:    true,
			SecondarySnippetIDs: []string{"mart_reviews_accordion"},
		}).
		AddSnippets(heroCard, buildPriceRatingHighlightBlock(prod), specsAcc.Build(), reviewsAcc.Build()).
		WithTracking("product_detail_viewed", map[string]any{
			"vertical": verticalID,
			"sku":      prod.SKU,
			"category": prod.Category,
		}).
		Build()
}

func (m *Module) summarizeOrderLocked(req domain.ExpressOrderRequest) (int, float64) {
	priceBySKU := make(map[string]int, len(m.catalog))
	for _, p := range m.catalog {
		priceBySKU[p.SKU] = p.UnitPriceCents
	}
	totalItems := 0
	totalCents := 0
	for sku, qty := range req.Items {
		if qty <= 0 {
			continue
		}
		totalItems += qty
		if cents, ok := priceBySKU[sku]; ok {
			totalCents += cents * qty
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
	if totalUSD <= 0 {
		if totalCents > 0 {
			totalUSD = float64(totalCents) / 100.0
		} else {
			totalUSD = float64(m.catalog[0].UnitPriceCents) / 100.0
		}
	}
	return totalItems, totalUSD
}
