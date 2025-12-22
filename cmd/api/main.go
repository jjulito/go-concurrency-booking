package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jjulito/reserva/config"
	"github.com/jjulito/reserva/internal/adapters/handler"
	"github.com/jjulito/reserva/internal/adapters/storage"
	"github.com/jjulito/reserva/internal/core/services"
)

func main() {
	// 1. Load Config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Storage Adapters
	pgPool, err := storage.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer pgPool.Close()

	redisClient, err := storage.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// 3. Repositories
	seatRepo := storage.NewPostgresSeatRepository(pgPool)
	resRepo := storage.NewPostgresReservationRepository(pgPool)
	eventRepo := storage.NewPostgresEventRepository(pgPool)
	lockRepo := storage.NewRedisLockRepository(redisClient)

	// 4. Service
	bookingService := services.NewBookingService(seatRepo, resRepo, eventRepo, lockRepo)

	// 5. HTTP Handler
	httpHandler := handler.NewHTTPHandler(bookingService)

	// 6. Router
	router := gin.Default()
	router.Use(handler.RateLimiterMiddleware(redisClient))
	httpHandler.RegisterRoutes(router)

	// 7. Background Worker
	worker := services.NewCleanupWorker(resRepo, bookingService, 1*time.Minute)
	// Start worker in a goroutine with context (ideally graceful shutdown context)
	worker.Start(context.Background())

	// 8. Start Server
	log.Printf("Starting server on port %s", cfg.AppPort)
	if err := router.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
