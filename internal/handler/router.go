package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/abeta-dev/go-libs/ginmw"
	"github.com/abeta-dev/go-libs/logger"
	"github.com/abeta-dev/go-libs/ratelimit"
	"github.com/abeta-dev/sdui-bff/internal/domain"
	"github.com/abeta-dev/sdui-bff/internal/verticals/bites"
	"github.com/abeta-dev/sdui-bff/internal/verticals/fresh"
	"github.com/abeta-dev/sdui-bff/internal/verticals/mart"
	"github.com/abeta-dev/sdui-bff/internal/verticals/pay"
	"github.com/abeta-dev/sdui-bff/internal/verticals/pulse"
	"github.com/abeta-dev/sdui-bff/internal/verticals/spark"
	"github.com/abeta-dev/sdui-bff/internal/verticals/trade"
	"github.com/gin-gonic/gin"
)

const serviceName = "abeta-sdui-bff"

// Dependencies bundles all usecase ports, vertical mutation handlers, and security options
// required to assemble the Abeta SDUI Domain BFF HTTP router.
type Dependencies struct {
	Composer       domain.ScreenComposer
	Actions        domain.ActionExecutor
	LayoutClient   domain.LayoutEngineClient
	Fresh          *fresh.Module
	Bites          *bites.Module
	Pulse          *pulse.Module
	Mart           *mart.Module
	Spark          *spark.Module
	Trade          *trade.Module
	Pay            *pay.Module
	JWTSecret      string
	RateLimitRPS   float64
	Logger         *slog.Logger
	AllowedOrigins []string
}

type bffHandler struct {
	deps Dependencies
}

// NewRouter constructs a configured Gin Engine with go-libs/ginmw middlewares,
// Brotli/gzip compression, SHA-256 ETag caching, OpenAPI 3.1 docs, and all vertical routes.
func NewRouter(deps Dependencies) *gin.Engine {
	log := deps.Logger
	if log == nil {
		log = logger.Default()
	}

	origins := deps.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{"*"}
	}

	engine := gin.New()
	engine.Use(
		ginmw.RequestID(),
		ginmw.Recovery(ginmw.WithRecoveryLogger(log)),
		ginmw.SecurityHeaders(serviceName),
		ginmw.CORS(origins),
		ginmw.LimitBodyDefault(),
	)

	if deps.RateLimitRPS > 0 {
		capacity := max(int(math.Ceil(deps.RateLimitRPS)), 1)
		refillRate := max(time.Duration(float64(time.Second)/deps.RateLimitRPS), time.Nanosecond)
		limiter := ratelimit.NewTokenBucket(capacity, refillRate)
		engine.Use(ginmw.RateLimitWithLimiter(limiter))
	}

	engine.Use(CompressionMiddleware())

	h := &bffHandler{deps: deps}

	engine.GET("/healthz", h.healthz)
	engine.GET("/readyz", h.readyz)
	registerOpenAPIRoutes(engine, serviceName, strings.TrimSpace(deps.JWTSecret) != "")

	v1 := engine.Group("/api/v1")
	if secret := strings.TrimSpace(deps.JWTSecret); secret != "" {
		if err := ginmw.SetJWTSecret(secret); err != nil {
			log.Error("failed to configure JWT secret", slog.Any("error", err))
		}
		v1.Use(ginmw.AuthMiddleware())
	}

	etag := ETagMiddleware()
	v1.GET("/apps", etag, h.listApps)
	v1.GET("/screens", etag, h.listScreens)
	v1.GET("/screens/:screenId", etag, h.getScreen)

	v1.POST("/actions/execute", h.executeAction)
	v1.POST("/orders/express", h.placeFreshExpressOrder)
	v1.POST("/fresh/orders/express", h.placeFreshExpressOrder)
	v1.POST("/bites/orders", h.placeFoodOrder)
	v1.POST("/pulse/bookings", h.bookPulseTickets)
	v1.POST("/mart/orders", h.placeMartOrder)
	v1.POST("/spark/swipe", h.recordSparkSwipe)
	v1.POST("/trade/rfq", h.submitTradeRFQ)
	v1.POST("/pay/transfers", h.executePayTransfer)
	v1.POST("/pay/cards/controls", h.updatePayCardControls)

	return engine
}

func (h *bffHandler) modeLabel() string {
	if h.deps.LayoutClient != nil && h.deps.LayoutClient.Enabled() {
		return "layout-engine"
	}
	return "standalone"
}

func (h *bffHandler) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": serviceName,
		"mode":    h.modeLabel(),
	})
}

func (h *bffHandler) readyz(c *gin.Context) {
	enabled := h.deps.LayoutClient != nil && h.deps.LayoutClient.Enabled()
	connected := enabled && h.deps.LayoutClient.HealthCheck(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"status":                  "ready",
		"service":                 serviceName,
		"mode":                    h.modeLabel(),
		"layout_engine_enabled":   enabled,
		"layout_engine_connected": connected,
	})
}

func (h *bffHandler) listApps(c *gin.Context) {
	if h.deps.Composer == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "screen composer unavailable")
		return
	}
	ginmw.RespondSuccess(c, gin.H{"apps": h.deps.Composer.ListApps()})
}

func (h *bffHandler) listScreens(c *gin.Context) {
	if h.deps.Composer == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "screen composer unavailable")
		return
	}
	ginmw.RespondSuccess(c, gin.H{"screens": h.deps.Composer.ListScreens()})
}

func (h *bffHandler) getScreen(c *gin.Context) {
	if h.deps.Composer == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "screen composer unavailable")
		return
	}

	city := strings.TrimSpace(c.Query("city"))
	if city == "" {
		city = strings.TrimSpace(c.GetHeader("X-City"))
	}

	extra := make(map[string]string)
	for key, vals := range c.Request.URL.Query() {
		switch key {
		case "q", "filter", "city", "cursor":
			continue
		default:
			if len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
				extra[key] = strings.TrimSpace(vals[0])
			}
		}
	}

	req := domain.ScreenRequest{
		ScreenID:    c.Param("screenId"),
		Query:       c.Query("q"),
		Filter:      c.Query("filter"),
		City:        city,
		Platform:    strings.TrimSpace(c.GetHeader("X-Platform")),
		UserSegment: strings.TrimSpace(c.GetHeader("X-User-Segment")),
		WindowClass: strings.TrimSpace(c.GetHeader("X-Window-Class")),
		TenantID:    strings.TrimSpace(c.GetHeader("X-Tenant-ID")),
		Cursor:      c.Query("cursor"),
		Extra:       extra,
	}

	page, source, err := h.deps.Composer.GetScreen(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}

	if source != "" {
		c.Header("X-SDUI-Source", source)
	}
	ginmw.RespondSuccess(c, page)
}

func (h *bffHandler) executeAction(c *gin.Context) {
	if h.deps.Actions == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "action executor unavailable")
		return
	}

	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		ginmw.RespondErrorWithDetail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid action payload", err.Error())
		return
	}

	var req domain.ActionExecuteRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		ginmw.RespondErrorWithDetail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid action payload", err.Error())
		return
	}

	// Compatibility bridge for KMP trigger-envelope callers.
	if strings.TrimSpace(req.ActionType) == "" {
		var envelope struct {
			Trigger *struct {
				ID      string         `json:"id"`
				Type    string         `json:"type"`
				Payload map[string]any `json:"payload"`
			} `json:"trigger"`
		}
		if json.Unmarshal(rawBody, &envelope) == nil && envelope.Trigger != nil {
			req.ActionID = envelope.Trigger.ID
			req.ActionType = envelope.Trigger.Type
			req.Payload = envelope.Trigger.Payload
		}
	}

	res, err := h.deps.Actions.Execute(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}
	ginmw.RespondSuccess(c, res)
}

func (h *bffHandler) placeFreshExpressOrder(c *gin.Context) {
	if h.deps.Fresh == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "fresh vertical unavailable")
		return
	}
	var req domain.ExpressOrderRequest
	if !bindOptionalJSON(c, &req, "invalid express order payload") {
		return
	}
	res, err := h.deps.Fresh.PlaceExpressOrder(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}
	ginmw.RespondSuccess(c, res)
}

func (h *bffHandler) placeFoodOrder(c *gin.Context) {
	if h.deps.Bites == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "bites vertical unavailable")
		return
	}
	var req domain.ExpressOrderRequest
	if !bindOptionalJSON(c, &req, "invalid food order payload") {
		return
	}
	res, err := h.deps.Bites.PlaceFoodOrder(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}
	ginmw.RespondSuccess(c, res)
}

func (h *bffHandler) bookPulseTickets(c *gin.Context) {
	if h.deps.Pulse == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "pulse vertical unavailable")
		return
	}
	var req domain.BookingRequest
	if !bindOptionalJSON(c, &req, "invalid booking payload") {
		return
	}
	res, err := h.deps.Pulse.BookTickets(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}
	ginmw.RespondSuccess(c, res)
}

func (h *bffHandler) placeMartOrder(c *gin.Context) {
	if h.deps.Mart == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "mart vertical unavailable")
		return
	}
	var req domain.ExpressOrderRequest
	if !bindOptionalJSON(c, &req, "invalid marketplace order payload") {
		return
	}
	res, err := h.deps.Mart.PlaceOrder(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}
	ginmw.RespondSuccess(c, res)
}

func (h *bffHandler) recordSparkSwipe(c *gin.Context) {
	if h.deps.Spark == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "spark vertical unavailable")
		return
	}
	var req domain.SwipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginmw.RespondErrorWithDetail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid swipe payload", err.Error())
		return
	}
	res, err := h.deps.Spark.RecordSwipe(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}
	ginmw.RespondSuccess(c, res)
}

func (h *bffHandler) submitTradeRFQ(c *gin.Context) {
	if h.deps.Trade == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "trade vertical unavailable")
		return
	}
	var req domain.RFQRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginmw.RespondErrorWithDetail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid RFQ payload", err.Error())
		return
	}
	res, err := h.deps.Trade.SubmitRFQ(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}
	ginmw.RespondSuccess(c, res)
}

func (h *bffHandler) executePayTransfer(c *gin.Context) {
	if h.deps.Pay == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "pay vertical unavailable")
		return
	}
	var req domain.TransferRequest
	if !bindOptionalJSON(c, &req, "invalid transfer payload") {
		return
	}
	res, err := h.deps.Pay.ExecuteTransfer(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}
	ginmw.RespondSuccess(c, res)
}

func (h *bffHandler) updatePayCardControls(c *gin.Context) {
	if h.deps.Pay == nil {
		ginmw.RespondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "pay vertical unavailable")
		return
	}
	var req domain.CardControlRequest
	if !bindOptionalJSON(c, &req, "invalid card controls payload") {
		return
	}
	res, err := h.deps.Pay.UpdateCardControls(c.Request.Context(), req)
	if err != nil {
		respondDomainError(c, err)
		return
	}
	ginmw.RespondSuccess(c, res)
}

func bindOptionalJSON(c *gin.Context, dst any, errMsg string) bool {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return true
	}
	if err := c.ShouldBindJSON(dst); err != nil && !errors.Is(err, io.EOF) {
		ginmw.RespondErrorWithDetail(c, http.StatusBadRequest, "VALIDATION_ERROR", errMsg, err.Error())
		return false
	}
	return true
}

func respondDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrScreenNotFound), errors.Is(err, domain.ErrNotFound):
		ginmw.RespondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		ginmw.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	default:
		ginmw.RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
