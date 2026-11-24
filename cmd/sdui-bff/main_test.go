package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/abeta-dev/go-libs/ginmw"
	"github.com/gin-gonic/gin"
)

func TestLoadConfig_DefaultsAndOverrides(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("GIN_MODE", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LAYOUT_ENGINE_URL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("RATE_LIMIT_RPS", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	def := loadConfig()
	if def.Port != "8085" || def.Addr != ":8085" || def.GinMode != gin.ReleaseMode {
		t.Fatalf("unexpected default config: %+v", def)
	}
	if def.LayoutEngineURL != "" || def.JWTSecret != "" || def.RateLimitRPS != 0 {
		t.Fatalf("unexpected optional defaults: %+v", def)
	}

	t.Setenv("PORT", "9095")
	t.Setenv("GIN_MODE", gin.TestMode)
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LAYOUT_ENGINE_URL", "http://localhost:8090")
	t.Setenv("JWT_SECRET", "cmd-test-secret")
	t.Setenv("RATE_LIMIT_RPS", "75.5")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://studio.abeta.dev, https://app.abeta.dev")
	t.Cleanup(func() { _ = ginmw.SetJWTSecret("") })

	cfg := loadConfig()
	if cfg.Port != "9095" || cfg.Addr != ":9095" || cfg.GinMode != gin.TestMode || cfg.LogLevel != "debug" {
		t.Fatalf("unexpected overridden config: %+v", cfg)
	}
	if cfg.LayoutEngineURL != "http://localhost:8090" || cfg.JWTSecret != "cmd-test-secret" || cfg.RateLimitRPS != 75.5 {
		t.Fatalf("unexpected integration/security config: %+v", cfg)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 CORS origins, got %+v", cfg.AllowedOrigins)
	}
}

func TestBuildServer_LiveRoundTrip(t *testing.T) {
	t.Setenv("GIN_MODE", gin.TestMode)
	cfg := loadConfig()
	cfg.GinMode = gin.TestMode

	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := buildServer(cfg, log)

	if srv.Addr != ":8085" || srv.Handler == nil || srv.Protocols == nil {
		t.Fatalf("unexpected server struct: %+v", srv)
	}
	if !srv.Protocols.HTTP1() || !srv.Protocols.HTTP2() || !srv.Protocols.UnencryptedHTTP2() {
		t.Fatalf("expected HTTP/1.1, HTTP/2, and h2c enabled, got %s", srv.Protocols.String())
	}
	if srv.ReadTimeout <= 0 || srv.WriteTimeout <= 0 || srv.IdleTimeout <= 0 {
		t.Fatalf("expected positive timeouts, got read=%v write=%v idle=%v", srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
	}

	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)

	// 1. /healthz
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"abeta-sdui-bff"`) {
		t.Fatalf("unexpected /healthz response: status=%d body=%s", resp.StatusCode, string(body))
	}

	// 2. /api/v1/apps returns all 7 verticals
	resp, err = http.Get(ts.URL + "/api/v1/apps")
	if err != nil {
		t.Fatalf("GET /api/v1/apps: %v", err)
	}
	var appsPayload struct {
		Success bool `json:"success"`
		Data    struct {
			Apps []map[string]any `json:"apps"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&appsPayload); err != nil {
		_ = resp.Body.Close()
		t.Fatalf("decode /api/v1/apps: %v", err)
	}
	_ = resp.Body.Close()
	if !appsPayload.Success || len(appsPayload.Data.Apps) != 7 {
		t.Fatalf("expected 7 wired verticals, got %+v", appsPayload)
	}

	// 3. /api/v1/screens/fresh-home
	resp, err = http.Get(ts.URL + "/api/v1/screens/fresh-home?city=Bengaluru")
	if err != nil {
		t.Fatalf("GET /api/v1/screens/fresh-home: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-SDUI-Source") != "standalone" {
		t.Fatalf("unexpected fresh-home response: status=%d source=%q", resp.StatusCode, resp.Header.Get("X-SDUI-Source"))
	}

	// 4. POST /api/v1/fresh/orders/express
	resp, err = http.Post(ts.URL+"/api/v1/fresh/orders/express", "application/json", strings.NewReader(`{"item_count":2}`))
	if err != nil {
		t.Fatalf("POST /api/v1/fresh/orders/express: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on express order, got %d", resp.StatusCode)
	}

	// 5. POST /api/v1/analytics/events & GET /api/v1/analytics/summary
	resp, err = http.Post(ts.URL+"/api/v1/analytics/events", "application/json", strings.NewReader(`{"session_id":"main_test","events":[{"event_name":"page_open","page_id":"fresh-home"},{"event_name":"interaction_click","page_id":"fresh-home","component_id":"tile_avocados"}]}`))
	if err != nil {
		t.Fatalf("POST /api/v1/analytics/events: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on analytics ingest, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/analytics/summary")
	if err != nil {
		t.Fatalf("GET /api/v1/analytics/summary: %v", err)
	}
	sumBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(sumBody), `"total_events":2`) {
		t.Fatalf("unexpected analytics summary response: status=%d body=%s", resp.StatusCode, string(sumBody))
	}
}

func TestBuildServer_H2C(t *testing.T) {
	cfg := loadConfig()
	cfg.GinMode = gin.TestMode
	srv := buildServer(cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	go func() {
		_ = srv.Serve(ln)
	}()

	clientProtocols := new(http.Protocols)
	clientProtocols.SetUnencryptedHTTP2(true)
	client := &http.Client{
		Transport: &http.Transport{Protocols: clientProtocols},
		Timeout:   3 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String()+"/api/v1/screens/spark-deck", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Accept-Encoding", "br")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("h2c request failed: %v", err)
	}
	_ = resp.Body.Close()

	if resp.Proto != "HTTP/2.0" || resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Encoding") != "br" {
		t.Fatalf("unexpected h2c response: proto=%s status=%d enc=%q", resp.Proto, resp.StatusCode, resp.Header.Get("Content-Encoding"))
	}
}

func TestRun_InvalidAddrAndGracefulShutdown(t *testing.T) {
	t.Setenv("GIN_MODE", gin.TestMode)
	t.Setenv("HTTP_ADDR", "127.0.0.1:99999")
	t.Setenv("SHUTDOWN_TIMEOUT", "1s")

	if err := run(); err == nil {
		t.Fatal("expected error when binding invalid port")
	}

	t.Setenv("HTTP_ADDR", "127.0.0.1:0")
	t.Setenv("LOG_LEVEL", "error")
	done := make(chan error, 1)
	go func() {
		done <- run()
	}()

	time.Sleep(150 * time.Millisecond)
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess: %v", err)
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Signal SIGTERM: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected clean shutdown, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for graceful shutdown")
	}
}
