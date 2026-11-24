package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/usecase"
	"github.com/abeta-dev/sdui-bff/internal/verticals/bites"
	"github.com/abeta-dev/sdui-bff/internal/verticals/fresh"
	"github.com/abeta-dev/sdui-bff/internal/verticals/mart"
	"github.com/abeta-dev/sdui-bff/internal/verticals/pay"
	"github.com/abeta-dev/sdui-bff/internal/verticals/pulse"
	"github.com/abeta-dev/sdui-bff/internal/verticals/spark"
	"github.com/abeta-dev/sdui-bff/internal/verticals/trade"
)

type stubLayoutClient struct {
	enabled      bool
	healthy      bool
	calls        int
	lastTenantID string
	lastSlug     string
	lastReq      domain.ScreenRequest
	fetchFn      func(ctx context.Context, tenantID, slug string, req domain.ScreenRequest) (snippets.PageResponse, bool, error)
}

func (s *stubLayoutClient) Enabled() bool { return s.enabled }

func (s *stubLayoutClient) HealthCheck(context.Context) bool { return s.healthy }

func (s *stubLayoutClient) FetchScreen(
	ctx context.Context,
	tenantID, screenSlug string,
	req domain.ScreenRequest,
) (snippets.PageResponse, bool, error) {
	s.calls++
	s.lastTenantID = tenantID
	s.lastSlug = screenSlug
	s.lastReq = req
	if s.fetchFn != nil {
		return s.fetchFn(ctx, tenantID, screenSlug, req)
	}
	return snippets.PageResponse{}, false, domain.ErrScreenNotFound
}

func allVerticals() []domain.VerticalModule {
	return []domain.VerticalModule{
		fresh.New(),
		bites.New(),
		pulse.New(),
		mart.New(),
		spark.New(),
		trade.New(),
		pay.New(),
	}
}

func TestScreenComposer_ListAppsAndScreens(t *testing.T) {
	t.Parallel()

	verticals := append([]domain.VerticalModule{nil}, allVerticals()...)
	composer := usecase.NewScreenComposer(nil, verticals...)

	apps := composer.ListApps()
	if len(apps) != 7 {
		t.Fatalf("expected 7 apps, got %d", len(apps))
	}

	expectedIDs := []string{
		"abeta-fresh",
		"abeta-bites",
		"abeta-pulse",
		"abeta-mart",
		"abeta-spark",
		"abeta-trade",
		"abeta-pay",
	}
	for i, want := range expectedIDs {
		if apps[i].ID != want {
			t.Errorf("app[%d].ID = %q, want %q", i, apps[i].ID, want)
		}
	}

	screens := composer.ListScreens()
	if len(screens) < 17 {
		t.Fatalf("expected at least 17 screens across 7 verticals, got %d", len(screens))
	}
}

func TestScreenComposer_StandaloneMode(t *testing.T) {
	t.Parallel()

	composer := usecase.NewScreenComposer(&stubLayoutClient{enabled: false}, allVerticals()...)

	page, source, err := composer.GetScreen(context.Background(), domain.ScreenRequest{
		ScreenID: "/fresh-home ",
		City:     "Bengaluru",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != usecase.SourceStandalone {
		t.Errorf("source = %q, want %q", source, usecase.SourceStandalone)
	}
	if page.PageID != "fresh-home" {
		t.Errorf("page.PageID = %q, want fresh-home", page.PageID)
	}
}

func TestScreenComposer_LayoutEngineHitAndHeaderForwarding(t *testing.T) {
	t.Parallel()

	cmsPage := snippets.NewPage("fresh-home", "CMS Override Fresh").Build()
	client := &stubLayoutClient{
		enabled: true,
		healthy: true,
		fetchFn: func(_ context.Context, _, _ string, _ domain.ScreenRequest) (snippets.PageResponse, bool, error) {
			return cmsPage, true, nil
		},
	}

	composer := usecase.NewScreenComposer(client, allVerticals()...)
	req := domain.ScreenRequest{
		ScreenID:    "fresh-home",
		City:        "Mumbai",
		Platform:    "ios",
		UserSegment: "pro_member",
		WindowClass: "expanded",
	}

	page, source, err := composer.GetScreen(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != usecase.SourceLayoutEngine {
		t.Errorf("source = %q, want %q", source, usecase.SourceLayoutEngine)
	}
	if page.Title != "CMS Override Fresh" {
		t.Errorf("page.Title = %q, want CMS Override Fresh", page.Title)
	}
	if client.calls != 1 {
		t.Fatalf("expected 1 Layout Engine call, got %d", client.calls)
	}
	if client.lastTenantID != "abeta-fresh" {
		t.Errorf("forwarded tenantID = %q, want abeta-fresh", client.lastTenantID)
	}
	if client.lastSlug != "fresh-home" {
		t.Errorf("forwarded slug = %q, want fresh-home", client.lastSlug)
	}
	if client.lastReq.City != "Mumbai" || client.lastReq.Platform != "ios" ||
		client.lastReq.UserSegment != "pro_member" || client.lastReq.WindowClass != "expanded" {
		t.Errorf("targeting headers not forwarded faithfully: %+v", client.lastReq)
	}

	// Explicit TenantID override should take precedence over vertical default.
	req.TenantID = "custom-tenant"
	_, _, err = composer.GetScreen(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on tenant override: %v", err)
	}
	if client.lastTenantID != "custom-tenant" {
		t.Errorf("expected explicit tenant override custom-tenant, got %q", client.lastTenantID)
	}
}

func TestScreenComposer_LayoutEngineFallbackOn404AndError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		fetchFn func(context.Context, string, string, domain.ScreenRequest) (snippets.PageResponse, bool, error)
	}{
		{
			name: "404 not found",
			fetchFn: func(context.Context, string, string, domain.ScreenRequest) (snippets.PageResponse, bool, error) {
				return snippets.PageResponse{}, false, domain.ErrScreenNotFound
			},
		},
		{
			name: "network error",
			fetchFn: func(context.Context, string, string, domain.ScreenRequest) (snippets.PageResponse, bool, error) {
				return snippets.PageResponse{}, false, errors.New("connection refused")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &stubLayoutClient{enabled: true, fetchFn: tc.fetchFn}
			composer := usecase.NewScreenComposer(client, allVerticals()...)

			page, source, err := composer.GetScreen(context.Background(), domain.ScreenRequest{
				ScreenID: "bites-home",
			})
			if err != nil {
				t.Fatalf("expected graceful fallback, got error: %v", err)
			}
			if source != usecase.SourceStandaloneFallback {
				t.Errorf("source = %q, want %q", source, usecase.SourceStandaloneFallback)
			}
			if page.PageID != "bites-home" {
				t.Errorf("page.PageID = %q, want bites-home", page.PageID)
			}
			if client.calls != 1 {
				t.Errorf("expected 1 Layout Engine attempt, got %d", client.calls)
			}
		})
	}
}

func TestScreenComposer_DynamicParamsBypassLayoutEngine(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		req  domain.ScreenRequest
	}{
		{name: "search query", req: domain.ScreenRequest{ScreenID: "mart-home", Query: "audio"}},
		{name: "category filter", req: domain.ScreenRequest{ScreenID: "pulse-home", Filter: "concerts"}},
		{name: "detail entity id", req: domain.ScreenRequest{ScreenID: "bites-restaurant", Extra: map[string]string{"id": "verde-bowl"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &stubLayoutClient{
				enabled: true,
				fetchFn: func(context.Context, string, string, domain.ScreenRequest) (snippets.PageResponse, bool, error) {
					return snippets.NewPage("override", "Should Not Call").Build(), true, nil
				},
			}
			composer := usecase.NewScreenComposer(client, allVerticals()...)

			page, source, err := composer.GetScreen(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if source != usecase.SourceStandalone {
				t.Errorf("source = %q, want %q", source, usecase.SourceStandalone)
			}
			if client.calls != 0 {
				t.Errorf("expected Layout Engine to be bypassed, got %d calls", client.calls)
			}
			if page.PageID != tc.req.ScreenID {
				t.Errorf("page.PageID = %q, want %q", page.PageID, tc.req.ScreenID)
			}
		})
	}
}

func TestScreenComposer_UnknownScreenProxyAnd404(t *testing.T) {
	t.Parallel()

	t.Run("proxies kit screen via Layout Engine", func(t *testing.T) {
		t.Parallel()
		client := &stubLayoutClient{
			enabled: true,
			fetchFn: func(_ context.Context, tenantID, slug string, _ domain.ScreenRequest) (snippets.PageResponse, bool, error) {
				if slug == "watch-status" {
					return snippets.NewPage("watch-status", "Watch Status").Build(), true, nil
				}
				return snippets.PageResponse{}, false, domain.ErrScreenNotFound
			},
		}
		composer := usecase.NewScreenComposer(client, allVerticals()...)

		page, source, err := composer.GetScreen(context.Background(), domain.ScreenRequest{ScreenID: "watch-status"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if source != usecase.SourceLayoutEngine || page.PageID != "watch-status" {
			t.Fatalf("unexpected result: source=%q page=%+v", source, page)
		}
		if client.lastTenantID != "abeta-fresh" {
			t.Errorf("expected default proxy tenant abeta-fresh, got %q", client.lastTenantID)
		}

		_, _, err = composer.GetScreen(context.Background(), domain.ScreenRequest{ScreenID: "non-existent"})
		if !errors.Is(err, domain.ErrScreenNotFound) {
			t.Fatalf("expected ErrScreenNotFound, got %v", err)
		}
	})

	t.Run("returns ErrScreenNotFound when standalone or empty", func(t *testing.T) {
		t.Parallel()
		composer := usecase.NewScreenComposer(nil, allVerticals()...)

		if _, _, err := composer.GetScreen(context.Background(), domain.ScreenRequest{ScreenID: "unknown-screen"}); !errors.Is(err, domain.ErrScreenNotFound) {
			t.Fatalf("expected ErrScreenNotFound for unknown screen, got %v", err)
		}
		if _, _, err := composer.GetScreen(context.Background(), domain.ScreenRequest{ScreenID: "   "}); !errors.Is(err, domain.ErrScreenNotFound) {
			t.Fatalf("expected ErrScreenNotFound for empty screen id, got %v", err)
		}
	})

	t.Run("respects cancelled context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		composer := usecase.NewScreenComposer(nil, allVerticals()...)
		if _, _, err := composer.GetScreen(ctx, domain.ScreenRequest{ScreenID: "fresh-home"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

func TestScreenComposer_EnsureComponentIDsAcrossAllVerticalsAndLayoutEngine(t *testing.T) {
	t.Parallel()

	composer := usecase.NewScreenComposer(nil, allVerticals()...)
	screens := composer.ListScreens()
	if len(screens) < 17 {
		t.Fatalf("expected >= 17 screens across 7 verticals, got %d", len(screens))
	}

	for _, sc := range screens {
		page, source, err := composer.GetScreen(context.Background(), domain.ScreenRequest{
			ScreenID: sc.ScreenID,
			City:     "Bengaluru",
		})
		if err != nil {
			t.Fatalf("GetScreen(%s) error: %v", sc.ScreenID, err)
		}
		if source != usecase.SourceStandalone {
			t.Errorf("GetScreen(%s) source = %q, want standalone", sc.ScreenID, source)
		}
		for i, env := range page.Snippets {
			if strings.TrimSpace(env.SnippetID) == "" {
				t.Errorf("%s snippet[%d] (%s) missing snippet_id", sc.ScreenID, i, env.SnippetType)
			}
			if env.SnippetType == snippets.SnippetTypeComposedBlock {
				cb, decodeErr := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](env)
				if decodeErr != nil {
					t.Fatalf("%s decode composed_block %s: %v", sc.ScreenID, env.SnippetID, decodeErr)
				}
				assertAllNodeIDsNonEmpty(t, sc.ScreenID, cb.Root)
			}
		}

		// Verify idempotence: calling EnsureComponentIDs again yields byte-identical JSON.
		raw1, _ := json.Marshal(page)
		snippets.EnsureComponentIDs(&page)
		raw2, _ := json.Marshal(page)
		if string(raw1) != string(raw2) {
			t.Errorf("EnsureComponentIDs not already normalized on %s", sc.ScreenID)
		}
	}

	// Layout Engine screens with omitted snippet_id, node_id, button id, and action id
	// must also be deterministically normalized by DefaultScreenComposer.GetScreen.
	anonAct := actions.NewShowToastAction("", actions.ShowToastPayload{Message: "hi"})
	anonBtn := atoms.ButtonAtom{Label: "Go", OnClick: &anonAct}
	cmsRaw := snippets.NewPage("cms-promo", "CMS Promo").
		AddSnippets(
			snippets.NewComposedBlockSnippet("", "promo_blk", snippets.VStack(8, snippets.ButtonNode(anonBtn))).Build(),
		).
		Build()

	leComposer := usecase.NewScreenComposer(&stubLayoutClient{
		enabled: true,
		healthy: true,
		fetchFn: func(context.Context, string, string, domain.ScreenRequest) (snippets.PageResponse, bool, error) {
			return cmsRaw, true, nil
		},
	}, allVerticals()...)

	normalized, source, err := leComposer.GetScreen(context.Background(), domain.ScreenRequest{ScreenID: "cms-promo"})
	if err != nil {
		t.Fatalf("GetScreen(cms-promo) error: %v", err)
	}
	if source != usecase.SourceLayoutEngine {
		t.Fatalf("source = %q, want layout-engine", source)
	}
	if len(normalized.Snippets) != 1 || strings.TrimSpace(normalized.Snippets[0].SnippetID) == "" {
		t.Fatalf("expected deterministic snippet_id on layout-engine screen, got %+v", normalized.Snippets)
	}
	cb, err := snippets.DecodeSnippetData[snippets.ComposedBlockSnippetData](normalized.Snippets[0])
	if err != nil {
		t.Fatalf("decode normalized composed_block: %v", err)
	}
	assertAllNodeIDsNonEmpty(t, "cms-promo", cb.Root)
	btnNode := cb.Root.Children[0]
	if btnNode.Button == nil || strings.TrimSpace(btnNode.Button.ID) == "" || btnNode.Button.OnClick == nil || strings.TrimSpace(btnNode.Button.OnClick.ID) == "" {
		t.Fatalf("expected deterministic button & action IDs, got %+v", btnNode.Button)
	}
}

func assertAllNodeIDsNonEmpty(t *testing.T, screenID string, node snippets.ComposedNode) {
	t.Helper()
	if strings.TrimSpace(node.NodeID) == "" {
		t.Errorf("%s composed node kind=%q has empty node_id", screenID, node.Kind)
	}
	for _, ch := range node.Children {
		assertAllNodeIDsNonEmpty(t, screenID, ch)
	}
}

func TestDefaultActionExecutor(t *testing.T) {
	t.Parallel()

	exec := usecase.NewActionExecutor()
	ctx := context.Background()

	t.Run("track_event coerces all properties to strings", func(t *testing.T) {
		t.Parallel()
		res, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionID:   "trk_1",
			ActionType: "track_event",
			Payload: map[string]any{
				"event_name": "cart_opened",
				"properties": map[string]any{
					"count":    3,
					"express":  true,
					"total":    19.99,
					"vertical": "abeta-fresh",
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "executed" {
			t.Fatalf("status = %q, want executed", res.Status)
		}
		props, ok := res.Data["properties"].(map[string]any)
		if !ok {
			t.Fatalf("expected properties map, got %T", res.Data["properties"])
		}
		for k, v := range props {
			if _, isStr := v.(string); !isStr {
				t.Errorf("property %q has non-string type %T (%v)", k, v, v)
			}
		}
		if props["count"] != "3" || props["express"] != "true" || props["total"] != "19.99" {
			t.Errorf("unexpected stringified properties: %+v", props)
		}
	})

	t.Run("show_toast emits show_toast follow-up", func(t *testing.T) {
		t.Parallel()
		res, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionID:   "toast_1",
			ActionType: "show_toast",
			Payload:    map[string]any{"message": "Saved!", "variant": "success", "duration_ms": float64(1800)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "executed" || len(res.FollowUpActions) != 1 || res.FollowUpActions[0].Type != actions.ActionShowToast {
			t.Fatalf("unexpected show_toast result: %+v", res)
		}
	})

	t.Run("navigate emits navigate follow-up and validates route", func(t *testing.T) {
		t.Parallel()
		res, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionID:   "nav_1",
			ActionType: "navigate",
			Payload: map[string]any{
				"route":  "mart-product-detail",
				"params": map[string]any{"id": "mrt_anc_buds", "rank": 1},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.FollowUpActions) != 1 || res.FollowUpActions[0].Type != actions.ActionNavigate {
			t.Fatalf("unexpected navigate follow-ups: %+v", res.FollowUpActions)
		}

		if _, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionType: "navigate",
			Payload:    map[string]any{},
		}); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput for empty route, got %v", err)
		}
	})

	t.Run("submit_form emits celebration toast and string-only tracking", func(t *testing.T) {
		t.Parallel()
		res, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionID:   "frm_1",
			ActionType: "submit_form",
			Payload:    map[string]any{"form_id": "rfq_intake", "company": "Northstar"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "executed" || len(res.FollowUpActions) != 3 {
			t.Fatalf("expected 3 follow-up actions, got %+v", res)
		}
		trackPayload, decodeErr := actions.DecodePayload[actions.TrackEventPayload](res.FollowUpActions[2])
		if decodeErr != nil {
			t.Fatalf("decode track payload: %v", decodeErr)
		}
		for k, v := range trackPayload.Properties {
			if _, isStr := v.(string); !isStr {
				t.Errorf("submit_form track property %q has non-string type %T", k, v)
			}
		}
	})

	t.Run("default action type emits string-only tracking", func(t *testing.T) {
		t.Parallel()
		res, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionType: "refresh_page",
			Payload:    map[string]any{"screen_id": "fresh-home"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "executed" || len(res.FollowUpActions) != 1 {
			t.Fatalf("unexpected default action result: %+v", res)
		}
		trackPayload, decodeErr := actions.DecodePayload[actions.TrackEventPayload](res.FollowUpActions[0])
		if decodeErr != nil {
			t.Fatalf("decode default track payload: %v", decodeErr)
		}
		for k, v := range trackPayload.Properties {
			if _, isStr := v.(string); !isStr {
				t.Errorf("default track property %q has non-string type %T", k, v)
			}
		}
	})

	t.Run("validation and cancelled context", func(t *testing.T) {
		t.Parallel()
		if _, err := exec.Execute(ctx, domain.ActionExecuteRequest{}); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput for empty action_type, got %v", err)
		}
		cctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := exec.Execute(cctx, domain.ActionExecuteRequest{ActionType: "track_event"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("fallback defaults and typed payload coercions", func(t *testing.T) {
		t.Parallel()

		// track_event with fallback event_name and map[string]string + scalar types
		res1, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionType: "track_event",
			Payload: map[string]any{
				"event":      "alt_event",
				"properties": map[string]string{"k": "v"},
			},
		})
		if err != nil || res1.Data["event_name"] != "alt_event" {
			t.Fatalf("unexpected alt track_event result: %+v, err=%v", res1, err)
		}

		resDefaultEvent, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionType: "track_event",
			Payload: map[string]any{
				"properties": map[string]any{
					"nilVal":   nil,
					"i64":      int64(42),
					"sliceVal": []int{1, 2},
				},
			},
		})
		if err != nil || resDefaultEvent.Data["event_name"] != "custom_event" {
			t.Fatalf("unexpected default track_event result: %+v, err=%v", resDefaultEvent, err)
		}

		// show_toast with defaults & string/int duration_ms
		resToast, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionType: "show_toast",
			Payload:    map[string]any{"duration_ms": "1500"},
		})
		if err != nil || resToast.Message != "Action completed" {
			t.Fatalf("unexpected default show_toast result: %+v, err=%v", resToast, err)
		}
		resToastNeg, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionType: "show_toast",
			Payload:    map[string]any{"duration_ms": -10},
		})
		if err != nil || len(resToastNeg.FollowUpActions) != 1 {
			t.Fatalf("unexpected negative-duration show_toast result: %+v, err=%v", resToastNeg, err)
		}

		// navigate with map[string]string params
		resNav, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionType: "navigate",
			Payload: map[string]any{
				"route":            "fresh-home",
				"params":           map[string]string{"city": "Pune"},
				"clear_back_stack": true,
			},
		})
		if err != nil || resNav.Status != "executed" {
			t.Fatalf("unexpected navigate result: %+v, err=%v", resNav, err)
		}

		// submit_form with empty payload uses default form_id
		resForm, err := exec.Execute(ctx, domain.ActionExecuteRequest{
			ActionType: "submit_form",
		})
		if err != nil || resForm.Data["form_id"] != "form" {
			t.Fatalf("unexpected default submit_form result: %+v, err=%v", resForm, err)
		}
	})
}
