package domain

import (
	"context"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/analytics"
	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
)

// VerticalModule is the Open/Closed Strategy port implemented by each vertical domain BFF
// (Abeta Fresh, Abeta Bites, Abeta Pulse, Abeta Mart, Abeta Spark, Abeta Trade, Abeta Pay).
type VerticalModule interface {
	Descriptor() AppDescriptor
	Screens() []ScreenSummary
	CanHandleScreen(screenID string) bool
	RenderScreen(ctx context.Context, req ScreenRequest) (snippets.PageResponse, error)
}

// LayoutEngineClient optionally fetches dynamic CMS-managed layouts from Abeta-dev/layout-engine
// when LAYOUT_ENGINE_URL is configured, falling back gracefully in standalone mode.
type LayoutEngineClient interface {
	Enabled() bool
	FetchScreen(ctx context.Context, tenantID, screenSlug string, req ScreenRequest) (snippets.PageResponse, bool, error)
	HealthCheck(ctx context.Context) bool
}

// ScreenComposer orchestrates screen resolution across Layout Engine + domain vertical hydration.
type ScreenComposer interface {
	ListApps() []AppDescriptor
	ListScreens() []ScreenSummary
	GetScreen(ctx context.Context, req ScreenRequest) (snippets.PageResponse, string, error)
}

// ActionExecutor handles generic POST /api/v1/actions/execute calls.
type ActionExecutor interface {
	Execute(ctx context.Context, req ActionExecuteRequest) (ActionResult, error)
}

// AnalyticsStore persists ingested client/BFF analytics batches and aggregates live summaries.
type AnalyticsStore interface {
	RecordBatch(ctx context.Context, req analytics.BatchRequest) (analytics.IngestResponse, error)
	GetSummary(ctx context.Context) (analytics.Summary, error)
}

// AnalyticsService validates, enriches, and exposes live SDUI analytics telemetry.
type AnalyticsService interface {
	IngestBatch(ctx context.Context, req analytics.BatchRequest) (analytics.IngestResponse, error)
	Summary(ctx context.Context) (analytics.Summary, error)
}
