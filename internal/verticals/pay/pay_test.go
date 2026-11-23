package pay_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/verticals/pay"
)

func TestPayDescriptorAndRouting(t *testing.T) {
	t.Parallel()

	mod := pay.New()
	desc := mod.Descriptor()
	if desc.ID != "abeta-pay" || desc.Name != "Abeta Pay" || desc.TenantID != "abeta-pay" || desc.EntryScreen != "pay-home" || desc.ThemePreset != "indigo" {
		t.Fatalf("unexpected descriptor: %+v", desc)
	}
	wantScreens := []string{"pay-home", "pay-transfer", "pay-cards", "pay-receipt"}
	if len(desc.Screens) != len(wantScreens) {
		t.Fatalf("descriptor screens = %v, want %v", desc.Screens, wantScreens)
	}
	for i, s := range wantScreens {
		if desc.Screens[i] != s || !mod.CanHandleScreen(s) {
			t.Errorf("expected screen %q handled at index %d", s, i)
		}
	}
	if mod.CanHandleScreen("fresh-home") {
		t.Error("expected fresh-home not handled by pay")
	}
	if len(mod.Screens()) != 4 {
		t.Fatalf("expected 4 screen summaries, got %d", len(mod.Screens()))
	}

	if _, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "unknown-pay"}); !errors.Is(err, domain.ErrScreenNotFound) {
		t.Fatalf("expected ErrScreenNotFound, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mod.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "pay-home"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestPayHomeScreenAndSearch(t *testing.T) {
	t.Parallel()

	mod := pay.New()
	page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pay-home"})
	if err != nil {
		t.Fatalf("RenderScreen(pay-home) error: %v", err)
	}
	assertAllTrackingPropsAreStrings(t, page)

	if page.PageID != "pay-home" {
		t.Fatalf("page_id = %q, want pay-home", page.PageID)
	}

	var foundMarquee, foundTracker, foundStatsBlock, foundQrBlock bool
	contactRows := 0
	for _, env := range page.Snippets {
		switch env.SnippetType {
		case snippets.SnippetTypeMarqueeStrip:
			foundMarquee = true
			mq, err := snippets.DecodeSnippetData[snippets.MarqueeStripSnippetData](env)
			if err != nil || len(mq.Items) != 3 {
				t.Fatalf("unexpected pay marquee_strip: %+v (err=%v)", mq, err)
			}
		case snippets.SnippetTypeOrderTracker:
			foundTracker = true
			tr, err := snippets.DecodeSnippetData[snippets.OrderTrackerSnippetData](env)
			if err != nil {
				t.Fatalf("decode order_tracker: %v", err)
			}
			if tr.OrderCode != "#PAY-3081" || len(tr.Steps) != 4 {
				t.Errorf("unexpected pay order_tracker: code=%q steps=%d", tr.OrderCode, len(tr.Steps))
			}
		case snippets.SnippetTypeComposedBlock:
			cb, err := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](env)
			if err != nil {
				t.Fatalf("decode composed_block: %v", err)
			}
			rawCB, _ := json.Marshal(cb)
			switch cb.RecipeName {
			case "wallet_stats_and_alert":
				foundStatsBlock = true
				for _, wantKind := range []string{snippets.ComposedNodeStatTile, snippets.ComposedNodeAlertBanner, snippets.ComposedNodeBadge} {
					if !strings.Contains(string(rawCB), `"`+wantKind+`"`) {
						t.Errorf("wallet_stats_and_alert missing composed node kind %q", wantKind)
					}
				}
			case "pay_scan_qr_card":
				foundQrBlock = true
				for _, wantKind := range []string{snippets.ComposedNodeQrCode, snippets.ComposedNodeChip} {
					if !strings.Contains(string(rawCB), `"`+wantKind+`"`) {
						t.Errorf("pay_scan_qr_card missing composed node kind %q", wantKind)
					}
				}
			}
		case snippets.SnippetTypeActionRow:
			if strings.HasPrefix(env.SnippetID, "pay_contact_") {
				contactRows++
			}
		}
	}
	if !foundMarquee || !foundTracker || !foundStatsBlock || !foundQrBlock || contactRows != 4 {
		t.Fatalf("pay-home missing expected sections: marquee=%v tracker=%v stats=%v qr=%v contacts=%d", foundMarquee, foundTracker, foundStatsBlock, foundQrBlock, contactRows)
	}

	// Search filtering contacts.
	mayaPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pay-home", Query: "maya"})
	if err != nil {
		t.Fatalf("RenderScreen(search maya) error: %v", err)
	}
	matchedRows := 0
	for _, env := range mayaPage.Snippets {
		if strings.HasPrefix(env.SnippetID, "pay_contact_") {
			matchedRows++
		}
	}
	if matchedRows != 1 {
		t.Fatalf("expected 1 contact matching maya, got %d", matchedRows)
	}

	// Empty search renders empty_state.
	emptyPage, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pay-home", Query: "no-such-handle-999"})
	if err != nil {
		t.Fatalf("RenderScreen(empty search) error: %v", err)
	}
	var foundEmpty bool
	for _, env := range emptyPage.Snippets {
		if env.SnippetType == snippets.SnippetTypeEmptyState {
			foundEmpty = true
		}
	}
	if !foundEmpty {
		t.Fatal("expected empty_state snippet when no contacts match")
	}
}

func TestPayTransferScreenAndExecution(t *testing.T) {
	t.Parallel()

	mod := pay.New()
	page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "pay-transfer",
		Filter:   "arjun",
	})
	if err != nil {
		t.Fatalf("RenderScreen(pay-transfer) error: %v", err)
	}
	assertAllTrackingPropsAreStrings(t, page)

	var foundOptionsBlock, foundForm, foundBiometric bool
	for _, env := range page.Snippets {
		switch env.SnippetType {
		case snippets.SnippetTypeComposedBlock:
			cb, decErr := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](env)
			if decErr != nil {
				t.Fatalf("decode transfer composed_block: %v", decErr)
			}
			if cb.RecipeName == "transfer_speed_funding_pin" {
				foundOptionsBlock = true
				rawCB, _ := json.Marshal(cb)
				for _, want := range []string{
					`"` + snippets.ComposedNodeSegmentedControl + `"`,
					`"` + snippets.ComposedNodeRadioGroup + `"`,
					`"` + snippets.ComposedNodeOtpInput + `"`,
					`"` + snippets.ComposedNodeKeyValueRow + `"`,
					`"$0.00"`,
					`"masked":true`,
					`/api/v1/pay/transfers`,
					`@arjun.rao`,
				} {
					if !strings.Contains(string(rawCB), want) {
						t.Errorf("pay-transfer composed_block missing %s", want)
					}
				}
			}
		case snippets.SnippetTypeForm:
			foundForm = true
			form, decErr := snippets.DecodeSnippetData[snippets.FormSnippetData](env)
			if decErr != nil || form.SubmitButton.OnClick == nil {
				t.Fatalf("unexpected transfer form: %+v (err=%v)", form, decErr)
			}
			callAPI, decErr := actions.DecodePayload[actions.CallAPIPayload](*form.SubmitButton.OnClick)
			if decErr != nil || callAPI.Endpoint != "/api/v1/pay/transfers" || callAPI.Method != "POST" {
				t.Fatalf("unexpected transfer call_api: %+v (err=%v)", callAPI, decErr)
			}
		case snippets.SnippetTypeActionRow:
			if env.SnippetID == "pay_biometric_auth_row" {
				foundBiometric = true
			}
		}
	}
	if !foundOptionsBlock || !foundForm || !foundBiometric {
		t.Fatalf("missing pay-transfer sections: options=%v form=%v bio=%v", foundOptionsBlock, foundForm, foundBiometric)
	}

	// Validation error cases.
	badCases := []domain.TransferRequest{
		{Recipient: "not-a-handle"},
		{AmountUSD: -10},
		{AmountCents: -100},
		{AmountCents: 300000},
		{Speed: "carrier_pigeon"},
		{FundingSource: "mattress"},
		{Pin: "12ab"},
		{Pin: "123"},
	}
	for _, tc := range badCases {
		if _, valErr := mod.ExecuteTransfer(context.Background(), tc); !errors.Is(valErr, domain.ErrInvalidInput) {
			t.Errorf("ExecuteTransfer(%+v): expected ErrInvalidInput, got %v", tc, valErr)
		}
	}

	// Execute a valid instant transfer.
	res, err := mod.ExecuteTransfer(context.Background(), domain.TransferRequest{
		Recipient:     "@arjun.rao",
		AmountUSD:     200.00,
		Speed:         "instant",
		FundingSource: "checking",
		Pin:           "4829",
		Memo:          "Team offsite dinner",
	})
	if err != nil {
		t.Fatalf("ExecuteTransfer valid error: %v", err)
	}
	assertAllTrackingPropsAreStrings(t, res)

	transferID, _ := res.Data["transfer_id"].(string)
	if transferID != "PAY-3082" {
		t.Fatalf("transfer_id = %q, want PAY-3082", transferID)
	}
	if len(res.FollowUpActions) != 3 {
		t.Fatalf("expected 3 follow_up_actions (celebration, toast, navigate), got %d", len(res.FollowUpActions))
	}
	if res.FollowUpActions[0].Type != actions.ActionTriggerFeedback {
		t.Errorf("follow-up[0] = %q, want trigger_feedback", res.FollowUpActions[0].Type)
	}
	fb, err := actions.DecodePayload[actions.TriggerFeedbackPayload](res.FollowUpActions[0])
	if err != nil || fb.Effect != actions.FeedbackCelebration {
		t.Errorf("unexpected celebration feedback: %+v (err=%v)", fb, err)
	}
	if res.FollowUpActions[1].Type != actions.ActionShowToast {
		t.Errorf("follow-up[1] = %q, want show_toast", res.FollowUpActions[1].Type)
	}
	nav, err := actions.DecodePayload[actions.NavigatePayload](res.FollowUpActions[2])
	if err != nil || !strings.HasPrefix(nav.Route, "pay-receipt") {
		t.Errorf("unexpected navigate follow-up: %+v (err=%v)", nav, err)
	}

	// Scheduled transfer from cashback_wallet with Note alias.
	schedRes, err := mod.ExecuteTransfer(context.Background(), domain.TransferRequest{
		AmountCents:   5000,
		Speed:         "scheduled",
		FundingSource: "cashback_wallet",
		Note:          "Monthly split",
	})
	if err != nil {
		t.Fatalf("scheduled ExecuteTransfer error: %v", err)
	}
	if schedRes.Data["status"] != "scheduled" {
		t.Errorf("expected status=scheduled, got %+v", schedRes.Data["status"])
	}
	if len(mod.Transfers()) != 3 {
		t.Fatalf("expected 3 transfers total (seed + 2 new), got %d", len(mod.Transfers()))
	}
}

func TestPayCardsAndControlsMutation(t *testing.T) {
	t.Parallel()

	mod := pay.New()

	// Initial active state.
	page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pay-cards"})
	if err != nil {
		t.Fatalf("RenderScreen(pay-cards) error: %v", err)
	}
	assertAllTrackingPropsAreStrings(t, page)

	var foundSecurityBlock, foundForm bool
	for _, env := range page.Snippets {
		switch env.SnippetType {
		case snippets.SnippetTypeComposedBlock:
			cb, decErr := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](env)
			if decErr != nil {
				t.Fatalf("decode card composed_block: %v", decErr)
			}
			if cb.RecipeName == "card_security_status" {
				foundSecurityBlock = true
				rawCB, _ := json.Marshal(cb)
				for _, want := range []string{
					`"` + snippets.ComposedNodeBadge + `"`,
					`"` + snippets.ComposedNodeAlertBanner + `"`,
					`"` + snippets.ComposedNodeKeyValueRow + `"`,
					`"ACTIVE"`,
				} {
					if !strings.Contains(string(rawCB), want) {
						t.Errorf("card_security_status missing %s", want)
					}
				}
			}
		case snippets.SnippetTypeForm:
			foundForm = true
			form, decErr := snippets.DecodeSnippetData[snippets.FormSnippetData](env)
			if decErr != nil || len(form.Switches) != 3 || len(form.Sliders) != 1 {
				t.Fatalf("unexpected cards form: switches=%d sliders=%d err=%v", len(form.Switches), len(form.Sliders), decErr)
			}
			callAPI, decErr := actions.DecodePayload[actions.CallAPIPayload](*form.SubmitButton.OnClick)
			if decErr != nil || callAPI.Endpoint != "/api/v1/pay/cards/controls" {
				t.Fatalf("unexpected card controls endpoint: %+v (err=%v)", callAPI, decErr)
			}
		}
	}
	if !foundSecurityBlock || !foundForm {
		t.Fatalf("missing pay-cards sections: security=%v form=%v", foundSecurityBlock, foundForm)
	}

	// Validation errors.
	for _, bad := range []domain.CardControlRequest{
		{SpendLimitUSD: -100},
		{SpendLimitUSD: 100},
		{SpendLimitUSD: 25000},
	} {
		if _, valErr := mod.UpdateCardControls(context.Background(), bad); !errors.Is(valErr, domain.ErrInvalidInput) {
			t.Errorf("UpdateCardControls(%+v): expected ErrInvalidInput, got %v", bad, valErr)
		}
	}

	// Freeze card, disable international & contactless, adjust limit to $8,500.
	res, err := mod.UpdateCardControls(context.Background(), domain.CardControlRequest{
		CardID:        "metal_4092",
		Frozen:        atoms.BoolPtr(true),
		International: atoms.BoolPtr(false),
		Contactless:   atoms.BoolPtr(false),
		SpendLimitUSD: 8500,
	})
	if err != nil {
		t.Fatalf("UpdateCardControls error: %v", err)
	}
	assertAllTrackingPropsAreStrings(t, res)
	if res.Data["state"] != "FROZEN" || len(res.FollowUpActions) != 3 {
		t.Fatalf("unexpected card controls result: %+v", res)
	}

	// Frozen state propagates to both pay-cards and pay-home.
	frozenCards, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pay-cards"})
	if err != nil {
		t.Fatalf("RenderScreen(frozen pay-cards) error: %v", err)
	}
	rawCards, _ := json.Marshal(frozenCards)
	if !strings.Contains(string(rawCards), "FROZEN") || !strings.Contains(string(rawCards), "$8,500.00") {
		t.Errorf("expected frozen card status and $8,500.00 limit on pay-cards")
	}

	frozenHome, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "pay-home"})
	if err != nil {
		t.Fatalf("RenderScreen(frozen pay-home) error: %v", err)
	}
	rawHome, _ := json.Marshal(frozenHome)
	if !strings.Contains(string(rawHome), "CARD FROZEN") {
		t.Errorf("expected CARD FROZEN badge on pay-home")
	}
}

func TestPayReceiptScreenAndShareAction(t *testing.T) {
	t.Parallel()

	mod := pay.New()
	page, err := mod.RenderScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "pay-receipt",
		Extra:    map[string]string{"id": "PAY-3081"},
	})
	if err != nil {
		t.Fatalf("RenderScreen(pay-receipt) error: %v", err)
	}
	assertAllTrackingPropsAreStrings(t, page)

	var foundTracker, foundBreakdown, foundShareRow bool
	for _, env := range page.Snippets {
		switch env.SnippetType {
		case snippets.SnippetTypeOrderTracker:
			foundTracker = true
		case snippets.SnippetTypeComposedBlock:
			cb, decErr := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](env)
			if decErr != nil {
				t.Fatalf("decode receipt composed_block: %v", decErr)
			}
			if cb.RecipeName == "pay_itemized_receipt" {
				foundBreakdown = true
				rawCB, _ := json.Marshal(cb)
				for _, want := range []string{
					`"` + snippets.ComposedNodeAlertBanner + `"`,
					`"` + snippets.ComposedNodeKeyValueRow + `"`,
					`"` + snippets.ComposedNodeQrCode + `"`,
					`"#PAY-3081"`,
					`"+$6.25"`,
					`"` + string(actions.ActionShare) + `"`,
				} {
					if !strings.Contains(string(rawCB), want) {
						t.Errorf("pay_itemized_receipt missing %s", want)
					}
				}
			}
		case snippets.SnippetTypeActionRow:
			if env.SnippetID == "pay_receipt_share_row" {
				foundShareRow = true
				row, decErr := snippets.DecodeSnippetData[snippets.ActionRowSnippetData](env)
				if decErr != nil || row.OnClick == nil || row.OnClick.Type != actions.ActionShare {
					t.Errorf("expected native share action on share row: %+v (err=%v)", row, decErr)
				}
			}
		}
	}
	if !foundTracker || !foundBreakdown || !foundShareRow {
		t.Fatalf("missing pay-receipt sections: tracker=%v breakdown=%v shareRow=%v", foundTracker, foundBreakdown, foundShareRow)
	}
}

func TestPayConcurrentSafety(t *testing.T) {
	t.Parallel()

	mod := pay.New()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			if _, err := mod.ExecuteTransfer(ctx, domain.TransferRequest{
				Recipient: "@elena.m",
				AmountUSD: 25.00,
				Speed:     "instant",
			}); err != nil {
				t.Errorf("concurrent ExecuteTransfer error: %v", err)
			}
		}()
		go func(idx int) {
			defer wg.Done()
			frozen := idx%2 == 0
			if _, err := mod.UpdateCardControls(ctx, domain.CardControlRequest{
				Frozen:        &frozen,
				SpendLimitUSD: 4000,
			}); err != nil {
				t.Errorf("concurrent UpdateCardControls error: %v", err)
			}
		}(i)
		go func() {
			defer wg.Done()
			_, _ = mod.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "pay-home"})
			_, _ = mod.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "pay-transfer"})
			_, _ = mod.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "pay-cards"})
			_, _ = mod.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "pay-receipt"})
			_ = mod.Transfers()
		}()
	}
	wg.Wait()

	if got := len(mod.Transfers()); got != 11 {
		t.Fatalf("expected 11 transfers (seed + 10 concurrent), got %d", got)
	}
}

func assertAllTrackingPropsAreStrings(t *testing.T, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	walkWireTree(t, tree, "$")
}

func walkWireTree(t *testing.T, node any, path string) {
	t.Helper()
	switch val := node.(type) {
	case map[string]any:
		if tracking, ok := val["tracking"].(map[string]any); ok {
			checkStringMap(t, tracking["properties"], path+".tracking.properties")
		}
		if val["type"] == string(actions.ActionTrackEvent) {
			if payload, ok := val["payload"].(map[string]any); ok {
				checkStringMap(t, payload["properties"], path+".payload.properties")
			}
		}
		for k, child := range val {
			walkWireTree(t, child, path+"."+k)
		}
	case []any:
		for _, item := range val {
			walkWireTree(t, item, path+"[]")
		}
	}
}

func checkStringMap(t *testing.T, raw any, path string) {
	t.Helper()
	if raw == nil {
		return
	}
	props, ok := raw.(map[string]any)
	if !ok {
		t.Errorf("%s is not a JSON object: %T", path, raw)
		return
	}
	for k, v := range props {
		if _, isStr := v.(string); !isStr {
			t.Errorf("KMP wire violation at %s.%s: expected string, got %T (%v)", path, k, v, v)
		}
	}
}
