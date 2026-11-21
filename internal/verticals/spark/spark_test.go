package spark_test

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
	"github.com/abeta-dev/sdui-bff/internal/verticals/spark"
)

func TestDescriptorAndScreens(t *testing.T) {
	t.Parallel()
	m := spark.New()

	desc := m.Descriptor()
	if desc.ID != "abeta-spark" || desc.TenantID != "abeta-spark" || desc.EntryScreen != "spark-deck" || desc.ThemePreset != "amber" {
		t.Fatalf("unexpected descriptor: %+v", desc)
	}
	if len(desc.Screens) != 2 || desc.Screens[0] != "spark-deck" || desc.Screens[1] != "spark-matches" {
		t.Fatalf("unexpected descriptor screens: %v", desc.Screens)
	}
	if !m.CanHandleScreen("spark-deck") || !m.CanHandleScreen("spark-matches") || m.CanHandleScreen("other") {
		t.Fatalf("unexpected CanHandleScreen behavior")
	}
	if len(m.Screens()) != 2 {
		t.Fatalf("expected 2 screen summaries, got %d", len(m.Screens()))
	}
	if len(m.Matches()) != 2 {
		t.Fatalf("expected 2 pre-seeded matches, got %d", len(m.Matches()))
	}

	_, err := m.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "missing"})
	if !errors.Is(err, domain.ErrScreenNotFound) {
		t.Fatalf("expected ErrScreenNotFound, got %v", err)
	}
}

func TestRenderSwipeDeckAndFilters(t *testing.T) {
	t.Parallel()
	m := spark.New()

	page, err := m.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "spark-deck"})
	if err != nil {
		t.Fatalf("RenderScreen(spark-deck) error: %v", err)
	}
	assertStringOnlyTrackingProps(t, page)

	var foundHeader, foundChips, foundDeck bool
	for _, env := range page.Snippets {
		switch env.SnippetType {
		case snippets.SnippetTypeHeader:
			foundHeader = true
			hdr, err := snippets.DecodeSnippetData[snippets.HeaderSnippetData](env)
			if err != nil {
				t.Fatalf("decode header: %v", err)
			}
			if hdr.TrailingAction == nil || !strings.Contains(hdr.TrailingAction.Label, "Matches (2)") {
				t.Fatalf("expected Matches (2) button, got %+v", hdr.TrailingAction)
			}
			nav, err := actions.DecodePayload[actions.NavigatePayload](*hdr.TrailingAction.OnClick)
			if err != nil || nav.Route != "spark-matches" {
				t.Fatalf("expected navigate to spark-matches, got %+v (err=%v)", nav, err)
			}

		case snippets.SnippetTypeChipGroup:
			foundChips = true
			cg, err := snippets.DecodeSnippetData[snippets.ChipGroupSnippetData](env)
			if err != nil {
				t.Fatalf("decode chip group: %v", err)
			}
			if len(cg.Chips) != 4 {
				t.Fatalf("expected 4 chips, got %d", len(cg.Chips))
			}
			resetChip := cg.Chips[3]
			if resetChip.Label != "Reset Deck" || resetChip.OnClick == nil {
				t.Fatalf("expected Reset Deck chip with action, got %+v", resetChip)
			}
			resetPayload, err := actions.DecodePayload[actions.CallAPIPayload](*resetChip.OnClick)
			if err != nil || resetPayload.Endpoint != "/api/v1/spark/swipe" || resetPayload.Body["direction"] != "reset" {
				t.Fatalf("unexpected reset chip payload: %+v (err=%v)", resetPayload, err)
			}

		case snippets.SnippetTypeSwipeDeck:
			foundDeck = true
			deck, err := snippets.DecodeSnippetData[snippets.SwipeDeckSnippetData](env)
			if err != nil {
				t.Fatalf("decode swipe deck: %v", err)
			}
			if len(deck.Cards) != 6 {
				t.Fatalf("expected 6 cards in deck, got %d", len(deck.Cards))
			}
			if deck.OnUndo == nil {
				t.Fatalf("expected deck OnUndo action")
			}
			undoPayload, err := actions.DecodePayload[actions.CallAPIPayload](*deck.OnUndo)
			if err != nil || undoPayload.Endpoint != "/api/v1/spark/swipe" || undoPayload.Body["direction"] != "undo" {
				t.Fatalf("unexpected deck OnUndo payload: %+v (err=%v)", undoPayload, err)
			}
			card := deck.Cards[0]
			assertSwipeAction(t, card.OnLike, card.CardID, "like")
			assertSwipeAction(t, card.OnNope, card.CardID, "nope")
			assertSwipeAction(t, card.OnSuperLike, card.CardID, "super_like")
		}
	}
	if !foundHeader || !foundChips || !foundDeck {
		t.Fatalf("missing deck snippets: header=%v chips=%v deck=%v", foundHeader, foundChips, foundDeck)
	}

	// Verify filter slices.
	verifiedPage, err := m.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "spark-deck", Filter: "verified"})
	if err != nil {
		t.Fatalf("RenderScreen(verified) error: %v", err)
	}
	if got := len(findSwipeDeck(t, verifiedPage).Cards); got != 4 {
		t.Fatalf("expected 4 verified profiles, got %d", got)
	}

	nearbyPage, err := m.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "spark-deck", Filter: "nearby"})
	if err != nil {
		t.Fatalf("RenderScreen(nearby) error: %v", err)
	}
	if got := len(findSwipeDeck(t, nearbyPage).Cards); got != 4 {
		t.Fatalf("expected 4 nearby profiles, got %d", got)
	}

	dtPage, err := m.RenderScreen(context.Background(), domain.ScreenRequest{ScreenID: "spark-deck", Filter: "design_tech"})
	if err != nil {
		t.Fatalf("RenderScreen(design_tech) error: %v", err)
	}
	if got := len(findSwipeDeck(t, dtPage).Cards); got != 4 {
		t.Fatalf("expected 4 design_tech profiles, got %d", got)
	}
}

func TestSwipeLifecycleLikeSuperLikeNopeUndoReset(t *testing.T) {
	t.Parallel()
	m := spark.New()
	ctx := context.Background()

	// 1. Mutual like triggers match celebration & persists in spark-matches.
	res, err := m.RecordSwipe(ctx, domain.SwipeRequest{ProfileID: "spk_aanya", Direction: "like"})
	if err != nil {
		t.Fatalf("RecordSwipe(like mutual) error: %v", err)
	}
	assertStringOnlyTrackingProps(t, res)
	if res.Data["matched"] != true {
		t.Fatalf("expected matched=true for mutual profile, got %+v", res.Data)
	}
	if len(res.FollowUpActions) != 2 {
		t.Fatalf("expected celebration + toast follow-ups, got %d", len(res.FollowUpActions))
	}
	fb, err := actions.DecodePayload[actions.TriggerFeedbackPayload](res.FollowUpActions[0])
	if err != nil || fb.Effect != actions.FeedbackCelebration || fb.Title != "It's a Match!" {
		t.Fatalf("unexpected celebration payload: %+v (err=%v)", fb, err)
	}
	if !strings.Contains(fb.Subtitle, "Aanya Sharma") {
		t.Fatalf("expected subtitle naming Aanya Sharma, got %q", fb.Subtitle)
	}
	if len(m.Matches()) != 3 {
		t.Fatalf("expected 3 matches after mutual like, got %d", len(m.Matches()))
	}

	// Render spark-matches and confirm new match appears as action_row.
	matchesPage, err := m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "spark-matches"})
	if err != nil {
		t.Fatalf("RenderScreen(spark-matches) error: %v", err)
	}
	assertStringOnlyTrackingProps(t, matchesPage)
	rowCount := 0
	foundAanya := false
	for _, env := range matchesPage.Snippets {
		if env.SnippetType == snippets.SnippetTypeActionRow {
			rowCount++
			row, decErr := snippets.DecodeSnippetData[snippets.ActionRowSnippetData](env)
			if decErr != nil {
				t.Fatalf("decode match row: %v", decErr)
			}
			if row.Title.Text == "Aanya Sharma" {
				foundAanya = true
			}
			if row.OnClick == nil || row.OnClick.Type != actions.ActionShowToast {
				t.Fatalf("expected icebreaker toast on match row, got %+v", row.OnClick)
			}
		}
	}
	if rowCount != 3 || !foundAanya {
		t.Fatalf("expected 3 match rows including Aanya Sharma, got count=%d found=%v", rowCount, foundAanya)
	}

	// 2. Non-mutual normal like gives subtle toast without match.
	likeRes, err := m.RecordSwipe(ctx, domain.SwipeRequest{ProfileID: "spk_meera", Direction: "like"})
	if err != nil {
		t.Fatalf("RecordSwipe(like non-mutual) error: %v", err)
	}
	if likeRes.Data["matched"] != false {
		t.Fatalf("expected matched=false for non-mutual like, got %+v", likeRes.Data)
	}
	if len(m.Matches()) != 3 {
		t.Fatalf("expected match count unchanged at 3, got %d", len(m.Matches()))
	}

	// 3. Super-like on non-mutual profile guarantees a match.
	superRes, err := m.RecordSwipe(ctx, domain.SwipeRequest{ProfileID: "spk_dev", Direction: "super_like"})
	if err != nil {
		t.Fatalf("RecordSwipe(super_like) error: %v", err)
	}
	if superRes.Data["matched"] != true {
		t.Fatalf("expected matched=true for super_like, got %+v", superRes.Data)
	}
	if len(m.Matches()) != 4 {
		t.Fatalf("expected 4 matches after super_like, got %d", len(m.Matches()))
	}

	// 4. Nope marks profile swiped with subtle Passed toast.
	nopeRes, err := m.RecordSwipe(ctx, domain.SwipeRequest{ProfileID: "spk_kabir", Direction: "nope"})
	if err != nil {
		t.Fatalf("RecordSwipe(nope) error: %v", err)
	}
	if nopeRes.Message != "Passed" {
		t.Fatalf("expected Passed message, got %q", nopeRes.Message)
	}

	// Deck should now have 2 unswiped profiles (rohan, zara).
	deckPage, _ := m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "spark-deck"})
	if got := len(findSwipeDeck(t, deckPage).Cards); got != 2 {
		t.Fatalf("expected 2 remaining cards, got %d", got)
	}

	// 5. Undo rewinds last swipe (spk_kabir) back onto the deck.
	undoRes, err := m.RecordSwipe(ctx, domain.SwipeRequest{Direction: "undo"})
	if err != nil {
		t.Fatalf("RecordSwipe(undo) error: %v", err)
	}
	if undoRes.Data["profile_id"] != "spk_kabir" {
		t.Fatalf("expected undo to restore spk_kabir, got %+v", undoRes.Data)
	}
	deckAfterUndo, _ := m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "spark-deck"})
	if got := len(findSwipeDeck(t, deckAfterUndo).Cards); got != 3 {
		t.Fatalf("expected 3 cards after undo, got %d", got)
	}

	// 6. Reset restores all 6 profiles and emits toast + navigate("spark-deck").
	resetRes, err := m.RecordSwipe(ctx, domain.SwipeRequest{Direction: "reset"})
	if err != nil {
		t.Fatalf("RecordSwipe(reset) error: %v", err)
	}
	if len(resetRes.FollowUpActions) != 2 || resetRes.FollowUpActions[1].Type != actions.ActionNavigate {
		t.Fatalf("expected toast + navigate follow-ups on reset, got %+v", resetRes.FollowUpActions)
	}
	deckAfterReset, _ := m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "spark-deck"})
	if got := len(findSwipeDeck(t, deckAfterReset).Cards); got != 6 {
		t.Fatalf("expected all 6 cards after reset, got %d", got)
	}

	// Error cases.
	if _, err := m.RecordSwipe(ctx, domain.SwipeRequest{Direction: "invalid"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid direction, got %v", err)
	}
	if _, err := m.RecordSwipe(ctx, domain.SwipeRequest{Direction: "like"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty profile_id, got %v", err)
	}
	if _, err := m.RecordSwipe(ctx, domain.SwipeRequest{ProfileID: "ghost", Direction: "like"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown profile, got %v", err)
	}
}

func TestSparkConcurrentSafety(t *testing.T) {
	t.Parallel()
	m := spark.New()
	ctx := context.Background()

	var wg sync.WaitGroup
	directions := []domain.SwipeRequest{
		{ProfileID: "spk_aanya", Direction: "like"},
		{ProfileID: "spk_kabir", Direction: "super_like"},
		{ProfileID: "spk_meera", Direction: "nope"},
		{Direction: "undo"},
		{ProfileID: "spk_rohan", Direction: "like"},
		{Direction: "reset"},
	}

	for i := 0; i < 18; i++ {
		req := directions[i%len(directions)]
		wg.Add(2)
		go func(r domain.SwipeRequest) {
			defer wg.Done()
			_, _ = m.RecordSwipe(ctx, r)
		}(req)
		go func() {
			defer wg.Done()
			_, _ = m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "spark-deck"})
			_, _ = m.RenderScreen(ctx, domain.ScreenRequest{ScreenID: "spark-matches"})
			_ = m.Matches()
		}()
	}
	wg.Wait()
}

func assertSwipeAction(t *testing.T, trigger *actions.ActionTrigger, expectedProfile, expectedDir string) {
	t.Helper()
	if trigger == nil {
		t.Fatalf("expected non-nil %s trigger for %s", expectedDir, expectedProfile)
	}
	p, err := actions.DecodePayload[actions.CallAPIPayload](*trigger)
	if err != nil {
		t.Fatalf("decode %s payload: %v", expectedDir, err)
	}
	if p.Endpoint != "/api/v1/spark/swipe" || p.Method != "POST" {
		t.Fatalf("unexpected endpoint/method: %+v", p)
	}
	if p.Body["profile_id"] != expectedProfile || p.Body["direction"] != expectedDir {
		t.Fatalf("unexpected swipe body for %s/%s: %+v", expectedProfile, expectedDir, p.Body)
	}
}

func findSwipeDeck(t *testing.T, page snippets.PageResponse) snippets.SwipeDeckSnippetData {
	t.Helper()
	for _, env := range page.Snippets {
		if env.SnippetType == snippets.SnippetTypeSwipeDeck {
			data, err := snippets.DecodeSnippetData[snippets.SwipeDeckSnippetData](env)
			if err != nil {
				t.Fatalf("decode swipe deck: %v", err)
			}
			return data
		}
	}
	t.Fatalf("swipe_deck not found on page %q", page.PageID)
	return snippets.SwipeDeckSnippetData{}
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
