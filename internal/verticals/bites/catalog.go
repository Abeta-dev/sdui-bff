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
