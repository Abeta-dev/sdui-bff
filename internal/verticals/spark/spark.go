// Package spark implements the Abeta Spark swipe discovery and live match vertical.
package spark

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
	verticalID    = "abeta-spark"
	screenDeck    = "spark-deck"
	screenMatches = "spark-matches"
	swipeEndpoint = "/api/v1/spark/swipe"
)

// Compile-time check ensuring Module satisfies domain.VerticalModule.
var _ domain.VerticalModule = (*Module)(nil)

// Module serves Abeta Spark swipe discovery SDUI screens and stateful swipe/match mutations.
type Module struct {
	mu        sync.RWMutex
	profiles  []discoveryProfile
	swipedIDs []string
	swipedSet map[string]bool
	matches   []domain.SparkMatch
	matchSeq  int
}

// New initializes the Abeta Spark vertical with curated profiles and pre-seeded matches.
func New() *Module {
	now := time.Now().UTC()
	return &Module{
		profiles:  seedProfiles(),
		swipedSet: make(map[string]bool),
		matchSeq:  1002,
		matches: []domain.SparkMatch{
			{
				MatchID:    "SPK-1001",
				ProfileID:  "spk_seed_elena",
				Name:       "Elena Vance",
				Role:       "Design Systems Lead",
				City:       "Bengaluru",
				SuperLiked: true,
				MatchedAt:  now.Add(-45 * time.Minute),
			},
			{
				MatchID:    "SPK-1002",
				ProfileID:  "spk_seed_arjun",
				Name:       "Arjun Rao",
				Role:       "Founding Engineer",
				City:       "Mumbai",
				SuperLiked: false,
				MatchedAt:  now.Add(-2 * time.Hour),
			},
		},
	}
}

// Descriptor returns the vertical metadata for the Super-App catalog.
func (m *Module) Descriptor() domain.AppDescriptor {
	return domain.AppDescriptor{
		ID:          verticalID,
		Name:        "Abeta Spark",
		Tagline:     "Curated Swipe Discovery, Super-Likes & Live Matches",
		Category:    "Social Discovery",
		TenantID:    verticalID,
		EntryScreen: screenDeck,
		ThemePreset: "amber",
		Screens:     []string{screenDeck, screenMatches},
	}
}

// Screens lists discoverable SDUI routes handled by Abeta Spark.
func (m *Module) Screens() []domain.ScreenSummary {
	return []domain.ScreenSummary{
		{
			ScreenID:    screenDeck,
			Title:       "Abeta Spark · Discovery Deck",
			Vertical:    verticalID,
			Description: "Interactive swipe card deck with live like, nope, super-like, rewind undo, and instant deck reset.",
		},
		{
			ScreenID:    screenMatches,
			Title:       "Abeta Spark · Your Matches",
			Vertical:    verticalID,
			Description: "Live mutual matches feed highlighting recent super-likes with one-tap icebreakers.",
		},
	}
}

// CanHandleScreen reports whether screenID belongs to Abeta Spark.
func (m *Module) CanHandleScreen(screenID string) bool {
	return screenID == screenDeck || screenID == screenMatches
}

// RenderScreen builds the SDUI PageResponse for spark-deck or spark-matches.
func (m *Module) RenderScreen(ctx context.Context, req domain.ScreenRequest) (snippets.PageResponse, error) {
	if err := ctx.Err(); err != nil {
		return snippets.PageResponse{}, err
	}
	switch req.ScreenID {
	case screenDeck:
		return m.renderDeck(req), nil
	case screenMatches:
		return m.renderMatches(), nil
	default:
		return snippets.PageResponse{}, fmt.Errorf("%w: %s", domain.ErrScreenNotFound, req.ScreenID)
	}
}

// RecordSwipe processes like, super_like, nope, undo, or reset swipe mutations safely.
func (m *Module) RecordSwipe(ctx context.Context, req domain.SwipeRequest) (domain.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActionResult{}, err
	}
	rawDirection := strings.TrimSpace(req.Direction)
	if rawDirection == "" {
		rawDirection = strings.TrimSpace(req.Action)
	}
	direction := strings.ToLower(rawDirection)

	switch direction {
	case "reset":
		return m.handleReset(), nil
	case "undo":
		return m.handleUndo(), nil
	case "like", "super_like", "nope":
		return m.handleProfileSwipe(strings.TrimSpace(req.ProfileID), direction)
	default:
		return domain.ActionResult{}, fmt.Errorf("%w: unsupported swipe direction %q", domain.ErrInvalidInput, req.Direction)
	}
}

// Matches returns a thread-safe snapshot of all persisted mutual matches.
func (m *Module) Matches() []domain.SparkMatch {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.SparkMatch, len(m.matches))
	copy(out, m.matches)
	return out
}

func (m *Module) handleReset() domain.ActionResult {
	m.mu.Lock()
	m.swipedIDs = nil
	m.swipedSet = make(map[string]bool)
	total := len(m.profiles)
	m.mu.Unlock()

	toast := actions.NewShowToastAction("spark_reset_toast", actions.ShowToastPayload{
		Message:    "Discovery deck reset — all profiles ready",
		DurationMs: 2200,
		Variant:    "info",
	})
	nav := actions.NewNavigateAction("spark_reset_nav", actions.NavigatePayload{
		Route: screenDeck,
	})
	return domain.ActionResult{
		Status:          "ok",
		Message:         "Deck reset",
		FollowUpActions: []actions.ActionTrigger{toast, nav},
		Data: map[string]any{
			"direction": "reset",
			"remaining": total,
		},
	}
}

func (m *Module) handleUndo() domain.ActionResult {
	m.mu.Lock()
	if len(m.swipedIDs) == 0 {
		remaining := len(m.profiles)
		m.mu.Unlock()
		toast := actions.NewShowToastAction("spark_undo_empty", actions.ShowToastPayload{
			Message:    "No previous swipe to rewind",
			DurationMs: 1800,
			Variant:    "info",
		})
		return domain.ActionResult{
			Status:          "ok",
			Message:         "Nothing to rewind",
			FollowUpActions: []actions.ActionTrigger{toast},
			Data: map[string]any{
				"direction": "undo",
				"rewound":   false,
				"remaining": remaining,
			},
		}
	}
	lastIdx := len(m.swipedIDs) - 1
	restoredID := m.swipedIDs[lastIdx]
	m.swipedIDs = m.swipedIDs[:lastIdx]
	delete(m.swipedSet, restoredID)
	remaining := len(m.profiles) - len(m.swipedSet)
	m.mu.Unlock()

	toast := actions.NewShowToastAction("spark_undo_toast", actions.ShowToastPayload{
		Message:    "Rewound last swipe",
		DurationMs: 2000,
		Variant:    "info",
	})
	return domain.ActionResult{
		Status:          "ok",
		Message:         "Rewound last swipe",
		FollowUpActions: []actions.ActionTrigger{toast},
		Data: map[string]any{
			"direction":  "undo",
			"rewound":    true,
			"profile_id": restoredID,
			"remaining":  remaining,
		},
	}
}

func (m *Module) handleProfileSwipe(profileID, direction string) (domain.ActionResult, error) {
	if profileID == "" {
		return domain.ActionResult{}, fmt.Errorf("%w: profile_id is required for %s", domain.ErrInvalidInput, direction)
	}

	m.mu.Lock()
	profile, ok := m.findProfileLocked(profileID)
	if !ok && strings.HasPrefix(profileID, "spark_") {
		// Accept dynamic CMS-authored profiles from Layout Engine without redeploying the BFF.
		slugName := strings.TrimPrefix(profileID, "spark_")
		displayName := strings.ReplaceAll(slugName, "_", " ")
		if len(displayName) > 0 {
			displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
		} else {
			displayName = "Featured Match"
		}
		profile = discoveryProfile{
			ID:         profileID,
			Name:       displayName,
			Role:       "Featured Discovery Member",
			City:       "Bengaluru",
			MutualLike: true,
		}
		ok = true
	}
	if !ok {
		m.mu.Unlock()
		return domain.ActionResult{}, fmt.Errorf("%w: profile %q", domain.ErrNotFound, profileID)
	}
	if !m.swipedSet[profile.ID] {
		m.swipedSet[profile.ID] = true
		m.swipedIDs = append(m.swipedIDs, profile.ID)
	}

	if direction == "nope" {
		remaining := len(m.profiles) - len(m.swipedSet)
		m.mu.Unlock()
		toast := actions.NewShowToastAction("spark_nope_"+profile.ID, actions.ShowToastPayload{
			Message:    "Passed",
			DurationMs: 1600,
			Variant:    "info",
		})
		return domain.ActionResult{
			Status:          "ok",
			Message:         "Passed",
			FollowUpActions: []actions.ActionTrigger{toast},
			Data: map[string]any{
				"profile_id": profile.ID,
				"direction":  direction,
				"matched":    false,
				"remaining":  remaining,
			},
		}, nil
	}

	isMatch := profile.MutualLike || direction == "super_like"
	var matchID string
	if isMatch {
		matchID = m.upsertMatchLocked(profile, direction == "super_like")
	}
	matchCount := len(m.matches)
	remaining := len(m.profiles) - len(m.swipedSet)
	m.mu.Unlock()

	if isMatch {
		celebration := actions.NewTriggerFeedbackAction("spark_match_celebration_"+profile.ID, actions.TriggerFeedbackPayload{
			Effect:     actions.FeedbackCelebration,
			Title:      "It's a Match!",
			Subtitle:   fmt.Sprintf("You and %s liked each other!", profile.Name),
			DurationMs: 3000,
		})
		toast := actions.NewShowToastAction("spark_match_toast_"+profile.ID, actions.ShowToastPayload{
			Message:    fmt.Sprintf("New match with %s · Say hi in Matches!", profile.Name),
			DurationMs: 2500,
			Variant:    "success",
		})
		return domain.ActionResult{
			Status:          "ok",
			Message:         fmt.Sprintf("Matched with %s", profile.Name),
			FollowUpActions: []actions.ActionTrigger{celebration, toast},
			Data: map[string]any{
				"profile_id":  profile.ID,
				"direction":   direction,
				"matched":     true,
				"match_id":    matchID,
				"match_count": matchCount,
				"remaining":   remaining,
			},
		}, nil
	}

	toast := actions.NewShowToastAction("spark_like_"+profile.ID, actions.ShowToastPayload{
		Message:    fmt.Sprintf("Liked %s!", profile.Name),
		DurationMs: 1800,
		Variant:    "info",
	})
	return domain.ActionResult{
		Status:          "ok",
		Message:         fmt.Sprintf("Liked %s!", profile.Name),
		FollowUpActions: []actions.ActionTrigger{toast},
		Data: map[string]any{
			"profile_id":  profile.ID,
			"direction":   direction,
			"matched":     false,
			"match_count": matchCount,
			"remaining":   remaining,
		},
	}, nil
}

func (m *Module) findProfileLocked(id string) (discoveryProfile, bool) {
	for _, p := range m.profiles {
		if p.ID == id {
			return p, true
		}
	}
	return discoveryProfile{}, false
}

func (m *Module) upsertMatchLocked(p discoveryProfile, superLiked bool) string {
	for i := range m.matches {
		if m.matches[i].ProfileID == p.ID {
			if superLiked {
				m.matches[i].SuperLiked = true
			}
			return m.matches[i].MatchID
		}
	}
	m.matchSeq++
	matchID := fmt.Sprintf("SPK-%04d", m.matchSeq)
	m.matches = append(m.matches, domain.SparkMatch{
		MatchID:    matchID,
		ProfileID:  p.ID,
		Name:       p.Name,
		Role:       p.Role,
		City:       p.City,
		SuperLiked: superLiked,
		MatchedAt:  time.Now().UTC(),
	})
	return matchID
}

func (m *Module) renderDeck(req domain.ScreenRequest) snippets.PageResponse {
	filter := normalizeFilter(req.Filter)

	m.mu.RLock()
	visible := make([]discoveryProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		if m.swipedSet[p.ID] {
			continue
		}
		if !matchesFilter(p, filter) {
			continue
		}
		visible = append(visible, p)
	}
	matchCount := len(m.matches)
	m.mu.RUnlock()

	matchesNav := actions.NewNavigateAction("spark_open_matches", actions.NavigatePayload{
		Route: screenMatches,
	})
	matchesBtn := atoms.NewButton("btn_spark_matches", fmt.Sprintf("Matches (%d)", matchCount), "outline", &matchesNav)

	header := snippets.NewHeaderSnippet(
		"spark_deck_header",
		atoms.NewText("Abeta Spark", theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText("Curated local creatives, founders & builders", theming.TypographyBodyMedium, theming.ColorTextSecondary)).
		WithTrailingAction(matchesBtn).
		WithLayout(sectionPadding()).
		Build()

	chips := buildFilterChips(filter)
	deck := buildSwipeDeck(visible)

	toolbarTitle := atoms.NewText("Abeta Spark", theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText(fmt.Sprintf("%d profiles ready · %d matches", len(visible), matchCount), theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenDeck, "Abeta Spark · Discovery").
		WithToolbar(snippets.ToolbarSpec{
			Title:    &toolbarTitle,
			Subtitle: &toolbarSub,
			BgColor:  atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(720),
		}).
		WithPullToRefresh(actions.NewRefreshPageAction("spark_pull_refresh", actions.RefreshPagePayload{
			PageID:     screenDeck,
			ShowLoader: true,
		})).
		AddPageAction(actions.NewTrackEventAction("spark_deck_viewed", actions.TrackEventPayload{
			EventName: "screen_viewed",
			Properties: map[string]any{
				"vertical":      verticalID,
				"screen_id":     screenDeck,
				"filter":        filter,
				"visible_cards": strconv.Itoa(len(visible)),
				"match_count":   strconv.Itoa(matchCount),
			},
		})).
		AddSnippets(header, chips, deck).
		WithTracking("page_loaded", map[string]any{
			"vertical":      verticalID,
			"page_id":       screenDeck,
			"filter":        filter,
			"visible_cards": strconv.Itoa(len(visible)),
			"match_count":   strconv.Itoa(matchCount),
		}).
		Build()
}

func (m *Module) renderMatches() snippets.PageResponse {
	matches := m.Matches()

	backNav := actions.NewNavigateAction("spark_matches_back", actions.NavigatePayload{Route: screenDeck})
	backIcon := atoms.NewIcon("arrow_back", 22, theming.ColorTextPrimary)
	backIcon.ContentDescription = "Back to Discovery Deck"
	backIcon.OnClick = &backNav
	backBtn := atoms.NewButton("btn_spark_back_deck", "Back to Deck", "outline", &backNav)

	header := snippets.NewHeaderSnippet(
		"spark_matches_header",
		atoms.NewText(fmt.Sprintf("Your Matches · %d", len(matches)), theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText("Tap any match to send an instant icebreaker", theming.TypographyBodyMedium, theming.ColorTextSecondary)).
		WithTrailingAction(backBtn).
		WithLayout(sectionPadding()).
		Build()

	spotlight := latestSpotlightMatch(matches)
	bannerTag := "NEW MATCH"
	bannerTagColor := theming.ColorSuccessContainer
	if spotlight.SuperLiked {
		bannerTag = "SUPER LIKE"
		bannerTagColor = theming.ColorWarningContainer
	}
	bannerBody := fmt.Sprintf("%s (%s · %s) is ready to chat — send a quick hello!", spotlight.Name, spotlight.Role, spotlight.City)
	spotlightToast := actions.NewShowToastAction("spark_spotlight_icebreaker", actions.ShowToastPayload{
		Message:    fmt.Sprintf("Icebreaker sent to %s!", spotlight.Name),
		DurationMs: 2400,
		Variant:    "success",
	})
	banner := snippets.NewBannerSnippet(
		"spark_match_spotlight",
		atoms.NewText("Spotlight Match · "+spotlight.Name, theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithBody(atoms.NewText(bannerBody, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithIcon(atoms.NewIcon("star", 24, theming.ColorBrandPrimary)).
		WithTag(atoms.NewTag(bannerTag, bannerTagColor)).
		WithCTA(atoms.NewButton("btn_spark_spotlight_hi", "Wave Hello", "primary", &spotlightToast)).
		WithLayout(cardLayout()).
		Build()

	pageSnippets := make([]snippets.SnippetEnvelope, 0, 2+len(matches))
	pageSnippets = append(pageSnippets, header, banner)

	// Render newest matches first so newly created matches surface immediately at the top.
	for i := len(matches) - 1; i >= 0; i-- {
		mt := matches[i]
		tagLabel := "MATCHED"
		tagColor := theming.ColorSuccessContainer
		if mt.SuperLiked {
			tagLabel = "SUPER LIKE"
			tagColor = theming.ColorWarningContainer
		}
		icebreaker := actions.NewShowToastAction("spark_icebreaker_"+mt.MatchID, actions.ShowToastPayload{
			Message:    fmt.Sprintf("Icebreaker sent to %s (%s)!", mt.Name, mt.City),
			DurationMs: 2400,
			Variant:    "success",
		})
		row := snippets.NewActionRowSnippet(
			"spark_match_row_"+mt.MatchID,
			atoms.NewText(mt.Name, theming.TypographyTitleSmall, theming.ColorTextPrimary),
		).
			WithSubtitle(atoms.NewText(fmt.Sprintf("%s · %s", mt.Role, mt.City), theming.TypographyBodySmall, theming.ColorTextSecondary)).
			WithLeadingIcon(atoms.NewIcon("person", 20, theming.ColorBrandPrimary)).
			WithTrailingTag(atoms.NewTag(tagLabel, tagColor)).
			WithTrailingIcon(atoms.NewIcon("chevron_right", 18, theming.ColorTextTertiary)).
			WithOnClick(icebreaker).
			WithLayout(cardLayout()).
			Build()
		pageSnippets = append(pageSnippets, row)
	}

	toolbarTitle := atoms.NewText(fmt.Sprintf("Your Matches · %d", len(matches)), theming.TypographyTitleLarge, theming.ColorTextPrimary)
	return snippets.NewPage(screenMatches, "Abeta Spark · Matches").
		WithToolbar(snippets.ToolbarSpec{
			Title:          &toolbarTitle,
			NavigationIcon: &backIcon,
			BgColor:        atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(720),
		}).
		AddSnippets(pageSnippets...).
		WithTracking("page_loaded", map[string]any{
			"vertical":    verticalID,
			"page_id":     screenMatches,
			"match_count": strconv.Itoa(len(matches)),
		}).
		Build()
}
