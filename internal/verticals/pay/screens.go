package pay

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

func (m *Module) renderHome(req domain.ScreenRequest) snippets.PageResponse {
	snap := m.snapshot()
	query := strings.TrimSpace(req.Query)
	contacts := filterContacts(snap.Contacts, query)

	navCards := actions.NewNavigateAction("nav_pay_cards_toolbar", actions.NavigatePayload{Route: screenCards})
	cardsIcon := atoms.NewIcon("settings", 22, theming.ColorBrandPrimary)
	cardsIcon.ContentDescription = "Open Smart Card Controls"
	cardsIcon.OnClick = &navCards

	searchBar := snippets.NewSearchBarSnippet("pay_search_bar", "pay_search", "Search contacts, @handles & transfers...").
		WithQuery(query).
		WithLeadingIcon(atoms.NewIcon("search", 18, theming.ColorTextTertiary)).
		WithSubmitLabel("Search").
		WithClearable(true).
		WithOnSubmit(actions.NewNavigateAction("submit_pay_search", actions.NavigatePayload{Route: "pay-home?q={{query}}"})).
		WithOnClear(actions.NewNavigateAction("clear_pay_search", actions.NavigatePayload{Route: screenHome})).
		WithLayout(sectionLayout()).
		Build()

	navHome := actions.NewNavigateAction("chip_pay_wallet", actions.NavigatePayload{Route: screenHome})
	navSend := actions.NewNavigateAction("chip_pay_send", actions.NavigatePayload{Route: screenTransfer})
	navCard := actions.NewNavigateAction("chip_pay_cards", actions.NavigatePayload{Route: screenCards})
	navRcpt := actions.NewNavigateAction("chip_pay_receipt", actions.NavigatePayload{Route: screenReceipt})
	quickChips := snippets.NewChipGroupSnippet(
		"pay_quick_chips",
		"pay_filters",
		snippets.FilterChipItem{ID: "wallet", Label: "Wallet", Selected: true, OnClick: &navHome},
		snippets.FilterChipItem{ID: "send", Label: "Send Money", OnClick: &navSend},
		snippets.FilterChipItem{ID: "cards", Label: "Smart Cards", OnClick: &navCard},
		snippets.FilterChipItem{ID: "receipt", Label: "Receipt #" + snap.LatestTransfer.TransferID, OnClick: &navRcpt},
	).WithLayout(sectionLayout()).Build()

	pageSnippets := []snippets.SnippetEnvelope{
		searchBar,
		buildPerksMarquee(),
		quickChips,
		buildWalletStatsAndAlertBlock(snap),
		buildSettlementOrderTracker(snap.LatestTransfer, true),
		buildPayScanQrBlock(snap),
	}

	contactsHeader := snippets.NewHeaderSnippet(
		"pay_contacts_header",
		atoms.NewText("Instant P2P Contacts", theming.TypographyTitleMedium, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText("Zero-fee transfers settle in under 2 seconds", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithTrailingTag(atoms.NewTag(fmt.Sprintf("%d READY", len(contacts)), theming.ColorBrandContainer)).
		WithLayout(sectionLayout()).
		Build()
	pageSnippets = append(pageSnippets, contactsHeader)

	if len(contacts) == 0 {
		resetNav := actions.NewNavigateAction("pay_reset_search", actions.NavigatePayload{Route: screenHome})
		pageSnippets = append(pageSnippets, snippets.NewEmptyStateSnippet(
			"pay_contacts_empty",
			atoms.NewText("No matching contacts", theming.TypographyTitleMedium, theming.ColorTextPrimary),
		).
			WithMessage(atoms.NewText("Clear your search to browse instant Abeta Pay contacts.", theming.TypographyBodyMedium, theming.ColorTextSecondary)).
			WithIcon(atoms.NewIcon("search", 32, theming.ColorTextTertiary)).
			WithActionButton(atoms.NewButton("btn_pay_reset_contacts", "Show All Contacts", "primary", &resetNav)).
			WithLayout(cardLayout()).
			Build())
	} else {
		for _, c := range contacts {
			sendContact := actions.NewNavigateAction("pay_send_"+c.ID, actions.NavigatePayload{
				Route:  screenTransfer + "?filter=" + c.ID,
				Params: map[string]string{"filter": c.ID},
			})
			pageSnippets = append(pageSnippets, buildPayActionRow(
				"pay_contact_"+c.ID,
				c.Name+" · "+c.Handle,
				fmt.Sprintf("%s · Last note: %s", c.Role, c.LastNote),
				"person",
				theming.ColorBrandPrimary,
				"INSTANT",
				theming.ColorSuccessContainer,
				sendContact,
			))
		}
	}

	toolbarTitle := atoms.NewText("Abeta Pay", theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText("Wallet & Smart Cards · Zero-Fee P2P", theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenHome, "Abeta Pay · Wallet & Smart Cards").
		WithToolbar(snippets.ToolbarSpec{
			Title:           &toolbarTitle,
			Subtitle:        &toolbarSub,
			TrailingActions: []atoms.IconAtom{cardsIcon},
			BgColor:         atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(payCenteredAdaptive()).
		WithPullToRefresh(actions.NewRefreshPageAction("pay_pull_refresh", actions.RefreshPagePayload{
			PageID:     screenHome,
			ShowLoader: true,
		})).
		AddSnippets(pageSnippets...).
		WithTracking("page_loaded", map[string]any{
			"vertical":       verticalID,
			"page_id":        screenHome,
			"transfer_count": strconv.Itoa(snap.TransferCount),
			"card_frozen":    strconv.FormatBool(snap.CardFrozen),
		}).
		Build()
}

func (m *Module) renderTransfer(req domain.ScreenRequest) snippets.PageResponse {
	snap := m.snapshot()
	selectedContact := resolveContactByFilter(snap.Contacts, req.Filter)

	backNav := actions.NewNavigateAction("pay_transfer_back", actions.NavigatePayload{Route: screenHome})
	backIcon := atoms.NewIcon("arrow_back", 22, theming.ColorTextPrimary)
	backIcon.ContentDescription = "Back to Abeta Pay"
	backIcon.OnClick = &backNav

	header := snippets.NewHeaderSnippet(
		"pay_transfer_header",
		atoms.NewText("Instant P2P Transfer", theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText("Available balance: "+formatCents(snap.BalanceCents)+" · Settles in < 2 seconds", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithTrailingTag(atoms.NewTag("ZERO FEE", theming.ColorSuccessContainer)).
		WithLayout(sectionLayout()).
		Build()

	chips := make([]snippets.FilterChipItem, 0, len(snap.Contacts))
	for _, c := range snap.Contacts {
		chipNav := actions.NewNavigateAction("pay_contact_chip_"+c.ID, actions.NavigatePayload{
			Route:  screenTransfer + "?filter=" + c.ID,
			Params: map[string]string{"filter": c.ID},
		})
		chips = append(chips, snippets.FilterChipItem{
			ID:       c.ID,
			Label:    c.Handle,
			Selected: c.ID == selectedContact.ID,
			OnClick:  &chipNav,
		})
	}
	contactChips := snippets.NewChipGroupSnippet("pay_contact_chips", "pay_recent_contacts", chips...).
		WithLayout(sectionLayout()).
		Build()

	optionsBlock := buildTransferOptionsBlock(snap, selectedContact)

	submitAct := buildTransferCallAPI(selectedContact.Handle, defaultTransferCents, "instant", "checking", selectedContact.LastNote, true)
	submitBtn := atoms.NewButton("btn_pay_send_now", "Send $125.00 Instantly", "primary", &submitAct)
	transferForm := snippets.NewFormSnippet("pay_transfer_form_snippet", "pay_transfer_form", submitBtn).
		WithTitle(atoms.NewText("Recipient & Amount", theming.TypographyTitleMedium, theming.ColorTextPrimary)).
		WithDescription(atoms.NewText("Validated client & server before real-time settlement.", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		AddFields(
			atoms.TextFieldAtom{
				FieldID:      "pay_recipient",
				Label:        "Recipient (@handle or email)",
				Placeholder:  "@maya.chen",
				InitialValue: selectedContact.Handle,
				HelperText:   "Instant Abeta Pay handle or verified email",
				Required:     true,
				Regex:        payRecipientRegex,
				KeyboardType: "text",
			},
			atoms.TextFieldAtom{
				FieldID:      "pay_amount",
				Label:        "Amount (USD)",
				Placeholder:  "125.00",
				InitialValue: "125.00",
				HelperText:   "Zero fee up to $2,500 per instant transfer",
				Required:     true,
				Regex:        payAmountRegex,
				KeyboardType: "number",
			},
			atoms.TextFieldAtom{
				FieldID:      "pay_note",
				Label:        "Memo (Optional)",
				Placeholder:  "Dinner split & coffee",
				InitialValue: selectedContact.LastNote,
				KeyboardType: "text",
			},
		).
		WithLayout(cardLayout()).
		Build()

	onAuth := buildTransferCallAPI(selectedContact.Handle, defaultTransferCents, "instant", "checking", selectedContact.LastNote, false)
	onFailed := actions.NewShowToastAction("pay_biometric_fallback", actions.ShowToastPayload{
		Message:    "Biometric unavailable on this host — use your 4-digit PIN above",
		DurationMs: 2200,
		Variant:    "warning",
	})
	bioAct := actions.NewBiometricAuthAction("pay_biometric_authorize", actions.BiometricAuthPayload{
		Reason:                "Authorize instant $125.00 P2P transfer to " + selectedContact.Handle,
		Title:                 "Abeta Pay Biometric Auth",
		AllowDeviceCredential: atoms.BoolPtr(true),
		OnAuthenticated:       &onAuth,
		OnFailed:              &onFailed,
	})
	bioRow := buildPayActionRow(
		"pay_biometric_auth_row",
		"Authorize with Face ID / Touch ID",
		"Confirm $125.00 instant transfer via OS biometric prompt",
		"person",
		theming.ColorBrandPrimary,
		"BIOMETRIC",
		theming.ColorBrandContainer,
		bioAct,
	)

	toolbarTitle := atoms.NewText("Send Money", theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText("Abeta Pay · Zero-Fee Instant P2P", theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenTransfer, "Send Money · Abeta Pay").
		WithToolbar(snippets.ToolbarSpec{
			Title:          &toolbarTitle,
			Subtitle:       &toolbarSub,
			NavigationIcon: &backIcon,
			BgColor:        atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(payCenteredAdaptive()).
		AddSnippets(header, contactChips, optionsBlock, transferForm, bioRow).
		WithTracking("page_loaded", map[string]any{
			"vertical": verticalID,
			"page_id":  screenTransfer,
			"contact":  selectedContact.ID,
		}).
		Build()
}

func (m *Module) renderCards(_ domain.ScreenRequest) snippets.PageResponse {
	snap := m.snapshot()

	backNav := actions.NewNavigateAction("pay_cards_back", actions.NavigatePayload{Route: screenHome})
	backIcon := atoms.NewIcon("arrow_back", 22, theming.ColorTextPrimary)
	backIcon.ContentDescription = "Back to Abeta Pay"
	backIcon.OnClick = &backNav

	headerTagLabel := "ACTIVE · •••• 4092"
	headerTagColor := theming.ColorSuccessContainer
	if snap.CardFrozen {
		headerTagLabel = "FROZEN · •••• 4092"
		headerTagColor = theming.ColorWarningContainer
	}

	header := snippets.NewHeaderSnippet(
		"pay_cards_header",
		atoms.NewText("Abeta Metal Virtual Card", theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithSubtitle(atoms.NewText("Real-time card security switches & monthly spend cap", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithTrailingTag(atoms.NewTag(headerTagLabel, headerTagColor)).
		WithLayout(sectionLayout()).
		Build()

	spendRatio := math.Min(float64(snap.MonthlySpendCents)/(snap.CardSpendLimitUSD*100.0), 1.0)
	spendMetric := snippets.NewMetricCardSnippet(
		"pay_cards_spend_metric",
		atoms.NewText("Monthly Card Spend", theming.TypographyLabelMedium, theming.ColorTextSecondary),
		atoms.NewText(formatCents(snap.MonthlySpendCents), theming.TypographyHeadlineSmall, theming.ColorTextPrimary),
	).
		WithDelta(atoms.NewTag(fmt.Sprintf("%d%% OF LIMIT", int(math.Round(spendRatio*100))), theming.ColorSuccessContainer)).
		WithIcon(atoms.NewIcon("settings", 20, theming.ColorBrandPrimary)).
		WithProgress(atoms.ProgressAtom{
			Kind:     "linear",
			Progress: atoms.Float64Ptr(spendRatio),
			Label:    fmt.Sprintf("%s / %s monthly spend limit", formatCents(snap.MonthlySpendCents), formatUSD(snap.CardSpendLimitUSD)),
			Color:    atoms.ColorRef(theming.ColorBrandPrimary),
		}).
		WithLayout(cardLayout()).
		Build()

	saveFreeze := actions.NewSaveStateAction("save_card_freeze", actions.SaveStatePayload{Key: "card_freeze", Value: snap.CardFrozen, Scope: "session"})
	saveIntl := actions.NewSaveStateAction("save_card_intl", actions.SaveStatePayload{Key: "card_international", Value: snap.CardInternational, Scope: "session"})
	saveNfc := actions.NewSaveStateAction("save_card_nfc", actions.SaveStatePayload{Key: "card_contactless", Value: snap.CardContactless, Scope: "session"})
	saveLimit := actions.NewSaveStateAction("save_card_limit", actions.SaveStatePayload{Key: "card_spend_limit", Value: snap.CardSpendLimitUSD, Scope: "session"})

	submitControls := actions.NewCallAPIAction("pay_update_card_controls", actions.CallAPIPayload{
		Endpoint: cardControlEndpoint,
		Method:   "POST",
		Body: map[string]any{
			"card_id":         "metal_4092",
			"frozen":          snap.CardFrozen,
			"international":   snap.CardInternational,
			"contactless":     snap.CardContactless,
			"spend_limit_usd": snap.CardSpendLimitUSD,
		},
	})

	freezeSwitch := atoms.NewSwitch("card_freeze", "Freeze Card Temporarily", snap.CardFrozen, &saveFreeze)
	freezeSwitch.Subtitle = "Instantly block all new card transactions & subscriptions"

	intlSwitch := atoms.NewSwitch("card_international", "International & FX Spend", snap.CardInternational, &saveIntl)
	intlSwitch.Subtitle = "Zero FX markup across 140+ global currencies"

	nfcSwitch := atoms.NewSwitch("card_contactless", "Contactless & Tap-to-Pay", snap.CardContactless, &saveNfc)
	nfcSwitch.Subtitle = "Enable NFC tap-to-pay on phones & wearables"

	limitSlider := atoms.NewSlider("card_spend_limit", "Monthly Spend Limit", snap.CardSpendLimitUSD, minSpendLimitUSD, maxSpendLimitUSD, &saveLimit)
	limitSlider.Steps = 29
	limitSlider.UnitLabel = "USD"

	controlsForm := snippets.NewFormSnippet(
		"pay_cards_controls_form",
		"pay_card_controls_form",
		atoms.NewButton("btn_save_pay_cards", "Save Card Controls", "primary", &submitControls),
	).
		WithTitle(atoms.NewText("Card Security & Spend Limits", theming.TypographyTitleMedium, theming.ColorTextPrimary)).
		WithDescription(atoms.NewText("Real-time switches & monthly spend cap for •••• 4092.", theming.TypographyBodySmall, theming.ColorTextSecondary)).
		AddSwitches(freezeSwitch, intlSwitch, nfcSwitch).
		AddSlider(limitSlider).
		WithLayout(cardLayout()).
		Build()

	toolbarTitle := atoms.NewText("Smart Card Controls", theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText("Abeta Metal · •••• 4092", theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenCards, "Smart Card Controls · Abeta Pay").
		WithToolbar(snippets.ToolbarSpec{
			Title:          &toolbarTitle,
			Subtitle:       &toolbarSub,
			NavigationIcon: &backIcon,
			BgColor:        atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(payCenteredAdaptive()).
		AddSnippets(header, buildCardSecurityStatusBlock(snap), spendMetric, controlsForm).
		WithTracking("page_loaded", map[string]any{
			"vertical":    verticalID,
			"page_id":     screenCards,
			"card_frozen": strconv.FormatBool(snap.CardFrozen),
			"limit_usd":   strconv.Itoa(int(math.Round(snap.CardSpendLimitUSD))),
		}).
		Build()
}

func (m *Module) renderReceipt(req domain.ScreenRequest) snippets.PageResponse {
	rec := m.findTransfer(req.Extra["id"])

	backNav := actions.NewNavigateAction("pay_receipt_back", actions.NavigatePayload{Route: screenHome})
	backIcon := atoms.NewIcon("arrow_back", 22, theming.ColorTextPrimary)
	backIcon.ContentDescription = "Back to Abeta Pay"
	backIcon.OnClick = &backNav

	shareAct := buildShareReceiptAction(rec)
	shareRow := buildPayActionRow(
		"pay_receipt_share_row",
		"Share Settlement Receipt #"+rec.TransferID,
		"Send verified zero-fee transfer confirmation via OS share sheet",
		"info",
		theming.ColorBrandPrimary,
		"SHARE",
		theming.ColorBrandContainer,
		shareAct,
	)

	toolbarTitle := atoms.NewText("Receipt #"+rec.TransferID, theming.TypographyTitleLarge, theming.ColorTextPrimary)
	toolbarSub := atoms.NewText("Abeta Pay · Instant Settlement", theming.TypographyBodySmall, theming.ColorTextSecondary)

	return snippets.NewPage(screenReceipt, fmt.Sprintf("Receipt #%s · Abeta Pay", rec.TransferID)).
		WithToolbar(snippets.ToolbarSpec{
			Title:          &toolbarTitle,
			Subtitle:       &toolbarSub,
			NavigationIcon: &backIcon,
			BgColor:        atoms.ColorRef(theming.ColorSurfacePrimary),
		}).
		WithAdaptiveLayout(payCenteredAdaptive()).
		AddSnippets(
			buildSettlementOrderTracker(rec, false),
			buildReceiptBreakdownBlock(rec),
			shareRow,
		).
		WithTracking("page_loaded", map[string]any{
			"vertical":    verticalID,
			"page_id":     screenReceipt,
			"transfer_id": rec.TransferID,
			"status":      rec.Status,
		}).
		Build()
}
