package handler_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abeta-dev/go-libs/ginmw"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	sduibff "github.com/abeta-dev/sdui-bff"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/handler"
	"github.com/abeta-dev/sdui-bff/internal/infra/layoutclient"
	"github.com/abeta-dev/sdui-bff/internal/usecase"
	"github.com/abeta-dev/sdui-bff/internal/verticals/bites"
	"github.com/abeta-dev/sdui-bff/internal/verticals/fresh"
	"github.com/abeta-dev/sdui-bff/internal/verticals/mart"
	"github.com/abeta-dev/sdui-bff/internal/verticals/pulse"
	"github.com/abeta-dev/sdui-bff/internal/verticals/spark"
	"github.com/abeta-dev/sdui-bff/internal/verticals/trade"
	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestRouter(layoutURL string, jwtSecret string, rateLimitRPS float64) (*gin.Engine, handler.Dependencies) {
	f := fresh.New()
	b := bites.New()
	p := pulse.New()
	m := mart.New()
	s := spark.New()
	tr := trade.New()

	lc := layoutclient.NewClient(layoutURL, nil)
	deps := handler.Dependencies{
		Composer:     usecase.NewScreenComposer(lc, f, b, p, m, s, tr),
		Actions:      usecase.NewActionExecutor(),
		LayoutClient: lc,
		Fresh:        f,
		Bites:        b,
		Pulse:        p,
		Mart:         m,
		Spark:        s,
		Trade:        tr,
		JWTSecret:    jwtSecret,
		RateLimitRPS: rateLimitRPS,
		Logger:       slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	return handler.NewRouter(deps), deps
}

func doJSONRequest(r http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestOpsAndDocsEndpoints(t *testing.T) {
	t.Parallel()
	router, _ := newTestRouter("", "", 0)

	t.Run("healthz standalone", func(t *testing.T) {
		w := doJSONRequest(router, http.MethodGet, "/healthz", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal healthz: %v", err)
		}
		if body["status"] != "ok" || body["service"] != "abeta-sdui-bff" || body["mode"] != "standalone" {
			t.Fatalf("unexpected healthz payload: %+v", body)
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Error("expected X-Request-ID header from ginmw.RequestID")
		}
	})

	t.Run("readyz standalone stays 200", func(t *testing.T) {
		w := doJSONRequest(router, http.MethodGet, "/readyz", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal readyz: %v", err)
		}
		if connected, ok := body["layout_engine_connected"].(bool); !ok || connected {
			t.Fatalf("expected layout_engine_connected=false in standalone, got %+v", body)
		}
	})

	t.Run("openapi.json and Scalar docs", func(t *testing.T) {
		wSpec := doJSONRequest(router, http.MethodGet, "/openapi.json", "", nil)
		if wSpec.Code != http.StatusOK {
			t.Fatalf("openapi status = %d, want 200", wSpec.Code)
		}
		var spec map[string]any
		if err := json.Unmarshal(wSpec.Body.Bytes(), &spec); err != nil {
			t.Fatalf("unmarshal openapi: %v", err)
		}
		if spec["openapi"] != "3.1.0" {
			t.Fatalf("openapi version = %v, want 3.1.0", spec["openapi"])
		}
		paths, _ := spec["paths"].(map[string]any)
		for _, p := range []string{
			"/llms.txt",
			"/llms-full.txt",
			"/api/v1/apps",
			"/api/v1/screens",
			"/api/v1/screens/{screenId}",
			"/api/v1/actions/execute",
			"/api/v1/orders/express",
			"/api/v1/fresh/orders/express",
			"/api/v1/bites/orders",
			"/api/v1/pulse/bookings",
			"/api/v1/mart/orders",
			"/api/v1/spark/swipe",
			"/api/v1/trade/rfq",
		} {
			if _, exists := paths[p]; !exists {
				t.Errorf("expected path %q in OpenAPI spec", p)
			}
		}

		wDocs := doJSONRequest(router, http.MethodGet, "/docs", "", nil)
		if wDocs.Code != http.StatusOK {
			t.Fatalf("docs status = %d, want 200", wDocs.Code)
		}
		if !strings.Contains(wDocs.Body.String(), `data-url="/openapi.json"`) ||
			!strings.Contains(wDocs.Body.String(), "@scalar/api-reference") {
			t.Fatalf("docs HTML missing Scalar reference: %s", wDocs.Body.String())
		}
	})

	t.Run("llms.txt and llms-full.txt plain-text endpoints", func(t *testing.T) {
		wIndex := doJSONRequest(router, http.MethodGet, "/llms.txt", "", nil)
		if wIndex.Code != http.StatusOK {
			t.Fatalf("GET /llms.txt status = %d, want 200", wIndex.Code)
		}
		if ct := wIndex.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Fatalf("GET /llms.txt Content-Type = %q, want text/plain", ct)
		}
		if !bytes.Equal(wIndex.Body.Bytes(), sduibff.LLMsTxt) {
			t.Fatal("GET /llms.txt body does not match embedded sduibff.LLMsTxt")
		}
		if !strings.Contains(wIndex.Body.String(), "# Abeta SDUI Domain BFF") {
			t.Fatalf("GET /llms.txt missing title header")
		}

		wFull := doJSONRequest(router, http.MethodGet, "/llms-full.txt", "", nil)
		if wFull.Code != http.StatusOK {
			t.Fatalf("GET /llms-full.txt status = %d, want 200", wFull.Code)
		}
		if ct := wFull.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Fatalf("GET /llms-full.txt Content-Type = %q, want text/plain", ct)
		}
		if !bytes.Equal(wFull.Body.Bytes(), sduibff.LLMsFullTxt) {
			t.Fatal("GET /llms-full.txt body does not match embedded sduibff.LLMsFullTxt")
		}
		if !strings.Contains(wFull.Body.String(), "X-SDUI-Source") || !strings.Contains(wFull.Body.String(), "Abeta Fresh") {
			t.Fatalf("GET /llms-full.txt missing expected architecture/vertical reference")
		}
	})
}

func TestDiscoveryAndAllSixVerticalScreens(t *testing.T) {
	t.Parallel()
	router, _ := newTestRouter("", "", 0)

	wApps := doJSONRequest(router, http.MethodGet, "/api/v1/apps", "", nil)
	if wApps.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/apps status = %d", wApps.Code)
	}
	var appsEnv struct {
		Success bool `json:"success"`
		Data    struct {
			Apps []domain.AppDescriptor `json:"apps"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wApps.Body.Bytes(), &appsEnv); err != nil {
		t.Fatalf("unmarshal apps: %v", err)
	}
	if !appsEnv.Success || len(appsEnv.Data.Apps) != 6 {
		t.Fatalf("expected success with 6 apps, got %+v", appsEnv)
	}

	wScreens := doJSONRequest(router, http.MethodGet, "/api/v1/screens", "", nil)
	if wScreens.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/screens status = %d", wScreens.Code)
	}
	var screensEnv struct {
		Success bool `json:"success"`
		Data    struct {
			Screens []domain.ScreenSummary `json:"screens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wScreens.Body.Bytes(), &screensEnv); err != nil {
		t.Fatalf("unmarshal screens: %v", err)
	}
	if !screensEnv.Success || len(screensEnv.Data.Screens) < 12 {
		t.Fatalf("expected >=12 screens, got %d", len(screensEnv.Data.Screens))
	}

	screenTests := []struct {
		path   string
		pageID string
	}{
		{"/api/v1/screens/fresh-home?city=Bengaluru", "fresh-home"},
		{"/api/v1/screens/store-home", "store-home"},
		{"/api/v1/screens/bites-home?filter=veg", "bites-home"},
		{"/api/v1/screens/bites-restaurant?id=verde-bowl", "bites-restaurant"},
		{"/api/v1/screens/bites-order-track", "bites-order-track"},
		{"/api/v1/screens/pulse-home?filter=concerts", "pulse-home"},
		{"/api/v1/screens/pulse-event-detail?id=evt-synth-symphony", "pulse-event-detail"},
		{"/api/v1/screens/mart-home?q=audio", "mart-home"},
		{"/api/v1/screens/mart-product-detail?id=mrt_anc_buds", "mart-product-detail"},
		{"/api/v1/screens/spark-deck", "spark-deck"},
		{"/api/v1/screens/spark-matches", "spark-matches"},
		{"/api/v1/screens/trade-home?filter=verified", "trade-home"},
		{"/api/v1/screens/trade-supplier?id=trd_ecopack", "trade-supplier"},
	}

	for _, tc := range screenTests {
		t.Run(tc.path, func(t *testing.T) {
			w := doJSONRequest(router, http.MethodGet, tc.path, "", map[string]string{
				"X-Platform":     "jvm",
				"X-Window-Class": "expanded",
			})
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, body = %s", tc.path, w.Code, w.Body.String())
			}
			if w.Header().Get("X-SDUI-Source") != usecase.SourceStandalone {
				t.Errorf("X-SDUI-Source = %q, want %q", w.Header().Get("X-SDUI-Source"), usecase.SourceStandalone)
			}
			var env struct {
				Success bool                  `json:"success"`
				Data    snippets.PageResponse `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.path, err)
			}
			if !env.Success || env.Data.PageID != tc.pageID || len(env.Data.Snippets) == 0 {
				t.Fatalf("unexpected page for %s: page_id=%q snippets=%d", tc.path, env.Data.PageID, len(env.Data.Snippets))
			}
		})
	}

	w404 := doJSONRequest(router, http.MethodGet, "/api/v1/screens/does-not-exist", "", nil)
	if w404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown screen, got %d: %s", w404.Code, w404.Body.String())
	}
}

func TestAllVerticalMutationsAndSparkMatchPersistence(t *testing.T) {
	t.Parallel()
	router, deps := newTestRouter("", "", 0)

	assertOKAction := func(t *testing.T, path, body string) domain.ActionResult {
		t.Helper()
		w := doJSONRequest(router, http.MethodPost, path, body, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("POST %s status = %d, body = %s", path, w.Code, w.Body.String())
		}
		var env struct {
			Success bool                `json:"success"`
			Data    domain.ActionResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		if !env.Success {
			t.Fatalf("POST %s returned success=false", path)
		}
		return env.Data
	}

	// Actions execute (direct & KMP trigger format)
	actRes := assertOKAction(t, "/api/v1/actions/execute", `{"action_id":"a1","action_type":"track_event","payload":{"event_name":"tap","properties":{"step":2}}}`)
	if actRes.Status != "executed" {
		t.Fatalf("expected status=executed, got %+v", actRes)
	}
	actTriggerRes := assertOKAction(t, "/api/v1/actions/execute", `{"trigger":{"id":"t1","type":"show_toast","payload":{"message":"Hi"}}}`)
	if actTriggerRes.Status != "executed" || len(actTriggerRes.FollowUpActions) != 1 {
		t.Fatalf("expected trigger bridge execution, got %+v", actTriggerRes)
	}
	if w := doJSONRequest(router, http.MethodPost, "/api/v1/actions/execute", `{}`, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty action_type, got %d", w.Code)
	}

	// Fresh express order + compat /api/v1/orders/express
	assertOKAction(t, "/api/v1/fresh/orders/express", `{"item_count":3,"total_usd":14.5,"address":"Indiranagar"}`)
	assertOKAction(t, "/api/v1/orders/express", ``)
	if deps.Fresh.OrderCount() != 2 {
		t.Fatalf("expected 2 Fresh orders across both routes, got %d", deps.Fresh.OrderCount())
	}

	// Bites food order
	bitesRes := assertOKAction(t, "/api/v1/bites/orders", `{"item_count":2,"address":"Koramangala"}`)
	if len(bitesRes.FollowUpActions) < 2 {
		t.Fatalf("expected Bites follow-up actions, got %+v", bitesRes.FollowUpActions)
	}

	// Pulse bookings (valid & invalid 400)
	assertOKAction(t, "/api/v1/pulse/bookings", `{"event_id":"evt-synth-symphony","seat_tier":"VIP","quantity":2,"city":"Bengaluru"}`)
	if w := doJSONRequest(router, http.MethodPost, "/api/v1/pulse/bookings", `{"quantity":-3}`, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 VALIDATION_ERROR for negative quantity, got %d: %s", w.Code, w.Body.String())
	}

	// Mart order
	assertOKAction(t, "/api/v1/mart/orders", `{"item_count":1,"total_usd":129.0}`)
	if deps.Mart.OrderCount() != 1 {
		t.Fatalf("expected 1 Mart order, got %d", deps.Mart.OrderCount())
	}

	// Spark swipe -> mutual match persists on spark-matches!
	swipeRes := assertOKAction(t, "/api/v1/spark/swipe", `{"profile_id":"spk_aanya","direction":"like"}`)
	if matched, _ := swipeRes.Data["matched"].(bool); !matched {
		t.Fatalf("expected mutual match for spk_aanya, got %+v", swipeRes.Data)
	}
	wMatches := doJSONRequest(router, http.MethodGet, "/api/v1/screens/spark-matches", "", nil)
	if wMatches.Code != http.StatusOK || !strings.Contains(wMatches.Body.String(), "Aanya Sharma") {
		t.Fatalf("expected persisted match Aanya Sharma on spark-matches, got code=%d body=%s", wMatches.Code, wMatches.Body.String())
	}
	if w := doJSONRequest(router, http.MethodPost, "/api/v1/spark/swipe", `{"direction":"diagonal"}`, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid swipe direction, got %d", w.Code)
	}
	if w := doJSONRequest(router, http.MethodPost, "/api/v1/spark/swipe", `{"profile_id":"unknown","direction":"like"}`, nil); w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown spark profile, got %d", w.Code)
	}

	// Trade RFQ (valid & invalid 400)
	assertOKAction(t, "/api/v1/trade/rfq", `{"supplier_id":"trd_ecopack","product_name":"Kraft Cartons","quantity":500,"company_name":"Northstar Retail","contact_email":"buyer@northstar.example"}`)
	if w := doJSONRequest(router, http.MethodPost, "/api/v1/trade/rfq", `{"company_name":"","contact_email":"invalid","quantity":0}`, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 VALIDATION_ERROR for invalid RFQ, got %d: %s", w.Code, w.Body.String())
	}
}

func TestETag304AndBrotliGzipCompression(t *testing.T) {
	t.Parallel()
	router, _ := newTestRouter("", "", 0)

	// 1. ETag + 304 Not Modified
	w1 := doJSONRequest(router, http.MethodGet, "/api/v1/screens/fresh-home", "", nil)
	if w1.Code != http.StatusOK {
		t.Fatalf("initial status = %d", w1.Code)
	}
	etag := w1.Header().Get("ETag")
	if etag == "" || w1.Header().Get("Cache-Control") != "public, max-age=30" {
		t.Fatalf("missing ETag or Cache-Control: %v", w1.Header())
	}

	w2 := doJSONRequest(router, http.MethodGet, "/api/v1/screens/fresh-home", "", map[string]string{
		"If-None-Match": etag,
	})
	if w2.Code != http.StatusNotModified {
		t.Fatalf("expected 304 Not Modified, got %d", w2.Code)
	}
	if w2.Body.Len() != 0 {
		t.Fatalf("expected empty body on 304, got %d bytes", w2.Body.Len())
	}

	// 2. Brotli compression + weak ETag 304 round-trip
	wBr := doJSONRequest(router, http.MethodGet, "/api/v1/screens/mart-home", "", map[string]string{
		"Accept-Encoding": "gzip, br",
	})
	if wBr.Code != http.StatusOK {
		t.Fatalf("brotli status = %d", wBr.Code)
	}
	if wBr.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("expected Content-Encoding=br, got %q", wBr.Header().Get("Content-Encoding"))
	}
	if !strings.HasPrefix(wBr.Header().Get("ETag"), "W/") {
		t.Errorf("expected weak ETag on compressed response, got %q", wBr.Header().Get("ETag"))
	}
	decompressed, err := io.ReadAll(brotli.NewReader(bytes.NewReader(wBr.Body.Bytes())))
	if err != nil {
		t.Fatalf("brotli decode failed: %v", err)
	}
	if !strings.Contains(string(decompressed), `"mart-home"`) {
		t.Fatalf("decompressed payload missing mart-home: %s", string(decompressed))
	}

	wBr304 := doJSONRequest(router, http.MethodGet, "/api/v1/screens/mart-home", "", map[string]string{
		"Accept-Encoding": "br",
		"If-None-Match":   wBr.Header().Get("ETag"),
	})
	if wBr304.Code != http.StatusNotModified {
		t.Fatalf("expected 304 with weak ETag, got %d", wBr304.Code)
	}

	// 3. Gzip compression
	wGz := doJSONRequest(router, http.MethodGet, "/api/v1/apps", "", map[string]string{
		"Accept-Encoding": "gzip",
	})
	if wGz.Code != http.StatusOK || wGz.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip 200, got code=%d enc=%q", wGz.Code, wGz.Header().Get("Content-Encoding"))
	}
	gr, err := gzip.NewReader(bytes.NewReader(wGz.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	gzBytes, err := io.ReadAll(gr)
	_ = gr.Close()
	if err != nil || !strings.Contains(string(gzBytes), "abeta-fresh") {
		t.Fatalf("gzip payload check failed: err=%v", err)
	}
}

func TestLayoutEngineIntegratedMode(t *testing.T) {
	t.Parallel()

	var capturedHeaders http.Header
	mockLE := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/v1/screens/fresh-home":
			capturedHeaders = r.Header.Clone()
			page := snippets.NewPage("fresh-home", "Studio Dynamic Fresh").Build()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": page})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mockLE.Close)

	router, _ := newTestRouter(mockLE.URL, "", 0)

	// /healthz & /readyz reflect Layout Engine connectivity
	wHealth := doJSONRequest(router, http.MethodGet, "/healthz", "", nil)
	if !strings.Contains(wHealth.Body.String(), `"layout-engine"`) {
		t.Fatalf("expected mode=layout-engine, got %s", wHealth.Body.String())
	}
	wReady := doJSONRequest(router, http.MethodGet, "/readyz", "", nil)
	if !strings.Contains(wReady.Body.String(), `"layout_engine_connected":true`) {
		t.Fatalf("expected layout_engine_connected=true, got %s", wReady.Body.String())
	}

	// Screen served dynamically from mock Layout Engine
	wHit := doJSONRequest(router, http.MethodGet, "/api/v1/screens/fresh-home", "", map[string]string{
		"X-City":         "Mumbai",
		"X-Platform":     "ios",
		"X-User-Segment": "vip",
		"X-Window-Class": "compact",
	})
	if wHit.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", wHit.Code, wHit.Body.String())
	}
	if wHit.Header().Get("X-SDUI-Source") != usecase.SourceLayoutEngine {
		t.Fatalf("X-SDUI-Source = %q, want %q", wHit.Header().Get("X-SDUI-Source"), usecase.SourceLayoutEngine)
	}
	if !strings.Contains(wHit.Body.String(), "Studio Dynamic Fresh") {
		t.Fatalf("expected CMS screen title, got %s", wHit.Body.String())
	}
	if capturedHeaders.Get("X-Tenant-ID") != "abeta-fresh" || capturedHeaders.Get("X-City") != "Mumbai" || capturedHeaders.Get("X-Platform") != "ios" {
		t.Errorf("targeting headers mismatch: %v", capturedHeaders)
	}

	// Screen missing in Layout Engine falls back gracefully
	wFallback := doJSONRequest(router, http.MethodGet, "/api/v1/screens/bites-home", "", nil)
	if wFallback.Code != http.StatusOK || wFallback.Header().Get("X-SDUI-Source") != usecase.SourceStandaloneFallback {
		t.Fatalf("expected standalone-fallback 200, got code=%d source=%q", wFallback.Code, wFallback.Header().Get("X-SDUI-Source"))
	}
}

func TestOptionalAuthAndRateLimiting(t *testing.T) {
	// Not parallel because ginmw.SetJWTSecret updates package-level auth secret.
	secret := "test-sdui-bff-jwt-secret"
	t.Cleanup(func() { _ = ginmw.SetJWTSecret("") })

	router, _ := newTestRouter("", secret, 1)

	// Unauthenticated request to /api/v1/apps is rejected with 401
	wUnauth := doJSONRequest(router, http.MethodGet, "/api/v1/apps", "", nil)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer token, got %d", wUnauth.Code)
	}

	// Rate limiting triggers 429 after burst capacity of 1 is exhausted on /healthz
	wRL1 := doJSONRequest(router, http.MethodGet, "/healthz", "", nil)
	if wRL1.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after exceeding 1 RPS burst, got %d", wRL1.Code)
	}

	// Authenticated request with valid JWT succeeds on a non-throttled router
	authRouter, _ := newTestRouter("", secret, 0)
	token, err := ginmw.GenerateToken(&ginmw.Claims{UserID: uuid.New(), TenantID: uuid.New()}, 15)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	wAuth := doJSONRequest(authRouter, http.MethodGet, "/api/v1/apps", "", map[string]string{
		"Authorization": "Bearer " + token,
	})
	if wAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid JWT, got %d: %s", wAuth.Code, wAuth.Body.String())
	}
}
