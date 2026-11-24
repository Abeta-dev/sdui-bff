// Package usecase implements application-layer orchestration for the Abeta SDUI Domain BFF,
// composing screens across optional Layout Engine hydration and Open/Closed vertical modules.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

// Screen resolution source markers exposed via the X-SDUI-Source response header.
const (
	SourceLayoutEngine       = "layout-engine"
	SourceStandalone         = "standalone"
	SourceStandaloneFallback = "standalone-fallback"

	defaultProxyTenantID = "abeta-fresh"
)

// Compile-time check ensuring DefaultScreenComposer satisfies domain.ScreenComposer.
var _ domain.ScreenComposer = (*DefaultScreenComposer)(nil)

// DefaultScreenComposer acts as the Open/Closed vertical registry and screen orchestrator.
// It resolves screens dynamically from an optional Layout Engine with automatic fallback to
// registered domain.VerticalModule implementations.
type DefaultScreenComposer struct {
	layoutClient domain.LayoutEngineClient
	verticals    []domain.VerticalModule
}

// NewScreenComposer creates a DefaultScreenComposer with an optional LayoutEngineClient
// and any number of VerticalModule implementations.
func NewScreenComposer(layoutClient domain.LayoutEngineClient, verticals ...domain.VerticalModule) *DefaultScreenComposer {
	registered := make([]domain.VerticalModule, 0, len(verticals))
	for _, v := range verticals {
		if v != nil {
			registered = append(registered, v)
		}
	}
	return &DefaultScreenComposer{
		layoutClient: layoutClient,
		verticals:    registered,
	}
}

// ListApps returns descriptors for all registered Super-App verticals in registration order.
func (c *DefaultScreenComposer) ListApps() []domain.AppDescriptor {
	apps := make([]domain.AppDescriptor, 0, len(c.verticals))
	for _, v := range c.verticals {
		apps = append(apps, v.Descriptor())
	}
	return apps
}

// ListScreens returns all discoverable screen summaries across all registered verticals.
func (c *DefaultScreenComposer) ListScreens() []domain.ScreenSummary {
	screens := make([]domain.ScreenSummary, 0, len(c.verticals)*2)
	for _, v := range c.verticals {
		screens = append(screens, v.Screens()...)
	}
	return screens
}

// GetScreen resolves an SDUI screen by ID, returning the PageResponse alongside a source
// marker ("layout-engine", "standalone", or "standalone-fallback"). Every returned page is
// normalized via snippets.EnsureComponentIDs so all snippets, composed nodes, atoms, and
// action triggers carry deterministic, stable component identifiers.
func (c *DefaultScreenComposer) GetScreen(ctx context.Context, req domain.ScreenRequest) (snippets.PageResponse, string, error) {
	if err := ctx.Err(); err != nil {
		return snippets.PageResponse{}, "", err
	}

	screenID := strings.TrimSpace(strings.TrimPrefix(req.ScreenID, "/"))
	if screenID == "" {
		return snippets.PageResponse{}, "", fmt.Errorf("%w: empty screen id", domain.ErrScreenNotFound)
	}
	req.ScreenID = screenID

	vertical := c.findVertical(screenID)
	layoutEnabled := c.layoutClient != nil && c.layoutClient.Enabled()

	if vertical != nil {
		if layoutEnabled && !hasDynamicVerticalParams(req) {
			tenantID := resolveTenantID(req.TenantID, vertical.Descriptor().TenantID)
			page, ok, err := c.layoutClient.FetchScreen(ctx, tenantID, screenID, req)
			if err == nil && ok {
				return finalizePage(page, SourceLayoutEngine)
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return snippets.PageResponse{}, "", ctxErr
			}

			fallbackPage, renderErr := vertical.RenderScreen(ctx, req)
			if renderErr != nil {
				return snippets.PageResponse{}, "", renderErr
			}
			return finalizePage(fallbackPage, SourceStandaloneFallback)
		}

		page, err := vertical.RenderScreen(ctx, req)
		if err != nil {
			return snippets.PageResponse{}, "", err
		}
		return finalizePage(page, SourceStandalone)
	}

	if layoutEnabled {
		tenantID := resolveTenantID(req.TenantID, defaultProxyTenantID)
		page, ok, err := c.layoutClient.FetchScreen(ctx, tenantID, screenID, req)
		if err == nil && ok {
			return finalizePage(page, SourceLayoutEngine)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return snippets.PageResponse{}, "", ctxErr
		}
		if err != nil && !errors.Is(err, domain.ErrScreenNotFound) {
			return snippets.PageResponse{}, "", fmt.Errorf("%w: %s", domain.ErrScreenNotFound, screenID)
		}
	}

	return snippets.PageResponse{}, "", fmt.Errorf("%w: %s", domain.ErrScreenNotFound, screenID)
}

func finalizePage(page snippets.PageResponse, source string) (snippets.PageResponse, string, error) {
	snippets.EnsureComponentIDs(&page)
	return page, source, nil
}

func (c *DefaultScreenComposer) findVertical(screenID string) domain.VerticalModule {
	for _, v := range c.verticals {
		if v.CanHandleScreen(screenID) {
			return v
		}
	}
	return nil
}

func hasDynamicVerticalParams(req domain.ScreenRequest) bool {
	if strings.TrimSpace(req.Query) != "" || strings.TrimSpace(req.Filter) != "" {
		return true
	}
	if req.Extra != nil && strings.TrimSpace(req.Extra["id"]) != "" {
		return true
	}
	return false
}

func resolveTenantID(requested, fallback string) string {
	if trimmed := strings.TrimSpace(requested); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
}
