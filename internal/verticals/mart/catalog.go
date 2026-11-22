package mart

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
)

type productSpec struct {
	Key   string
	Value string
}

type productReview struct {
	Author  string
	Rating  string
	Comment string
}

type catalogProduct struct {
	SKU            string
	Title          string
	Subtitle       string
	Description    string
	Category       string
	PriceDisplay   string
	OriginalPrice  string
	UnitPriceCents int
	Badge          string
	BadgeColor     theming.ColorToken
	Rating         string
	LightningDeal  bool
	Specs          []productSpec
	Reviews        []productReview
}

type categoryChip struct {
	ID    string
	Label string
}

func buildMarqueeStrip() snippets.SnippetEnvelope {
	shippingIcon := atoms.NewIcon("local_shipping", 16, theming.ColorSuccess)
	boltIcon := atoms.NewIcon("bolt", 16, theming.ColorWarning)
	checkIcon := atoms.NewIcon("check_circle", 16, theming.ColorBrandPrimary)
	freeShipBadge := atoms.NewTag("FREE $25+", theming.ColorSuccessContainer)
	flashBadge := atoms.NewTag("FLASH 28%", theming.ColorErrorContainer)

	return snippets.NewMarqueeStripSnippet(
		"mart_deals_marquee",
		"mart_live_deals_ticker",
		snippets.MarqueeItem{
			ID:    "mq_mart_free_ship",
			Text:  "Free Next-Day Delivery over $25",
			Icon:  &shippingIcon,
			Badge: &freeShipBadge,
		},
		snippets.MarqueeItem{
			ID:    "mq_mart_flash_buds",
			Text:  "Flash Drop: AuraPods Pro ANC 28% OFF",
			Icon:  &boltIcon,
			Badge: &flashBadge,
		},
		snippets.MarqueeItem{
			ID:   "mq_mart_warranty",
			Text: "Verified Sellers · 30-Day Hassle-Free Returns",
			Icon: &checkIcon,
		},
	).
		WithVelocityDpPerSec(46).
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
		WithTracking("mart_marquee_impression", map[string]any{
			"vertical": verticalID,
			"strip_id": "mart_live_deals_ticker",
		}).
		Build()
}

func buildFlashDealComposedBlock(products []catalogProduct) snippets.SnippetEnvelope {
	featured := products[0]
	for _, p := range products {
		if p.SKU == "mrt_anc_buds" {
			featured = p
			break
		}
	}
	claimNav := actions.NewNavigateAction("mart_claim_flash_"+featured.SKU, actions.NavigatePayload{
		Route:  fmt.Sprintf("%s?id=%s", screenDetail, featured.SKU),
		Params: map[string]string{"id": featured.SKU},
	})
	return snippets.MoldFlashDealRecipe(
		"mart_flash_deal_block",
		"mart_flash_"+featured.SKU,
		featured.Title+" · Flash Drop",
		featured.PriceDisplay,
		featured.OriginalPrice,
		480,
		claimNav,
	)
}

func buildCategoryChips(activeFilter, query string) snippets.SnippetEnvelope {
	categories := []categoryChip{
		{ID: "all", Label: "All"},
		{ID: "electronics", Label: "Electronics"},
		{ID: "audio", Label: "Audio"},
		{ID: "home", Label: "Home"},
		{ID: "fashion", Label: "Fashion"},
	}
	builder := snippets.NewChipGroupSnippet("mart_category_chips", "mart_categories").
		WithTitle(atoms.NewText("Browse Categories", theming.TypographyTitleSmall, theming.ColorTextPrimary)).
		WithLayout(sectionPadding())

	for _, cat := range categories {
		route := screenHome
		params := map[string]string{}
		if cat.ID != "all" {
			route = fmt.Sprintf("%s?filter=%s", screenHome, cat.ID)
			params["filter"] = cat.ID
		}
		if query != "" {
			sep := "?"
			if strings.Contains(route, "?") {
				sep = "&"
			}
			route = route + sep + "q=" + query
			params["q"] = query
		}
		navAct := actions.NewNavigateAction("mart_chip_"+cat.ID, actions.NavigatePayload{
			Route:  route,
			Params: params,
		})
		builder.AddChip(snippets.FilterChipItem{
			ID:       cat.ID,
			Label:    cat.Label,
			Selected: activeFilter == cat.ID,
			OnClick:  &navAct,
		})
	}
	return builder.Build()
}

func buildLightningCarousel(deals []catalogProduct) snippets.SnippetEnvelope {
	carousel := snippets.NewCarouselSnippet("mart_lightning_carousel").
		WithTitle(atoms.NewText("Lightning Deals", theming.TypographyTitleMedium, theming.ColorTextPrimary)).
		WithItemWidthDp(248).
		WithItemWidthByWidth(snippets.ResponsiveIntSpec{
			Compact:  atoms.IntPtr(236),
			Medium:   atoms.IntPtr(268),
			Expanded: atoms.IntPtr(296),
		}).
		WithSpacingDp(12).
		WithLayout(sectionPadding())

	for _, p := range deals {
		openDetail := actions.NewNavigateAction("mart_deal_"+p.SKU, actions.NavigatePayload{
			Route:  fmt.Sprintf("%s?id=%s", screenDetail, p.SKU),
			Params: map[string]string{"id": p.SKU},
		})
		card := snippets.NewCardSnippet(
			"mart_deal_card_"+p.SKU,
			atoms.NewText(p.Title, theming.TypographyTitleSmall, theming.ColorTextPrimary),
		).
			WithParentID("mart_lightning_carousel").
			WithSubtitle(atoms.NewText(fmt.Sprintf("%s · was %s", p.PriceDisplay, p.OriginalPrice), theming.TypographyLabelMedium, theming.ColorBrandPrimary)).
			WithBadge(atoms.NewTag(p.Badge, p.BadgeColor)).
			WithOnClick(openDetail).
			WithLayout(cardLayout()).
			Build()
		carousel.AddItem(card)
	}
	return carousel.Build()
}

func buildCatalogSection(products []catalogProduct, filter, query string) snippets.SnippetEnvelope {
	if len(products) == 0 {
		resetAct := actions.NewNavigateAction("mart_reset_filters", actions.NavigatePayload{Route: screenHome})
		resetBtn := atoms.NewButton("btn_mart_reset", "Show All Products", "primary", &resetAct)
		return snippets.NewEmptyStateSnippet(
			"mart_empty_catalog",
			atoms.NewText("No matching products", theming.TypographyTitleMedium, theming.ColorTextPrimary),
		).
			WithMessage(atoms.NewText("Try another search term or browse all categories.", theming.TypographyBodyMedium, theming.ColorTextSecondary)).
			WithIcon(atoms.NewIcon("search", 32, theming.ColorTextTertiary)).
			WithActionButton(resetBtn).
			WithLayout(cardLayout()).
			Build()
	}

	grid := snippets.NewGridSnippet("mart_catalog_grid", 2).
		WithTitle(atoms.NewText(fmt.Sprintf("Marketplace Picks (%d)", len(products)), theming.TypographyTitleMedium, theming.ColorTextPrimary)).
		WithColumnsByWidth(snippets.ResponsiveColumnsSpec{
			Compact:  atoms.IntPtr(2),
			Medium:   atoms.IntPtr(3),
			Expanded: atoms.IntPtr(4),
		}).
		WithSpacingDp(12).
		WithLayout(sectionPadding()).
		WithTracking("catalog_grid_viewed", map[string]any{
			"vertical":     verticalID,
			"filter":       filter,
			"query":        query,
			"result_count": strconv.Itoa(len(products)),
		})

	for idx, p := range products {
		grid.AddItem(buildProductCard(idx, p))
	}
	return grid.Build()
}

func buildProductCard(index int, p catalogProduct) snippets.SnippetEnvelope {
	fieldID := "cart_qty_" + p.SKU
	incAct := actions.NewBatchAction("mart_inc_"+p.SKU, actions.BatchPayload{
		Actions: []actions.ActionTrigger{
			actions.NewTriggerFeedbackAction("mart_bounce_"+p.SKU, actions.TriggerFeedbackPayload{
				Effect:     actions.FeedbackCartBounce,
				TargetID:   "floating_cart_dock",
				DurationMs: 320,
			}),
			actions.NewTrackEventAction("mart_track_inc_"+p.SKU, actions.TrackEventPayload{
				EventName: "cart_item_incremented",
				Properties: map[string]any{
					"vertical":         verticalID,
					"sku":              p.SKU,
					"unit_price_cents": strconv.Itoa(p.UnitPriceCents),
				},
			}),
		},
		StopOnError: true,
	})
	decAct := actions.NewTrackEventAction("mart_dec_"+p.SKU, actions.TrackEventPayload{
		EventName: "cart_item_decremented",
		Properties: map[string]any{
			"vertical": verticalID,
			"sku":      p.SKU,
		},
	})
	stepper := atoms.NewStepper(fieldID, "ADD", 0, &incAct, &decAct)
	stepper.MinQuantity = 0
	stepper.MaxQuantity = 10

	detailNav := actions.NewNavigateAction("mart_open_"+p.SKU, actions.NavigatePayload{
		Route:  fmt.Sprintf("%s?id=%s", screenDetail, p.SKU),
		Params: map[string]string{"id": p.SKU},
	})

	return snippets.NewProductCardSnippet(
		"mart_prod_"+p.SKU,
		p.SKU,
		atoms.NewText(p.Title, theming.TypographyTitleSmall, theming.ColorTextPrimary),
		atoms.NewText(p.PriceDisplay, theming.TypographyLabelLarge, theming.ColorTextPrimary),
		p.UnitPriceCents,
		stepper,
	).
		WithParentID("mart_catalog_grid").
		WithSubtitle(atoms.NewText(p.Subtitle, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithOriginalPrice(atoms.NewText(p.OriginalPrice, theming.TypographyBodySmall, theming.ColorTextTertiary)).
		WithBadge(atoms.NewTag(p.Badge, p.BadgeColor)).
		WithImage(atoms.ImageAtom{
			URL:                fmt.Sprintf("https://cdn.abeta.dev/mart/%s.png", p.SKU),
			ContentDescription: p.Title,
			HeightDp:           atoms.IntPtr(112),
			ScaleMode:          "crop",
			CornerRadius:       atoms.UniformRadius(theming.RadiusSM.Dp()),
		}).
		WithRating(atoms.NewText(p.Rating, theming.TypographyLabelSmall, theming.ColorSuccess)).
		WithOnClick(detailNav).
		WithLayout(cardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionScaleIn,
			DurationToken: atoms.MotionDurationFast,
			StaggerMs:     50,
			DelayMs:       index * 50,
		}).
		Build()
}

func filterProducts(products []catalogProduct, filter, query string) []catalogProduct {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]catalogProduct, 0, len(products))
	for _, p := range products {
		if filter != "all" && !strings.EqualFold(p.Category, filter) {
			continue
		}
		if q != "" {
			haystack := strings.ToLower(p.Title + " " + p.Subtitle + " " + p.Category + " " + p.SKU + " " + p.Badge)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

func filterLightningDeals(products []catalogProduct) []catalogProduct {
	out := make([]catalogProduct, 0, 4)
	for _, p := range products {
		if p.LightningDeal {
			out = append(out, p)
		}
	}
	return out
}

func sectionPadding() atoms.LayoutConfig {
	return atoms.LayoutConfig{
		Padding:      atoms.SymmetricSpacing(theming.SpacingLG.Dp(), theming.SpacingSM.Dp()),
		FillMaxWidth: atoms.BoolPtr(true),
	}
}

func cardLayout() atoms.LayoutConfig {
	return atoms.LayoutConfig{
		Padding:      atoms.UniformSpacing(theming.SpacingMD.Dp()),
		BgColor:      atoms.ColorRef(theming.ColorSurfacePrimary),
		CornerRadius: atoms.UniformRadius(theming.RadiusMD.Dp()),
		FillMaxWidth: atoms.BoolPtr(true),
	}
}

func seedCatalog() []catalogProduct {
	return []catalogProduct{
		{
			SKU: "mrt_laptop_pro", Title: "NovaBook 14 OLED Laptop", Subtitle: "16 GB RAM · 512 GB NVMe",
			Description: "Ultralight 1.24 kg aluminum workstation with 2.8K 120 Hz OLED panel and all-day battery.",
			Category:    "electronics", PriceDisplay: "$899.00", OriginalPrice: "$1,099.00", UnitPriceCents: 89900,
			Badge: "LIGHTNING", BadgeColor: theming.ColorWarningContainer, Rating: "4.9 ★ (412)", LightningDeal: true,
			Specs: []productSpec{
				{Key: "Display", Value: "14.0-inch 2.8K OLED · 120 Hz"},
				{Key: "Memory & Storage", Value: "16 GB LPDDR5X · 512 GB PCIe Gen4 SSD"},
				{Key: "Battery", Value: "72 Wh · 65 W USB-C fast charge"},
			},
			Reviews: []productReview{
				{Author: "Riya S.", Rating: "5.0 ★", Comment: "Crisp OLED colors and silent cooling even under heavy builds."},
				{Author: "Devansh K.", Rating: "4.8 ★", Comment: "Fast next-day shipping and crisp keyboard feel."},
			},
		},
		{
			SKU: "mrt_tablet_air", Title: "SlatePad Pro 11 Tablet", Subtitle: "120 Hz Display · Stylus Ready",
			Description: "Edge-to-edge creative slate engineered for low-latency sketching and split-screen multitasking.",
			Category:    "electronics", PriceDisplay: "$449.00", OriginalPrice: "$529.00", UnitPriceCents: 44900,
			Badge: "TOP RATED", BadgeColor: theming.ColorInfoContainer, Rating: "4.7 ★ (238)", LightningDeal: false,
			Specs: []productSpec{
				{Key: "Display", Value: "11.0-inch IPS · 120 Hz Adaptive"},
				{Key: "Connectivity", Value: "Wi-Fi 6E · USB-C DisplayPort"},
			},
			Reviews: []productReview{{Author: "Karan P.", Rating: "4.8 ★", Comment: "Smooth stylus response and solid two-day battery life."}},
		},
		{
			SKU: "mrt_anc_buds", Title: "AuraPods Pro ANC Earbuds", Subtitle: "Adaptive ANC · 36h Playback",
			Description: "Dual-driver acoustic earbuds with real-time adaptive noise cancellation and multipoint pairing.",
			Category:    "audio", PriceDisplay: "$129.00", OriginalPrice: "$179.00", UnitPriceCents: 12900,
			Badge: "FLASH 28%", BadgeColor: theming.ColorSuccessContainer, Rating: "4.8 ★ (894)", LightningDeal: true,
			Specs: []productSpec{
				{Key: "Noise Cancellation", Value: "Hybrid 45 dB Adaptive ANC"},
				{Key: "Battery Life", Value: "9h buds + 27h wireless charging case"},
			},
			Reviews: []productReview{{Author: "Neha M.", Rating: "4.9 ★", Comment: "Blocks cabin noise effortlessly and stays comfortable for hours."}},
		},
		{
			SKU: "mrt_studio_cans", Title: "Resonance Studio Headphones", Subtitle: "Planar Drivers · Spatial Audio",
			Description: "Closed-back reference wireless headphones tuned for balanced studio monitoring and travel.",
			Category:    "audio", PriceDisplay: "$219.00", OriginalPrice: "$269.00", UnitPriceCents: 21900,
			Badge: "HI-RES", BadgeColor: theming.ColorBrandContainer, Rating: "4.9 ★ (310)", LightningDeal: false,
			Specs: []productSpec{
				{Key: "Drivers", Value: "45 mm planar magnetic"},
				{Key: "Codecs", Value: "LDAC · AAC · USB-C lossless"},
			},
			Reviews: []productReview{{Author: "Siddharth V.", Rating: "5.0 ★", Comment: "Detailed mids and zero listening fatigue during long sessions."}},
		},
		{
			SKU: "mrt_espresso_bar", Title: "CremaCraft Compact Espresso", Subtitle: "15-Bar Pump · Steam Wand",
			Description: "Stainless-steel countertop espresso maker with rapid thermoblock heating and PID temperature control.",
			Category:    "home", PriceDisplay: "$189.00", OriginalPrice: "$249.00", UnitPriceCents: 18900,
			Badge: "DEAL", BadgeColor: theming.ColorWarningContainer, Rating: "4.8 ★ (521)", LightningDeal: true,
			Specs: []productSpec{
				{Key: "Pressure", Value: "15-bar Italian vibration pump"},
				{Key: "Heat-Up", Value: "25-second PID thermoblock"},
			},
			Reviews: []productReview{{Author: "Ananya D.", Rating: "4.8 ★", Comment: "Consistent crema every morning and tiny counter footprint."}},
		},
		{
			SKU: "mrt_air_purifier", Title: "PureBreeze HEPA-13 Purifier", Subtitle: "22 dB Whisper · PM2.5 Sensor",
			Description: "True HEPA-13 + activated carbon room purifier covering 480 sq ft with automated sleep mode.",
			Category:    "home", PriceDisplay: "$139.00", OriginalPrice: "$169.00", UnitPriceCents: 13900,
			Badge: "ECO SMART", BadgeColor: theming.ColorSuccessContainer, Rating: "4.7 ★ (196)", LightningDeal: false,
			Specs: []productSpec{
				{Key: "Filtration", Value: "3-Stage Pre-filter + HEPA-13 + Carbon"},
				{Key: "Noise Floor", Value: "22 dB in night mode"},
			},
			Reviews: []productReview{{Author: "Vikram T.", Rating: "4.7 ★", Comment: "Noticeable air quality improvement within 15 minutes."}},
		},
		{
			SKU: "mrt_merino_hoodie", Title: "CloudWeave Merino Zip Hoodie", Subtitle: "Breathable Knit · All-Season",
			Description: "18.5-micron ethically sourced merino wool zip hoodie with articulated travel pockets.",
			Category:    "fashion", PriceDisplay: "$79.00", OriginalPrice: "$109.00", UnitPriceCents: 7900,
			Badge: "LIMITED", BadgeColor: theming.ColorBrandContainer, Rating: "4.8 ★ (342)", LightningDeal: true,
			Specs: []productSpec{
				{Key: "Fabric", Value: "88% Merino Wool · 12% Recycled Nylon"},
				{Key: "Care", Value: "Machine washable cold, flat dry"},
			},
			Reviews: []productReview{{Author: "Megha R.", Rating: "4.9 ★", Comment: "Regulates warmth great on flights and packs down super light."}},
		},
		{
			SKU: "mrt_commuter_pack", Title: "TransitShell 24L Tech Pack", Subtitle: "Weatherproof · 16\" Sleeve",
			Description: "Ballistic nylon daily carry backpack with suspended laptop vault and luggage pass-through.",
			Category:    "fashion", PriceDisplay: "$94.00", OriginalPrice: "$119.00", UnitPriceCents: 9400,
			Badge: "BESTSELLER", BadgeColor: theming.ColorInfoContainer, Rating: "4.9 ★ (275)", LightningDeal: false,
			Specs: []productSpec{
				{Key: "Capacity", Value: "24 L · 16-inch padded laptop bay"},
				{Key: "Shell", Value: "840D recycled ballistic nylon + YKK AquaGuard"},
			},
			Reviews: []productReview{{Author: "Tarun K.", Rating: "5.0 ★", Comment: "Stands upright on its own and kept my gear bone dry in heavy rain."}},
		},
	}
}
