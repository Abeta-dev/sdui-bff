package bites

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
)

type dishSpec struct {
	SKU            string
	Name           string
	Subtitle       string
	PriceDisplay   string
	OriginalPrice  string
	UnitPriceCents int
	Veg            bool
	Badge          string
	Rating         string
}

type restaurantSpec struct {
	ID           string
	Name         string
	Cuisine      string
	Tags         []string
	Rating       float64
	PureVeg      bool
	DeliveryMins int
	PriceForTwo  string
	KitchenArea  string
	Badge        string
	Dishes       []dishSpec
}

func buildMarqueeStrip() snippets.SnippetEnvelope {
	boltIcon := atoms.NewIcon("bolt", 16, theming.ColorBrandPrimary)
	starIcon := atoms.NewIcon("star", 16, theming.ColorWarning)
	shippingIcon := atoms.NewIcon("local_shipping", 16, theming.ColorSuccess)
	prioBadge := atoms.NewTag("FREE $18+", theming.ColorSuccessContainer)
	hotBadge := atoms.NewTag("HOT DEALS", theming.ColorWarningContainer)

	return snippets.NewMarqueeStripSnippet(
		"bites_offers_marquee",
		"bites_hot_offers_ticker",
		snippets.MarqueeItem{
			ID:    "mq_bites_priority",
			Text:  "FREE Priority Delivery over $18",
			Icon:  &shippingIcon,
			Badge: &prioBadge,
		},
		snippets.MarqueeItem{
			ID:    "mq_bites_hot_meals",
			Text:  "20% off Slow-Cooked Biryani & Wood-Fired Pies",
			Icon:  &boltIcon,
			Badge: &hotBadge,
		},
		snippets.MarqueeItem{
			ID:   "mq_bites_thermal",
			Text: "Sealed thermal delivery bags · Live kitchen ETA",
			Icon: &starIcon,
		},
	).
		WithVelocityDpPerSec(44).
		WithSeparator("•").
		WithLayout(atoms.LayoutConfig{
			Padding:      atoms.SymmetricSpacing(theming.SpacingMD.Dp(), theming.SpacingSM.Dp()),
			BgColor:      atoms.ColorRef(theming.ColorSurfaceSecondary),
			FillMaxWidth: atoms.BoolPtr(true),
		}).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionFadeIn,
			DurationToken: atoms.MotionDurationFast,
			LoopEffect:    atoms.MotionLoopMarquee,
		}).
		WithTracking("bites_marquee_impression", map[string]any{
			"vertical": verticalID,
			"strip_id": "bites_hot_offers_ticker",
		}).
		Build()
}

func buildOrderTrackerSnippet(order foodOrder) snippets.SnippetEnvelope {
	confirmedIcon := atoms.NewIcon("check_circle", 18, theming.ColorSuccess)
	kitchenIcon := atoms.NewIcon("star", 18, theming.ColorSuccess)
	riderIcon := atoms.NewIcon("local_shipping", 18, theming.ColorBrandPrimary)
	deliveredIcon := atoms.NewIcon("home", 18, theming.ColorTextTertiary)

	etaCountdown := atoms.NewCountdown("bites_order_eta_timer", 660, "Arriving in")
	etaCountdown.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopPulse}

	etaBadge := atoms.NewTag(fmt.Sprintf("%d MINS", order.ETAMins), theming.ColorSuccessContainer)
	etaBadge.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopFloat}

	callRiderAct := actions.NewShowToastAction("bites_call_rider_"+order.OrderID, actions.ShowToastPayload{
		Message:    fmt.Sprintf("Connecting masked voice line to %s", order.RiderName),
		DurationMs: 2200,
		Variant:    "info",
	})
	callRiderBtn := atoms.NewButton("btn_bites_call_rider", "Call Rider", "primary", &callRiderAct)
	callRiderBtn.Size = "sm"

	backToRestaurants := actions.NewNavigateAction("bites_back_to_restaurants", actions.NavigatePayload{
		Route: screenHome,
	})
	backBtn := atoms.NewButton("btn_bites_back_restaurants", "Back to Restaurants", "outline", &backToRestaurants)
	backBtn.Size = "sm"

	return snippets.NewOrderTrackerSnippet(
		"bites_live_order_tracker",
		"bites_tracker_"+order.OrderID,
		"#"+order.OrderID,
		atoms.NewText(fmt.Sprintf("%s · %d items en route", order.RestaurantName, order.ItemCount), theming.TypographyTitleMedium, theming.ColorTextPrimary),
		snippets.OrderTrackerStep{
			ID:        "step_confirmed",
			Label:     "Order Confirmed",
			Caption:   order.RestaurantName,
			Icon:      &confirmedIcon,
			Completed: true,
			Active:    false,
		},
		snippets.OrderTrackerStep{
			ID:        "step_preparing",
			Label:     "Kitchen Preparing",
			Caption:   "Sealed in thermal hot bag",
			Icon:      &kitchenIcon,
			Completed: true,
			Active:    false,
		},
		snippets.OrderTrackerStep{
			ID:        "step_en_route",
			Label:     "Rider En Route",
			Caption:   order.RiderName + " · 1.1 km away",
			Icon:      &riderIcon,
			Completed: false,
			Active:    true,
		},
		snippets.OrderTrackerStep{
			ID:        "step_delivered",
			Label:     "Delivered",
			Caption:   order.Address,
			Icon:      &deliveredIcon,
			Completed: false,
			Active:    false,
		},
	).
		WithEtaCountdown(etaCountdown).
		WithEtaBadge(etaBadge).
		WithRider(order.RiderName, fmt.Sprintf("4.9★ · Insulated thermal pack · Delivering to %s", order.Address)).
		WithPrimaryAction(callRiderBtn).
		WithSecondaryAction(backBtn).
		WithLayout(cardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideUp,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("bites_order_tracker_viewed", map[string]any{
			"vertical": verticalID,
			"order_id": order.OrderID,
		}).
		Build()
}

func buildRestaurantHighlightsBlock(rest restaurantSpec) snippets.SnippetEnvelope {
	rating := atoms.NewRating("bites_rest_rating_"+rest.ID, rest.Rating, 5)
	rating.ReviewCount = atoms.IntPtr(840)
	rating.Label = rest.Cuisine

	sig := rest.Dishes[0]
	price := atoms.NewPrice("bites_sig_price_"+sig.SKU, sig.PriceDisplay, sig.OriginalPrice)
	price.DiscountLabel = "CHEF DEAL"
	price.UnitSuffix = "· " + sig.Name

	saveSpeed := actions.NewSaveStateAction("bites_save_delivery_speed", actions.SaveStatePayload{
		Key:   "bites_delivery_speed",
		Value: "priority_hot",
		Scope: "session",
	})
	speedGroup := atoms.NewRadioGroup(
		"bites_delivery_speed",
		"Delivery Speed Option",
		"priority_hot",
		atoms.OptionItem{ID: "priority_hot", Label: fmt.Sprintf("Priority Hot (%d mins)", rest.DeliveryMins), Subtitle: "Direct thermal dispatch · zero intermediate stops", Badge: "FAST"},
		atoms.OptionItem{ID: "standard", Label: fmt.Sprintf("Standard (%d mins)", rest.DeliveryMins+8), Subtitle: "Insulated hot bag · $0.99 delivery"},
		atoms.OptionItem{ID: "eco_saver", Label: "Eco Saver", Subtitle: "Consolidated route · $0 delivery fee", Badge: "FREE"},
	)
	speedGroup.OnChange = &saveSpeed

	sealRow := atoms.NewKeyValueRow("bites_kv_seal", "Thermal Bag & Tamper Seal", "$0.00")
	sealRow.ValueColor = atoms.ColorRef(theming.ColorSuccess)
	sealRow.ShowDivider = true

	freeRow := atoms.NewKeyValueRow("bites_kv_free_prio", "Free Priority Delivery", "Unlocks at $18.00")
	freeRow.ValueColor = atoms.ColorRef(theming.ColorBrandPrimary)
	freeRow.Emphasized = true

	root := snippets.CardNode(
		cardLayout(),
		snippets.VStack(
			theming.SpacingMD.Dp(),
			snippets.TextNode(atoms.NewText("Kitchen Highlights & Delivery Slot", theming.TypographyTitleMedium, theming.ColorTextPrimary)),
			snippets.RatingNode(rating).WithNodeID("bites_rating_node"),
			snippets.PriceNode(price).WithNodeID("bites_price_node"),
			snippets.DividerNode(atoms.DividerAtom{
				ThicknessDp: atoms.Float64Ptr(1),
				Color:       atoms.ColorRef(theming.ColorDivider),
			}),
			snippets.RadioGroupNode(speedGroup).WithNodeID("bites_speed_radio_node"),
			snippets.KeyValueRowNode(sealRow),
			snippets.KeyValueRowNode(freeRow),
		),
	).WithNodeID("bites_highlights_root")

	return snippets.NewComposedBlockSnippet("bites_restaurant_highlights_block", "bites_highlights_"+rest.ID, root).
		WithRecipeName("restaurant_highlights_card").
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionFadeIn,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("composed_recipe_viewed", map[string]any{
			"vertical":      verticalID,
			"recipe":        "restaurant_highlights_card",
			"restaurant_id": rest.ID,
		}).
		Build()
}

func buildFilterChipGroup(activeFilter string) snippets.SnippetEnvelope {
	specs := []struct {
		id    string
		label string
	}{
		{"all", "All"},
		{"veg", "Pure Veg"},
		{"top_rated", "Rating 4.0+"},
		{"fast", "Under 25m"},
		{"biryani", "Biryani"},
		{"pizza", "Wood-Fired"},
	}

	chips := make([]snippets.FilterChipItem, 0, len(specs))
	for _, sp := range specs {
		nav := actions.NewNavigateAction("bites_filter_"+sp.id, actions.NavigatePayload{
			Route: "bites-home?filter=" + sp.id,
		})
		chips = append(chips, snippets.FilterChipItem{
			ID:       sp.id,
			Label:    sp.label,
			Selected: sp.id == activeFilter,
			OnClick:  &nav,
		})
	}
	return snippets.NewChipGroupSnippet("bites_filter_chips", "bites_filters", chips...).
		WithLayout(sectionLayout()).
		Build()
}

func buildCollectionsCarousel() snippets.SnippetEnvelope {
	collections := []struct {
		id     string
		title  string
		sub    string
		badge  string
		filter string
	}{
		{"col_biryani", "Slow-Cooked Biryani", "Fragrant dum pots & saffron rice", "TRENDING", "biryani"},
		{"col_veg", "Pure Veg Gems", "100% vegetarian kitchens rated 4.3+", "PURE VEG", "veg"},
		{"col_fast", "Under 25 Min Express", "Hot bowls & wood-fired slices fast", "FAST", "fast"},
	}

	items := make([]snippets.SnippetEnvelope, 0, len(collections))
	for _, col := range collections {
		nav := actions.NewNavigateAction("bites_col_"+col.id, actions.NavigatePayload{
			Route: "bites-home?filter=" + col.filter,
		})
		items = append(items, snippets.NewCardSnippet(
			"bites_"+col.id,
			atoms.NewText(col.title, theming.TypographyTitleSmall, theming.ColorTextPrimary),
		).
			WithParentID("bites_collections_carousel").
			WithSubtitle(atoms.NewText(col.sub, theming.TypographyBodySmall, theming.ColorTextSecondary)).
			WithBadge(atoms.NewTag(col.badge, theming.ColorWarningContainer)).
			WithOnClick(nav).
			WithLayout(compactCardLayout()).
			Build())
	}

	return snippets.NewCarouselSnippet("bites_collections_carousel").
		WithTitle(atoms.NewText("Curated Collections", theming.TypographyTitleMedium, theming.ColorTextPrimary)).
		WithItemWidthDp(240).
		WithItemWidthByWidth(snippets.ResponsiveIntSpec{
			Compact:  atoms.IntPtr(220),
			Medium:   atoms.IntPtr(260),
			Expanded: atoms.IntPtr(300),
		}).
		WithSpacingDp(12).
		AddItems(items...).
		WithLayout(sectionLayout()).
		Build()
}

func buildRestaurantCard(index int, r restaurantSpec) snippets.SnippetEnvelope {
	nav := actions.NewNavigateAction("bites_open_"+r.ID, actions.NavigatePayload{
		Route: "bites-restaurant?id=" + r.ID,
	})
	badgeColor := theming.ColorWarningContainer
	if r.PureVeg {
		badgeColor = theming.ColorSuccessContainer
	}
	sub := fmt.Sprintf("%s · %.1f ★ · %d mins · %s", r.Cuisine, r.Rating, r.DeliveryMins, r.PriceForTwo)

	return snippets.NewCardSnippet(
		"bites_rest_"+r.ID,
		atoms.NewText(r.Name, theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(sub, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithDescription(atoms.NewText(fmt.Sprintf("Kitchen in %s · %d signature dishes ready", r.KitchenArea, len(r.Dishes)), theming.TypographyBodySmall, theming.ColorTextTertiary)).
		WithBadge(atoms.NewTag(r.Badge, badgeColor)).
		WithPrimaryButton(atoms.NewButton("btn_menu_"+r.ID, "View Menu", "primary", &nav)).
		WithOnClick(nav).
		WithLayout(cardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideUp,
			DurationToken: atoms.MotionDurationFast,
			StaggerMs:     50,
			DelayMs:       index * 40,
		}).
		WithTracking("bites_restaurant_impression", map[string]any{
			"restaurant_id": r.ID,
			"pure_veg":      strconv.FormatBool(r.PureVeg),
		}).
		Build()
}

func buildDishProductCard(index int, restID string, d dishSpec) snippets.SnippetEnvelope {
	fieldID := "cart_qty_" + d.SKU
	incAct := actions.NewBatchAction("bites_inc_"+d.SKU, actions.BatchPayload{
		Actions: []actions.ActionTrigger{
			actions.NewTriggerFeedbackAction("bites_bounce_"+d.SKU, actions.TriggerFeedbackPayload{
				Effect:     actions.FeedbackCartBounce,
				TargetID:   "floating_cart_dock",
				DurationMs: 280,
			}),
			actions.NewTrackEventAction("bites_track_inc_"+d.SKU, actions.TrackEventPayload{
				EventName: "bites_dish_added",
				Properties: map[string]any{
					"restaurant_id":    restID,
					"dish_sku":         d.SKU,
					"unit_price_cents": strconv.Itoa(d.UnitPriceCents),
				},
			}),
		},
		StopOnError: true,
	})
	decAct := actions.NewTrackEventAction("bites_dec_"+d.SKU, actions.TrackEventPayload{
		EventName: "bites_dish_removed",
		Properties: map[string]any{
			"restaurant_id": restID,
			"dish_sku":      d.SKU,
		},
	})
	stepper := atoms.NewStepper(fieldID, "ADD", 0, &incAct, &decAct)
	stepper.MinQuantity = 0
	stepper.MaxQuantity = 10

	badgeColor := theming.ColorWarningContainer
	if d.Veg {
		badgeColor = theming.ColorSuccessContainer
	}

	return snippets.NewProductCardSnippet(
		"bites_dish_"+d.SKU,
		d.SKU,
		atoms.NewText(d.Name, theming.TypographyTitleSmall, theming.ColorTextPrimary),
		atoms.NewText(d.PriceDisplay, theming.TypographyLabelLarge, theming.ColorTextPrimary),
		d.UnitPriceCents,
		stepper,
	).
		WithParentID("bites_menu_grid").
		WithSubtitle(atoms.NewText(d.Subtitle, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithOriginalPrice(atoms.NewText(d.OriginalPrice, theming.TypographyBodySmall, theming.ColorTextTertiary)).
		WithBadge(atoms.NewTag(d.Badge, badgeColor)).
		WithRating(atoms.NewText(d.Rating, theming.TypographyLabelSmall, theming.ColorSuccess)).
		WithLayout(compactCardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionScaleIn,
			DurationToken: atoms.MotionDurationFast,
			DelayMs:       index * 40,
		}).
		Build()
}

func filterRestaurants(rests []restaurantSpec, filter, query string) []restaurantSpec {
	q := strings.ToLower(query)
	out := make([]restaurantSpec, 0, len(rests))
	for _, r := range rests {
		if !matchesFilter(r, filter) {
			continue
		}
		if q != "" && !matchesQuery(r, q) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func matchesFilter(r restaurantSpec, filter string) bool {
	switch filter {
	case "", "all":
		return true
	case "veg":
		return r.PureVeg
	case "top_rated":
		return r.Rating >= 4.0
	case "fast":
		return r.DeliveryMins <= 25
	default:
		for _, tag := range r.Tags {
			if tag == filter {
				return true
			}
		}
		return strings.Contains(strings.ToLower(r.Cuisine), filter)
	}
}

func matchesQuery(r restaurantSpec, q string) bool {
	if strings.Contains(strings.ToLower(r.Name+" "+r.Cuisine+" "+r.KitchenArea+" "+r.Badge), q) {
		return true
	}
	for _, d := range r.Dishes {
		if strings.Contains(strings.ToLower(d.Name+" "+d.Subtitle), q) {
			return true
		}
	}
	return false
}

func resolveCity(raw string) string {
	if c := strings.TrimSpace(raw); c != "" {
		return c
	}
	return defaultCity
}

func sectionLayout() atoms.LayoutConfig {
	return atoms.LayoutConfig{
		Padding:      atoms.SymmetricSpacing(theming.SpacingMD.Dp(), theming.SpacingXS.Dp()),
		FillMaxWidth: atoms.BoolPtr(true),
	}
}

func cardLayout() atoms.LayoutConfig {
	return atoms.LayoutConfig{
		Padding:      atoms.UniformSpacing(theming.SpacingMD.Dp()),
		Margin:       atoms.SymmetricSpacing(theming.SpacingMD.Dp(), theming.SpacingXS.Dp()),
		BgColor:      atoms.ColorRef(theming.ColorSurfacePrimary),
		CornerRadius: atoms.UniformRadius(theming.RadiusMD.Dp()),
		Border: &atoms.BorderSpec{
			WidthDp: 1,
			Color:   *atoms.ColorRef(theming.ColorBorderSubtle),
		},
		FillMaxWidth: atoms.BoolPtr(true),
	}
}

func compactCardLayout() atoms.LayoutConfig {
	return atoms.LayoutConfig{
		Padding:      atoms.UniformSpacing(theming.SpacingSM.Dp()),
		BgColor:      atoms.ColorRef(theming.ColorSurfacePrimary),
		CornerRadius: atoms.UniformRadius(theming.RadiusMD.Dp()),
		Border: &atoms.BorderSpec{
			WidthDp: 1,
			Color:   *atoms.ColorRef(theming.ColorBorderSubtle),
		},
		FillMaxWidth: atoms.BoolPtr(true),
	}
}

func seedRestaurants() []restaurantSpec {
	return []restaurantSpec{
		{
			ID: "spice-atelier", Name: "Spice Atelier Kitchen", Cuisine: "Hyderabadi Biryani & Kebabs",
			Tags: []string{"biryani", "fast"}, Rating: 4.6, PureVeg: false, DeliveryMins: 22,
			PriceForTwo: "$18 for two", KitchenArea: "Indiranagar", Badge: "4.6 ★ · FAST",
			Dishes: []dishSpec{
				{SKU: "dum_biryani", Name: "Saffron Chicken Dum Biryani", Subtitle: "Aged basmati · Mirchi salan", PriceDisplay: "$9.50", OriginalPrice: "$11.00", UnitPriceCents: 950, Veg: false, Badge: "BESTSELLER", Rating: "4.8 ★"},
				{SKU: "paneer_biryani", Name: "Charred Paneer Subz Biryani", Subtitle: "Smoked cottage cheese · Raita", PriceDisplay: "$8.40", OriginalPrice: "$9.50", UnitPriceCents: 840, Veg: true, Badge: "VEG", Rating: "4.6 ★"},
				{SKU: "galouti_platter", Name: "Smoked Galouti Kebab Box", Subtitle: "4 pcs · Saffron paratha", PriceDisplay: "$7.90", OriginalPrice: "$8.90", UnitPriceCents: 790, Veg: false, Badge: "CHEF PICK", Rating: "4.7 ★"},
			},
		},
		{
			ID: "verde-bowl", Name: "Verde Bowl & Thali", Cuisine: "South Indian & Sattvic Bowls",
			Tags: []string{"veg", "fast"}, Rating: 4.8, PureVeg: true, DeliveryMins: 18,
			PriceForTwo: "$14 for two", KitchenArea: "Koramangala", Badge: "PURE VEG · 4.8 ★",
			Dishes: []dishSpec{
				{SKU: "ghee_roast_dosa", Name: "Crispy Ghee Podi Masala Dosa", Subtitle: "Fermented batter · Coconut chutney", PriceDisplay: "$6.50", OriginalPrice: "$7.50", UnitPriceCents: 650, Veg: true, Badge: "PURE VEG", Rating: "4.9 ★"},
				{SKU: "millet_thali", Name: "Heirloom Millet Wellness Thali", Subtitle: "Sambar · Rasam · Curd rice", PriceDisplay: "$8.90", OriginalPrice: "$10.20", UnitPriceCents: 890, Veg: true, Badge: "HIGH FIBER", Rating: "4.8 ★"},
			},
		},
		{
			ID: "ember-hearth", Name: "Ember Hearth Pizzeria", Cuisine: "Wood-Fired Sourdough Pizza",
			Tags: []string{"pizza", "fast"}, Rating: 4.5, PureVeg: false, DeliveryMins: 24,
			PriceForTwo: "$22 for two", KitchenArea: "Lavelle Road", Badge: "WOOD-FIRED",
			Dishes: []dishSpec{
				{SKU: "margherita_dop", Name: "San Marzano Burrata Pizza", Subtitle: "48h fermented dough · Fresh basil", PriceDisplay: "$12.50", OriginalPrice: "$14.00", UnitPriceCents: 1250, Veg: true, Badge: "ARTISAN", Rating: "4.7 ★"},
				{SKU: "truffle_funghi", Name: "Roasted Wild Mushroom Pie", Subtitle: "Truffle oil · Smoked scamorza", PriceDisplay: "$13.20", OriginalPrice: "$15.00", UnitPriceCents: 1320, Veg: true, Badge: "CHEF SPECIAL", Rating: "4.6 ★"},
			},
		},
		{
			ID: "saffron-handi", Name: "Saffron Handi House", Cuisine: "Awadhi Pure Veg Biryani & Curries",
			Tags: []string{"biryani", "veg"}, Rating: 4.3, PureVeg: true, DeliveryMins: 28,
			PriceForTwo: "$16 for two", KitchenArea: "Jayanagar", Badge: "PURE VEG",
			Dishes: []dishSpec{
				{SKU: "jackfruit_biryani", Name: "Awadhi Jackfruit Dum Biryani", Subtitle: "Clay-pot sealed · Mint raita", PriceDisplay: "$8.80", OriginalPrice: "$9.90", UnitPriceCents: 880, Veg: true, Badge: "SIGNATURE", Rating: "4.5 ★"},
				{SKU: "dal_bukhara", Name: "24-Hour Slow Dal Makhani", Subtitle: "Tandoori butter naan combo", PriceDisplay: "$7.60", OriginalPrice: "$8.50", UnitPriceCents: 760, Veg: true, Badge: "CLASSIC", Rating: "4.4 ★"},
			},
		},
		{
			ID: "forno-antico", Name: "Forno Antico Craft", Cuisine: "Wood-Fired Flatbreads & Pasta",
			Tags: []string{"pizza", "veg"}, Rating: 3.9, PureVeg: true, DeliveryMins: 30,
			PriceForTwo: "$15 for two", KitchenArea: "HSR Layout", Badge: "PURE VEG",
			Dishes: []dishSpec{
				{SKU: "pesto_flatbread", Name: "Basil Pesto Wood-Fired Flatbread", Subtitle: "Pine nuts · Aged cheddar", PriceDisplay: "$9.20", OriginalPrice: "$10.50", UnitPriceCents: 920, Veg: true, Badge: "FRESH BAKE", Rating: "4.1 ★"},
				{SKU: "arrabbiata_penne", Name: "Spicy Arrabbiata Penne", Subtitle: "Charred garlic · Olive oil", PriceDisplay: "$8.50", OriginalPrice: "$9.40", UnitPriceCents: 850, Veg: true, Badge: "VEG", Rating: "4.0 ★"},
			},
		},
		{
			ID: "noodle-harbor", Name: "Noodle Harbor Wok", Cuisine: "Pan-Asian Ramen & Dimsum",
			Tags: []string{"fast"}, Rating: 4.4, PureVeg: false, DeliveryMins: 21,
			PriceForTwo: "$20 for two", KitchenArea: "Whitefield", Badge: "21M EXPRESS",
			Dishes: []dishSpec{
				{SKU: "miso_ramen", Name: "Roasted Garlic Miso Ramen", Subtitle: "Hand-pulled noodles · Bok choy", PriceDisplay: "$10.90", OriginalPrice: "$12.00", UnitPriceCents: 1090, Veg: false, Badge: "HOT BOWL", Rating: "4.6 ★"},
				{SKU: "truffle_dimsum", Name: "Truffle Edamame Dimsum", Subtitle: "6 pcs · Chili crisp oil", PriceDisplay: "$7.80", OriginalPrice: "$8.80", UnitPriceCents: 780, Veg: true, Badge: "VEG", Rating: "4.5 ★"},
			},
		},
	}
}
