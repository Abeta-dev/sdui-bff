package handler_test

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/abeta-dev/sdui-bff/internal/handler"
)

func TestMiddlewareEdgeCases(t *testing.T) {
	t.Parallel()
	router, _ := newTestRouter("", "", 0)

	t.Run("HEAD skips compression", func(t *testing.T) {
		w := doJSONRequest(router, http.MethodHead, "/api/v1/apps", "", map[string]string{
			"Accept-Encoding": "br",
		})
		if w.Header().Get("Content-Encoding") != "" {
			t.Errorf("HEAD should not set Content-Encoding, got %q", w.Header().Get("Content-Encoding"))
		}
	})

	t.Run("Accept-Encoding quality negotiation", func(t *testing.T) {
		wGzip := doJSONRequest(router, http.MethodGet, "/api/v1/apps", "", map[string]string{
			"Accept-Encoding": "br;q=0.2, gzip;q=0.9",
		})
		if wGzip.Code != http.StatusOK || wGzip.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("expected gzip when higher q, got code=%d enc=%q", wGzip.Code, wGzip.Header().Get("Content-Encoding"))
		}

		wRejected := doJSONRequest(router, http.MethodGet, "/api/v1/apps", "", map[string]string{
			"Accept-Encoding": "br;q=0, gzip;q=0",
		})
		if wRejected.Code != http.StatusOK || wRejected.Header().Get("Content-Encoding") != "" {
			t.Fatalf("expected identity when q=0, got code=%d enc=%q", wRejected.Code, wRejected.Header().Get("Content-Encoding"))
		}

		wWildcard := doJSONRequest(router, http.MethodGet, "/api/v1/apps", "", map[string]string{
			"Accept-Encoding": "*;q=0.8",
		})
		if wWildcard.Code != http.StatusOK || wWildcard.Header().Get("Content-Encoding") != "br" {
			t.Fatalf("expected br for wildcard Accept-Encoding, got code=%d enc=%q", wWildcard.Code, wWildcard.Header().Get("Content-Encoding"))
		}
	})

	t.Run("If-None-Match wildcard and multi-tag", func(t *testing.T) {
		w1 := doJSONRequest(router, http.MethodGet, "/api/v1/apps", "", nil)
		etag := w1.Header().Get("ETag")

		wStar := doJSONRequest(router, http.MethodGet, "/api/v1/apps", "", map[string]string{
			"If-None-Match": "*",
		})
		if wStar.Code != http.StatusNotModified {
			t.Fatalf("expected 304 for If-None-Match: *, got %d", wStar.Code)
		}

		wMulti := doJSONRequest(router, http.MethodGet, "/api/v1/apps", "", map[string]string{
			"If-None-Match": `"other-tag", ` + etag,
		})
		if wMulti.Code != http.StatusNotModified {
			t.Fatalf("expected 304 for comma-separated ETag match, got %d", wMulti.Code)
		}
	})
}

func TestHandlerNilDependenciesAndMalformedJSON(t *testing.T) {
	t.Parallel()

	emptyRouter := handler.NewRouter(handler.Dependencies{
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})

	unavailableRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/apps"},
		{http.MethodGet, "/api/v1/screens"},
		{http.MethodGet, "/api/v1/screens/fresh-home"},
		{http.MethodPost, "/api/v1/actions/execute"},
		{http.MethodPost, "/api/v1/fresh/orders/express"},
		{http.MethodPost, "/api/v1/bites/orders"},
		{http.MethodPost, "/api/v1/pulse/bookings"},
		{http.MethodPost, "/api/v1/mart/orders"},
		{http.MethodPost, "/api/v1/spark/swipe"},
		{http.MethodPost, "/api/v1/trade/rfq"},
	}

	for _, tc := range unavailableRoutes {
		w := doJSONRequest(emptyRouter, tc.method, tc.path, `{}`, nil)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s with nil dep: got %d, want 503", tc.method, tc.path, w.Code)
		}
	}

	fullRouter, _ := newTestRouter("", "", 0)
	malformedRoutes := []string{
		"/api/v1/actions/execute",
		"/api/v1/fresh/orders/express",
		"/api/v1/bites/orders",
		"/api/v1/pulse/bookings",
		"/api/v1/mart/orders",
		"/api/v1/spark/swipe",
		"/api/v1/trade/rfq",
	}
	for _, path := range malformedRoutes {
		w := doJSONRequest(fullRouter, http.MethodPost, path, `{not-valid-json`, nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("POST %s malformed JSON: got %d, want 400", path, w.Code)
		}
	}
}
