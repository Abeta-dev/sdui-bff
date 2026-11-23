package pulse

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
)

type seatTierSpec struct {
	Name         string
	PriceDisplay string
	PriceUSD     float64
	Perks        string
	SeatsLeft    int
	BadgeColor   theming.ColorToken
}

type eventSpec struct {
	ID            string
	Title         string
	Category      string
	CategoryLabel string
	Venue         string
	City          string
	Schedule      string
	StartingPrice string
	Description   string
	Rating        string
	Badge         string
	Tiers         []seatTierSpec
}

func buildBookAPIAction(evt eventSpec, seatTier string, qty int, city string) actions.ActionTrigger {
	tierKey := strings.ToLower(strings.ReplaceAll(seatTier, " ", "_"))
	return actions.NewCallAPIAction("pulse_book_"+evt.ID+"_"+tierKey, actions.CallAPIPayload{
		Endpoint: "/api/v1/pulse/bookings",
		Method:   "POST",
		Body: map[string]any{
			"event_id":   evt.ID,
			"event_name": evt.Title,
			"seat_tier":  seatTier,
			"quantity":   qty,
			"city":       city,
		},
	})
}

func buildEarlyBirdNudge(city string, spotlight eventSpec) snippets.SnippetEnvelope {
	badge := atoms.NewTag("EARLY BIRD", theming.ColorWarningContainer)
	badge.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopWiggle}

	countdown := atoms.NewCountdown("pulse_early_bird_timer", 540, "Early-bird seats closing in")
	countdown.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopHeartbeat}

	spotlightNav := actions.NewNavigateAction("pulse_early_bird_open", actions.NavigatePayload{
		Route: "pulse-event-detail?id=" + spotlight.ID,
	})
	ctaBtn := atoms.NewButton("btn_pulse_early_bird", "Lock Seats", "primary", &spotlightNav)
	ctaBtn.Size = "sm"

	return snippets.NewNudgeBarSnippet(
		"pulse_early_bird_nudge",
		"pulse_early_bird_nudge",
		atoms.NewText("Early-Bird Passes Live in "+city, theming.TypographyTitleSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(fmt.Sprintf("%s · Reserve front-row pricing before tier jump", spotlight.Title), theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithIcon(atoms.NewIcon("star", 20, theming.ColorBrandPrimary)).
		WithBadge(badge).
		WithCountdown(countdown).
		WithProgress(0.82).
		WithCTA(ctaBtn).
		WithDismissible(true).
		WithPointerEdge(snippets.PointerEdgeBottom).
		WithOnDismiss(actions.NewTrackEventAction("pulse_nudge_dismissed", actions.TrackEventPayload{
			EventName: "nudge_dismissed",
			Properties: map[string]any{
				"vertical": verticalID,
				"nudge_id": "pulse_early_bird_nudge",
			},
		})).
		WithLayout(cardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideDown,
			DurationToken: atoms.MotionDurationFast,
		}).
		WithTracking("pulse_nudge_impression", map[string]any{
			"vertical": verticalID,
			"nudge_id": "pulse_early_bird_nudge",
			"city":     city,
		}).
		Build()
}

func buildEventEPassPreviewBlock(evt eventSpec, city string) snippets.SnippetEnvelope {
	ratingVal := 4.8
	if fields := strings.Fields(evt.Rating); len(fields) > 0 {
		if parsed, err := strconv.ParseFloat(fields[0], 64); err == nil && parsed > 0 {
			ratingVal = parsed
		}
	}

	earlyBirdAlert := atoms.NewAlertBanner(
		"pulse_detail_early_bird_alert",
		"Early-Bird Tier Pricing Active",
		fmt.Sprintf("Reserve %s seats in %s before tier pricing closes — instant QR e-pass delivered.", evt.Title, city),
		atoms.VariantWarning,
	)

	rating := atoms.NewRating("pulse_event_rating_"+evt.ID, ratingVal, 5)
	rating.ReviewCount = atoms.IntPtr(1240)
	rating.Label = "Verified Attendee Rating"

	qrPayload := fmt.Sprintf("abeta-pulse://epass/%s?city=%s&venue=%s", evt.ID, city, evt.Venue)
	qr := atoms.NewQrCode("pulse_epass_qr_"+evt.ID, qrPayload, "Mobile E-Pass Preview · "+evt.Badge, 156)
	qr.Caption = fmt.Sprintf("Fast-lane QR entry at %s · %s", evt.Venue, evt.Schedule)

	root := snippets.CardNode(
		cardLayout(),
		snippets.VStack(
			theming.SpacingMD.Dp(),
			snippets.AlertBannerNode(earlyBirdAlert).WithNodeID("pulse_early_bird_alert_node"),
			snippets.RatingNode(rating).WithNodeID("pulse_rating_node"),
			snippets.DividerNode(atoms.DividerAtom{
				ThicknessDp: atoms.Float64Ptr(1),
				Color:       atoms.ColorRef(theming.ColorDivider),
			}),
			snippets.QrCodeNode(qr).WithNodeID("pulse_epass_qr_node").WithContentAlignment(snippets.ComposedAlignCenter),
		),
	).WithNodeID("pulse_epass_preview_root")

	return snippets.NewComposedBlockSnippet("pulse_epass_preview_block", "pulse_epass_"+evt.ID, root).
		WithRecipeName("event_epass_preview").
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideUp,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("composed_recipe_viewed", map[string]any{
			"vertical": verticalID,
			"recipe":   "event_epass_preview",
			"event_id": evt.ID,
		}).
		Build()
}

func buildCategoryChips(activeFilter string) snippets.SnippetEnvelope {
	specs := []struct {
		id    string
		label string
	}{
		{"all", "All"},
		{"movies", "Movies"},
		{"concerts", "Concerts"},
		{"comedy", "Comedy"},
		{"dining", "Dining"},
	}
	chips := make([]snippets.FilterChipItem, 0, len(specs))
	for _, sp := range specs {
		nav := actions.NewNavigateAction("pulse_filter_"+sp.id, actions.NavigatePayload{
			Route: "pulse-home?filter=" + sp.id,
		})
		chips = append(chips, snippets.FilterChipItem{
			ID:       sp.id,
			Label:    sp.label,
			Selected: sp.id == activeFilter,
			OnClick:  &nav,
		})
	}
	return snippets.NewChipGroupSnippet("pulse_category_chips", "pulse_categories", chips...).
		WithLayout(sectionLayout()).
		Build()
}

func buildEventCard(index int, evt eventSpec) snippets.SnippetEnvelope {
	nav := actions.NewNavigateAction("pulse_open_"+evt.ID, actions.NavigatePayload{
		Route: "pulse-event-detail?id=" + evt.ID,
	})
	return snippets.NewCardSnippet(
		"pulse_evt_"+evt.ID,
		atoms.NewText(evt.Title, theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithParentID("pulse_events_grid").
		WithSubtitle(atoms.NewText(fmt.Sprintf("%s · %s", evt.Venue, evt.Schedule), theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithDescription(atoms.NewText(fmt.Sprintf("From %s · %s · %s", evt.StartingPrice, evt.Rating, evt.CategoryLabel), theming.TypographyBodySmall, theming.ColorTextTertiary)).
		WithBadge(atoms.NewTag(evt.Badge, theming.ColorBrandContainer)).
		WithPrimaryButton(atoms.NewButton("btn_evt_"+evt.ID, "View Seats", "primary", &nav)).
		WithOnClick(nav).
		WithLayout(compactCardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionScaleIn,
			DurationToken: atoms.MotionDurationFast,
			DelayMs:       index * 45,
		}).
		WithTracking("pulse_event_impression", map[string]any{
			"event_id": evt.ID,
			"category": evt.Category,
		}).
		Build()
}

func filterEvents(events []eventSpec, filter, query string) []eventSpec {
	q := strings.ToLower(query)
	out := make([]eventSpec, 0, len(events))
	for _, evt := range events {
		if filter != "all" && evt.Category != filter {
			continue
		}
		if q != "" {
			haystack := strings.ToLower(evt.Title + " " + evt.Category + " " + evt.Venue + " " + evt.Description + " " + evt.Badge)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		out = append(out, evt)
	}
	return out
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

func defaultTiers(goldUSD, vipUSD, balconyUSD float64) []seatTierSpec {
	return []seatTierSpec{
		{Name: "Gold", PriceDisplay: fmt.Sprintf("$%.0f", goldUSD), PriceUSD: goldUSD, Perks: "Prime center rows · Fast-lane QR check-in", SeatsLeft: 24, BadgeColor: theming.ColorWarningContainer},
		{Name: "VIP Lounge", PriceDisplay: fmt.Sprintf("$%.0f", vipUSD), PriceUSD: vipUSD, Perks: "Front lounge seating · Complimentary refreshments", SeatsLeft: 10, BadgeColor: theming.ColorBrandContainer},
		{Name: "Balcony", PriceDisplay: fmt.Sprintf("$%.0f", balconyUSD), PriceUSD: balconyUSD, Perks: "Panoramic elevated view · Instant e-pass", SeatsLeft: 42, BadgeColor: theming.ColorSuccessContainer},
	}
}

func seedEvents() []eventSpec {
	return []eventSpec{
		{
			ID: "evt-orbital-imax", Title: "Orbital Drift: IMAX 70mm", Category: "movies", CategoryLabel: "Movies",
			Venue: "Indiranagar Grand Cineplex", City: defaultCity, Schedule: "Fri, 7:30 PM", StartingPrice: "$10",
			Description: "Laser-projected 70mm sci-fi epic with spatial Dolby Atmos soundstage.", Rating: "4.9 ★", Badge: "IMAX 70MM",
			Tiers: defaultTiers(16, 28, 10),
		},
		{
			ID: "evt-synth-symphony", Title: "Midnight Synth Symphony Live", Category: "concerts", CategoryLabel: "Concerts",
			Venue: "Whitefield Open Arena", City: defaultCity, Schedule: "Sat, 8:00 PM", StartingPrice: "$28",
			Description: "32-piece string ensemble performing live analog electronic arrangements under the stars.", Rating: "4.8 ★", Badge: "LIVE STAGE",
			Tiers: defaultTiers(45, 95, 28),
		},
		{
			ID: "evt-punchline-lab", Title: "Punchline Lab: Late Set", Category: "comedy", CategoryLabel: "Comedy",
			Venue: "Koramangala Comedy Loft", City: defaultCity, Schedule: "Sat, 9:30 PM", StartingPrice: "$12",
			Description: "90-minute unfiltered stand-up showcase featuring four touring headliners.", Rating: "4.7 ★", Badge: "SELLING FAST",
			Tiers: defaultTiers(18, 32, 12),
		},
		{
			ID: "evt-chefs-table", Title: "Monsoon Tasting Menu (7-Course)", Category: "dining", CategoryLabel: "Dining",
			Venue: "Lavelle Terrace Kitchen", City: defaultCity, Schedule: "Sun, 7:00 PM", StartingPrice: "$48",
			Description: "Seasonal coastal botanicals paired with artisanal zero-proof infusions.", Rating: "4.9 ★", Badge: "CHEF TABLE",
			Tiers: defaultTiers(65, 110, 48),
		},
		{
			ID: "evt-noir-premiere", Title: "Velvet Horizon: Director's Cut", Category: "movies", CategoryLabel: "Movies",
			Venue: "MG Road Art Cinema", City: defaultCity, Schedule: "Sun, 6:00 PM", StartingPrice: "$9",
			Description: "Restored neo-noir thriller screening followed by live cinematographer Q&A.", Rating: "4.6 ★", Badge: "PREMIERE",
			Tiers: defaultTiers(14, 24, 9),
		},
		{
			ID: "evt-acoustic-rooftop", Title: "Rooftop Acoustic Sessions", Category: "concerts", CategoryLabel: "Concerts",
			Venue: "Courtyard Skydeck", City: defaultCity, Schedule: "Fri, 8:30 PM", StartingPrice: "$24",
			Description: "Intimate unplugged indie showcase with skyline sunset acoustics.", Rating: "4.8 ★", Badge: "ROOFTOP",
			Tiers: defaultTiers(35, 72, 24),
		},
	}
}
