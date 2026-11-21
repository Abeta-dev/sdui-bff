package layoutclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/atoms"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/theming"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/infra/layoutclient"
)

func TestClientStandaloneDisabledMode(t *testing.T) {
	t.Parallel()

	for _, baseURL := range []string{"", "   ", "\t\n"} {
		client := layoutclient.NewClient(baseURL, nil)
		if client.Enabled() {
			t.Fatalf("expected Enabled() == false for baseURL %q", baseURL)
		}

		page, found, err := client.FetchScreen(context.Background(), "abeta-fresh", "fresh-home", domain.ScreenRequest{})
		if err != nil {
			t.Fatalf("expected nil error in standalone mode, got %v", err)
		}
		if found {
			t.Fatalf("expected found == false in standalone mode")
		}
		if page.PageID != "" {
			t.Fatalf("expected empty PageResponse in standalone mode, got %+v", page)
		}
		if client.HealthCheck(context.Background()) {
			t.Fatalf("expected HealthCheck() == false in standalone mode")
		}
	}
}

func TestClientFetchScreenHappyPathAndHeaders(t *testing.T) {
	t.Parallel()

	var (
		gotPath        string
		gotAccept      string
		gotTenant      string
		gotPlatform    string
		gotCity        string
		gotUserSegment string
		gotWindowClass string
	)

	expectedPage := snippets.NewPage("fresh-home", "Abeta Fresh CMS").
		AddSnippet(
			snippets.NewHeaderSnippet(
				"cms_hdr",
				atoms.NewText("CMS Managed Fresh", theming.TypographyTitleLarge, theming.ColorTextPrimary),
			).Build(),
		).
		WithTracking("cms_screen_loaded", map[string]any{"source": "layout-engine"}).
		Build()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		gotPath = r.URL.Path
		gotAccept = r.Header.Get("Accept")
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotPlatform = r.Header.Get("X-Platform")
		gotCity = r.Header.Get("X-City")
		gotUserSegment = r.Header.Get("X-User-Segment")
		gotWindowClass = r.Header.Get("X-Window-Class")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    expectedPage,
		})
	}))
	defer srv.Close()

	client := layoutclient.NewClient(srv.URL+"/", srv.Client())
	if !client.Enabled() {
		t.Fatalf("expected Enabled() == true")
	}

	req := domain.ScreenRequest{
		ScreenID:    "fresh-home",
		City:        "Bengaluru",
		Platform:    "ios",
		UserSegment: "plus_member",
		WindowClass: "compact",
	}

	page, found, err := client.FetchScreen(context.Background(), "abeta-fresh", "fresh-home", req)
	if err != nil {
		t.Fatalf("FetchScreen returned unexpected error: %v", err)
	}
	if !found {
		t.Fatalf("expected found == true")
	}
	if page.PageID != "fresh-home" || page.Title != "Abeta Fresh CMS" {
		t.Fatalf("unexpected page payload: %+v", page)
	}
	if len(page.Snippets) != 1 || page.Snippets[0].SnippetID != "cms_hdr" {
		t.Fatalf("expected 1 decoded snippet, got %+v", page.Snippets)
	}

	if gotPath != "/api/v1/screens/fresh-home" {
		t.Errorf("path = %q, want /api/v1/screens/fresh-home", gotPath)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
	if gotTenant != "abeta-fresh" {
		t.Errorf("X-Tenant-ID = %q, want abeta-fresh", gotTenant)
	}
	if gotPlatform != "ios" {
		t.Errorf("X-Platform = %q, want ios", gotPlatform)
	}
	if gotCity != "Bengaluru" {
		t.Errorf("X-City = %q, want Bengaluru", gotCity)
	}
	if gotUserSegment != "plus_member" {
		t.Errorf("X-User-Segment = %q, want plus_member", gotUserSegment)
	}
	if gotWindowClass != "compact" {
		t.Errorf("X-Window-Class = %q, want compact", gotWindowClass)
	}
}

func TestClientFetchScreenFallbacks(t *testing.T) {
	t.Parallel()

	t.Run("http 404 returns not found fallback", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"success":false}`, http.StatusNotFound)
		}))
		defer srv.Close()

		client := layoutclient.NewClient(srv.URL, srv.Client())
		_, found, err := client.FetchScreen(context.Background(), "abeta-fresh", "unknown-screen", domain.ScreenRequest{})
		if found {
			t.Fatalf("expected found == false on 404")
		}
		if !errors.Is(err, domain.ErrScreenNotFound) {
			t.Fatalf("expected ErrScreenNotFound, got %v", err)
		}
	})

	t.Run("http 500 returns error fallback", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := layoutclient.NewClient(srv.URL, srv.Client())
		_, found, err := client.FetchScreen(context.Background(), "abeta-fresh", "fresh-home", domain.ScreenRequest{})
		if found || err == nil {
			t.Fatalf("expected found=false and non-nil err on 500, got found=%v err=%v", found, err)
		}
	})

	t.Run("success false envelope returns error fallback", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":false,"data":{}}`))
		}))
		defer srv.Close()

		client := layoutclient.NewClient(srv.URL, srv.Client())
		_, found, err := client.FetchScreen(context.Background(), "abeta-fresh", "fresh-home", domain.ScreenRequest{})
		if found || err == nil {
			t.Fatalf("expected found=false and err for success=false, got found=%v err=%v", found, err)
		}
	})

	t.Run("malformed json returns error fallback", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer srv.Close()

		client := layoutclient.NewClient(srv.URL, srv.Client())
		_, found, err := client.FetchScreen(context.Background(), "abeta-fresh", "fresh-home", domain.ScreenRequest{})
		if found || err == nil {
			t.Fatalf("expected found=false and err for bad JSON, got found=%v err=%v", found, err)
		}
	})

	t.Run("connection error returns fallback", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		closedURL := srv.URL
		srv.Close()

		client := layoutclient.NewClient(closedURL, &http.Client{Timeout: 500 * time.Millisecond})
		_, found, err := client.FetchScreen(context.Background(), "abeta-fresh", "fresh-home", domain.ScreenRequest{})
		if found || err == nil {
			t.Fatalf("expected found=false and network err, got found=%v err=%v", found, err)
		}
	})

	t.Run("empty screen slug rejected", func(t *testing.T) {
		t.Parallel()
		client := layoutclient.NewClient("http://127.0.0.1:9999", nil)
		_, found, err := client.FetchScreen(context.Background(), "abeta-fresh", " ", domain.ScreenRequest{})
		if found || err == nil {
			t.Fatalf("expected found=false and err for empty slug, got found=%v err=%v", found, err)
		}
	})
}

func TestClientHealthCheck(t *testing.T) {
	t.Parallel()

	t.Run("healthy 200", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/healthz" {
				t.Errorf("unexpected healthz path %q", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer srv.Close()

		client := layoutclient.NewClient(srv.URL, srv.Client())
		if !client.HealthCheck(context.Background()) {
			t.Fatalf("expected HealthCheck() == true")
		}
	})

	t.Run("unhealthy 503", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		client := layoutclient.NewClient(srv.URL, srv.Client())
		if client.HealthCheck(context.Background()) {
			t.Fatalf("expected HealthCheck() == false on 503")
		}
	})

	t.Run("timeout returns false", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			select {
			case <-done:
			case <-time.After(3 * time.Second):
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer func() {
			close(done)
			srv.Close()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		client := layoutclient.NewClient(srv.URL, srv.Client())
		if client.HealthCheck(ctx) {
			t.Fatalf("expected HealthCheck() == false on timeout")
		}
	})
}
