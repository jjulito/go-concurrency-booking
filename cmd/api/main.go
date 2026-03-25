package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"reserva/config"
	"reserva/internal/adapters/handler"
	"reserva/internal/adapters/storage"
	"reserva/internal/core/services"
)

func main() {
	// Structured JSON logging — every log line is a JSON object with level, time,
	// msg, and any key-value pairs passed to the call site. This makes logs
	// queryable in Datadog, CloudWatch, GCP Logging, etc.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// 1. Load Config
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// 2. Storage Adapters
	pgPool, err := storage.NewPostgresDB(cfg)
	if err != nil {
		slog.Error("Failed to connect to Postgres", "error", err)
		os.Exit(1)
	}
	defer pgPool.Close()

	redisClient, err := storage.NewRedisClient(cfg)
	if err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	// 3. Repositories
	seatRepo := storage.NewPostgresSeatRepository(pgPool)
	resRepo := storage.NewPostgresReservationRepository(pgPool)
	eventRepo := storage.NewPostgresEventRepository(pgPool)
	lockRepo := storage.NewRedisLockRepository(redisClient)
	transactor := storage.NewPostgresTransactor(pgPool)

	// 4. Service
	bookingService := services.NewBookingService(seatRepo, resRepo, eventRepo, lockRepo, transactor)

	// 5. HTTP Handler
	httpHandler := handler.NewHTTPHandler(bookingService, cfg.StripeWebhookSecret)

	// 6. Router
	router := gin.Default()
	router.Use(handler.RateLimiterMiddleware(redisClient))
	httpHandler.RegisterRoutes(router)

	// Health endpoints — registered outside the API group, bypass rate limiting
	// and auth so k8s probes can reach them unconditionally.
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pgPool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unavailable", "error": err.Error()})
			return
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "redis_unavailable", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 7. Cancellable context tied to OS signals.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	worker := services.NewCleanupWorker(resRepo, bookingService, 1*time.Minute)
	worker.Start(ctx)

	// 8. HTTP Server
	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: router,
	}

	go func() {
		slog.Info("Server starting", "port", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutdown signal received, draining in-flight requests")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server stopped cleanly")
}
