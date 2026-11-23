package pay

import (
	"fmt"
	"strings"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

const (
	payRecipientRegex = `^(@[a-zA-Z0-9._-]{2,}|[^@\s]+@[^@\s]+\.[^@\s]+)$`
	payAmountRegex    = `^\d+(\.\d{1,2})?$`
)

type payContact struct {
	ID       string
	Name     string
	Handle   string
	Role     string
	Initials string
	LastNote string
}

type walletSnapshot struct {
	BalanceCents      int
	CashbackCents     int
	MonthlySpendCents int
	CardFrozen        bool
	CardInternational bool
	CardContactless   bool
	CardSpendLimitUSD float64
	LatestTransfer    domain.TransferRecord
	TransferCount     int
	Contacts          []payContact
}

func seedContacts() []payContact {
	return []payContact{
		{ID: "maya", Name: "Maya Chen", Handle: "@maya.chen", Role: "Design Lead", Initials: "MC", LastNote: "Dinner split"},
		{ID: "arjun", Name: "Arjun Rao", Handle: "@arjun.rao", Role: "Systems Architect", Initials: "AR", LastNote: "Coffee run"},
		{ID: "elena", Name: "Elena Morales", Handle: "@elena.m", Role: "Product Ops", Initials: "EM", LastNote: "Team lunch"},
		{ID: "noah", Name: "Noah Park", Handle: "@noah.park", Role: "Mobile Eng", Initials: "NP", LastNote: "Concert tickets"},
	}
}

func formatCents(cents int) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	dollars := cents / 100
	rem := cents % 100
	var whole string
	if dollars >= 1000 {
		whole = fmt.Sprintf("%d,%03d", dollars/1000, dollars%1000)
	} else {
		whole = fmt.Sprintf("%d", dollars)
	}
	if neg {
		return fmt.Sprintf("-$%s.%02d", whole, rem)
	}
	return fmt.Sprintf("$%s.%02d", whole, rem)
}

func formatUSD(amount float64) string {
	return formatCents(int(amount*100 + 0.5))
}

func formatSpeedLabel(speed string) string {
	switch strings.ToLower(strings.TrimSpace(speed)) {
	case "scheduled":
		return "Scheduled (Next Business Day)"
	case "autopay":
		return "AutoPay (Monthly Recurring)"
	default:
		return "Instant Settlement (< 2s)"
	}
}

func formatFundingLabel(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "cashback_wallet":
		return "Cashback Vault"
	case "reserve":
		return "High-Yield Reserve (5.0% APY)"
	default:
		return "Checking Account (•••• 4092)"
	}
}

func filterContacts(contacts []payContact, query string) []payContact {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return contacts
	}
	out := make([]payContact, 0, len(contacts))
	for _, c := range contacts {
		haystack := strings.ToLower(c.Name + " " + c.Handle + " " + c.Role + " " + c.LastNote)
		if strings.Contains(haystack, q) {
			out = append(out, c)
		}
	}
	return out
}

func resolveContactByFilter(contacts []payContact, filter string) payContact {
	f := strings.ToLower(strings.TrimSpace(filter))
	for _, c := range contacts {
		if c.ID == f || strings.EqualFold(c.Handle, f) {
			return c
		}
	}
	return contacts[0]
}

func payCenteredAdaptive() snippets.AdaptivePageSpec {
	return snippets.AdaptivePageSpec{
		LayoutMode: snippets.LayoutModeCentered,
		MaxWidthDp: atoms.IntPtr(1080),
	}
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

func moldedCardLayout(border theming.ColorToken) atoms.LayoutConfig {
	return atoms.LayoutConfig{
		Padding:      atoms.UniformSpacing(theming.SpacingLG.Dp()),
		Margin:       atoms.SymmetricSpacing(theming.SpacingMD.Dp(), theming.SpacingXS.Dp()),
		BgColor:      atoms.ColorRef(theming.ColorSurfacePrimary),
		CornerRadius: atoms.UniformRadius(theming.RadiusLG.Dp()),
		Border: &atoms.BorderSpec{
			WidthDp: 1,
			Color:   *atoms.ColorRef(border),
		},
		FillMaxWidth: atoms.BoolPtr(true),
	}
}

func buildPerksMarquee() snippets.SnippetEnvelope {
	instantIcon := atoms.NewIcon("bolt", 16, theming.ColorBrandPrimary)
	cashbackIcon := atoms.NewIcon("star", 16, theming.ColorSuccess)
	fxIcon := atoms.NewIcon("check_circle", 16, theming.ColorInfo)

	instantBadge := atoms.NewTag("INSTANT", theming.ColorSuccessContainer)
	cashbackBadge := atoms.NewTag("5% BACK", theming.ColorBrandContainer)
	fxBadge := atoms.NewTag("0% FX", theming.ColorInfoContainer)

	instantToast := actions.NewShowToastAction("pay_marquee_instant_toast", actions.ShowToastPayload{
		Message:    "Instant P2P transfers settle in under 2 seconds with $0 fee",
		DurationMs: 2200,
		Variant:    "info",
	})
	cashbackToast := actions.NewShowToastAction("pay_marquee_cashback_toast", actions.ShowToastPayload{
		Message:    "5% instant cashback active on Abeta Metal Virtual Card",
		DurationMs: 2200,
		Variant:    "success",
	})

	return snippets.NewMarqueeStripSnippet(
		"pay_perks_marquee",
		"pay_live_perks",
		snippets.MarqueeItem{
			ID:      "pay_mq_instant",
			Text:    "Zero-fee instant P2P transfers",
			Icon:    &instantIcon,
			Badge:   &instantBadge,
			OnClick: &instantToast,
		},
		snippets.MarqueeItem{
			ID:      "pay_mq_cashback",
			Text:    "5% cashback on Abeta Metal Card",
			Icon:    &cashbackIcon,
			Badge:   &cashbackBadge,
			OnClick: &cashbackToast,
		},
		snippets.MarqueeItem{
			ID:    "pay_mq_fx",
			Text:  "0% international FX markup across 140+ currencies",
			Icon:  &fxIcon,
			Badge: &fxBadge,
		},
	).
		WithVelocityDpPerSec(48).
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
		WithTracking("pay_marquee_impression", map[string]any{
			"vertical": verticalID,
			"strip_id": "pay_live_perks",
		}).
		Build()
}

func buildWalletStatsAndAlertBlock(snap walletSnapshot) snippets.SnippetEnvelope {
	walletIcon := atoms.NewIcon("star", 20, theming.ColorBrandPrimary)
	cashbackIcon := atoms.NewIcon("check_circle", 20, theming.ColorSuccess)

	navTransfer := actions.NewNavigateAction("pay_stat_nav_transfer", actions.NavigatePayload{Route: screenTransfer})
	navCards := actions.NewNavigateAction("pay_stat_nav_cards", actions.NavigatePayload{Route: screenCards})

	balanceStat := atoms.NewStatTile("pay_stat_balance", "Available Balance", formatCents(snap.BalanceCents))
	balanceStat.DeltaLabel = "+5.0% APY"
	balanceStat.DeltaPositive = atoms.BoolPtr(true)
	balanceStat.Caption = "Checking · •••• 4092"
	balanceStat.Icon = &walletIcon
	balanceStat.OnClick = &navTransfer

	cashbackStat := atoms.NewStatTile("pay_stat_cashback", "Cashback Vault", formatCents(snap.CashbackCents))
	cashbackStat.DeltaLabel = "5% Metal Tier"
	cashbackStat.DeltaPositive = atoms.BoolPtr(true)
	cashbackStat.Caption = fmt.Sprintf("%d instant settlements", snap.TransferCount)
	cashbackStat.Icon = &cashbackIcon
	cashbackStat.OnClick = &navCards

	statusBadge := atoms.BadgeAtom{
		ID:      "pay_wallet_status_badge",
		Label:   "RAILS LIVE",
		Variant: atoms.VariantSuccess,
		Motion:  &atoms.MotionSpec{LoopEffect: atoms.MotionLoopPulse},
	}
	alert := atoms.NewAlertBanner(
		"pay_home_alert_banner",
		"Real-Time Zero-Fee Clearing Active",
		"P2P transfers settle in < 2s with 5% Metal cashback and 3D-Secure protection.",
		atoms.VariantSuccess,
	)
	alert.ActionLabel = "Card Controls"
	alert.OnAction = &navCards
	if snap.CardFrozen {
		statusBadge.Label = "CARD FROZEN"
		statusBadge.Variant = atoms.VariantWarning
		alert = atoms.NewAlertBanner(
			"pay_home_alert_banner",
			"Abeta Metal Card Temporarily Frozen",
			"New card charges are paused while P2P wallet transfers remain active.",
			atoms.VariantWarning,
		)
		alert.ActionLabel = "Unfreeze"
		alert.OnAction = &navCards
	}

	root := snippets.CardNode(
		moldedCardLayout(theming.ColorBrandPrimary),
		snippets.VStack(
			theming.SpacingMD.Dp(),
			snippets.HStack(
				theming.SpacingSM.Dp(),
				snippets.TextNode(atoms.NewText("Abeta Pay Wallet Telemetry", theming.TypographyTitleMedium, theming.ColorTextPrimary)).WithWeight(1),
				snippets.BadgeNode(statusBadge).WithNodeID("pay_home_status_badge_node"),
			).WithArrangement(snippets.ComposedAlignSpaceBetween).WithAlignment("", snippets.ComposedAlignCenter),
			snippets.HStack(
				theming.SpacingMD.Dp(),
				snippets.StatTileNode(balanceStat).WithNodeID("pay_stat_balance_node").WithWeight(1),
				snippets.StatTileNode(cashbackStat).WithNodeID("pay_stat_cashback_node").WithWeight(1),
			),
			snippets.AlertBannerNode(alert).WithNodeID("pay_home_alert_node"),
		),
	).WithNodeID("pay_wallet_stats_root")

	return snippets.NewComposedBlockSnippet("pay_wallet_stats_block", "pay_wallet_stats", root).
		WithRecipeName("wallet_stats_and_alert").
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideUp,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("composed_recipe_viewed", map[string]any{
			"vertical": verticalID,
			"recipe":   "wallet_stats_and_alert",
		}).
		Build()
}

func buildPayScanQrBlock(snap walletSnapshot) snippets.SnippetEnvelope {
	qrPayload := fmt.Sprintf("abeta-pay://pay?handle=@alex.rivera&ref=%s&currency=USD", snap.LatestTransfer.TransferID)
	copyHandle := actions.NewCopyToClipboardAction("pay_copy_handle", actions.CopyToClipboardPayload{
		Text:  "@alex.rivera",
		Label: "Abeta Pay Handle",
	})
	qrAtom := atoms.NewQrCode("pay_home_scan_qr", qrPayload, "Scan to Pay @alex.rivera", 156)
	qrAtom.Caption = "Zero-fee instant P2P QR · Works across Abeta Pay & Partner Rails"
	qrAtom.OnClick = &copyHandle

	navSend := actions.NewNavigateAction("pay_qr_send_now", actions.NavigatePayload{Route: screenTransfer})
	sendBtn := atoms.NewButton("btn_pay_qr_send", "Send Money", "primary", &navSend)
	sendBtn.Size = "sm"

	quickChips := make([]snippets.ComposedNode, 0, len(snap.Contacts))
	for i, c := range snap.Contacts {
		chipNav := actions.NewNavigateAction("pay_qr_chip_"+c.ID, actions.NavigatePayload{
			Route:  screenTransfer + "?filter=" + c.ID,
			Params: map[string]string{"filter": c.ID},
		})
		quickChips = append(quickChips, snippets.ChipNode(atoms.NewChip("pay_chip_"+c.ID, c.Handle, i == 0, &chipNav)))
	}

	root := snippets.CardNode(
		moldedCardLayout(theming.ColorBorderSubtle),
		snippets.VStack(
			theming.SpacingMD.Dp(),
			snippets.HStack(
				theming.SpacingSM.Dp(),
				snippets.TextNode(atoms.NewText("Scan & Instant Pay QR", theming.TypographyTitleMedium, theming.ColorTextPrimary)).WithWeight(1),
				snippets.TagNode(atoms.NewTag("ZERO FEE", theming.ColorSuccessContainer)),
			).WithArrangement(snippets.ComposedAlignSpaceBetween).WithAlignment("", snippets.ComposedAlignCenter),
			snippets.QrCodeNode(qrAtom).WithNodeID("pay_home_qr_node").WithContentAlignment(snippets.ComposedAlignCenter),
			snippets.FlowRowNode(theming.SpacingSM.Dp(), quickChips...),
			snippets.ButtonNode(sendBtn),
		),
	).WithNodeID("pay_qr_scan_root")

	return snippets.NewComposedBlockSnippet("pay_qr_scan_block", "pay_qr_scan_card", root).
		WithRecipeName("pay_scan_qr_card").
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionFadeIn,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("composed_recipe_viewed", map[string]any{
			"vertical": verticalID,
			"recipe":   "pay_scan_qr_card",
		}).
		Build()
}

func buildSettlementOrderTracker(rec domain.TransferRecord, includeReceiptNav bool) snippets.SnippetEnvelope {
	checkIcon := atoms.NewIcon("check_circle", 16, theming.ColorSuccess)
	settledIcon := atoms.NewIcon("star", 16, theming.ColorTextInverse)

	etaCountdown := atoms.NewCountdown("pay_settlement_cd_"+rec.TransferID, 12, "Ledger sync")
	etaCountdown.UrgentThresholdSeconds = 5
	etaCountdown.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopPulse}

	etaBadge := atoms.NewTag("INSTANT", theming.ColorSuccessContainer)
	etaBadge.Motion = &atoms.MotionSpec{LoopEffect: atoms.MotionLoopPulse}

	shareAct := buildShareReceiptAction(rec)
	primaryBtn := atoms.NewButton("btn_pay_tracker_primary", "Share Receipt", "primary", &shareAct)
	primaryBtn.Size = "sm"
	if includeReceiptNav {
		navReceipt := actions.NewNavigateAction("pay_tracker_open_receipt", actions.NavigatePayload{
			Route:  screenReceipt + "?id=" + rec.TransferID,
			Params: map[string]string{"id": rec.TransferID},
		})
		primaryBtn = atoms.NewButton("btn_pay_tracker_receipt", "View Receipt", "primary", &navReceipt)
		primaryBtn.Size = "sm"
	}

	navTransfer := actions.NewNavigateAction("pay_tracker_send_again", actions.NavigatePayload{Route: screenTransfer})
	secondaryBtn := atoms.NewButton("btn_pay_tracker_send_again", "Send Again", "outline", &navTransfer)
	secondaryBtn.Size = "sm"

	statusCaption := "Confirmed"
	if rec.Status == "scheduled" {
		statusCaption = "Scheduled"
	}

	return snippets.NewOrderTrackerSnippet(
		"pay_settlement_tracker",
		"pay_tracker_"+rec.TransferID,
		"#"+rec.TransferID,
		atoms.NewText(
			fmt.Sprintf("%s sent to %s · %s", formatUSD(rec.AmountUSD), rec.Recipient, formatSpeedLabel(rec.Speed)),
			theming.TypographyTitleMedium,
			theming.ColorTextPrimary,
		),
		snippets.OrderTrackerStep{ID: "step_initiated", Label: "Initiated", Caption: "PIN & limit verified", Icon: &checkIcon, Completed: true},
		snippets.OrderTrackerStep{ID: "step_verified", Label: "Biometric Verified", Caption: "3DS & fraud check", Icon: &checkIcon, Completed: true},
		snippets.OrderTrackerStep{ID: "step_clearing", Label: "Instant Clearing", Caption: "Zero-fee rails", Icon: &checkIcon, Completed: true},
		snippets.OrderTrackerStep{ID: "step_settled", Label: "Settled", Caption: statusCaption, Icon: &settledIcon, Active: true},
	).
		WithEtaCountdown(etaCountdown).
		WithEtaBadge(etaBadge).
		WithRider("Abeta Real-Time Clearing", fmt.Sprintf("Ref #%s · Zero fee · +%s (5%%) cashback credited", rec.TransferID, formatUSD(rec.CashbackUSD))).
		WithPrimaryAction(primaryBtn).
		WithSecondaryAction(secondaryBtn).
		WithLayout(cardLayout()).
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideUp,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("pay_settlement_tracker_viewed", map[string]any{
			"vertical":    verticalID,
			"transfer_id": rec.TransferID,
		}).
		Build()
}

func buildTransferOptionsBlock(snap walletSnapshot, target payContact) snippets.SnippetEnvelope {
	saveSpeed := actions.NewSaveStateAction("pay_save_speed", actions.SaveStatePayload{
		Key:   "transfer_speed",
		Value: "instant",
		Scope: "session",
	})
	speedControl := atoms.NewSegmentedControl(
		"transfer_speed",
		"instant",
		atoms.OptionItem{ID: "instant", Label: "Instant", Subtitle: "< 2s"},
		atoms.OptionItem{ID: "scheduled", Label: "Scheduled", Subtitle: "Next day"},
		atoms.OptionItem{ID: "autopay", Label: "AutoPay", Subtitle: "Monthly"},
	)
	speedControl.OnChange = &saveSpeed

	saveSource := actions.NewSaveStateAction("pay_save_funding", actions.SaveStatePayload{
		Key:   "funding_source",
		Value: "checking",
		Scope: "session",
	})
	fundingGroup := atoms.NewRadioGroup(
		"funding_source",
		"Funding Source",
		"checking",
		atoms.OptionItem{ID: "checking", Label: "Checking Account", Subtitle: formatCents(snap.BalanceCents) + " available", Badge: "PRIMARY"},
		atoms.OptionItem{ID: "cashback_wallet", Label: "Cashback Wallet", Subtitle: formatCents(snap.CashbackCents) + " rewards", Badge: "5% BACK"},
		atoms.OptionItem{ID: "reserve", Label: "High-Yield Reserve", Subtitle: "5.0% APY instant liquidity"},
	)
	fundingGroup.OnChange = &saveSource

	pinComplete := actions.NewShowToastAction("pay_pin_verified_toast", actions.ShowToastPayload{
		Message:    "4-digit Transaction PIN captured — ready to send",
		DurationMs: 2000,
		Variant:    "success",
	})
	pinInput := atoms.NewOtpInput("transfer_pin", "Transaction PIN (4-digit)", 4, &pinComplete)
	pinInput.Masked = true
	pinInput.HelperText = "Protected by hardware enclave · Required for instant P2P transfers"

	amountRow := atoms.NewKeyValueRow("pay_kv_amount", "Transfer Amount", "$125.00")
	amountRow.Sublabel = "Recipient: " + target.Handle
	amountRow.ShowDivider = true

	feeRow := atoms.NewKeyValueRow("pay_kv_fee", "Transfer Fee", "$0.00")
	feeRow.Sublabel = "Zero-fee Abeta Real-Time Rails"
	feeRow.ValueColor = atoms.ColorRef(theming.ColorSuccess)
	feeRow.ShowDivider = true

	cashbackRow := atoms.NewKeyValueRow("pay_kv_cashback", "5% Metal Cashback", "+$6.25")
	cashbackRow.ValueColor = atoms.ColorRef(theming.ColorSuccess)
	cashbackRow.ShowDivider = true

	totalRow := atoms.NewKeyValueRow("pay_kv_total", "Total Debit", "$125.00")
	totalRow.Emphasized = true

	submitAct := buildTransferCallAPI(target.Handle, 12500, "instant", "checking", target.LastNote, false)
	submitBtn := atoms.NewButton("btn_pay_molded_send", "Confirm & Send $125.00", "primary", &submitAct)

	root := snippets.CardNode(
		moldedCardLayout(theming.ColorBrandPrimary),
		snippets.VStack(
			theming.SpacingMD.Dp(),
			snippets.TextNode(atoms.NewText("Transfer Speed & Security", theming.TypographyTitleMedium, theming.ColorTextPrimary)),
			snippets.SegmentedControlNode(speedControl).WithNodeID("pay_speed_segmented_node"),
			snippets.RadioGroupNode(fundingGroup).WithNodeID("pay_funding_radio_node"),
			snippets.OtpInputNode(pinInput).WithNodeID("pay_pin_otp_node"),
			snippets.DividerNode(atoms.DividerAtom{
				ThicknessDp: atoms.Float64Ptr(1),
				Color:       atoms.ColorRef(theming.ColorDivider),
			}),
			snippets.KeyValueRowNode(amountRow).WithNodeID("pay_fee_amount_row"),
			snippets.KeyValueRowNode(feeRow).WithNodeID("pay_fee_zero_row"),
			snippets.KeyValueRowNode(cashbackRow).WithNodeID("pay_fee_cashback_row"),
			snippets.KeyValueRowNode(totalRow).WithNodeID("pay_fee_total_row"),
			snippets.ButtonNode(submitBtn),
		),
	).WithNodeID("pay_transfer_options_root")

	return snippets.NewComposedBlockSnippet("pay_transfer_options_block", "pay_transfer_options", root).
		WithRecipeName("transfer_speed_funding_pin").
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideUp,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("composed_recipe_viewed", map[string]any{
			"vertical": verticalID,
			"recipe":   "transfer_speed_funding_pin",
		}).
		Build()
}

func buildCardSecurityStatusBlock(snap walletSnapshot) snippets.SnippetEnvelope {
	activeShields := 1 // 3D-Secure always active
	if snap.CardInternational {
		activeShields++
	}
	if snap.CardContactless {
		activeShields++
	}

	statusBadge := atoms.BadgeAtom{
		ID:      "pay_card_state_badge",
		Label:   "ACTIVE",
		Variant: atoms.VariantSuccess,
		Motion:  &atoms.MotionSpec{LoopEffect: atoms.MotionLoopPulse},
	}
	countBadge := atoms.NewBadge("pay_card_rules_badge", activeShields, 9, atoms.VariantBrand)

	alert := atoms.NewAlertBanner(
		"pay_card_security_alert",
		"Metal Card Security Shield Active",
		"Biometric 3D-Secure, instant spend alerts, and zero-liability protection are enabled.",
		atoms.VariantSuccess,
	)
	if snap.CardFrozen {
		statusBadge.Label = "FROZEN"
		statusBadge.Variant = atoms.VariantWarning
		alert = atoms.NewAlertBanner(
			"pay_card_security_alert",
			"Card Frozen — New Transactions Blocked",
			"All online, contactless, and POS authorizations on •••• 4092 are paused until unfrozen.",
			atoms.VariantWarning,
		)
	}

	limitRow := atoms.NewKeyValueRow("pay_card_kv_limit", "Monthly Spend Cap", formatUSD(snap.CardSpendLimitUSD))
	limitRow.Emphasized = true
	limitRow.ShowDivider = true

	fxLabel := "Enabled (0% FX markup)"
	fxColor := theming.ColorSuccess
	if !snap.CardInternational {
		fxLabel = "Disabled"
		fxColor = theming.ColorTextSecondary
	}
	fxRow := atoms.NewKeyValueRow("pay_card_kv_fx", "International Spend", fxLabel)
	fxRow.ValueColor = atoms.ColorRef(fxColor)
	fxRow.ShowDivider = true

	nfcLabel := "Enabled (NFC Tap-to-Pay)"
	if !snap.CardContactless {
		nfcLabel = "Disabled"
	}
	nfcRow := atoms.NewKeyValueRow("pay_card_kv_nfc", "Contactless Payments", nfcLabel)

	root := snippets.CardNode(
		moldedCardLayout(theming.ColorBrandPrimary),
		snippets.VStack(
			theming.SpacingMD.Dp(),
			snippets.HStack(
				theming.SpacingSM.Dp(),
				snippets.TextNode(atoms.NewText("Abeta Metal •••• 4092", theming.TypographyTitleMedium, theming.ColorTextPrimary)).WithWeight(1),
				snippets.BadgeNode(statusBadge).WithNodeID("pay_card_status_badge_node"),
				snippets.BadgeNode(countBadge).WithNodeID("pay_card_count_badge_node"),
			).WithArrangement(snippets.ComposedAlignSpaceBetween).WithAlignment("", snippets.ComposedAlignCenter),
			snippets.AlertBannerNode(alert).WithNodeID("pay_card_alert_node"),
			snippets.KeyValueRowNode(limitRow),
			snippets.KeyValueRowNode(fxRow),
			snippets.KeyValueRowNode(nfcRow),
		),
	).WithNodeID("pay_card_security_root")

	return snippets.NewComposedBlockSnippet("pay_card_security_block", "pay_card_security", root).
		WithRecipeName("card_security_status").
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionFadeIn,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("composed_recipe_viewed", map[string]any{
			"vertical": verticalID,
			"recipe":   "card_security_status",
		}).
		Build()
}

func buildReceiptBreakdownBlock(rec domain.TransferRecord) snippets.SnippetEnvelope {
	settledAlert := atoms.NewAlertBanner(
		"pay_receipt_settled_alert",
		fmt.Sprintf("Transfer #%s Settled", rec.TransferID),
		fmt.Sprintf("%s credited to %s via Abeta Real-Time Rails with zero transfer fee.", formatUSD(rec.AmountUSD), rec.Recipient),
		atoms.VariantSuccess,
	)

	refRow := atoms.NewKeyValueRow("pay_rcpt_ref", "Reference ID", "#"+rec.TransferID)
	refRow.Emphasized = true
	refRow.ShowDivider = true

	merchantRow := atoms.NewKeyValueRow("pay_rcpt_recipient", "Recipient / Merchant", rec.Recipient)
	merchantRow.Sublabel = rec.Memo
	merchantRow.ShowDivider = true

	speedRow := atoms.NewKeyValueRow("pay_rcpt_speed", "Settlement Rail", formatSpeedLabel(rec.Speed))
	speedRow.ShowDivider = true

	fundingRow := atoms.NewKeyValueRow("pay_rcpt_funding", "Funding Source", formatFundingLabel(rec.FundingSource))
	fundingRow.ShowDivider = true

	feeRow := atoms.NewKeyValueRow("pay_rcpt_fee", "Network Fee", formatUSD(rec.FeeUSD))
	feeRow.ValueColor = atoms.ColorRef(theming.ColorSuccess)
	feeRow.ShowDivider = true

	cashbackRow := atoms.NewKeyValueRow("pay_rcpt_cashback", "5% Cashback Credited", "+"+formatUSD(rec.CashbackUSD))
	cashbackRow.ValueColor = atoms.ColorRef(theming.ColorSuccess)
	cashbackRow.Emphasized = true

	qrPayload := fmt.Sprintf("abeta-pay://receipt/%s?recipient=%s&amount=%.2f&status=%s", rec.TransferID, rec.Recipient, rec.AmountUSD, rec.Status)
	qrAtom := atoms.NewQrCode("pay_receipt_verify_qr", qrPayload, "Verification Receipt #"+rec.TransferID, 168)
	qrAtom.Caption = "Scan to verify instant ledger settlement on Abeta Pay"

	shareAct := buildShareReceiptAction(rec)
	shareBtn := atoms.NewButton("btn_pay_receipt_share_card", "Share Verified Receipt", "primary", &shareAct)

	root := snippets.CardNode(
		moldedCardLayout(theming.ColorSuccess),
		snippets.VStack(
			theming.SpacingMD.Dp(),
			snippets.AlertBannerNode(settledAlert).WithNodeID("pay_receipt_alert_node"),
			snippets.KeyValueRowNode(refRow),
			snippets.KeyValueRowNode(merchantRow),
			snippets.KeyValueRowNode(speedRow),
			snippets.KeyValueRowNode(fundingRow),
			snippets.KeyValueRowNode(feeRow),
			snippets.KeyValueRowNode(cashbackRow),
			snippets.DividerNode(atoms.DividerAtom{
				ThicknessDp: atoms.Float64Ptr(1),
				Color:       atoms.ColorRef(theming.ColorDivider),
			}),
			snippets.QrCodeNode(qrAtom).WithNodeID("pay_receipt_qr_node").WithContentAlignment(snippets.ComposedAlignCenter),
			snippets.ButtonNode(shareBtn),
		),
	).WithNodeID("pay_receipt_breakdown_root")

	return snippets.NewComposedBlockSnippet("pay_receipt_breakdown_block", "pay_receipt_breakdown", root).
		WithRecipeName("pay_itemized_receipt").
		WithMotion(atoms.MotionSpec{
			Enter:         atoms.MotionTransitionSlideUp,
			DurationToken: atoms.MotionDurationNormal,
		}).
		WithTracking("composed_recipe_viewed", map[string]any{
			"vertical":    verticalID,
			"recipe":      "pay_itemized_receipt",
			"transfer_id": rec.TransferID,
		}).
		Build()
}

func buildTransferCallAPI(recipient string, amountCents int, speed, fundingSource, memo string, withValidations bool) actions.ActionTrigger {
	act := actions.NewCallAPIAction("pay_execute_transfer", actions.CallAPIPayload{
		Endpoint: transferEndpoint,
		Method:   "POST",
		Body: map[string]any{
			"recipient":      recipient,
			"amount_cents":   amountCents,
			"currency":       "USD",
			"speed":          speed,
			"funding_source": fundingSource,
			"memo":           memo,
		},
	})
	if withValidations {
		act = act.WithValidations(
			actions.ValidationRule{FieldID: "pay_recipient", Rule: "required", Message: "Recipient handle or email is required"},
			actions.ValidationRule{FieldID: "pay_recipient", Rule: "regex", Value: payRecipientRegex, Message: "Enter a valid @handle or email"},
			actions.ValidationRule{FieldID: "pay_amount", Rule: "required", Message: "Transfer amount is required"},
			actions.ValidationRule{FieldID: "pay_amount", Rule: "regex", Value: payAmountRegex, Message: "Enter a valid USD amount (e.g. 125.00)"},
		)
	}
	return act
}

func buildShareReceiptAction(rec domain.TransferRecord) actions.ActionTrigger {
	return actions.NewShareAction("share_pay_receipt_"+rec.TransferID, actions.SharePayload{
		Title:        "Abeta Pay Receipt #" + rec.TransferID,
		Text:         fmt.Sprintf("Sent %s to %s via Abeta Pay (Zero Fee · +%s cashback · Ref #%s)", formatUSD(rec.AmountUSD), rec.Recipient, formatUSD(rec.CashbackUSD), rec.TransferID),
		URL:          "https://abeta.dev/pay/receipt/" + rec.TransferID,
		ChooserTitle: "Share Payment Receipt",
	})
}

func buildPayActionRow(id, title, subtitle, iconName string, iconColor theming.ColorToken, tagLabel string, tagBg theming.ColorToken, onClick actions.ActionTrigger) snippets.SnippetEnvelope {
	return snippets.NewActionRowSnippet(id, atoms.NewText(title, theming.TypographyTitleSmall, theming.ColorTextPrimary)).
		WithSubtitle(atoms.NewText(subtitle, theming.TypographyBodySmall, theming.ColorTextSecondary)).
		WithLeadingIcon(atoms.NewIcon(iconName, 20, iconColor)).
		WithTrailingTag(atoms.NewTag(tagLabel, tagBg)).
		WithTrailingIcon(atoms.NewIcon("chevron_right", 18, theming.ColorTextTertiary)).
		WithOnClick(onClick).
		WithLayout(cardLayout()).
		Build()
}
