package domain

import (
	"context"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
)

// VerticalModule is the Open/Closed Strategy port implemented by each vertical domain BFF
// (Abeta Fresh, Abeta Bites, Abeta Pulse, Abeta Mart, Abeta Spark, Abeta Trade).
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
