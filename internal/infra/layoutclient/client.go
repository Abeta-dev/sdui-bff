// Package layoutclient provides an optional HTTP client for fetching dynamic
// CMS-managed SDUI layouts from an external Abeta Layout Engine service.
// When no base URL is configured, the client operates in standalone mode so the
// domain BFF runs with zero external dependencies.
package layoutclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

const (
	defaultRequestTimeout = 5 * time.Second
	healthCheckTimeout    = 1500 * time.Millisecond
	maxResponseBytes      = 4 << 20 // 4 MiB safety bound for screen payloads
)

// Compile-time check ensuring Client satisfies domain.LayoutEngineClient.
var _ domain.LayoutEngineClient = (*Client)(nil)

// Client fetches SDUI PageResponse payloads from a remote Layout Engine.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

type screenResponseEnvelope struct {
	Success bool                  `json:"success"`
	Data    snippets.PageResponse `json:"data"`
}

// NewClient constructs a Layout Engine HTTP client.
// When baseURL is empty or whitespace, Enabled returns false and FetchScreen
// becomes a no-op returning (PageResponse{}, false, nil) for standalone mode.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultRequestTimeout}
	}
	return &Client{
		baseURL:    normalized,
		httpClient: client,
	}
}

// Enabled reports whether a Layout Engine base URL is configured.
func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

// FetchScreen queries GET <baseURL>/api/v1/screens/<screenSlug> with SDUI targeting
// headers and decodes the standard ginmw {"success": true, "data": <PageResponse>}
// envelope. On 404, network error, timeout, or invalid payload it returns
// (PageResponse{}, false, err) so callers can fall back to standalone rendering.
func (c *Client) FetchScreen(
	ctx context.Context,
	tenantID string,
	screenSlug string,
	req domain.ScreenRequest,
) (snippets.PageResponse, bool, error) {
	if !c.Enabled() {
		return snippets.PageResponse{}, false, nil
	}

	slug := strings.TrimSpace(screenSlug)
	if slug == "" {
		return snippets.PageResponse{}, false, fmt.Errorf("layoutclient: screen slug is required")
	}

	endpoint := fmt.Sprintf("%s/api/v1/screens/%s", c.baseURL, url.PathEscape(slug))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return snippets.PageResponse{}, false, fmt.Errorf("layoutclient: build request for %q: %w", slug, err)
	}

	effectiveTenant := strings.TrimSpace(tenantID)
	if effectiveTenant == "" {
		effectiveTenant = strings.TrimSpace(req.TenantID)
	}
	httpReq.Header.Set("Accept", "application/json")
	setHeaderIfNonEmpty(httpReq.Header, "X-Tenant-ID", effectiveTenant)
	setHeaderIfNonEmpty(httpReq.Header, "X-Platform", req.Platform)
	setHeaderIfNonEmpty(httpReq.Header, "X-City", req.City)
	setHeaderIfNonEmpty(httpReq.Header, "X-User-Segment", req.UserSegment)
	setHeaderIfNonEmpty(httpReq.Header, "X-Window-Class", req.WindowClass)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return snippets.PageResponse{}, false, fmt.Errorf("layoutclient: fetch screen %q: %w", slug, err)
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return snippets.PageResponse{}, false, fmt.Errorf("layoutclient: screen %q: %w", slug, domain.ErrScreenNotFound)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return snippets.PageResponse{}, false, fmt.Errorf("layoutclient: unexpected status %d for screen %q", resp.StatusCode, slug)
	}

	var envelope screenResponseEnvelope
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes))
	if err := decoder.Decode(&envelope); err != nil {
		return snippets.PageResponse{}, false, fmt.Errorf("layoutclient: decode envelope for %q: %w", slug, err)
	}
	if !envelope.Success {
		return snippets.PageResponse{}, false, fmt.Errorf("layoutclient: layout engine returned success=false for %q", slug)
	}

	return envelope.Data, true, nil
}

// HealthCheck verifies GET <baseURL>/healthz using a bounded 1.5s timeout.
func (c *Client) HealthCheck(ctx context.Context) bool {
	if !c.Enabled() {
		return false
	}

	checkCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(checkCtx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return false
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false
	}
	defer drainAndClose(resp.Body)

	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
}

func setHeaderIfNonEmpty(h http.Header, key, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		h.Set(key, trimmed)
	}
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 4096))
	_ = body.Close()
}
