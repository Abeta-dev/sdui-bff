// Command sdui-bff starts the Abeta Multi-Vertical SDUI Domain BFF HTTP server
// powering Abeta Fresh, Abeta Bites, Abeta Pulse, Abeta Mart, Abeta Spark, Abeta Trade, and Abeta Pay.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abeta-dev/go-libs/env"
	"github.com/abeta-dev/go-libs/logger"
	"github.com/abeta-dev/go-libs/shutdown"
	"github.com/abeta-dev/sdui-bff/internal/handler"
	"github.com/abeta-dev/sdui-bff/internal/infra/analyticsstore"
	"github.com/abeta-dev/sdui-bff/internal/infra/layoutclient"
	"github.com/abeta-dev/sdui-bff/internal/usecase"
	"github.com/abeta-dev/sdui-bff/internal/verticals/bites"
	"github.com/abeta-dev/sdui-bff/internal/verticals/fresh"
	"github.com/abeta-dev/sdui-bff/internal/verticals/mart"
	"github.com/abeta-dev/sdui-bff/internal/verticals/pay"
	"github.com/abeta-dev/sdui-bff/internal/verticals/pulse"
	"github.com/abeta-dev/sdui-bff/internal/verticals/spark"
	"github.com/abeta-dev/sdui-bff/internal/verticals/trade"
	"github.com/gin-gonic/gin"
)

// Build metadata injected at compile time via -ldflags -X.
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

type appConfig struct {
	Port            string
	Addr            string
	GinMode         string
	LogLevel        string
	LayoutEngineURL string
	JWTSecret       string
	RateLimitRPS    float64
	AllowedOrigins  []string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

func loadConfig() appConfig {
	port := strings.TrimSpace(env.String("PORT", "8085"))
	if port == "" {
		port = "8085"
	}
	addr := strings.TrimSpace(env.String("HTTP_ADDR", ":"+port))
	if addr == "" {
		addr = ":" + port
	}

	originsRaw := env.String("CORS_ALLOWED_ORIGINS", "*")
	parts := strings.Split(originsRaw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		origins = []string{"*"}
	}

	var rateLimitRPS float64
	if rawRPS := strings.TrimSpace(os.Getenv("RATE_LIMIT_RPS")); rawRPS != "" {
		if parsed, err := strconv.ParseFloat(rawRPS, 64); err == nil && parsed > 0 {
			rateLimitRPS = parsed
		}
	}

	return appConfig{
		Port:            port,
		Addr:            addr,
		GinMode:         env.String("GIN_MODE", gin.ReleaseMode),
		LogLevel:        strings.ToLower(strings.TrimSpace(env.String("LOG_LEVEL", "info"))),
		LayoutEngineURL: strings.TrimSpace(env.String("LAYOUT_ENGINE_URL", "")),
		JWTSecret:       strings.TrimSpace(env.String("JWT_SECRET", "")),
		RateLimitRPS:    rateLimitRPS,
		AllowedOrigins:  origins,
		ReadTimeout:     env.Duration("HTTP_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    env.Duration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:     env.Duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout: env.Duration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})).
		With(
			slog.String("service", "abeta-sdui-bff"),
			slog.String("version", version),
		)
}

func buildServer(cfg appConfig, log *slog.Logger) *http.Server {
	if log == nil {
		log = logger.Default()
	}
	gin.SetMode(cfg.GinMode)

	freshMod := fresh.New()
	bitesMod := bites.New()
	pulseMod := pulse.New()
	martMod := mart.New()
	sparkMod := spark.New()
	tradeMod := trade.New()
	payMod := pay.New()

	layoutClient := layoutclient.NewClient(cfg.LayoutEngineURL, nil)
	composer := usecase.NewScreenComposer(
		layoutClient,
		freshMod,
		bitesMod,
		pulseMod,
		martMod,
		sparkMod,
		tradeMod,
		payMod,
	)
	actionExec := usecase.NewActionExecutor()
	analyticsStore := analyticsstore.NewInMemoryStore(0)
	analyticsSvc := usecase.NewAnalyticsService(analyticsStore)

	router := handler.NewRouter(handler.Dependencies{
		Composer:       composer,
		Actions:        actionExec,
		Analytics:      analyticsSvc,
		LayoutClient:   layoutClient,
		Fresh:          freshMod,
		Bites:          bitesMod,
		Pulse:          pulseMod,
		Mart:           martMod,
		Spark:          sparkMod,
		Trade:          tradeMod,
		Pay:            payMod,
		JWTSecret:      cfg.JWTSecret,
		RateLimitRPS:   cfg.RateLimitRPS,
		Logger:         log,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	protocols.SetHTTP2(true)

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		Protocols:         protocols,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func run() error {
	cfg := loadConfig()
	log := newLogger(cfg.LogLevel)
	srv := buildServer(cfg, log)

	mode := "standalone"
	if cfg.LayoutEngineURL != "" {
		mode = "layout-engine"
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("starting Abeta SDUI Domain BFF server",
			slog.String("addr", cfg.Addr),
			slog.String("mode", mode),
			slog.String("commit", commit),
			slog.String("build_date", buildDate),
			slog.Bool("auth_enabled", cfg.JWTSecret != ""),
			slog.Float64("rate_limit_rps", cfg.RateLimitRPS),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", err)
		}
	}()

	mgr := shutdown.New(cfg.ShutdownTimeout)
	mgr.Register("http_server", func(ctx context.Context) error {
		log.Info("shutting down Abeta SDUI Domain BFF server")
		return srv.Shutdown(ctx)
	})

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- mgr.Wait()
	}()

	select {
	case err := <-errCh:
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		_ = mgr.Execute(ctx)
		return err
	case err := <-waitCh:
		if err != nil {
			return err
		}
	}

	log.Info("Abeta SDUI Domain BFF stopped cleanly")
	return nil
}

func main() {
	if err := run(); err != nil {
		logger.Default().Error("sdui-bff exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}
