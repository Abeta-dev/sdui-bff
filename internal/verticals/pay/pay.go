// Package pay implements the Abeta Pay fintech wallet, P2P transfers,
// smart metal card controls, and instant settlement receipt vertical.
package pay

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

const (
	verticalID          = "abeta-pay"
	screenHome          = "pay-home"
	screenTransfer      = "pay-transfer"
	screenCards         = "pay-cards"
	screenReceipt       = "pay-receipt"
	transferEndpoint    = "/api/v1/pay/transfers"
	cardControlEndpoint = "/api/v1/pay/cards/controls"

	initialBalanceCents  = 248050
	initialCashbackCents = 14280
	initialSpendCents    = 142000
	defaultLimitUSD      = 5000.0
	minSpendLimitUSD     = 500.0
	maxSpendLimitUSD     = 15000.0
	maxTransferCents     = 250000
	defaultTransferCents = 12500
)

// Compile-time check ensuring Module satisfies domain.VerticalModule.
var _ domain.VerticalModule = (*Module)(nil)

// Module serves Abeta Pay SDUI screens, instant P2P transfer mutations,
// and smart metal card security controls.
type Module struct {
	mu                sync.RWMutex
	balanceCents      int
	cashbackCents     int
	monthlySpendCents int
	cardFrozen        bool
	cardInternational bool
	cardContactless   bool
	cardSpendLimitUSD float64
	contacts          []payContact
	transferSeq       int
	transfers         []domain.TransferRecord
}

// New initializes the Abeta Pay vertical with seeded wallet telemetry and settlement #PAY-3081.
func New() *Module {
	seedTransfer := domain.TransferRecord{
		TransferID:    "PAY-3081",
		Recipient:     "@maya.chen",
		AmountUSD:     125.00,
		FeeUSD:        0.00,
		CashbackUSD:   6.25,
		Currency:      "USD",
		Speed:         "instant",
		FundingSource: "checking",
		Memo:          "Dinner split & coffee",
		Status:        "settled",
		CreatedAt:     time.Date(2026, 11, 20, 9, 42, 3, 0, time.UTC),
	}
	return &Module{
		balanceCents:      initialBalanceCents,
		cashbackCents:     initialCashbackCents,
		monthlySpendCents: initialSpendCents,
		cardFrozen:        false,
		cardInternational: true,
		cardContactless:   true,
		cardSpendLimitUSD: defaultLimitUSD,
		contacts:          seedContacts(),
		transferSeq:       3081,
		transfers:         []domain.TransferRecord{seedTransfer},
	}
}

// Descriptor returns the vertical metadata for the 7-in-1 Super-App catalog.
func (m *Module) Descriptor() domain.AppDescriptor {
	return domain.AppDescriptor{
		ID:          verticalID,
		Name:        "Abeta Pay",
		Tagline:     "Zero-Fee P2P Wallet, Smart Metal Cards & Instant Settlement",
		Category:    "Fintech & Payments",
		TenantID:    verticalID,
		EntryScreen: screenHome,
		ThemePreset: "indigo",
		Screens:     []string{screenHome, screenTransfer, screenCards, screenReceipt},
	}
}

// Screens lists discoverable SDUI screen routes handled by Abeta Pay.
func (m *Module) Screens() []domain.ScreenSummary {
	return []domain.ScreenSummary{
		{
			ScreenID:    screenHome,
			Title:       "Abeta Pay · Wallet & Smart Cards",
			Vertical:    verticalID,
			Description: "Wallet balance & cashback StatTiles, perks marquee, live #PAY-3081 tracker, instant contacts, and molded scan-to-pay QR.",
		},
		{
			ScreenID:    screenTransfer,
			Title:       "Abeta Pay · Instant Transfer",
			Vertical:    verticalID,
			Description: "P2P transfer flow with SegmentedControl speed selector, RadioGroup funding source, masked 4-digit PIN OtpInput, and KeyValueRow fee summary.",
		},
		{
			ScreenID:    screenCards,
			Title:       "Abeta Pay · Smart Card Controls",
			Vertical:    verticalID,
			Description: "Metal virtual card switches, monthly spend limit slider, Badge status, and AlertBanner security status.",
		},
		{
			ScreenID:    screenReceipt,
			Title:       "Abeta Pay · Settlement Receipt",
			Vertical:    verticalID,
			Description: "Itemized KeyValueRow settlement breakdown, verification QrCode, settled AlertBanner, live tracker, and native OS share.",
		},
	}
}

// CanHandleScreen reports whether screenID belongs to Abeta Pay.
func (m *Module) CanHandleScreen(screenID string) bool {
	switch screenID {
	case screenHome, screenTransfer, screenCards, screenReceipt:
		return true
	default:
		return false
	}
}

// RenderScreen dispatches rendering across pay-home, pay-transfer, pay-cards, and pay-receipt.
func (m *Module) RenderScreen(ctx context.Context, req domain.ScreenRequest) (snippets.PageResponse, error) {
	if err := ctx.Err(); err != nil {
		return snippets.PageResponse{}, err
	}
	switch req.ScreenID {
	case screenHome:
		return m.renderHome(req), nil
	case screenTransfer:
		return m.renderTransfer(req), nil
	case screenCards:
		return m.renderCards(req), nil
	case screenReceipt:
		return m.renderReceipt(req), nil
	default:
		return snippets.PageResponse{}, fmt.Errorf("%w: %s", domain.ErrScreenNotFound, req.ScreenID)
	}
}

// ExecuteTransfer validates and records an instant/scheduled P2P transfer, returning
// celebration + toast + pay-receipt navigation follow_up_actions.
func (m *Module) ExecuteTransfer(ctx context.Context, req domain.TransferRequest) (domain.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActionResult{}, err
	}

	recipient := strings.TrimSpace(req.Recipient)
	if recipient == "" {
		recipient = "@maya.chen"
	} else if !strings.Contains(recipient, "@") {
		return domain.ActionResult{}, fmt.Errorf("%w: recipient must be a valid @handle or email", domain.ErrInvalidInput)
	}

	if req.AmountUSD < 0 || req.AmountCents < 0 {
		return domain.ActionResult{}, fmt.Errorf("%w: transfer amount cannot be negative", domain.ErrInvalidInput)
	}
	amountCents := req.AmountCents
	if amountCents == 0 && req.AmountUSD > 0 {
		amountCents = int(math.Round(req.AmountUSD * 100))
	}
	if amountCents == 0 {
		amountCents = defaultTransferCents
	}
	if amountCents > maxTransferCents {
		return domain.ActionResult{}, fmt.Errorf("%w: instant transfer amount cannot exceed $2,500.00", domain.ErrInvalidInput)
	}

	speed := strings.ToLower(strings.TrimSpace(req.Speed))
	if speed == "" {
		speed = "instant"
	}
	switch speed {
	case "instant", "scheduled", "autopay":
	default:
		return domain.ActionResult{}, fmt.Errorf("%w: unsupported transfer speed %q", domain.ErrInvalidInput, speed)
	}

	funding := strings.ToLower(strings.TrimSpace(req.FundingSource))
	if funding == "" {
		funding = "checking"
	}
	switch funding {
	case "checking", "cashback_wallet", "reserve":
	default:
		return domain.ActionResult{}, fmt.Errorf("%w: unsupported funding_source %q", domain.ErrInvalidInput, funding)
	}

	if pin := strings.TrimSpace(req.Pin); pin != "" {
		if len(pin) != 4 || !isDigitsOnly(pin) {
			return domain.ActionResult{}, fmt.Errorf("%w: pin must be exactly 4 numeric digits", domain.ErrInvalidInput)
		}
	}

	memo := strings.TrimSpace(req.Memo)
	if memo == "" {
		memo = strings.TrimSpace(req.Note)
	}
	if memo == "" {
		memo = "Instant P2P transfer"
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "USD"
	}

	cashbackCents := int(math.Round(float64(amountCents) * 0.05))
	amountUSD := float64(amountCents) / 100.0
	cashbackUSD := float64(cashbackCents) / 100.0
	status := "settled"
	if speed == "scheduled" {
		status = "scheduled"
	}

	m.mu.Lock()
	m.transferSeq++
	transferID := fmt.Sprintf("PAY-%d", m.transferSeq)
	if funding == "cashback_wallet" {
		m.cashbackCents = max(m.cashbackCents-amountCents, 0)
	} else {
		m.balanceCents = max(m.balanceCents-amountCents, 0)
	}
	m.cashbackCents += cashbackCents
	rec := domain.TransferRecord{
		TransferID:    transferID,
		Recipient:     recipient,
		AmountUSD:     amountUSD,
		FeeUSD:        0.00,
		CashbackUSD:   cashbackUSD,
		Currency:      currency,
		Speed:         speed,
		FundingSource: funding,
		Memo:          memo,
		Status:        status,
		CreatedAt:     time.Now().UTC(),
	}
	m.transfers = append(m.transfers, rec)
	balanceAfter := m.balanceCents
	m.mu.Unlock()

	celebration := actions.NewTriggerFeedbackAction("pay_transfer_celebration_"+transferID, actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCelebration,
		Title:      "Transfer Sent!",
		Subtitle:   fmt.Sprintf("%s sent to %s · Ref #%s", formatUSD(amountUSD), recipient, transferID),
		DurationMs: 2600,
	})
	toast := actions.NewShowToastAction("pay_transfer_toast_"+transferID, actions.ShowToastPayload{
		Message:    fmt.Sprintf("Settlement #%s confirmed · +%s (5%%) cashback credited", transferID, formatUSD(cashbackUSD)),
		DurationMs: 2500,
		Variant:    "success",
	})
	navReceipt := actions.NewNavigateAction("pay_transfer_nav_receipt_"+transferID, actions.NavigatePayload{
		Route:  screenReceipt + "?id=" + transferID,
		Params: map[string]string{"id": transferID},
	})

	return domain.ActionResult{
		Status:          "ok",
		Message:         fmt.Sprintf("Transfer #%s (%s to %s) %s", transferID, formatUSD(amountUSD), recipient, status),
		FollowUpActions: []actions.ActionTrigger{celebration, toast, navReceipt},
		Data: map[string]any{
			"transfer_id":           transferID,
			"recipient":             recipient,
			"amount_usd":            amountUSD,
			"fee_usd":               0.0,
			"cashback_usd":          cashbackUSD,
			"currency":              currency,
			"speed":                 speed,
			"funding_source":        funding,
			"status":                status,
			"balance_remaining_usd": float64(balanceAfter) / 100.0,
		},
	}, nil
}

// UpdateCardControls updates smart metal card switches and spend limit in thread-safe state.
func (m *Module) UpdateCardControls(ctx context.Context, req domain.CardControlRequest) (domain.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActionResult{}, err
	}
	if req.SpendLimitUSD < 0 {
		return domain.ActionResult{}, fmt.Errorf("%w: spend_limit_usd cannot be negative", domain.ErrInvalidInput)
	}
	if req.SpendLimitUSD > 0 && (req.SpendLimitUSD < minSpendLimitUSD || req.SpendLimitUSD > maxSpendLimitUSD) {
		return domain.ActionResult{}, fmt.Errorf("%w: spend_limit_usd must be between $500 and $15,000", domain.ErrInvalidInput)
	}

	m.mu.Lock()
	if req.Frozen != nil {
		m.cardFrozen = *req.Frozen
	}
	if req.International != nil {
		m.cardInternational = *req.International
	}
	if req.Contactless != nil {
		m.cardContactless = *req.Contactless
	}
	if req.SpendLimitUSD > 0 {
		m.cardSpendLimitUSD = req.SpendLimitUSD
	}
	frozen := m.cardFrozen
	intl := m.cardInternational
	nfc := m.cardContactless
	limitUSD := m.cardSpendLimitUSD
	m.mu.Unlock()

	cardID := strings.TrimSpace(req.CardID)
	if cardID == "" {
		cardID = "metal_4092"
	}

	stateLabel := "ACTIVE"
	toastVariant := "success"
	toastMsg := fmt.Sprintf("Metal Card controls synced · Monthly cap %s", formatUSD(limitUSD))
	if frozen {
		stateLabel = "FROZEN"
		toastVariant = "warning"
		toastMsg = "Abeta Metal Card •••• 4092 frozen — all new transactions blocked"
	}

	haptic := actions.NewTriggerFeedbackAction("pay_cards_haptic_"+cardID, actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackHapticTick,
		Title:      "Card Controls Updated",
		Subtitle:   toastMsg,
		DurationMs: 1800,
	})
	toast := actions.NewShowToastAction("pay_cards_toast_"+cardID, actions.ShowToastPayload{
		Message:    toastMsg,
		DurationMs: 2400,
		Variant:    toastVariant,
	})
	track := actions.NewTrackEventAction("pay_cards_track_"+cardID, actions.TrackEventPayload{
		EventName: "pay_card_controls_updated",
		Properties: map[string]any{
			"vertical":      verticalID,
			"card_id":       cardID,
			"card_state":    stateLabel,
			"international": strconv.FormatBool(intl),
			"contactless":   strconv.FormatBool(nfc),
			"limit_usd":     strconv.Itoa(int(math.Round(limitUSD))),
		},
	})

	return domain.ActionResult{
		Status:          "ok",
		Message:         toastMsg,
		FollowUpActions: []actions.ActionTrigger{haptic, toast, track},
		Data: map[string]any{
			"card_id":         cardID,
			"frozen":          frozen,
			"international":   intl,
			"contactless":     nfc,
			"spend_limit_usd": limitUSD,
			"state":           stateLabel,
		},
	}, nil
}

// Transfers returns a concurrency-safe snapshot of all recorded Abeta Pay transfers.
func (m *Module) Transfers() []domain.TransferRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.TransferRecord, len(m.transfers))
	copy(out, m.transfers)
	return out
}

func (m *Module) snapshot() walletSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	contacts := make([]payContact, len(m.contacts))
	copy(contacts, m.contacts)
	latest := m.transfers[len(m.transfers)-1]
	return walletSnapshot{
		BalanceCents:      m.balanceCents,
		CashbackCents:     m.cashbackCents,
		MonthlySpendCents: m.monthlySpendCents,
		CardFrozen:        m.cardFrozen,
		CardInternational: m.cardInternational,
		CardContactless:   m.cardContactless,
		CardSpendLimitUSD: m.cardSpendLimitUSD,
		LatestTransfer:    latest,
		TransferCount:     len(m.transfers),
		Contacts:          contacts,
	}
}

func (m *Module) findTransfer(id string) domain.TransferRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	trimmed := strings.TrimSpace(strings.TrimPrefix(id, "#"))
	if trimmed != "" {
		for i := len(m.transfers) - 1; i >= 0; i-- {
			if strings.EqualFold(m.transfers[i].TransferID, trimmed) {
				return m.transfers[i]
			}
		}
	}
	return m.transfers[len(m.transfers)-1]
}

func isDigitsOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
