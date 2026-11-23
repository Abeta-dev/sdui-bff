// Package pulse implements the Abeta Pulse events, movies, concerts, comedy, and dining vertical.
package pulse

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
	verticalID        = "abeta-pulse"
	screenHome        = "pulse-home"
	screenEventDetail = "pulse-event-detail"
	defaultCity       = "Bengaluru"
	defaultTicketsQty = 2
)

// Compile-time check ensuring Module satisfies domain.VerticalModule.
var _ domain.VerticalModule = (*Module)(nil)

type bookingRecord struct {
	BookingID string
	EventID   string
	EventName string
	SeatTier  string
	Quantity  int
	City      string
	BookedAt  time.Time
}

// Module serves Abeta Pulse SDUI screens and live ticket booking mutations.
type Module struct {
	mu         sync.RWMutex
	events     []eventSpec
	bookings   []bookingRecord
	bookingSeq int
}

// New initializes the Abeta Pulse vertical with 6 curated events across 4 categories.
func New() *Module {
	return &Module{
		events:     seedEvents(),
		bookingSeq: 3000,
	}
}

// Descriptor returns the vertical metadata for the Super-App catalog.
func (m *Module) Descriptor() domain.AppDescriptor {
	return domain.AppDescriptor{
		ID:          verticalID,
		Name:        "Abeta Pulse",
		Tagline:     "Movies, Live Concerts, Stand-Up Comedy & Curated Dining",
		Category:    "Events & Ticketing",
		TenantID:    verticalID,
		EntryScreen: screenHome,
		ThemePreset: "indigo",
		Screens:     []string{screenHome, screenEventDetail},
	}
}

// Screens lists discoverable SDUI screen routes handled by Abeta Pulse.
func (m *Module) Screens() []domain.ScreenSummary {
	return []domain.ScreenSummary{
		{
			ScreenID:    screenHome,
			Title:       "Abeta Pulse · Events & Movies",
			Vertical:    verticalID,
			Description: "Going-out discovery feed across movies, concerts, comedy, and dining experiences.",
		},
		{
			ScreenID:    screenEventDetail,
			Title:       "Event Details & Seat Tiers",
			Vertical:    verticalID,
			Description: "Event hero, venue schedule, one-tap seat tier reservations, and native OS share.",
		},
	}
}

// CanHandleScreen reports whether screenID belongs to Abeta Pulse.
func (m *Module) CanHandleScreen(screenID string) bool {
	return screenID == screenHome || screenID == screenEventDetail
}

// RenderScreen dispatches rendering for pulse-home and pulse-event-detail.
func (m *Module) RenderScreen(ctx context.Context, req domain.ScreenRequest) (snippets.PageResponse, error) {
	if err := ctx.Err(); err != nil {
		return snippets.PageResponse{}, err
	}
	switch req.ScreenID {
	case screenHome:
		return m.renderHomeScreen(req), nil
	case screenEventDetail:
		return m.renderEventDetailScreen(req), nil
	default:
		return snippets.PageResponse{}, fmt.Errorf("%w: %s", domain.ErrScreenNotFound, req.ScreenID)
	}
}

// BookTickets validates a ticket reservation, persists a booking confirmation (PLS-xxxx),
// and emits celebration + toast follow-up actions.
func (m *Module) BookTickets(ctx context.Context, req domain.BookingRequest) (domain.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActionResult{}, err
	}
	if req.Quantity < 0 {
		return domain.ActionResult{}, fmt.Errorf("%w: quantity must be greater than zero", domain.ErrInvalidInput)
	}
	qty := req.Quantity
	if qty == 0 {
		qty = defaultTicketsQty
	}

	seatTier := strings.TrimSpace(req.SeatTier)
	if seatTier == "" {
		seatTier = strings.TrimSpace(req.Tier)
	}
	if seatTier == "" {
		seatTier = "Gold"
	}
	city := resolveCity(req.City)

	m.mu.Lock()
	m.bookingSeq++
	bookingID := fmt.Sprintf("PLS-%04d", m.bookingSeq)
	eventID := strings.TrimSpace(req.EventID)
	eventName := strings.TrimSpace(req.EventName)

	for i := range m.events {
		if m.events[i].ID == eventID {
			if eventName == "" {
				eventName = m.events[i].Title
			}
			for j := range m.events[i].Tiers {
				if strings.EqualFold(m.events[i].Tiers[j].Name, seatTier) && m.events[i].Tiers[j].SeatsLeft >= qty {
					m.events[i].Tiers[j].SeatsLeft -= qty
				}
			}
			break
		}
	}
	if eventID == "" && len(m.events) > 0 {
		eventID = m.events[0].ID
	}
	if eventName == "" {
		eventName = m.events[0].Title
	}

	m.bookings = append(m.bookings, bookingRecord{
		BookingID: bookingID,
		EventID:   eventID,
		EventName: eventName,
		SeatTier:  seatTier,
		Quantity:  qty,
		City:      city,
		BookedAt:  time.Now().UTC(),
	})
	totalBookings := len(m.bookings)
	m.mu.Unlock()

	celebration := actions.NewTriggerFeedbackAction("pulse_celebration_"+bookingID, actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCelebration,
		Title:      "Tickets Booked!",
		Subtitle:   fmt.Sprintf("%d × %s · %s", qty, seatTier, eventName),
		DurationMs: 2800,
	})
	toast := actions.NewShowToastAction("pulse_toast_"+bookingID, actions.ShowToastPayload{
		Message:    fmt.Sprintf("Booking #%s confirmed — e-pass ready!", bookingID),
		DurationMs: 2600,
		Variant:    "success",
	})

	return domain.ActionResult{
		Status:          "ok",
		Message:         fmt.Sprintf("Booking #%s confirmed — e-pass ready!", bookingID),
		FollowUpActions: []actions.ActionTrigger{celebration, toast},
		Data: map[string]any{
			"booking_id":     bookingID,
			"event_id":       eventID,
			"event_name":     eventName,
			"seat_tier":      seatTier,
			"quantity":       qty,
			"city":           city,
			"total_bookings": totalBookings,
		},
	}, nil
}

// BookingCount returns the number of ticket bookings persisted so far.
func (m *Module) BookingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.bookings)
}

func (m *Module) renderHomeScreen(req domain.ScreenRequest) snippets.PageResponse {
	city := resolveCity(req.City)
	query := strings.TrimSpace(req.Query)
	filter := strings.ToLower(strings.TrimSpace(req.Filter))
	if filter == "" {
		filter = "all"
	}

	m.mu.RLock()
	events := make([]eventSpec, len(m.events))
	copy(events, m.events)
	bookingCount := len(m.bookings)
	m.mu.RUnlock()

	filtered := filterEvents(events, filter, query)

	headerSub := fmt.Sprintf("Movies, concerts, comedy & dining in %s", city)
	trailingTag := atoms.NewTag(city, theming.ColorBrandContainer)
	if bookingCount > 0 {
		headerSub = fmt.Sprintf("%s · %d active e-pass bookings", city, bookingCount)
		trailingTag = atoms.NewTag(fmt.Sprintf("%d BOOKED", bookingCount), theming.ColorSuccessContainer)
	}

	header := snippets.NewHeaderSnippet(
		"pulse_header",
		atoms.NewText("Abeta Pulse · Going Out", theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(headerSub, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithLeadingIcon(atoms.NewIcon("star", 22, theming.ColorBrandPrimary)).
		WithTrailingTag(trailingTag).
		WithLayout(sectionLayout()).
		Build()

	searchBar := snippets.NewSearchBarSnippet("pulse_search_bar", "pulse_search", "Search IMAX movies, live concerts, comedy, dining...").
		WithQuery(query).
		WithLeadingIcon(atoms.NewIcon("search", 20, theming.ColorTextSecondary)).
		WithSubmitLabel("Search").
		WithOnSubmit(actions.NewNavigateAction("pulse_search_submit", actions.NavigatePayload{Route: "pulse-home?q={{query}}"})).
		WithOnClear(actions.NewNavigateAction("pulse_search_clear", actions.NavigatePayload{Route: screenHome})).
		WithLayout(sectionLayout()).
		Build()

	spotlight := events[0]
	if len(filtered) > 0 {
		spotlight = filtered[0]
	}
	spotlightNav := actions.NewNavigateAction("pulse_open_spotlight", actions.NavigatePayload{
		Route: "pulse-event-detail?id=" + spotlight.ID,
	})
	bannerLayout := cardLayout()
	bannerLayout.BgColor = atoms.ColorRef(theming.ColorBrandContainer)
	spotlightBody := fmt.Sprintf("%s · %s · Starting %s", spotlight.Venue, spotlight.Schedule, spotlight.StartingPrice)
	if bookingCount > 0 {
		spotlightBody = fmt.Sprintf("%s · %d bookings in your wallet", spotlightBody, bookingCount)
	}

	banner := snippets.NewBannerSnippet(
		"pulse_spotlight_banner",
		atoms.NewText("Spotlight · "+spotlight.Title, theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithBody(atoms.NewText(spotlightBody, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithIcon(atoms.NewIcon("star", 22, theming.ColorBrandPrimary)).
		WithTag(atoms.NewTag(spotlight.Badge, theming.ColorInfoContainer)).
		WithCTA(atoms.NewButton("btn_pulse_spotlight", "Book Seats", "primary", &spotlightNav)).
		WithOnClick(spotlightNav).
		WithLayout(bannerLayout).
		Build()

	var eventsSection snippets.SnippetEnvelope
	if len(filtered) == 0 {
		resetAct := actions.NewNavigateAction("pulse_reset_filters", actions.NavigatePayload{Route: screenHome})
		eventsSection = snippets.NewEmptyStateSnippet(
			"pulse_empty_state",
			atoms.NewText("No events matched your search", theming.TypographyTitleMedium, theming.ColorTextPrimary),
		).
			WithMessage(atoms.NewText("Try clearing your search or switching event categories.", theming.TypographyBodyMedium, theming.ColorTextSecondary)).
			WithIcon(atoms.NewIcon("search", 32, theming.ColorTextTertiary)).
			WithActionButton(atoms.NewButton("btn_pulse_reset", "Reset Filters", "primary", &resetAct)).
			WithLayout(cardLayout()).
			Build()
	} else {
		cards := make([]snippets.SnippetEnvelope, 0, len(filtered))
		for idx, evt := range filtered {
			cards = append(cards, buildEventCard(idx, evt))
		}
		eventsSection = snippets.NewGridSnippet("pulse_events_grid", 2).
			WithTitle(atoms.NewText("Curated Experiences", theming.TypographyTitleMedium, theming.ColorTextPrimary)).
			WithColumnsByWidth(snippets.ResponsiveColumnsSpec{
				Compact:  atoms.IntPtr(1),
				Medium:   atoms.IntPtr(2),
				Expanded: atoms.IntPtr(3),
			}).
			WithSpacingDp(12).
			AddItems(cards...).
			WithLayout(sectionLayout()).
			Build()
	}

	earlyBirdNudge := buildEarlyBirdNudge(city, spotlight)

	toolbarTitle := atoms.NewText("Abeta Pulse", theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText(city+" · Live Tickets", theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenHome, "Abeta Pulse · Events & Movies").
		WithToolbar(snippets.ToolbarSpec{
			Title:    &toolbarTitle,
			Subtitle: &toolbarSub,
			BgColor:  atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(1080),
		}).
		WithPullToRefresh(actions.NewRefreshPageAction("pulse_refresh_home", actions.RefreshPagePayload{
			PageID:     screenHome,
			ShowLoader: true,
		})).
		AddSnippets(header, searchBar, buildCategoryChips(filter), earlyBirdNudge, banner, eventsSection).
		WithTracking("page_loaded", map[string]any{
			"vertical":      verticalID,
			"page_id":       screenHome,
			"city":          city,
			"filter":        filter,
			"result_count":  strconv.Itoa(len(filtered)),
			"booking_count": strconv.Itoa(bookingCount),
		}).
		Build()
}

func (m *Module) renderEventDetailScreen(req domain.ScreenRequest) snippets.PageResponse {
	city := resolveCity(req.City)
	eventID := strings.TrimSpace(req.Extra["id"])

	m.mu.RLock()
	evt := m.events[0]
	if eventID != "" {
		for _, candidate := range m.events {
			if candidate.ID == eventID {
				evt = candidate
				break
			}
		}
	}
	tiers := make([]seatTierSpec, len(evt.Tiers))
	copy(tiers, evt.Tiers)
	m.mu.RUnlock()

	shareAct := actions.NewShareAction("pulse_share_"+evt.ID, actions.SharePayload{
		Title:        evt.Title + " · Abeta Pulse",
		Text:         fmt.Sprintf("Join me at %s (%s, %s) — tickets starting %s on Abeta Pulse!", evt.Title, evt.Venue, evt.Schedule, evt.StartingPrice),
		URL:          "https://pulse.abeta.dev/events/" + evt.ID,
		ChooserTitle: "Share Event",
	})

	header := snippets.NewHeaderSnippet(
		"pulse_detail_header",
		atoms.NewText(evt.Title, theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(fmt.Sprintf("%s · %s · %s", evt.CategoryLabel, evt.Venue, evt.Schedule), theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithTrailingTag(atoms.NewTag(evt.Badge, theming.ColorBrandContainer)).
		WithLayout(sectionLayout()).
		Build()

	heroCard := snippets.NewCardSnippet(
		"pulse_event_hero",
		atoms.NewText(evt.Title, theming.TypographyTitleLarge, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(fmt.Sprintf("%s · %s", evt.Schedule, evt.Rating), theming.TypographyLabelLarge, theming.ColorBrandPrimary)).
		WithDescription(atoms.NewText(evt.Description, theming.TypographyBodyMedium, theming.ColorTextSecondary)).
		WithBadge(atoms.NewTag("E-PASS INSTANT", theming.ColorSuccessContainer)).
		WithMedia(atoms.ImageAtom{
			URL:                fmt.Sprintf("https://cdn.abeta.dev/pulse/%s.png", evt.ID),
			ContentDescription: evt.Title,
			HeightDp:           atoms.IntPtr(180),
			ScaleMode:          "crop",
			CornerRadius:       atoms.UniformRadius(theming.RadiusMD.Dp()),
		}).
		WithSecondaryButton(atoms.NewButton("btn_pulse_share_hero", "Share Event", "outline", &shareAct)).
		WithLayout(cardLayout()).
		Build()

	venueRow := snippets.NewActionRowSnippet(
		"pulse_venue_info",
		atoms.NewText(evt.Venue+", "+city, theming.TypographyTitleSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(evt.Schedule+" · Gates open 30 mins prior · Mobile QR entry", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithLeadingIcon(atoms.NewIcon("info", 22, theming.ColorBrandPrimary)).
		WithTrailingTag(atoms.NewTag("VERIFIED VENUE", theming.ColorInfoContainer)).
		WithLayout(cardLayout()).
		Build()

	pageSnippets := []snippets.SnippetEnvelope{header, heroCard, venueRow, buildEventEPassPreviewBlock(evt, city)}
	for idx, tier := range tiers {
		bookAct := buildBookAPIAction(evt, tier.Name, defaultTicketsQty, city)
		tierID := strings.ToLower(strings.ReplaceAll(tier.Name, " ", "_"))
		pageSnippets = append(pageSnippets, snippets.NewCardSnippet(
			"pulse_tier_"+tierID,
			atoms.NewText(fmt.Sprintf("%s · %s", tier.Name, tier.PriceDisplay), theming.TypographyTitleMedium, theming.ColorTextPrimary),
		).
			WithSubtitle(atoms.NewText(tier.Perks, theming.TypographyBodySmall, theming.ColorTextSecondary)).
			WithBadge(atoms.NewTag(fmt.Sprintf("%d seats left", tier.SeatsLeft), tier.BadgeColor)).
			WithPrimaryButton(atoms.NewButton("btn_book_"+tierID, fmt.Sprintf("Book 2 %s", tier.Name), "primary", &bookAct)).
			WithLayout(cardLayout()).
			WithMotion(atoms.MotionSpec{
				Enter:         atoms.MotionTransitionSlideUp,
				DurationToken: atoms.MotionDurationFast,
				DelayMs:       idx * 50,
			}).
			Build())
	}

	defaultBookAct := buildBookAPIAction(evt, "Gold", defaultTicketsQty, city)
	primaryCTA := atoms.NewButton("btn_pulse_bottom_book", "Book Gold (2)", "primary", &defaultBookAct)
	secondaryCTA := atoms.NewButton("btn_pulse_bottom_share", "Share", "outline", &shareAct)
	helperTxt := atoms.NewText(fmt.Sprintf("Instant QR e-pass · %s", evt.Schedule), theming.TypographyBodySmall, theming.ColorTextSecondary)

	backAct := actions.NewNavigateAction("pulse_back_home", actions.NavigatePayload{Route: screenHome})
	backIcon := atoms.NewIcon("arrow_back", 22, theming.ColorTextPrimary)
	backIcon.OnClick = &backAct
	toolbarTitle := atoms.NewText(evt.Title, theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText(evt.Venue, theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenEventDetail, evt.Title+" · Abeta Pulse").
		WithToolbar(snippets.ToolbarSpec{
			Title:          &toolbarTitle,
			Subtitle:       &toolbarSub,
			NavigationIcon: &backIcon,
			BgColor:        atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(1040),
		}).
		AddSnippets(pageSnippets...).
		WithBottomBar(snippets.BottomBarSpec{
			PrimaryButton:   &primaryCTA,
			SecondaryButton: &secondaryCTA,
			HelperText:      &helperTxt,
		}).
		WithTracking("page_loaded", map[string]any{
			"vertical": verticalID,
			"page_id":  screenEventDetail,
			"event_id": evt.ID,
			"category": evt.Category,
			"city":     city,
		}).
		Build()
}
