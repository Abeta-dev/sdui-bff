package fresh

import (
	"fmt"
	"strconv"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
)

type groceryItem struct {
	SKU            string
	Title          string
	Subtitle       string
	Category       string
	PriceDisplay   string
	OriginalPrice  string
	UnitPriceCents int
	Badge          string
	BadgeColor     theming.ColorToken
	Rating         string
	InitialQty     int
}

type categoryChipSpec struct {
	ID    string
	Label string
}

func buildHeaderSnippet(city string, orderCount int) snippets.SnippetEnvelope {
	subtitle := fmt.Sprintf("Chilled micro-hub in %s · Seal-verified cold chain", city)
	if orderCount > 0 {
		subtitle = fmt.Sprintf("%s · %d express orders dispatched today", city, orderCount)
	}
	return snippets.NewHeaderSnippet(
		"fresh_header",
		atoms.NewText("Abeta Fresh · 8-Min Delivery", theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(subtitle, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithLeadingIcon(atoms.NewIcon("home", 22, theming.ColorBrandPrimary)).
		WithTrailingTag(atoms.NewTag(city, theming.ColorSuccessContainer)).
		WithLayout(sectionLayout()).
		Build()
}

func buildMarqueeStripSnippet() snippets.SnippetEnvelope {
	boltIcon := atoms.NewIcon("bolt", 16, theming.ColorBrandPrimary)
	shippingIcon := atoms.NewIcon("local_shipping", 16, theming.ColorSuccess)
	checkIcon := atoms.NewIcon("check_circle", 16, theming.ColorSuccess)
	expressBadge := atoms.NewTag("8 MIN", theming.ColorSuccessContainer)
	freeBadge := atoms.NewTag("FREE", theming.ColorBrandContainer)

	return snippets.NewMarqueeStripSnippet(
		"fresh_perks_marquee",
		"fresh_perks_ticker",
		snippets.MarqueeItem{
			ID:    "mq_fresh_dispatch",
			Text:  "8-min dispatch",
			Icon:  &boltIcon,
			Badge: &expressBadge,
		},
		snippets.MarqueeItem{
			ID:    "mq_fresh_free_delivery",
			Text:  "Free delivery over $15",
			Icon:  &shippingIcon,
			Badge: &freeBadge,
		},
		snippets.MarqueeItem{
			ID:   "mq_fresh_zero_surge",
			Text: "Zero surge pricing",
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
		WithTracking("fresh_marquee_impression", map[string]any{
			"vertical": verticalID,
			"strip_id": "fresh_perks_ticker",
		}).
		Build()
}

func buildFlashNudgeSnippet(city string) snippets.SnippetEnvelope {
	badge := atoms.NewTag("FLASH DROP", theming.ColorWarningContainer)
	badge.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopWiggle}

	countdown := atoms.NewCountdown("fresh_drop_timer", 420, "Flash drop ends in")
	countdown.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopHeartbeat}

	produceNav := actions.NewNavigateAction("fresh_flash_shop_produce", actions.NavigatePayload{
		Route: "fresh-home?filter=produce",
	})
	ctaBtn := atoms.NewButton("btn_fresh_flash_drop", "Shop Drop", "primary", &produceNav)
	ctaBtn.Size = "sm"

	return snippets.NewNudgeBarSnippet(
		"fresh_flash_nudge",
		"fresh_flash_nudge",
		atoms.NewText("Chilled Flash Drop · Extra 20% off in "+city, theming.TypographyTitleSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText("Reserved for your 8-min express slot · Limited cold-hub stock", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithIcon(atoms.NewIcon("bolt", 20, theming.ColorWarning)).
		WithBadge(badge).
		WithCountdown(countdown).
		WithProgress(0.78).
		WithCTA(ctaBtn).
		WithDismissible(true).
		WithPointerEdge(snippets.PointerEdgeBottom).
		WithOnDismiss(actions.NewTrackEventAction("fresh_flash_nudge_dismissed", actions.TrackEventPayload{
			EventName: "nudge_dismissed",
			Properties: map[string]any{
				"vertical": verticalID,
				"nudge_id": "fresh_flash_nudge",
			},
		})).
		WithLayout(cardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideDown,
			DurationToken: atoms.MotionDurationFast,
		}).
		WithTracking("fresh_nudge_impression", map[string]any{
			"vertical": verticalID,
			"nudge_id": "fresh_flash_nudge",
			"city":     city,
		}).
		Build()
}

func buildLoyaltyComposedSnippet(orderCount int) snippets.SnippetEnvelope {
	const totalStamps = 5
	earned := 3 + orderCount
	if earned > totalStamps {
		earned = totalStamps
	}
	rewardLabel := fmt.Sprintf("%d more express orders to unlock $5 Fresh Club credit", totalStamps-earned)
	if earned >= totalStamps {
		rewardLabel = "$5 Fresh Club reward unlocked! Tap to redeem on your next basket"
	}
	cta := actions.NewShowToastAction("fresh_loyalty_claim_toast", actions.ShowToastPayload{
		Message:    "Abeta Fresh Club · Earn 1 stamp per 8-min order for $5 credit",
		DurationMs: 2200,
		Variant:    "success",
	})
	return snippets.MoldLoyaltyStampRecipe(
		"fresh_loyalty_stamp_card",
		"abeta_fresh_club",
		"Abeta Fresh Club",
		earned,
		totalStamps,
		rewardLabel,
		cta,
	)
}

func buildDeliverySummaryComposedBlock(city string) snippets.SnippetEnvelope {
	saveSlot := actions.NewSaveStateAction("fresh_save_delivery_slot", actions.SaveStatePayload{
		Key:   "fresh_delivery_slot",
		Value: "express_8m",
		Scope: "session",
	})
	slotSelect := atoms.NewSelect(
		"fresh_delivery_slot",
		"Express Delivery Slot",
		"Choose arrival window",
		"express_8m",
		atoms.OptionItem{ID: "express_8m", Label: "8-Min Chilled Express", Subtitle: "Instant micro-hub dispatch in " + city, Badge: "FASTEST"},
		atoms.OptionItem{ID: "scheduled_30m", Label: "30-Min Scheduled", Subtitle: "Pick an exact chilled arrival window"},
		atoms.OptionItem{ID: "evening_eco", Label: "Evening Eco Slot", Subtitle: "Zero-emission consolidated drop", Badge: "ECO"},
	)
	slotSelect.HelperText = "Free 8-min express delivery unlocks automatically at $15.00"
	slotSelect.OnChange = &saveSlot

	coldChainRow := atoms.NewKeyValueRow("fresh_kv_cold_chain", "Cold-Chain Seal & Packaging", "$0.00")
	coldChainRow.ValueColor = atoms.ColorRef(theming.ColorSuccess)
	coldChainRow.ShowDivider = true

	deliveryRow := atoms.NewKeyValueRow("fresh_kv_delivery", "Express Delivery ("+city+")", "FREE over $15")
	deliveryRow.ValueColor = atoms.ColorRef(theming.ColorSuccess)
	deliveryRow.ShowDivider = true

	savingsRow := atoms.NewKeyValueRow("fresh_kv_savings", "Fresh Club Member Savings", "-$2.50")
	savingsRow.ValueColor = atoms.ColorRef(theming.ColorBrandPrimary)
	savingsRow.Emphasized = true

	coldBadge := atoms.BadgeAtom{
		ID:      "fresh_cold_chain_badge",
		Label:   "CHILLED",
		Variant: atoms.VariantSuccess,
	}

	root := snippets.CardNode(
		cardLayout(),
		snippets.VStack(
			theming.SpacingMD.Dp(),
			snippets.HStack(
				theming.SpacingSM.Dp(),
				snippets.TextNode(atoms.NewText("Delivery Slot & Fee Summary", theming.TypographyTitleMedium, theming.ColorTextPrimary)).WithWeight(1),
				snippets.BadgeNode(coldBadge),
			).WithArrangement(snippets.ComposedAlignSpaceBetween).WithAlignment("", snippets.ComposedAlignCenter),
			snippets.SelectNode(slotSelect).WithNodeID("fresh_slot_select_node"),
			snippets.DividerNode(atoms.DividerAtom{
				ThicknessDp: atoms.Float64Ptr(1),
				Color:       atoms.ColorRef(theming.ColorDivider),
			}),
			snippets.KeyValueRowNode(coldChainRow),
			snippets.KeyValueRowNode(deliveryRow),
			snippets.KeyValueRowNode(savingsRow),
		),
	).WithNodeID("fresh_delivery_summary_root")

	return snippets.NewComposedBlockSnippet("fresh_delivery_summary_block", "fresh_delivery_summary", root).
		WithRecipeName("express_delivery_summary").
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionFadeIn,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("composed_recipe_viewed", map[string]any{
			"vertical": verticalID,
			"recipe":   "express_delivery_summary",
			"city":     city,
		}).
		Build()
}

func buildSearchSnippet(query string) snippets.SnippetEnvelope {
	return snippets.NewSearchBarSnippet("fresh_search_bar", "fresh_search", "Search avocados, A2 milk, sourdough, drinks...").
		WithQuery(query).
		WithLeadingIcon(atoms.NewIcon("search", 20, theming.ColorTextSecondary)).
		WithSubmitLabel("Search").
		WithOnSubmit(actions.NewNavigateAction("fresh_search_submit", actions.NavigatePayload{
			Route: "fresh-home?q={{query}}",
		})).
		WithOnClear(actions.NewNavigateAction("fresh_search_clear", actions.NavigatePayload{
			Route: screenHome,
		})).
		WithLayout(sectionLayout()).
		Build()
}

func buildCategoryChipsSnippet(activeFilter string) snippets.SnippetEnvelope {
	categories := []categoryChipSpec{
		{ID: "all", Label: "All"},
		{ID: "produce", Label: "Produce"},
		{ID: "dairy", Label: "Dairy"},
		{ID: "bakery", Label: "Bakery"},
		{ID: "snacks", Label: "Snacks"},
		{ID: "drinks", Label: "Drinks"},
	}

	chips := make([]snippets.FilterChipItem, 0, len(categories))
	for _, cat := range categories {
		nav := actions.NewNavigateAction("fresh_chip_"+cat.ID, actions.NavigatePayload{
			Route: "fresh-home?filter=" + cat.ID,
		})
		chips = append(chips, snippets.FilterChipItem{
			ID:       cat.ID,
			Label:    cat.Label,
			Selected: cat.ID == activeFilter,
			OnClick:  &nav,
		})
	}

	return snippets.NewChipGroupSnippet("fresh_category_chips", "fresh_categories", chips...).
		WithLayout(sectionLayout()).
		Build()
}

func buildPromoBannerSnippet(city, filter, query string, filteredCount, orderCount int) snippets.SnippetEnvelope {
	title := "8-Min Chilled Express Dispatch"
	body := fmt.Sprintf("%d essentials ready in %s · %d express orders delivered today", filteredCount, city, orderCount)
	tagLabel := "8 MINS"
	if query != "" || filter != "all" {
		title = fmt.Sprintf("%d Fresh Picks Ready", filteredCount)
		body = fmt.Sprintf("Filtered catalog in %s · %d express orders placed today", city, orderCount)
		tagLabel = fmt.Sprintf("%d ITEMS", filteredCount)
	} else if orderCount > 0 {
		tagLabel = fmt.Sprintf("%d ORDERS", orderCount)
	}

	layout := cardLayout()
	layout.BgColor = atoms.ColorRef(theming.ColorBrandContainer)

	return snippets.NewBannerSnippet(
		"fresh_promo_banner",
		atoms.NewText(title, theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithBody(atoms.NewText(body, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithIcon(atoms.NewIcon("star", 22, theming.ColorBrandPrimary)).
		WithTag(atoms.NewTag(tagLabel, theming.ColorSuccessContainer)).
		WithLayout(layout).
		WithTracking("fresh_banner_impression", map[string]any{
			"city":           city,
			"filter":         filter,
			"filtered_count": strconv.Itoa(filteredCount),
			"order_count":    strconv.Itoa(orderCount),
		}).
		Build()
}

func buildProductGridSnippet(items []groceryItem) snippets.SnippetEnvelope {
	cards := make([]snippets.SnippetEnvelope, 0, len(items))
	for idx, item := range items {
		cards = append(cards, buildProductCard(idx, item))
	}

	return snippets.NewGridSnippet("fresh_catalog_grid", 2).
		WithTitle(atoms.NewText("Daily Essentials", theming.TypographyTitleMedium, theming.ColorTextPrimary)).
		WithColumnsByWidth(snippets.ResponsiveColumnsSpec{
			Compact:  atoms.IntPtr(2),
			Medium:   atoms.IntPtr(3),
			Expanded: atoms.IntPtr(4),
		}).
		WithSpacingDp(12).
		AddItems(cards...).
		WithLayout(sectionLayout()).
		WithTracking("fresh_grid_viewed", map[string]any{
			"item_count": strconv.Itoa(len(items)),
		}).
		Build()
}

func buildProductCard(index int, item groceryItem) snippets.SnippetEnvelope {
	fieldID := "cart_qty_" + item.SKU
	incAction := actions.NewBatchAction("fresh_inc_"+item.SKU, actions.BatchPayload{
		Actions: []actions.ActionTrigger{
			actions.NewTriggerFeedbackAction("fresh_bounce_"+item.SKU, actions.TriggerFeedbackPayload{
				Effect:     actions.FeedbackCartBounce,
				TargetID:   "floating_cart_dock",
				DurationMs: 280,
			}),
			actions.NewTrackEventAction("fresh_track_inc_"+item.SKU, actions.TrackEventPayload{
				EventName: "fresh_cart_incremented",
				Properties: map[string]any{
					"sku":              item.SKU,
					"category":         item.Category,
					"unit_price_cents": strconv.Itoa(item.UnitPriceCents),
				},
			}),
		},
		StopOnError: true,
	})
	decAction := actions.NewTrackEventAction("fresh_dec_"+item.SKU, actions.TrackEventPayload{
		EventName: "fresh_cart_decremented",
		Properties: map[string]any{
			"sku":      item.SKU,
			"category": item.Category,
		},
	})
	stepper := atoms.NewStepper(fieldID, "ADD", item.InitialQty, &incAction, &decAction)
	stepper.MinQuantity = 0
	stepper.MaxQuantity = 10

	previewToast := actions.NewShowToastAction("fresh_preview_"+item.SKU, actions.ShowToastPayload{
		Message:    fmt.Sprintf("%s (%s) · Delivered chilled in %d mins", item.Title, item.PriceDisplay, expressETAMin),
		DurationMs: 2000,
		Variant:    "info",
	})

	return snippets.NewProductCardSnippet(
		"fresh_prod_"+item.SKU,
		item.SKU,
		atoms.NewText(item.Title, theming.TypographyTitleSmall, theming.ColorTextPrimary),
		atoms.NewText(item.PriceDisplay, theming.TypographyLabelLarge, theming.ColorTextPrimary),
		item.UnitPriceCents,
		stepper,
	).
		WithParentID("fresh_catalog_grid").
		WithSubtitle(atoms.NewText(item.Subtitle, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithOriginalPrice(atoms.NewText(item.OriginalPrice, theming.TypographyBodySmall, theming.ColorTextTertiary)).
		WithBadge(atoms.NewTag(item.Badge, item.BadgeColor)).
		WithImage(atoms.ImageAtom{
			URL:                fmt.Sprintf("https://cdn.abeta.dev/fresh/%s.png", item.SKU),
			ContentDescription: item.Title,
			HeightDp:           atoms.IntPtr(110),
			ScaleMode:          "crop",
			CornerRadius:       atoms.UniformRadius(theming.RadiusSM.Dp()),
		}).
		WithRating(atoms.NewText(item.Rating, theming.TypographyLabelSmall, theming.ColorSuccess)).
		WithOnClick(previewToast).
		WithLayout(compactCardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionScaleIn,
			DurationToken: atoms.MotionDurationFast,
			StaggerMs:     50,
			DelayMs:       index * 40,
		}).
		Build()
}

func buildEmptyStateSnippet(query, filter string) snippets.SnippetEnvelope {
	resetAction := actions.NewNavigateAction("fresh_reset_filters", actions.NavigatePayload{
		Route: screenHome,
	})
	msg := fmt.Sprintf("No groceries matched %q in %s. Clear your filters to browse all chilled essentials.", query, filter)
	if query == "" {
		msg = fmt.Sprintf("No groceries are currently listed under %q. Reset filters to view all items.", filter)
	}
	return snippets.NewEmptyStateSnippet(
		"fresh_empty_state",
		atoms.NewText("No matching groceries", theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithMessage(atoms.NewText(msg, theming.TypographyBodyMedium, theming.ColorTextSecondary)).
		WithIcon(atoms.NewIcon("search", 32, theming.ColorTextTertiary)).
		WithActionButton(atoms.NewButton("btn_fresh_reset_filters", "Reset Filters", "primary", &resetAction)).
		WithLayout(cardLayout()).
		Build()
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

func seedCatalog() []groceryItem {
	return []groceryItem{
		{
			SKU: "avocados", Title: "Fresh Hass Avocados", Subtitle: "2 pcs · Ready to eat",
			Category: "produce", PriceDisplay: "$3.49", OriginalPrice: "$4.49", UnitPriceCents: 349,
			Badge: "BESTSELLER", BadgeColor: theming.ColorSuccessContainer, Rating: "4.9 ★", InitialQty: 1,
		},
		{
			SKU: "strawberries", Title: "Organic Strawberries", Subtitle: "250 g · Sweet harvest",
			Category: "produce", PriceDisplay: "$4.99", OriginalPrice: "$5.99", UnitPriceCents: 499,
			Badge: "ORGANIC", BadgeColor: theming.ColorBrandContainer, Rating: "4.8 ★", InitialQty: 0,
		},
		{
			SKU: "cherry_tomatoes", Title: "Vine Cherry Tomatoes", Subtitle: "300 g · Greenhouse",
			Category: "produce", PriceDisplay: "$2.79", OriginalPrice: "$3.29", UnitPriceCents: 279,
			Badge: "LOCAL", BadgeColor: theming.ColorInfoContainer, Rating: "4.7 ★", InitialQty: 0,
		},
		{
			SKU: "a2_milk", Title: "A2 Farm Whole Milk", Subtitle: "1 L · Chilled glass bottle",
			Category: "dairy", PriceDisplay: "$3.29", OriginalPrice: "$3.79", UnitPriceCents: 329,
			Badge: "CHILLED", BadgeColor: theming.ColorInfoContainer, Rating: "4.9 ★", InitialQty: 0,
		},
		{
			SKU: "greek_yogurt", Title: "Greek Strained Yogurt", Subtitle: "400 g · 18g protein",
			Category: "dairy", PriceDisplay: "$3.99", OriginalPrice: "$4.49", UnitPriceCents: 399,
			Badge: "HIGH PROTEIN", BadgeColor: theming.ColorBrandContainer, Rating: "4.8 ★", InitialQty: 0,
		},
		{
			SKU: "sourdough", Title: "Artisan Sourdough Loaf", Subtitle: "450 g · Stone-baked",
			Category: "bakery", PriceDisplay: "$4.99", OriginalPrice: "$5.79", UnitPriceCents: 499,
			Badge: "BAKED TODAY", BadgeColor: theming.ColorWarningContainer, Rating: "4.9 ★", InitialQty: 1,
		},
		{
			SKU: "butter_croissant", Title: "Flaky Butter Croissants", Subtitle: "2 pcs · Warm oven pack",
			Category: "bakery", PriceDisplay: "$3.89", OriginalPrice: "$4.50", UnitPriceCents: 389,
			Badge: "WARM", BadgeColor: theming.ColorWarningContainer, Rating: "4.8 ★", InitialQty: 0,
		},
		{
			SKU: "kettle_chips", Title: "Kettle Sea Salt Chips", Subtitle: "140 g · Olive oil cooked",
			Category: "snacks", PriceDisplay: "$2.99", OriginalPrice: "$3.49", UnitPriceCents: 299,
			Badge: "CRUNCHY", BadgeColor: theming.ColorWarningContainer, Rating: "4.8 ★", InitialQty: 0,
		},
		{
			SKU: "tamari_almonds", Title: "Roasted Tamari Almonds", Subtitle: "180 g · Small batch",
			Category: "snacks", PriceDisplay: "$5.49", OriginalPrice: "$6.49", UnitPriceCents: 549,
			Badge: "ROASTED", BadgeColor: theming.ColorBrandContainer, Rating: "4.9 ★", InitialQty: 0,
		},
		{
			SKU: "oat_cold_brew", Title: "Oat Milk Cold Brew", Subtitle: "250 ml · Single origin",
			Category: "drinks", PriceDisplay: "$3.79", OriginalPrice: "$4.29", UnitPriceCents: 379,
			Badge: "CHILLED", BadgeColor: theming.ColorInfoContainer, Rating: "4.8 ★", InitialQty: 0,
		},
		{
			SKU: "yuzu_sparkling", Title: "Sparkling Yuzu Water", Subtitle: "330 ml · Zero sugar",
			Category: "drinks", PriceDisplay: "$1.99", OriginalPrice: "$2.49", UnitPriceCents: 199,
			Badge: "ZERO SUGAR", BadgeColor: theming.ColorSuccessContainer, Rating: "4.7 ★", InitialQty: 0,
		},
	}
}
