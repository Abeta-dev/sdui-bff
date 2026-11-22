package spark

import (
	"fmt"
	"strings"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

type discoveryProfile struct {
	ID           string
	Name         string
	Age          int
	Role         string
	City         string
	Bio          string
	DistanceKm   int
	DistanceText string
	Verified     bool
	DesignTech   bool
	Tags         []string
	MutualLike   bool
}

func buildPrimeBoostNudge(visibleCount, matchCount int) snippets.SnippetEnvelope {
	boostBadge := atoms.NewTag("PRIME BOOST", theming.ColorWarningContainer)
	boostBadge.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopHeartbeat}

	boostAct := actions.NewTriggerFeedbackAction("spark_prime_boost_fx", actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCelebration,
		Title:      "Prime Boost Activated!",
		Subtitle:   "3× discovery visibility live for the next 30 minutes",
		DurationMs: 2400,
	})
	boostCTA := atoms.NewButton("btn_spark_prime_boost", "Boost Now", "primary", &boostAct)
	boostCTA.Size = "sm"

	return snippets.NewNudgeBarSnippet(
		"spark_prime_boost_nudge",
		"spark_prime_boost",
		atoms.NewText("Prime Boost · 3× Profile Visibility", theming.TypographyTitleSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(
			fmt.Sprintf("%d curated profiles ready · %d active matches in your inbox", visibleCount, matchCount),
			theming.TypographyBodySmall,
			theming.ColorTextSecondary,
		)).
		WithIcon(atoms.NewIcon("star", 20, theming.ColorWarning)).
		WithBadge(boostBadge).
		WithProgress(0.85).
		WithCTA(boostCTA).
		WithDismissible(true).
		WithPointerEdge(snippets.PointerEdgeTop).
		WithOnDismiss(actions.NewTrackEventAction("spark_boost_nudge_dismissed", actions.TrackEventPayload{
			EventName: "nudge_dismissed",
			Properties: map[string]any{
				"vertical": verticalID,
				"nudge_id": "spark_prime_boost",
			},
		})).
		WithLayout(cardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideDown,
			DurationToken: atoms.MotionDurationFast,
		}).
		WithTracking("spark_nudge_impression", map[string]any{
			"vertical": verticalID,
			"nudge_id": "spark_prime_boost",
		}).
		Build()
}

func buildFilterChips(activeFilter string) snippets.SnippetEnvelope {
	filterSpecs := []struct {
		id    string
		label string
	}{
		{id: "nearby", label: "Nearby"},
		{id: "verified", label: "Verified"},
		{id: "design_tech", label: "Design & Tech"},
	}

	builder := snippets.NewChipGroupSnippet("spark_filter_chips", "spark_filters").
		WithLayout(sectionPadding())

	for _, f := range filterSpecs {
		selected := activeFilter == f.id
		route := fmt.Sprintf("%s?filter=%s", screenDeck, f.id)
		params := map[string]string{"filter": f.id}
		if selected {
			route = screenDeck
			params = nil
		}
		nav := actions.NewNavigateAction("spark_filter_"+f.id, actions.NavigatePayload{
			Route:  route,
			Params: params,
		})
		builder.AddChip(snippets.FilterChipItem{
			ID:       f.id,
			Label:    f.label,
			Selected: selected,
			OnClick:  &nav,
		})
	}

	resetAct := actions.NewCallAPIAction("spark_chip_reset", actions.CallAPIPayload{
		Endpoint: swipeEndpoint,
		Method:   "POST",
		Body:     map[string]any{"direction": "reset"},
	})
	builder.AddChip(snippets.FilterChipItem{
		ID:      "reset_deck",
		Label:   "Reset Deck",
		Icon:    &atoms.IconAtom{Name: "refresh", SizeDp: atoms.IntPtr(16), Color: atoms.ColorRef(theming.ColorBrandPrimary)},
		OnClick: &resetAct,
	})
	return builder.Build()
}

func buildSwipeDeck(profiles []discoveryProfile) snippets.SnippetEnvelope {
	undoAct := actions.NewCallAPIAction("spark_deck_undo", actions.CallAPIPayload{
		Endpoint: swipeEndpoint,
		Method:   "POST",
		Body:     map[string]any{"direction": "undo"},
	})

	deck := snippets.NewSwipeDeckSnippet("spark_swipe_deck", "spark_discovery").
		WithLabels("LIKE", "NOPE", "SUPER LIKE").
		WithEmptyState(
			atoms.NewText("You're all caught up!", theming.TypographyTitleMedium, theming.ColorTextPrimary),
			atoms.NewText("Tap Reset Deck to replay profiles or visit Matches to start chatting.", theming.TypographyBodyMedium, theming.ColorTextSecondary),
		).
		WithOnUndo(undoAct).
		WithLayout(sectionPadding())

	for _, p := range profiles {
		likeAct := actions.NewCallAPIAction("spark_like_"+p.ID, actions.CallAPIPayload{
			Endpoint: swipeEndpoint,
			Method:   "POST",
			Body:     map[string]any{"profile_id": p.ID, "direction": "like"},
		})
		nopeAct := actions.NewCallAPIAction("spark_nope_"+p.ID, actions.CallAPIPayload{
			Endpoint: swipeEndpoint,
			Method:   "POST",
			Body:     map[string]any{"profile_id": p.ID, "direction": "nope"},
		})
		superAct := actions.NewCallAPIAction("spark_super_"+p.ID, actions.CallAPIPayload{
			Endpoint: swipeEndpoint,
			Method:   "POST",
			Body:     map[string]any{"profile_id": p.ID, "direction": "super_like"},
		})
		previewAct := actions.NewShowToastAction("spark_preview_"+p.ID, actions.ShowToastPayload{
			Message:    fmt.Sprintf("%s · %s in %s", p.Name, p.Role, p.City),
			DurationMs: 1800,
			Variant:    "info",
		})

		sub := atoms.NewText(fmt.Sprintf("%s · %s", p.Role, p.City), theming.TypographyBodyMedium, theming.ColorTextSecondary)
		bio := atoms.NewText(p.Bio, theming.TypographyBodySmall, theming.ColorTextPrimary)
		dist := atoms.NewText(p.DistanceText, theming.TypographyLabelSmall, theming.ColorTextTertiary)
		badge := atoms.NewTag("NEW", theming.ColorInfoContainer)
		if p.Verified {
			badge = atoms.NewTag("VERIFIED", theming.ColorSuccessContainer)
		}

		tags := make([]atoms.TagAtom, 0, len(p.Tags))
		for _, t := range p.Tags {
			tags = append(tags, atoms.NewTag(t, theming.ColorBrandContainer))
		}

		deck.AddCard(snippets.SwipeCardItem{
			CardID:       p.ID,
			Title:        atoms.NewText(fmt.Sprintf("%s, %d", p.Name, p.Age), theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
			Subtitle:     &sub,
			Bio:          &bio,
			Badge:        &badge,
			DistanceText: &dist,
			Image: &atoms.ImageAtom{
				URL:                fmt.Sprintf("https://cdn.abeta.dev/spark/%s.png", p.ID),
				ContentDescription: p.Name,
				HeightDp:           atoms.IntPtr(260),
				ScaleMode:          "crop",
				CornerRadius:       atoms.UniformRadius(theming.RadiusLG.Dp()),
			},
			Tags:        tags,
			OnLike:      &likeAct,
			OnNope:      &nopeAct,
			OnSuperLike: &superAct,
			OnClick:     &previewAct,
		})
	}
	return deck.Build()
}

func latestSpotlightMatch(matches []domain.SparkMatch) domain.SparkMatch {
	for i := len(matches) - 1; i >= 0; i-- {
		if matches[i].SuperLiked {
			return matches[i]
		}
	}
	if len(matches) > 0 {
		return matches[len(matches)-1]
	}
	return domain.SparkMatch{Name: "Abeta Community", Role: "Discovery", City: "Bengaluru"}
}

func normalizeFilter(raw string) string {
	f := strings.ToLower(strings.TrimSpace(raw))
	switch f {
	case "nearby", "verified", "design_tech":
		return f
	case "design", "tech", "design & tech":
		return "design_tech"
	default:
		return "all"
	}
}

func matchesFilter(p discoveryProfile, filter string) bool {
	switch filter {
	case "nearby":
		return p.DistanceKm <= 5
	case "verified":
		return p.Verified
	case "design_tech":
		return p.DesignTech
	default:
		return true
	}
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

func seedProfiles() []discoveryProfile {
	return []discoveryProfile{
		{
			ID: "spk_aanya", Name: "Aanya Sharma", Age: 27, Role: "Staff Product Designer", City: "Bengaluru",
			Bio:        "Building tactile mobile interfaces, espresso rituals & weekend ceramic studios.",
			DistanceKm: 2, DistanceText: "2 km away", Verified: true, DesignTech: true,
			Tags: []string{"Design Systems", "Espresso", "Ceramics"}, MutualLike: true,
		},
		{
			ID: "spk_kabir", Name: "Kabir Menon", Age: 29, Role: "Principal Systems Architect", City: "Bengaluru",
			Bio:        "Distributed engines by day, vinyl crates and sunrise trail runs by morning.",
			DistanceKm: 3, DistanceText: "3 km away", Verified: true, DesignTech: true,
			Tags: []string{"Go & Rust", "Vinyl", "Trail Running"}, MutualLike: true,
		},
		{
			ID: "spk_meera", Name: "Meera Krishnan", Age: 26, Role: "Creative Technologist", City: "Mumbai",
			Bio:        "Interactive shaders, kinetic typography, and late-night ramen spots.",
			DistanceKm: 4, DistanceText: "4 km away", Verified: true, DesignTech: true,
			Tags: []string{"Creative Code", "Ramen", "Cinema"}, MutualLike: false,
		},
		{
			ID: "spk_rohan", Name: "Rohan Deshmukh", Age: 30, Role: "Culinary Director", City: "Pune",
			Bio:        "Sourdough fermentation, coastal spice trails, and acoustic guitar sessions.",
			DistanceKm: 7, DistanceText: "7 km away", Verified: true, DesignTech: false,
			Tags: []string{"Artisan Baking", "Acoustic", "Travel"}, MutualLike: true,
		},
		{
			ID: "spk_zara", Name: "Zara Patel", Age: 28, Role: "AI Research Lead", City: "Hyderabad",
			Bio:        "Multimodal models, indoor bouldering gyms, and indie sci-fi book clubs.",
			DistanceKm: 2, DistanceText: "2 km away", Verified: false, DesignTech: true,
			Tags: []string{"AI Research", "Bouldering", "Sci-Fi"}, MutualLike: true,
		},
		{
			ID: "spk_dev", Name: "Dev Malhotra", Age: 31, Role: "Architectural Photographer", City: "Delhi",
			Bio:        "Brutalist geometry, analog 35mm film rolls, and rooftop jazz nights.",
			DistanceKm: 9, DistanceText: "9 km away", Verified: false, DesignTech: false,
			Tags: []string{"35mm Film", "Architecture", "Jazz"}, MutualLike: false,
		},
	}
}
