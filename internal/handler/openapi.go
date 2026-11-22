package handler

import (
	"encoding/json"
	"html"
	"net/http"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/snippets"
	sduibff "github.com/abeta-dev/sdui-bff"
	"github.com/gin-gonic/gin"
)

const openAPIVersion = "3.1.0"

var canonicalSnippetTypes = []string{
	snippets.SnippetTypeHeader,
	snippets.SnippetTypeBanner,
	snippets.SnippetTypeMetricCard,
	snippets.SnippetTypeCard,
	snippets.SnippetTypeActionRow,
	snippets.SnippetTypeForm,
	snippets.SnippetTypeEmptyState,
	snippets.SnippetTypeDivider,
	snippets.SnippetTypeSpacer,
	snippets.SnippetTypeCarousel,
	snippets.SnippetTypeGrid,
	snippets.SnippetTypeTabs,
	snippets.SnippetTypeAccordion,
	snippets.SnippetTypeProductCard,
	snippets.SnippetTypeSwipeDeck,
	snippets.SnippetTypeSearchBar,
	snippets.SnippetTypeChipGroup,
	snippets.SnippetTypeNudgeBar,
	snippets.SnippetTypeMarqueeStrip,
	snippets.SnippetTypeOrderTracker,
	snippets.SnippetTypeComposedBlock,
}

var verticalScreenIDs = []string{
	"fresh-home",
	"store-home",
	"bites-home",
	"bites-restaurant",
	"bites-order-track",
	"pulse-home",
	"pulse-event-detail",
	"mart-home",
	"mart-product-detail",
	"spark-deck",
	"spark-matches",
	"trade-home",
	"trade-supplier",
}

func registerOpenAPIRoutes(engine *gin.Engine, name string, requireAuth bool) {
	specBytes := mustBuildOpenAPISpec(name, requireAuth)
	docsHTML := buildDocsHTML(name)

	engine.GET("/openapi.json", ETagMiddleware(), func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", specBytes)
	})

	engine.GET("/docs", func(c *gin.Context) {
		c.Header(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; style-src 'self' https://cdn.jsdelivr.net https://fonts.scalar.com 'unsafe-inline'; font-src 'self' https://cdn.jsdelivr.net https://fonts.scalar.com data:; img-src 'self' data: https:; connect-src 'self'; frame-ancestors 'none';",
		)
		c.Data(http.StatusOK, "text/html; charset=utf-8", docsHTML)
	})

	engine.GET("/llms.txt", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", sduibff.LLMsTxt)
	})

	engine.GET("/llms-full.txt", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", sduibff.LLMsFullTxt)
	})
}

func buildDocsHTML(name string) []byte {
	title := html.EscapeString(name) + " — API Docs"
	page := `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>` + title + `</title>
  <style>
    body { margin: 0; padding: 0; font-family: system-ui, -apple-system, sans-serif; }
  </style>
</head>
<body>
  <script id="api-reference" data-url="/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  <noscript>
    <p>OpenAPI 3.1 schema available at <a href="/openapi.json">/openapi.json</a>.</p>
  </noscript>
</body>
</html>
`
	return []byte(page)
}

func mustBuildOpenAPISpec(name string, requireAuth bool) []byte {
	var apiSecurity []map[string][]string
	if requireAuth {
		apiSecurity = []map[string][]string{{"bearerAuth": {}}}
	}

	jsonContent := func(schema map[string]any) map[string]any {
		return map[string]any{"application/json": map[string]any{"schema": schema}}
	}
	ref := func(schemaName string) map[string]any {
		return map[string]any{"$ref": "#/components/schemas/" + schemaName}
	}
	envelope := func(dataSchema map[string]any) map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"success": map[string]any{"type": "boolean", "example": true},
				"data":    dataSchema,
			},
			"required": []string{"success"},
		}
	}
	headerParam := func(paramName, desc string) map[string]any {
		return map[string]any{
			"name":        paramName,
			"in":          "header",
			"required":    false,
			"description": desc,
			"schema":      map[string]any{"type": "string"},
		}
	}
	queryParam := func(paramName, desc string) map[string]any {
		return map[string]any{
			"name":        paramName,
			"in":          "query",
			"required":    false,
			"description": desc,
			"schema":      map[string]any{"type": "string"},
		}
	}
	postMutation := func(summary, opID, tag, schemaName string, required bool) map[string]any {
		return map[string]any{
			"post": map[string]any{
				"summary":     summary,
				"operationId": opID,
				"tags":        []string{tag},
				"security":    apiSecurity,
				"requestBody": map[string]any{
					"required": required,
					"content":  jsonContent(ref(schemaName)),
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Mutation accepted with follow_up_actions",
						"content":     jsonContent(envelope(ref("ActionResult"))),
					},
					"400": map[string]any{"description": "Validation error"},
					"404": map[string]any{"description": "Resource not found"},
				},
			},
		}
	}

	etagHeader := map[string]any{
		"description": "SHA-256 content entity tag",
		"schema":      map[string]any{"type": "string"},
	}
	cacheHeader := map[string]any{
		"description": "HTTP cache policy",
		"schema":      map[string]any{"type": "string", "example": defaultCacheControl},
	}
	sourceHeader := map[string]any{
		"description": "Screen resolution source (layout-engine, standalone, or standalone-fallback)",
		"schema":      map[string]any{"type": "string", "enum": []string{"layout-engine", "standalone", "standalone-fallback"}},
	}

	spec := map[string]any{
		"openapi": openAPIVersion,
		"info": map[string]any{
			"title":       name,
			"version":     "1.1.0",
			"description": "Abeta Multi-Vertical SDUI Domain BFF powered by github.com/abeta-dev/kmp/bff@v0.3.0 (16 Wire Atoms, 21 canonical SDUI snippet types including nudge_bar, marquee_strip, order_tracker, and server-molded composed_block recipes, 18 ActionTypes, 7 ambient MotionLoopEffects, and Free-Delivery threshold FloatingCartSpec docks) across Abeta Fresh, Abeta Bites, Abeta Pulse, Abeta Mart, Abeta Spark, and Abeta Trade with optional Layout Engine hydration.",
		},
		"servers": []map[string]any{
			{"url": "/", "description": "Current SDUI BFF origin"},
		},
		"tags": []map[string]any{
			{"name": "Operations", "description": "Liveness and readiness probes"},
			{"name": "Documentation", "description": "OpenAPI 3.1 spec, Scalar UI, and llmstxt.org AI-agent documentation"},
			{"name": "Discovery", "description": "Super-App catalog and SDUI screen composition"},
			{"name": "Actions", "description": "Generic SDUI action execution"},
			{"name": "Abeta Fresh", "description": "8-minute quick-commerce grocery express checkout"},
			{"name": "Abeta Bites", "description": "Food delivery orders and live prep tracking"},
			{"name": "Abeta Pulse", "description": "Live events, cinema, comedy, and dining bookings"},
			{"name": "Abeta Mart", "description": "Multi-category marketplace orders"},
			{"name": "Abeta Spark", "description": "Swipe discovery deck and live match persistence"},
			{"name": "Abeta Trade", "description": "B2B wholesale supplier sourcing and RFQ submissions"},
		},
		"paths": map[string]any{
			"/healthz": map[string]any{
				"get": map[string]any{
					"summary":     "Liveness probe",
					"operationId": "getHealthz",
					"tags":        []string{"Operations"},
					"responses": map[string]any{
						"200": map[string]any{"description": "Service is healthy", "content": jsonContent(map[string]any{"type": "object"})},
					},
				},
			},
			"/readyz": map[string]any{
				"get": map[string]any{
					"summary":     "Readiness probe with Layout Engine connectivity status",
					"operationId": "getReadyz",
					"tags":        []string{"Operations"},
					"responses": map[string]any{
						"200": map[string]any{"description": "Service is ready", "content": jsonContent(map[string]any{"type": "object"})},
					},
				},
			},
			"/openapi.json": map[string]any{
				"get": map[string]any{
					"summary":     "OpenAPI 3.1 JSON specification",
					"operationId": "getOpenAPI",
					"tags":        []string{"Documentation"},
					"responses": map[string]any{
						"200": map[string]any{"description": "OpenAPI 3.1 JSON", "content": jsonContent(map[string]any{"type": "object"})},
					},
				},
			},
			"/docs": map[string]any{
				"get": map[string]any{
					"summary":     "Scalar interactive API documentation UI",
					"operationId": "getDocs",
					"tags":        []string{"Documentation"},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Scalar HTML documentation",
							"content":     map[string]any{"text/html": map[string]any{"schema": map[string]any{"type": "string"}}},
						},
					},
				},
			},
			"/llms.txt": map[string]any{
				"get": map[string]any{
					"summary":     "llmstxt.org AI-agent index",
					"operationId": "getLLMsTxt",
					"tags":        []string{"Documentation"},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Plain-text llmstxt.org Markdown index",
							"content":     map[string]any{"text/plain": map[string]any{"schema": map[string]any{"type": "string"}}},
						},
					},
				},
			},
			"/llms-full.txt": map[string]any{
				"get": map[string]any{
					"summary":     "Complete single-file LLM architecture and API reference",
					"operationId": "getLLMsFullTxt",
					"tags":        []string{"Documentation"},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Plain-text full LLM reference",
							"content":     map[string]any{"text/plain": map[string]any{"schema": map[string]any{"type": "string"}}},
						},
					},
				},
			},
			"/api/v1/apps": map[string]any{
				"get": map[string]any{
					"summary":     "List all 6 Super-App verticals",
					"operationId": "listApps",
					"tags":        []string{"Discovery"},
					"security":    apiSecurity,
					"parameters":  []map[string]any{headerParam("If-None-Match", "Conditional ETag header")},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Registered Super-App descriptors",
							"headers":     map[string]any{"ETag": etagHeader, "Cache-Control": cacheHeader},
							"content": jsonContent(envelope(map[string]any{
								"type": "object",
								"properties": map[string]any{
									"apps": map[string]any{"type": "array", "items": ref("AppDescriptor")},
								},
							})),
						},
						"304": map[string]any{"description": "Not Modified"},
					},
				},
			},
			"/api/v1/screens": map[string]any{
				"get": map[string]any{
					"summary":     "List all discoverable SDUI screen routes across all 6 verticals",
					"operationId": "listScreens",
					"tags":        []string{"Discovery"},
					"security":    apiSecurity,
					"parameters":  []map[string]any{headerParam("If-None-Match", "Conditional ETag header")},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Screen summary catalog",
							"headers":     map[string]any{"ETag": etagHeader, "Cache-Control": cacheHeader},
							"content": jsonContent(envelope(map[string]any{
								"type": "object",
								"properties": map[string]any{
									"screens": map[string]any{"type": "array", "items": ref("ScreenSummary")},
								},
							})),
						},
						"304": map[string]any{"description": "Not Modified"},
					},
				},
			},
			"/api/v1/screens/{screenId}": map[string]any{
				"get": map[string]any{
					"summary":     "Resolve and render an SDUI screen payload",
					"operationId": "getScreen",
					"tags":        []string{"Discovery"},
					"security":    apiSecurity,
					"parameters": []map[string]any{
						{
							"name":        "screenId",
							"in":          "path",
							"required":    true,
							"description": "Vertical or Layout Engine screen identifier",
							"schema":      map[string]any{"type": "string", "enum": verticalScreenIDs},
						},
						queryParam("q", "Live search query"),
						queryParam("filter", "Category or facet filter"),
						queryParam("city", "Target city override"),
						queryParam("id", "Entity identifier for detail screens"),
						queryParam("cursor", "Pagination cursor"),
						headerParam("X-Tenant-ID", "Explicit Layout Engine tenant override"),
						headerParam("X-Platform", "Client platform (android, ios, macos, jvm, web)"),
						headerParam("X-City", "Client city header"),
						headerParam("X-User-Segment", "User targeting segment"),
						headerParam("X-Window-Class", "Adaptive window width class (compact, medium, expanded)"),
						headerParam("If-None-Match", "Conditional ETag header"),
					},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "SDUI v1 PageResponse envelope",
							"headers": map[string]any{
								"ETag":          etagHeader,
								"Cache-Control": cacheHeader,
								"X-SDUI-Source": sourceHeader,
							},
							"content": jsonContent(envelope(ref("PageResponse"))),
						},
						"304": map[string]any{"description": "Not Modified"},
						"404": map[string]any{"description": "Screen not found"},
					},
				},
			},
			"/api/v1/actions/execute":      postMutation("Execute a generic SDUI action", "executeAction", "Actions", "ActionExecuteRequest", true),
			"/api/v1/orders/express":       postMutation("Place an Abeta Fresh express order (compat route)", "placeExpressOrderCompat", "Abeta Fresh", "ExpressOrderRequest", false),
			"/api/v1/fresh/orders/express": postMutation("Place an Abeta Fresh 8-minute express order", "placeFreshExpressOrder", "Abeta Fresh", "ExpressOrderRequest", false),
			"/api/v1/bites/orders":         postMutation("Place an Abeta Bites food delivery order", "placeFoodOrder", "Abeta Bites", "ExpressOrderRequest", false),
			"/api/v1/pulse/bookings":       postMutation("Book Abeta Pulse event or cinema tickets", "bookPulseTickets", "Abeta Pulse", "BookingRequest", true),
			"/api/v1/mart/orders":          postMutation("Place an Abeta Mart marketplace order", "placeMartOrder", "Abeta Mart", "ExpressOrderRequest", false),
			"/api/v1/spark/swipe":          postMutation("Record an Abeta Spark swipe decision", "recordSparkSwipe", "Abeta Spark", "SwipeRequest", true),
			"/api/v1/trade/rfq":            postMutation("Submit an Abeta Trade B2B Request For Quotation", "submitTradeRFQ", "Abeta Trade", "RFQRequest", true),
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
			"schemas": openAPISchemas(),
		},
	}

	raw, err := json.Marshal(spec)
	if err != nil {
		panic(err)
	}
	return raw
}

func openAPISchemas() map[string]any {
	return map[string]any{
		"AppDescriptor": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":           map[string]any{"type": "string"},
				"name":         map[string]any{"type": "string"},
				"tagline":      map[string]any{"type": "string"},
				"category":     map[string]any{"type": "string"},
				"tenant_id":    map[string]any{"type": "string"},
				"entry_screen": map[string]any{"type": "string"},
				"theme_preset": map[string]any{"type": "string"},
				"screens":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
		},
		"ScreenSummary": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"screen_id":   map[string]any{"type": "string"},
				"title":       map[string]any{"type": "string"},
				"vertical":    map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
			},
		},
		"PageResponse": map[string]any{
			"type":        "object",
			"description": "Typed v1 SDUI PageResponse from github.com/abeta-dev/kmp/bff@v0.3.0 (21 canonical snippet types, adaptive_layout, floating_cart with free-delivery threshold nudges, and string-only tracking)",
			"properties": map[string]any{
				"page_id": map[string]any{"type": "string"},
				"title":   map[string]any{"type": "string"},
				"snippets": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"snippet_id":   map[string]any{"type": "string"},
							"snippet_type": map[string]any{"type": "string", "enum": canonicalSnippetTypes},
							"data":         map[string]any{"type": "object", "additionalProperties": true},
						},
					},
				},
				"floating_cart": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"label":                         map[string]any{"type": "string"},
						"free_delivery_threshold_cents": map[string]any{"type": "integer"},
						"nudge_prefix":                  map[string]any{"type": "string"},
						"unlocked_label":                map[string]any{"type": "string"},
					},
				},
				"adaptive_layout": map[string]any{"type": "object", "additionalProperties": true},
				"tracking":        map[string]any{"type": "object", "additionalProperties": true},
			},
		},
		"ActionExecuteRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action_id":   map[string]any{"type": "string"},
				"action_type": map[string]any{"type": "string", "example": "track_event"},
				"payload":     map[string]any{"type": "object", "additionalProperties": true},
			},
			"required": []string{"action_type"},
		},
		"ActionResult": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":            map[string]any{"type": "string"},
				"message":           map[string]any{"type": "string"},
				"follow_up_actions": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				"data":              map[string]any{"type": "object", "additionalProperties": true},
			},
		},
		"ExpressOrderRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"vertical":   map[string]any{"type": "string"},
				"items":      map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer"}},
				"address":    map[string]any{"type": "string"},
				"coupon":     map[string]any{"type": "string"},
				"payment":    map[string]any{"type": "string"},
				"notes":      map[string]any{"type": "string"},
				"item_count": map[string]any{"type": "integer"},
				"total_usd":  map[string]any{"type": "number"},
			},
		},
		"BookingRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"event_id":   map[string]any{"type": "string"},
				"event_name": map[string]any{"type": "string"},
				"seat_tier":  map[string]any{"type": "string"},
				"quantity":   map[string]any{"type": "integer"},
				"city":       map[string]any{"type": "string"},
			},
		},
		"SwipeRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"profile_id": map[string]any{"type": "string"},
				"direction":  map[string]any{"type": "string", "enum": []string{"like", "nope", "super_like", "undo", "reset"}},
			},
			"required": []string{"direction"},
		},
		"RFQRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"supplier_id":   map[string]any{"type": "string"},
				"product_name":  map[string]any{"type": "string"},
				"quantity":      map[string]any{"type": "integer"},
				"company_name":  map[string]any{"type": "string"},
				"contact_email": map[string]any{"type": "string"},
				"delivery_city": map[string]any{"type": "string"},
				"notes":         map[string]any{"type": "string"},
			},
			"required": []string{"company_name", "contact_email", "quantity"},
		},
	}
}
