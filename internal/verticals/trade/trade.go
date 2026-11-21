// Package trade implements the Abeta Trade B2B wholesale & RFQ sourcing vertical.
package trade

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
	verticalID     = "abeta-trade"
	screenHome     = "trade-home"
	screenSupplier = "trade-supplier"
	defaultCity    = "Bengaluru"
	rfqEndpoint    = "/api/v1/trade/rfq"
)

// Compile-time check ensuring Module satisfies domain.VerticalModule.
var _ domain.VerticalModule = (*Module)(nil)

// Module serves Abeta Trade B2B sourcing SDUI screens and validated RFQ submissions.
type Module struct {
	mu        sync.RWMutex
	suppliers []supplierProfile
	rfqSeq    int
	rfqs      []domain.RFQRecord
}

// New initializes the Abeta Trade vertical with verified wholesale manufacturers.
func New() *Module {
	return &Module{
		suppliers: seedSuppliers(),
		rfqSeq:    1000,
	}
}

// Descriptor returns the vertical metadata for the Super-App catalog.
func (m *Module) Descriptor() domain.AppDescriptor {
	return domain.AppDescriptor{
		ID:          verticalID,
		Name:        "Abeta Trade",
		Tagline:     "Verified B2B Wholesale Manufacturers, MOQ Pricing & Instant RFQs",
		Category:    "B2B Wholesale",
		TenantID:    verticalID,
		EntryScreen: screenHome,
		ThemePreset: "emerald",
		Screens:     []string{screenHome, screenSupplier},
	}
}

// Screens lists discoverable SDUI routes handled by Abeta Trade.
func (m *Module) Screens() []domain.ScreenSummary {
	return []domain.ScreenSummary{
		{
			ScreenID:    screenHome,
			Title:       "Abeta Trade · B2B Sourcing",
			Vertical:    verticalID,
			Description: "Verified wholesale directory with GST filters, Trade Assurance banner, and MOQ bulk tier previews.",
		},
		{
			ScreenID:    screenSupplier,
			Title:       "Abeta Trade · Supplier & RFQ",
			Vertical:    verticalID,
			Description: "Supplier TrustScore metrics, bulk MOQ pricing tiers, validated RFQ quotation form, and recent RFQs.",
		},
	}
}

// CanHandleScreen reports whether screenID belongs to Abeta Trade.
func (m *Module) CanHandleScreen(screenID string) bool {
	return screenID == screenHome || screenID == screenSupplier
}

// RenderScreen builds the SDUI PageResponse for trade-home or trade-supplier.
func (m *Module) RenderScreen(ctx context.Context, req domain.ScreenRequest) (snippets.PageResponse, error) {
	if err := ctx.Err(); err != nil {
		return snippets.PageResponse{}, err
	}
	switch req.ScreenID {
	case screenHome:
		return m.renderHome(req), nil
	case screenSupplier:
		return m.renderSupplier(req), nil
	default:
		return snippets.PageResponse{}, fmt.Errorf("%w: %s", domain.ErrScreenNotFound, req.ScreenID)
	}
}

// SubmitRFQ validates and persists a B2B Request For Quotation, emitting celebration + toast follow-ups.
func (m *Module) SubmitRFQ(ctx context.Context, req domain.RFQRequest) (domain.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActionResult{}, err
	}

	company := strings.TrimSpace(req.CompanyName)
	if company == "" {
		company = strings.TrimSpace(req.Company)
	}
	if company == "" {
		return domain.ActionResult{}, fmt.Errorf("%w: company_name is required", domain.ErrInvalidInput)
	}
	email := strings.TrimSpace(req.ContactEmail)
	if email == "" {
		email = strings.TrimSpace(req.Email)
	}
	if email == "" || !strings.Contains(email, "@") {
		return domain.ActionResult{}, fmt.Errorf("%w: contact_email must be a valid email address", domain.ErrInvalidInput)
	}
	if req.Quantity <= 0 {
		return domain.ActionResult{}, fmt.Errorf("%w: quantity must be greater than zero", domain.ErrInvalidInput)
	}

	m.mu.Lock()
	supplier := m.findSupplierOrDefaultLocked(strings.TrimSpace(req.SupplierID))
	productName := strings.TrimSpace(req.ProductName)
	if productName == "" {
		productName = supplier.PrimaryProduct
	}
	m.rfqSeq++
	quoteID := fmt.Sprintf("RFQ-%04d", m.rfqSeq)
	rec := domain.RFQRecord{
		QuoteID:      quoteID,
		SupplierID:   supplier.ID,
		ProductName:  productName,
		Quantity:     req.Quantity,
		CompanyName:  company,
		ContactEmail: email,
		Status:       "submitted",
		CreatedAt:    time.Now().UTC(),
	}
	m.rfqs = append(m.rfqs, rec)
	rfqCount := len(m.rfqs)
	m.mu.Unlock()

	celebration := actions.NewTriggerFeedbackAction("trade_rfq_celebration_"+quoteID, actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCelebration,
		Title:      "RFQ Sent to Verified Supplier!",
		Subtitle:   fmt.Sprintf("%s received Quote #%s (%d units)", supplier.Name, quoteID, req.Quantity),
		DurationMs: 2800,
	})
	toast := actions.NewShowToastAction("trade_rfq_toast_"+quoteID, actions.ShowToastPayload{
		Message:    fmt.Sprintf("Quote #%s submitted — response within 2 hours", quoteID),
		DurationMs: 2800,
		Variant:    "success",
	})

	return domain.ActionResult{
		Status:          "ok",
		Message:         fmt.Sprintf("Quote #%s submitted — response within 2 hours", quoteID),
		FollowUpActions: []actions.ActionTrigger{celebration, toast},
		Data: map[string]any{
			"quote_id":     quoteID,
			"supplier_id":  supplier.ID,
			"product_name": productName,
			"quantity":     req.Quantity,
			"company_name": company,
			"status":       rec.Status,
			"rfq_count":    rfqCount,
		},
	}, nil
}

// RFQs returns a thread-safe snapshot of all submitted RFQ records.
func (m *Module) RFQs() []domain.RFQRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.RFQRecord, len(m.rfqs))
	copy(out, m.rfqs)
	return out
}

func (m *Module) findSupplierOrDefaultLocked(id string) supplierProfile {
	for _, s := range m.suppliers {
		if s.ID == id {
			return s
		}
	}
	return m.suppliers[0]
}

func (m *Module) renderHome(req domain.ScreenRequest) snippets.PageResponse {
	query := strings.TrimSpace(req.Query)
	filter := strings.ToLower(strings.TrimSpace(req.Filter))
	if filter == "" {
		filter = "all"
	}

	m.mu.RLock()
	allSuppliers := make([]supplierProfile, len(m.suppliers))
	copy(allSuppliers, m.suppliers)
	rfqCount := len(m.rfqs)
	m.mu.RUnlock()

	filtered := filterSuppliers(allSuppliers, filter, query)

	header := snippets.NewHeaderSnippet(
		"trade_home_header",
		atoms.NewText("Abeta Trade · B2B Sourcing", theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText("Factory-direct wholesale & GST-verified manufacturers", theming.TypographyBodyMedium, theming.ColorTextSecondary)).
		WithTrailingTag(atoms.NewTag(fmt.Sprintf("RFQs: %d", rfqCount), theming.ColorBrandContainer)).
		WithLayout(sectionPadding()).
		Build()

	searchSubmit := actions.NewNavigateAction("trade_search_submit", actions.NavigatePayload{
		Route: "trade-home?q={{query}}",
	})
	searchClear := actions.NewNavigateAction("trade_search_clear", actions.NavigatePayload{
		Route: screenHome,
	})
	searchBar := snippets.NewSearchBarSnippet("trade_search_bar", "trade_search", "Search factories, MOQ products, cities...").
		WithQuery(query).
		WithSubmitLabel("Find").
		WithOnSubmit(searchSubmit).
		WithOnClear(searchClear).
		WithLayout(cardLayout()).
		Build()

	chips := buildTradeChips(filter, query)

	assuranceToast := actions.NewShowToastAction("trade_assurance_info", actions.ShowToastPayload{
		Message:    "Abeta Trade Assurance: 100% escrow protection & GST verification active",
		DurationMs: 2400,
		Variant:    "info",
	})
	banner := snippets.NewBannerSnippet(
		"trade_assurance_banner",
		atoms.NewText("Abeta Trade Assurance", theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithBody(atoms.NewText("Escrow payment protection, GST invoice checks & guaranteed 2-hour RFQ turnaround.", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithIcon(atoms.NewIcon("check", 22, theming.ColorSuccess)).
		WithTag(atoms.NewTag("ESCROW PROTECTED", theming.ColorSuccessContainer)).
		WithCTA(atoms.NewButton("btn_trade_assurance", "How It Works", "outline", &assuranceToast)).
		WithLayout(cardLayout()).
		Build()

	pageSnippets := make([]snippets.SnippetEnvelope, 0, 4+len(filtered))
	pageSnippets = append(pageSnippets, header, searchBar, chips, banner)

	if len(filtered) == 0 {
		resetAct := actions.NewNavigateAction("trade_reset_filters", actions.NavigatePayload{Route: screenHome})
		pageSnippets = append(pageSnippets,
			snippets.NewEmptyStateSnippet(
				"trade_empty_suppliers",
				atoms.NewText("No matching manufacturers", theming.TypographyTitleMedium, theming.ColorTextPrimary),
			).
				WithMessage(atoms.NewText("Clear your search or switch category filters to browse all suppliers.", theming.TypographyBodyMedium, theming.ColorTextSecondary)).
				WithIcon(atoms.NewIcon("search", 32, theming.ColorTextTertiary)).
				WithActionButton(atoms.NewButton("btn_trade_reset", "Reset Filters", "primary", &resetAct)).
				WithLayout(cardLayout()).
				Build(),
		)
	} else {
		for _, s := range filtered {
			pageSnippets = append(pageSnippets, buildSupplierCard(s))
		}
	}

	toolbarTitle := atoms.NewText("Abeta Trade", theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText("Verified B2B Wholesale Directory", theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenHome, "Abeta Trade · B2B Sourcing").
		WithToolbar(snippets.ToolbarSpec{
			Title:    &toolbarTitle,
			Subtitle: &toolbarSub,
			BgColor:  atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(snippets.AdaptivePageSpec{
			LayoutMode: snippets.LayoutModeCentered,
			MaxWidthDp: atoms.IntPtr(1080),
		}).
		WithPullToRefresh(actions.NewRefreshPageAction("trade_pull_refresh", actions.RefreshPagePayload{
			PageID:     screenHome,
			ShowLoader: true,
		})).
		AddPageAction(actions.NewTrackEventAction("trade_home_viewed", actions.TrackEventPayload{
			EventName: "screen_viewed",
			Properties: map[string]any{
				"vertical":     verticalID,
				"screen_id":    screenHome,
				"filter":       filter,
				"query":        query,
				"result_count": strconv.Itoa(len(filtered)),
				"rfq_count":    strconv.Itoa(rfqCount),
			},
		})).
		AddSnippets(pageSnippets...).
		WithTracking("page_loaded", map[string]any{
			"vertical":     verticalID,
			"page_id":      screenHome,
			"filter":       filter,
			"result_count": strconv.Itoa(len(filtered)),
			"rfq_count":    strconv.Itoa(rfqCount),
		}).
		Build()
}

func (m *Module) renderSupplier(req domain.ScreenRequest) snippets.PageResponse {
	city := strings.TrimSpace(req.City)
	if city == "" {
		city = defaultCity
	}

	m.mu.RLock()
	supplier := m.findSupplierOrDefaultLocked(strings.TrimSpace(req.Extra["id"]))
	recentRFQs := make([]domain.RFQRecord, len(m.rfqs))
	copy(recentRFQs, m.rfqs)
	m.mu.RUnlock()

	backNav := actions.NewNavigateAction("trade_supplier_back", actions.NavigatePayload{Route: screenHome})
	backIcon := atoms.NewIcon("arrow_back", 22, theming.ColorTextPrimary)
	backIcon.ContentDescription = "Back to Abeta Trade"
	backIcon.OnClick = &backNav

	gstBadge := "WHOLESALE"
	gstColor := theming.ColorInfoContainer
	if supplier.GSTVerified {
		gstBadge = "GST VERIFIED"
		gstColor = theming.ColorSuccessContainer
	}

	header := snippets.NewHeaderSnippet(
		"trade_supplier_header",
		atoms.NewText(supplier.Name, theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText(
			fmt.Sprintf("%s · Response %s · %s", supplier.City, supplier.ResponseTime, supplier.Summary),
			theming.TypographyBodyMedium,
			theming.ColorTextSecondary,
		)).
		WithTrailingTag(atoms.NewTag(gstBadge, gstColor)).
		WithLayout(sectionPadding()).
		Build()

	trustMetric := snippets.NewMetricCardSnippet(
		"trade_supplier_trust_metric",
		atoms.NewText("Supplier TrustScore & GST Compliance", theming.TypographyLabelLarge, theming.ColorTextSecondary),
		atoms.NewText(fmt.Sprintf("%d / 100", supplier.TrustScore), theming.TypographyHeadlineMedium, theming.ColorTextPrimary),
	).
		WithDelta(atoms.NewTag(gstBadge+" · "+supplier.GSTNumber, gstColor)).
		WithIcon(atoms.NewIcon("star", 22, theming.ColorSuccess)).
		WithProgress(atoms.ProgressAtom{
			Kind:     "linear",
			Progress: atoms.Float64Ptr(float64(supplier.TrustScore) / 100.0),
			Label:    "Verified fulfillment reliability",
			Color:    atoms.ColorRef(theming.ColorSuccess),
		}).
		WithLayout(cardLayout()).
		Build()

	pageSnippets := make([]snippets.SnippetEnvelope, 0, 4+len(supplier.Tiers)+len(recentRFQs))
	pageSnippets = append(pageSnippets, header, trustMetric)
	pageSnippets = append(pageSnippets, buildTierCards(supplier)...)
	pageSnippets = append(pageSnippets, buildRFQForm(supplier, city))

	if len(recentRFQs) > 0 {
		rfqHeader := snippets.NewHeaderSnippet(
			"trade_recent_rfqs_header",
			atoms.NewText(fmt.Sprintf("Recently Submitted RFQs (%d)", len(recentRFQs)), theming.TypographyTitleMedium, theming.ColorTextPrimary),
		).
			WithLayout(sectionPadding()).
			Build()
		pageSnippets = append(pageSnippets, rfqHeader)
		for i := len(recentRFQs) - 1; i >= 0; i-- {
			rec := recentRFQs[i]
			statusToast := actions.NewShowToastAction("trade_rfq_status_"+rec.QuoteID, actions.ShowToastPayload{
				Message:    fmt.Sprintf("%s · %d units for %s (%s)", rec.QuoteID, rec.Quantity, rec.CompanyName, strings.ToUpper(rec.Status)),
				DurationMs: 2200,
				Variant:    "info",
			})
			pageSnippets = append(pageSnippets,
				snippets.NewActionRowSnippet(
					"trade_rfq_row_"+rec.QuoteID,
					atoms.NewText(fmt.Sprintf("%s · %s", rec.QuoteID, rec.ProductName), theming.TypographyTitleSmall, theming.ColorTextPrimary),
				).
					WithSubtitle(atoms.NewText(fmt.Sprintf("%s · %d units · %s", rec.CompanyName, rec.Quantity, rec.ContactEmail), theming.TypographyBodySmall, theming.ColorTextSecondary)).
					WithTrailingTag(atoms.NewTag(strings.ToUpper(rec.Status), theming.ColorSuccessContainer)).
					WithOnClick(statusToast).
					WithLayout(cardLayout()).
					Build(),
			)
		}
	}

	toolbarTitle := atoms.NewText(supplier.Name, theming.TypographyTitleMedium, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText(supplier.City+" · MOQ & RFQ", theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenSupplier, supplier.Name+" · Abeta Trade").
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
			SecondarySnippetIDs: []string{"trade_rfq_form_snippet"},
		}).
		AddSnippets(pageSnippets...).
		WithTracking("supplier_viewed", map[string]any{
			"vertical":     verticalID,
			"supplier_id":  supplier.ID,
			"category":     supplier.Category,
			"gst_verified": strconv.FormatBool(supplier.GSTVerified),
			"rfq_count":    strconv.Itoa(len(recentRFQs)),
		}).
		Build()
}
