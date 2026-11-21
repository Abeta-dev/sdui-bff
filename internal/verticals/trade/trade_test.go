package trade_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/verticals/trade"
)

func TestDescriptorAndScreens(t *testing.T) {
	t.Parallel()
	m := trade.New()

	desc := m.Descriptor()
	if desc.ID != "abeta-trade" || desc.TenantID != "abeta-trade" || desc.EntryScreen != "trade-home" || desc.ThemePreset != "emerald" {
		t.Fatalf("unexpected descriptor: %+v", desc)
	}
	if len(desc.Screens) != 2 || desc.Screens[0] != "trade-home" || desc.Screens[1] != "trade-supplier" {
		t.Fatalf("unexpected descriptor screens: %v", desc.Screens)
	}
	if !m.CanHandleScreen("trade-home") || !m.CanHandleScreen("trade-supplier") || m.CanHandleScreen("unknown") {
		t.Fatalf("unexpected CanHandleScreen results")
	}
	if len(m.Screens()) != 2 {
		t.Fatalf("expected 2 screen summaries, got %d", len(m.Screens()))
	}

	_, err := m.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "bad-screen"})
	if !errors.Is(err, domain.ErrScreenNotFound) {
		t.Fatalf("expected ErrScreenNotFound, got %v", err)
	}
}

func TestTradeHomeFilteringAndSearch(t *testing.T) {
	t.Parallel()
	m := trade.New()
	ctx := context.Background()

	page, err := m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "trade-home"})
	if err != nil {
		t.Fatalf("RenderScreen(trade-home) error: %v", err)
	}
	assertStringOnlyTrackingProps(t, page)

	var foundHeader, foundSearch, foundChips, foundBanner bool
	supplierCards := 0
	for _, env := range page.Snippets {
		switch env.SnippetType {
		case snippets.SnippetTypeHeader:
			foundHeader = true
		case snippets.SnippetTypeSearchBar:
			foundSearch = true
			sb, decErr := snippets.DecodeSnippetData[snippets.SearchBarSnippetData](env)
			if decErr != nil {
				t.Fatalf("decode search bar: %v", decErr)
			}
			if sb.SearchID != "trade_search" || sb.OnSubmit == nil {
				t.Fatalf("unexpected trade search bar: %+v", sb)
			}
			nav, decErr := actions.DecodePayload[actions.NavigatePayload](*sb.OnSubmit)
			if decErr != nil || nav.Route != "trade-home?q={{query}}" {
				t.Fatalf("unexpected search route: %+v (err=%v)", nav, decErr)
			}
		case snippets.SnippetTypeChipGroup:
			foundChips = true
			cg, decErr := snippets.DecodeSnippetData[snippets.ChipGroupSnippetData](env)
			if decErr != nil || len(cg.Chips) != 5 {
				t.Fatalf("expected 5 filter chips, got %+v (err=%v)", cg, decErr)
			}
		case snippets.SnippetTypeBanner:
			foundBanner = true
		case snippets.SnippetTypeCard:
			supplierCards++
			card, decErr := snippets.DecodeSnippetData[snippets.CardSnippetData](env)
			if decErr != nil || card.OnClick == nil {
				t.Fatalf("expected supplier card click navigation")
			}
			nav, decErr := actions.DecodePayload[actions.NavigatePayload](*card.OnClick)
			if decErr != nil || !strings.HasPrefix(nav.Route, "trade-supplier?id=") {
				t.Fatalf("unexpected supplier navigation route: %+v (err=%v)", nav, decErr)
			}
		}
	}
	if !foundHeader || !foundSearch || !foundChips || !foundBanner {
		t.Fatalf("missing expected trade-home snippets: header=%v search=%v chips=%v banner=%v", foundHeader, foundSearch, foundChips, foundBanner)
	}
	if supplierCards != 6 {
		t.Fatalf("expected 6 B2B suppliers, got %d", supplierCards)
	}

	// GST-verified filter excludes non-GST suppliers (4 verified out of 6).
	verifiedPage, err := m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "trade-home", Filter: "verified"})
	if err != nil {
		t.Fatalf("RenderScreen(verified) error: %v", err)
	}
	if got := countCards(verifiedPage); got != 4 {
		t.Fatalf("expected 4 GST-verified suppliers, got %d", got)
	}

	// Category filter.
	pkgPage, err := m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "trade-home", Filter: "packaging"})
	if err != nil {
		t.Fatalf("RenderScreen(packaging) error: %v", err)
	}
	if got := countCards(pkgPage); got != 2 {
		t.Fatalf("expected 2 packaging suppliers, got %d", got)
	}

	// Search query filter.
	searchPage, err := m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "trade-home", Query: "Cotton"})
	if err != nil {
		t.Fatalf("RenderScreen(search) error: %v", err)
	}
	if got := countCards(searchPage); got != 1 {
		t.Fatalf("expected 1 supplier matching Cotton, got %d", got)
	}
}

func TestSupplierMOQPageAndRFQValidationAndSubmission(t *testing.T) {
	t.Parallel()
	m := trade.New()
	ctx := context.Background()

	// Inspect supplier detail page before RFQ submission.
	supPage, err := m.RenderScreen(ctx, domain.ScreenRequest{
		ScreenID: "trade-supplier",
		Extra:    map[string]string{"id": "trd_ecopack"},
	})
	if err != nil {
		t.Fatalf("RenderScreen(trade-supplier) error: %v", err)
	}
	assertStringOnlyTrackingProps(t, supPage)

	if !strings.Contains(supPage.Title, "EcoCrate Corrugated Packaging") {
		t.Fatalf("unexpected supplier page title: %q", supPage.Title)
	}

	var foundMetric, foundForm bool
	tierCards := 0
	for _, env := range supPage.Snippets {
		switch env.SnippetType {
		case snippets.SnippetTypeMetricCard:
			foundMetric = true
		case snippets.SnippetTypeCard:
			tierCards++
		case snippets.SnippetTypeForm:
			foundForm = true
			form, decErr := snippets.DecodeSnippetData[snippets.FormSnippetData](env)
			if decErr != nil {
				t.Fatalf("decode form: %v", decErr)
			}
			if len(form.Fields) != 4 {
				t.Fatalf("expected 4 RFQ fields, got %d", len(form.Fields))
			}
			if form.SubmitButton.OnClick == nil {
				t.Fatalf("expected submit button action")
			}
			callPayload, decErr := actions.DecodePayload[actions.CallAPIPayload](*form.SubmitButton.OnClick)
			if decErr != nil || callPayload.Endpoint != "/api/v1/trade/rfq" || callPayload.Method != "POST" {
				t.Fatalf("unexpected RFQ submit payload: %+v (err=%v)", callPayload, decErr)
			}
			if len(form.SubmitButton.OnClick.Validations) < 4 {
				t.Fatalf("expected declarative form validation rules, got %d", len(form.SubmitButton.OnClick.Validations))
			}
		}
	}
	if !foundMetric || !foundForm || tierCards != 3 {
		t.Fatalf("unexpected supplier layout: metric=%v form=%v tierCards=%d", foundMetric, foundForm, tierCards)
	}

	// Validation error cases.
	invalidCases := []struct {
		name string
		req  domain.RFQRequest
	}{
		{
			name: "missing company name",
			req:  domain.RFQRequest{ContactEmail: "buyer@abeta.dev", Quantity: 250},
		},
		{
			name: "invalid email without at sign",
			req:  domain.RFQRequest{CompanyName: "Abeta Corp", ContactEmail: "invalid-email", Quantity: 250},
		},
		{
			name: "zero quantity",
			req:  domain.RFQRequest{CompanyName: "Abeta Corp", ContactEmail: "buyer@abeta.dev", Quantity: 0},
		},
		{
			name: "negative quantity",
			req:  domain.RFQRequest{CompanyName: "Abeta Corp", ContactEmail: "buyer@abeta.dev", Quantity: -10},
		},
	}
	for _, tc := range invalidCases {
		if _, valErr := m.SubmitRFQ(ctx, tc.req); !errors.Is(valErr, domain.ErrInvalidInput) {
			t.Fatalf("case %q: expected ErrInvalidInput, got %v", tc.name, valErr)
		}
	}

	// Valid submission with default supplier/product fallback when omitted.
	res, err := m.SubmitRFQ(ctx, domain.RFQRequest{
		CompanyName:  "Abeta Supply Co",
		ContactEmail: "procurement@abeta.dev",
		Quantity:     1000,
	})
	if err != nil {
		t.Fatalf("SubmitRFQ valid error: %v", err)
	}
	assertStringOnlyTrackingProps(t, res)

	quoteID, _ := res.Data["quote_id"].(string)
	if !strings.HasPrefix(quoteID, "RFQ-") {
		t.Fatalf("expected RFQ-xxxx quote_id, got %q", quoteID)
	}
	if len(res.FollowUpActions) != 2 {
		t.Fatalf("expected 2 follow-up actions, got %d", len(res.FollowUpActions))
	}
	fb, err := actions.DecodePayload[actions.TriggerFeedbackPayload](res.FollowUpActions[0])
	if err != nil || fb.Effect != actions.FeedbackCelebration || fb.Title != "RFQ Sent to Verified Supplier!" {
		t.Fatalf("unexpected celebration follow-up: %+v (err=%v)", fb, err)
	}
	toast, err := actions.DecodePayload[actions.ShowToastPayload](res.FollowUpActions[1])
	if err != nil || !strings.Contains(toast.Message, "response within 2 hours") {
		t.Fatalf("unexpected toast follow-up: %+v (err=%v)", toast, err)
	}

	if len(m.RFQs()) != 1 || m.RFQs()[0].QuoteID != quoteID {
		t.Fatalf("expected persisted RFQ %q, got %+v", quoteID, m.RFQs())
	}

	// Re-rendering trade-supplier now surfaces recently submitted RFQ action rows.
	afterPage, err := m.RenderScreen(ctx, domain.ScreenRequest{
		ScreenID: "trade-supplier",
		Extra:    map[string]string{"id": "trd_ecopack"},
	})
	if err != nil {
		t.Fatalf("RenderScreen after RFQ error: %v", err)
	}
	foundRFQRow := false
	for _, env := range afterPage.Snippets {
		if env.SnippetType == snippets.SnippetTypeActionRow && strings.Contains(env.SnippetID, quoteID) {
			foundRFQRow = true
		}
	}
	if !foundRFQRow {
		t.Fatalf("expected submitted RFQ row %q on trade-supplier screen", quoteID)
	}
}

func TestTradeConcurrentSafety(t *testing.T) {
	t.Parallel()
	m := trade.New()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			_, err := m.SubmitRFQ(ctx, domain.RFQRequest{
				SupplierID:   "trd_voltcore",
				ProductName:  "Smart IoT Sensor Modules",
				Quantity:     100 + n*50,
				CompanyName:  "Abeta Industrial",
				ContactEmail: "ops@abeta.dev",
			})
			if err != nil {
				t.Errorf("concurrent SubmitRFQ error: %v", err)
			}
		}(i)
		go func() {
			defer wg.Done()
			_, _ = m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "trade-home", Filter: "verified"})
			_, _ = m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "trade-supplier", Extra: map[string]string{"id": "trd_voltcore"}})
			_ = m.RFQs()
		}()
	}
	wg.Wait()

	if got := len(m.RFQs()); got != 10 {
		t.Fatalf("expected 10 persisted RFQs, got %d", got)
	}
}

func countCards(page snippets.PageResponse) int {
	n := 0
	for _, env := range page.Snippets {
		if env.SnippetType == snippets.SnippetTypeCard {
			n++
		}
	}
	return n
}

func assertStringOnlyTrackingProps(t *testing.T, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	var walk func(node any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			if tr, ok := typed["tracking"].(map[string]any); ok {
				if props, ok := tr["properties"].(map[string]any); ok {
					for k, val := range props {
						if _, isStr := val.(string); !isStr {
							t.Errorf("tracking.properties[%q] must be string, got %T (%v)", k, val, val)
						}
					}
				}
			}
			if typed["type"] == string(actions.ActionTrackEvent) {
				if payload, ok := typed["payload"].(map[string]any); ok {
					if props, ok := payload["properties"].(map[string]any); ok {
						for k, val := range props {
							if _, isStr := val.(string); !isStr {
								t.Errorf("track_event.properties[%q] must be string, got %T (%v)", k, val, val)
							}
						}
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(root)
}
