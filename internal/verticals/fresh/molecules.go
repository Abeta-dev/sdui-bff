package fresh

import (
	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/molecules"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
)

func buildHeroMediaPagerSnippet(city string) snippets.SnippetEnvelope {
	bannerTap := actions.NewNavigateAction("fresh_hero_banner_tap", actions.NavigatePayload{
		Route:  "fresh-home?filter=produce",
		Params: map[string]string{"campaign": "morning_harvest", "city": city},
	}).WithTracking("hero_banner_clicked", map[string]string{
		"vertical": verticalID,
		"surface":  screenHome,
		"city":     city,
	})

	pager := atoms.NewMediaPager("fresh_hero_pager", []atoms.ImageAtom{
		{
			ID:                 "fresh_hero_slide_1",
			URL:                "https://cdn.abeta.dev/fresh/hero-harvest.png",
			ContentDescription: "Morning Harvest 25% OFF",
			ScaleMode:          "crop",
			CornerRadius:       atoms.UniformRadius(theming.RadiusMD.Dp()),
		},
		{
			ID:                 "fresh_hero_slide_2",
			URL:                "https://cdn.abeta.dev/fresh/hero-dairy.png",
			ContentDescription: "Chilled A2 Dairy in 8 Mins",
			ScaleMode:          "crop",
			CornerRadius:       atoms.UniformRadius(theming.RadiusMD.Dp()),
		},
		{
			ID:                 "fresh_hero_slide_3",
			URL:                "https://cdn.abeta.dev/fresh/hero-bakery.png",
			ContentDescription: "Warm Artisan Sourdough",
			ScaleMode:          "crop",
			CornerRadius:       atoms.UniformRadius(theming.RadiusMD.Dp()),
		},
	}, 168)
	pager.AutoAdvanceMs = atoms.IntPtr(4500)
	pager.CornerRadius = atoms.UniformRadius(theming.RadiusMD.Dp())
	pager.OnPageClick = &bannerTap

	root := snippets.MediaPagerNode(pager).
		WithNodeID("fresh_hero_pager_node").
		WithLayout(sectionLayout()).
		WithTracking("node_impression", map[string]any{
			"component_id": "fresh_hero_pager_node",
			"vertical":     verticalID,
			"city":         city,
		})

	return snippets.NewComposedBlockSnippet("fresh_hero_media_block", "fresh_hero_media", root).
		WithRecipeName("express_hero_media_pager").
		WithTracking("snippet_impression", map[string]any{
			"snippet_id": "fresh_hero_media_block",
			"vertical":   verticalID,
			"city":       city,
		}).
		Build()
}

func buildExpressCategoryGridSnippet() snippets.SnippetEnvelope {
	catNav := func(catID string) *actions.ActionTrigger {
		act := actions.NewNavigateAction("fresh_cat_tile_"+catID, actions.NavigatePayload{
			Route: "fresh-home?filter=" + catID,
		}).WithTracking("category_tile_clicked", map[string]string{
			"vertical":    verticalID,
			"category_id": catID,
		})
		return &act
	}

	return molecules.CategoryGridSection(
		"fresh_category_grid",
		"fresh_cats",
		"Shop by Category",
		[]molecules.CategoryTileSpec{
			{
				CategoryID: "produce",
				Label:      "Fruits & Veg",
				Subtitle:   "Farm chilled",
				ImageURL:   "https://cdn.abeta.dev/fresh/cat-produce.png",
				BadgeLabel: "FRESH",
				OnClick:    catNav("produce"),
			},
			{
				CategoryID: "dairy",
				Label:      "Dairy & Eggs",
				Subtitle:   "A2 & protein",
				ImageURL:   "https://cdn.abeta.dev/fresh/cat-dairy.png",
				OnClick:    catNav("dairy"),
			},
			{
				CategoryID: "bakery",
				Label:      "Bakery",
				Subtitle:   "Warm daily",
				ImageURL:   "https://cdn.abeta.dev/fresh/cat-bakery.png",
				OnClick:    catNav("bakery"),
			},
			{
				CategoryID: "drinks",
				Label:      "Cold Drinks",
				Subtitle:   "Ice cold",
				ImageURL:   "https://cdn.abeta.dev/fresh/cat-drinks.png",
				OnClick:    catNav("drinks"),
			},
			{
				CategoryID: "snacks",
				Label:      "Munchies",
				Subtitle:   "Small batch",
				ImageURL:   "https://cdn.abeta.dev/fresh/cat-snacks.png",
				OnClick:    catNav("snacks"),
			},
			{
				CategoryID: "all",
				Label:      "All Aisles",
				Subtitle:   "8-min picks",
				ImageURL:   "https://cdn.abeta.dev/fresh/cat-all.png",
				BadgeLabel: "8 MIN",
				OnClick:    catNav("all"),
			},
		},
	)
}

func buildExpressBestsellersSnippet() snippets.SnippetEnvelope {
	incAvocado := actions.NewTriggerFeedbackAction("inc_express_avocados", actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCartBounce,
		TargetID:   "floating_cart_dock",
		DurationMs: 280,
	}).WithTracking("cart_item_incremented", map[string]string{
		"product_id": "avocados",
		"vertical":   verticalID,
	})
	decAvocado := actions.NewTrackEventAction("dec_express_avocados", actions.TrackEventPayload{
		EventName: "cart_item_decremented",
		Properties: map[string]any{
			"product_id": "avocados",
			"vertical":   verticalID,
		},
	})

	selectVariant := actions.NewSaveStateAction("save_avocado_pack", actions.SaveStatePayload{
		Key:   "variant_avocados",
		Value: "500g",
		Scope: "session",
	})
	confirmVariant := actions.NewDismissOverlayAction("confirm_avocado_pack", actions.DismissOverlayPayload{
		OverlayID: "avocado_pack_sheet",
	}).WithTracking("variant_confirmed", map[string]string{
		"product_id": "avocados",
		"vertical":   verticalID,
	})

	variantSheet := molecules.VariantPickerSheetAction(molecules.VariantPickerSpec{
		ActionID:          "open_fresh_avocado_variants",
		DialogID:          "avocado_pack_sheet",
		ProductID:         "avocados",
		Title:             "Fresh Hass Avocados",
		Subtitle:          "8-min chilled delivery · Pick pack weight",
		ImageURL:          "https://cdn.abeta.dev/fresh/avocados.png",
		SelectedID:        "500g",
		PriceText:         "$3.49",
		OriginalPriceText: "$4.49",
		DiscountLabel:     "22% OFF",
		Options: []atoms.OptionItem{
			{ID: "500g", Label: "500 g", Subtitle: "2 pcs · $3.49", Badge: "POPULAR"},
			{ID: "1kg", Label: "1 kg", Subtitle: "4 pcs · $6.29", Badge: "SAVE 15%"},
			{ID: "2kg", Label: "2 kg", Subtitle: "8 pcs · $11.49", Badge: "BEST VALUE"},
		},
		OnSelectChange: &selectVariant,
		PositiveAction: &confirmVariant,
		SheetScreenID:  "variant-picker-avocados",
	})

	favAvocado := actions.NewShowToastAction("fav_express_avocados", actions.ShowToastPayload{
		Message:    "Saved Fresh Hass Avocados to favorites",
		DurationMs: 1600,
		Variant:    "success",
	}).WithTracking("product_favorited", map[string]string{
		"product_id": "avocados",
		"vertical":   verticalID,
	})

	tiles := []snippets.ComposedNode{
		molecules.ExpressProductTile(molecules.ProductTileSpec{
			ProductID:         "avocados",
			Title:             "Fresh Hass Avocados",
			Subtitle:          "500 g · Ready to eat",
			ImageURL:          "https://cdn.abeta.dev/fresh/avocados.png",
			PriceText:         "$3.49",
			OriginalPriceText: "$4.49",
			DiscountRibbon:    "22% OFF",
			EtaLabel:          "8 MINS",
			Rating:            4.9,
			ReviewCount:       atoms.IntPtr(612),
			StepperFieldID:    "express_qty_avocados",
			Quantity:          1,
			OnIncrement:       &incAvocado,
			OnDecrement:       &decAvocado,
			VariantLabel:      "500 g ▾",
			OnVariantClick:    &variantSheet,
			OnFavorite:        &favAvocado,
		}),
		molecules.ExpressProductTile(molecules.ProductTileSpec{
			ProductID:         "sourdough",
			Title:             "Artisan Sourdough Loaf",
			Subtitle:          "450 g · Stone-baked",
			ImageURL:          "https://cdn.abeta.dev/fresh/sourdough.png",
			PriceText:         "$4.99",
			OriginalPriceText: "$5.79",
			DiscountRibbon:    "14% OFF",
			TopEndBadge:       "VEG",
			EtaLabel:          "8 MINS",
			Rating:            4.8,
			ReviewCount:       atoms.IntPtr(384),
			StepperFieldID:    "express_qty_sourdough",
			Quantity:          1,
			OnIncrement:       &incAvocado,
			OnDecrement:       &decAvocado,
		}),
	}

	root := snippets.VStack(
		theming.SpacingSM.Dp(),
		snippets.TextNode(atoms.NewText("Bestsellers Near You", theming.TypographyTitleMedium, theming.ColorTextPrimary)).
			WithNodeID("fresh_bestsellers_title"),
		snippets.GridNode(2, theming.SpacingSM.Dp(), tiles...).
			WithNodeID("fresh_bestsellers_grid").
			WithColumnsByWidth(snippets.ResponsiveColumnsSpec{
				Compact:  atoms.IntPtr(2),
				Medium:   atoms.IntPtr(3),
				Expanded: atoms.IntPtr(4),
			}),
	).
		WithNodeID("fresh_bestsellers_root").
		WithLayout(sectionLayout())

	return snippets.NewComposedBlockSnippet("fresh_bestsellers_block", "fresh_bestsellers", root).
		WithRecipeName("express_product_grid").
		WithTracking("snippet_impression", map[string]any{
			"snippet_id": "fresh_bestsellers_block",
			"vertical":   verticalID,
		}).
		Build()
}

func buildImpulseUpsellRailSnippet() snippets.SnippetEnvelope {
	quickAdd := actions.NewTriggerFeedbackAction("fresh_upsell_quick_add", actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCartBounce,
		TargetID:   "floating_cart_dock",
		DurationMs: 240,
	})
	return molecules.ImpulseUpsellRail(
		"fresh_upsell_rail",
		"fresh_upsell",
		"Before You Checkout",
		[]molecules.ProductTileSpec{
			{
				ProductID:         "yuzu_sparkling",
				Title:             "Sparkling Yuzu Water",
				Subtitle:          "330 ml · Chilled",
				ImageURL:          "https://cdn.abeta.dev/fresh/yuzu_sparkling.png",
				PriceText:         "$1.99",
				OriginalPriceText: "$2.49",
				DiscountRibbon:    "20% OFF",
				EtaLabel:          "8 MINS",
				OnIncrement:       &quickAdd,
			},
			{
				ProductID:         "kettle_chips",
				Title:             "Kettle Sea Salt Chips",
				Subtitle:          "140 g · Olive oil",
				ImageURL:          "https://cdn.abeta.dev/fresh/kettle_chips.png",
				PriceText:         "$2.99",
				OriginalPriceText: "$3.49",
				TopEndBadge:       "VEG",
				EtaLabel:          "8 MINS",
				OnIncrement:       &quickAdd,
			},
			{
				ProductID:         "oat_cold_brew",
				Title:             "Oat Milk Cold Brew",
				Subtitle:          "250 ml · Single origin",
				ImageURL:          "https://cdn.abeta.dev/fresh/oat_cold_brew.png",
				PriceText:         "$3.79",
				OriginalPriceText: "$4.29",
				DiscountRibbon:    "12% OFF",
				EtaLabel:          "8 MINS",
				OnIncrement:       &quickAdd,
			},
		},
	)
}

func buildDeliveryInstructionsSnippet() snippets.SnippetEnvelope {
	savePref := actions.NewSaveStateAction("save_fresh_delivery_pref", actions.SaveStatePayload{
		Key:   "leave_at_door",
		Value: true,
		Scope: "session",
	})
	return molecules.DeliveryInstructionsCard(
		"fresh_delivery_instructions",
		"fresh_deliv",
		molecules.DeliveryInstructionsSpec{
			Title:                "Delivery Instructions",
			LeaveAtDoorFieldID:   "fresh_leave_at_door",
			LeaveAtDoor:          true,
			OnLeaveAtDoorChange:  &savePref,
			DontRingBellFieldID:  "fresh_dont_ring_bell",
			DontRingBellLabel:    "Don't ring the doorbell",
			DontRingBell:         true,
			OnDontRingBellChange: &savePref,
			NotesFieldID:         "fresh_delivery_notes",
			NotesPlaceholder:     "Leave at security desk or call on arrival",
			TipChips: []molecules.TipChipOption{
				{ID: "1", Label: "$1", OnClick: &savePref},
				{ID: "2", Label: "$2", Selected: true, OnClick: &savePref},
				{ID: "3", Label: "$3", OnClick: &savePref},
			},
			TipSliderFieldID:  "fresh_custom_tip",
			TipSliderLabel:    "Custom Rider Tip",
			TipSliderValue:    2,
			TipSliderMax:      10,
			OnTipSliderChange: &savePref,
		},
	)
}
