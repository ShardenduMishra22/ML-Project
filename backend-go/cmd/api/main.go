package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"landrisk/backend-go/internal/cache"
	"landrisk/backend-go/internal/config"
	"landrisk/backend-go/internal/db"
	"landrisk/backend-go/internal/handlers"
	"landrisk/backend-go/internal/kgis"
	"landrisk/backend-go/internal/ml"
	"landrisk/backend-go/internal/service"
)

func main() {
	cfg := config.Load()
	configureLogger(cfg.LogLevel)

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect postgres")
	}
	defer pool.Close()

	redisCache, err := cache.New(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize redis")
	}
	defer redisCache.Close()
	if err := redisCache.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("redis ping failed")
	}

	repo := db.NewRepository(pool)
	kgisClient := kgis.NewClient(cfg.KGISBaseURL, cfg.KGISTimeout, cfg.KGISRetryCount, cfg.KGISCacheTTL, redisCache, log.Logger)
	mlClient := ml.NewClient(cfg.MLServiceURL, cfg.MLTimeout, cfg.MLCacheTTL, redisCache, log.Logger)
	analyzer := service.NewAnalyzer(cfg, repo, redisCache, kgisClient, mlClient, log.Logger)
	h := handlers.New(analyzer, log.Logger)

	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.RequestTimeout,
		WriteTimeout: cfg.RequestTimeout,
	})
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.FrontendOrigin,
		AllowMethods: "GET,POST,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept",
	}))
	app.Use(func(c *fiber.Ctx) error {
		started := time.Now()
		err := c.Next()
		log.Info().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", c.Response().StatusCode()).
			Int64("durationMs", time.Since(started).Milliseconds()).
			Msg("request completed")
		return err
	})
	h.Register(app)

	go func() {
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "closed") {
				log.Fatal().Err(err).Msg("fiber server crashed")
			}
		}
	}()

	log.Info().Str("port", cfg.AppPort).Msg("backend service started")
	awaitShutdown(app)
}

func awaitShutdown(app *fiber.App) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	<-signalChan

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- app.Shutdown()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Error().Err(err).Msg("graceful shutdown failed")
		}
	case <-ctx.Done():
		log.Error().Msg("shutdown timeout exceeded")
	}
}

func configureLogger(level string) {
	parsed, err := zerolog.ParseLevel(level)
	if err != nil {
		parsed = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(parsed)
	zerolog.TimeFieldFormat = time.RFC3339Nano
	if os.Getenv("LOG_PRETTY") == "1" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}
	log.Info().Str("level", parsed.String()).Msg("logger configured")
}
